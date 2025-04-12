package misc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/misc/misc"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	weatherAPIKey = "8de2d8b3a93542c9a2d8b3a935a2c909"
)

func handlerTranslate(message *telegram.NewMessage) error {
	var text string
	i18n := localization.Get(message)

	if message.IsReply() {
		reply, err := message.GetReplyMessage()
		if err != nil {
			return err
		}
		text = reply.Text()
		if message.Args() != "" {
			text = message.Args() + " " + text
		}
	} else if message.Args() != "" {
		text = message.Args()
	} else {
		_, err := message.Reply(i18n("translator-no-args-provided"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	translation, err := misc.Translator(text, message)
	if err != nil {
		return err
	}

	translations := make([]string, 0, len(translation.Sentences))
	for _, sentence := range translation.Sentences {
		translations = append(translations, sentence.Trans)
	}
	unescapedText, err := url.QueryUnescape(strings.Join(translations, ""))
	if err != nil {
		_, err := message.Reply(i18n("translator-error"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}
	_, err = message.Reply(fmt.Sprintf("<b>%s</b> -> <b>%s</b>\n<code>%s</code>", translation.Source, translation.Target, unescapedText), telegram.SendOptions{
		ParseMode: telegram.HTML,
	})

	return err
}

type weatherSearch struct {
	Location struct {
		Latitude  []float64 `json:"latitude"`
		Longitude []float64 `json:"longitude"`
		Address   []string  `json:"address"`
	} `json:"location"`
}

func handlerWeather(message *telegram.NewMessage) error {
	i18n := localization.Get(message)
	if message.Args() == "" {
		_, err := message.Reply(i18n("weather-no-location-provided"), telegram.SendOptions{
			ParseMode: telegram.HTML,
		})
		return err
	}

	chatLang, err := localization.GetChatLanguage(message.ChatID(), message.ChatType())
	if err != nil {
		return err
	}

	response, err := utils.Request("https://api.weather.com/v3/location/search", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"apiKey":   weatherAPIKey,
			"query":    message.Args(),
			"language": strings.Split(chatLang, "-")[0],
			"format":   "json",
		},
	})

	if err != nil || response.Body == nil {
		return err
	}
	defer response.Body.Close()

	var weatherSearchData weatherSearch
	err = json.NewDecoder(response.Body).Decode(&weatherSearchData)
	if err != nil {
		return err
	}

	buttons := telegram.ButtonBuilder{}.Keyboard()
	for i := 0; i < len(weatherSearchData.Location.Address) && i < 5; i++ {
		buttons.Rows = append(buttons.Rows, telegram.ButtonBuilder{}.Row(telegram.ButtonBuilder{}.Data(
			weatherSearchData.Location.Address[i],
			fmt.Sprintf("_weather|%f|%f",
				weatherSearchData.Location.Latitude[i],
				weatherSearchData.Location.Longitude[i],
			),
		)))
	}

	_, err = message.Reply(i18n("weather-select-location"), telegram.SendOptions{
		ParseMode:   telegram.HTML,
		ReplyMarkup: buttons,
	})

	return err
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
	V3LocationPoint struct {
		Location struct {
			City   string `json:"city"`
			Locale struct {
				Locale3 any    `json:"locale3"`
				Locale4 string `json:"locale4"`
			} `json:"locale"`
			AdminDistrict  string `json:"adminDistrict"`
			Country        string `json:"country"`
			DisplayContext string `json:"displayContext"`
		} `json:"location"`
	} `json:"v3-location-point"`
}

func callbackWeather(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	chatLang, err := localization.GetChatLanguage(update.ChatID, update.ChatType())
	if err != nil {
		return err
	}
	callbackData := strings.Split(update.DataString(), "|")

	latitude, err := strconv.ParseFloat(callbackData[1], 64)
	if err != nil {
		return err
	}
	longitude, err := strconv.ParseFloat(callbackData[2], 64)
	if err != nil {
		return err
	}

	response, err := utils.Request("https://api.weather.com/v3/aggcommon/v3-wx-observations-current;v3-location-point", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"apiKey":   weatherAPIKey,
			"geocode":  fmt.Sprintf("%.3f,%.3f", latitude, longitude),
			"language": strings.Split(chatLang, "-")[0],
			"units":    i18n("measurement-unit"),
			"format":   "json",
		},
	})

	if err != nil || response.Body == nil {
		return err
	}
	defer response.Body.Close()

	var weatherResultData weatherResult
	err = json.NewDecoder(response.Body).Decode(&weatherResultData)
	if err != nil {
		return err
	}

	var localNameParts []string
	if locale4 := weatherResultData.V3LocationPoint.Location.Locale.Locale4; locale4 != "" {
		localNameParts = append(localNameParts, locale4)
	}

	if locale3, ok := weatherResultData.V3LocationPoint.Location.Locale.Locale3.(string); ok && locale3 != "" {
		localNameParts = append(localNameParts, locale3)
	}

	localNameParts = append(localNameParts,
		weatherResultData.V3LocationPoint.Location.City,
		weatherResultData.V3LocationPoint.Location.AdminDistrict,
		weatherResultData.V3LocationPoint.Location.Country)

	localName := strings.Join(localNameParts, ", ")

	_, err = update.Edit(i18n("weather-details", map[string]any{
		"localname":            localName,
		"temperature":          weatherResultData.V3WxObservationsCurrent.Temperature,
		"temperatureFeelsLike": weatherResultData.V3WxObservationsCurrent.TemperatureFeelsLike,
		"relativeHumidity":     weatherResultData.V3WxObservationsCurrent.RelativeHumidity,
		"windSpeed":            weatherResultData.V3WxObservationsCurrent.WindSpeed,
	}))
	return err
}

func Load(client *telegram.Client) {
	utils.SotreHelp("misc")
	client.On("command:translate", handlers.HandleCommand(handlerTranslate))
	client.On("command:tr", handlers.HandleCommand(handlerTranslate))
	client.On("command:weather", handlers.HandleCommand(handlerWeather))
	client.On("command:clima", handlers.HandleCommand(handlerWeather))
	client.On("callback:weather", callbackWeather)

	handlers.DisableableCommands = append(handlers.DisableableCommands, "translate", "tr", "weather", "clima")
}
