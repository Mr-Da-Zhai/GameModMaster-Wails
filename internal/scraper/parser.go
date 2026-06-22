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

// gameVersionRe matches "Game Version: XXXXX" (also "Game Version XXXXX" without colon)
var gameVersionRe = regexp.MustCompile(`(?is)Game\s*Version\s*[:：]?\s*(.+?)(?:·| Last Updated|$|\n)`)

// lastUpdatedRe matches "Last Updated: 2026.06.15"
var lastUpdatedRe = regexp.MustCompile(`(?i)Last\s*Updated\s*[:：]\s*([0-9./\-]+)`)

// metaRe matches the leading summary line "27 Options · Game Version: Early Access+ · Last Updated: 2026.06.15"
var summaryRe = regexp.MustCompile(`(?is)(\d+)\s*(?:trainer\s+)?options?\s*[·•・]\s*Game\s*Version\s*[:：]?\s*(.+?)\s*[·•・]\s*Last\s*Updated`)

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

		// Thumbnail / cover URL.
		// The .post-details-thumb wrapper is often an empty img with no src;
		// the real cover lives in img.wp-post-image (attachment-stylizer-small).
		coverURL := ""
		if thumbImg := s.Find(".post-details-thumb img"); thumbImg.Length() > 0 {
			if src, exists := thumbImg.Attr("src"); exists && src != "" {
				coverURL = strings.TrimSpace(src)
			}
		}
		if coverURL == "" {
			if img := s.Find("img.wp-post-image").First(); img.Length() > 0 {
				if src, exists := img.Attr("src"); exists && src != "" {
					coverURL = strings.TrimSpace(src)
				}
			}
		}
		game.CoverURL = coverURL

		// Options number — look in entry text then title
		entryText := s.Find(".entry").Text()
		game.OptionsNum = parseOptionsNum(entryText)
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

// TrainerPage holds the result of parsing a detail page: the shared game meta
// plus one Trainer per downloadable version found in the versions table.
type TrainerPage struct {
	GameVersion string           // e.g. "Early Access+" — shared across versions
	Options     string           // e.g. "27 Options"
	UpdatedAt   int64            // best-effort page-level timestamp (Last Updated)
	Trainers    []*model.Trainer // one entry per version row
}

