package repo

import (
	"path/filepath"
	"testing"

	"GameModMaster/internal/model"
)

// newTestDB opens a throwaway in-memory-ish DB in t.TempDir() so each test
// is fully isolated. The schema (including the CASCADE foreign keys that
// this test cares about) is created by Open.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// TestBatchUpsertDoesNotCascadeDeleteStates is a regression test for a
// serious data-loss bug: both games and trainers used INSERT OR REPLACE,
// which under SQLite deletes the conflicting row and inserts a new one
// (with a new rowid even though our subselect re-supplies the same id).
// Because trainers.game_id REFERENCES games(id) ON DELETE CASCADE and
// trainer_states.trainer_id REFERENCES trainers(id) ON DELETE CASCADE,
// every re-scrape cascaded all the way down and wiped the user's
// download/install state.
//
// This test seeds a game + trainer + downloaded state, then BatchUpserts
// the same game and trainer again (simulating a refresh) and asserts the
// state row survives.
func TestBatchUpsertDoesNotCascadeDeleteStates(t *testing.T) {
	db := newTestDB(t)
	gRepo := NewGameRepo(db)
	tRepo := NewTrainerRepo(db)
	sRepo := NewStateRepo(db)

	game := &model.Game{
		SourceID:  "elden-ring",
		NameEN:    "Elden Ring",
		NameLocal: "艾尔登法环",
		UpdatedAt: 1700000000,
	}
	if err := gRepo.BatchUpsert([]*model.Game{game}); err != nil {
		t.Fatalf("seed game: %v", err)
	}
	storedGame, err := gRepo.GetBySourceID("elden-ring")
	if err != nil || storedGame == nil {
		t.Fatalf("resolve seeded game: %v (%v)", storedGame, err)
	}

	trainer := &model.Trainer{
		GameID:      storedGame.ID,
		Version:     "v1.16",
		GameVersion: "v1.16",
		SourceHash:  "abc123",
		FileName:    "Elden.Ring.v1.16.Plus.28.Trainer-FLiNG",
		UpdatedAt:   1700000000,
	}
	if err := tRepo.BatchUpsert([]*model.Trainer{trainer}); err != nil {
		t.Fatalf("seed trainer: %v", err)
	}
	storedTrainer, err := tRepo.GetByID(storedGame.ID)
	if err != nil {
		t.Fatalf("list trainers: %v", err)
	}
	_ = storedTrainer // GetByID returns single by id; use hash lookup instead

	// Resolve the trainer id via the index/GetByGameID for the state write.
	seededTrainers, err := tRepo.GetByGameID(storedGame.ID)
	if err != nil {
		t.Fatalf("list trainers by game: %v", err)
	}
	if len(seededTrainers) != 1 {
		t.Fatalf("expected 1 seeded trainer, got %d", len(seededTrainers))
	}
	trainerID := seededTrainers[0].ID

	// Mark as downloaded (this is the state that must survive a refresh).
	if err := sRepo.Upsert(&model.TrainerState{
		TrainerID: trainerID,
		Status:    model.StatusDownloaded,
		LocalPath: "E:\\fake\\trainer.exe",
	}); err != nil {
		t.Fatalf("seed state: %v", err)
	}

	// Sanity: state is there before the re-upsert.
	before, err := sRepo.GetByTrainerID(trainerID)
	if err != nil || before == nil {
		t.Fatalf("state missing before re-upsert: %v (%v)", before, err)
	}

	// Re-upsert the SAME game and trainer (simulates a refresh where the
	// remote data is unchanged). With the old INSERT OR REPLACE this would
	// cascade-delete the trainer_states row.
	game.NameEN = "Elden Ring" // unchanged on purpose
	if err := gRepo.BatchUpsert([]*model.Game{game}); err != nil {
		t.Fatalf("re-upsert game: %v", err)
	}
	trainer.Version = "v1.16" // unchanged on purpose
	if err := tRepo.BatchUpsert([]*model.Trainer{trainer}); err != nil {
		t.Fatalf("re-upsert trainer: %v", err)
	}

	// The state MUST still be there.
	after, err := sRepo.GetByTrainerID(trainerID)
	if err != nil {
		t.Fatalf("query state after re-upsert: %v", err)
	}
	if after == nil {
		t.Fatalf("REGRESSION: trainer_states row for trainer %d was wiped by BatchUpsert (CASCADE delete)", trainerID)
	}
	if after.Status != model.StatusDownloaded {
		t.Errorf("status changed: got %d, want %d", after.Status, model.StatusDownloaded)
	}
	if after.LocalPath != "E:\\fake\\trainer.exe" {
		t.Errorf("local_path changed: got %q, want %q", after.LocalPath, "E:\\fake\\trainer.exe")
	}

	// And the trainer row must be the same one (same id) — re-upsert must
	// not have deleted+recreated it.
	afterTrainers, err := tRepo.GetByGameID(storedGame.ID)
	if err != nil {
		t.Fatalf("list trainers after re-upsert: %v", err)
	}
	if len(afterTrainers) != 1 || afterTrainers[0].ID != trainerID {
		t.Errorf("trainer id changed after re-upsert: before=%d after=%+v", trainerID, afterTrainers)
	}
}
