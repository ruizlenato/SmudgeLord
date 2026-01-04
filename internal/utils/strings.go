package utils

import (
	"encoding/base64"
	"fmt"
	"html"
	"math/rand"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/go-telegram/bot/models"
)

func getTag(entity models.MessageEntity) (openTag, closeTag string) {
	switch entity.Type {
	case models.MessageEntityTypeBold:
		return "<b>", "</b>"
	case models.MessageEntityTypeItalic:
		return "<i>", "</i>"
	case models.MessageEntityTypeCode:
		return "<code>", "</code>"
	case models.MessageEntityTypeUnderline:
		return "<u>", "</u>"
	case models.MessageEntityTypeStrikethrough:
		return "<s>", "</s>"
	case models.MessageEntityTypeTextLink:
		url := html.EscapeString(entity.URL)
		return fmt.Sprintf("<a href=%q>", url), "</a>"
	case models.MessageEntityTypeBlockquote:
		return "<blockquote>", "</blockquote>"
	default:
		return "", ""
	}
}

type tagPoint struct {
	pos     int
	closing bool
	tag     string
	start   int
	end     int
	idx     int
}

func collectTagPoints(entities []models.MessageEntity) []tagPoint {
	var points []tagPoint

	for idx, entity := range entities {
		offset := entity.Offset
		length := entity.Length
		if offset < 0 || length <= 0 {
			continue
		}

		start := int(offset)
		end := int(offset + length)

		openTag, closeTag := getTag(entity)
		if openTag != "" {
			points = append(points,
				tagPoint{pos: start, closing: false, tag: openTag, start: start, end: end, idx: idx},
				tagPoint{pos: end, closing: true, tag: closeTag, start: start, end: end, idx: idx})
		}
	}

	sort.Slice(points, func(i, j int) bool {
		if points[i].pos != points[j].pos {
			return points[i].pos < points[j].pos
		}
		if points[i].closing != points[j].closing {
			return points[i].closing
		}
		if !points[i].closing {
			if points[i].end != points[j].end {
				return points[i].end > points[j].end
			}
			return points[i].idx < points[j].idx
		}
		if points[i].start != points[j].start {
			return points[i].start > points[j].start
		}
		return points[i].idx > points[j].idx
	})

	return points
}

func buildFormattedText(utf16Text []uint16, points []tagPoint) string {
	var builder strings.Builder
	builder.Grow(len(utf16Text) + len(points)*5)

	lastPos := 0
	pointIndex := 0
	for i := 0; i <= len(utf16Text); i++ {
		for pointIndex < len(points) && points[pointIndex].pos == i {
			if i > lastPos {
				builder.WriteString(string(utf16.Decode(utf16Text[lastPos:i])))
			}
			builder.WriteString(points[pointIndex].tag)
			lastPos = i
			pointIndex++
		}
	}

	if lastPos < len(utf16Text) {
		builder.WriteString(string(utf16.Decode(utf16Text[lastPos:])))
	}

	return builder.String()
}

func FormatText(text string, entities []models.MessageEntity) string {
	utf16Text := utf16.Encode([]rune(text))
	points := collectTagPoints(entities)
	return buildFormattedText(utf16Text, points)
}

func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result strings.Builder
	for range n {
		result.WriteByte(letters[rand.Intn(len(letters))])
	}
	return result.String()
}

func EscapeHTML(s string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return replacer.Replace(s)
}

func SanitizeString(input string) string {
	illegalChars := []rune{'}', '{', '%', '>', '<', '^', ';', ':', '`', '$', '"', '@', '=', '?', '|', '*'}

	result := strings.ReplaceAll(input, "/", "_")
	result = strings.ReplaceAll(result, "\\", "_")

	for _, char := range illegalChars {
		result = strings.ReplaceAll(result, string(char), "")
	}

	return result
}

func FileTypeByFileID(fileID string) int32 {
	if fileID == "" {
		return 0
	}

	fileID = strings.ReplaceAll(fileID, "-", "+")
	fileID = strings.ReplaceAll(fileID, "_", "/")

	decoded, err := base64.RawStdEncoding.DecodeString(fileID)
	if err != nil {
		decoded, err = base64.StdEncoding.DecodeString(fileID)
		if err != nil {
			return 0
		}
	}

	if len(decoded) < 4 {
		return 0
	}

	fileType := int32(decoded[0]) | int32(decoded[1])<<8 | int32(decoded[2])<<16 | int32(decoded[3])<<24
	fileType = fileType & 0xFF

	return fileType
}
