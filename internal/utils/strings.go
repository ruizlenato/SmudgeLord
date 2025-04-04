package utils

import (
	"fmt"
	"math/rand"
	"strings"
	"unicode/utf16"

	"github.com/go-telegram/bot/models"
)

func FormatText(text string, entities []models.MessageEntity) string {
	textRunes := utf16.Encode([]rune(text))

	for i := len(entities) - 1; i >= 0; i-- {
		entity := entities[i]
		switch entity.Type {
		case "bold":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</b>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune("<b>")), textRunes[entity.Offset:]...)...)
		case "italic":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</i>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune("<i>")), textRunes[entity.Offset:]...)...)
		case "code":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</code>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune("<code>")), textRunes[entity.Offset:]...)...)
		case "text_link":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append(utf16.Encode([]rune("</a>")), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append(utf16.Encode([]rune(fmt.Sprintf("<a href='%s'>", entity.URL))), textRunes[entity.Offset:]...)...)
		}
	}

	return string(utf16.Decode(textRunes))
}

func RandomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var result strings.Builder
	for i := 0; i < n; i++ {
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
