package localization

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/lus/fluent.go/fluent"
	"golang.org/x/text/language"

	"github.com/ruizlenato/smudgelord/internal/database"
)

const defaultLanguage = "en-us"

var (
	LangBundles           = make(map[string]*fluent.Bundle)
	langBundlesMutex      sync.RWMutex
	availableLocalesMutex sync.Mutex
)

func LoadLanguages() error {
	database.AvailableLocales = nil
	dir := "internal/localization/locales"
	var wg sync.WaitGroup

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking through directory: %w", err)
		}

		if !info.IsDir() && filepath.Ext(path) == ".ftl" {
			wg.Add(1)
			go processLanguageFile(path, &wg)
		}
		return nil
	})

	wg.Wait()
	return err
}

func processLanguageFile(path string, wg *sync.WaitGroup) {
	defer wg.Done()

	langCode := filepath.Base(path[:len(path)-len(filepath.Ext(path))])
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("Couldn't read language file", "Path", path, "Error", err.Error())
		return
	}

	resource, parseErrors := fluent.NewResource(string(data))
	if len(parseErrors) > 0 {
		slog.Error("Couldn't parse language file", "Path", path, "Errors", parseErrors)
		return
	}

	langBundle := fluent.NewBundle(language.MustParse(langCode))
	if errs := langBundle.AddResource(resource); len(errs) > 0 {
		slog.Error("Couldn't add resource to language bundle", "LangCode", langCode, "Errors", errs)
		return
	}

	langBundlesMutex.Lock()
	LangBundles[langCode] = langBundle
	langBundlesMutex.Unlock()

	availableLocalesMutex.Lock()
	database.AvailableLocales = append(database.AvailableLocales, langCode)
	availableLocalesMutex.Unlock()
}

func createFormatContext(args map[string]any) *fluent.FormatContext {
	return fluent.WithVariables(args)
}

func GetChatLanguage(update *gotgbot.Update) (string, error) {
	var tableName string
	var chatID int64
	var chatType string

	switch {
	case update.Message != nil:
		chatID = update.Message.Chat.Id
		chatType = update.Message.Chat.Type
	case update.CallbackQuery != nil:
		if update.CallbackQuery.Message != nil {
			chat := update.CallbackQuery.Message.GetChat()
			chatID = chat.Id
			chatType = chat.Type
		} else {
			chatID = update.CallbackQuery.From.Id
			chatType = gotgbot.ChatTypePrivate
		}
	case update.InlineQuery != nil:
		chatID = update.InlineQuery.From.Id
		chatType = gotgbot.ChatTypePrivate
	case update.ChosenInlineResult != nil:
		chatID = update.ChosenInlineResult.From.Id
		chatType = gotgbot.ChatTypePrivate
	default:
		return "", errors.New("invalid update")
	}

	if chatType == gotgbot.ChatTypeGroup || chatType == gotgbot.ChatTypeSupergroup {
		tableName = "chats"
	} else {
		tableName = "users"
	}

	row := database.DB.QueryRow(fmt.Sprintf("SELECT language FROM %s WHERE id = ?;", tableName), chatID)
	var language string
	if err := row.Scan(&language); err != nil {
		return "", fmt.Errorf("failed to get language for chat ID %d: %w", chatID, err)
	}

	return language, nil
}

func Get(ctx *ext.Context) func(string, ...map[string]any) string {
	return func(key string, args ...map[string]any) string {
		language, err := GetChatLanguage(ctx.Update)
		if err != nil {
			slog.Error(err.Error())
			language = defaultLanguage
		}

		langBundlesMutex.RLock()
		bundle, ok := LangBundles[language]
		langBundlesMutex.RUnlock()

		if !ok {
			langBundlesMutex.RLock()
			bundle, ok = LangBundles[defaultLanguage]
			langBundlesMutex.RUnlock()
			if !ok {
				return fmt.Sprintf("Key '%s' not found.", key)
			}
		}

		variables := map[string]any{}
		if len(args) > 0 && args[0] != nil {
			variables = args[0]
		}

		message, _, err := bundle.FormatMessage(key, createFormatContext(variables))
		if err != nil {
			slog.Error("Couldn't format message", "Key", key, "Error", err.Error())
			return fmt.Sprintf("Key '%s' not found.", key)
		}

		return message
	}
}

func HumanizeTimeSinceGotgbot(duration time.Duration, ctx *ext.Context) string {
	var timeDuration int
	var stringKey string

	i18n := Get(ctx)
	switch {
	case duration < time.Minute:
		timeDuration = int(duration.Seconds())
		stringKey = "relative-duration-seconds"
	case duration < time.Hour:
		timeDuration = int(duration.Minutes())
		stringKey = "relative-duration-minutes"
	case duration < 24*time.Hour:
		timeDuration = int(duration.Hours())
		stringKey = "relative-duration-hours"
	default:
		timeDuration = int(duration.Hours() / 24)
		stringKey = "relative-duration-days"
	}

	return i18n(stringKey, map[string]any{"time": timeDuration})
}
