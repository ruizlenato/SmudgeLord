package menu

import (
	"fmt"
	"log"
	"strings"

	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func handleStart(bot *telego.Bot, update telego.Update) {
	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	message := update.Message
	if update.CallbackQuery != nil {
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	if messageFields := strings.Fields(message.Text); len(messageFields) > 1 && messageFields[1] == "privacy" {
		handlePrivacy(bot, update)
		return
	}

	i18n := localization.Get(message.Chat)

	keyboard := telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         i18n("button.about"),
				CallbackData: "aboutMenu",
			},
			telego.InlineKeyboardButton{
				Text:         fmt.Sprintf("%s %s", i18n("language.flag"), i18n("button.language")),
				CallbackData: "languageMenu",
			},
		),
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         i18n("button.privacy"),
				CallbackData: "privacy",
			},
			telego.InlineKeyboardButton{
				Text:         i18n("button.help"),
				CallbackData: "helpMenu",
			},
		),
	)

	if update.CallbackQuery != nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
			MessageID: update.CallbackQuery.Message.GetMessageID(),
			Text:      fmt.Sprintf(i18n("menu.startMessage"), message.Chat.FirstName, botUser.FirstName),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: keyboard,
		})
		return
	}

	if strings.Contains(message.Chat.Type, "group") {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      fmt.Sprintf(i18n("menu.startGroupMessage"), botUser.FirstName),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
			ReplyMarkup: telegoutil.InlineKeyboard(telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("button.start"),
					URL:  fmt.Sprintf("https://t.me/%s?start=start", botUser.Username),
				})),
		})
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      fmt.Sprintf(i18n("menu.startMessage"), message.From.FirstName, botUser.FirstName),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: keyboard,
	})
}

// handlePrivacy displays the Privacy Policy menu in response to a callback query or command.
func handlePrivacy(bot *telego.Bot, update telego.Update) {
	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	message := update.Message
	if update.CallbackQuery != nil {
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	i18n := localization.Get(message.Chat)

	if update.CallbackQuery != nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
			MessageID: update.CallbackQuery.Message.GetMessageID(),
			Text:      i18n("menu.privacyMessage"),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: telegoutil.InlineKeyboard(
				telegoutil.InlineKeyboardRow(
					telego.InlineKeyboardButton{
						Text:         i18n("button.aboutYourData"),
						CallbackData: "aboutYourData",
					},
				),
				telegoutil.InlineKeyboardRow(
					telego.InlineKeyboardButton{
						Text:         i18n("button.back"),
						CallbackData: "start",
					},
				),
			),
		})
		return
	}

	if strings.Contains(message.Chat.Type, "group") {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("menu.privacyGroupMessage"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
			ReplyMarkup: telegoutil.InlineKeyboard(telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("button.privacyPolicy"),
					URL:  fmt.Sprintf("https://t.me/%s?start=privacy", botUser.Username),
				})),
		})
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      i18n("menu.privacyMessage"),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("button.aboutYourData"),
					CallbackData: "aboutYourData",
				},
			),
		),
	})
}

func handleAboutYourData(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.(*telego.Message).GetChat()
	i18n := localization.Get(chat)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("menu.yourDataMessage"),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("button.back"),
					CallbackData: "privacy",
				},
			),
		),
	})
}

// aboutMenu displays the about menu in response to a callback query.
func handleAboutMenu(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.(*telego.Message).GetChat()
	i18n := localization.Get(chat)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("menu.aboutMessage"),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("button.donation"),
					URL:  "https://ko-fi.com/ruizlenato",
				},
				telego.InlineKeyboardButton{
					Text: i18n("button.newsChannel"),
					URL:  "https://t.me/SmudgeLordChannel",
				},
			),
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("button.back"),
					CallbackData: "start",
				},
			),
		),
	})
}

// helpMenu displays the help menu in response to a callback query.
// It edits the original message with the updated help menu text and inline keyboard.
func handleHelpMenu(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.(*telego.Message).GetChat()
	i18n := localization.Get(chat)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(chat.ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        i18n("menu.helpMessage"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(helpers.GetHelpKeyboard(i18n)...),
	})
}

func handleHelpMessage(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.(*telego.Message).GetChat()
	i18n := localization.Get(chat)
	module := strings.ReplaceAll(update.CallbackQuery.Data, "helpMessage ", "")

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n(fmt.Sprintf("%s.help", module)),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("button.back"),
					CallbackData: "helpMenu",
				},
			),
		),
	})
}

