package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"
)

func checkDisabledCommand(command string) bool {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM commandsDisabled WHERE command = ? LIMIT 1);"
	err := database.DB.QueryRow(query, command).Scan(&exists)
	if err != nil {
		fmt.Printf("Error checking command: %v\n", err)
		return false
	}
	return exists
}

func handleDisableable(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message)
	text := i18n("config.cmdDisableables")
	for _, command := range helpers.DisableableCommands {
		text += "\n- <code>" + command + "</code>"
	}
	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      text,
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return
}

func handleDisable(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message)
	contains := func(array []string, str string) bool {
		for _, item := range array {
			if item == str {
				return true
			}
		}
		return false
	}
	_, _, args := telegoutil.ParseCommand(message.Text)
	if len(args) == 0 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("config.cmdDisableUsage"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if !contains(helpers.DisableableCommands, args[0]) {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("config.cmdCantDisable"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if checkDisabledCommand(args[0]) {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      fmt.Sprintf(i18n("config.cmdAlreadyDisabled"), args[0]),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	query := "INSERT INTO commandsDisabled (chat_id, command) VALUES (?, ?);"
	_, err := database.DB.Exec(query, message.Chat.ID, args[0])
	if err != nil {
		fmt.Print("Error inserting command: " + err.Error())
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      fmt.Sprintf(i18n("config.cmdDisabled"), args[0]),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return
}

func handleEnable(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message)
	contains := func(array []string, str string) bool {
		for _, item := range array {
			if item == str {
				return true
			}
		}
		return false
	}
	_, _, args := telegoutil.ParseCommand(message.Text)
	if len(args) == 0 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("config.cmdEnableUsage"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if !contains(helpers.DisableableCommands, args[0]) {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("config.cmdCantDisable"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if !checkDisabledCommand(args[0]) {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      fmt.Sprintf(i18n("config.cmdAlreadyEnabled"), args[0]),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	query := "DELETE FROM commandsDisabled WHERE command = ?;"
	_, err := database.DB.Exec(query, args[0])
	if err != nil {
		fmt.Print("Error deleting command: " + err.Error())
		return
	}
	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      fmt.Sprintf(i18n("config.cmdEnabled"), args[0]),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return
}

func handleDisabled(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message)
	text := i18n("config.disabledCmds")
	rows, err := database.DB.Query("SELECT command FROM commandsDisabled WHERE chat_id = ?", message.Chat.ID)
	if err != nil {
		return
	}
	defer rows.Close()

	var commands []string
	for rows.Next() {
		var command string
		if err := rows.Scan(&command); err != nil {
			return
		}
		commands = append(commands, command)
	}

	if len(commands) == 0 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("config.noDisabledCmds"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	for _, command := range commands {
		text += "\n- <code>" + command + "</code>"
	}
	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      text,
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
	return
}

func callbackLanguageMenu(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)

	buttons := make([][]telego.InlineKeyboardButton, 0, len(database.AvailableLocales))
	for _, lang := range database.AvailableLocales {
		loaded, ok := localization.LangCache[lang]
		if !ok {
			log.Fatalf("Language '%s' not found in the cache.", lang)
		}

		buttons = append(buttons, []telego.InlineKeyboardButton{{
			Text: localization.GetStringFromNestedMap(loaded, "language.flag") +
				localization.GetStringFromNestedMap(loaded, "language.name"),
			CallbackData: fmt.Sprintf("setLang %s", lang),
		}})
	}

	row := database.DB.QueryRow("SELECT language FROM users WHERE id = ?;", update.CallbackQuery.Message.GetChat().ID)
	if strings.Contains(update.CallbackQuery.Message.GetChat().Type, "group") {
		row = database.DB.QueryRow("SELECT language FROM groups WHERE id = ?;", update.CallbackQuery.Message.GetChat().ID)
	}
	var language string
	err := row.Scan(&language)
	if err != nil {
		log.Print("callbackLanguageMenu - Error querying user:", err)
	}

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        fmt.Sprintf(i18n("menu.languageMessage"), i18n("language.flag"), i18n("language.name")),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
}

func callbackLanguageSet(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)
	lang := strings.ReplaceAll(update.CallbackQuery.Data, "setLang ", "")

	dbQuery := "UPDATE users SET language = ? WHERE id = ?;"
	if strings.Contains(update.CallbackQuery.Message.GetChat().Type, "group") {
		dbQuery = "UPDATE groups SET language = ? WHERE id = ?;"
	}
	_, err := database.DB.Exec(dbQuery, lang, update.CallbackQuery.Message.GetChat().ID)
	if err != nil {
		log.Print("Error updating language:", err)
	}

	buttons := make([][]telego.InlineKeyboardButton, 0, len(database.AvailableLocales))

	if update.CallbackQuery.Message.GetChat().Type == telego.ChatTypePrivate {
		buttons = append(buttons, []telego.InlineKeyboardButton{{
			Text:         i18n("button.back"),
			CallbackData: "start",
		}})
	} else {
		buttons = append(buttons, []telego.InlineKeyboardButton{{
			Text:         i18n("button.back"),
			CallbackData: "config",
		}})
	}

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        i18n("menu.languageChangedSuccessfully"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
}

func createConfigKeyboard(i18n func(string) string) *telego.InlineKeyboardMarkup {
	return telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         i18n("medias.name"),
				CallbackData: "mediaConfig",
			},
		),
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         "LastFM",
				CallbackData: "lastFMConfig",
			},
		),
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         i18n("language.flag") + i18n("button.language"),
				CallbackData: "languageMenu",
			},
		),
	)
}

