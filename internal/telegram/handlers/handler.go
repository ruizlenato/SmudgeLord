package handlers

import (
	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database"
)

func HanndleCommand(handler func(m *telegram.NewMessage) error) func(m *telegram.NewMessage) error {
	return func(m *telegram.NewMessage) error {
		database.SaveUsers(m)
		return handler(m)
	}
}
