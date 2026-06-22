package index

import (
	"fmt"
	"sort"
	"strings"

	"GameModMaster/internal/model"
	"GameModMaster/internal/repo"
)

type Index struct {
	// ID lookups
	GamesByID    map[int32]*model.Game
	TrainersByID map[int32]*model.Trainer
	StatesByID   map[int32]*model.TrainerState

	// Name lookups (lowercase keys)
	GamesByNameEN    map[string]*model.Game
	GamesByNameLocal map[string]*model.Game

	// Game -> Trainers
	TrainersByGame map[int32][]*model.Trainer

	// Sorted list for home page
	GamesByUpdated []*model.Game

	// Name mapping (en->zh, alias->zh)
	NameMapping map[string]string
	AliasIndex  map[string]string
}

// New creates a new empty index
func New() *Index {
	return &Index{
		GamesByID:        make(map[int32]*model.Game),
		TrainersByID:     make(map[int32]*model.Trainer),
		StatesByID:       make(map[int32]*model.TrainerState),
		GamesByNameEN:    make(map[string]*model.Game),
		GamesByNameLocal: make(map[string]*model.Game),
		TrainersByGame:   make(map[int32][]*model.Trainer),
		GamesByUpdated:   nil,
		NameMapping:      make(map[string]string),
		AliasIndex:       make(map[string]string),
	}
}

// LoadFromDB loads all data from repositories into memory
func (idx *Index) LoadFromDB(gameRepo *repo.GameRepo, trainerRepo *repo.TrainerRepo, stateRepo *repo.StateRepo) error {
	// Load games
	games, err := gameRepo.GetAll()
	if err != nil {
		return fmt.Errorf("load games: %w", err)
	}

	idx.GamesByID = make(map[int32]*model.Game, len(games))
	idx.GamesByNameEN = make(map[string]*model.Game, len(games))
	idx.GamesByNameLocal = make(map[string]*model.Game, len(games))
	idx.GamesByUpdated = make([]*model.Game, len(games))

	for i, g := range games {
		idx.GamesByID[g.ID] = g
		idx.GamesByNameEN[strings.ToLower(g.NameEN)] = g
		if g.NameLocal != "" {
			idx.GamesByNameLocal[strings.ToLower(g.NameLocal)] = g
		}
		idx.GamesByUpdated[i] = g
	}

	// Load all trainers grouped by game
	// We iterate all games to get their trainers
	idx.TrainersByID = make(map[int32]*model.Trainer)
	idx.TrainersByGame = make(map[int32][]*model.Trainer)

	for _, g := range games {
		trainers, err := trainerRepo.GetByGameID(g.ID)
		if err != nil {
			return fmt.Errorf("load trainers for game %d: %w", g.ID, err)
		}
		for _, t := range trainers {
			idx.TrainersByID[t.ID] = t
		}
		if len(trainers) > 0 {
			idx.TrainersByGame[g.ID] = trainers
		}
	}

	// Load states
	states, err := stateRepo.ListAll()
	if err != nil {
		return fmt.Errorf("load states: %w", err)
	}

	idx.StatesByID = make(map[int32]*model.TrainerState, len(states))
	for _, s := range states {
		idx.StatesByID[s.TrainerID] = s
	}

	return nil
}

// LoadNameMapping loads the name mapping from the mapping service
func (idx *Index) LoadNameMapping(mapping map[string]string, aliases map[string]string) {
	idx.NameMapping = mapping
	idx.AliasIndex = aliases
}

