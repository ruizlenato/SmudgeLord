package modules

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"smudgelord/smudgelord/localization"
	"smudgelord/smudgelord/utils"
	"smudgelord/smudgelord/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

const (
	weatherAPIKey = "8de2d8b3a93542c9a2d8b3a935a2c909"
)

type weatherSearch struct {
	Location struct {
		Latitude  []float64 `json:"latitude"`
		Longitude []float64 `json:"longitude"`
		Address   []string  `json:"address"`
	} `json:"location"`
}

type weatherResult struct {
	ID                      string `json:"id"`
	V3WxObservationsCurrent struct {
		IconCode             int    `json:"iconCode"`
		RelativeHumidity     int    `json:"relativeHumidity"`
		Temperature          int    `json:"temperature"`
		TemperatureFeelsLike int    `json:"temperatureFeelsLike"`
		WindSpeed            int    `json:"windSpeed"`
		WxPhraseLong         string `json:"wxPhraseLong"`
	} `json:"v3-wx-observations-current"`
}

func weather(bot *telego.Bot, message telego.Message) {
	var weatherQuery string

	lang, _ := localization.GetChatLanguage(message.Chat)
	i18n := localization.Get(message.Chat)

	if len(strings.Fields(message.Text)) > 1 {
		weatherQuery = strings.Fields(message.Text)[1]
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("weather.no-location"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	var weatherData weatherSearch // Declare weatherData variable

	body := utils.RequestGET("https://api.weather.com/v3/location/search", utils.RequestGETParams{Query: map[string]string{"apiKey": weatherAPIKey, "query": weatherQuery, "language": strings.Split(lang, "-")[0], "format": "json"}}).Body()
	err := json.Unmarshal(body, &weatherData)
	if err != nil || len(weatherData.Location.Address) == 0 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("weather.location-unknown"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	body = utils.RequestGET("https://api.weather.com/v3/aggcommon/v3-wx-observations-current", utils.RequestGETParams{Query: map[string]string{"apiKey": weatherAPIKey, "geocode": fmt.Sprintf("%.3f,%.3f", weatherData.Location.Latitude[0], weatherData.Location.Longitude[0]), "language": strings.Split(lang, "-")[0], "units": i18n("weather.measurement-unit"), "format": "json"}}).Body()
	var weatherResult weatherResult
	err = json.Unmarshal(body, &weatherResult)
	if err != nil {
		log.Print("[misc/weather] Error unmarshalling weather data:", err)
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.From.ID),
		Text:      fmt.Sprintf(i18n("weather.details"), weatherData.Location.Address[0], weatherResult.V3WxObservationsCurrent.Temperature, weatherResult.V3WxObservationsCurrent.TemperatureFeelsLike, weatherResult.V3WxObservationsCurrent.RelativeHumidity, weatherResult.V3WxObservationsCurrent.WindSpeed),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func LoadMisc(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("misc")
	bh.HandleMessage(weather, telegohandler.CommandEqual("weather"))
	bh.HandleMessage(weather, telegohandler.CommandEqual("clima"))
}
