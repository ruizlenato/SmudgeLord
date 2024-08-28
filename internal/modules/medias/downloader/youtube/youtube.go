package youtube

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/kkdai/youtube/v2"
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

func downloadStream(youtubeClient *youtube.Client, video *youtube.Video, format *youtube.Format, outputFile *os.File) error {
	stream, _, err := youtubeClient.GetStream(video, format)
	if err != nil {
		log.Println("[youtube/Downloader] Error getting stream: ", err)
		return err
	}
	defer stream.Close()

	_, err = io.Copy(outputFile, stream)
	if err != nil {
		os.Remove(outputFile.Name())
		log.Println("[youtube/Downloader] Error copying stream to file: ", err)
		return err
	}

	return nil
}

func Downloader(callbackData []string) (*os.File, *youtube.Video, error) {
	youtubeClient := youtube.Client{}
	var client *http.Client
	if config.Socks5Proxy != "" {
		proxyURL, _ := url.Parse(config.Socks5Proxy)
		client = &http.Client{Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}}
		youtubeClient = youtube.Client{HTTPClient: client}
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

	var outputFile *os.File
	switch callbackData[0] {
	case "_aud":
		outputFile, err = os.CreateTemp("", "SmudgeYoutube_*.m4a")
	case "_vid":
		outputFile, err = os.CreateTemp("", "SmudgeYoutube_*.mp4")
	}
	if err != nil {
		log.Println("[youtube/Downloader] Error creating temporary file: ", err)
		return nil, video, err
	}

	err = downloadStream(&youtubeClient, video, format, outputFile)
	if err != nil {
		return nil, video, err
	}

	if callbackData[0] == "_vid" {
		audioFormat, err := getVideoFormat(video, 140)
		if err != nil {
			return nil, video, err
		}
		audioFile, err := os.CreateTemp("", "SmudgeYoutube_*.m4a")
		if err != nil {
			log.Println("[youtube/Downloader] Error creating temporary audio file: ", err)
			return nil, video, err
		}

		err = downloadStream(&youtubeClient, video, audioFormat, audioFile)
		if err != nil {
			return nil, video, err
		}

		outputFile = downloader.MergeAudioVideo(outputFile, audioFile)
	}

	_, err = outputFile.Seek(0, 0)
	if err != nil {
		return nil, video, err
	}

	return outputFile, video, nil
}
