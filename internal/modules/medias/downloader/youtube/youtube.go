package youtube

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/kkdai/youtube/v2"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func getVideoFormat(video *youtube.Video, itag int) (*youtube.Format, error) {
	formats := video.Formats.Itag(itag)
	if len(formats) == 0 {
		return nil, errors.New("invalid itag")
	}

	formatsWithAudio := formats.WithAudioChannels()
	if len(formatsWithAudio) > 0 {
		for _, format := range formats.WithAudioChannels() {
			audioTrack := &format.AudioTrack
			if (*audioTrack) != nil && (*audioTrack).AudioIsDefault {
				return &format, nil
			}
		}
		return &formatsWithAudio[0], nil
	}

	return &formats[0], nil
}

func ConfigureYoutubeClient() youtube.Client {
	if config.Socks5Proxy != "" {
		proxyURL, parseErr := url.Parse(config.Socks5Proxy)
		if parseErr != nil {
			slog.Error(
				"Couldn't parse proxy URL",
				"Proxy", config.Socks5Proxy,
				"Error", parseErr.Error(),
			)
			return youtube.Client{}
		}
		httpClient := &http.Client{
			Transport: &http.Transport{
				Proxy:                 http.ProxyURL(proxyURL),
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
			},
			Timeout: 120 * time.Second,
		}
		return youtube.Client{HTTPClient: httpClient}
	}

	return youtube.Client{}
}

func downloadStream(youtubeClient youtube.Client, video *youtube.Video, format *youtube.Format) ([]byte, error) {
	for attempt := 1; attempt <= 5; attempt++ {
		stream, _, err := youtubeClient.GetStream(video, format)
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		defer stream.Close()

		var buf strings.Builder
		if _, err := io.Copy(&buf, stream); err == nil {
			return []byte(buf.String()), nil
		}

		time.Sleep(2 * time.Second)
	}

	return nil, errors.New("YouTube — Failed after 5 attempts")
}

func Downloader(callbackData []string) ([]byte, *youtube.Video, string, error) {
	youtubeClient := ConfigureYoutubeClient()
	video, err := youtubeClient.GetVideo(callbackData[1])
	if err != nil {
		return nil, video, "", err
	}

	itag, _ := strconv.Atoi(callbackData[2])
	format, err := getVideoFormat(video, itag)
	if err != nil {
		return nil, video, "", err
	}

	fileBytes, err := downloadStream(youtubeClient, video, format)
	if err != nil {
		slog.Error(
			"Could't download video, all attempts failed",
			"Error", err.Error(),
		)
		return nil, video, "", err
	}

	if callbackData[0] == "_vid" {
		audioFormat, err := getVideoFormat(video, 140)
		if err != nil {
			return nil, video, "", err
		}
		audioBytes, err := downloadStream(youtubeClient, video, audioFormat)
		if err != nil {
			return nil, video, "", err
		}
		fileBytes, err = downloader.MergeAudioVideo(fileBytes, audioBytes)
		if err != nil {
			slog.Error(
				"Couldn't merge audio and video",
				"Error", err.Error(),
			)
			return nil, video, "", err
		}
	}

	return fileBytes, video, format.MimeType, nil
}

func GetBestQualityVideoStream(formats []youtube.Format) *youtube.Format {
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

	return &bestFormat
}

func Handle(message string) downloader.PostInfo {
	youtubeClient := ConfigureYoutubeClient()
	video, err := youtubeClient.GetVideo(message)
	if err != nil {
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(video.ID); err == nil {
		return postInfo
	}

	videoStream := GetBestQualityVideoStream(video.Formats.Type("video/mp4"))

	format, err := getVideoFormat(video, videoStream.ItagNo)
	if err != nil {
		return downloader.PostInfo{}
	}

	fileBytes, err := downloadStream(youtubeClient, video, format)
	if err != nil {
		slog.Error(
			"Couldn't download video stream",
			"Error", err.Error(),
		)
		return downloader.PostInfo{}
	}

	audioFormat, err := getVideoFormat(video, 140)
	if err != nil {
		slog.Error(
			"Couldn't get audio format",
			"Error", err.Error(),
		)
		return downloader.PostInfo{}
	}

	audioBytes, err := downloadStream(youtubeClient, video, audioFormat)
	if err != nil {
		slog.Error(
			"Couldn't download audio stream",
			"Error", err.Error(),
		)
		return downloader.PostInfo{}
	}

	fileBytes, err = downloader.MergeAudioVideo(fileBytes, audioBytes)
	if err != nil {
		slog.Error(
			"Couldn't merge audio and video",
			"Error", err.Error(),
		)
		return downloader.PostInfo{}
	}

	videoFile, err := helpers.UploadVideo(helpers.UploadVideoParams{
		File:              fileBytes,
		Filename:          utils.SanitizeString(fmt.Sprintf("SmudgeLord-YouTube_%s_%s.mp4", video.Author, video.Title)),
		SupportsStreaming: true,
		Width:             int32(video.Formats.Itag(videoStream.ItagNo)[0].Width),
		Height:            int32(video.Formats.Itag(videoStream.ItagNo)[0].Height),
	})
	if err != nil {
		if !telegram.MatchError(err, "CHAT_WRITE_FORBIDDEN") {
			slog.Error(
				"Couldn't upload video",
				"Error", err.Error(),
			)
		}
		return downloader.PostInfo{}
	}

	return downloader.PostInfo{
		ID:      video.ID,
		Medias:  []telegram.InputMedia{&videoFile},
		Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
	}
}
