package service

import "testing"

func TestLookup_LayeredMatching(t *testing.T) {
	s := NewMappingService()
	// Simulate the real mapping file shape.
	if err := s.LoadFromBytes([]byte(`[
		{"name_en":"Baldur's Gate 3","name_zh":"博德之门3","aliases":["BG3"]},
		{"name_en":"Resident Evil 4","name_zh":"生化危机4","aliases":["生化危机4重制版"]},
		{"name_en":"HITMAN 3","name_zh":"杀手3","aliases":["Hitman 3"]},
		{"name_en":"Total War: Warhammer III","name_zh":"全面战争：战锤3","aliases":[]}
	]`)); err != nil {
		t.Fatalf("load: %v", err)
	}

	cases := []struct {
		name   string
		input  string
		want   string
		found  bool
	}{
		// 1. exact case-insensitive
		{"exact", "Baldur's Gate 3", "博德之门3", true},
		{"exact lower", "baldur's gate 3", "博德之门3", true},
		// 2. alias
		{"alias", "BG3", "博德之门3", true},
		// 3. normalized: HTML entity + apostrophe collapse
		{"html entity apostrophe", "Baldur&#8217;s Gate 3", "博德之门3", true},
		{"no apostrophe", "Baldurs Gate 3", "博德之门3", true},
		// 4. subtitle strip
		{"subtitle colon", "Total War: Warhammer III", "全面战争：战锤3", true},
		// 5. trailing Trainer token
		{"trailing trainer", "HITMAN 3 Trainer", "杀手3", true},
		// miss
		{"unknown", "Some Unknown Game", "Some Unknown Game", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, found := s.Lookup(c.input)
			if found != c.found {
				t.Fatalf("Lookup(%q) found=%v want %v (got %q)", c.input, found, c.found, got)
			}
			if found && got != c.want {
				t.Errorf("Lookup(%q) = %q want %q", c.input, got, c.want)
			}
			if !found {
				// On miss, GetChineseName should return the original
				if got := s.GetChineseName(c.input); got != c.input {
					t.Errorf("GetChineseName(%q) = %q, want original", c.input, got)
				}
			}
		})
	}
}

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"Baldur's Gate 3":    "baldursgate3",
		"Baldur&#8217;s Gate 3": "baldursgate3",
		"HITMAN 3":           "hitman3",
		"Resident Evil 4":    "residentevil4",
		"  Spaced  Out ":     "spacedout",
	}
	for in, want := range cases {
		if got := normalizeKey(in); got != want {
			t.Errorf("normalizeKey(%q) = %q want %q", in, got, want)
		}
	}
}
