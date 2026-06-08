package model

type Trainer struct {
	ID            int32  `json:"id"`
	GameID        int32  `json:"game_id"`
	Version       string `json:"version"`
	GameVersion   string `json:"game_version"`
	DownloadURL   string `json:"download_url"`
	FileSize      int32  `json:"file_size"`
	FileName      string `json:"file_name"`
	DownloadCount int32  `json:"download_count"`
	SourceHash    string `json:"source_hash"`
	UpdatedAt     int64  `json:"updated_at"`
}
