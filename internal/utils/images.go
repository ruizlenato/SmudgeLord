package utils

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"os"

	"github.com/anthonynsimon/bild/transform"
)

func ResizeSticker(input []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	resizedImg := transform.Resize(img, 512, 512, transform.Lanczos)

	var buf bytes.Buffer
	if err := png.Encode(&buf, resizedImg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func processThumbnailImage(img image.Image) ([]byte, error) {
	var buf bytes.Buffer

	originalWidth := img.Bounds().Dx()
	originalHeight := img.Bounds().Dy()
	if originalWidth > 320 || originalHeight > 320 {
		aspectRatio := float64(originalWidth) / float64(originalHeight)
		var newWidth, newHeight int
		if originalWidth > originalHeight {
			newWidth = 320
			newHeight = int(float64(newWidth) / aspectRatio)
		} else {
			newHeight = 320
			newWidth = int(float64(newHeight) * aspectRatio)
		}
		img = transform.Resize(img, newWidth, newHeight, transform.Linear)
	}

	quality := 100
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}

	for int64(buf.Len()) > 200*1024 && quality > 10 {
		quality -= 10
		buf.Reset()
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func ResizeThumbnail(thumbnail *os.File) error {
	if _, err := thumbnail.Seek(0, 0); err != nil {
		return err
	}
	img, _, err := image.Decode(thumbnail)
	if err != nil {
		return err
	}
	finalBytes, err := processThumbnailImage(img)
	if err != nil {
		return err
	}
	if err := os.WriteFile(thumbnail.Name(), finalBytes, 0o644); err != nil {
		return err
	}
	_, err = thumbnail.Seek(0, 0)
	return err
}

func ResizeThumbnailFromBytes(input []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	return processThumbnailImage(img)
}
