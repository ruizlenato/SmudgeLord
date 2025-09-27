package utils

import (
	"crypto/rand"
	"fmt"
	"html"
	"math/big"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/amarnathcjd/gogram/telegram"
)

func getTag(entity telegram.MessageEntity) (openTag, closeTag string) {
	switch e := entity.(type) {
	case *telegram.MessageEntityBold:
		return "<b>", "</b>"
	case *telegram.MessageEntityItalic:
		return "<i>", "</i>"
	case *telegram.MessageEntityCode:
		return "<code>", "</code>"
	case *telegram.MessageEntityUnderline:
		return "<u>", "</u>"
	case *telegram.MessageEntityStrike:
		return "<s>", "</s>"
	case *telegram.MessageEntityTextURL:
		url := html.EscapeString(e.URL)
		return fmt.Sprintf("<a href=%q>", url), "</a>"
	case *telegram.MessageEntityBlockquote:
		return "<blockquote>", "</blockquote>"
	default:
		return "", ""
	}
}

type tagPoint struct {
	pos     int
	closing bool
	tag     string
}

func getEntityOffsetAndLength(entity telegram.MessageEntity) (int32, int32, bool) {
	switch e := entity.(type) {
	case *telegram.MessageEntityBold:
		return e.Offset, e.Length, true
	case *telegram.MessageEntityItalic:
		return e.Offset, e.Length, true
	case *telegram.MessageEntityCode:
		return e.Offset, e.Length, true
	case *telegram.MessageEntityTextURL:
		return e.Offset, e.Length, true
	case *telegram.MessageEntityUnderline:
		return e.Offset, e.Length, true
	case *telegram.MessageEntityStrike:
		return e.Offset, e.Length, true
	case *telegram.MessageEntityBlockquote:
		return e.Offset, e.Length, true
	default:
		return 0, 0, false
	}
}

func collectTagPoints(entities []telegram.MessageEntity) []tagPoint {
	var points []tagPoint

	for _, entity := range entities {
		offset, length, valid := getEntityOffsetAndLength(entity)
		if !valid {
			continue
		}

		openTag, closeTag := getTag(entity)
		if openTag != "" {
			points = append(points,
				tagPoint{pos: int(offset), closing: false, tag: openTag},
				tagPoint{pos: int(offset + length), closing: true, tag: closeTag})
		}
	}

	sort.Slice(points, func(i, j int) bool {
		if points[i].pos != points[j].pos {
			return points[i].pos < points[j].pos
		}
		return points[i].closing
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

func FormatText(text string, entities []telegram.MessageEntity) string {
	utf16Text := utf16.Encode([]rune(text))
	points := collectTagPoints(entities)
	return buildFormattedText(utf16Text, points)
}

func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result strings.Builder
	for range n {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		result.WriteByte(letters[idx.Int64()])
	}
	return result.String()
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