// ParseTrainerDetail parses the HTML from a flingtrainer.com detail/trainer page
// and returns all trainer versions found on the page.
//
// FLiNG detail pages list multiple versions in a `.download-attachments` table.
// Each row is one version: [icon, file name + download link, date, size, downloads].
// Game-level metadata (options count, game version) lives in the leading summary
// line of the `.entry` block.
func ParseTrainerDetail(html string) (*TrainerPage, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	page := &TrainerPage{}

	// Parse the summary line, e.g.:
	//   "27 Options · Game Version: Early Access+ · Last Updated: 2026.06.15"
	summaryText := strings.TrimSpace(doc.Find(".entry > p").First().Text())

	if m := summaryRe.FindStringSubmatch(summaryText); len(m) >= 3 {
		page.Options = m[1] + " Options"
		page.GameVersion = strings.TrimSpace(m[2])
	} else {
		// Fallbacks when the summary line shape differs
		if opts := extractOptionsString(summaryText); opts != "" {
			page.Options = opts
		}
		page.GameVersion = extractGameVersion(summaryText)
	}

	// Page-level updated time from "Last Updated: YYYY.MM.DD"
	if m := lastUpdatedRe.FindStringSubmatch(summaryText); len(m) >= 2 {
		page.UpdatedAt = parseDotDate(m[1])
	}
	if page.UpdatedAt == 0 {
		// Fall back to the most recent version-row date parsed below
	}

	// Parse the versions table. Each <tr> after the header is one version.
	var latestRowTime int64
	doc.Find(".download-attachments tr").Each(func(i int, tr *goquery.Selection) {
		// Skip header row (contains "File" / "Date added")
		firstCellText := strings.TrimSpace(tr.Find("td").First().Text()) + " " + strings.TrimSpace(tr.Find("th").First().Text())
		if i == 0 && (strings.Contains(strings.ToLower(firstCellText), "file") || tr.Find("td").Length() == 0) {
			return
		}

		cells := tr.Find("td")
		if cells.Length() < 2 {
			return
		}

		trainer := &model.Trainer{}

		// Find the download anchor anywhere in the row (filename cell)
		var href, fileName string
		tr.Find("a").Each(func(_ int, a *goquery.Selection) {
			if href != "" {
				return
			}
			if h, ok := a.Attr("href"); ok && strings.Contains(h, "/downloads/") {
				href = strings.TrimSpace(h)
				fileName = strings.TrimSpace(a.Text())
			}
		})
		trainer.DownloadURL = href
		trainer.FileName = fileName
		// Prefer a real version string parsed from the filename (e.g.
		// "v1.12"); only fall back to the page-level "N Options" string
		// if the filename yielded nothing useful.
		if v := parseVersionFromFileName(fileName); v != "" {
			trainer.Version = v
		} else {
			trainer.Version = page.Options
		}

		// Map cells by position. The row layout is:
		//   [0] icon, [1] filename/link, [2] date, [3] size, [4] downloads
		// but the filename sometimes occupies cell [0]. Be defensive: collect
		// the non-link text cells and assign size/downloads by heuristics.
		var textCells []string
		cells.Each(func(_ int, td *goquery.Selection) {
			// skip the cell that contains the link (already captured)
			if td.Find("a[href*='/downloads/']").Length() > 0 {
				return
			}
			// skip icon-only cells
			t := strings.TrimSpace(td.Text())
			if t == "" && td.Find("img").Length() > 0 {
				return
			}
			if t != "" {
				textCells = append(textCells, t)
			}
		})

		for _, tc := range textCells {
			switch {
			case trainer.FileSize == 0 && looksLikeFileSize(tc):
				trainer.FileSize = parseFileSize(tc)
			case trainer.DownloadCount == 0 && looksLikeInt(tc):
				if n, err := strconv.Atoi(strings.ReplaceAll(tc, ",", "")); err == nil {
					trainer.DownloadCount = int32(n)
				}
			case trainer.UpdatedAt == 0 && looksLikeDate(tc):
				if ts := parseRowDate(tc); ts > 0 {
					trainer.UpdatedAt = ts
					if ts > latestRowTime {
						latestRowTime = ts
					}
				}
			}
		}

		// SourceHash: dedup key derived from the download URL token
		if trainer.DownloadURL != "" {
			trainer.SourceHash = simpleHash(trainer.DownloadURL)
		} else {
			// No usable download link — skip this row
			return
		}

		// Game version fallback to page-level version
		trainer.GameVersion = page.GameVersion

		// If we still don't have an updated_at, use the page-level one
		if trainer.UpdatedAt == 0 {
			trainer.UpdatedAt = page.UpdatedAt
		}

		page.Trainers = append(page.Trainers, trainer)
	})

	if page.UpdatedAt == 0 {
		page.UpdatedAt = latestRowTime
	}
	if page.UpdatedAt == 0 {
		page.UpdatedAt = time.Now().Unix()
	}

	return page, nil
}

// ExtractTrainerID extracts the slug from a flingtrainer URL.
// e.g. "https://flingtrainer.com/trainer/elden-ring-trainer/" -> "elden-ring-trainer"
func ExtractTrainerID(sourceURL string) string {
	u := strings.TrimSpace(sourceURL)
	u = strings.TrimSuffix(u, "/")

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

// parseDotDate parses "2026.06.15" into a Unix timestamp.
func parseDotDate(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", ".")
	s = strings.ReplaceAll(s, "/", ".")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0
	}
	y, err1 := strconv.Atoi(parts[0])
	m, err2 := strconv.Atoi(parts[1])
	d, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return 0
	}
	if m < 1 || m > 12 {
		return 0
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC).Unix()
}

// parseRowDate parses a row date like "2026-06-15 07:51" into a Unix timestamp.
func parseRowDate(s string) int64 {
	s = strings.TrimSpace(s)
	layouts := []string{"2006-01-02 15:04", "2006-01-02", "2006.01.02"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}

// looksLikeFileSize returns true for strings like "855 KB", "1.2 MB".
func looksLikeFileSize(s string) bool {
	return regexp.MustCompile(`(?i)^\s*[\d.,]+\s*(?:B|KB|MB|GB|bytes?)\s*$`).MatchString(s)
}

// looksLikeInt returns true for strings that are plain integers (with optional commas).
func looksLikeInt(s string) bool {
	cleaned := strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	_, err := strconv.Atoi(cleaned)
	return err == nil && cleaned != ""
}

// looksLikeDate returns true for strings matching common date layouts.
func looksLikeDate(s string) bool {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02 15:04", "2006-01-02", "2006.01.02"} {
		if _, err := time.Parse(layout, s); err == nil {
			return true
		}
	}
	return false
}

