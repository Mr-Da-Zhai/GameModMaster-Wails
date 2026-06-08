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
	"time"

	"GameModMaster/internal/index"
	"GameModMaster/internal/model"
	"GameModMaster/internal/repo"
	"GameModMaster/internal/scraper"
	"GameModMaster/internal/service"
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

	return a
}

// Shutdown closes the database connection on app exit.
func (a *AppService) Shutdown() {
	if a.db != nil {
		a.db.Close()
		log.Println("[AppService] Database closed")
	}
}

// resolveDataDir sets the data directory:
// - Beside the executable if writable
// - Otherwise under os.UserConfigDir()
func (a *AppService) resolveDataDir() {
	// Try beside executable first
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		candidate := filepath.Join(exeDir, "data")
		// Check if we can write to the exe directory
		testFile := filepath.Join(exeDir, ".gamm_write_test")
		if f, err := os.Create(testFile); err == nil {
			f.Close()
			os.Remove(testFile)
			a.dataDir = candidate
			return
		}
	}

	// Fallback to user config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	a.dataDir = filepath.Join(configDir, "GameModMaster", "data")
}

// refreshIndex reloads the in-memory index from the database.
func (a *AppService) refreshIndex() {
	if err := a.idx.Refresh(a.gameRepo, a.trainerRepo, a.stateRepo); err != nil {
		log.Printf("[AppService] Failed to refresh index: %v", err)
	}
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
			"id":          g.ID,
			"source_id":   g.SourceID,
			"name_en":     g.NameEN,
			"name_local":  g.NameLocal,
			"display_name": displayName,
			"cover_url":   g.CoverURL,
			"source_url":  g.SourceURL,
			"options_num": g.OptionsNum,
			"updated_at":  g.UpdatedAt,
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

// ===== Action Methods =====

// DownloadTrainer downloads a trainer file.
func (a *AppService) DownloadTrainer(trainerID int32) error {
	t, ok := a.idx.TrainersByID[trainerID]
	if !ok {
		return fmt.Errorf("trainer not found: %d", trainerID)
	}

	if t.DownloadURL == "" {
		return fmt.Errorf("trainer %d has no download URL", trainerID)
	}

	// Determine download directory
	downloadDir := filepath.Join(a.dataDir, "downloads")
	fileName := t.FileName
	if fileName == "" {
		fileName = filepath.Base(t.DownloadURL)
	}

	// Download the file
	ctx := context.Background()
	localPath, err := a.downloadService.Download(ctx, t.DownloadURL, downloadDir, fileName, nil)
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
		trainerDir := filepath.Join(a.dataDir, "downloads", fmt.Sprintf("trainer_%d", trainerID))
		os.RemoveAll(trainerDir)
	}

	// Remove state
	if err := a.downloadService.RemoveState(trainerID); err != nil {
		return fmt.Errorf("remove state failed: %w", err)
	}

	a.refreshIndex()
	return nil
}

// RefreshData fetches latest data from flingtrainer.com and updates DB.
func (a *AppService) RefreshData() error {
	// Fetch first 3 pages of new data
	totalSaved, err := a.scraperService.FetchMultiplePages(1, 3)
	if err != nil {
		log.Printf("[AppService] Refresh partial failure: %v", err)
		// Still refresh what we got
	}

	// Rebuild index with new data
	a.refreshIndex()

	if err != nil {
		return fmt.Errorf("refresh completed with errors (saved %d games): %w", totalSaved, err)
	}

	log.Printf("[AppService] Refresh complete: %d games updated", totalSaved)
	return nil
}

// ===== Settings Methods =====

// GetSettings returns app settings from kv_store.
func (a *AppService) GetSettings() map[string]interface{} {
	settings := map[string]interface{}{
		"data_dir": a.dataDir,
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

	return settings
}

// SaveSettings saves settings to kv_store.
func (a *AppService) SaveSettings(settings map[string]interface{}) error {
	now := time.Now().Unix()

	for key, val := range settings {
		// Skip internal keys
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

		_, err := a.db.Exec(
			"INSERT OR REPLACE INTO kv_store (key, value, updated_at) VALUES (?, ?, ?)",
			key, valueStr, now,
		)
		if err != nil {
			return fmt.Errorf("save setting %q: %w", key, err)
		}
	}

	return nil
}

// GetDataDir returns the data directory path.
func (a *AppService) GetDataDir() string {
	return a.dataDir
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
		"id":           g.ID,
		"source_id":    g.SourceID,
		"name_en":      g.NameEN,
		"name_local":   g.NameLocal,
		"display_name": displayName,
		"cover_url":    g.CoverURL,
		"source_url":   g.SourceURL,
		"options_num":  g.OptionsNum,
		"updated_at":   g.UpdatedAt,
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
		"id":            t.ID,
		"game_id":       t.GameID,
		"game_name":     displayName,
		"game_name_en":  g.NameEN,
		"cover_url":     g.CoverURL,
		"version":       t.Version,
		"game_version":  t.GameVersion,
		"download_url":  t.DownloadURL,
		"file_size":     t.FileSize,
		"file_name":     t.FileName,
		"download_count": t.DownloadCount,
		"source_hash":   t.SourceHash,
		"updated_at":    t.UpdatedAt,
		"status":        int(s.Status),
		"local_path":    s.LocalPath,
		"installed_at":  s.InstalledAt,
		"launched_at":   s.LaunchedAt,
	}
}
