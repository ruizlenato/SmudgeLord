package helpers

import (
	"fmt"

	"github.com/mymmrac/telego"
)

type module struct {
	Name string
	// TODO: Add support for keyboard
	// Keyboard [][]telego.InlineKeyboardButton
}

var helpModule = make(map[string]module)

func Store(name string) {
	helpModule[name] = module{
		Name: name,
		// TODO: Add support for keyboard
		// Keyboard: keyboard,
	}
}

func GetHelpKeyboard(i18n func(string) string) [][]telego.InlineKeyboardButton {
	buttons := make([][]telego.InlineKeyboardButton, 0, len(helpModule))
	i := 0
	var row []telego.InlineKeyboardButton

	for name := range helpModule {
		if i == 2 {
			buttons = append(buttons, row)
			row = []telego.InlineKeyboardButton{}
			i = 0
		}
		row = append(row, telego.InlineKeyboardButton{
			Text:         i18n(fmt.Sprintf("%s.name", name)),
			CallbackData: fmt.Sprintf("helpMessage %s", name),
		})
		i++
	}
	if len(row) > 0 {
		buttons = append(buttons, row)
	}
	buttons = append(buttons, []telego.InlineKeyboardButton{{
		Text:         i18n("button.back"),
		CallbackData: "start",
	}})
	return buttons
}