// parseFileSize converts a human-readable size ("855 KB", "1.2 MB") into bytes.
func parseFileSize(s string) int32 {
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	if len(fields) != 2 {
		return 0
	}
	num, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	unit := strings.ToUpper(fields[1])
	var mult float64
	switch unit {
	case "B", "BYTES":
		mult = 1
	case "KB":
		mult = 1024
	case "MB":
		mult = 1024 * 1024
	case "GB":
		mult = 1024 * 1024 * 1024
	default:
		return 0
	}
	return int32(num * mult)
}

// parseMonthName converts month name (short or long) to number.
func parseMonthName(name string) int {
	names := map[string]int{
		"january": 1, "february": 2, "march": 3, "april": 4,
		"may": 5, "june": 6, "july": 7, "august": 8,
		"september": 9, "october": 10, "november": 11, "december": 12,
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

// versionInFileNameRe matches a version token inside a FLiNG trainer filename.
// FLiNG filenames look like:
//   "Crimson.Desert.v1.0-v1.12.Plus.12.Trainer-FLiNG"
//   "Elden.Ring.v1.16.Plus.28.Trainer-FLiNG"
//   "Foo.v2025.06.15.Plus.10.Trainer-FLiNG"
//   "Foo.Update.2025.06.15.Plus.10.Trainer-FLiNG"
// We capture the "v..." segment (with an optional "-v..." range tail),
// stopping at ".Plus", ".Trainer", "-FLiNG", or end.
var versionInFileNameRe = regexp.MustCompile(`(?i)\b(v?\d+(?:\.\d+)*(?:[-_ ]*v?\d+(?:\.\d+)*)*)`)

// ParseVersionFromFileName extracts a human-readable trainer version string
// from a FLiNG filename. Examples:
//
//	"Crimson.Desert.v1.0-v1.12.Plus.12.Trainer-FLiNG" -> "v1.0-v1.12"
//	"Elden.Ring.v1.16.Plus.28.Trainer-FLiNG"          -> "v1.16"
//	"Foo.Bar.v1.2.3.Plus.10.Trainer-FLiNG"            -> "v1.2.3"
//	"Foo.Bar.Plus.10.Trainer-FLiNG" (no version)      -> ""
//
// For a version range "v1.0-v1.12" we keep the full range so the user can see
// which game versions the trainer covers. The trailing upper bound is what
// matters for "is this the latest" comparisons.
//
// Used to replace the meaningless "12 Options" string that used to populate
// Trainer.Version — the options count is already shown separately.
//
// Exported so app.go can reparse historical trainer rows on the fly without
// re-crawling.
func ParseVersionFromFileName(fileName string) string {
	return parseVersionFromFileName(fileName)
}

// parseVersionFromFileName is the internal implementation; see the exported
// wrapper above for the contract.
func parseVersionFromFileName(fileName string) string {
	if fileName == "" {
		return ""
	}
	// Strip a trailing "-FLiNG" / "-Fling" suffix so it doesn't interfere.
	base := strings.TrimSpace(fileName)
	base = strings.TrimSuffix(base, "-FLiNG")
	base = strings.TrimSuffix(base, "-Fling")

	// Look for the version token. We anchor on either a leading "v" with
	// digits, or a bare "N.N" shape, but skip leading tokens that are just
	// numbers (years like "2025" are ambiguous — require a dot or v prefix).
	for _, m := range versionInFileNameRe.FindAllString(base, -1) {
		token := strings.TrimSpace(m)
		if token == "" {
			continue
		}
		// Reject bare integers (likely years / option counts) — we want
		// something shaped like a version: contains a dot, or a "v" prefix.
		if !strings.Contains(token, ".") && !strings.HasPrefix(strings.ToLower(token), "v") {
			continue
		}
		// Reject the ".Plus.NN." options count token and similar.
		low := strings.ToLower(token)
		if strings.Contains(low, "plus") || strings.Contains(low, "trainer") {
			continue
		}
		return token
	}
	return ""
}
