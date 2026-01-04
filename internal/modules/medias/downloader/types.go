package downloader

import "github.com/go-telegram/bot/models"

var GenericHeaders = map[string]string{
	"Accept":             "*/*",
	"Accept-Language":    "en",
	"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Sec-Ch-UA":          `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
	"Sec-Ch-UA-Mobile":   "?0",
	"Sec-Ch-UA-Platform": `"Windows"`,
}

type Medias struct {
	Caption     string   `json:"caption"`
	Medias      []string `json:"medias"`
	InvertMedia bool     `json:"invert_media"`
}

type PostInfo struct {
	Medias      []models.InputMedia
	ID          string
	Caption     string
	Service     string
	InvertMedia bool
	NoMedia     bool
}

type YouTube struct {
	Video   string `json:"video"`
	Audio   string `json:"audio"`
	Caption string `json:"caption"`
}

type InputMedia struct {
	File      []byte
	Thumbnail []byte
}
