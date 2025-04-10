package utils

import (
	"sort"

	"github.com/amarnathcjd/gogram/telegram"
)

type module struct {
	Name string
}

var helpModule = make(map[string]module)

func SotreHelp(name string) {
	helpModule[name] = module{
		Name: name,
	}
}

func GetHelpKeyboard(i18n func(string, ...map[string]interface{}) string) telegram.ReplyMarkup {
	moduleNames := make([]string, 0, len(helpModule))
	for name := range helpModule {
		moduleNames = append(moduleNames, name)
	}

	sort.Strings(moduleNames)

	var keyboard []*telegram.KeyboardButtonRow

	row := make([]telegram.KeyboardButton, 0, len(moduleNames))
	for _, name := range moduleNames {
		if len(row) == 3 {
			keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(row...))
			row = nil
		}
		row = append(row, telegram.ButtonBuilder{}.Data(i18n(name), "helpMessage "+name))
	}

	if len(row) > 0 {
		keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(row...))
	}

	backButton := telegram.ButtonBuilder{}.Data(i18n("back-button"), "start")
	keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(backButton))

	return telegram.ButtonBuilder{}.Keyboard(keyboard...)
}
