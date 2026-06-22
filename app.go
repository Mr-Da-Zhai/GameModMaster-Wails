package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"GameModMaster/internal/index"
	"GameModMaster/internal/model"
	"GameModMaster/internal/repo"
	"GameModMaster/internal/scraper"
	"GameModMaster/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Event names emitted to the frontend.
const (
	EventRefreshProgress   = "refresh:progress"
	EventDownloadProgress  = "download:progress"
	EventDetailProgress    = "detail:progress"
)

// AppService is the main service exposed to the Wails frontend.
// All methods on this struct are callable from JavaScript via Wails bindings.
type AppService struct {
	db              *repo.DB
	gameRepo        *repo.GameRepo
	trainerRepo     *repo.TrainerRepo
	stateRepo       *repo.StateRepo
	idx             *index.Index
	mappingService  *service.MappingService
	scraperService  *scraper.Scraper
	downloadService *service.DownloadService
	dataDir         string

	ctx    context.Context
	window *application.WebviewWindow

	// refresh task state (guarded by refreshMu)
	refreshMu     sync.Mutex
	refreshing    bool
	refreshResult string

	// cancelRefresh is set non-nil while a refresh is running; cancelling
	// closes it to signal the crawler loop to stop after the current page.
	// Guarded by refreshMu.
	cancelRefresh chan struct{}
	// cancelDetail is set non-nil while a GetTrainerDetail background fetch
	// is running; cancelling closes it to abort the in-flight HTTP request.
	cancelDetailMu sync.Mutex
	cancelDetail   map[int32]context.CancelFunc
	// downloadCancel holds cancel funcs for in-flight downloads, keyed by
	// trainer id. Guarded by downloadCancelMu.
	downloadCancelMu sync.Mutex
	downloadCancel   map[int32]context.CancelFunc

	// detailFetching atomically tracks which game ids have an in-flight
	// lazy detail fetch (used by the UI to show a spinner per row).
	detailFetching sync.Map // map[int32]struct{}
}

// NewAppService creates and initializes the AppService.
// embeddedMapping is the name_mapping.json data embedded in the binary.
func NewAppService(embeddedMapping []byte) *AppService {
	a := &AppService{
		cancelDetail:   make(map[int32]context.CancelFunc),
		downloadCancel: make(map[int32]context.CancelFunc),
	}

	// 1. Determine data directory
	a.resolveDataDir()
	log.Printf("[AppService] Data directory: %s", a.dataDir)

	// 2. Open SQLite database
	dbPath := filepath.Join(a.dataDir, "gamm.db")
	db, err := repo.Open(dbPath)
	if err != nil {
		log.Fatalf("[AppService] Failed to open database: %v", err)
	}
	a.db = db

	// 3. Init repositories
	a.gameRepo = repo.NewGameRepo(a.db)
	a.trainerRepo = repo.NewTrainerRepo(a.db)
	a.stateRepo = repo.NewStateRepo(a.db)

	// 4. Load name mapping from embedded data
	a.mappingService = service.NewMappingService()
	if len(embeddedMapping) > 0 {
		if err := a.mappingService.LoadFromBytes(embeddedMapping); err != nil {
			log.Printf("[AppService] Warning: failed to load name mapping: %v", err)
		} else {
			log.Printf("[AppService] Name mapping loaded (%d entries)", len(a.mappingService.GetMapping()))
		}
	}

	// 5. Build memory index from database
	a.idx = index.New()
	if err := a.idx.LoadFromDB(a.gameRepo, a.trainerRepo, a.stateRepo); err != nil {
		log.Fatalf("[AppService] Failed to build index: %v", err)
	}
	a.idx.LoadNameMapping(a.mappingService.GetMapping(), a.mappingService.GetAliases())
	log.Printf("[AppService] Index built: %d games, %d trainers, %d states",
		len(a.idx.GamesByID), len(a.idx.TrainersByID), len(a.idx.StatesByID))

	// 6. Init scraper and download services
	a.scraperService = scraper.NewScraper(a.gameRepo, a.trainerRepo, a.mappingService)
	a.downloadService = service.NewDownloadService(a.stateRepo)

	// 7. Persist mapping count so the settings page can show it.
	a.persistMappingCount()

	return a
}

// ServiceStartup is called by Wails once the application is running.
// We capture the context for cancellation of long-running tasks and kick off
// the first-run data seeding if the database is empty.
func (a *AppService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	a.ctx = ctx

	// First-run seeding: if the index is empty, fetch data in the background so
	// the user never sees an empty home page with a manual "load" button.
	if len(a.idx.GamesByID) == 0 && !a.IsRefreshing() {
		log.Printf("[AppService] First run detected (empty DB) — seeding data in background")
		// RefreshData launches a goroutine and returns immediately.
		_ = a.RefreshData()
	}

	return nil
}

// SetWindow stores a reference to the main window so the service can emit events.
// Called from main() after the window is created.
func (a *AppService) SetWindow(w *application.WebviewWindow) {
	a.window = w
}

