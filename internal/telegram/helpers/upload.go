package helpers

import (
	"errors"
	"fmt"
	"mime"
	"path/filepath"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/gabriel-vasile/mimetype"
	tg "github.com/ruizlenato/smudgelord/internal/telegram"
)

type Media struct {
	Client *telegram.Client
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

	return uploadedMedia, nil
}

func getMimeType(file []byte, filename, currentMimeType string) string {
	if currentMimeType != "" {
		return currentMimeType
	}

	if filename != "" {
		if mimeType := mime.TypeByExtension(filepath.Ext(filename)); mimeType != "" {
			return mimeType
		}
	}

	return mimetype.Detect(file).String()
}

func generateFilename(file []byte, currentFilename, defaultPrefix, defaultExt string) string {
	if currentFilename != "" {
		return currentFilename
	}

	extension := mimetype.Detect(file).Extension()
	if extension == "" || extension == ".unknown" || extension == ".webp" {
		extension = defaultExt
	}

	return fmt.Sprintf("%s%s", defaultPrefix, extension)
}

type UploadPhotoParams struct {
	Spoiler  bool
	File     []byte
	Filename string
}

func UploadPhoto(params UploadPhotoParams) (telegram.InputMediaPhoto, error) {
	if len(params.File) == 0 {
		return telegram.InputMediaPhoto{}, errors.New("file is required")
	}

	media := &Media{
		Client: tg.Client,
	}

	params.Filename = generateFilename(params.File, params.Filename, "photo", ".jpg")

	file, err := media.GetInputFile(params.File, params.Filename)
	if err != nil {
		return telegram.InputMediaPhoto{}, err
	}

	messageMedia, err := media.Client.MessagesUploadMedia("", &telegram.InputPeerSelf{}, &telegram.InputMediaUploadedPhoto{
		Spoiler:    params.Spoiler,
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

func UploadVideo(params UploadVideoParams) (telegram.InputMediaDocument, error) {
	if len(params.File) == 0 {
		return telegram.InputMediaDocument{}, errors.New("file is required")
	}

	media := &Media{
		Client: tg.Client,
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

	params.MimeType = getMimeType(params.File, params.Filename, params.MimeType)
	params.Filename = generateFilename(params.File, params.Filename, "SmudgeLord_video", ".mp4")

	noSound := true
	if params.NoSoundVideo != nil {
		noSound = *params.NoSoundVideo
	}

	messageMedia, err := media.Client.MessagesUploadMedia("", &telegram.InputPeerSelf{}, &telegram.InputMediaUploadedDocument{
		NosoundVideo: noSound,
		ForceFile:    params.ForceFile,
		Spoiler:      params.Spoiler,
		File:         file,
		Thumb:        thumbnail,
		MimeType:     params.MimeType,
		Attributes: []telegram.DocumentAttribute{
			&telegram.DocumentAttributeVideo{
				SupportsStreaming: params.SupportsStreaming,
				W:                 params.Width,
				H:                 params.Height,
				Duration:          params.Duration,
			},
			&telegram.DocumentAttributeFilename{FileName: params.Filename},
		},
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

type UploadAudioParams struct {
	File      []byte
	MimeType  string
	Filename  string
	Thumb     []byte
	Duration  int32
	Performer string
	Title     string
}

func UploadAudio(params UploadAudioParams) (telegram.InputMediaDocument, error) {
	if len(params.File) == 0 {
		return telegram.InputMediaDocument{}, errors.New("file is required")
	}

	media := &Media{
		Client: tg.Client,
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

	params.MimeType = getMimeType(params.File, params.Filename, params.MimeType)
	params.Filename = generateFilename(params.File, params.Filename, "SmudgeLord_audio", ".mp3")

	messageMedia, err := media.Client.MessagesUploadMedia("", &telegram.InputPeerSelf{}, &telegram.InputMediaUploadedDocument{
		File:     file,
		MimeType: params.MimeType,
		Thumb:    thumbnail,
		Attributes: []telegram.DocumentAttribute{
			&telegram.DocumentAttributeAudio{
				Duration:  params.Duration,
				Performer: params.Performer,
				Title:     params.Title,
			},
			&telegram.DocumentAttributeFilename{FileName: params.Filename},
		},
	})
	if err != nil {
		return telegram.InputMediaDocument{}, err
	}

	switch doc := messageMedia.(*telegram.MessageMediaDocument).Document.(type) {
	case *telegram.DocumentObj:
		return telegram.InputMediaDocument{
			ID: &telegram.InputDocumentObj{
				ID:            doc.ID,
				AccessHash:    doc.AccessHash,
				FileReference: doc.FileReference,
			},
		}, nil
	case *telegram.DocumentEmpty:
		return telegram.InputMediaDocument{ID: &telegram.InputDocumentEmpty{}}, nil
	}

	return telegram.InputMediaDocument{}, errors.New("failed to upload audio")
}
