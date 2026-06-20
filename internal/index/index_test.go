package index

import (
	"testing"

	"GameModMaster/internal/model"
)

func TestSearchGames_ChineseFindsEnglish(t *testing.T) {
	idx := New()

	// Simulate DB-loaded games with both English and localized names.
	games := []*model.Game{
		{ID: 1, NameEN: "Resident Evil 4 Trainer", NameLocal: "生化危机4"},
		{ID: 2, NameEN: "Crimson Desert Trainer", NameLocal: "红色沙漠"},
		{ID: 3, NameEN: "Baldur's Gate 3 Trainer", NameLocal: "博德之门3"},
	}
	idx.GamesByUpdated = make([]*model.Game, len(games))
	for i, g := range games {
		idx.GamesByID[g.ID] = g
		idx.GamesByNameEN[lower(g.NameEN)] = g
		idx.GamesByNameLocal[lower(g.NameLocal)] = g
		idx.GamesByUpdated[i] = g
	}

	// Alias index maps lowercase english -> chinese (as the mapping service does).
	idx.AliasIndex = map[string]string{
		"resident evil 4 trainer": "生化危机4",
		"crimson desert trainer":  "红色沙漠",
		"baldur's gate 3 trainer": "博德之门3",
		"生化危机4":                "Resident Evil 4",
		"博德之门3":                "Baldur's Gate 3",
	}

	cases := []struct {
		query   string
		wantHit bool
	}{
		{"生化危机", true},   // Chinese substring of name_local
		{"沙漠", true},       // Chinese substring
		{"Resident", true},  // English substring
		{"resident evil", true},
		{"不存在的游戏", false},
	}
	for _, c := range cases {
		res := idx.SearchGames(c.query, 20)
		got := len(res) > 0
		if got != c.wantHit {
			t.Errorf("SearchGames(%q) hit=%v want %v (got %d results)", c.query, got, c.wantHit, len(res))
		}
	}
}

func lower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