func handleConfigMenu(bot *telego.Bot, update telego.Update) {
	message := update.Message
	if message == nil {
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	chat := message.GetChat()
	i18n := localization.Get(chat)

	keyboard := telegoutil.InlineKeyboard(
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

	if update.Message == nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chat.ID),
			MessageID:   update.CallbackQuery.Message.GetMessageID(),
			Text:        i18n("menu.configMessage"),
			ParseMode:   "HTML",
			ReplyMarkup: keyboard,
		})
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:      telegoutil.ID(update.Message.Chat.ID),
			Text:        i18n("menu.configMessage"),
			ParseMode:   "HTML",
			ReplyMarkup: keyboard,
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
	}
}

func handleLanguageMenu(bot *telego.Bot, update telego.Update) {
	message := update.Message
	if message == nil {
		message = update.CallbackQuery.Message.(*telego.Message)
	}

	chat := message.GetChat()
	i18n := localization.Get(chat)

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

	// Query the database to retrieve the language info based on the chat type.
	row := database.DB.QueryRow("SELECT language FROM users WHERE id = ?;", chat.ID)
	if strings.Contains(chat.Type, "group") {
		row = database.DB.QueryRow("SELECT language FROM groups WHERE id = ?;", chat.ID)
	}
	var language string        // Variable to store the language information retrieved from the database.
	err := row.Scan(&language) // Scan method to retrieve the value of the "language" column from the query result.
	if err != nil {
		log.Print("[start/languageMenu] Error querying user:", err)
	}

	if update.Message == nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chat.ID),
			MessageID:   update.CallbackQuery.Message.GetMessageID(),
			Text:        fmt.Sprintf(i18n("menu.languageMessage"), i18n("language.flag"), i18n("language.name")),
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
		})
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:      telegoutil.ID(update.Message.Chat.ID),
			Text:        fmt.Sprintf(i18n("menu.languageMessage"), i18n("language.flag"), i18n("language.name")),
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
	}
}

// languageSet updates the language preference for a user or a group based on the provided CallbackQuery.
// It retrieves the language information from the CallbackQuery data, determines the appropriate database table (users or groups),
// and updates the language for the corresponding user or group in the database.
func handleLanguageSet(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update.CallbackQuery.Message.GetChat())
	lang := strings.ReplaceAll(update.CallbackQuery.Data, "setLang ", "")

	// Determine the appropriate database table based on the chat type.
	dbQuery := "UPDATE users SET language = ? WHERE id = ?;"
	if strings.Contains(update.CallbackQuery.Message.GetChat().Type, "group") {
		dbQuery = "UPDATE groups SET language = ? WHERE id = ?;"
	}
	_, err := database.DB.Exec(dbQuery, lang, update.CallbackQuery.Message.GetChat().ID)
	if err != nil {
		log.Print("[start/languageSet] Error updating language:", err)
	}

	buttons := make([][]telego.InlineKeyboardButton, 0, len(database.AvailableLocales))

	if update.CallbackQuery.Message.GetChat().Type == telego.ChatTypePrivate {
		buttons = append(buttons, []telego.InlineKeyboardButton{{
			Text:         i18n("button.back"),
			CallbackData: "start",
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

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	bh.Handle(handleStart, telegohandler.Or(
		telegohandler.CommandEqual("start"),
		telegohandler.CallbackDataEqual("start")))
	bh.Handle(handlePrivacy, telegohandler.Or(
		telegohandler.CommandEqual("privacy"),
		telegohandler.CallbackDataEqual("privacy")))
	bh.Handle(handleLanguageMenu, telegohandler.Or(
		telegohandler.CallbackDataEqual("languageMenu"),
		telegohandler.CommandEqual("lang")), helpers.IsAdmin(bot))
	bh.Handle(handleAboutYourData, telegohandler.CallbackDataEqual("aboutYourData"))
	bh.Handle(handleLanguageSet, telegohandler.CallbackDataPrefix("setLang"), helpers.IsAdmin(bot))
	bh.Handle(handleHelpMenu, telegohandler.CallbackDataEqual("helpMenu"))
	bh.Handle(handleAboutMenu, telegohandler.CallbackDataEqual("aboutMenu"))
	bh.Handle(handleHelpMessage, telegohandler.CallbackDataPrefix("helpMessage"))
	bh.Handle(handleConfigMenu, telegohandler.CommandEqual("config"), helpers.IsAdmin(bot), helpers.IsGroup)
	bh.Handle(handleConfigMenu, telegohandler.CallbackDataEqual("configMenu"), helpers.IsAdmin(bot))
}
