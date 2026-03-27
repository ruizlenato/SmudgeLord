package config

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func isGroupGotgbot(ctx *ext.Context) bool {
	return ctx.EffectiveChat != nil && (ctx.EffectiveChat.Type == gotgbot.ChatTypeGroup || ctx.EffectiveChat.Type == gotgbot.ChatTypeSupergroup)
}

func disableableHandlerGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	if !isGroupGotgbot(ctx) {
		i18n := localization.GetGotgbot(ctx)
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("only-groups"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
	var text strings.Builder
	text.WriteString(i18n("disableables-commands"))
	for _, command := range utils.DisableableCommands {
		text.WriteString("\n- <code>" + command + "</code>")
	}

	_, _ = b.SendMessage(ctx.EffectiveChat.Id, text.String(), &gotgbot.SendMessageOpts{
		ParseMode: gotgbot.ParseModeHTML,
		LinkPreviewOptions: &gotgbot.LinkPreviewOptions{
			PreferLargeMedia: true,
			ShowAboveText:    true,
		},
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
	})
	return nil
}

func disableHandlerGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	if !isGroupGotgbot(ctx) {
		i18n := localization.GetGotgbot(ctx)
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("only-groups"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
	fields := strings.Fields(ctx.EffectiveMessage.GetText())
	if len(fields) <= 1 {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("disable-commands-usage"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}

	command := fields[1]
	if !slices.Contains(utils.DisableableCommands, command) {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("command-not-deactivatable", map[string]any{"command": command}), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}

	if utils.CheckDisabledCommand(command, ctx.EffectiveChat.Id) {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("command-already-disabled", map[string]any{"command": command}), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}

	if err := insertDisabledCommand(ctx.EffectiveChat.Id, command); err != nil {
		slog.Error("Error inserting command", "error", err.Error())
		return nil
	}

	_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("command-disabled", map[string]any{"command": command}), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
	return nil
}

func enableHandlerGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	if !isGroupGotgbot(ctx) {
		i18n := localization.GetGotgbot(ctx)
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("only-groups"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
	fields := strings.Fields(ctx.EffectiveMessage.GetText())
	if len(fields) <= 1 {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("enable-commands-usage"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}

	command := fields[1]
	if !utils.CheckDisabledCommand(command, ctx.EffectiveChat.Id) {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("command-already-enabled", map[string]any{"command": command}), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}

	if err := deleteDisabledCommand(command); err != nil {
		slog.Error("Error deleting command", "error", err.Error())
		return nil
	}

	_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("command-enabled", map[string]any{"command": command}), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
	return nil
}

func disabledHandlerGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	if !isGroupGotgbot(ctx) {
		i18n := localization.GetGotgbot(ctx)
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("only-groups"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
	commands, err := getDisabledCommands(ctx.EffectiveChat.Id)
	if err != nil {
		return nil
	}
	if len(commands) == 0 {
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("no-disabled-commands"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
		return nil
	}

	var text strings.Builder
	text.WriteString(i18n("disabled-commands"))
	for _, command := range commands {
		text.WriteString("\n- <code>" + command + "</code>")
	}

	_, _ = b.SendMessage(ctx.EffectiveChat.Id, text.String(), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId}})
	return nil
}

func languageMenuCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)

	buttons := make([][]gotgbot.InlineKeyboardButton, 0, len(database.AvailableLocales))
	for _, lang := range database.AvailableLocales {
		loaded, ok := localization.LangBundles[lang]
		if !ok {
			slog.Error("Language not found in the cache", "lang", lang, "availableLocales", database.AvailableLocales)
			os.Exit(1)
		}
		languageFlag, _, _ := loaded.FormatMessage("language-flag")
		languageName, _, _ := loaded.FormatMessage("language-name")
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{Text: languageFlag + languageName, CallbackData: fmt.Sprintf("setLang %s", lang)}})
	}

	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()
	_, _, _ = b.EditMessageText(i18n("language-menu", map[string]any{"languageFlag": i18n("language-flag"), "languageName": i18n("language-name")}), &gotgbot.EditMessageTextOpts{
		ChatId:      chat.Id,
		MessageId:   msgID,
		ParseMode:   gotgbot.ParseModeHTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
	return nil
}

func setLanguageCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
	lang := strings.ReplaceAll(ctx.CallbackQuery.Data, "setLang ", "")
	chat := ctx.CallbackQuery.Message.GetChat()

	dbQuery := "UPDATE chats SET language = ? WHERE id = ?;"
	if chat.Type == gotgbot.ChatTypePrivate {
		dbQuery = "UPDATE users SET language = ? WHERE id = ?;"
	}
	if _, err := database.DB.Exec(dbQuery, lang, chat.Id); err != nil {
		slog.Error("Couldn't update language", "ChatID", chat.Id, "Error", err.Error())
	}

	buttons := make([][]gotgbot.InlineKeyboardButton, 0, 1)
	if chat.Type == gotgbot.ChatTypePrivate {
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{Text: i18n("back-button"), CallbackData: "start"}})
	} else {
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{Text: i18n("back-button"), CallbackData: "config"}})
	}

	_, _, _ = b.EditMessageText(i18n("language-changed"), &gotgbot.EditMessageTextOpts{
		ChatId:      chat.Id,
		MessageId:   ctx.CallbackQuery.Message.GetMessageId(),
		ParseMode:   gotgbot.ParseModeHTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
	return nil
}

func createConfigKeyboardGotgbot(i18n func(string, ...map[string]any) string) gotgbot.InlineKeyboardMarkup {
	return gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
		{{Text: i18n("medias"), CallbackData: "mediaConfig"}},
		{{Text: i18n("language-flag") + i18n("language-button"), CallbackData: "languageMenu"}},
	}}
}

func configHandlerGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}
	if !isGroupGotgbot(ctx) {
		i18n := localization.GetGotgbot(ctx)
		_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("only-groups"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
		return nil
	}

	i18n := localization.GetGotgbot(ctx)
	_, _ = b.SendMessage(ctx.EffectiveChat.Id, i18n("config-message"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: ctx.EffectiveMessage.MessageId},
		ReplyMarkup:     createConfigKeyboardGotgbot(i18n),
	})
	return nil
}

func configCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()

	_, _, _ = b.EditMessageText(i18n("config-message"), &gotgbot.EditMessageTextOpts{
		ChatId:      chat.Id,
		MessageId:   msgID,
		ParseMode:   gotgbot.ParseModeHTML,
		ReplyMarkup: createConfigKeyboardGotgbot(i18n),
	})
	return nil
}

func mediaConfigCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}
	chat := ctx.CallbackQuery.Message.GetChat()
	mediasCaption, mediasAuto, err := getMediaConfig(chat.Id)
	if err != nil {
		slog.Error("Couldn't query media config", "ChatID", chat.Id, "Error", err.Error())
		return nil
	}

	configType := strings.ReplaceAll(ctx.CallbackQuery.Data, "mediaConfig ", "")
	if configType != "mediaConfig" {
		query := fmt.Sprintf("UPDATE chats SET %s = ? WHERE id = ?;", configType)
		switch configType {
		case "mediasCaption":
			mediasCaption = !mediasCaption
			_, err = database.DB.Exec(query, mediasCaption, chat.Id)
		case "mediasAuto":
			mediasAuto = !mediasAuto
			_, err = database.DB.Exec(query, mediasAuto, chat.Id)
		}
		if err != nil {
			return nil
		}
	}

	i18n := localization.GetGotgbot(ctx)
	state := func(v bool) string {
		if v {
			return "✅"
		}
		return "☑️"
	}

	buttons := [][]gotgbot.InlineKeyboardButton{
		{{Text: i18n("caption-button"), CallbackData: "ieConfig caption-help"}, {Text: state(mediasCaption), CallbackData: "mediaConfig mediasCaption"}},
		{{Text: i18n("automatic-button"), CallbackData: "ieConfig auto-help"}, {Text: state(mediasAuto), CallbackData: "mediaConfig mediasAuto"}},
		{{Text: i18n("back-button"), CallbackData: "config"}},
	}

	_, _, _ = b.EditMessageText(i18n("config-medias"), &gotgbot.EditMessageTextOpts{
		ChatId:      chat.Id,
		MessageId:   ctx.CallbackQuery.Message.GetMessageId(),
		ParseMode:   gotgbot.ParseModeHTML,
		ReplyMarkup: gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})
	return nil
}

func explainConfigCallbackGotgbot(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil {
		return nil
	}
	i18n := localization.GetGotgbot(ctx)
	ieConfig := strings.ReplaceAll(ctx.CallbackQuery.Data, "ieConfig ", "")
	_, _ = b.AnswerCallbackQuery(ctx.CallbackQuery.Id, &gotgbot.AnswerCallbackQueryOpts{Text: i18n(ieConfig), ShowAlert: true})
	return nil
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("languageMenu"), languageMenuCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("setLang"), setLanguageCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCommand("config", configHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("config"), configCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("mediaConfig"), mediaConfigCallbackGotgbot))
	dispatcher.AddHandler(handlers.NewCommand("disableable", disableableHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCommand("disable", disableHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCommand("enable", enableHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCommand("disabled", disabledHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("disableable"), disableableHandlerGotgbot))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("ieConfig"), explainConfigCallbackGotgbot))

	utils.SaveHelp("config")
}