// emitEvent broadcasts a custom event to the frontend. No-op if no window yet.
func (a *AppService) emitEvent(name string, data ...any) {
	if a.window == nil {
		return
	}
	a.window.EmitEvent(name, data...)
}

// Shutdown closes the database connection on app exit.
func (a *AppService) Shutdown() {
	if a.db != nil {
		a.db.Close()
		log.Println("[AppService] Database closed")
	}
}

// resolveDataDir sets the data directory to a stable, user-scoped location
// that is NOT beside the executable (which could be on the Desktop and easily
// deleted together with the app).
//
//   - Windows: %LOCALAPPDATA%\GameModMaster  (e.g. C:\Users\<u>\AppData\Local\GameModMaster)
//   - macOS:   ~/Library/Application Support/GameModMaster
//   - Linux:   $XDG_DATA_HOME/GameModMaster  (defaults to ~/.local/share/GameModMaster)
//
// If a legacy `data/` directory exists beside the executable (older builds
// stored the DB there), its contents are migrated into the new location once.
func (a *AppService) resolveDataDir() {
	a.dataDir = userAppDataDir()

	if err := os.MkdirAll(a.dataDir, 0755); err != nil {
		log.Printf("[AppService] Warning: could not create data dir %s: %v", a.dataDir, err)
	}

	// One-time migration from the old beside-exe `data/` location.
	a.migrateLegacyDataDir()
}

// userAppDataDir returns the platform-appropriate, user-scoped data directory.
// Falls back to the current working directory only if nothing else works.
func userAppDataDir() string {
	// os.UserConfigDir gives Roaming on Windows; we prefer LocalAppData so the
	// (potentially large) DB and downloads don't roam across machines.
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LocalAppData"); local != "" {
			return filepath.Join(local, "GameModMaster")
		}
	}
	if cfg, err := os.UserConfigDir(); err == nil && cfg != "" {
		return filepath.Join(cfg, "GameModMaster")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".GameModMaster")
	}
	return filepath.Join(".", "GameModMaster")
}

// migrateLegacyDataDir moves gamm.db (+ downloads) from beside the executable
// into the new user-scoped location, so upgrades don't lose old data and the
// Desktop stays clean. Best-effort: failures are logged, not fatal.
func (a *AppService) migrateLegacyDataDir() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	legacyDir := filepath.Join(filepath.Dir(exePath), "data")
	info, err := os.Stat(legacyDir)
	if err != nil || !info.IsDir() {
		return // nothing to migrate
	}

	log.Printf("[AppService] Migrating legacy data from %s -> %s", legacyDir, a.dataDir)

	entries, err := os.ReadDir(legacyDir)
	if err != nil {
		log.Printf("[AppService] read legacy dir failed: %v", err)
		return
	}

	for _, e := range entries {
		src := filepath.Join(legacyDir, e.Name())
		dst := filepath.Join(a.dataDir, e.Name())

		// Don't overwrite an existing, newer file in the destination.
		if _, err := os.Stat(dst); err == nil {
			continue
		}

		if err := os.Rename(src, dst); err != nil {
			// Rename can fail across volumes (e.g. Desktop on a different drive
			// than LocalAppData). Fall back to copy + remove for the DB file.
			if copyErr := copyFile(src, dst); copyErr != nil {
				log.Printf("[AppService] migrate %s failed: %v", e.Name(), copyErr)
				continue
			}
			_ = os.Remove(src)
		}
		log.Printf("[AppService] migrated %s", e.Name())
	}

	// Remove the now-empty legacy dir (and its `downloads/` subdir) if possible.
	_ = os.RemoveAll(legacyDir)
}

// copyFile copies a single file from src to dst (used as a Rename fallback).
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	buf := make([]byte, 32*1024)
	for {
		n, err := in.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			break
		}
	}
	return nil
}

// refreshIndex reloads the in-memory index from the database.
func (a *AppService) refreshIndex() {
	if err := a.idx.Refresh(a.gameRepo, a.trainerRepo, a.stateRepo); err != nil {
		log.Printf("[AppService] Failed to refresh index: %v", err)
	}
}

// persistMappingCount stores the loaded mapping count into kv_store so the
// settings page can read it through GetSettings().
func (a *AppService) persistMappingCount() {
	count := len(a.mappingService.GetMapping())
	if count == 0 {
		return
	}
	_ = a.setKV("mapping_count", fmt.Sprintf("%d", count))
}

// setKV writes a single key/value into kv_store.
func (a *AppService) setKV(key, value string) error {
	now := time.Now().Unix()
	_, err := a.db.Exec(
		"INSERT OR REPLACE INTO kv_store (key, value, updated_at) VALUES (?, ?, ?)",
		key, value, now,
	)
	return err
}

// ===== Data Query Methods =====

