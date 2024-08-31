package threads

type ThreadsData *struct {
	Data ThreadsDataData `json:"data"`
}

type ThreadsDataData struct {
	Data ThreadsDataDataData `json:"data"`
}

type ThreadsDataDataData struct {
	Edges []ThreadsDataEdge `json:"edges"`
}

type ThreadsDataEdge struct {
	Node ThreadsDataNode `json:"node"`
}

type ThreadsDataNode struct {
	ThreadItems []ThreadItem `json:"thread_items"`
}

type ThreadItem struct {
	Post                 Post   `json:"post"`
	LineType             string `json:"line_type"`
	ShouldShowRepliesCta bool   `json:"should_show_replies_cta"`
}

type Post struct {
	User          User            `json:"user"`
	ID            string          `json:"id"`
	Code          string          `json:"code"`
	CarouselMedia []CarouselMedia `json:"carousel_media"`
	Caption       Caption         `json:"caption"`
}

type User struct {
	Username string `json:"username"`
}

type CarouselMedia struct {
	OriginalHeight int             `json:"original_height"`
	OriginalWidth  int             `json:"original_width"`
	ImageVersions  *ImageVersions  `json:"image_versions2"`
	VideoVersions  []VideoVersions `json:"video_versions"`
}

type VideoVersions struct {
	URL string `json:"url"`
}

type ImageVersions struct {
	Candidates []ImageCandidate `json:"candidates"`
}

type ImageCandidate struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
	Width  int    `json:"width"`
}

type Caption struct {
	Text string `json:"text"`
}
