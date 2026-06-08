package scraper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"GameModMaster/internal/model"

	"github.com/PuerkitoBio/goquery"
)

// optionsRe matches patterns like "15 Options" or "15 Trainer Options"
var optionsRe = regexp.MustCompile(`(?i)(\d+)\s*(?:trainer\s+)?options?`)

// gameVersionRe matches "Game Version: XXXXX"
var gameVersionRe = regexp.MustCompile(`(?i)Game\s*Version\s*[:：]\s*(.+)`)

// ParseTrainerList parses the HTML from a flingtrainer.com list page
// and returns a slice of Game structs.
func ParseTrainerList(html string) ([]*model.Game, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var games []*model.Game

	doc.Find("article.post-standard").Each(func(i int, s *goquery.Selection) {
		game := &model.Game{}

		// Title and source URL
		titleLink := s.Find("h2.post-title a")
		game.NameEN = strings.TrimSpace(titleLink.Text())
		if href, exists := titleLink.Attr("href"); exists {
			game.SourceURL = strings.TrimSpace(href)
		}
		game.SourceID = ExtractTrainerID(game.SourceURL)

		// Thumbnail / cover URL
		thumbImg := s.Find(".post-details-thumb img")
		if src, exists := thumbImg.Attr("src"); exists {
			game.CoverURL = strings.TrimSpace(src)
		}

		// Options number — look for pattern in the entry text
		entryText := s.Find(".entry").Text()
		game.OptionsNum = parseOptionsNum(entryText)
		// Also try the title if not found in entry
		if game.OptionsNum == 0 {
			game.OptionsNum = parseOptionsNum(game.NameEN)
		}

		// Date: day + month + year
		day := strings.TrimSpace(s.Find(".post-details-day").Text())
		month := strings.TrimSpace(s.Find(".post-details-month").Text())
		year := strings.TrimSpace(s.Find(".post-details-year").Text())
		game.UpdatedAt = parseDateParts(year, month, day)

		games = append(games, game)
	})

	return games, nil
}

// ParseTrainerDetail parses the HTML from a flingtrainer.com detail/trainer page
// and returns a Trainer struct.
func ParseTrainerDetail(html string) (*model.Trainer, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	trainer := &model.Trainer{}

	// Title (for reference, not stored directly)
	// title := strings.TrimSpace(doc.Find("h1.post-title").Text())

	// Full entry text for extracting version info
	entryText := doc.Find(".entry").Text()

	// Version: extract options number string like "15 Options"
	trainer.Version = extractOptionsString(entryText)

	// Game version
	trainer.GameVersion = extractGameVersion(entryText)

	// Download link
	attachLink := doc.Find(".attachment-link")
	if href, exists := attachLink.Attr("href"); exists {
		trainer.DownloadURL = strings.TrimSpace(href)
	}

	// Download count
	downloadsText := doc.Find(".attachment-downloads").Text()
	trainer.DownloadCount = parseDownloadCount(downloadsText)

	// Main image (for potential future use)
	// mainImg := doc.Find(".entry img.aligncenter")
	// if src, exists := mainImg.Attr("src"); exists {
	// 	_ = src
	// }

	// SourceHash: SHA-ish hash of download URL for dedup
	if trainer.DownloadURL != "" {
		trainer.SourceHash = simpleHash(trainer.DownloadURL)
	}

	// UpdatedAt: use current time as fallback; ideally parsed from page
	trainer.UpdatedAt = time.Now().Unix()

	return trainer, nil
}

// ExtractTrainerID extracts the slug from a flingtrainer URL.
// e.g. "https://flingtrainer.com/trainer/elden-ring-trainer/" -> "elden-ring-trainer"
func ExtractTrainerID(sourceURL string) string {
	u := strings.TrimSpace(sourceURL)
	u = strings.TrimSuffix(u, "/")

	// Take the last path segment
	parts := strings.Split(u, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// parseOptionsNum extracts the options count from text like "15 Options"
func parseOptionsNum(text string) int16 {
	matches := optionsRe.FindStringSubmatch(text)
	if len(matches) >= 2 {
		n, err := strconv.Atoi(matches[1])
		if err == nil {
			return int16(n)
		}
	}
	return 0
}

// extractOptionsString returns the full options string from text (e.g. "15 Options")
func extractOptionsString(text string) string {
	matches := optionsRe.FindStringSubmatch(text)
	if len(matches) >= 1 {
		return matches[0]
	}
	return ""
}

// extractGameVersion extracts the game version from text like "Game Version: 1.09+"
func extractGameVersion(text string) string {
	matches := gameVersionRe.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// parseDownloadCount extracts the download count from text like "Downloads: 12345"
func parseDownloadCount(text string) int32 {
	// Remove all non-digit characters except digits
	text = strings.TrimSpace(text)
	// Try to find a number in the text
	re := regexp.MustCompile(`(\d[\d,]*)`)
	matches := re.FindStringSubmatch(text)
	if len(matches) >= 2 {
		numStr := strings.ReplaceAll(matches[1], ",", "")
		n, err := strconv.Atoi(numStr)
		if err == nil {
			return int32(n)
		}
	}
	return 0
}

// parseDateParts converts day/month/year strings to Unix timestamp.
// Returns 0 if parsing fails.
func parseDateParts(year, month, day string) int64 {
	if year == "" || month == "" || day == "" {
		return 0
	}

	y, err := strconv.Atoi(year)
	if err != nil {
		return 0
	}

	d, err := strconv.Atoi(day)
	if err != nil {
		return 0
	}

	// Month can be a name ("Jan", "January") or a number
	var monthNum int
	if mn, err := strconv.Atoi(month); err == nil {
		monthNum = mn
	} else {
		monthNum = parseMonthName(month)
	}

	if monthNum < 1 || monthNum > 12 {
		return 0
	}

	t := time.Date(y, time.Month(monthNum), d, 0, 0, 0, 0, time.UTC)
	return t.Unix()
}

// parseMonthName converts month name (short or long) to number.
func parseMonthName(name string) int {
	names := map[string]int{
		"january":   1, "february": 2, "march":     3, "april":     4,
		"may":       5, "june":     6, "july":      7, "august":    8,
		"september": 9, "october":  10, "november": 11, "december": 12,
		"jan": 1, "feb": 2, "mar": 3, "apr": 4,
		"jun": 6, "jul": 7, "aug": 8, "sep": 9,
		"oct": 10, "nov": 11, "dec": 12,
	}
	if n, ok := names[strings.ToLower(name)]; ok {
		return n
	}
	return 0
}

// simpleHash produces a deterministic hex hash string from input.
// Uses FNV-1a for a quick, dependency-free hash.
func simpleHash(s string) string {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for _, b := range []byte(s) {
		h ^= uint64(b)
		h *= prime64
	}
	return fmt.Sprintf("%016x", h)
}
