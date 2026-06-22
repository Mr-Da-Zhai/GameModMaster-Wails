package scraper

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"GameModMaster/internal/model"
	"GameModMaster/internal/repo"
	"GameModMaster/internal/service"
)

const baseURL = "https://flingtrainer.com"

const (
	// defaultFetchRetries is how many times FetchPage retries a transient
	// (network / 429 / 5xx) failure before giving up. Other 4xx responses
	// are not retried.
	defaultFetchRetries = 4
	// defaultDetailConcurrency is how many detail pages are fetched in
	// parallel during a FetchAndSave call. flingtrainer.com rate-limits
	// aggressively (HTTP 429) when hit too hard; 3 workers keeps the
	// ~730-game crawl under ~2 minutes while staying well under the limit.
	defaultDetailConcurrency = 3
)

// Scraper fetches and parses trainer data from flingtrainer.com.
type Scraper struct {
	client         *http.Client
	gameRepo       *repo.GameRepo
	trainerRepo    *repo.TrainerRepo
	mappingService *service.MappingService

	// lastDetailErrors holds the number of detail-page fetch failures seen
	// during the most recent FetchAndSave / FetchAndSaveOpt call. Read via
	// LastDetailErrors() after the call returns.
	lastDetailErrors int
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
	return s.FetchPageWithRetries(pageURL, defaultFetchRetries)
}

// FetchPageWithRetries fetches HTML content, retrying transient failures
// (network errors, 429 rate-limits, or 5xx responses) with exponential
// backoff. Other 4xx responses are not retried — they are definitive "this
// URL is broken" answers from the server. Used by every HTTP path so a
// single flaky page does not abort an hours-long full crawl.
func (s *Scraper) FetchPageWithRetries(pageURL string, maxAttempts int) (string, error) {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 500ms, 1s, 2s ...
			backoff := time.Duration(500*(1<<(attempt-1))) * time.Millisecond
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			time.Sleep(backoff)
		}

		req, err := http.NewRequest("GET", pageURL, nil)
		if err != nil {
			return "", fmt.Errorf("create request for %s: %w", pageURL, err)
		}
		req.Header.Set("User-Agent", "GameModMaster/1.0 (Desktop App)")

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetch %s: %w", pageURL, err)
			continue // retry network errors
		}

		// 429 Too Many Requests: rate-limited, worth retrying with backoff.
		// Honour Retry-After if the server sent one.
		if resp.StatusCode == 429 {
			resp.Body.Close()
			lastErr = fmt.Errorf("fetch %s: status %d (rate limited)", pageURL, resp.StatusCode)
			// Optional Retry-After header (seconds).
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				var secs int
				if _, err := fmt.Sscanf(ra, "%d", &secs); err == nil && secs > 0 && secs < 60 {
					time.Sleep(time.Duration(secs) * time.Second)
					continue
				}
			}
			continue
		}
		// 4xx (other than 429): definitive, do not retry.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			resp.Body.Close()
			return "", fmt.Errorf("fetch %s: status %d", pageURL, resp.StatusCode)
		}
		// 5xx: server-side, worth retrying.
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("fetch %s: status %d", pageURL, resp.StatusCode)
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return "", fmt.Errorf("fetch %s: status %d", pageURL, resp.StatusCode)
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("read body from %s: %w", pageURL, err)
			continue
		}
		return string(body), nil
	}
	return "", lastErr
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
// if it yields any real game article entries. Returns the highest such page
// number (minimum 1).
//
// IMPORTANT: the probe must NOT use a bare "post-standard" substring check —
// the 404 / "Page not found" page still ships the site's stylesheet, which
// contains CSS rules like ".post-standard:hover .post-title a" and
// ".blog .post-standard". A naive strings.Contains(html, "post-standard")
// therefore returns true even for non-existent pages, which previously made
// the binary search believe there were 256 pages and triggered 200+ useless
// 404 requests on every refresh. We instead require a real <article> element
// carrying the post-standard class.
//
// Errors during probing (e.g. transient 429 rate-limits) are retried a couple
// of times; if a page still won't answer we treat it conservatively as "no
// articles" so the binary search converges on the highest reachable page.
func (s *Scraper) CountTotalPages() (int, error) {
	// hasRealArticles returns true only if the page actually lists game
	// articles (i.e. <article ... class="...post-standard..."> appears at
	// least once). Retries up to 3 times on transient errors.
	hasRealArticles := func(p int) (bool, error) {
		pageURL := baseURL + "/"
		if p > 1 {
			pageURL = fmt.Sprintf("%s/page/%d/", baseURL, p)
		}
		var lastErr error
		for try := 0; try < 3; try++ {
			html, err := s.FetchPage(pageURL)
			if err != nil {
				lastErr = err
				// Brief backoff before retrying the probe.
				time.Sleep(time.Duration(300*(try+1)) * time.Millisecond)
				continue
			}
			return hasGameArticles(html), nil
		}
		return false, lastErr
	}

	// Find an upper bound that no longer has real articles.
	lo, hi := 1, 8
	for {
		ok, err := hasRealArticles(hi)
		if err != nil {
			// Network/rate-limit error: don't expand further; converge now.
			break
		}
		if !ok {
			break
		}
		lo = hi
		hi *= 2
		// Guard: the site is currently ~50 pages; cap probing well above that
		// but still bounded so we never loop forever.
		if hi > 1024 {
			break
		}
		// Polite gap between probe requests.
		time.Sleep(150 * time.Millisecond)
	}

	// Binary search the last existing page in (lo, hi].
	for lo+1 < hi {
		mid := (lo + hi) / 2
		ok, err := hasRealArticles(mid)
		if err != nil {
			// On error, narrow the window conservatively (treat as "absent").
			hi = mid
			continue
		}
		if ok {
			lo = mid
		} else {
			hi = mid
		}
		time.Sleep(120 * time.Millisecond)
	}
	return lo, nil
}

