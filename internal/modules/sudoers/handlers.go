package sudoers

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
)

var announceMessageText string

func buildTypeSelectionKeyboard(update any) []*telegram.KeyboardButtonRow {
	var keyboard []*telegram.KeyboardButtonRow

	i18n := localization.Get(update)

	groupsButton := telegram.ButtonBuilder{}.Data("👥 Groups", "announce_type groups")
	usersButton := telegram.ButtonBuilder{}.Data("👤 Users", "announce_type users")
	keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(groupsButton, usersButton))

	cancelButton := telegram.ButtonBuilder{}.Data(i18n("cancel-button"), "announce_type cancel")
	keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(cancelButton))

	return keyboard
}

func buildLanguageKeyboard(update *telegram.CallbackQuery, targetType string) ([]*telegram.KeyboardButtonRow, error) {
	var keyboard []*telegram.KeyboardButtonRow
	var currentRow []telegram.KeyboardButton

	i18n := localization.Get(update)

	for _, lang := range database.AvailableLocales {
		loaded, ok := localization.LangBundles[lang]
		if !ok {
			return nil, fmt.Errorf("language '%s' not found in the cache", lang)
		}

		languageFlag, _, _ := loaded.FormatMessage("language-flag")
		languageName, _, _ := loaded.FormatMessage("language-name")

		currentRow = append(currentRow, telegram.ButtonBuilder{}.Data(
			languageFlag+languageName,
			fmt.Sprintf("announce_lang %s %s", targetType, lang),
		))

		if len(currentRow) == 3 {
			keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(currentRow...))
			currentRow = nil
		}
	}

	if len(currentRow) > 0 {
		keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(currentRow...))
	}

	allButton := telegram.ButtonBuilder{}.Data(i18n("all-languages"), fmt.Sprintf("announce_lang %s all", targetType))
	keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(allButton))

	backButton := telegram.ButtonBuilder{}.Data(i18n("back-button"), "announce_back")
	cancelButton := telegram.ButtonBuilder{}.Data(i18n("cancel-button"), "announce_cancel")
	keyboard = append(keyboard, telegram.ButtonBuilder{}.Row(backButton, cancelButton))

	return keyboard, nil
}

func announceHandler(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	announceMessageText = message.Args()

	if announceMessageText == "" {
		_, err := message.Reply(i18n("announce-usage"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})

		return err
	}
	keyboard := buildTypeSelectionKeyboard(message)

	_, err := message.Reply(i18n("select-type-announcement"), telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(keyboard...),
	})
	return err
}

func sendAnnouncement(client *telegram.Client, targetType, language string) (int, int, error) {
	var query string
	switch targetType {
	case "groups":
		if language == "all" {
			query = "SELECT id FROM groups;"
		} else {
			query = fmt.Sprintf("SELECT id FROM groups WHERE language = '%s';", language)
		}
	case "users":
		if language == "all" {
			query = "SELECT id FROM users;"
		} else {
			query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s';", language)
		}
	default:
		if language == "all" {
			query = "SELECT id FROM users UNION SELECT id FROM groups;"
		} else {
			query = fmt.Sprintf("SELECT id FROM users WHERE language = '%s' UNION SELECT id FROM groups WHERE language = '%s';", language, language)
		}
	}

	rows, err := database.DB.Query(query)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			slog.Error(
				"Error scanning ID",
				"error", err.Error(),
			)
			continue
		}
		ids = append(ids, id)
	}

	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	var wg sync.WaitGroup
	var successCount, errorCount, sentCount int64

	semaphore := make(chan struct{}, 40)

	for _, id := range ids {
		wg.Add(1)
		go func(chatID int64) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			_, err := client.SendMessage(chatID, announceMessageText, &telegram.SendOptions{
				ParseMode: telegram.HTML,
			})

			if err != nil {
				slog.Error(
					"Error sending announcement",
					"chat_id", chatID,
					"error", err.Error(),
				)
				atomic.AddInt64(&errorCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}

			currentSent := atomic.AddInt64(&sentCount, 1)

			slog.Debug(
				"Announcement sent",
				"chat_id", chatID,
				"sent", currentSent,
				"success", atomic.LoadInt64(&successCount),
				"failed", atomic.LoadInt64(&errorCount),
			)

			if currentSent%40 == 0 {
				time.Sleep(2 * time.Second)
			}
		}(id)
	}

	wg.Wait()
	return int(atomic.LoadInt64(&successCount)), int(atomic.LoadInt64(&errorCount)), nil
}

