package modules

import (
	"fmt"
	"log"
	"strings"

	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func start(bot *telego.Bot, update telego.Update) {
	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	if update.Message == nil {
		Message := update.CallbackQuery.Message.(*telego.Message)
		i18n := localization.Get(Message.Chat)
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
			MessageID: update.CallbackQuery.Message.GetMessageID(),
			Text:      fmt.Sprintf(i18n("start.message-private"), Message.Chat.FirstName, botUser.FirstName),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
			ReplyMarkup: telegoutil.InlineKeyboard(
				telegoutil.InlineKeyboardRow(
					telego.InlineKeyboardButton{
						Text:         i18n("button.language"),
						CallbackData: "languageMenu",
					},
					telego.InlineKeyboardButton{
						Text:         i18n("button.help"),
						CallbackData: "helpMenu",
					},
				),
				telegoutil.InlineKeyboardRow(telego.InlineKeyboardButton{
					Text:         i18n("button.about"),
					CallbackData: "aboutMenu",
				}),
			),
		})
		return
	}

	i18n := localization.Get(update.Message.Chat)

	if strings.Contains(update.Message.Chat.Type, "group") {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      fmt.Sprintf(i18n("start.message-group"), botUser.FirstName),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: update.Message.MessageID,
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
		ChatID:    telegoutil.ID(update.Message.Chat.ID),
		Text:      fmt.Sprintf(i18n("start.message-private"), update.Message.From.FirstName, botUser.FirstName),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("button.language"),
					CallbackData: "languageMenu",
				},
				telego.InlineKeyboardButton{
					Text:         i18n("button.help"),
					CallbackData: "helpMenu",
				},
			),
			telegoutil.InlineKeyboardRow(telego.InlineKeyboardButton{
				Text:         i18n("button.about"),
				CallbackData: "aboutMenu",
			}),
		),
	})
}

func languageMenu(bot *telego.Bot, update telego.Update) {
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
			Text:        fmt.Sprintf(i18n("menu.language-mesage"), i18n("language.flag"), i18n("language.name")),
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
		})
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:      telegoutil.ID(update.Message.Chat.ID),
			Text:        fmt.Sprintf(i18n("menu.language-mesage"), i18n("language.flag"), i18n("language.name")),
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
func languageSet(bot *telego.Bot, update telego.Update) {
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
		Text:        i18n("menu.language-changed-successfully"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
}

// helpMenu displays the help menu in response to a callback query.
// It edits the original message with the updated help menu text and inline keyboard.
func helpMenu(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.(*telego.Message).GetChat()
	i18n := localization.Get(chat)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(chat.ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        i18n("menu.help-message"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(helpers.GetHelpKeyboard(i18n)...),
	})
}

// aboutMenu displays the about menu in response to a callback query.
func aboutMenu(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.(*telego.Message).GetChat()
	i18n := localization.Get(chat)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("menu.about-message"),
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
			),
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("button.news-channel"),
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

func helpMessage(bot *telego.Bot, update telego.Update) {
	chat := update.CallbackQuery.Message.(*telego.Message).GetChat()
	i18n := localization.Get(chat)
	module := strings.ReplaceAll(update.CallbackQuery.Data, "helpMessage ", "")

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(chat.ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n(fmt.Sprintf("%s.help", module)),
		ParseMode: "HTML",
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

func configMenu(bot *telego.Bot, update telego.Update) {
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
				Text:         i18n("button.language"),
				CallbackData: "languageMenu",
			},
		),
	)

	if update.Message == nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chat.ID),
			MessageID:   update.CallbackQuery.Message.GetMessageID(),
			Text:        i18n("menu.config-message"),
			ParseMode:   "HTML",
			ReplyMarkup: keyboard,
		})
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:      telegoutil.ID(update.Message.Chat.ID),
			Text:        i18n("menu.config-message"),
			ParseMode:   "HTML",
			ReplyMarkup: keyboard,
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
	}
}

func LoadStart(bh *telegohandler.BotHandler, bot *telego.Bot) {
	bh.Handle(start, telegohandler.Or(telegohandler.CommandEqual("start"), telegohandler.CallbackDataEqual("start")))
	bh.Handle(languageMenu, telegohandler.Or(telegohandler.CallbackDataEqual("languageMenu"), telegohandler.CommandEqual("lang")), helpers.IsAdmin(bot))
	bh.Handle(languageSet, telegohandler.CallbackDataPrefix("setLang"), helpers.IsAdmin(bot))
	bh.Handle(helpMenu, telegohandler.CallbackDataEqual("helpMenu"))
	bh.Handle(aboutMenu, telegohandler.CallbackDataEqual("aboutMenu"))
	bh.Handle(helpMessage, telegohandler.CallbackDataPrefix("helpMessage"))
	bh.Handle(configMenu, telegohandler.CommandEqual("config"), helpers.IsAdmin(bot), helpers.IsGroup)
	bh.Handle(configMenu, telegohandler.CallbackDataEqual("configMenu"), helpers.IsAdmin(bot))
}
