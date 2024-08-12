package utils

import (
	"fmt"

	"github.com/amarnathcjd/gogram/telegram"
)

func FormatText(text string, entities []telegram.MessageEntity) string {
	textRunes := []rune(text)

	for i := len(entities) - 1; i >= 0; i-- {
		switch entity := entities[i].(type) {
		case *telegram.MessageEntityBold:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</b>"), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune("<b>"), textRunes[entity.Offset:]...)...)
		case *telegram.MessageEntityItalic:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</i>"), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune("<i>"), textRunes[entity.Offset:]...)...)
		case *telegram.MessageEntityCode:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</code>"), textRunes[entity.Offset+entity.Length:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune("<code>"), textRunes[entity.Offset:]...)...)
		case *telegram.MessageEntityTextURL:
			textRunes = append(textRunes[:entity.Offset+entity.Length], append([]rune("</a>"), textRunes[entity.Offset+entity.Length-1:]...)...)
			textRunes = append(textRunes[:entity.Offset], append([]rune(fmt.Sprintf("<a href='%s'>", entity.URL)), textRunes[entity.Offset:]...)...)
		}
	}
	return string(textRunes)
}
