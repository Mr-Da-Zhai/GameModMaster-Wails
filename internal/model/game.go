package model

type Game struct {
	ID         int32  `json:"id"`
	SourceID   string `json:"source_id"`
	NameEN     string `json:"name_en"`
	NameLocal  string `json:"name_local"`
	CoverURL   string `json:"cover_url"`
	SourceURL  string `json:"source_url"`
	OptionsNum int16  `json:"options_num"`
	UpdatedAt  int64  `json:"updated_at"`
}
