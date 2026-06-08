package model

type TrainerStatus uint8

const (
	StatusAvailable  TrainerStatus = 0
	StatusDownloaded TrainerStatus = 1
	StatusInstalled  TrainerStatus = 2
)

type TrainerState struct {
	TrainerID   int32         `json:"trainer_id"`
	Status      TrainerStatus `json:"status"`
	LocalPath   string        `json:"local_path"`
	InstalledAt int64         `json:"installed_at"`
	LaunchedAt  int64         `json:"launched_at"`
}

// GameWithTrainers is a view model for displaying a game with its trainers
type GameWithTrainers struct {
	Game     Game          `json:"game"`
	Trainers []Trainer     `json:"trainers"`
	State    *TrainerState `json:"state,omitempty"`
}

// TrainerWithState is a view model for a trainer with its state and game info
type TrainerWithState struct {
	Trainer Trainer       `json:"trainer"`
	Game    Game          `json:"game"`
	State   *TrainerState `json:"state,omitempty"`
}
