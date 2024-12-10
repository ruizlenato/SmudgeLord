package localization

import (
	"fmt"
	"log"
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
		log.Printf("localization/LoadLanguages — Error reading file %s: %v", path, err)
		return
	}

	resource, parseErrors := fluent.NewResource(string(data))
	if len(parseErrors) > 0 {
		log.Printf("localization/LoadLanguages — Errors parsing file %s: %v", path, parseErrors)
		return
	}

	langBundle := fluent.NewBundle(language.MustParse(langCode))
	if errs := langBundle.AddResource(resource); len(errs) > 0 {
		log.Printf("localization/LoadLanguages — Errors adding resource for %s: %v", langCode, errs)
		return
	}

	langBundlesMutex.Lock()
	defer langBundlesMutex.Unlock()
	LangBundles[langCode] = langBundle

	availableLocalesMutex.Lock()
	defer availableLocalesMutex.Unlock()
	database.AvailableLocales = append(database.AvailableLocales, langCode)
}

func GetChatLanguage(chatID int64, chatType string) (string, error) {
	var tableName string
	if chatType == "chat" {
		tableName = "chats"
	} else {
		tableName = "users"
	}

	row := database.DB.QueryRow(fmt.Sprintf("SELECT language FROM %s WHERE id = ?;", tableName), chatID)
	var language string
	err := row.Scan(&language)
	return language, err
}

type TelegramUpdate interface {
	ChatID() int64
}

func Get(update interface{}) func(string, ...map[string]interface{}) string {
	var chatID int64
	var chatType string
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
	}

	return func(key string, args ...map[string]interface{}) string {
		language, err := GetChatLanguage(chatID, chatType)
		if err != nil {
			log.Printf("Error retrieving language for chat %v: %v", chatID, err)
			return fmt.Sprintf("Key '%s' not found.", key)
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

		var variables map[string]interface{}
		if len(args) > 0 && args[0] != nil {
			variables = args[0]
		} else {
			variables = make(map[string]interface{})
		}

		context := createFormatContext(variables)
		message, _, err := bundle.FormatMessage(key, context)
		if err != nil {
			log.Printf("Error formatting message with key %s: %v", key, err)
			return fmt.Sprintf("Key '%s' not found.", key)
		}

		return message
	}
}

func createFormatContext(args map[string]interface{}) *fluent.FormatContext {
	return fluent.WithVariables(args)
}

func GetStringFromNestedMap(langMap map[string]interface{}, key string) string {
	keys := strings.Split(key, ".")
	currentMap := langMap

	for _, k := range keys {
		value, ok := currentMap[k]
		if !ok {
			return "KEY_NOT_FOUND"
		}

		if nestedMap, isMap := value.(map[string]interface{}); isMap {
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

	return i18n(stringKey, map[string]interface{}{"count": timeDuration})
}

func getTranslatedTimeSince(i18n func(string, ...interface{}) string, stringKey string, timeDuration int) string {
	singularKey := fmt.Sprintf("%s.singular", stringKey)
	pluralKey := fmt.Sprintf("%s.plural", stringKey)

	timeSince := fmt.Sprintf("%d %s", timeDuration, i18n(singularKey))
	if timeDuration > 1 {
		timeSince = fmt.Sprintf("%d %s", timeDuration, i18n(pluralKey))
	}

	return timeSince
}
