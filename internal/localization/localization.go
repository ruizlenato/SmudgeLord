package localization

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/go-telegram/bot/models"
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
		slog.Error("Couldn't read language file",
			"Path", path,
			"Error", err.Error())
		return
	}

	resource, parseErrors := fluent.NewResource(string(data))
	if len(parseErrors) > 0 {
		slog.Error("Couldn't parse language file",
			"Path", path,
			"Errors", parseErrors)
		return
	}

	langBundle := fluent.NewBundle(language.MustParse(langCode))
	if errs := langBundle.AddResource(resource); len(errs) > 0 {
		slog.Error("Couldn't add resource to language bundle",
			"LangCode", langCode,
			"Errors", errs)
		return
	}

	langBundlesMutex.Lock()
	defer langBundlesMutex.Unlock()
	LangBundles[langCode] = langBundle

	availableLocalesMutex.Lock()
	defer availableLocalesMutex.Unlock()
	database.AvailableLocales = append(database.AvailableLocales, langCode)
}

func GetChatLanguage(update *models.Update) (string, error) {
	var tableName, idColumn string
	var chatID int64
	var chatType models.ChatType

	if update.Message != nil {
		chatID = update.Message.Chat.ID
		chatType = update.Message.Chat.Type
	} else if update.CallbackQuery != nil {
		chatID = update.CallbackQuery.Message.Message.Chat.ID
		chatType = update.CallbackQuery.Message.Message.Chat.Type
	} else if update.InlineQuery != nil {
		chatID = update.InlineQuery.From.ID
		chatType = models.ChatTypePrivate
	} else if update.ChosenInlineResult != nil {
		chatID = update.ChosenInlineResult.From.ID
		chatType = models.ChatTypePrivate
	} else {
		return "", errors.New("invalid update")
	}

	if chatType == models.ChatTypeGroup || chatType == models.ChatTypeSupergroup {
		tableName = "chats"
		idColumn = "id"
	} else {
		tableName = "users"
		idColumn = "id"
	}

	row := database.DB.QueryRow(fmt.Sprintf("SELECT language FROM %s WHERE %s = ?;", tableName, idColumn), chatID)
	var language string
	err := row.Scan(&language)
	if err != nil {
		return "", fmt.Errorf("failed to get language for chat ID %d: %w", chatID, err)
	}
	return language, nil
}

func Get(update *models.Update) func(string, ...map[string]any) string {
	return func(key string, args ...map[string]any) string {
		language, err := GetChatLanguage(update)
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

		var variables map[string]any
		if len(args) > 0 && args[0] != nil {
			variables = args[0]
		} else {
			variables = make(map[string]any)
		}

		context := createFormatContext(variables)
		message, _, err := bundle.FormatMessage(key, context)
		if err != nil {
			slog.Error("Couldn't format message",
				"Key", key,
				"Error", err.Error())
			return fmt.Sprintf("Key '%s' not found.", key)
		}

		return message
	}
}

func createFormatContext(args map[string]any) *fluent.FormatContext {
	return fluent.WithVariables(args)
}

func HumanizeTimeSince(duration time.Duration, update *models.Update) string {
	var timeDuration int
	var stringKey string

	i18n := Get(update)
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
	case duration < 7*24*time.Hour:
		timeDuration = int(duration.Hours() / 24)
		stringKey = "relative-duration-days"
	case duration < 30*24*time.Hour:
		timeDuration = int(duration.Hours() / (24 * 7))
		stringKey = "relative-duration-weeks"
	default:
		timeDuration = int(duration.Hours() / (24 * 30))
		stringKey = "relative-duration-months"
	}

	return i18n(stringKey, map[string]any{"count": timeDuration})
}
