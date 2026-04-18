package tiktok

const (
	APIHostname  = "api16-normal-c-useast1a.tiktokv.com"
	AppUserAgent = "com.zhiliaoapp.musically/2023501030 (Linux; U; Android 13; en_US; Pixel 7; Build/TD1A.220804.031; Cronet/58.0.2991.0)"
	WebUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

type Handler struct {
	username         string
	postID           string
	webURL           string
	webData          TikTokData
	cookies          string
	deviceID         string
	mediaUnavailable bool
}

type TikTokData *struct {
	AwemeList []Aweme `json:"aweme_list"`
}

type Aweme struct {
	AwemeID       string        `json:"aweme_id"`
	Desc          *string       `json:"desc"`
	Author        Author        `json:"author"`
	Video         Video         `json:"video"`
	ImagePostInfo ImagePostInfo `json:"image_post_info"`
	AwemeType     int           `json:"aweme_type"`
}

type Author struct {
	Nickname *string `json:"nickname"`
	UniqueID string  `json:"unique_id"`
}

type Video struct {
	PlayAddr    PlayAddr      `json:"play_addr"`
	Cover       Cover         `json:"cover"`
	Height      int           `json:"height"`
	Width       int           `json:"width"`
	Duration    int           `json:"duration"`
	BitrateInfo []BitrateInfo `json:"bitrate_info"`
}

type BitrateInfo struct {
	GearName string   `json:"gear_name"`
	Bitrate  int      `json:"bitrate"`
	PlayAddr PlayAddr `json:"play_addr"`
}

type ImagePostInfo struct {
	Images []Image `json:"images"`
}

type Image struct {
	DisplayImage Cover `json:"display_image"`
	Thumbnail    Cover `json:"thumbnail"`
}

type PlayAddr struct {
	URLList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

type Cover struct {
	URLList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

type WebUniversalData struct {
	DefaultScope WebDefaultScope `json:"__DEFAULT_SCOPE__"`
}

type WebDefaultScope struct {
	Webapp WebApp `json:"webapp.video-detail"`
}

type WebApp struct {
	ItemInfo WebItemInfo `json:"itemInfo"`
}

type WebItemInfo struct {
	ItemStruct WebItemStruct `json:"itemStruct"`
}

type WebItemStruct struct {
	ID        string        `json:"id"`
	Desc      string        `json:"desc"`
	Author    WebAuthor     `json:"author"`
	Video     WebVideo      `json:"video"`
	ImagePost *WebImagePost `json:"imagePost,omitempty"`
}

type WebAuthor struct {
	Nickname string `json:"nickname"`
	UniqueID string `json:"uniqueId"`
}

type WebVideo struct {
	PlayAddr     string           `json:"playAddr"`
	DownloadAddr string           `json:"downloadAddr"`
	Cover        any              `json:"cover"`
	OriginCover  string           `json:"originCover"`
	Duration     int              `json:"duration"`
	Width        int              `json:"width"`
	Height       int              `json:"height"`
	BitrateInfo  []WebBitrateInfo `json:"bitrateInfo"`
}

type WebBitrateInfo struct {
	GearName string      `json:"gearName"`
	Bitrate  int         `json:"bitrate"`
	PlayAddr WebPlayAddr `json:"PlayAddr"`
}

type WebPlayAddr struct {
	URLList []string `json:"UrlList"`
}

type WebImagePost struct {
	Images []WebImagePostImage `json:"images"`
}

type WebImagePostImage struct {
	DisplayImage struct {
		URLList []string `json:"urlList"`
	} `json:"displayImage"`
}