// GetTrainers returns paginated trainers for the home page.
// Returns games sorted by update time with their trainer info and states.
func (a *AppService) GetTrainers(page int, pageSize int) ([]map[string]interface{}, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	games := a.idx.GamesByUpdated
	total := len(games)
	start := (page - 1) * pageSize
	if start >= total {
		return []map[string]interface{}{}, nil
	}
	end := start + pageSize
	if end > total {
		end = total
	}

	pageGames := games[start:end]
	results := make([]map[string]interface{}, 0, len(pageGames))

	for _, g := range pageGames {
		entry := a.buildGameEntry(g)
		results = append(results, entry)
	}

	return results, nil
}

// GetTotalGames returns the total number of games in the index.
func (a *AppService) GetTotalGames() int {
	return len(a.idx.GamesByID)
}

// SearchTrainers performs a LIVE search against flingtrainer.com.
//
// The local DB only caches a subset of games (whatever has been browsed or
// downloaded), so searching it would miss most of the library. Instead we
// always query the remote site: a Chinese query is first resolved to its
// English title via the name mapping, then searched remotely. Results are
// translated to Chinese display names, cached locally, and returned.
func (a *AppService) SearchTrainers(query string) ([]map[string]interface{}, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return a.GetTrainers(1, 50)
	}

	games, err := a.scraperService.SearchRemote(q)
	if err != nil {
		return nil, fmt.Errorf("remote search: %w", err)
	}

	// Rebuild the in-memory index so the freshly cached games are queryable
	// (needed for detail/download flows that go through the index by ID).
	a.refreshIndex()

	results := make([]map[string]interface{}, 0, len(games))
	for _, g := range games {
		// Re-read from index to pick up the DB-assigned ID after upsert.
		if stored := a.idx.GamesByNameEN[strings.ToLower(g.NameEN)]; stored != nil {
			g = stored
		}
		results = append(results, a.buildGameEntry(g))
	}

	return results, nil
}

// GetTrainerDetail returns detail for a specific game (all trainer versions).
//
// If trainer details are already cached, this is fast and synchronous.
// If they are NOT cached (typical for games found through remote search),
// this returns a sentinel error so the frontend can show a loading state
// and call PrefetchTrainerDetail(gameID) to fetch in the background; that
// call emits a "detail:progress" event when done, after which the frontend
// calls GetTrainerDetail again to render the data.
//
// This avoids blocking the (single-threaded) Wails binding call on a slow
// remote fetch, which previously froze the whole UI for several seconds.
func (a *AppService) GetTrainerDetail(gameID int32) (map[string]interface{}, error) {
	g, ok := a.idx.GamesByID[gameID]
	if !ok {
		return nil, fmt.Errorf("game not found: %d", gameID)
	}

	needFetch := len(a.idx.GetTrainersForGame(gameID)) == 0 && g.SourceURL != ""

	if needFetch {
		// Trigger a background prefetch if one isn't already running, then
		// return a sentinel error immediately so the UI can show "loading".
		if !a.isDetailFetching(gameID) {
			go a.prefetchDetail(gameID)
		}
		return nil, ErrDetailNotReady
	}

	return a.buildTrainerDetailResult(gameID), nil
}

// ErrDetailNotReady is returned by GetTrainerDetail when trainer rows for the
// requested game are not cached yet. The frontend should treat it as "loading"
// (a background prefetch has already been kicked off) and re-call
// GetTrainerDetail when it receives the "detail:progress" done event.
var ErrDetailNotReady = fmt.Errorf("detail not cached: prefetch in progress")

// PrefetchTrainerDetail explicitly kicks off a background fetch of a game's
// detail page and returns immediately. Completion is reported via the
// "detail:progress" event with {game_id, done, error}. Safe to call even if
// a fetch is already running for the same game (no-op in that case).
func (a *AppService) PrefetchTrainerDetail(gameID int32) error {
	if _, ok := a.idx.GamesByID[gameID]; !ok {
		return fmt.Errorf("game not found: %d", gameID)
	}
	if a.isDetailFetching(gameID) {
		return nil
	}
	go a.prefetchDetail(gameID)
	return nil
}

// isDetailFetching reports whether a background detail fetch is in progress.
func (a *AppService) isDetailFetching(gameID int32) bool {
	_, ok := a.detailFetching.Load(gameID)
	return ok
}

// prefetchDetail fetches and stores a game's trainer detail in the background,
// then emits detail:progress. Safe to run concurrently; dedup'd via the
// detailFetching map so a second call for the same game is a no-op.
func (a *AppService) prefetchDetail(gameID int32) {
	// Dedup: if already fetching, do nothing.
	if _, loaded := a.detailFetching.LoadOrStore(gameID, struct{}{}); loaded {
		return
	}
	defer a.detailFetching.Delete(gameID)

	g, ok := a.idx.GamesByID[gameID]
	if !ok {
		a.emitEvent(EventDetailProgress, map[string]interface{}{
			"game_id": gameID,
			"done":    true,
			"error":   "game not found",
		})
		return
	}

	// Skip if data is already cached (race: someone else fetched it).
	if len(a.idx.GetTrainersForGame(gameID)) > 0 {
		a.emitEvent(EventDetailProgress, map[string]interface{}{
			"game_id": gameID,
			"done":    true,
		})
		return
	}

	page, err := a.scraperService.FetchDetailPage(g.SourceURL)
	if err != nil {
		a.emitEvent(EventDetailProgress, map[string]interface{}{
			"game_id": gameID,
			"done":    true,
			"error":   err.Error(),
		})
		return
	}

	var toSave []*model.Trainer
	for _, t := range page.Trainers {
		t.GameID = g.ID
		toSave = append(toSave, t)
	}
	if len(toSave) > 0 {
		if err := a.trainerRepo.BatchUpsert(toSave); err != nil {
			a.emitEvent(EventDetailProgress, map[string]interface{}{
				"game_id": gameID,
				"done":    true,
				"error":   err.Error(),
			})
			return
		}
	}
	a.refreshIndex()

	a.emitEvent(EventDetailProgress, map[string]interface{}{
		"game_id": gameID,
		"done":    true,
	})
}

