package downloader

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

var GenericHeaders = map[string]string{
	"Accept":             "*/*",
	"Accept-Language":    "en",
	"User-Agent":         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Sec-Ch-UA":          `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`,
	"Sec-Ch-UA-Mobile":   "?0",
	"Sec-Ch-UA-Platform": `"Windows"`,
}

func CloneHeaders(src map[string]string) map[string]string {
	cloned := make(map[string]string, len(src))
	for k, v := range src {
		cloned[k] = v
	}
	return cloned
}

type Medias struct {
	Caption       string   `json:"caption"`
	Medias        []string `json:"medias"`
	InvertMedia   bool     `json:"invert_media"`
	Article       string   `json:"article,omitempty"`
	ArticleMedias []string `json:"article_medias,omitempty"`
}

type PostInfo struct {
	Medias            []gotgbot.InputMedia
	ID                string
	Caption           string
	Service           string
	InvertMedia       bool
	Article           *ArticleContent
	NoMedia           bool
	Unavailable       bool
	UnavailableReason string
	FileTooLarge      bool
	Cleanup           func()
}

type Options struct {
	BotPremium bool
}

type ArticleContent struct {
	HTML  string
	Media []gotgbot.InputRichMessageMedia
}

func RichMediaHTMLTag(id string, media gotgbot.InputMedia) string {
	switch media.(type) {
	case *gotgbot.InputMediaPhoto:
		return fmt.Sprintf(`<img src="tg://photo?id=%s"></img>`, id)
	case *gotgbot.InputMediaVideo:
		return fmt.Sprintf(`<video src="tg://video?id=%s"></video>`, id)
	}
	return ""
}

func RichMessageMediaFromInputMedia(id string, media gotgbot.InputMedia) gotgbot.InputRichMessageMedia {
	switch v := media.(type) {
	case *gotgbot.InputMediaPhoto:
		return gotgbot.InputRichMessageMedia{
			Id: id,
			Media: &gotgbot.InputMediaPhoto{
				Media: v.Media,
			},
		}
	case *gotgbot.InputMediaVideo:
		return gotgbot.InputRichMessageMedia{
			Id: id,
			Media: &gotgbot.InputMediaVideo{
				Media:     v.Media,
				Thumbnail: v.Thumbnail,
				Width:     v.Width,
				Height:    v.Height,
				Duration:  v.Duration,
			},
		}
	}
	return gotgbot.InputRichMessageMedia{Id: id, Media: media}
}

func AppendRichMedia(sb *strings.Builder, medias []gotgbot.InputMedia, startID int) []gotgbot.InputRichMessageMedia {
	mediaList := make([]gotgbot.InputRichMessageMedia, 0, len(medias))
	var photoRun []string
	flushPhotos := func() {
		if len(photoRun) > 1 {
			sb.WriteString("<tg-collage>\n")
			for _, tag := range photoRun {
				sb.WriteString(tag)
				sb.WriteString("\n")
			}
			sb.WriteString("</tg-collage>\n")
		} else {
			for _, tag := range photoRun {
				sb.WriteString(tag)
				sb.WriteString("\n")
			}
		}
		photoRun = photoRun[:0]
	}
	for i, media := range medias {
		id := strconv.Itoa(startID + i)
		mediaList = append(mediaList, RichMessageMediaFromInputMedia(id, media))
		tag := RichMediaHTMLTag(id, media)
		if _, ok := media.(*gotgbot.InputMediaPhoto); ok {
			photoRun = append(photoRun, tag)
		} else {
			flushPhotos()
			sb.WriteString(tag)
			sb.WriteString("\n")
		}
	}
	flushPhotos()
	return mediaList
}

func NewNoMediaPostInfo(id string) PostInfo {
	return PostInfo{ID: id, NoMedia: true}
}

func NewUnavailablePostInfo(id string) PostInfo {
	return PostInfo{ID: id, NoMedia: true, Unavailable: true}
}

func NewUnavailablePostInfoWithReason(id, reason string) PostInfo {
	return PostInfo{ID: id, NoMedia: true, Unavailable: true, UnavailableReason: reason}
}

func NewFileTooLargePostInfo(id string) PostInfo {
	return PostInfo{ID: id, NoMedia: true, FileTooLarge: true}
}

type YouTube struct {
	Video   string `json:"video"`
	Audio   string `json:"audio"`
	Caption string `json:"caption"`
}

type InputMedia struct {
	File      []byte
	Thumbnail []byte
}

type FileTooLargeError struct {
	Size    int64
	MaxSize int64
}

func (e *FileTooLargeError) Error() string {
	return fmt.Sprintf("file size %d exceeds maximum allowed size %d", e.Size, e.MaxSize)
}

func NewFileTooLargeError(size, maxSize int64) *FileTooLargeError {
	return &FileTooLargeError{Size: size, MaxSize: maxSize}
}

func InputFileFromBytes(filename string, data []byte) gotgbot.InputFile {
	return gotgbot.InputFileByReader(filename, bytes.NewReader(data))
}

func InputFileFromReader(filename string, r io.Reader) gotgbot.InputFile {
	return gotgbot.InputFileByReader(filename, r)
}

// CombineCleanups returns a single cleanup function that calls all provided
// cleanup functions. Nil cleanups are safely skipped.
func CombineCleanups(cleanups ...func()) func() {
	return func() {
		for _, cleanup := range cleanups {
			if cleanup != nil {
				cleanup()
			}
		}
	}
}
