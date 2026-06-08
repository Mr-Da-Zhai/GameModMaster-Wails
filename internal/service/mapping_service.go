package service

import (
	"encoding/json"
	"os"
	"strings"
)

// nameEntry represents a single entry in name_mapping.json.
type nameEntry struct {
	NameEN  string   `json:"name_en"`
	NameZH  string   `json:"name_zh"`
	Aliases []string `json:"aliases"`
}

// MappingService provides English-to-Chinese name lookups using an in-memory index.
type MappingService struct {
	nameToZH map[string]string // lowercase english name → chinese name
	aliasToZH map[string]string // lowercase alias → chinese name
}

// NewMappingService creates a MappingService with empty maps.
func NewMappingService() *MappingService {
	return &MappingService{
		nameToZH:  make(map[string]string),
		aliasToZH: make(map[string]string),
	}
}

// Load reads the name mapping JSON file and builds both lookup maps.
func (s *MappingService) Load(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return err
	}
	return s.LoadFromBytes(data)
}

// LoadFromBytes parses name mapping from raw JSON bytes.
func (s *MappingService) LoadFromBytes(data []byte) error {
	var entries []nameEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}

	// Rebuild maps to allow reloading
	s.nameToZH = make(map[string]string, len(entries))
	s.aliasToZH = make(map[string]string)

	for _, e := range entries {
		key := strings.ToLower(e.NameEN)
		s.nameToZH[key] = e.NameZH

		for _, alias := range e.Aliases {
			aliasKey := strings.ToLower(alias)
			s.aliasToZH[aliasKey] = e.NameZH
		}
	}

	return nil
}

// GetChineseName looks up the Chinese name by English name (case insensitive).
// Returns the original name if no mapping is found.
func (s *MappingService) GetChineseName(englishName string) string {
	if zh, ok := s.nameToZH[strings.ToLower(englishName)]; ok {
		return zh
	}
	return englishName
}

// TranslateBatch translates a batch of names, returning map[original]→translated.
// Names without a mapping keep their original value.
func (s *MappingService) TranslateBatch(names []string) map[string]string {
	result := make(map[string]string, len(names))
	for _, name := range names {
		result[name] = s.GetChineseName(name)
	}
	return result
}

// Lookup searches across English names and aliases (case insensitive).
// Returns the Chinese name and whether a match was found.
func (s *MappingService) Lookup(query string) (string, bool) {
	q := strings.ToLower(query)

	if zh, ok := s.nameToZH[q]; ok {
		return zh, true
	}
	if zh, ok := s.aliasToZH[q]; ok {
		return zh, true
	}
	return "", false
}

// GetMapping returns the internal nameToZH map (en->zh).
func (s *MappingService) GetMapping() map[string]string {
	return s.nameToZH
}

// GetAliases returns the internal aliasToZH map (alias->zh).
func (s *MappingService) GetAliases() map[string]string {
	return s.aliasToZH
}