// buildTrainerDetailResult assembles the JSON-friendly detail payload for a
// game assuming trainer rows are already cached in the index.
func (a *AppService) buildTrainerDetailResult(gameID int32) map[string]interface{} {
	g := a.idx.GamesByID[gameID]

	trainers := a.idx.GetTrainersForGame(gameID)
	trainerList := make([]map[string]interface{}, 0, len(trainers))

	for _, t := range trainers {
		tMap := map[string]interface{}{
			"id":             t.ID,
			"game_id":        t.GameID,
			"version":        t.Version,
			"game_version":   t.GameVersion,
			"download_url":   t.DownloadURL,
			"file_size":      t.FileSize,
			"file_name":      t.FileName,
			"download_count": t.DownloadCount,
			"source_hash":    t.SourceHash,
			"updated_at":     t.UpdatedAt,
		}

		// Attach state if exists
		state := a.idx.GetTrainerState(t.ID)
		if state != nil {
			tMap["status"] = int(state.Status)
			tMap["local_path"] = state.LocalPath
			tMap["installed_at"] = state.InstalledAt
			tMap["launched_at"] = state.LaunchedAt
		} else {
			tMap["status"] = int(model.StatusAvailable)
		}

		trainerList = append(trainerList, tMap)
	}

	// Determine best display name
	displayName := g.NameLocal
	if displayName == "" {
		displayName = g.NameEN
	}

	return map[string]interface{}{
		"game": map[string]interface{}{
			"id":           g.ID,
			"source_id":    g.SourceID,
			"name_en":      g.NameEN,
			"name_local":   g.NameLocal,
			"display_name": displayName,
			"cover_url":    g.CoverURL,
			"source_url":   g.SourceURL,
			"options_num":  g.OptionsNum,
			"updated_at":   g.UpdatedAt,
		},
		"trainers": trainerList,
	}
}

// GetDownloadedTrainers returns all trainers with status = downloaded or installed.
func (a *AppService) GetDownloadedTrainers() ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for _, g := range a.idx.GamesByUpdated {
		trainers := a.idx.GetTrainersForGame(g.ID)
		for _, t := range trainers {
			state := a.idx.GetTrainerState(t.ID)
			if state == nil {
				continue
			}
			if state.Status == model.StatusDownloaded || state.Status == model.StatusInstalled {
				entry := a.buildTrainerWithGameEntry(t, g, state)
				results = append(results, entry)
			}
		}
	}

	return results, nil
}

// GetInstalledTrainers returns all trainers with status = installed.
func (a *AppService) GetInstalledTrainers() ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	for _, g := range a.idx.GamesByUpdated {
		trainers := a.idx.GetTrainersForGame(g.ID)
		for _, t := range trainers {
			state := a.idx.GetTrainerState(t.ID)
			if state == nil || state.Status != model.StatusInstalled {
				continue
			}
			entry := a.buildTrainerWithGameEntry(t, g, state)
			results = append(results, entry)
		}
	}

	return results, nil
}

// IsRefreshing reports whether a refresh task is currently running.
func (a *AppService) IsRefreshing() bool {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	return a.refreshing
}

// GetRefreshResult returns the human-readable result of the last refresh.
func (a *AppService) GetRefreshResult() string {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	return a.refreshResult
}

// ===== Action Methods =====

