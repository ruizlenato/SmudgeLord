package lastfm

type image struct {
	Size string `json:"size"`
	Text string `json:"#text"`
}

type artist struct {
	URL   string  `json:"url"`
	Name  string  `json:"name"`
	Image []image `json:"image"`
	Stats struct {
		Userplaycount string `json:"userplaycount"`
	} `json:"stats"`
}

type album struct {
	Artist        string  `json:"artist"`
	Text          string  `json:"#text"`
	Name          string  `json:"name"`
	Image         []image `json:"image"`
	Userplaycount int     `json:"userplaycount"`
}

type track struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Attr struct {
		Nowplaying string `json:"nowplaying"`
	} `json:"@attr,omitempty"`
	Artist        artist   `json:"artist"`
	Album         album    `json:"album"`
	Image         *[]image `json:"image"`
	UserPlaycount string   `json:"userplaycount"`
	Loved         string   `json:"loved"`
}

type recentTracks struct {
	RecentTracks struct {
		Track []track `json:"track"`
		Attr  struct {
			User  *string `json:"user"`
			Total string  `json:"total"`
		} `json:"@attr"`
	} `json:"recenttracks"`
}

type userInfo struct {
	User *struct {
		Name        string  `json:"name"`
		Realname    string  `json:"realname"`
		Playcount   string  `json:"playcount"`
		ArtistCount string  `json:"artist_count"`
		TrackCount  string  `json:"track_count"`
		AlbumCount  string  `json:"album_count"`
		Image       []image `json:"image"`
		Country     string  `json:"country"`
		URL         string  `json:"url"`
	} `json:"user"`
}

type getInfo struct {
	Track  track  `json:"track"`
	Album  album  `json:"album"`
	Artist artist `json:"artist"`
}
