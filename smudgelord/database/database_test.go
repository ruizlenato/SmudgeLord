package database_test

import (
	"smudgelord/smudgelord/database"
	"testing"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
)

// TODO: use testify for assertions
func TestSaveUser(t *testing.T) {
	err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		database.Close()
		database.DB = nil
	})

	err = database.CreateTables()
	if err != nil {
		t.Fatal(err)
	}

	database.SaveUsers(nil, telego.Update{
		Message: &telego.Message{
			From: &telego.User{
				ID:           123,
				LanguageCode: "en-us",
				Username:     "ruizlenato",
			},
		},
	}, telegohandler.Handler(func(*telego.Bot, telego.Update) {}))

	var u struct {
		id       int64
		lang     string
		username string
	}
	err = database.DB.
		QueryRow(`SELECT id, language, username FROM users where id = 123`).
		Scan(&u.id, &u.lang, &u.username)

	if err != nil {
		t.Fatal(err)
	}
	if u.id != 123 {
		t.Fatalf("id - want: %v, got: %v", 123, u.id)
	}
	if u.lang != "en-us" {
		t.Fatalf("lang - want: %v, got: %v", "en-us", u.lang)
	}
	if u.username != "@ruizlenato" {
		t.Fatalf("lang - want: %v, got: %v", "@ruizlenato", u.username)
	}
}
