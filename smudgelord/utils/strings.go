package utils

import (
	"fmt"

	"github.com/mymmrac/telego"
)

func FormatText(text string, entities []telego.MessageEntity) string {
	// Convert text to a slice of runes
	textRunes := []rune(text)

	// Insert entities in reverse order to keep track of shifting offsets
	for i := len(entities) - 1; i >= 0; i-- {
		entity := entities[i]
		switch entity.Type {
		case "bold":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</b>"), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune("<b>"), textRunes[entity.Offset:]...)...)
		case "italic":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</i>"), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune("<i>"), textRunes[entity.Offset:]...)...)
		case "code":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</code>"), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune("<code>"), textRunes[entity.Offset:]...)...)
		case "text_link":
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</a>"), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune(fmt.Sprintf("<a href='%s'>", entity.URL)), textRunes[entity.Offset:]...)...)
		}
	}
	return string(textRunes)
}
