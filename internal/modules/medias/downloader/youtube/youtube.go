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

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/ruizlenato/youtubedl"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	maxRetries    = 5
	retryDelay    = 2 * time.Second
	maxRetryDelay = 30 * time.Second
)

func getVideoFormat(video *youtubedl.Video, itag int) (*youtubedl.Format, error) {
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

func ConfigureYoutubeClient() *youtubedl.Client {
	if len(config.Socks5Proxies) == 0 {
		ytClient, err := youtubedl.New()
		if err != nil {
			slog.Error("Couldn't create youtube-dl client", "Error", err.Error())
			return nil
		}
		return ytClient
	}

	var lastErr error
	for index, proxy := range config.Socks5Proxies {
		proxyURL, parseErr := url.Parse(proxy)
		if parseErr != nil {
			slog.Error("Couldn't parse proxy URL", "Proxy", proxy, "Index", index, "Error", parseErr.Error())
			lastErr = parseErr
			continue
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

		ytClient, err := utils.RetryWithBackoff(
			func() (*youtubedl.Client, error) {
				return youtubedl.New(youtubedl.WithHTTPClient(httpClient))
			},
			maxRetries,
			retryDelay,
			maxRetryDelay,
			2,
		)
		if err != nil {
			slog.Warn("Couldn't create youtube-dl client with proxy, trying next proxy",
				"Proxy", proxy,
				"Index", index,
				"Error", err.Error())
			lastErr = err
			continue
		}

		return ytClient
	}

	if lastErr != nil {
		slog.Error("Couldn't create youtube-dl client with configured proxies", "Error", lastErr.Error())
	}

	slog.Warn("Falling back to direct YouTube client without proxy")
	ytClient, err := youtubedl.New()
	if err != nil {
		slog.Error("Couldn't create direct youtube-dl client", "Error", err.Error())
		return nil
	}

	return ytClient
}

func downloadStream(youtubeClient *youtubedl.Client, video *youtubedl.Video, format *youtubedl.Format) ([]byte, error) {
	result, err := utils.RetryWithBackoff(
		func() ([]byte, error) {
			stream, _, err := youtubeClient.GetStream(video, format)
			if err != nil {
				return nil, err
			}
			defer stream.Close()

			var buf strings.Builder
			if _, err := io.Copy(&buf, stream); err != nil {
				return nil, err
			}
			return []byte(buf.String()), nil
		},
		maxRetries,
		retryDelay,
		maxRetryDelay,
		2,
	)

	if err != nil {
		return nil, errors.New("YouTube — Failed after max retries")
	}
	return result, nil
}

func Downloader(callbackData []string) ([]byte, *youtubedl.Video, error) {
	youtubeClient := ConfigureYoutubeClient()
	if youtubeClient == nil {
		return nil, nil, errors.New("YouTube client unavailable")
	}

	video, err := youtubeClient.GetVideo(callbackData[1])
	if err != nil {
		return nil, video, err
	}

	itag, _ := strconv.Atoi(callbackData[2])
	format, err := getVideoFormat(video, itag)
	if err != nil {
		return nil, video, err
	}

	fileBytes, err := downloadStream(youtubeClient, video, format)
	if err != nil {
		slog.Error("Could't download video, all attempts failed",
			"Error", err.Error())
		return nil, video, err
	}

	if callbackData[0] == "_vid" {
		audioFormat, err := getVideoFormat(video, 140)
		if err != nil {
			return nil, video, err
		}
		audioBytes, err := downloadStream(youtubeClient, video, audioFormat)
		if err != nil {
			return nil, video, err
		}
		fileBytes, err = downloader.MergeAudioVideoBytes(fileBytes, audioBytes)
		if err != nil {
			slog.Error("Could't merge audio and video",
				"Error", err.Error())
			return nil, video, err
		}
	}

	return fileBytes, video, nil
}

func GetBestQualityVideoStream(formats []youtubedl.Format) *youtubedl.Format {
	var bestFormat youtubedl.Format
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

func Handle(videoURL string) downloader.PostInfo {
	youtubeClient := ConfigureYoutubeClient()
	if youtubeClient == nil {
		return downloader.PostInfo{}
	}
	video, err := youtubeClient.GetVideo(videoURL)
	if err != nil {
		if strings.Contains(err.Error(), "can't bypass age restriction") {
			slog.Warn("Age restricted video",
				"URL", videoURL)
			return downloader.PostInfo{}
		}
		slog.Error("Couldn't get video",
			"URL", videoURL,
			"Error", err.Error())
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
		slog.Error("Couldn't download video stream",
			"Error", err.Error())
		return downloader.PostInfo{}
	}

	audioFormat, err := getVideoFormat(video, 140)
	if err != nil {
		slog.Error("Couldn't get audio format",
			"Error", err.Error())
		return downloader.PostInfo{}
	}

	audioBytes, err := downloadStream(youtubeClient, video, audioFormat)
	if err != nil {
		slog.Error("Couldn't download audio stream",
			"Error", err.Error())
		return downloader.PostInfo{}
	}

	fileBytes, err = downloader.MergeAudioVideoBytes(fileBytes, audioBytes)
	if err != nil {
		slog.Error("Couldn't merge audio and video",
			"Error", err.Error())
		return downloader.PostInfo{}
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-YouTube_%s_%s.mp4", video.Author, video.Title))
	return downloader.PostInfo{
		ID: video.ID,
		Medias: []gotgbot.InputMedia{&gotgbot.InputMediaVideo{
			Media:             downloader.InputFileFromBytes(filename, fileBytes),
			Width:             int64(video.Formats.Itag(videoStream.ItagNo)[0].Width),
			Height:            int64(video.Formats.Itag(videoStream.ItagNo)[0].Height),
			SupportsStreaming: true,
		}},
		Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
	}

}