// SearchGames searches by query across name_en, name_local, and aliases.
//
// The query is matched (case-insensitive, substring) against each game's
// English name, localized (Chinese) name, and the alias index. The alias
// index also lets a Chinese query resolve to the English name and vice-versa,
// so typing e.g. "生化危机" finds "Resident Evil 4" even when the stored
// name_local happens to differ. Results are de-duplicated and capped at limit.
func (idx *Index) SearchGames(query string, limit int) []*model.Game {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || limit <= 0 {
		return nil
	}

	seen := make(map[int32]bool)
	var results []*model.Game

	addGame := func(g *model.Game) {
		if g == nil || seen[g.ID] {
			return
		}
		seen[g.ID] = true
		results = append(results, g)
	}

	// Expand the query: collect any English names whose alias/translation
	// contains the query, so a Chinese query can still match games stored
	// primarily by English name.
	extraENKeys := map[string]bool{}
	for aliasKey, zh := range idx.AliasIndex {
		if strings.Contains(aliasKey, q) || strings.Contains(strings.ToLower(zh), q) {
			extraENKeys[aliasKey] = true
		}
	}

	for _, g := range idx.GamesByUpdated {
		if len(results) >= limit {
			break
		}
		loEN := strings.ToLower(g.NameEN)
		loLocal := strings.ToLower(g.NameLocal)

		hit := strings.Contains(loEN, q) ||
			strings.Contains(loLocal, q) ||
			extraENKeys[loEN]

		// Also: the query may be a Chinese alias whose English key (stored as
		// name_en) we should match against.
		if !hit {
			for aliasKey := range extraENKeys {
				if loEN == aliasKey {
					hit = true
					break
				}
			}
		}

		if hit {
			addGame(g)
		}
	}

	// Fall back to alias-driven lookup by Chinese name (reverse map).
	if len(results) < limit {
		for aliasKey, zh := range idx.AliasIndex {
			if len(results) >= limit {
				break
			}
			if strings.Contains(aliasKey, q) || strings.Contains(strings.ToLower(zh), q) {
				if g, ok := idx.GamesByNameEN[aliasKey]; ok {
					addGame(g)
				} else if g, ok := idx.GamesByNameLocal[strings.ToLower(zh)]; ok {
					addGame(g)
				}
			}
		}
	}

	return results
}

// Suggestion is a single autocomplete suggestion with a relevance score.
// Higher score = better match. Score bands:
//   100 = exact (case-insensitive) name match
//    80 = prefix match on the english or localized name
//    60 = prefix match on any alias
//    40 = substring match on the english or localized name
//    20 = substring match via the alias index
type Suggestion struct {
	Game  *model.Game
	Score int
}

// SearchSuggestions is a relevance-ordered variant of SearchGames, optimised
// for the autocomplete dropdown. It returns at most `limit` suggestions
// ranked so prefix matches float to the top (typing "elden" surfaces
// "Elden Ring" before "Lord of the Rings: War in the North" which merely
// contains the substring).
//
// Pure in-memory; safe to call on every keystroke.
func (idx *Index) SearchSuggestions(query string, limit int) []Suggestion {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" || limit <= 0 {
		return nil
	}

	scored := make(map[int32]int) // game ID -> best score

	// 1. Direct matches against stored names.
	for _, g := range idx.GamesByID {
		loEN := strings.ToLower(g.NameEN)
		loLocal := strings.ToLower(g.NameLocal)

		var s int
		switch {
		case loEN == q || loLocal == q:
			s = 100
		case strings.HasPrefix(loEN, q) || strings.HasPrefix(loLocal, q):
			s = 80
		case strings.Contains(loEN, q) || strings.Contains(loLocal, q):
			s = 40
		}
		if s > scored[g.ID] {
			scored[g.ID] = s
		}
	}

	// 2. Alias-driven matches (lets a Chinese query resolve to an English
	//    title even when the game's stored name_local differs).
	for aliasKey, zh := range idx.AliasIndex {
		zhLow := strings.ToLower(zh)
		var s int
		switch {
		case aliasKey == q || zhLow == q:
			s = 100
		case strings.HasPrefix(aliasKey, q) || strings.HasPrefix(zhLow, q):
			s = 60
		case strings.Contains(aliasKey, q) || strings.Contains(zhLow, q):
			s = 20
		}
		if s == 0 {
			continue
		}
		// aliasKey is a lowercase english name; resolve to the game.
		if g, ok := idx.GamesByNameEN[aliasKey]; ok {
			if s > scored[g.ID] {
				scored[g.ID] = s
			}
		}
	}

	if len(scored) == 0 {
		return nil
	}

	// Collect and sort by score desc, then by english name asc for stability.
	out := make([]Suggestion, 0, len(scored))
	for id, s := range scored {
		if g, ok := idx.GamesByID[id]; ok {
			out = append(out, Suggestion{Game: g, Score: s})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Game.NameEN < out[j].Game.NameEN
	})

	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// GetTrainersForGame returns all trainers for a game
func (idx *Index) GetTrainersForGame(gameID int32) []*model.Trainer {
	return idx.TrainersByGame[gameID]
}

// GetTrainerState returns the state for a trainer
func (idx *Index) GetTrainerState(trainerID int32) *model.TrainerState {
	return idx.StatesByID[trainerID]
}

// Refresh reloads all data from DB (called after data changes)
func (idx *Index) Refresh(gameRepo *repo.GameRepo, trainerRepo *repo.TrainerRepo, stateRepo *repo.StateRepo) error {
	return idx.LoadFromDB(gameRepo, trainerRepo, stateRepo)
}
