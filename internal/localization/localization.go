package localization

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/amarnathcjd/gogram/telegram"

	"github.com/ruizlenato/smudgelord/internal/database"

	"github.com/lus/fluent.go/fluent"
	"golang.org/x/text/language"
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
		slog.Error(
			"Couldn't read language file",
			"Path", path,
			"Error", err.Error(),
		)
		return
	}

	resource, parseErrors := fluent.NewResource(string(data))
	if len(parseErrors) > 0 {
		slog.Error(
			"Couldn't parse language file",
			"Path", path,
			"Errors", parseErrors,
		)
		return
	}

	langBundle := fluent.NewBundle(language.MustParse(langCode))
	if errs := langBundle.AddResource(resource); len(errs) > 0 {
		slog.Error(
			"Couldn't add resource to language bundle",
			"LangCode", langCode,
			"Errors", errs,
		)
		return
	}

	langBundlesMutex.Lock()
	defer langBundlesMutex.Unlock()
	LangBundles[langCode] = langBundle

	availableLocalesMutex.Lock()
	defer availableLocalesMutex.Unlock()
	database.AvailableLocales = append(database.AvailableLocales, langCode)
}

func GetChatLanguage(update any) (string, error) {
	var chatID int64
	var chatType string

	switch u := update.(type) {
	case *telegram.NewMessage:
		chatID = u.ChatID()
		chatType = u.ChatType()
		if u.ChatType() == "user" {
			chatType = "user"
		}
	case *telegram.CallbackQuery:
		chatID = u.GetChatID()
		chatType = "chat"
		if u.ChatType() == "user" {
			chatType = "user"
		}
	case *telegram.InlineQuery:
		chatID = u.SenderID
		chatType = "user"
	case *telegram.InlineSend:
		chatID = u.SenderID
		chatType = "user"
	}

	row := database.DB.QueryRow("SELECT language FROM chats WHERE id = ?;", chatID)
	if chatType == telegram.EntityUser {
		row = database.DB.QueryRow("SELECT language FROM users WHERE id = ?;", chatID)
	}

	var language string
	err := row.Scan(&language)
	return language, err
}

func isInlineUpdate(update any) bool {
	switch update.(type) {
	case *telegram.InlineQuery, *telegram.InlineSend:
		return true
	default:
		return false
	}
}

func Get(update any) func(string, ...map[string]any) string {
	var chatID int64
	var chatType string
	var sender *telegram.UserObj

	switch u := update.(type) {
	case *telegram.NewMessage:
		chatID = u.ChatID()
		chatType = "chat"
		if u.ChatType() == "user" {
			chatType = "user"
		}
	case *telegram.CallbackQuery:
		chatID = u.GetChatID()
		chatType = "chat"
		if u.ChatType() == "user" {
			chatType = "user"
		}
	case *telegram.InlineQuery:
		chatID = u.SenderID
		chatType = "user"
		sender = u.Sender
	case *telegram.InlineSend:
		chatID = u.SenderID
		chatType = "user"
		sender = u.Sender
	}

	return func(key string, args ...map[string]any) string {
		language, err := GetChatLanguage(update)
		if err != nil && !isInlineUpdate(update) {
			slog.Warn(
				"Couldn't get chat language initially, attempting fallback",
				"ChatID", chatID,
				"ChatType", chatType,
				"Error", err.Error(),
			)

			if chatType == "user" && sender != nil {
				database.SaveUsers(update)
			}

			language, err = GetChatLanguage(update)
			if err != nil {
				slog.Error(
					"Couldn't get chat language",
					"ChatID", chatID,
					"ChatType", chatType,
					"Error", err.Error(),
				)
				language = defaultLanguage
			}
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
			slog.Error(
				"Couldn't format message",
				"Key", key,
				"Error", err.Error(),
			)
			return fmt.Sprintf("Key '%s' not found.", key)
		}

		return message
	}
}

func createFormatContext(args map[string]any) *fluent.FormatContext {
	return fluent.WithVariables(args)
}

func GetStringFromNestedMap(langMap map[string]any, key string) string {
	keys := strings.Split(key, ".")
	currentMap := langMap

	for _, k := range keys {
		value, ok := currentMap[k]
		if !ok {
			return "KEY_NOT_FOUND"
		}

		if nestedMap, isMap := value.(map[string]any); isMap {
			currentMap = nestedMap
		} else if strValue, isString := value.(string); isString {
			return strValue
		} else {
			return "KEY_NOT_FOUND"
		}
	}

	return "KEY_NOT_FOUND"
}

func HumanizeTimeSince(duration time.Duration, message *telegram.NewMessage) string {
	var timeDuration int
	var stringKey string

	i18n := Get(message)
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
