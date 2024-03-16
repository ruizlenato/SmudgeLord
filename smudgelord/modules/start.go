package modules

import (
	"fmt"
	"log"
	"smudgelord/smudgelord/database"
	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils/helpers"
	"strings"

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
			Text:      fmt.Sprintf(i18n("start_message_private"), Message.Chat.FirstName, botUser.FirstName),
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
				),
			),
		})
		return
	}

	i18n := localization.Get(update.Message.Chat)

	if strings.Contains(update.Message.Chat.Type, "group") {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      fmt.Sprintf(i18n("start_message_group"), botUser.FirstName),
			ParseMode: "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("start_button"),
					URL:  fmt.Sprintf("https://t.me/%s?start=start", botUser.Username),
				})),
		})
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(update.Message.Chat.ID),
		Text:      fmt.Sprintf(i18n("start_message_private"), update.Message.From.FirstName, botUser.FirstName),
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
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(update.Message.Chat.ID),
			Text:      fmt.Sprintf(i18n("start_message_private"), update.Message.From.FirstName, botUser.FirstName),
			ParseMode: "HTML",
			LinkPreviewOptions: &telego.LinkPreviewOptions{
				IsDisabled: true,
			},
		})
	}
			),
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
		log.Print(err)
	}

	if update.Message == nil {
		bot.EditMessageText(&telego.EditMessageTextParams{
			ChatID:      telegoutil.ID(chat.ID),
			MessageID:   update.CallbackQuery.Message.GetMessageID(),
			Text:        fmt.Sprintf(i18n("language_menu_mesage"), i18n("language.flag"), i18n("language.name")),
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
		})
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:      telegoutil.ID(update.Message.Chat.ID),
			Text:        fmt.Sprintf(i18n("language_menu_mesage"), i18n("language.flag"), i18n("language.name")),
			ParseMode:   "HTML",
			ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
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
		log.Print("Error inserting user:", err)
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
		Text:        i18n("language_changed_successfully"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
}

func LoadStart(bh *telegohandler.BotHandler, bot *telego.Bot) {
	bh.Handle(start, telegohandler.CommandEqual("start"))
	bh.Handle(start, telegohandler.CallbackDataEqual("start"))
	bh.Handle(languageMenu, telegohandler.CommandEqual("lang"), helpers.IsAdmin(bot))
	bh.Handle(languageMenu, telegohandler.CallbackDataEqual("languageMenu"))
	bh.Handle(languageSet, telegohandler.CallbackDataPrefix("setLang"), helpers.IsAdmin(bot))
}