// DownloadTrainer downloads a trainer file.
// Runs synchronously; progress is emitted via the "download:progress" event.
// Use CancelDownload(trainerID) to abort an in-flight download.
func (a *AppService) DownloadTrainer(trainerID int32) error {
	t, ok := a.idx.TrainersByID[trainerID]
	if !ok {
		return fmt.Errorf("trainer not found: %d", trainerID)
	}

	if t.DownloadURL == "" {
		return fmt.Errorf("trainer %d has no download URL", trainerID)
	}

	// Determine download directory (honour user-configured path if set)
	downloadDir := a.downloadDir()
	fileName := t.FileName
	if fileName == "" {
		fileName = filepath.Base(t.DownloadURL)
	}

	// Progress callback -> emit event to frontend
	progress := func(downloaded, total int64, speed float64) {
		a.emitEvent(EventDownloadProgress, map[string]interface{}{
			"trainer_id": trainerID,
			"downloaded": downloaded,
			"total":      total,
			"speed":      speed,
		})
	}

	// Create a cancellable context for this download and register it so
	// CancelDownload can abort it.
	ctx, cancel := context.WithCancel(a.requestContext())
	a.registerDownloadCancel(trainerID, cancel)
	defer a.unregisterDownloadCancel(trainerID)

	localPath, err := a.downloadService.Download(ctx, t.DownloadURL, downloadDir, fileName, progress)
	if err != nil {
		// Distinguish cancellation from real failures so the UI can react.
		if ctx.Err() == context.Canceled {
			a.emitEvent(EventDownloadProgress, map[string]interface{}{
				"trainer_id": trainerID,
				"cancelled":  true,
				"done":       true,
			})
			return fmt.Errorf("download cancelled")
		}
		return fmt.Errorf("download failed: %w", err)
	}

	// If it's a zip file, extract it
	if strings.HasSuffix(strings.ToLower(localPath), ".zip") {
		extractDir := filepath.Join(downloadDir, fmt.Sprintf("trainer_%d", trainerID))
		extracted, err := a.downloadService.ExtractZIP(localPath, extractDir)
		if err != nil {
			return fmt.Errorf("extract failed: %w", err)
		}

		// Pick the launchable file: prefer .exe > .ink > first file. Earlier
		// code took extracted[0] blindly which could land on a README and
		// "lose" the actual trainer exe.
		localPath = pickLaunchable(extracted, localPath)
		// Clean up zip file
		os.Remove(filepath.Join(downloadDir, fileName))
	}

	// Mark as downloaded
	if err := a.downloadService.MarkDownloaded(trainerID, localPath); err != nil {
		return fmt.Errorf("mark downloaded failed: %w", err)
	}

	// Signal completion
	a.emitEvent(EventDownloadProgress, map[string]interface{}{
		"trainer_id": trainerID,
		"done":       true,
	})

	// Refresh index
	a.refreshIndex()
	return nil
}

// CancelDownload aborts an in-flight download for the given trainer id.
// Returns an error if no download is currently running for that trainer.
func (a *AppService) CancelDownload(trainerID int32) error {
	a.downloadCancelMu.Lock()
	defer a.downloadCancelMu.Unlock()
	cancel, ok := a.downloadCancel[trainerID]
	if !ok {
		return fmt.Errorf("no download in progress for trainer %d", trainerID)
	}
	cancel()
	return nil
}

func (a *AppService) registerDownloadCancel(trainerID int32, cancel context.CancelFunc) {
	a.downloadCancelMu.Lock()
	defer a.downloadCancelMu.Unlock()
	a.downloadCancel[trainerID] = cancel
}

func (a *AppService) unregisterDownloadCancel(trainerID int32) {
	a.downloadCancelMu.Lock()
	defer a.downloadCancelMu.Unlock()
	delete(a.downloadCancel, trainerID)
}

// pickLaunchable chooses the best file to treat as the trainer executable from
// a list of extracted files. Preference order: .exe (case-insensitive) > .ink
// > first file. Falls back to fallbackPath if the list is empty.
func pickLaunchable(files []string, fallbackPath string) string {
	if len(files) == 0 {
		return fallbackPath
	}
	// First pass: prefer an .exe whose name does NOT look like an
	// uninstaller/readme.
	for _, f := range files {
		lf := strings.ToLower(filepath.Base(f))
		if strings.HasSuffix(lf, ".exe") && !looksLikeAuxFile(lf) {
			return f
		}
	}
	// Second pass: any .exe.
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".exe") {
			return f
		}
	}
	// Third pass: .ink.
	for _, f := range files {
		if strings.HasSuffix(strings.ToLower(f), ".ink") {
			return f
		}
	}
	return files[0]
}

// looksLikeAuxFile returns true for filenames that are clearly NOT the trainer
// binary (uninstallers, readme, etc.) so we don't pick them as the launchable.
func looksLikeAuxFile(lowerBasename string) bool {
	return strings.Contains(lowerBasename, "unins") ||
		strings.Contains(lowerBasename, "readme") ||
		strings.Contains(lowerBasename, "license") ||
		strings.Contains(lowerBasename, "changelog")
}

// InstallTrainer installs a downloaded trainer.
func (a *AppService) InstallTrainer(trainerID int32) error {
	state := a.idx.GetTrainerState(trainerID)
	if state == nil {
		return fmt.Errorf("trainer %d is not downloaded", trainerID)
	}
	if state.Status != model.StatusDownloaded {
		return fmt.Errorf("trainer %d must be downloaded first (current status: %d)", trainerID, state.Status)
	}

	// Mark as installed
	if err := a.downloadService.MarkInstalled(trainerID, state.LocalPath); err != nil {
		return fmt.Errorf("mark installed failed: %w", err)
	}

	a.refreshIndex()
	return nil
}

