package youtube

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/kkdai/youtube/v2"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
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
			time.Sleep(3 * time.Second)
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
		slog.Error("Could't download video, all attempts failed",
			"Error", err.Error())
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
	filename := fmt.Sprintf("SmudgeLord-YouTube_%s_%s", utils.SanitizeString(video.Author), utils.SanitizeString(video.Title))
	if len(filename) > 255 {
		filename = filename[:255]
	}
	switch callbackData[0] {
	case "_aud":
		outputFile, err = os.Create(filepath.Join(os.TempDir(), filename+".m4a"))
	case "_vid":
		outputFile, err = os.Create(filepath.Join(os.TempDir(), filename+".mp4"))
	}
	if err != nil {
		slog.Error("Couldn't create temporary file", "Error", err.Error())
		return nil, video, err
	}

	defer func() {
		if err != nil {
			outputFile.Close()
			os.Remove(outputFile.Name())
		}
	}()

	err = downloadStream(&youtubeClient, video, format, outputFile)
	if err != nil {
		return nil, video, err
	}

	if callbackData[0] == "_vid" {
		err = downloadAndMergeAudio(&youtubeClient, video, outputFile)
		if err != nil {
			return nil, video, err
		}
	}

	outputFile.Seek(0, 0)
	return outputFile, video, nil
}

func downloadAndMergeAudio(youtubeClient *youtube.Client, video *youtube.Video, videoFile *os.File) error {
	audioFormat, err := getVideoFormat(video, 140)
	if err != nil {
		return err
	}

	audioFile, err := os.CreateTemp("", "SmudgeYoutube_*.m4a")
	if err != nil {
		return err
	}
	defer audioFile.Close()

	err = downloadStream(youtubeClient, video, audioFormat, audioFile)
	if err != nil {
		return err
	}

	return downloader.MergeAudioVideo(videoFile, audioFile)
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

func Handle(videoURL string) ([]models.InputMedia, []string) {
	youtubeClient := configureYoutubeClient()
	video, err := youtubeClient.GetVideo(videoURL)
	if err != nil {
		if strings.Contains(err.Error(), "can't bypass age restriction") {
			slog.Warn("Age restricted video",
				"URL", videoURL)
			return nil, []string{}
		}
		slog.Error("Couldn't get video",
			"URL", videoURL,
			"Error", err.Error())
		return nil, []string{}
	}

	videoStream := GetBestQualityVideoStream(video.Formats.Type("video/mp4"))

	format, err := getVideoFormat(video, videoStream.ItagNo)
	if err != nil {
		return nil, []string{}
	}

	outputFile, err := os.Create(filepath.Join(os.TempDir(), fmt.Sprintf("SmudgeLord-YouTube_%s_%s.mp4", video.Author, video.Title)))
	if err != nil {
		slog.Error("Couldn't create temporary file",
			"Error", err.Error())
		return nil, []string{}
	}

	if err = downloadStream(&youtubeClient, video, format, outputFile); err != nil {
		return nil, []string{}
	}

	err = downloadAndMergeAudio(&youtubeClient, video, outputFile)
	if err != nil {
		slog.Error("Could't merge audio and video")
		return nil, []string{}
	}

	return []models.InputMedia{&models.InputMediaVideo{
		Media:             "attach://" + outputFile.Name(),
		Width:             video.Formats.Itag(videoStream.ItagNo)[0].Width,
		Height:            video.Formats.Itag(videoStream.ItagNo)[0].Height,
		SupportsStreaming: true,
		MediaAttachment:   outputFile,
	}}, []string{fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title), video.ID}
}
