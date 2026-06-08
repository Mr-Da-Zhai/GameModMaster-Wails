package index

import (
	"fmt"
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

// SearchGames searches by query across name_en, name_local, and aliases
func (idx *Index) SearchGames(query string, limit int) []*model.Game {
	q := strings.ToLower(query)
	seen := make(map[int32]bool)
	var results []*model.Game

	// Helper to add a game if not already seen
	addGame := func(g *model.Game) {
		if g == nil || seen[g.ID] {
			return
		}
		seen[g.ID] = true
		results = append(results, g)
	}

	// Search through all games (already sorted by updated_at DESC from LoadFromDB)
	for _, g := range idx.GamesByUpdated {
		if len(results) >= limit {
			break
		}
		if strings.Contains(strings.ToLower(g.NameEN), q) ||
			strings.Contains(strings.ToLower(g.NameLocal), q) {
			addGame(g)
		}
	}

	// Search through aliases if we haven't hit the limit
	if len(results) < limit {
		for alias, nameZH := range idx.AliasIndex {
			if len(results) >= limit {
				break
			}
			if strings.Contains(strings.ToLower(alias), q) {
				if g, ok := idx.GamesByNameLocal[strings.ToLower(nameZH)]; ok {
					addGame(g)
				}
			}
		}
	}

	return results
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
