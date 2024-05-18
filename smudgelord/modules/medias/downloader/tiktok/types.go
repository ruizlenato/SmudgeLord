package tiktok

type TikTokData *struct {
	VideoURL string    `json:"video_url"`
	Images   *[]string `json:"images"`
	Title    string    `json:"title"`
}
