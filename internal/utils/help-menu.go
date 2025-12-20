package utils

import (
	"fmt"
	"sort"

	"github.com/go-telegram/bot/models"
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

func GetHelpKeyboard(i18n func(string, ...map[string]any) string) [][]models.InlineKeyboardButton {
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

	buttons := make([][]models.InlineKeyboardButton, 0, (len(modules)+2)/3)
	for i := 0; i < len(modules); i += 3 {
		row := make([]models.InlineKeyboardButton, 0, 3)
		for j := i; j < i+3 && j < len(modules); j++ {
			row = append(row, models.InlineKeyboardButton{
				Text:         modules[j].text,
				CallbackData: fmt.Sprintf("helpMessage %s", modules[j].name),
			})
		}
		buttons = append(buttons, row)
	}

	buttons = append(buttons, []models.InlineKeyboardButton{{
		Text:         i18n("back-button"),
		CallbackData: "start",
	}})
	return buttons
}
