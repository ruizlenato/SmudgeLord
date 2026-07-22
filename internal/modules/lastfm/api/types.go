package lastfmapi

type LastFM struct {
	apiKey string
}

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
	Artist        artist  `json:"artist"`
	Album         album   `json:"album"`
	Image         []image `json:"image"`
	UserPlaycount string  `json:"userplaycount"`
	Loved         string  `json:"loved"`
}

type recentTracks struct {
	RecentTracks *struct {
		Track *[]track `json:"track"`
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

type topAttr struct {
	Page       string `json:"page"`
	PerPage    string `json:"perPage"`
	Total      string `json:"total"`
	TotalPages string `json:"totalPages"`
}

type topItemArtist struct {
	Name string `json:"name"`
}

type topAlbum struct {
	Name      string        `json:"name"`
	Playcount string        `json:"playcount"`
	Artist    topItemArtist `json:"artist"`
	Image     []image       `json:"image"`
}

type topArtist struct {
	Name      string  `json:"name"`
	Playcount string  `json:"playcount"`
	Image     []image `json:"image"`
}

type topTrack struct {
	Name      string        `json:"name"`
	Playcount string        `json:"playcount"`
	Artist    topItemArtist `json:"artist"`
	Image     []image       `json:"image"`
}

type topAlbumsResponse struct {
	TopAlbums *struct {
		Albums []topAlbum `json:"album"`
		Attr   topAttr    `json:"@attr"`
	} `json:"topalbums"`
}

type topArtistsResponse struct {
	TopArtists *struct {
		Artists []topArtist `json:"artist"`
		Attr    topAttr     `json:"@attr"`
	} `json:"topartists"`
}

type topTracksResponse struct {
	TopTracks *struct {
		Tracks []topTrack `json:"track"`
		Attr   topAttr    `json:"@attr"`
	} `json:"toptracks"`
}

type TopCollageItem struct {
	Title     string
	Subtitle  string
	Playcount int
	ImageURL  string
}

type weeklyItemArtist struct {
	Text string `json:"#text"`
	Name string `json:"name"`
}

type weeklyAlbum struct {
	Name      string           `json:"name"`
	MBID      string           `json:"mbid"`
	Playcount string           `json:"playcount"`
	Artist    weeklyItemArtist `json:"artist"`
	Image     []image          `json:"image"`
}

type weeklyArtist struct {
	Name      string  `json:"name"`
	Playcount string  `json:"playcount"`
	Image     []image `json:"image"`
}

type weeklyTrack struct {
	Name      string           `json:"name"`
	Playcount string           `json:"playcount"`
	Artist    weeklyItemArtist `json:"artist"`
	Image     []image          `json:"image"`
}

type weeklyAlbumsResponse struct {
	WeeklyAlbumChart *struct {
		Albums []weeklyAlbum `json:"album"`
	} `json:"weeklyalbumchart"`
}

type weeklyArtistsResponse struct {
	WeeklyArtistChart *struct {
		Artists []weeklyArtist `json:"artist"`
	} `json:"weeklyartistchart"`
}

type weeklyTracksResponse struct {
	WeeklyTrackChart *struct {
		Tracks []weeklyTrack `json:"track"`
	} `json:"weeklytrackchart"`
}
