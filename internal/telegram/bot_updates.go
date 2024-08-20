package telegram

import (
	"fmt"

	"github.com/ruizlenato/smudgelord/internal/config"

	"github.com/fasthttp/router"
	"github.com/mymmrac/telego"
	"github.com/valyala/fasthttp"
)

func SetupWebhook(bot *telego.Bot) (<-chan telego.Update, error) {
	err := bot.SetWebhook(&telego.SetWebhookParams{
		DropPendingUpdates: true,
		URL:                config.WebhookURL + bot.Token(),
	})
	if err != nil {
		return nil, fmt.Errorf("set webhook: %w", err)
	}
	return bot.UpdatesViaWebhook("/bot"+bot.Token(),
		telego.WithWebhookServer(telego.FastHTTPWebhookServer{
			Logger: bot.Logger(),
			Server: &fasthttp.Server{},
			Router: router.New(),
		}),
	)
}

func SetupLongPolling(bot *telego.Bot) (<-chan telego.Update, error) {
	err := bot.DeleteWebhook(&telego.DeleteWebhookParams{
		DropPendingUpdates: true,
	})
	if err != nil {
		return nil, fmt.Errorf("delete webhook: %w", err)
	}
	return bot.UpdatesViaLongPolling(&telego.GetUpdatesParams{
		Timeout: 4,
	}, telego.WithLongPollingUpdateInterval(0))
}
