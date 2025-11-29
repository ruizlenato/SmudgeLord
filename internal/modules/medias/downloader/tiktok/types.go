package tiktok

type Handler struct {
	username string
	postID   string
}

type TikTokData *struct {
	AwemeList []Aweme `json:"aweme_list"`
}

type Aweme struct {
	AwemeID       string        `json:"aweme_id"`
	Desc          *string       `json:"desc"`
	Author        Author        `json:"author"`
	Music         Music         `json:"music"`
	Video         Video         `json:"video"`
	ImagePostInfo ImagePostInfo `json:"image_post_info"`
	ShareURL      string        `json:"share_url"`
	AwemeType     int           `json:"aweme_type"`
}

type Author struct {
	Nickname     *string      `json:"nickname"`
	UniqueID     string       `json:"unique_id"`
	AvatarLarger AvatarLarger `json:"avatar_larger"`
}

type AvatarLarger struct {
	URLList []string `json:"url_list"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
}

type Music struct {
	Title  string `json:"title"`
	Author string `json:"author"`
	Album  string `json:"album"`
}

type PlayURL struct {
	URI       string   `json:"uri"`
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	URLPrefix any      `json:"url_prefix"`
}

type Video struct {
	PlayAddr PlayAddr `json:"play_addr"`
	Cover    Cover    `json:"cover"`
	Height   int      `json:"height"`
	Width    int      `json:"width"`
	Duration int      `json:"duration"`
}

type ImagePostInfo struct {
	Images []Image `json:"images"`
}

type Image struct {
	DisplayImage Cover `json:"display_image"`
	Thumbnail    Cover `json:"thumbnail"`
}

type PlayAddr struct {
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	DataSize  int      `json:"data_size"`
	FileHash  string   `json:"file_hash"`
	URLPrefix any      `json:"url_prefix"`
}

type Cover struct {
	URLList   []string `json:"url_list"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	URLPrefix any      `json:"url_prefix"`
}

var TikTokQueryParams = map[string]string{
	"iid":             "7318518857994389254",
	"device_id":       "7318517321748022790",
	"channel":         "googleplay",
	"version_code":    "300904",
	"device_platform": "android",
	"device_type":     "ASUS_Z01QD",
	"os_version":      "9",
	"aid":             "1128",
}
