package service

import (
	"encoding/json"
	"os"
	"strings"
	"unicode"
)

// nameEntry represents a single entry in name_mapping.json.
type nameEntry struct {
	NameEN  string   `json:"name_en"`
	NameZH  string   `json:"name_zh"`
	Aliases []string `json:"aliases"`
}

// MappingService provides English-to-Chinese name lookups using an in-memory index.
//
// Lookup is layered:
//  1. Exact (case-insensitive) match on name_en
//  2. Exact (case-insensitive) match on aliases
//  3. Normalized match (decode HTML entities, strip punctuation, collapse spaces)
//  4. Subtitle-stripped match (drop everything after ":" / " - " / " | ")
//  5. Token-suffix match (drop a trailing "Trainer" token)
type MappingService struct {
	// raw entries kept for debugging / export
	entries []nameEntry

	// Exact lookups — lowercase keys
	exactEN     map[string]string // lowercase english name → chinese
	exactAlias  map[string]string // lowercase alias → chinese
	normIndex   map[string]string // normalized key → chinese
}

// NewMappingService creates a MappingService with empty maps.
func NewMappingService() *MappingService {
	return &MappingService{
		exactEN:    make(map[string]string),
		exactAlias: make(map[string]string),
		normIndex:  make(map[string]string),
	}
}

// Load reads the name mapping JSON file and builds the lookup maps.
func (s *MappingService) Load(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return s.LoadFromBytes(data)
}

// LoadFromBytes parses name mapping from raw JSON bytes and builds the indexes.
func (s *MappingService) LoadFromBytes(data []byte) error {
	var entries []nameEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	s.entries = entries
	s.exactEN = make(map[string]string, len(entries))
	s.exactAlias = make(map[string]string)
	s.normIndex = make(map[string]string, len(entries)*3)

	for _, e := range entries {
		if e.NameEN == "" || e.NameZH == "" {
			continue
		}
		// Exact (case-insensitive) keys
		s.exactEN[strings.ToLower(e.NameEN)] = e.NameZH
		for _, alias := range e.Aliases {
			s.exactAlias[strings.ToLower(alias)] = e.NameZH
		}
		// Normalized keys (also register aliases normalized)
		registerNorm := func(raw string) {
			k := normalizeKey(raw)
			if k != "" {
				// Don't overwrite an existing normalized entry (first wins)
				if _, ok := s.normIndex[k]; !ok {
					s.normIndex[k] = e.NameZH
				}
			}
		}
		registerNorm(e.NameEN)
		registerNorm(stripSubtitle(e.NameEN))
		for _, alias := range e.Aliases {
			registerNorm(alias)
		}
	}

	return nil
}

// GetChineseName looks up the Chinese name for an English name.
// Returns the original name if no mapping is found.
func (s *MappingService) GetChineseName(englishName string) string {
	if zh, ok := s.Lookup(englishName); ok {
		return zh
	}
	return englishName
}

// TranslateBatch translates a batch of names, returning map[original]→translated.
func (s *MappingService) TranslateBatch(names []string) map[string]string {
	result := make(map[string]string, len(names))
	for _, name := range names {
		result[name] = s.GetChineseName(name)
	}
	return result
}

// Lookup performs the layered search and returns (chineseName, found).
func (s *MappingService) Lookup(query string) (string, bool) {
	q := strings.TrimSpace(query)
	if q == "" {
		return "", false
	}

	lq := strings.ToLower(q)

	// 1. Exact name_en
	if zh, ok := s.exactEN[lq]; ok {
		return zh, true
	}
	// 2. Exact alias
	if zh, ok := s.exactAlias[lq]; ok {
		return zh, true
	}
	// 3. Normalized full match
	nk := normalizeKey(q)
	if zh, ok := s.normIndex[nk]; ok {
		return zh, true
	}
	// 4. Strip subtitle, then normalized match
	if sub := normalizeKey(stripSubtitle(q)); sub != "" && sub != nk {
		if zh, ok := s.normIndex[sub]; ok {
			return zh, true
		}
	}
	// 5. Strip a trailing "Trainer" token, then normalized match
	if stripped := normalizeKey(stripTrainerToken(q)); stripped != "" && stripped != nk {
		if zh, ok := s.normIndex[stripped]; ok {
			return zh, true
		}
	}

	return "", false
}

// GetMapping returns the lowercase english name → chinese map (en->zh).
func (s *MappingService) GetMapping() map[string]string {
	return s.exactEN
}

// GetAliases returns the lowercase alias → chinese map (alias->zh).
func (s *MappingService) GetAliases() map[string]string {
	return s.exactAlias
}

// Count returns the number of loaded entries.
func (s *MappingService) Count() int {
	return len(s.entries)
}

// normalizeKey produces a canonical lookup key from a raw name:
//   - decode common HTML entities (&#8217; &#039; &#8211; &#038; …)
//   - lowercase
//   - strip all punctuation/whitespace
//
// "Baldur&#8217;s Gate 3", "Baldur's Gate 3", "Baldurs Gate 3"
// all collapse to "baldursgate3".
func normalizeKey(s string) string {
	s = decodeEntities(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 0x4e00 && r <= 0x9fff: // CJK ideographs kept as-is
			b.WriteRune(r)
		default:
			// drop punctuation, spaces, symbols
		}
	}
	return b.String()
}

// stripSubtitle drops the subtitle portion of a name.
// Everything after the first of these delimiters is removed:
//   - " : "  (colon subtitle)
//   - " - "  (dash subtitle)
//   - " | "  (bar separator)
//
// "Foo: Bar" -> "Foo", "Foo - Bar" -> "Foo".
func stripSubtitle(s string) string {
	s = decodeEntities(s)
	delims := []string{" : ", " - ", " | "}
	cut := s
	for _, d := range delims {
		if idx := strings.Index(cut, d); idx >= 0 {
			candidate := strings.TrimSpace(cut[:idx])
			if len(candidate) >= 2 {
				cut = candidate
			}
		}
	}
	return cut
}

// stripTrainerToken removes a trailing "Trainer" word if present.
func stripTrainerToken(s string) string {
	lo := strings.ToLower(strings.TrimSpace(s))
	if strings.HasSuffix(lo, " trainer") {
		return strings.TrimSpace(s[:len(s)-len(" trainer")])
	}
	if lo == "trainer" {
		return ""
	}
	return s
}

// decodeEntities replaces a handful of HTML numeric/named entities that appear
// in scraped game titles with their plain-text equivalents.
func decodeEntities(s string) string {
	r := strings.NewReplacer(
		"&#8217;", "'",
		"&#8216;", "'",
		"&#8220;", "\"",
		"&#8221;", "\"",
		"&#8211;", "-",
		"&#8212;", "-",
		"&#039;", "'",
		"&#038;", "&",
		"&amp;", "&",
		"&quot;", "\"",
		"&#39;", "'",
		"&apos;", "'",
	)
	return r.Replace(s)
}
