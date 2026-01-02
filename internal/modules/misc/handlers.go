package misc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

func translateHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	var text string
	i18n := localization.Get(update)

	if update.Message.ReplyToMessage != nil {
		replyText := ""
		if messageText := update.Message.ReplyToMessage.Text; messageText != "" {
			replyText = messageText
		} else if caption := update.Message.ReplyToMessage.Caption; caption != "" {
			replyText = caption
		}
		text = replyText
		if len(update.Message.Text) > 4 {
			text = update.Message.Text[4:] + " " + replyText
		}
	} else if len(update.Message.Text) > 4 && strings.Fields(update.Message.Text)[0] == "/tr" {
		text = update.Message.Text[4:]
	}

	if messageFields := strings.Fields(update.Message.Text); messageFields[0] == "/translate" && len(update.Message.Text) > 11 {
		text = update.Message.Text[11:]
	}

	if text == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("translator-no-args-provided"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	var sourceLang string
	var targetLang string

	language := getTranslateLang(text, update)
	if strings.HasPrefix(text, language) {
		text = strings.Replace(text, language, "", 1)
		text = strings.TrimSpace(text)
	}

	if text == "" {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("translator-no-args-provided"),
			ParseMode: models.ParseModeHTML,
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
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

	response, err := utils.Request(fmt.Sprintf("https://translate.google.com/translate_a/single?client=at&dt=t&dj=1&sl=%s&tl=%s&q=%s",
		sourceLang, targetLang, url.QueryEscape(text)), utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			`User-Agent`:   fmt.Sprintf(`GoogleTranslate/6.28.0.05.421483610 (%s)`, devices[rand.Intn(len(devices))]),
			`Content-Type`: `application/x-www-form-urlencoded;charset=utf-8`,
		},
	})

	if err != nil {
		slog.Error("Couldn't request translation",
			"Error", err.Error())
		return
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&translation)
	if err != nil {
		slog.Error("Couldn't unmarshal translation data",
			"Error", err.Error())
	}

	var translations []string
	for _, sentence := range translation.Sentences {
		translations = append(translations, sentence.Trans)
	}
	textUnescaped, _ := (url.QueryUnescape(strings.Join(translations, "")))

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      fmt.Sprintf("<b>%s</b> -> <b>%s</b>\n<code>%s</code>", translation.Source, targetLang, textUnescaped),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
	})
}

func getTranslateLang(text string, update *models.Update) string {
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

	chatLang, err := localization.GetChatLanguage(update)
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

func searchWeather(local, language string) (weatherSearch, error) {
	var weatherSearchData weatherSearch
	response, err := utils.Request("https://api.weather.com/v3/location/search", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"apiKey":   weatherAPIKey,
			"query":    local,
			"language": language,
			"format":   "json",
		},
	})
	if err != nil {
		return weatherSearchData, err
	}
	if response.Body == nil {
		return weatherSearchData, fmt.Errorf("response body is nil")
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&weatherSearchData)
	if err != nil {
		return weatherSearchData, err
	}

	return weatherSearchData, nil
}

func weatherHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	var weatherQuery string
	i18n := localization.Get(update)

	if len(strings.Fields(update.Message.Text)) > 1 {
		weatherQuery = strings.Fields(update.Message.Text)[1]
	} else {
		b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:    update.Message.Chat.ID,
			Text:      i18n("weather-no-location-provided"),
			ParseMode: "HTML",
			ReplyParameters: &models.ReplyParameters{
				MessageID: update.Message.ID,
			},
		})
		return
	}

	chatLang, err := localization.GetChatLanguage(update)
	if err != nil {
		return
	}

	weatherSearchData, err := searchWeather(weatherQuery, strings.Split(chatLang, "-")[0])
	if err != nil {
		return
	}

	buttons := make([][]models.InlineKeyboardButton, 0, len(database.AvailableLocales))
	for i := 0; i < len(weatherSearchData.Location.Address) && i < 5; i++ {
		buttons = append(buttons, []models.InlineKeyboardButton{{
			Text: weatherSearchData.Location.Address[i],
			CallbackData: fmt.Sprintf("_weather|%f|%f",
				weatherSearchData.Location.Latitude[i],
				weatherSearchData.Location.Longitude[i],
			),
		}})
	}

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      i18n("weather-select-location"),
		ParseMode: models.ParseModeHTML,
		ReplyParameters: &models.ReplyParameters{
			MessageID: update.Message.ID,
		},
		ReplyMarkup: &models.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
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

func weatherSearchResult(geocode, language string, i18n func(string, ...map[string]any) string) (weatherResult, error) {
	var weatherResultData weatherResult
	response, err := utils.Request("https://api.weather.com/v3/aggcommon/v3-wx-observations-current;v3-location-point", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"apiKey":   weatherAPIKey,
			"geocode":  geocode,
			"language": language,
			"units":    i18n("measurement-unit"),
			"format":   "json",
		},
	})

	if err != nil {
		return weatherResultData, err
	}
	if response.Body == nil {
		return weatherResultData, fmt.Errorf("response body is nil")
	}
	defer response.Body.Close()

	err = json.NewDecoder(response.Body).Decode(&weatherResultData)
	if err != nil {
		return weatherResultData, err
	}

	return weatherResultData, nil
}