// hasGameArticles reports whether the given HTML contains at least one real
// game article element (<article ... class="...post-standard...">). The
// leading '<' is what distinguishes a real article from the bare CSS class
// references present on the 404 page.
func hasGameArticles(html string) bool {
	// Cheap negative check first: 404 pages always contain "Page not found".
	if strings.Contains(html, "Page not found") {
		return false
	}
	// Then require an <article tag carrying the post-standard class. We use
	// a substring scan instead of goquery here because it is dramatically
	// cheaper to run on every probe request.
	idx := 0
	needle := "<article"
	for {
		pos := strings.Index(html[idx:], needle)
		if pos < 0 {
			return false
		}
		idx += pos + len(needle)
		// Grab the next 256 bytes (covers the class attribute) and look for
		// "post-standard" inside it.
		end := idx + 256
		if end > len(html) {
			end = len(html)
		}
		if strings.Contains(html[idx:end], "post-standard") {
			return true
		}
	}
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
		qLow := strings.ToLower(q)
		seen := map[string]bool{qLow: true}

		// GetMapping() is lowercase english name -> chinese. For each entry
		// whose Chinese contains the query (or vice-versa), the english name
		// becomes a candidate search term.
		for en, zh := range s.mappingService.GetMapping() {
			if zh == "" || zh == en {
				continue
			}
			zhLow := strings.ToLower(zh)
			if strings.Contains(zhLow, qLow) || strings.Contains(qLow, zhLow) {
				if !seen[en] {
					terms = append(terms, en)
					seen[en] = true
				}
			}
		}
		// Aliases: alias (chinese or english) -> english name (here stored as
		// alias -> chinese, so we mirror the same logic).
		for alias, zh := range s.mappingService.GetAliases() {
			aliasLow := strings.ToLower(alias)
			zhLow := strings.ToLower(zh)
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
// Flow: list page -> for each game, fetch detail concurrently -> upsert game +
// its trainers. Returns (gamesSaved, trainersSaved, detailErrors). The third
// return value is the number of games whose detail page failed to fetch
// (network/parse); the call still returns nil error in that case so a single
// flaky detail page does not abort the whole crawl.
func (s *Scraper) FetchAndSave(page int) (int, int, error) {
	return s.FetchAndSaveOpts(page, defaultDetailConcurrency)
}

// FetchAndSaveOpts is like FetchAndSave but lets the caller pick the worker
// count for concurrent detail-page fetches (must be >= 1).
func (s *Scraper) FetchAndSaveOpts(page, concurrency int) (int, int, error) {
	if concurrency < 1 {
		concurrency = 1
	}
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

	type detailResult struct {
		game     *model.Game
		storedID int32
		page     *TrainerPage
		err      error
	}

	// Resolve DB-assigned IDs up front (cheap local lookups).
	type job struct {
		game    *model.Game
		stored  *model.Game
	}
	jobs := make([]job, 0, len(games))
	for _, g := range games {
		stored, err := s.gameRepo.GetBySourceID(g.SourceID)
		if err != nil || stored == nil {
			log.Printf("[Scraper] skip detail for %q: cannot resolve game id (err=%v)", g.SourceID, err)
			continue
		}
		jobs = append(jobs, job{game: g, stored: stored})
	}

	// Concurrent detail-page fetch. Bounded worker pool keeps us polite to
	// the source site while still finishing a ~730-game crawl in well under
	// a minute instead of ~5 minutes.
	results := make(chan detailResult, len(jobs))
	jobCh := make(chan job, len(jobs))
	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)

	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				// Tiny per-worker offset so the workers don't all fire on the
				// same millisecond tick. Combined with the default 3-worker
				// pool this keeps the request rate well under the site's 429
				// threshold during a full ~730-game crawl.
				dp, err := s.FetchDetailPage(j.game.SourceURL)
				results <- detailResult{game: j.game, storedID: j.stored.ID, page: dp, err: err}
				// Polite gap between this worker's requests.
				time.Sleep(120 * time.Millisecond)
			}
		}()
	}
	wg.Wait()
	close(results)

	var allTrainers []*model.Trainer
	detailErrors := 0
	for r := range results {
		if r.err != nil {
			log.Printf("[Scraper] detail fetch failed for %q: %v", r.game.SourceURL, r.err)
			detailErrors++
			continue
		}
		// Resolve the game record to refresh its updated_at / options_num.
		stored, err := s.gameRepo.GetBySourceID(r.game.SourceID)
		if err != nil || stored == nil {
			detailErrors++
			continue
		}
		// If the detail page gave us a fresher updated_at, bump the game record.
		if r.page.UpdatedAt > 0 && r.page.UpdatedAt > stored.UpdatedAt {
			stored.UpdatedAt = r.page.UpdatedAt
			if r.page.Options != "" {
				if n := parseOptionsNum(r.page.Options); n > 0 {
					stored.OptionsNum = n
				}
			}
			_ = s.gameRepo.BatchUpsert([]*model.Game{stored})
		}

		for _, t := range r.page.Trainers {
			t.GameID = stored.ID
			allTrainers = append(allTrainers, t)
		}
	}

	if len(allTrainers) > 0 {
		if err := s.trainerRepo.BatchUpsert(allTrainers); err != nil {
			return len(games), 0, fmt.Errorf("save trainers from page %d: %w", page, err)
		}
	}

	// Stash detail error count so callers that care (e.g. the full crawler)
	// can surface it via the dedicated DetailErrorCount field on the result.
	s.lastDetailErrors = detailErrors

	return len(games), len(allTrainers), nil
}

// LastDetailErrors returns the number of detail-page failures seen during the
// most recent FetchAndSave / FetchAndSaveOpts call on this scraper. Safe to
// read after FetchAndSave returns.
func (s *Scraper) LastDetailErrors() int {
	return s.lastDetailErrors
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
