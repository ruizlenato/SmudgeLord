package utils

import (
	"fmt"
	"sort"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

type module struct {
	Name string
}

var helpModule = make(map[string]module)

func SaveHelp(name string) {
	helpModule[name] = module{
		Name: name,
	}
}

func GetHelpKeyboard(i18n func(string, ...map[string]any) string) [][]gotgbot.InlineKeyboardButton {
	modules := make([]struct {
		name string
		text string
	}, 0, len(helpModule))

	for name := range helpModule {
		modules = append(modules, struct {
			name string
			text string
		}{name, i18n(name)})
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].text < modules[j].text
	})

	buttons := make([][]gotgbot.InlineKeyboardButton, 0, (len(modules)+2)/3)
	for i := 0; i < len(modules); i += 3 {
		row := make([]gotgbot.InlineKeyboardButton, 0, 3)
		for j := i; j < i+3 && j < len(modules); j++ {
			row = append(row, gotgbot.InlineKeyboardButton{
				Text:         modules[j].text,
				CallbackData: fmt.Sprintf("helpMessage %s", modules[j].name),
			})
		}
		buttons = append(buttons, row)
	}

	buttons = append(buttons, []gotgbot.InlineKeyboardButton{{
		Text:         i18n("back-button"),
		CallbackData: "start",
	}})

	return buttons
}
