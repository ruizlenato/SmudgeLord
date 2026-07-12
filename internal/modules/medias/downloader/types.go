package downloader

import (
	"bytes"
	"fmt"
	"io"

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
	Caption     string   `json:"caption"`
	Medias      []string `json:"medias"`
	InvertMedia bool     `json:"invert_media"`
}

type PostInfo struct {
	Medias            []gotgbot.InputMedia
	ID                string
	Caption           string
	Service           string
	InvertMedia       bool
	NoMedia           bool
	Unavailable       bool
	UnavailableReason string
	FileTooLarge      bool
	Cleanup           func()
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
