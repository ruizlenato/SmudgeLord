package tiktok

type Handler struct {
	postID string
}

type TikTokData *struct {
	AwemeList []Aweme `json:"aweme_list"`
}

type Aweme struct {
	AwemeID       string        `json:"aweme_id"`
	Desc          *string       `json:"desc"`
	Author        Author        `json:"author,omitzero"`
	Music         Music         `json:"music,omitzero"`
	Video         Video         `json:"video,omitzero"`
	ImagePostInfo ImagePostInfo `json:"image_post_info,omitzero"`
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
