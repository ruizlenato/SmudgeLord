package utils

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"

	"github.com/anthonynsimon/bild/transform"
	"golang.org/x/image/webp"
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

func ResizeThumbnail(input []byte) ([]byte, error) {
	img, err := decodeImage(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}
	return processThumbnailImage(img)
}

func ReencodeImage(input []byte) ([]byte, error) {
	img, err := decodeImage(bytes.NewReader(input))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	switch img.(type) {
	case *image.NRGBA, *image.NRGBA64, *image.RGBA, *image.RGBA64, *image.Paletted:
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
	default:
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func decodeImage(r *bytes.Reader) (image.Image, error) {
	header := make([]byte, 12)
	n, err := r.Read(header)
	if err != nil || n < 12 {
		return nil, err
	}

	r.Seek(0, 0)

	if len(header) >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		return webp.Decode(r)
	}

	img, _, err := image.Decode(r)
	return img, err
}
