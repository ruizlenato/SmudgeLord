package xiaohongshu

type Handler struct {
	username string
	postID   string
}

type XiaohongshuData *struct {
	Note struct {
		NoteDetailMap map[string]Note `json:"noteDetailMap"`
	} `json:"note"`
}

type Note struct {
	Note struct {
		Desc  string `json:"desc"`
		Video struct {
			Media struct {
				Stream VideoStream `json:"stream"`
			} `json:"media"`
			Image struct {
				FirstFrameFileid string `json:"firstFrameFileid"`
				ThumbnailFileid  string `json:"thumbnailFileid"`
			} `json:"image"`
		} `json:"video"`
		Type      string   `json:"type"`
		ImageList []Images `json:"imageList"`
		Title     string   `json:"title"`
		User      struct {
			Nickname string `json:"nickname"`
		} `json:"user"`
	} `json:"note"`
}

type Images struct {
	URLDefault string      `json:"urlDefault"`
	Height     int         `json:"height"`
	Width      int         `json:"width"`
	LivePhoto  bool        `json:"livePhoto"`
	Stream     VideoStream `json:"stream"`
}

type VideoStream struct {
	H264 []VideoInfo `json:"h264"`
	H265 []VideoInfo `json:"h265"`
	H266 []VideoInfo `json:"h266"`
	Av1  []VideoInfo `json:"av1"`
}

type VideoInfo *struct {
	Duration  int    `json:"duration"`
	Height    int    `json:"height"`
	Width     int    `json:"width"`
	MasterURL string `json:"masterUrl"`
}
