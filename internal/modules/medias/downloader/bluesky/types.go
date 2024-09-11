package bluesky

type BlueskyData *struct {
	Thread Thread `json:"thread"`
}

type Thread struct {
	Post Post `json:"post"`
}

type Post struct {
	URI    string `json:"uri"`
	Cid    string `json:"cid"`
	Author Author `json:"author"`
	Record Record `json:"record"`
	Embed  Embed  `json:"embed"`
}

type Author struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"displayName"`
}

type Record struct {
	Text string `json:"text"`
}

type Embed struct {
	Type        string  `json:"$type"`
	Images      []Image `json:"images"`
	Playlist    string  `json:"playlist"`
	Thumbnail   string  `json:"thumbnail"`
	AspectRatio struct {
		Height int `json:"height"`
		Width  int `json:"width"`
	} `json:"aspectRatio"`
}

type Image struct {
	Thumb       string      `json:"thumb"`
	Fullsize    string      `json:"fullsize"`
	Alt         string      `json:"alt"`
	AspectRatio AspectRatio `json:"aspectRatio"`
}

type AspectRatio struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}
