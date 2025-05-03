package helpers

import (
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
		extension := mimetype.Detect(params.File).Extension()
		switch extension {
		case "", ".webp", ".unknown":
			extension = ".jpg"
		}

		params.Filename = fmt.Sprintf("photo%s", extension)
	}

	file, err := media.GetInputFile(params.File, params.Filename)
	if err != nil {
		return telegram.InputMediaPhoto{}, err
	}

	messageMedia, err := message.Client.MessagesUploadMedia("", &telegram.InputPeerSelf{}, &telegram.InputMediaUploadedPhoto{
		Spoiler:    false,
		File:       file,
		TtlSeconds: 0,
	})
	if err != nil {
		return telegram.InputMediaPhoto{}, err
	}

	switch photo := messageMedia.(*telegram.MessageMediaPhoto).Photo.(type) {
	case *telegram.PhotoObj:
		return telegram.InputMediaPhoto{
			Spoiler: params.Spoiler,
			ID: &telegram.InputPhotoObj{
				ID:            photo.ID,
				AccessHash:    photo.AccessHash,
				FileReference: photo.FileReference,
			},
		}, nil
	case *telegram.PhotoEmpty:
		return telegram.InputMediaPhoto{ID: &telegram.InputPhotoEmpty{}}, nil
	}

	return telegram.InputMediaPhoto{}, errors.New("failed to upload photo")
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
	NoSoundVideo      *bool
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

	var thumbnail telegram.InputFile
	if len(params.Thumb) > 0 {
		thumbnail, err = media.GetInputFile(params.Thumb)
		if err != nil {
			return telegram.InputMediaDocument{}, err
		}
	}

	if params.MimeType == "" {
		params.MimeType = mimetype.Detect(params.File).String()
	}

	if params.Filename == "" {
		exts, err := mime.ExtensionsByType(params.MimeType)
		if err != nil {
			params.Filename = "video.mp4"
		} else {
			params.Filename = fmt.Sprintf("video%s", exts[0])
		}
	}

	noSound := true
	if params.NoSoundVideo != nil {
		noSound = *params.NoSoundVideo
	}

	messageMedia, err := message.Client.MessagesUploadMedia("", &telegram.InputPeerSelf{}, &telegram.InputMediaUploadedDocument{
		NosoundVideo: noSound,
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

	switch doc := messageMedia.(*telegram.MessageMediaDocument).Document.(type) {
	case *telegram.DocumentObj:
		return telegram.InputMediaDocument{
			Spoiler: params.Spoiler,
			ID: &telegram.InputDocumentObj{
				ID:            doc.ID,
				AccessHash:    doc.AccessHash,
				FileReference: doc.FileReference,
			},
		}, nil
	case *telegram.DocumentEmpty:
		return telegram.InputMediaDocument{ID: &telegram.InputDocumentEmpty{}}, nil
	}

	return telegram.InputMediaDocument{}, errors.New("failed to upload video")
}
