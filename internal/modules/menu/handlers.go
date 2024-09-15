package menu

import (
	"fmt"
	"log"
	"strings"

	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func createStartKeyboard(i18n func(string, ...map[string]interface{}) string) *telego.InlineKeyboardMarkup {
	return telegoutil.InlineKeyboard(
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         i18n("about-button"),
				CallbackData: "about",
			},
			telego.InlineKeyboardButton{
				Text:         fmt.Sprintf("%s %s", i18n("language-flag"), i18n("language-button")),
				CallbackData: "languageMenu",
			},
		),
		telegoutil.InlineKeyboardRow(
			telego.InlineKeyboardButton{
				Text:         i18n("help-button"),
				CallbackData: "helpMenu",
			},
		),
	)
}

func handleStart(bot *telego.Bot, message telego.Message) {
	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	if messageFields := strings.Fields(message.Text); len(messageFields) > 1 && messageFields[1] == "privacy" {
		handlePrivacy(bot, message)
		return
	}

	i18n := localization.Get(message)

	if strings.Contains(message.Chat.Type, "group") {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID: telegoutil.ID(message.Chat.ID),
			Text: i18n("start-message-group",
				map[string]interface{}{
					"botName": botUser.FirstName,
				}),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
			ReplyMarkup: telegoutil.InlineKeyboard(telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("start-button"),
					URL:  fmt.Sprintf("https://t.me/%s?start=start", botUser.Username),
				})),
		})
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID: telegoutil.ID(message.Chat.ID),
		Text: i18n("start-message",
			map[string]interface{}{
				"userName": message.From.FirstName,
				"botName":  botUser.FirstName,
			}),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: createStartKeyboard(i18n),
	})
}

func callbackStart(bot *telego.Bot, update telego.Update) {
	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}
	i18n := localization.Get(update)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text: i18n("start-message",
			map[string]interface{}{
				"userName": update.CallbackQuery.Message.GetChat().FirstName,
				"botName":  botUser.FirstName,
			}),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: createStartKeyboard(i18n),
	})
}

func handlePrivacy(bot *telego.Bot, message telego.Message) {
	botUser, err := bot.GetMe()
	if err != nil {
		log.Fatal(err)
	}

	i18n := localization.Get(message)

	if strings.Contains(message.Chat.Type, "group") {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("privacy-policy-group"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
			ReplyMarkup: telegoutil.InlineKeyboard(telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("privacy-policy-button"),
					URL:  fmt.Sprintf("https://t.me/%s?start=privacy", botUser.Username),
				})),
		})
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      i18n("privacy-policy-private"),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("about-your-data-button"),
					CallbackData: "aboutYourData",
				},
			),
		),
	})
}

func callbackPrivacy(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)
	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("privacy-policy-private"),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("about-your-data-button"),
					CallbackData: "aboutYourData",
				},
			),
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("back-button"),
					CallbackData: "about",
				},
			),
		),
	})
}

func callbackAboutYourData(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("about-your-data"),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("back-button"),
					CallbackData: "privacy",
				},
			),
		),
	})
}

func callbackAboutMenu(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n("about"),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text: i18n("donation-button"),
					URL:  "https://ko-fi.com/ruizlenato",
				},
				telego.InlineKeyboardButton{
					Text: i18n("news-channel-button"),
					URL:  "https://t.me/SmudgeLordChannel",
				},
			),
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("privacy-policy-button"),
					CallbackData: "privacy",
				},
			),
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("back-button"),
					CallbackData: "start",
				},
			),
		),
	})
}

func callbackHelpMenu(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:      telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID:   update.CallbackQuery.Message.GetMessageID(),
		Text:        i18n("help"),
		ParseMode:   "HTML",
		ReplyMarkup: telegoutil.InlineKeyboard(helpers.GetHelpKeyboard(i18n)...),
	})
}

func callbackHelpMessage(bot *telego.Bot, update telego.Update) {
	i18n := localization.Get(update)
	module := strings.ReplaceAll(update.CallbackQuery.Data, "helpMessage ", "")

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text:      i18n(fmt.Sprintf("%s-help", module)),
		ParseMode: "HTML",
		LinkPreviewOptions: &telego.LinkPreviewOptions{
			IsDisabled: true,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(
			telegoutil.InlineKeyboardRow(
				telego.InlineKeyboardButton{
					Text:         i18n("back-button"),
					CallbackData: "helpMenu",
				},
			),
		),
	})
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	bh.HandleMessage(handleStart, telegohandler.CommandEqual("start"))
	bh.Handle(callbackStart, telegohandler.CallbackDataEqual("start"))
	bh.HandleMessage(handlePrivacy, telegohandler.CommandEqual("privacy"))
	bh.Handle(callbackPrivacy, telegohandler.CallbackDataEqual("privacy"))
	bh.Handle(callbackAboutYourData, telegohandler.CallbackDataEqual("aboutYourData"))
	bh.Handle(callbackHelpMenu, telegohandler.CallbackDataEqual("helpMenu"))
	bh.Handle(callbackAboutMenu, telegohandler.CallbackDataEqual("about"))
	bh.Handle(callbackHelpMessage, telegohandler.CallbackDataPrefix("helpMessage"))
}
