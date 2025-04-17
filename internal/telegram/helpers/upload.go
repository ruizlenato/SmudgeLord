package helpers

import (
	"bytes"
	"errors"
	"fmt"
	"mime"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/gabriel-vasile/mimetype"
)

type Media struct {
	Message *telegram.NewMessage
	Client  *telegram.Client
}

func GetInputFile(message *telegram.NewMessage, file []byte, filename ...string) (telegram.InputFile, error) {
	if len(file) == 0 {
		return nil, errors.New("file data is required")
	}

	var fileName string
	if len(filename) > 0 {
		fileName = filename[0]
	}

	uploadedMedia, err := message.Client.UploadFile(file, &telegram.UploadOptions{
		FileName: fileName,
	})
	if err != nil {
		return nil, err
	}

	return uploadedMedia, nil
}

func (m *Media) GetInputFile(file []byte, filename ...string) (telegram.InputFile, error) {
	if len(file) == 0 {
		return nil, errors.New("file path is required")
	}

	var fileName string
	if len(filename) > 0 {
		fileName = filename[0]
	}

	uploadedMedia, err := m.Client.UploadFile(file, &telegram.UploadOptions{
		FileName: fileName,
	})
	if err != nil {
		return nil, err
	}

	return uploadedMedia, err
}

type UploadPhotoParams struct {
	Spoiler  bool
	File     []byte
	Filename string
}

func UploadPhoto(message *telegram.NewMessage, params UploadPhotoParams) (telegram.InputMediaPhoto, error) {
	if len(params.File) == 0 {
		return telegram.InputMediaPhoto{}, errors.New("file is required")
	}

	media := &Media{
		Message: message,
		Client:  message.Client,
	}

	if params.Filename == "" {
		mimeType, err := mimetype.DetectReader(bytes.NewReader(params.File))
		if err != nil {
			return telegram.InputMediaPhoto{}, fmt.Errorf("failed to detect mime type: %w", err)
		}
		params.Filename = fmt.Sprintf("photo.%s", mimeType.Extension())
	}

	file, err := media.GetInputFile(params.File, params.Filename)
	if err != nil {
		return telegram.InputMediaPhoto{}, err
	}

	senderPeer, err := media.Client.ResolvePeer(message.ChannelID())
	if err != nil {
		return telegram.InputMediaPhoto{}, err
	}

	messageMedia, err := message.Client.MessagesUploadMedia("", senderPeer, &telegram.InputMediaUploadedPhoto{
		Spoiler: params.Spoiler,
		File:    file,
	})
	if err != nil {
		return telegram.InputMediaPhoto{}, err
	}

	return telegram.InputMediaPhoto{
		Spoiler: params.Spoiler,
		ID: &telegram.InputPhotoObj{
			ID:            messageMedia.(*telegram.MessageMediaPhoto).Photo.(*telegram.PhotoObj).ID,
			AccessHash:    messageMedia.(*telegram.MessageMediaPhoto).Photo.(*telegram.PhotoObj).AccessHash,
			FileReference: messageMedia.(*telegram.MessageMediaPhoto).Photo.(*telegram.PhotoObj).FileReference,
		},
	}, nil
}

type UploadVideoParams struct {
	File              []byte
	MimeType          string
	Filename          string
	Spoiler           bool
	Duration          float64
	Width             int32
	Height            int32
	SupportsStreaming bool
	Thumb             []byte
	NoSoundVideo      bool
	ForceFile         bool
}

func UploadVideo(message *telegram.NewMessage, params UploadVideoParams) (telegram.InputMediaDocument, error) {
	if len(params.File) == 0 {
		return telegram.InputMediaDocument{}, errors.New("file is required")
	}

	media := &Media{
		Message: message,
		Client:  message.Client,
	}

	file, err := media.GetInputFile(params.File, params.Filename)
	if err != nil {
		return telegram.InputMediaDocument{}, err
	}

	senderPeer, err := media.Client.ResolvePeer(message.ChannelID())
	if err != nil {
		return telegram.InputMediaDocument{}, err
	}

	var thumbnail telegram.InputFile
	if len(params.Thumb) > 0 {
		thumbnail, err = media.GetInputFile(params.Thumb)
		if err != nil {
			return telegram.InputMediaDocument{}, err
		}
	}

	if params.MimeType == "" {
		mimeType, err := mimetype.DetectReader(bytes.NewReader(params.File))
		if err != nil {
			return telegram.InputMediaDocument{}, fmt.Errorf("failed to detect mime type: %w", err)
		}
		params.MimeType = mimeType.String()
	}

	if params.Filename == "" {
		exts, err := mime.ExtensionsByType(params.MimeType)
		if err != nil {
			params.Filename = "video.mp4"
		} else {
			params.Filename = fmt.Sprintf("video%s", exts[0])
		}
	}

	messageMedia, err := message.Client.MessagesUploadMedia("", senderPeer, &telegram.InputMediaUploadedDocument{
		NosoundVideo: params.NoSoundVideo,
		ForceFile:    params.ForceFile,
		Spoiler:      params.Spoiler,
		File:         file,
		Thumb:        thumbnail,
		MimeType:     params.MimeType,
		Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
			SupportsStreaming: params.SupportsStreaming,
			W:                 params.Width,
			H:                 params.Height,
			Duration:          params.Duration,
		}},
	})
	if err != nil {
		return telegram.InputMediaDocument{}, err
	}

	return telegram.InputMediaDocument{
		Spoiler: params.Spoiler,
		ID: &telegram.InputDocumentObj{
			ID:            messageMedia.(*telegram.MessageMediaDocument).Document.(*telegram.DocumentObj).ID,
			AccessHash:    messageMedia.(*telegram.MessageMediaDocument).Document.(*telegram.DocumentObj).AccessHash,
			FileReference: messageMedia.(*telegram.MessageMediaDocument).Document.(*telegram.DocumentObj).FileReference,
		},
	}, nil
}
