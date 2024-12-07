package youtube

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"

	"github.com/kkdai/youtube/v2"
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
				Proxy: http.ProxyURL(proxyURL),
			},
		}
		return youtube.Client{HTTPClient: client}
	}
	return youtube.Client{}
}

func downloadStream(youtubeClient *youtube.Client, video *youtube.Video, format *youtube.Format, outputFile *os.File) error {
	stream, _, err := youtubeClient.GetStream(video, format)
	if err != nil {
		log.Println("YouTube/Downloader — Error getting stream: ", err)
		return err
	}
	defer stream.Close()

	_, err = io.Copy(outputFile, stream)
	if err != nil {
		os.Remove(outputFile.Name())
		log.Println("YouTube/Downloader — Error copying stream to file: ", err)
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
		log.Println("YouTube/Downloader — Error creating temporary file: ", err)
		return nil, video, err
	}

	err = downloadStream(&youtubeClient, video, format, outputFile)
	if err != nil {
		return nil, video, err
	}

	if callbackData[0] == "_vid" {
		err, outputFile = downloadAndMergeAudio(&youtubeClient, video, outputFile)
		if err != nil {
			return nil, video, err
		}
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

func Handle(message *telegram.NewMessage) ([]telegram.InputMedia, []string) {
	youtubeClient := youtube.Client{}
	video, err := youtubeClient.GetVideo(message.Text())
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
		log.Println("YouTube — error creating temporary file: ", err)
		return nil, []string{}
	}

	if err = downloadStream(&youtubeClient, video, format, outputFile); err != nil {
		return nil, []string{}
	}

	err, outputFile = downloadAndMergeAudio(&youtubeClient, video, outputFile)
	if err != nil {
		return nil, []string{}
	}

	videoFile, err := helpers.UploadDocument(message, helpers.UploadDocumentParams{
		File: outputFile.Name(),
		Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
			SupportsStreaming: true,
			W:                 int32(video.Formats.Itag(videoStream.ItagNo)[0].Width),
			H:                 int32(video.Formats.Itag(videoStream.ItagNo)[0].Height),
		}},
	})
	if err != nil {
		fmt.Println("YouTube — Error uploading video: ", err)
	}

	return []telegram.InputMedia{&videoFile}, []string{fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title), video.ID}
}
