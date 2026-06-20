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
	EventRefreshProgress = "refresh:progress"
	EventDownloadProgress = "download:progress"
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
}

// NewAppService creates and initializes the AppService.
// embeddedMapping is the name_mapping.json data embedded in the binary.
func NewAppService(embeddedMapping []byte) *AppService {
	a := &AppService{}

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

// SearchTrainers searches by query (Chinese or English).
func (a *AppService) SearchTrainers(query string) ([]map[string]interface{}, error) {
	if query == "" {
		return a.GetTrainers(1, 50)
	}

	games := a.idx.SearchGames(query, 50)
	results := make([]map[string]interface{}, 0, len(games))

	for _, g := range games {
		entry := a.buildGameEntry(g)
		results = append(results, entry)
	}

	return results, nil
}

// GetTrainerDetail returns detail for a specific game (all trainer versions).
func (a *AppService) GetTrainerDetail(gameID int32) (map[string]interface{}, error) {
	g, ok := a.idx.GamesByID[gameID]
	if !ok {
		return nil, fmt.Errorf("game not found: %d", gameID)
	}

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

	result := map[string]interface{}{
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

	return result, nil
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

	ctx := a.requestContext()
	localPath, err := a.downloadService.Download(ctx, t.DownloadURL, downloadDir, fileName, progress)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// If it's a zip file, extract it
	if strings.HasSuffix(strings.ToLower(localPath), ".zip") {
		extractDir := filepath.Join(downloadDir, fmt.Sprintf("trainer_%d", trainerID))
		extracted, err := a.downloadService.ExtractZIP(localPath, extractDir)
		if err != nil {
			return fmt.Errorf("extract failed: %w", err)
		}

		// Use the first extracted file as the local path
		if len(extracted) > 0 {
			localPath = extracted[0]
		}
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
func (a *AppService) RefreshData() error {
	a.refreshMu.Lock()
	if a.refreshing {
		a.refreshMu.Unlock()
		return fmt.Errorf("refresh already in progress")
	}
	a.refreshing = true
	a.refreshResult = ""
	a.refreshMu.Unlock()

	go a.runRefresh()
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
	a.refreshMu.Unlock()
	defer func() {
		a.refreshMu.Lock()
		a.refreshing = false
		a.refreshMu.Unlock()
	}()

	// pageCount <= 0 means "fetch everything".
	return a.doFetch(1, 0)
}

// runRefresh executes the fetch off the main goroutine.
func (a *AppService) runRefresh() {
	defer func() {
		a.refreshMu.Lock()
		a.refreshing = false
		a.refreshMu.Unlock()
	}()

	// Always fetch the full library so search covers every game.
	summary, err := a.doFetch(1, 0)
	if err != nil {
		log.Printf("[AppService] Refresh error: %v", err)
		summary = fmt.Sprintf("%s (部分出错: %v)", summary, err)
	}
	a.refreshMu.Lock()
	a.refreshResult = summary
	a.refreshMu.Unlock()

	// Notify the frontend that the refresh finished.
	a.emitEvent(EventRefreshProgress, map[string]interface{}{
		"done":    true,
		"summary": summary,
	})
}

// doFetch performs the multi-page crawl with progress events.
// pageCount <= 0 means "probe and fetch all pages".
func (a *AppService) doFetch(startPage, pageCount int) (string, error) {
	total := pageCount
	if total <= 0 {
		// Probe the site for the real last page once, up front.
		probed, err := a.scraperService.CountTotalPages()
		if err != nil {
			log.Printf("[AppService] count pages failed: %v", err)
			probed = 49 // sensible fallback
		}
		total = probed - startPage + 1
		if total < 1 {
			total = 1
		}
		log.Printf("[AppService] full crawl: %d pages", total)
		// Tell the UI how many pages to expect.
		a.emitEvent(EventRefreshProgress, map[string]interface{}{
			"total":   total,
			"current": 0,
			"phase":   "probe",
		})
	}

	totalGames := 0
	totalTrainers := 0
	var firstErr error

	for i := 0; i < total; i++ {
		page := startPage + i
		games, trainers, err := a.scraperService.FetchAndSave(page)
		totalGames += games
		totalTrainers += trainers
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Printf("[AppService] page %d: %v", page, err)
		}

		// Emit progress on every page so the UI bar advances smoothly.
		a.emitEvent(EventRefreshProgress, map[string]interface{}{
			"page":     page,
			"total":    total,
			"current":  i + 1,
			"games":    totalGames,
			"trainers": totalTrainers,
		})

		// Refresh the in-memory index periodically (every 5 pages) so the home
		// grid populates incrementally instead of staying empty for minutes.
		if (i+1)%5 == 0 {
			a.refreshIndex()
		}
	}

	// Final rebuild with all new data
	a.refreshIndex()

	summary := fmt.Sprintf("已更新 %d 个游戏, %d 个修改器", totalGames, totalTrainers)
	return summary, firstErr
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
