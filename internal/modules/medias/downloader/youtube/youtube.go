package youtube

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kkdai/youtube/v2"
	"github.com/mymmrac/telego"
	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
)

func getVideoFormat(video *youtube.Video, itag int) (*youtube.Format, error) {
	formats := video.Formats.Itag(itag)
	if len(formats) == 0 {
		return nil, errors.New("invalid itag")
	}
	return &formats[0], nil
}

func configureYoutubeClient() youtube.Client {
	if config.Socks5Proxy != "" {
		proxyURL, _ := url.Parse(config.Socks5Proxy)
		client := &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyURL(proxyURL),
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
			},
			Timeout: 120 * time.Second,
		}
		resp, err := client.Get("https://www.google.com")
		if err == nil && resp.StatusCode == http.StatusOK {
			return youtube.Client{HTTPClient: client}
		}
		slog.Warn("Proxy connection failed, falling back to direct connection")
	}
	return youtube.Client{}
}

func copyStreamWithRetries(youtubeClient *youtube.Client, video *youtube.Video, format *youtube.Format, outputFile *os.File) error {
	for attempt := 1; attempt <= 5; attempt++ {
		stream, _, err := youtubeClient.GetStream(video, format)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		_, err = io.Copy(outputFile, stream)
		stream.Close()

		if err == nil {
			return nil
		}

		outputFile.Seek(0, 0)
		outputFile.Truncate(0)
		time.Sleep(2 * time.Second)
	}

	os.Remove(outputFile.Name())
	return errors.New("YouTube — Failed after 5 attempts")
}

func downloadStream(youtubeClient *youtube.Client, video *youtube.Video, format *youtube.Format, outputFile *os.File) error {
	err := copyStreamWithRetries(youtubeClient, video, format, outputFile)
	if err != nil {
		slog.Error("Could't download video, all attempts failed", "Error", err.Error())
		return err
	}
	return nil
}

func Downloader(callbackData []string) (*os.File, *youtube.Video, error) {
	youtubeClient := configureYoutubeClient()
	video, err := youtubeClient.GetVideo(callbackData[1])
	if err != nil {
		return nil, video, err
	}

	itag, _ := strconv.Atoi(callbackData[2])
	format, err := getVideoFormat(video, itag)
	if err != nil {
		return nil, video, err
	}

	var outputFile *os.File
	switch callbackData[0] {
	case "_aud":
		outputFile, err = os.CreateTemp("", "SmudgeYoutube_*.m4a")
	case "_vid":
		outputFile, err = os.CreateTemp("", "SmudgeYoutube_*.mp4")
	}
	if err != nil {
		slog.Error("Could't create temporary file", "Error", err.Error())
		return nil, video, err
	}

	err = downloadStream(&youtubeClient, video, format, outputFile)
	if err != nil {
		return nil, video, err
	}

	if callbackData[0] == "_vid" {
		err, _ = downloadAndMergeAudio(&youtubeClient, video, outputFile)
		if err != nil {
			return nil, video, err
		}
	}

	_, err = outputFile.Seek(0, 0)
	if err != nil {
		return nil, video, err
	}

	return outputFile, video, nil
}

func downloadAndMergeAudio(youtubeClient *youtube.Client, video *youtube.Video, videoFile *os.File) (error, *os.File) {
	audioFormat, err := getVideoFormat(video, 140)
	if err != nil {
		return err, nil
	}

	audioFile, err := os.CreateTemp("", "SmudgeYoutube_*.m4a")
	if err != nil {
		return err, nil
	}
	defer audioFile.Close()

	err = downloadStream(youtubeClient, video, audioFormat, audioFile)
	if err != nil {
		return err, nil
	}

	return nil, downloader.MergeAudioVideo(videoFile, audioFile)
}

func GetBestQualityVideoStream(formats []youtube.Format) youtube.Format {
	var bestFormat youtube.Format
	var maxBitrate int

	isDesiredQuality := func(qualityLabel string) bool {
		supportedQualities := []string{"1080p", "720p", "480p", "360p", "240p", "144p"}
		for _, supported := range supportedQualities {
			if strings.Contains(qualityLabel, supported) {
				return true
			}
		}
		return false
	}

	for _, format := range formats {
		if format.Bitrate > maxBitrate && isDesiredQuality(format.QualityLabel) {
			maxBitrate = format.Bitrate
			bestFormat = format
		}
	}

	return bestFormat
}

func Handle(videoURL string) ([]telego.InputMedia, []string) {
	youtubeClient := configureYoutubeClient()
	video, err := youtubeClient.GetVideo(videoURL)
	if err != nil {
		return nil, []string{}
	}

	videoStream := GetBestQualityVideoStream(video.Formats.Type("video/mp4"))

	format, err := getVideoFormat(video, videoStream.ItagNo)
	if err != nil {
		return nil, []string{}
	}

	outputFile, err := os.CreateTemp("", "SmudgeYoutube_*.mp4")
	if err != nil {
		slog.Error("Could't create temporary file", "Error", err.Error())
		return nil, []string{}
	}

	if err = downloadStream(&youtubeClient, video, format, outputFile); err != nil {
		return nil, []string{}
	}

	err, outputFile = downloadAndMergeAudio(&youtubeClient, video, outputFile)
	if err != nil {
		return nil, []string{}
	}

	return []telego.InputMedia{&telego.InputMediaVideo{
		Type:              telego.MediaTypeVideo,
		Media:             telego.InputFile{File: outputFile},
		Width:             video.Formats.Itag(videoStream.ItagNo)[0].Width,
		Height:            video.Formats.Itag(videoStream.ItagNo)[0].Height,
		SupportsStreaming: true,
	}}, []string{fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title), video.ID}
}
