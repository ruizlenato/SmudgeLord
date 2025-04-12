package utils

import (
	"fmt"
	"math/rand"
	"strings"
	"unicode/utf16"

	"github.com/amarnathcjd/gogram/telegram"
)

func FormatText(text string, entities []telegram.MessageEntity) string {
	textRunes := utf16.Encode([]rune(text))

	for i := len(entities) - 1; i >= 0; i-- {
		switch entity := entities[i].(type) {
		case *telegram.MessageEntityBold:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</b>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune("<b>")), textRunes[entity.Offset:]...)...)
		case *telegram.MessageEntityItalic:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</i>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune("<i>")), textRunes[entity.Offset:]...)...)
		case *telegram.MessageEntityCode:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</code>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune("<code>")), textRunes[entity.Offset:]...)...)
		case *telegram.MessageEntityTextURL:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</a>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune(fmt.Sprintf("<a href='%s'>", entity.URL))), textRunes[entity.Offset:]...)...)
		}
	}
	return string(utf16.Decode(textRunes))
}

func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result strings.Builder
	for range n {
		result.WriteByte(letters[rand.Intn(len(letters))])
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
