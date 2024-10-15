package utils

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"

	"github.com/anthonynsimon/bild/imgio"
	"github.com/anthonynsimon/bild/transform"
)

func ResizeSticker(input []byte) (*os.File, error) {
	img, _, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	resizedImg := transform.Resize(img, 512, 512, transform.Lanczos)

	tempFile, err := os.CreateTemp("", "Smudge*.png")
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			tempFile.Close()
			os.Remove(tempFile.Name())
		}
	}()

	if err := imgio.Save(tempFile.Name(), resizedImg, imgio.PNGEncoder()); err != nil {
		return nil, err
	}

	if _, err = tempFile.Seek(0, 0); err != nil {
		return nil, err
	}

	return tempFile, nil
}

func ResizeThumbnail(thumbnail *os.File) error {
	img, _, err := image.Decode(thumbnail)
	if err != nil {
		return err
	}
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

		resizedImg := transform.Resize(img, newWidth, newHeight, transform.Linear)
		err = imgio.Save(thumbnail.Name(), resizedImg, imgio.PNGEncoder())
		if err != nil {
			return err
		}
	}

	thumbnailInfo, err := thumbnail.Stat()
	if err != nil {
		return err
	}

	if thumbnailInfo.Size() > 200*1024 {
		quality := 100
		for thumbnailInfo.Size() > 200*1024 && quality > 10 {
			quality -= 10

			thumbnail.Seek(0, 0)
			img, _, err = image.Decode(thumbnail)
			if err != nil {
				return err
			}

			var buf bytes.Buffer
			err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
			if err != nil {
				return err
			}

			err = os.WriteFile(thumbnail.Name(), buf.Bytes(), 0o644)
			if err != nil {
				return err
			}

			thumbnailInfo, err = thumbnail.Stat()
			if err != nil {
				return err
			}
		}
	}
	_, err = thumbnail.Seek(0, 0)
	if err != nil {
		return err
	}

	return nil
}
