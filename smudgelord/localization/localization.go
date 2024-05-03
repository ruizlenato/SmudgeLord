package localization

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"smudgelord/smudgelord/database"

	"github.com/goccy/go-yaml"
	"github.com/mymmrac/telego"
)

const defaultLanguage = "en-us"

var LangCache = make(map[string]map[string]interface{})

// LoadLanguages loads language files from the localtizations directory and populates the global cache.
// Each file in the directory should be a JSON file representing translations for a specific language.
// The language files are identified by their language code, derived from the file name without the extension.
// Loaded languages are stored in the cache for quick access during program execution.
// Additionally, the language codes are appended to the global list of available locales (AvailableLocales).
//
// Parameters:
//   - dir: The directory containing JSON files with translations for each language.
//
// Returns:
//   - error: Returns an error if there is any issue during the loading of languages.
//     Otherwise, it returns nil, indicating successful loading of languages.
func LoadLanguages() error {
	database.AvailableLocales = nil
	dir := "locales"

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && filepath.Ext(path) == ".yaml" {
			// Extract the language code from the file name without the extension
			langCode := filepath.Base(path[:len(path)-len(filepath.Ext(path))])

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Unmarshal the language YAML data and store it in the cache
			langMap := make(map[string]interface{})
			err = yaml.Unmarshal(data, &langMap)
			if err != nil {
				return err
			}

			LangCache[langCode] = langMap

			// Append the file name to the global variable availableLocales
			database.AvailableLocales = append(database.AvailableLocales, langCode)
		}

		return nil
	})

	return err
}

// getChatLanguage retrieves the language for a specific chat from the database.
//
// Parameters:
//   - chat: The telego.Chat representing the user or group for which to retrieve the language.
//
// Returns:
//   - string: The language code for the chat.
//   - error: An error if there is any issue retrieving the language from the database.
func GetChatLanguage(chat telego.Chat) (string, error) {
	var tableName, idColumn string
	if strings.Contains(chat.Type, "group") {
		tableName = "groups"
		idColumn = "id"
	} else {
		tableName = "users"
		idColumn = "id"
	}

	row := database.DB.QueryRow(fmt.Sprintf("SELECT language FROM %s WHERE %s = ?;", tableName, idColumn), chat.ID)
	var language string
	err := row.Scan(&language)
	return language, err
}

// Get returns a function that, given a message key, returns the translated message for a specific chat.
// It uses the language set for the chat, falling back to the default language if the chat's language is not found.
//
// Parameters:
//   - chat: The telego.Chat representing the user or group for which to retrieve the language.
//
// Returns:
//   - func(string) string: A function that, given a message key, returns the translated message.

func Get(chat telego.Chat) func(string) string {
	return func(key string) string {
		language, err := GetChatLanguage(chat)
		if err != nil {
			log.Printf("[localization/Get]: Error retrieving language for chat %v: %v", chat.ID, err)
			return "KEY_NOT_FOUND"
		}

		langMap, ok := LangCache[language]
		if !ok {
			// Use default language if the requested language is not found
			langMap, ok = LangCache[defaultLanguage]
			if !ok {
				return "KEY_NOT_FOUND"
			}
		}

		// Use a helper function to traverse the nested structure and get the final string
		value := GetStringFromNestedMap(langMap, key)
		return value
	}
}

// Helper function to traverse nested map and get the final string value
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

// HumanizeTimeSince returns a human-readable representation of the time duration since a given duration.
// It takes a duration and a chat as input parameters and returns a string.
// Parameters:
//   - duration: The time duration since a specific event.
//   - chat: Used to determine the localization settings for the returned string.
//
// Returns:
//   - string: A human-readable representation of the time duration since a specific event.
func HumanizeTimeSince(duration time.Duration, chat telego.Chat) string {
	// Get the i18n function for the chat
	i18n := Get(chat)

	var timeDuration int
	var stringKey string

	switch {
	case duration < time.Minute:
		timeDuration = int(duration.Seconds())
		stringKey = "relativeDuration.%s.s"
	case duration < time.Hour:
		timeDuration = int(duration.Minutes())
		stringKey = "relativeDuration.%s.m"
	case duration < 24*time.Hour:
		timeDuration = int(duration.Hours())
		stringKey = "relativeDuration.%s.h"
	case duration < 7*24*time.Hour:
		timeDuration = int(duration.Hours() / 24)
		stringKey = "relativeDuration.%s.d"
	case duration < 30*24*time.Hour:
		timeDuration = int(duration.Hours() / (24 * 7))
		stringKey = "relativeDuration.%s.w"
	default:
		timeDuration = int(duration.Hours() / (24 * 30))
		stringKey = "relativeDuration.%s.M"
	}

	// Use a helper function to get the translated string based on the string key and time duration
	timeSince := getTranslatedTimeSince(i18n, stringKey, timeDuration)
	return timeSince
}

// Helper function to get the translated time since string
func getTranslatedTimeSince(i18n func(string) string, stringKey string, timeDuration int) string {
	singularKey := fmt.Sprintf(stringKey, "singular")
	pluralKey := fmt.Sprintf(stringKey, "plural")

	timeSince := fmt.Sprintf("%d %s", timeDuration, i18n(singularKey))
	if timeDuration > 1 {
		timeSince = fmt.Sprintf("%d %s", timeDuration, i18n(pluralKey))
	}

	return timeSince
}