func announceCallback(update *telegram.CallbackQuery) error {
	data := update.DataString()
	i18n := localization.Get(update)

	if strings.HasPrefix(data, "announce_type") {
		targetType := strings.ReplaceAll(data, "announce_type ", "")

		if targetType == "cancel" {
			_, err := update.Edit(i18n("announcement-cancelled"), &telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return err
		}

		keyboard, err := buildLanguageKeyboard(update, targetType)
		if err != nil {
			slog.Error(
				"Error building language keyboard",
				"error", err.Error(),
			)
			_, editErr := update.Edit("Error creating language selection menu", &telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return editErr
		}

		_, err = update.Edit(i18n("select-language-announcement", map[string]any{
			"targetType": targetType,
		}), &telegram.SendOptions{
			ParseMode:   telegram.HTML,
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(keyboard...),
		})

		return err
	}

	if strings.HasPrefix(data, "announce_lang") {
		parts := strings.Split(strings.ReplaceAll(data, "announce_lang ", ""), " ")
		if len(parts) != 2 {
			return fmt.Errorf("invalid callback data format")
		}

		targetType := parts[0]
		language := parts[1]

		_, err := update.Edit(i18n("announcement-configured", map[string]any{
			"targetType": targetType,
			"language":   language,
		}), &telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		if err != nil {
			slog.Error(err.Error())
			return err
		}

		var query string
		switch targetType {
		case "groups":
			if language == "all" {
				query = "SELECT COUNT(*) FROM groups;"
			} else {
				query = fmt.Sprintf("SELECT COUNT(*) FROM groups WHERE language = '%s';", language)
			}
		case "users":
			if language == "all" {
				query = "SELECT COUNT(*) FROM users;"
			} else {
				query = fmt.Sprintf("SELECT COUNT(*) FROM users WHERE language = '%s';", language)
			}
		default:
			if language == "all" {
				query = "SELECT (SELECT COUNT(*) FROM users) + (SELECT COUNT(*) FROM groups) AS total_count;"
			} else {
				query = fmt.Sprintf("SELECT (SELECT COUNT(*) FROM users WHERE language = '%s') + (SELECT COUNT(*) FROM groups WHERE language = '%s') AS total_count;", language, language)
			}
		}

		var totalTargets int
		err = database.DB.QueryRow(query).Scan(&totalTargets)
		if err != nil {
			slog.Error(
				"Error counting targets",
				"error", err.Error(),
			)
			_, editErr := update.Edit("Error counting announcement targets", &telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return editErr
		}

		_, err = update.Edit(i18n("sending-announcement", map[string]any{
			"targetType":   targetType,
			"language":     language,
			"totalTargets": totalTargets,
		}), &telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		if err != nil {
			slog.Error(err.Error())
			return err
		}

		successCount, failedCount, err := sendAnnouncement(update.Client, targetType, language)
		if err != nil {
			slog.Error(
				"Error sending announcement",
				"error", err.Error(),
			)
			_, editErr := update.Edit(i18n("announcement-error"), &telegram.SendOptions{
				ParseMode: telegram.HTML,
			})
			return editErr
		}

		_, err = update.Edit(i18n("sended-announcement", map[string]any{
			"targetType":   targetType,
			"language":     language,
			"totalTargets": totalTargets,
			"successCount": successCount,
			"failedCount":  failedCount,
		}), &telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	if data == "announce_back" {
		keyboard := buildTypeSelectionKeyboard(update)

		_, err := update.Edit(i18n("select-type-announcement"), &telegram.SendOptions{
			ParseMode:   telegram.HTML,
			ReplyMarkup: telegram.ButtonBuilder{}.Keyboard(keyboard...),
		})

		return err
	}

	if data == "announce_cancel" {
		_, err := update.Edit(i18n("announcement-cancelled"), &telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	return nil
}

func Load(client *telegram.Client) {
	client.On("command:announce", handlers.HandleCommand(announceHandler), telegram.Filter{Func: helpers.IsBotOwner})
	client.On("callback:^announce", announceCallback, telegram.Filter{Func: helpers.IsBotOwner})
}
