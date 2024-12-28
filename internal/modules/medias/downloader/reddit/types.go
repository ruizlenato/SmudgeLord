package reddit

type RedditPost []KindData

type KindData struct {
	Data struct {
		Children []struct {
			Data Data `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type Data struct {
	Title                 string                `json:"title"`
	MediaMetadata         *map[string]MediaItem `json:"media_metadata"`
    				GalleryData struct {
					Items []struct {
						MediaID string `json:"media_id"`
						ID      int    `json:"id"`
					} `json:"items"`
				} `json:"gallery_data"`
	SubredditNamePrefixed string                `json:"subreddit_name_prefixed"`
	IsRedditMediaDomain   bool                  `json:"is_reddit_media_domain"`
	Domain                string                `json:"domain"`
	Preview               struct {
		Images  []Images `json:"images"`
		Enabled bool     `json:"enabled"`
	} `json:"preview"`
	Author string `json:"author"`
	Media  struct {
		RedditVideo RedditVideo `json:"reddit_video"`
	} `json:"media"`
	URL     string `json:"url"`
	IsVideo bool   `json:"is_video"`
}


type MediaItem struct {
	E string     `json:"e"`
	M string     `json:"m"`
	S Resolution `json:"s"`
}

type Resolution struct {
	Y int    `json:"y"`
	X int    `json:"x"`
	U string `json:"u"`
}

type Images *struct {
	Source struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"source"`
	Resolutions []struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	} `json:"resolutions"`
	Variants struct{} `json:"variants"`
	ID       string   `json:"id"`
}

type RedditVideo struct {
	FallbackURL      string `json:"fallback_url"`
	Height           int    `json:"height"`
	Width            int    `json:"width"`
	ScrubberMediaURL string `json:"scrubber_media_url"`
	DashURL          string `json:"dash_url"`
	Duration         int    `json:"duration"`
	HlsURL           string `json:"hls_url"`
	IsGif            bool   `json:"is_gif"`
}
