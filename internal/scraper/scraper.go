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

// CountTotalPages probes flingtrainer.com to determine the last list page.
// It does a binary search over page numbers: a page counts as "existing" only
// if it yields any real <article class="post-standard"> game entries. Returns
// the highest such page number (minimum 1).
func (s *Scraper) CountTotalPages() (int, error) {
	// Fast probe: try a high upper bound first to anchor the search.
	hasPage := func(p int) (bool, error) {
		url := baseURL + "/"
		if p > 1 {
			url = fmt.Sprintf("%s/page/%d/", baseURL, p)
		}
		html, err := s.FetchPage(url)
		if err != nil {
			return false, err
		}
		return strings.Contains(html, "post-standard"), nil
	}

	// Find an upper bound that no longer exists.
	lo, hi := 1, 8
	for {
		ok, err := hasPage(hi)
		if err != nil {
			return lo, nil // be tolerant: return what we know
		}
		if !ok {
			break
		}
		lo = hi
		hi *= 2
		if hi > 256 {
			break
		}
	}

	// Binary search the last existing page in (lo, hi].
	for lo+1 < hi {
		mid := (lo + hi) / 2
		ok, err := hasPage(mid)
		if err != nil {
			break
		}
		if ok {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo, nil
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

// SearchRemote performs a live search against flingtrainer.com, resolving a
// Chinese query to its English title first (FLiNG's site search is English
// only). Each result game has its Chinese display name attached and is cached
// to the local DB so subsequent detail lookups resolve quickly.
//
// Resolution order for a Chinese query:
//  1. Look the query up in the name mapping (zh -> en / alias -> en).
//  2. If several English candidates exist, query the site for each and merge.
//  3. If nothing maps, fall back to the raw query (may still hit English).
func (s *Scraper) SearchRemote(query string) ([]*model.Game, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	// Collect English search terms to try. Always include the raw query so an
	// English input still works directly.
	terms := []string{q}

	// Expand a (possibly Chinese) query into English names via the mapping.
	if s.mappingService != nil {
		seen := map[string]bool{strings.ToLower(q): true}
		for en := range s.mappingService.GetMapping() {
			// en here is the lowercased english key
			zh, _ := s.mappingService.Lookup(en)
			zhLow := strings.ToLower(zh)
			qLow := strings.ToLower(q)
			if zh != "" && zh != en && (strings.Contains(zhLow, qLow) || strings.Contains(qLow, zhLow)) {
				if !seen[strings.ToLower(en)] {
					terms = append(terms, en)
					seen[strings.ToLower(en)] = true
				}
			}
		}
		// Also check aliases (alias -> zh)
		for alias, zh := range s.mappingService.GetAliases() {
			aliasLow := strings.ToLower(alias)
			zhLow := strings.ToLower(zh)
			qLow := strings.ToLower(q)
			if strings.Contains(aliasLow, qLow) || strings.Contains(zhLow, qLow) {
				if !seen[aliasLow] {
					terms = append(terms, alias)
					seen[aliasLow] = true
				}
			}
		}
	}

	// Cap the number of remote requests we make per search.
	if len(terms) > 5 {
		terms = terms[:5]
	}

	seenGames := map[string]bool{}
	var merged []*model.Game

	for _, term := range terms {
		games, err := s.SearchTrainers(term)
		if err != nil {
			// One failed term shouldn't abort the whole search.
			continue
		}
		for _, g := range games {
			if g.SourceID == "" || seenGames[g.SourceID] {
				continue
			}
			seenGames[g.SourceID] = true
			merged = append(merged, g)
		}
	}

	// Cache the freshly-found games so detail/download flows work without a
	// re-fetch. Errors here are non-fatal.
	if len(merged) > 0 {
		_ = s.gameRepo.BatchUpsert(merged)
	}

	return merged, nil
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

		// Polite delay between detail requests (kept short so a full
		// ~700-game crawl finishes in a reasonable time).
		time.Sleep(150 * time.Millisecond)
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
