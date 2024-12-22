package misc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"strconv"
	"strings"

	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/ruizlenato/smudgelord/internal/utils/helpers"

	"github.com/mymmrac/telego"
	"github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

func handleTranslate(bot *telego.Bot, message telego.Message) {
	var text string
	i18n := localization.Get(message)

	if message.ReplyToMessage != nil {
		replyText := ""
		if messageText := message.ReplyToMessage.Text; messageText != "" {
			replyText = messageText
		} else if caption := message.ReplyToMessage.Caption; caption != "" {
			replyText = caption
		}
		text = replyText
		if len(message.Text) > 4 {
			text = message.Text[4:] + " " + replyText
		}
	} else if len(message.Text) > 4 && strings.Fields(message.Text)[0] == "/tr" {
		text = message.Text[4:]
	}

	if messageFields := strings.Fields(message.Text); messageFields[0] == "/translate" && len(message.Text) > 11 {
		text = message.Text[11:]
	}

	if text == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("translator-no-args-provided"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	var sourceLang string
	var targetLang string

	language := getTranslateLang(text, message.Chat)
	if strings.HasPrefix(text, language) {
		text = strings.Replace(text, language, "", 1)
		text = strings.TrimSpace(text)
	}

	if text == "" {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.Chat.ID),
			Text:      i18n("translator-no-args-provided"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	if langParts := strings.Split(language, "-"); len(langParts) > 1 {
		sourceLang = langParts[0]
		targetLang = langParts[1]
	} else {
		targetLang = language
		sourceLang = "auto"
	}

	translation := new(struct {
		Sentences []struct {
			Trans   string `json:"trans"`
			Orig    string `json:"orig"`
			Backend int    `json:"backend"`
		} `json:"sentences"`
		Source string `json:"src"`
	})

	devices := []string{
		"Linux; U; Android 10; Pixel 4",
		"Linux; U; Android 10; Pixel 4 XL",
		"Linux; U; Android 10; Pixel 4a",
		"Linux; U; Android 10; Pixel 4a XL",
		"Linux; U; Android 11; Pixel 4",
		"Linux; U; Android 11; Pixel 4 XL",
		"Linux; U; Android 11; Pixel 4a",
		"Linux; U; Android 11; Pixel 4a XL",
		"Linux; U; Android 11; Pixel 5",
		"Linux; U; Android 11; Pixel 5a",
		"Linux; U; Android 12; Pixel 4",
		"Linux; U; Android 12; Pixel 4 XL",
		"Linux; U; Android 12; Pixel 4a",
		"Linux; U; Android 12; Pixel 4a XL",
		"Linux; U; Android 12; Pixel 5",
		"Linux; U; Android 12; Pixel 5a",
		"Linux; U; Android 12; Pixel 6",
		"Linux; U; Android 12; Pixel 6 Pro",
	}

	request, response, err := utils.Request(fmt.Sprintf("https://translate.google.com/translate_a/single?client=at&dt=t&dj=1&sl=%s&tl=%s&q=%s",
		sourceLang, targetLang, url.QueryEscape(text)), utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			`User-Agent`:   fmt.Sprintf(`GoogleTranslate/6.28.0.05.421483610 (%s)`, devices[rand.Intn(len(devices))]),
			`Content-Type`: `application/x-www-form-urlencoded;charset=utf-8`,
		},
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		slog.Error("Couldn't request translation", "Error", err.Error())
		return
	}

	err = json.Unmarshal(response.Body(), &translation)
	if err != nil {
		slog.Error("Couldn't unmarshal translation data", "Error", err.Error())
	}

	var translations []string
	for _, sentence := range translation.Sentences {
		translations = append(translations, sentence.Trans)
	}
	textUnescaped, _ := (url.QueryUnescape(strings.Join(translations, "")))

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      fmt.Sprintf("<b>%s</b> -> <b>%s</b>\n<code>%s</code>", translation.Source, targetLang, textUnescaped),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func getTranslateLang(text string, chat telego.Chat) string {
	languages := [135]string{
		`af`, `sq`, `am`, `ar`, `hy`,
		`as`, `ay`, `az`, `bm`, `eu`,
		`be`, `bn`, `bho`, `bs`, `bg`,
		`ca`, `ceb`, `zh`, `co`, `hr`,
		`cs`, `da`, `dv`, `doi`, `nl`,
		`en`, `eo`, `et`, `ee`, `fil`,
		`fi`, `fr`, `fy`, `gl`, `ka`,
		`de`, `el`, `gn`, `gu`, `ht`,
		`ha`, `haw`, `he`, `iw`, `hi`,
		`hmn`, `hu`, `is`, `ig`, `ilo`,
		`id`, `ga`, `it`, `ja`, `jv`,
		`jw`, `kn`, `kk`, `km`, `rw`,
		`gom`, `ko`, `kri`, `ku`, `ckb`,
		`ky`, `lo`, `la`, `lv`, `ln`,
		`lt`, `lg`, `lb`, `mk`, `mai`,
		`mg`, `ms`, `ml`, `mt`, `mi`,
		`mr`, `mni`, `lus`, `mn`, `my`,
		`ne`, `no`, `ny`, `or`, `om`,
		`ps`, `fa`, `pl`, `pt`, `pa`,
		`qu`, `ro`, `ru`, `sm`, `sa`,
		`gd`, `nso`, `sr`, `st`, `sn`,
		`sd`, `si`, `sk`, `sl`, `so`,
		`es`, `su`, `sw`, `sv`, `tl`,
		`tg`, `ta`, `tt`, `te`, `th`,
		`ti`, `ts`, `tr`, `tk`, `ak`,
		`uk`, `ur`, `ug`, `uz`, `vi`,
		`cy`, `xh`, `yi`, `yo`, `zu`,
	}
	checkLang := func(item string) bool {
		for _, s := range languages {
			if s == item {
				return true
			}
		}
		return false
	}

	chatLang, err := localization.GetChatLanguage(chat)
	if err != nil {
		chatLang = "en"
	}

	if len(strings.Fields(text)) > 0 {
		lang := strings.Fields(text)[0]
		langParts := strings.Split(lang, "-")

		if !checkLang(langParts[0]) {
			lang = strings.Split(chatLang, "-")[0]
		}

		if len(langParts) > 1 && !checkLang(langParts[1]) {
			lang = strings.Split(chatLang, "-")[0]
		}

		return lang
	}
	return "en"
}

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

func handleWeather(bot *telego.Bot, message telego.Message) {
	var weatherQuery string
	i18n := localization.Get(message)

	if len(strings.Fields(message.Text)) > 1 {
		_, _, args := telegoutil.ParseCommand(message.Text)
		weatherQuery = strings.Join(args, " ")
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("weather-no-location-provided"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	chatLang, err := localization.GetChatLanguage(message.Chat)
	if err != nil {
		return
	}

	var weatherSearchData weatherSearch

	request, response, err := utils.Request("https://api.weather.com/v3/location/search", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"apiKey": weatherAPIKey,
			"query":  weatherQuery,
			"language": strings.Split(chatLang, "-")[0] +
				"-" +
				strings.ToUpper(strings.Split(chatLang, "-")[1]),
			"format": "json",
		},
	})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		slog.Error("Couldn't request weather search", "Error", err.Error())
		return
	}

	err = json.Unmarshal(response.Body(), &weatherSearchData)
	if err != nil {
		return
	}

	buttons := make([][]telego.InlineKeyboardButton, 0, len(database.AvailableLocales))
	for i := 0; i < len(weatherSearchData.Location.Address) && i < 5; i++ {
		buttons = append(buttons, []telego.InlineKeyboardButton{{
			Text: weatherSearchData.Location.Address[i],
			CallbackData: fmt.Sprintf("_weather|%f|%f",
				weatherSearchData.Location.Latitude[i],
				weatherSearchData.Location.Longitude[i],
			),
		}})
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      i18n("weather-select-location"),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
		ReplyMarkup: telegoutil.InlineKeyboard(buttons...),
	})
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

func callbackWeather(bot *telego.Bot, update telego.Update) {
	var weatherResultData weatherResult
	i18n := localization.Get(update)
	message := update.CallbackQuery.Message.(*telego.Message)

	chatLang, err := localization.GetChatLanguage(message.Chat)
	if err != nil {
		return
	}
	callbackData := strings.Split(update.CallbackQuery.Data, "|")

	latitude, err := strconv.ParseFloat(callbackData[1], 64)
	if err != nil {
		return
	}
	longitude, err := strconv.ParseFloat(callbackData[2], 64)
	if err != nil {
		return
	}

	request, response, err := utils.Request("https://api.weather.com/v3/aggcommon/v3-wx-observations-current;v3-location-point",
		utils.RequestParams{
			Method: "GET",
			Query: map[string]string{
				"apiKey":  weatherAPIKey,
				"geocode": fmt.Sprintf("%.3f,%.3f", latitude, longitude),
				"language": strings.Split(chatLang, "-")[0] +
					"-" +
					strings.ToUpper(strings.Split(chatLang, "-")[1]),
				"units":  i18n("measurement-unit"),
				"format": "json",
			},
		})
	defer utils.ReleaseRequestResources(request, response)

	if err != nil {
		slog.Error("Couldn't request weather data", "Error", err.Error())
		return
	}

	err = json.Unmarshal(response.Body(), &weatherResultData)
	if err != nil {
		return
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

	bot.EditMessageText(&telego.EditMessageTextParams{
		ChatID:    telegoutil.ID(update.CallbackQuery.Message.GetChat().ID),
		MessageID: update.CallbackQuery.Message.GetMessageID(),
		Text: i18n("weather-details",
			map[string]interface{}{
				"localname":            localName,
				"temperature":          weatherResultData.V3WxObservationsCurrent.Temperature,
				"temperatureFeelsLike": weatherResultData.V3WxObservationsCurrent.TemperatureFeelsLike,
				"relativeHumidity":     weatherResultData.V3WxObservationsCurrent.RelativeHumidity,
				"windSpeed":            weatherResultData.V3WxObservationsCurrent.WindSpeed,
			}),
		ParseMode: "HTML",
	})
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("misc")
	bh.HandleMessage(handleWeather, telegohandler.Or(telegohandler.CommandEqual("weather"), telegohandler.CommandEqual("clima")))
	bh.Handle(callbackWeather, telegohandler.CallbackDataContains("_weather"))
	bh.HandleMessage(handleTranslate, telegohandler.Or(
		telegohandler.CommandEqual("translate"),
		telegohandler.CommandEqual("tr")),
	)

	helpers.DisableableCommands = append(helpers.DisableableCommands, "tr", "translate", "weather", "clima")
}