// LaunchTrainer launches an installed trainer executable.
func (a *AppService) LaunchTrainer(trainerID int32) error {
	state := a.idx.GetTrainerState(trainerID)
	if state == nil || state.Status != model.StatusInstalled {
		return fmt.Errorf("trainer %d is not installed", trainerID)
	}

	if state.LocalPath == "" {
		return fmt.Errorf("trainer %d has no local path", trainerID)
	}

	// Check file exists
	if _, err := os.Stat(state.LocalPath); os.IsNotExist(err) {
		return fmt.Errorf("trainer file not found: %s", state.LocalPath)
	}

	// Launch
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "start", "", state.LocalPath)
	} else if runtime.GOOS == "darwin" {
		cmd = exec.Command("open", state.LocalPath)
	} else {
		cmd = exec.Command("xdg-open", state.LocalPath)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launch failed: %w", err)
	}

	// Update launch time
	if err := a.downloadService.MarkLaunched(trainerID); err != nil {
		log.Printf("[AppService] Warning: failed to update launch time: %v", err)
	}

	a.refreshIndex()
	return nil
}

// DeleteTrainer removes a downloaded/installed trainer.
func (a *AppService) DeleteTrainer(trainerID int32) error {
	state := a.idx.GetTrainerState(trainerID)
	if state == nil {
		return fmt.Errorf("trainer %d has no state to delete", trainerID)
	}

	// Remove local files if they exist
	if state.LocalPath != "" {
		if err := os.Remove(state.LocalPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[AppService] Warning: failed to remove file %s: %v", state.LocalPath, err)
		}
		// Try to remove the trainer directory too
		trainerDir := filepath.Join(a.downloadDir(), fmt.Sprintf("trainer_%d", trainerID))
		os.RemoveAll(trainerDir)
	}

	// Remove state
	if err := a.downloadService.RemoveState(trainerID); err != nil {
		return fmt.Errorf("remove state failed: %w", err)
	}

	a.refreshIndex()
	return nil
}

// RefreshData fetches latest data from flingtrainer.com asynchronously.
// It returns immediately; progress and completion are reported via the
// "refresh:progress" event. Use IsRefreshing() / GetRefreshResult() to poll.
//
// If a previous crawl was interrupted, this resumes from the last page that
// was successfully saved (so closing and reopening the app doesn't restart
// the full crawl from page 1). Once finished, the resume marker is cleared.
func (a *AppService) RefreshData() error {
	a.refreshMu.Lock()
	if a.refreshing {
		a.refreshMu.Unlock()
		return fmt.Errorf("refresh already in progress")
	}
	a.refreshing = true
	a.refreshResult = ""
	a.cancelRefresh = make(chan struct{})
	a.refreshMu.Unlock()

	go a.runRefresh()
	return nil
}

// CancelRefresh asks an in-progress crawl to stop after the current page.
// Returns an error if no crawl is running. The summary emitted to the UI on
// completion is annotated with "(已取消)" so the user knows it was not a
// normal finish.
func (a *AppService) CancelRefresh() error {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	if !a.refreshing || a.cancelRefresh == nil {
		return fmt.Errorf("no refresh in progress")
	}
	select {
	case <-a.cancelRefresh:
		// already closed
	default:
		close(a.cancelRefresh)
	}
	return nil
}

// RefreshDataSync fetches latest data synchronously and returns when done.
// Useful for first-run seeding or tests; prefer RefreshData from the UI.
// Fetches ALL pages (full library) so search works for every FLiNG game.
func (a *AppService) RefreshDataSync() (string, error) {
	a.refreshMu.Lock()
	if a.refreshing {
		a.refreshMu.Unlock()
		return "", fmt.Errorf("refresh already in progress")
	}
	a.refreshing = true
	a.cancelRefresh = make(chan struct{})
	a.refreshMu.Unlock()
	defer func() {
		a.refreshMu.Lock()
		a.refreshing = false
		a.cancelRefresh = nil
		a.refreshMu.Unlock()
	}()

	// pageCount <= 0 means "fetch everything". Cancellation is impossible
	// here (no event loop), so pass a never-closed channel.
	summary, err, detailErrors, _ := a.doFetch(1, 0, make(chan struct{}))
	if detailErrors > 0 {
		summary = fmt.Sprintf("%s · %d 个详情页获取失败", summary, detailErrors)
	}
	return summary, err
}