func handleConfig(bot *telego.Bot, message telego.Message) {
	i18n := localization.Get(message)

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:      telegoutil.ID(message.Chat.ID),
		Text:        i18n("config.menuText"),
		ParseMode:   "HTML",
		ReplyMarkup: createConfigKeyboard(i18n),
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func callbackConfig(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)
	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        i18n("config.menuText"),
		ParseMode:   "HTML",
		ReplyMarkup: createConfigKeyboard(i18n),
	})
}

func getMediaConfig(chatID int64) (bool, bool, error) {
	var mediasCaption, mediasAuto bool
	err := database.DB.QueryRow("SELECT mediasCaption, mediasAuto FROM groups WHERE id = ?;", chatID).Scan(&mediasCaption, &mediasAuto)
	return mediasCaption, mediasAuto, err
}

func updateMediaConfig(chatID int64, configType string, value bool) error {
	query := fmt.Sprintf("UPDATE groups SET %s = ? WHERE id = ?;", configType)
	_, err := database.DB.Exec(query, value, chatID)
	return err
}

func callbackMediaConfig(bot *telego.Bot, update telego.Update) {
	chatID := update.CallbackQuery.Message.GetChat().ID
	mediasCaption, mediasAuto, err := getMediaConfig(chatID)
	if err != nil {
		log.Print("Error querying media config:", err)
		return
	}

	configType := strings.ReplaceAll(update.CallbackQuery.Data, "mediaConfig ", "")
	if configType != "mediaConfig" {
		query := fmt.Sprintf("UPDATE groups SET %s = ? WHERE id = ?;", configType)
		var err error
		switch configType {
		case "mediasCaption":
			mediasCaption = !mediasCaption
			_, err = database.DB.Exec(query, mediasCaption, update.CallbackQuery.Message.GetChat().ID)
		case "mediasAuto":
			mediasAuto = !mediasAuto
			_, err = database.DB.Exec(query, mediasAuto, update.CallbackQuery.Message.GetChat().ID)
		}
		if err != nil {
			return
		}
	}
	i18n := localization.Get(update)

	state := func(mediasAuto bool) string {
		if mediasAuto {
			return "✅"
		}
		return "☑️"
	}

	buttons := [][]telego.InlineKeyboardButton{
		{
			{Text: i18n("button.caption"), CallbackData: "ieConfig mediasCaption"},
			{Text: state(mediasCaption), CallbackData: "mediaConfig mediasCaption"},
		},
		{
			{Text: i18n("button.automatic"), CallbackData: "ieConfig mediasAuto"},
			{Text: state(mediasAuto), CallbackData: "mediaConfig mediasAuto"},
		},
	}

	buttons = append(buttons, []telego.InlineKeyboardButton{{
		Text:         i18n("button.back"),
		CallbackData: "config",
	}})

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        i18n("medias.config"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("config")
	bh.HandleMessage(handleDisableable, telegohandler.Or(
		telegohandler.CommandEqual("disableables"),
		telegohandler.CommandEqual("disableable"),
	), helpers.IsAdmin(bot), helpers.IsGroup)
	bh.HandleMessage(handleDisable, telegohandler.CommandEqual("disable"), helpers.IsAdmin(bot), helpers.IsGroup)
	bh.HandleMessage(handleEnable, telegohandler.CommandEqual("enable"), helpers.IsAdmin(bot), helpers.IsGroup)
	bh.HandleMessage(handleDisabled, telegohandler.CommandEqual("disabled"), helpers.IsAdmin(bot), helpers.IsGroup)
	bh.Handle(callbackLanguageMenu, telegohandler.CallbackDataEqual("languageMenu"), helpers.IsAdmin(bot))
	bh.Handle(callbackLanguageSet, telegohandler.CallbackDataPrefix("setLang"), helpers.IsAdmin(bot))
	bh.HandleMessage(handleConfig, telegohandler.CommandEqual("config"), helpers.IsAdmin(bot), helpers.IsGroup)
	bh.Handle(callbackConfig, telegohandler.CallbackDataEqual("config"), helpers.IsAdmin(bot))
	bh.Handle(callbackMediaConfig, telegohandler.CallbackDataPrefix("mediaConfig"), helpers.IsAdmin(bot))
}
