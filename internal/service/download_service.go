package service

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"GameModMaster/internal/model"
	"GameModMaster/internal/repo"
)

type ProgressCallback func(downloaded int64, total int64, speed float64)

type DownloadService struct {
	stateRepo *repo.StateRepo
	client    *http.Client
}

func NewDownloadService(stateRepo *repo.StateRepo) *DownloadService {
	return &DownloadService{
		stateRepo: stateRepo,
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

// Download downloads a trainer file to the specified directory
// Returns the local file path on success
// Progress is reported via the callback
func (s *DownloadService) Download(ctx context.Context, url string, destDir string, fileName string, progress ProgressCallback) (string, error) {
	// Create request with context for cancellation
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "GameModMaster/3.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	// Ensure dest directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	// Determine filename
	if fileName == "" {
		fileName = filepath.Base(url)
	}
	destPath := filepath.Join(destDir, fileName)

	// Create file
	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	// Download with progress
	total := resp.ContentLength
	var downloaded int64
	startTime := time.Now()
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		select {
		case <-ctx.Done():
			f.Close()
			os.Remove(destPath) // Clean up partial download
			return "", ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return "", werr
			}
			downloaded += int64(n)

			if progress != nil && total > 0 {
				elapsed := time.Since(startTime).Seconds()
				speed := float64(downloaded) / elapsed // bytes/sec
				progress(downloaded, total, speed)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return destPath, nil
}

// ExtractZIP extracts a ZIP file to the specified directory
func (s *DownloadService) ExtractZIP(zipPath string, destDir string) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var extracted []string
	for _, f := range r.File {
		// Skip directories and __MACOSX
		if f.FileInfo().IsDir() || strings.HasPrefix(f.Name, "__MACOSX") {
			continue
		}

		destPath := filepath.Join(destDir, filepath.Base(f.Name))
		extracted = append(extracted, destPath)

		if err := s.extractFile(f, destPath); err != nil {
			return nil, err
		}
	}
	return extracted, nil
}

func (s *DownloadService) extractFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	w, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer w.Close()

	_, err = io.Copy(w, rc)
	return err
}

// MarkDownloaded updates trainer state to downloaded
func (s *DownloadService) MarkDownloaded(trainerID int32, localPath string) error {
	state := &model.TrainerState{
		TrainerID:   trainerID,
		Status:      model.StatusDownloaded,
		LocalPath:   localPath,
		InstalledAt: 0,
		LaunchedAt:  0,
	}
	return s.stateRepo.Upsert(state)
}

// MarkInstalled updates trainer state to installed
func (s *DownloadService) MarkInstalled(trainerID int32, localPath string) error {
	state := &model.TrainerState{
		TrainerID:   trainerID,
		Status:      model.StatusInstalled,
		LocalPath:   localPath,
		InstalledAt: time.Now().Unix(),
		LaunchedAt:  0,
	}
	return s.stateRepo.Upsert(state)
}

// MarkLaunched updates the last launch time
func (s *DownloadService) MarkLaunched(trainerID int32) error {
	state, err := s.stateRepo.GetByTrainerID(trainerID)
	if err != nil {
		return err
	}
	if state == nil {
		return fmt.Errorf("trainer %d not found in state table", trainerID)
	}
	state.LaunchedAt = time.Now().Unix()
	return s.stateRepo.Upsert(state)
}

// RemoveState removes the trainer state record
func (s *DownloadService) RemoveState(trainerID int32) error {
	// Delete by upserting with status Available and empty paths
	state := &model.TrainerState{
		TrainerID: trainerID,
		Status:    model.StatusAvailable,
	}
	return s.stateRepo.Upsert(state)
}