// runRefresh executes the fetch off the main goroutine.
func (a *AppService) runRefresh() {
	defer func() {
		a.refreshMu.Lock()
		a.refreshing = false
		a.cancelRefresh = nil
		a.refreshMu.Unlock()
	}()

	// Snapshot the cancel channel under the lock; doFetch reads it locally.
	a.refreshMu.Lock()
	cancelCh := a.cancelRefresh
	a.refreshMu.Unlock()

	// Resume from the last saved page if a previous crawl was interrupted.
	startPage := 1
	if saved := a.getKV("resume_from_page"); saved != "" {
		if n, err := strconv.Atoi(saved); err == nil && n > 1 {
			startPage = n
			log.Printf("[AppService] resuming crawl from page %d", startPage)
		}
	}

	// Always fetch the full library so search covers every game.
	summary, err, detailErrors, cancelled := a.doFetch(startPage, 0, cancelCh)
	if cancelled {
		summary = fmt.Sprintf("%s (已取消 — 进度已保存，下次将续传)", summary)
	} else if err != nil {
		log.Printf("[AppService] Refresh error: %v", err)
		summary = fmt.Sprintf("%s (部分出错: %v)", summary, err)
	}
	if !cancelled && detailErrors > 0 {
		summary = fmt.Sprintf("%s · %d 个详情页获取失败", summary, detailErrors)
	}
	a.refreshMu.Lock()
	a.refreshResult = summary
	a.refreshMu.Unlock()

	// Crawl finished (not cancelled) — clear the resume marker so the next
	// manual refresh starts fresh. If cancelled, keep the marker so the next
	// RefreshData() picks up where we left off.
	if !cancelled {
		_ = a.setKV("resume_from_page", "")
	}

	// Notify the frontend that the refresh finished.
	a.emitEvent(EventRefreshProgress, map[string]interface{}{
		"done":          true,
		"cancelled":     cancelled,
		"summary":       summary,
		"detail_errors": detailErrors,
	})
}

// doFetch performs the multi-page crawl with progress events.
// pageCount <= 0 means "probe and fetch all pages".
// cancelCh, if closed, signals the loop to stop after the current page.
// Returns (summary, firstError, totalDetailErrors, cancelled).
func (a *AppService) doFetch(startPage, pageCount int, cancelCh chan struct{}) (string, error, int, bool) {
	total := pageCount
	if total <= 0 {
		// Probe the site for the real last page once, up front.
		probed, err := a.scraperService.CountTotalPages()
		if err != nil {
			log.Printf("[AppService] count pages failed: %v", err)
			probed = 49 // sensible fallback (kept in sync with current site size)
		}
		total = probed - startPage + 1
		if total < 1 {
			total = 1
		}
		log.Printf("[AppService] full crawl: %d pages (start=%d)", total, startPage)
		// Tell the UI how many pages to expect.
		a.emitEvent(EventRefreshProgress, map[string]interface{}{
			"total":   total,
			"current": 0,
			"phase":   "probe",
		})
	}

	totalGames := 0
	totalTrainers := 0
	totalDetailErrors := 0
	var firstErr error
	cancelled := false

	for i := 0; i < total; i++ {
		// Honor a cancel signal BEFORE starting the next page.
		select {
		case <-cancelCh:
			cancelled = true
		default:
		}
		if cancelled {
			break
		}

		page := startPage + i
		games, trainers, err := a.scraperService.FetchAndSave(page)
		totalGames += games
		totalTrainers += trainers
		if derrs := a.scraperService.LastDetailErrors(); derrs > 0 {
			totalDetailErrors += derrs
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Printf("[AppService] page %d: %v", page, err)
		}

		// Persist the resume marker AFTER each successful page save so a crash
		// or app-quit during a long crawl lets us pick up where we left off.
		// We set it to the NEXT page (only if there is one).
		if page < startPage+total-1 {
			_ = a.setKV("resume_from_page", strconv.Itoa(page+1))
		}

		// Emit progress on every page so the UI bar advances smoothly.
		a.emitEvent(EventRefreshProgress, map[string]interface{}{
			"page":          page,
			"total":         total,
			"current":       i + 1,
			"games":         totalGames,
			"trainers":      totalTrainers,
			"detail_errors": totalDetailErrors,
		})

		// Refresh the in-memory index periodically (every 3 pages) so the home
		// grid populates incrementally instead of staying empty for minutes.
		if (i+1)%3 == 0 {
			a.refreshIndex()
		}
	}

	// Final rebuild with all new data
	a.refreshIndex()

	summary := fmt.Sprintf("已更新 %d 个游戏, %d 个修改器", totalGames, totalTrainers)
	return summary, firstErr, totalDetailErrors, cancelled
}

// requestContext returns the app context if available, else background.
func (a *AppService) requestContext() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

// downloadDir returns the configured download directory.
func (a *AppService) downloadDir() string {
	if custom := a.getKV("download_dir"); custom != "" {
		return custom
	}
	return filepath.Join(a.dataDir, "downloads")
}

// getKV reads a single key from kv_store (empty string if missing).
func (a *AppService) getKV(key string) string {
	row := a.db.QueryRow("SELECT value FROM kv_store WHERE key = ?", key)
	var v string
	if err := row.Scan(&v); err != nil {
		return ""
	}
	return v
}

// ===== Settings Methods =====

// GetSettings returns app settings from kv_store.
func (a *AppService) GetSettings() map[string]interface{} {
	settings := map[string]interface{}{
		"data_dir":    a.dataDir,
		"download_dir": a.downloadDir(),
	}

	// Load settings from kv_store
	rows, err := a.db.Query("SELECT key, value FROM kv_store")
	if err != nil {
		return settings
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		// Try to parse JSON values
		var jsonVal interface{}
		if err := json.Unmarshal([]byte(value), &jsonVal); err == nil {
			settings[key] = jsonVal
		} else {
			settings[key] = value
		}
	}

	// Ensure mapping_count reflects the live mapping service if kv is empty.
	if settings["mapping_count"] == nil || settings["mapping_count"] == "" {
		if n := len(a.mappingService.GetMapping()); n > 0 {
			settings["mapping_count"] = n
		}
	}

	return settings
}

