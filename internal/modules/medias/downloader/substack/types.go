package substack

type Handler struct {
	postID string
}

type APIData *struct {
	Item struct {
		EntityKey string `json:"entity_key"`
		Comment   struct {
			ID          int64        `json:"id"`
			Body        string       `json:"body"`
			Name        string       `json:"name"`
			Handle      string       `json:"handle"`
			Attachments []Attachment `json:"attachments"`
		} `json:"comment"`
	} `json:"item"`
}

type Attachment struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	ImageURL string `json:"imageUrl"`
}
