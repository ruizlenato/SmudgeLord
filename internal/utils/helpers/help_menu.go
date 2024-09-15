package helpers

import (
	"fmt"
	"sort"

	"github.com/mymmrac/telego"
)

type module struct {
	Name string
}

var helpModule = make(map[string]module)

func Store(name string) {
	helpModule[name] = module{
		Name: name,
	}
}

func GetHelpKeyboard(i18n func(string, ...map[string]interface{}) string) [][]telego.InlineKeyboardButton {
	var moduleNames []string
	for name := range helpModule {
		moduleNames = append(moduleNames, name)
	}

	sort.Strings(moduleNames)

	buttons := make([][]telego.InlineKeyboardButton, 0, len(moduleNames))
	var row []telego.InlineKeyboardButton
	for _, name := range moduleNames {
		if len(row) == 3 {
			buttons = append(buttons, row)
			row = nil
		}
		row = append(row, telego.InlineKeyboardButton{
			Text:         i18n(name),
			CallbackData: fmt.Sprintf("helpMessage %s", name),
		})
	}
	if len(row) > 0 {
		buttons = append(buttons, row)
	}

	buttons = append(buttons, []telego.InlineKeyboardButton{{
		Text:         i18n("back-button"),
		CallbackData: "start",
	}})
	return buttons
}