// SaveSettings saves settings to kv_store.
func (a *AppService) SaveSettings(settings map[string]interface{}) error {
	for key, val := range settings {
		// Skip read-only keys
		if key == "data_dir" {
			continue
		}

		var valueStr string
		switch v := val.(type) {
		case string:
			valueStr = v
		default:
			bytes, err := json.Marshal(v)
			if err != nil {
				return fmt.Errorf("marshal setting %q: %w", key, err)
			}
			valueStr = string(bytes)
		}

		if err := a.setKV(key, valueStr); err != nil {
			return fmt.Errorf("save setting %q: %w", key, err)
		}
	}

	return nil
}

// SetDownloadDir configures the download directory and creates it if needed.
func (a *AppService) SetDownloadDir(dir string) error {
	if dir == "" {
		return fmt.Errorf("download dir cannot be empty")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create download dir: %w", err)
	}
	return a.setKV("download_dir", dir)
}

// GetDataDir returns the data directory path.
func (a *AppService) GetDataDir() string {
	return a.dataDir
}

// GetMappingCount returns the number of loaded name-mapping entries.
func (a *AppService) GetMappingCount() int {
	return a.mappingService.Count()
}

// MappingEntry mirrors service.MappingEntry for the frontend.
type MappingEntry struct {
	NameEN  string   `json:"name_en"`
	NameZH  string   `json:"name_zh"`
	Aliases []string `json:"aliases"`
}

// GetMappingEntries returns a paginated, optionally filtered, view of the
// name-mapping table. Used by the mapping-management UI in Settings.
// query is a case-insensitive substring match against name_en, name_zh,
// and aliases. offset/limit control pagination.
func (a *AppService) GetMappingEntries(query string, offset, limit int) ([]MappingEntry, error) {
	raw := a.mappingService.ListEntries(query, offset, limit)
	out := make([]MappingEntry, 0, len(raw))
	for _, r := range raw {
		out = append(out, MappingEntry{
			NameEN:  r.NameEN,
			NameZH:  r.NameZH,
			Aliases: r.Aliases,
		})
	}
	return out, nil
}

// ===== Helper Methods =====

// buildGameEntry creates a JSON-friendly map for a game with its latest trainer and state.
func (a *AppService) buildGameEntry(g *model.Game) map[string]interface{} {
	// Get the latest trainer (first in the sorted list)
	trainers := a.idx.GetTrainersForGame(g.ID)

	// Determine best display name
	displayName := g.NameLocal
	if displayName == "" {
		displayName = g.NameEN
	}

	entry := map[string]interface{}{
		"id":            g.ID,
		"source_id":     g.SourceID,
		"name_en":       g.NameEN,
		"name_local":    g.NameLocal,
		"display_name":  displayName,
		"cover_url":     g.CoverURL,
		"source_url":    g.SourceURL,
		"options_num":   g.OptionsNum,
		"updated_at":    g.UpdatedAt,
		"trainer_count": len(trainers),
	}

	// Attach latest trainer info
	if len(trainers) > 0 {
		latest := trainers[0]
		entry["latest_trainer"] = map[string]interface{}{
			"id":             latest.ID,
			"version":        latest.Version,
			"game_version":   latest.GameVersion,
			"download_count": latest.DownloadCount,
			"file_size":      latest.FileSize,
		}

		// Attach state of latest trainer
		state := a.idx.GetTrainerState(latest.ID)
		if state != nil {
			entry["status"] = int(state.Status)
			entry["local_path"] = state.LocalPath
		} else {
			entry["status"] = int(model.StatusAvailable)
		}
	} else {
		entry["status"] = int(model.StatusAvailable)
	}

	return entry
}

// buildTrainerWithGameEntry creates a JSON-friendly map for a trainer with its game info.
func (a *AppService) buildTrainerWithGameEntry(t *model.Trainer, g *model.Game, s *model.TrainerState) map[string]interface{} {
	displayName := g.NameLocal
	if displayName == "" {
		displayName = g.NameEN
	}

	return map[string]interface{}{
		"id":             t.ID,
		"game_id":        t.GameID,
		"game_name":      displayName,
		"game_name_en":   g.NameEN,
		"cover_url":      g.CoverURL,
		"version":        t.Version,
		"game_version":   t.GameVersion,
		"download_url":   t.DownloadURL,
		"file_size":      t.FileSize,
		"file_name":      t.FileName,
		"download_count": t.DownloadCount,
		"source_hash":    t.SourceHash,
		"updated_at":     t.UpdatedAt,
		"status":         int(s.Status),
		"local_path":     s.LocalPath,
		"installed_at":   s.InstalledAt,
		"launched_at":    s.LaunchedAt,
	}
}
