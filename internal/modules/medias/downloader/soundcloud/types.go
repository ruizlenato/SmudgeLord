package soundcloud

type Handler struct {
	id       string
	clientID string
	format   string
}

type SoundCloudAPI struct {
	ID                 int64             `json:"id"`
	Title              string            `json:"title"`
	Duration           int               `json:"duration"`
	Policy             string            `json:"policy"`
	TrackAuthorization string            `json:"track_authorization"`
	ArtworkURL         string            `json:"artwork_url"`
	Genre              string            `json:"genre"`
	DisplayDate        string            `json:"display_date"`
	License            string            `json:"license"`
	User               User              `json:"user"`
	Media              Media             `json:"media"`
	PublisherMetadata  PublisherMetadata `json:"publisher_metadata"`
}

type User struct {
	Username string `json:"username"`
}

type Media struct {
	Transcodings []Transcoding `json:"transcodings"`
}

type Transcoding struct {
	URL                 string `json:"url"`
	Preset              string `json:"preset"`
	Snipped             bool   `json:"snipped"`
	Format              Format `json:"format"`
	IsLegacyTranscoding bool   `json:"is_legacy_transcoding"`
}

type Format struct {
	Protocol string `json:"protocol"`
	MimeType string `json:"mime_type"`
}

type PublisherMetadata struct {
	AlbumTitle     string `json:"album_title"`
	WriterComposer string `json:"writer_composer"`
}

type StreamResponse struct {
	URL string `json:"url"`
}
