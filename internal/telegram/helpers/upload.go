package helpers

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/gabriel-vasile/mimetype"
)

type UploadDocumentParams struct {
	ForceFile  bool
	Spoiler    bool
	File       string
	Thumb      string
	MimeType   string
	Attributes []telegram.DocumentAttribute
	Stickers   []telegram.InputDocument
	TTLSeconds int32
}

func UploadDocument(message *telegram.NewMessage, params UploadDocumentParams) (telegram.InputMediaUploadedDocument, error) {
	if params.File == "" {
		return telegram.InputMediaUploadedDocument{}, errors.New("file is required")
	}

	if params.MimeType == "" {
		mimeType, err := mimetype.DetectFile(params.File)
		if err != nil {
			return telegram.InputMediaUploadedDocument{}, fmt.Errorf("failed to detect mime type: %w", err)
		}
		params.MimeType = mimeType.String()
	}

	file, err := GetInputFile(message, params.File)
	if err != nil {
		return telegram.InputMediaUploadedDocument{}, err
	}

	var thumbnail telegram.InputFile
	if params.Thumb != "" {
		thumbnail, err = GetInputFile(message, params.Thumb)
		if err != nil {
			return telegram.InputMediaUploadedDocument{}, err
		}
	}

	return telegram.InputMediaUploadedDocument{
		NosoundVideo: true,
		ForceFile:    params.ForceFile,
		Spoiler:      params.Spoiler,
		File:         file,
		Thumb:        thumbnail,
		MimeType:     params.MimeType,
		Attributes:   params.Attributes,
		Stickers:     params.Stickers,
		TtlSeconds:   params.TTLSeconds,
	}, nil
}

type UploadPhotoParams struct {
	Spoiler    bool
	File       string
	Stickers   []telegram.InputDocument
	TTLSeconds int32
}

func UploadPhoto(message *telegram.NewMessage, params UploadPhotoParams) (telegram.InputMediaUploadedPhoto, error) {
	if params.File == "" {
		return telegram.InputMediaUploadedPhoto{}, errors.New("file is required")
	}
	file, err := GetInputFile(message, params.File)
	if err != nil {
		return telegram.InputMediaUploadedPhoto{}, err
	}

	return telegram.InputMediaUploadedPhoto{
		Spoiler:    params.Spoiler,
		File:       file,
		Stickers:   params.Stickers,
		TtlSeconds: params.TTLSeconds,
	}, nil
}

func GetInputFile(message *telegram.NewMessage, file string) (telegram.InputFile, error) {
	if file == "" {
		return nil, errors.New("file path is required")
	}

	uploadedMedia, err := message.Client.UploadFile(file)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.Remove(file); err != nil {
			log.Print("Failed to remove file: ", err)
		}
	}()
	return uploadedMedia, nil
}
