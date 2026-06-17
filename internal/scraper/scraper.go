package scraper

import (
	"fmt"
	"io"
	"log"
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
// It returns the parsed trainer versions (download table) plus page metadata.
func (s *Scraper) FetchDetailPage(sourceURL string) (*TrainerPage, error) {
	html, err := s.FetchPage(sourceURL)
	if err != nil {
		return nil, err
	}

	page, err := ParseTrainerDetail(html)
	if err != nil {
		return nil, fmt.Errorf("parse detail page %s: %w", sourceURL, err)
	}

	return page, nil
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

// FetchAndSave fetches a list page, then crawls each game's detail page to
// collect trainer versions, and persists everything to the DB.
//
// Flow: list page -> for each game, fetch detail -> upsert game + its trainers.
// Returns (gamesSaved, trainersSaved).
func (s *Scraper) FetchAndSave(page int) (int, int, error) {
	games, err := s.FetchListPage(page)
	if err != nil {
		return 0, 0, err
	}

	if len(games) == 0 {
		return 0, 0, nil
	}

	// Persist games first (so we have their IDs via source_id upsert).
	if err := s.gameRepo.BatchUpsert(games); err != nil {
		return 0, 0, fmt.Errorf("save games from page %d: %w", page, err)
	}

	// Now fetch detail pages and collect trainers, attaching game IDs.
	var allTrainers []*model.Trainer
	for _, g := range games {
		// Resolve the DB-assigned ID for this game (by source_id).
		stored, err := s.gameRepo.GetBySourceID(g.SourceID)
		if err != nil || stored == nil {
			log.Printf("[Scraper] skip detail for %q: cannot resolve game id (err=%v)", g.SourceID, err)
			continue
		}

		page, err := s.FetchDetailPage(g.SourceURL)
		if err != nil {
			log.Printf("[Scraper] detail fetch failed for %q: %v", g.SourceURL, err)
			continue
		}

		// If the detail page gave us a fresher updated_at, bump the game record.
		if page.UpdatedAt > 0 && page.UpdatedAt > stored.UpdatedAt {
			stored.UpdatedAt = page.UpdatedAt
			if page.Options != "" {
				if n := parseOptionsNum(page.Options); n > 0 {
					stored.OptionsNum = n
				}
			}
			_ = s.gameRepo.BatchUpsert([]*model.Game{stored})
		}

		for _, t := range page.Trainers {
			t.GameID = stored.ID
			allTrainers = append(allTrainers, t)
		}

		// Polite delay between detail requests.
		time.Sleep(500 * time.Millisecond)
	}

	if len(allTrainers) > 0 {
		if err := s.trainerRepo.BatchUpsert(allTrainers); err != nil {
			return len(games), 0, fmt.Errorf("save trainers from page %d: %w", page, err)
		}
	}

	return len(games), len(allTrainers), nil
}

// SearchAndSave searches for trainers, then crawls detail pages for any games
// not already in the DB, and persists results.
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
// It returns the total number of games and trainers saved across all pages.
func (s *Scraper) FetchMultiplePages(startPage, count int) (int, int, error) {
	totalGames := 0
	totalTrainers := 0

	for i := 0; i < count; i++ {
		page := startPage + i
		games, trainers, err := s.FetchAndSave(page)
		if err != nil {
			// Return what we have so far along with the error
			return totalGames, totalTrainers, fmt.Errorf("page %d: %w", page, err)
		}
		totalGames += games
		totalTrainers += trainers

		// Polite delay between list pages (skip after the last page)
		if i < count-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return totalGames, totalTrainers, nil
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
