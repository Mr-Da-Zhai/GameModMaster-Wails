package scraper

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"GameModMaster/internal/model"
	"GameModMaster/internal/repo"
	"GameModMaster/internal/service"
)

const baseURL = "https://flingtrainer.com"

// Scraper fetches and parses trainer data from flingtrainer.com.
type Scraper struct {
	client         *http.Client
	gameRepo       *repo.GameRepo
	trainerRepo    *repo.TrainerRepo
	mappingService *service.MappingService
}

// NewScraper creates a new Scraper with the given dependencies.
func NewScraper(gameRepo *repo.GameRepo, trainerRepo *repo.TrainerRepo, mappingService *service.MappingService) *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		gameRepo:       gameRepo,
		trainerRepo:    trainerRepo,
		mappingService: mappingService,
	}
}

// FetchPage fetches HTML content from a URL with a User-Agent header.
func (s *Scraper) FetchPage(pageURL string) (string, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return "", fmt.Errorf("create request for %s: %w", pageURL, err)
	}
	req.Header.Set("User-Agent", "GameModMaster/1.0 (Desktop App)")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", pageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: status %d", pageURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body from %s: %w", pageURL, err)
	}

	return string(body), nil
}

// FetchListPage fetches and parses a list page (page 1, 2, etc.).
// It translates game names and returns the games found on that page.
func (s *Scraper) FetchListPage(page int) ([]*model.Game, error) {
	var pageURL string
	if page <= 1 {
		pageURL = baseURL + "/"
	} else {
		pageURL = fmt.Sprintf("%s/page/%d/", baseURL, page)
	}

	html, err := s.FetchPage(pageURL)
	if err != nil {
		return nil, err
	}

	games, err := ParseTrainerList(html)
	if err != nil {
		return nil, fmt.Errorf("parse list page %d: %w", page, err)
	}

	// Translate game names
	for _, g := range games {
		g.NameLocal = s.translateGameName(g.NameEN)
	}

	return games, nil
}

// FetchDetailPage fetches and parses a detail page for a specific game.
// It returns the trainer data extracted from the page.
func (s *Scraper) FetchDetailPage(sourceURL string) (*model.Trainer, error) {
	html, err := s.FetchPage(sourceURL)
	if err != nil {
		return nil, err
	}

	trainer, err := ParseTrainerDetail(html)
	if err != nil {
		return nil, fmt.Errorf("parse detail page %s: %w", sourceURL, err)
	}

	return trainer, nil
}

// SearchTrainers searches flingtrainer.com for trainers matching the query.
// It returns parsed games with translated names.
func (s *Scraper) SearchTrainers(query string) ([]*model.Game, error) {
	searchURL := fmt.Sprintf("%s/?s=%s", baseURL, url.QueryEscape(query))

	html, err := s.FetchPage(searchURL)
	if err != nil {
		return nil, err
	}

	games, err := ParseTrainerList(html)
	if err != nil {
		return nil, fmt.Errorf("parse search results for %q: %w", query, err)
	}

	// Translate game names
	for _, g := range games {
		g.NameLocal = s.translateGameName(g.NameEN)
	}

	return games, nil
}

// FetchAndSave fetches a list page, translates names, and saves to DB.
// It returns the number of new/updated games saved.
func (s *Scraper) FetchAndSave(page int) (int, error) {
	games, err := s.FetchListPage(page)
	if err != nil {
		return 0, err
	}

	if len(games) == 0 {
		return 0, nil
	}

	// Save to DB via BatchUpsert
	if err := s.gameRepo.BatchUpsert(games); err != nil {
		return 0, fmt.Errorf("save games from page %d: %w", page, err)
	}

	return len(games), nil
}

// SearchAndSave searches for trainers, translates names, and saves results to DB.
func (s *Scraper) SearchAndSave(query string) ([]*model.Game, error) {
	games, err := s.SearchTrainers(query)
	if err != nil {
		return nil, err
	}

	if len(games) == 0 {
		return games, nil
	}

	// Save to DB via BatchUpsert
	if err := s.gameRepo.BatchUpsert(games); err != nil {
		return nil, fmt.Errorf("save search results for %q: %w", query, err)
	}

	return games, nil
}

// FetchMultiplePages fetches and saves multiple consecutive list pages
// with a polite delay between requests.
// It returns the total number of games saved across all pages.
func (s *Scraper) FetchMultiplePages(startPage, count int) (int, error) {
	totalSaved := 0

	for i := 0; i < count; i++ {
		page := startPage + i
		saved, err := s.FetchAndSave(page)
		if err != nil {
			// Return what we have so far along with the error
			return totalSaved, fmt.Errorf("page %d: %w", page, err)
		}
		totalSaved += saved

		// Polite delay between requests (skip after the last page)
		if i < count-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return totalSaved, nil
}

// translateGameName translates a game's English name to Chinese using the mapping service.
// If no mapping is found, the original English name is returned.
func (s *Scraper) translateGameName(nameEN string) string {
	if s.mappingService == nil {
		return nameEN
	}
	return s.mappingService.GetChineseName(nameEN)
}

// buildURL constructs a full URL from a potentially relative path.
func buildURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return baseURL + strings.TrimPrefix(path, "/")
}
