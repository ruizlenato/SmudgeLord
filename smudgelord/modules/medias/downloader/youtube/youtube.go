package youtube

import (
	"errors"
	"io"
	"log"
	"os"
	"strconv"

	"smudgelord/smudgelord/config"
	"smudgelord/smudgelord/modules/medias/downloader"

	"github.com/kkdai/youtube/v2"
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
	video, err := youtubeClient.GetVideo(callbackData[1])
	if err != nil {
		return nil, video, err
	}

	itag, _ := strconv.Atoi(callbackData[2])
	format, err := getVideoFormat(video, itag)
	if err != nil {
		return nil, video, err
	}

	sizeLimit := int64(1572864000) // 1.5 GB
	if config.BotAPIURL == "" {
		sizeLimit = 52428800 // 50 MB
	}

	mediaSize := format.ContentLength
	if callbackData[0] == "_vid" {
		audioFormat, err := getVideoFormat(video, 140)
		if err != nil {
			return nil, video, err
		}
		mediaSize += audioFormat.ContentLength
	}

	if mediaSize > sizeLimit {
		return nil, video, errors.New("file size is too large")
	}

	var outputFile *os.File
	switch callbackData[0] {
	case "_aud":
		outputFile, err = os.CreateTemp("", "youtubeSmudge_*.m4a")
	case "_vid":
		outputFile, err = os.CreateTemp("", "youtubeSmudge_*.mp4")
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
		audioFile, err := os.CreateTemp("", "youtubeSmudge_*.m4a")
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

	return outputFile, video, nil
}