func callbackWeather(ctx context.Context, b *bot.Bot, update *models.Update) {
	i18n := localization.Get(update)

	chatLang, err := localization.GetChatLanguage(update)
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

	weatherResultData, err := weatherSearchResult(fmt.Sprintf("%.3f,%.3f", latitude, longitude), strings.Split(chatLang, "-")[0], i18n)
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

	b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    update.CallbackQuery.Message.Message.Chat.ID,
		MessageID: update.CallbackQuery.Message.Message.ID,
		Text: i18n("weather-details",
			map[string]any{
				"localname":            localName,
				"temperature":          weatherResultData.V3WxObservationsCurrent.Temperature,
				"temperatureFeelsLike": weatherResultData.V3WxObservationsCurrent.TemperatureFeelsLike,
				"relativeHumidity":     weatherResultData.V3WxObservationsCurrent.RelativeHumidity,
				"windSpeed":            weatherResultData.V3WxObservationsCurrent.WindSpeed,
			}),
		ParseMode: models.ParseModeHTML,
	})
}

func weatherInlineQuery(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.InlineQuery == nil {
		return
	}

	i18n := localization.Get(update)

	chatLang, err := localization.GetChatLanguage(update)
	if err != nil {
		return
	}
	weatherSearchData, err := searchWeather(update.InlineQuery.Query, strings.Split(chatLang, "-")[0])
	if err != nil {
		return
	}

	results := make([]models.InlineQueryResult, 0, 5)
	for i := 0; i < len(weatherSearchData.Location.Address) && i < 5; i++ {
		results = append(results, &models.InlineQueryResultArticle{
			ID:    fmt.Sprintf("weather-%f,%f", weatherSearchData.Location.Latitude[i], weatherSearchData.Location.Longitude[i]),
			Title: weatherSearchData.Location.Address[i],
			InputMessageContent: &models.InputTextMessageContent{
				MessageText: i18n("loading"),
				ParseMode:   models.ParseModeHTML,
			},
			ReplyMarkup: &models.InlineKeyboardMarkup{
				InlineKeyboard: [][]models.InlineKeyboardButton{{
					{
						Text:         "⏳",
						CallbackData: "NONE",
					},
				}},
			},
		})
	}

	if len(results) > 0 {
		b.AnswerInlineQuery(ctx, &bot.AnswerInlineQueryParams{
			InlineQueryID: update.InlineQuery.ID,
			Results:       results,
			CacheTime:     0,
		})
	}
}

func WeatherInline(ctx context.Context, b *bot.Bot, update *models.Update, geocode string) {
	i18n := localization.Get(update)

	chatLang, err := localization.GetChatLanguage(update)
	if err != nil {
		slog.Error("Couldn't get chat language",
			"error", err.Error())
		return
	}
	weatherResultData, err := weatherSearchResult(geocode, strings.Split(chatLang, "-")[0], i18n)
	if err != nil {
		slog.Error("Couldn't get weather data",
			"error", err.Error())
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

	if _, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		InlineMessageID: update.ChosenInlineResult.InlineMessageID,
		Text: i18n("weather-details", map[string]any{
			"localname":            localName,
			"temperature":          weatherResultData.V3WxObservationsCurrent.Temperature,
			"temperatureFeelsLike": weatherResultData.V3WxObservationsCurrent.TemperatureFeelsLike,
			"relativeHumidity":     weatherResultData.V3WxObservationsCurrent.RelativeHumidity,
			"windSpeed":            weatherResultData.V3WxObservationsCurrent.WindSpeed,
		}),
		ParseMode: models.ParseModeHTML,
	}); err != nil {
		slog.Error("Couldn't edit message",
			"Error", err.Error())
	}
}

func slapHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil {
		return
	}

	user := update.Message.From
	if user == nil {
		return
	}

	i18n := localization.Get(update)

	var targetUser *models.User

	if update.Message.ReplyToMessage != nil && update.Message.ReplyToMessage.From != nil {
		targetUser = update.Message.ReplyToMessage.From
	} else {
		return
	}

	if targetUser.ID == user.ID {
		return
	}

	userName := utils.EscapeHTML(user.FirstName)
	targetName := utils.EscapeHTML(targetUser.FirstName)

	actionTypes := []struct {
		key        string
		variations []string
	}{
		{
			key:        "slap-hit",
			variations: []string{"vodka", "bat", "shovel", "fish", "fryingpan", "penis", "baguette", "hammer"},
		},
		{
			key:        "slap-throw",
			variations: []string{"cliff", "window", "mud", "pie", "water"},
		},
		{
			key:        "slap-push",
			variations: []string{"lava", "stairs", "street"},
		},
	}

	selectedAction := actionTypes[rand.Intn(len(actionTypes))]
	selectedVariation := selectedAction.variations[rand.Intn(len(selectedAction.variations))]

	paramName := "item"
	switch selectedAction.key {
	case "slap-push":
		paramName = "location"
	case "slap-tie":
		paramName = "action"
	case "slap-challenge":
		paramName = "challenge"
	}

	message := i18n(selectedAction.key, map[string]any{
		"userName":   userName,
		"targetName": targetName,
		paramName:    selectedVariation,
	})

	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:    update.Message.Chat.ID,
		Text:      message,
		ParseMode: models.ParseModeHTML,
	})
}
func Load(b *bot.Bot) {
	b.RegisterHandler(bot.HandlerTypeCommand, "weather", weatherHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "clima", weatherHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, "^_weather", callbackWeather)
	b.RegisterHandler(bot.HandlerTypeCommand, "translate", translateHandler)
	b.RegisterHandler(bot.HandlerTypeCommand, "tr", translateHandler)
	b.RegisterHandler(bot.HandlerTypeInlineQuery, "^(weather|clima).+", weatherInlineQuery)
	b.RegisterHandler(bot.HandlerTypeCommand, "slap", slapHandler, utils.IsGroup)

	utils.SaveHelp("misc")
	utils.DisableableCommands = append(utils.DisableableCommands,
		"tr", "translate", "weather", "clima", "slap")
}
