package launcher

import (
	"encoding/json"
)

type mediaPreviewData struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	AppName   string `json:"appName"`
	Artwork   string `json:"artwork"`
	Position  int64  `json:"position"`
	Duration  int64  `json:"duration"`
	IsPlaying bool   `json:"isPlaying"`
}

func decodeMediaPreview(value string) (mediaPreviewData, error) {
	var data mediaPreviewData
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return mediaPreviewData{}, err
	}
	return data, nil
}
