package bluesky

type Handler struct {
	username string
	postID   string
}

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

type Media struct {
	Type      string  `json:"$type"`
	Images    []Image `json:"images"`
	Items     []Image `json:"items"`
	Playlist  string  `json:"playlist"`
	Thumbnail string  `json:"thumbnail"`
}

type AspectRatio struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type Embed struct {
	Type        string      `json:"$type"`
	Media       Media       `json:"media"`
	Images      []Image     `json:"images"`
	Items       []Image     `json:"items"`
	Playlist    *string     `json:"playlist"`
	Thumbnail   *string     `json:"thumbnail"`
	AspectRatio AspectRatio `json:"aspectRatio"`
	Record      *RecordView `json:"record"`
}

type RecordView struct {
	Type   string       `json:"$type"`
	Record *RecordView  `json:"record"`
	URI    string       `json:"uri"`
	Author Author       `json:"author"`
	Value  RecordValue  `json:"value"`
	Embeds []QuoteEmbed `json:"embeds"`
}

type RecordValue struct {
	Text string `json:"text"`
}

type QuoteEmbed struct {
	Type      string  `json:"$type"`
	Images    []Image `json:"images"`
	Items     []Image `json:"items"`
	Media     Media   `json:"media"`
	Playlist  string  `json:"playlist"`
	Thumbnail string  `json:"thumbnail"`
}

func (rv *RecordView) ViewRecord() *RecordView {
	if rv == nil {
		return nil
	}
	if rv.Record != nil {
		return rv.Record
	}
	return rv
}

type Image struct {
	Thumb       string      `json:"thumb"`
	Fullsize    string      `json:"fullsize"`
	Alt         string      `json:"alt"`
	AspectRatio AspectRatio `json:"aspectRatio"`
}
