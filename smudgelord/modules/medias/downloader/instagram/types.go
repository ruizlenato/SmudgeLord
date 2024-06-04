package instagram

type InstagramData *struct {
	ShortcodeMedia *ShortcodeMedia `json:"shortcode_media"`
	Data           struct {
		XDTShortcodeMedia *ShortcodeMedia `json:"xdt_shortcode_media"`
	} `json:"data,omitempty"`
}

type ShortcodeMedia struct {
	Typename              string                `json:"__typename"`
	ID                    string                `json:"id"`
	Shortcode             string                `json:"shortcode"`
	Dimensions            Dimensions            `json:"dimensions"`
	DisplayResources      []DisplayResources    `json:"display_resources"`
	IsVideo               bool                  `json:"is_video"`
	Title                 string                `json:"title"`
	VideoURL              string                `json:"video_url"`
	Owner                 Owner                 `json:"owner"`
	DisplayURL            string                `json:"display_url"`
	EdgeMediaToCaption    EdgeMediaToCaption    `json:"edge_media_to_caption"`
	EdgeSidecarToChildren EdgeSidecarToChildren `json:"edge_sidecar_to_children"`
	CoauthorProducers     *[]CoauthorProducers  `json:"coauthor_producers"`
}

type Owner struct {
	Username string `json:"username"`
}

type CoauthorProducers struct {
	Username string `json:"username"`
}

type Dimensions struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type DisplayResources struct {
	ConfigWidth  int    `json:"config_width"`
	ConfigHeight int    `json:"config_height"`
	Src          string `json:"src"`
}

type EdgeMediaToCaption struct {
	Edges []Edges `json:"edges"`
}
type Edges struct {
	Node struct {
		Typename         string             `json:"__typename"`
		Text             string             `json:"text"`
		ID               string             `json:"id"`
		Shortcode        string             `json:"shortcode"`
		CommenterCount   int                `json:"commenter_count"`
		Dimensions       Dimensions         `json:"dimensions"`
		DisplayResources []DisplayResources `json:"display_resources"`
		IsVideo          bool               `json:"is_video"`
		VideoURL         string             `json:"video_url,omitempty"`
		DisplayURL       string             `json:"display_url"`
	} `json:"node"`
}

type EdgeSidecarToChildren struct {
	Edges []Edges `json:"edges"`
}

type StoriesData struct {
	URL string `json:"url"`
}
