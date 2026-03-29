package misc

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/url"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"

	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const weatherAPIKey = "8de2d8b3a93542c9a2d8b3a935a2c909"

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

func getTranslateLang(text string, ctx *ext.Context) string {
	languages := [135]string{`af`, `sq`, `am`, `ar`, `hy`, `as`, `ay`, `az`, `bm`, `eu`, `be`, `bn`, `bho`, `bs`, `bg`, `ca`, `ceb`, `zh`, `co`, `hr`, `cs`, `da`, `dv`, `doi`, `nl`, `en`, `eo`, `et`, `ee`, `fil`, `fi`, `fr`, `fy`, `gl`, `ka`, `de`, `el`, `gn`, `gu`, `ht`, `ha`, `haw`, `he`, `iw`, `hi`, `hmn`, `hu`, `is`, `ig`, `ilo`, `id`, `ga`, `it`, `ja`, `jv`, `jw`, `kn`, `kk`, `km`, `rw`, `gom`, `ko`, `kri`, `ku`, `ckb`, `ky`, `lo`, `la`, `lv`, `ln`, `lt`, `lg`, `lb`, `mk`, `mai`, `mg`, `ms`, `ml`, `mt`, `mi`, `mr`, `mni`, `lus`, `mn`, `my`, `ne`, `no`, `ny`, `or`, `om`, `ps`, `fa`, `pl`, `pt`, `pa`, `qu`, `ro`, `ru`, `sm`, `sa`, `gd`, `nso`, `sr`, `st`, `sn`, `sd`, `si`, `sk`, `sl`, `so`, `es`, `su`, `sw`, `sv`, `tl`, `tg`, `ta`, `tt`, `te`, `th`, `ti`, `ts`, `tr`, `tk`, `ak`, `uk`, `ur`, `ug`, `uz`, `vi`, `cy`, `xh`, `yi`, `yo`, `zu`}
	checkLang := func(item string) bool {
		for _, s := range languages {
			if s == item {
				return true
			}
		}
		return false
	}

	chatLang, err := localization.GetChatLanguage(ctx.Update)
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

func translateHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	msg := ctx.EffectiveMessage
	i18n := localization.Get(ctx)
	text := ""

	if msg.ReplyToMessage != nil {
		replyText := msg.ReplyToMessage.GetText()
		if replyText == "" {
			replyText = msg.ReplyToMessage.Caption
		}
		text = replyText
		if len(msg.GetText()) > 4 {
			text = msg.GetText()[4:] + " " + replyText
		}
	} else if len(msg.GetText()) > 4 && strings.Fields(msg.GetText())[0] == "/tr" {
		text = msg.GetText()[4:]
	}

	if messageFields := strings.Fields(msg.GetText()); len(messageFields) > 0 && messageFields[0] == "/translate" && len(msg.GetText()) > 11 {
		text = msg.GetText()[11:]
	}

	if text == "" {
		_, _ = b.SendMessage(msg.Chat.Id, i18n("translator-no-args-provided"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId}})
		return nil
	}

	language := getTranslateLang(text, ctx)
	if strings.HasPrefix(text, language) {
		text = strings.TrimSpace(strings.Replace(text, language, "", 1))
	}
	if text == "" {
		_, _ = b.SendMessage(msg.Chat.Id, i18n("translator-no-args-provided"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId}})
		return nil
	}

	sourceLang := "auto"
	targetLang := language
	if langParts := strings.Split(language, "-"); len(langParts) > 1 {
		sourceLang = langParts[0]
		targetLang = langParts[1]
	}

	translation := new(struct {
		Sentences []struct {
			Trans string `json:"trans"`
		} `json:"sentences"`
		Source string `json:"src"`
	})

	devices := []string{"Linux; U; Android 10; Pixel 4", "Linux; U; Android 11; Pixel 5", "Linux; U; Android 12; Pixel 6"}
	response, err := utils.Request(fmt.Sprintf("https://translate.google.com/translate_a/single?client=at&dt=t&dj=1&sl=%s&tl=%s&q=%s", sourceLang, targetLang, url.QueryEscape(text)), utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			"User-Agent":   fmt.Sprintf("GoogleTranslate/6.28.0.05.421483610 (%s)", devices[rand.Intn(len(devices))]),
			"Content-Type": "application/x-www-form-urlencoded;charset=utf-8",
		},
	})
	if err != nil {
		slog.Error("Couldn't request translation", "Error", err.Error())
		return nil
	}
	defer response.Body.Close()

	if err := json.NewDecoder(response.Body).Decode(&translation); err != nil {
		slog.Error("Couldn't unmarshal translation data", "Error", err.Error())
		return nil
	}

	translations := make([]string, 0, len(translation.Sentences))
	for _, sentence := range translation.Sentences {
		translations = append(translations, sentence.Trans)
	}
	textUnescaped, _ := url.QueryUnescape(strings.Join(translations, ""))

	_, _ = b.SendMessage(msg.Chat.Id, fmt.Sprintf("<b>%s</b> -> <b>%s</b>\n<code>%s</code>", translation.Source, targetLang, textUnescaped), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId},
	})

	return nil
}

func weatherHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil {
		return nil
	}

	msg := ctx.EffectiveMessage
	i18n := localization.Get(ctx)
	fields := strings.Fields(msg.GetText())
	if len(fields) <= 1 {
		_, _ = b.SendMessage(msg.Chat.Id, i18n("weather-no-location-provided"), &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML, ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId}})
		return nil
	}

	chatLang, err := localization.GetChatLanguage(ctx.Update)
	if err != nil {
		return nil
	}

	weatherQuery := fields[1]
	weatherSearchData, err := searchWeather(weatherQuery, strings.Split(chatLang, "-")[0])
	if err != nil {
		return nil
	}

	buttons := make([][]gotgbot.InlineKeyboardButton, 0, len(database.AvailableLocales))
	for i := 0; i < len(weatherSearchData.Location.Address) && i < 5; i++ {
		buttons = append(buttons, []gotgbot.InlineKeyboardButton{{
			Text:         weatherSearchData.Location.Address[i],
			CallbackData: fmt.Sprintf("_weather|%f|%f", weatherSearchData.Location.Latitude[i], weatherSearchData.Location.Longitude[i]),
		}})
	}

	_, _ = b.SendMessage(msg.Chat.Id, i18n("weather-select-location"), &gotgbot.SendMessageOpts{
		ParseMode:       gotgbot.ParseModeHTML,
		ReplyParameters: &gotgbot.ReplyParameters{MessageId: msg.MessageId},
		ReplyMarkup:     gotgbot.InlineKeyboardMarkup{InlineKeyboard: buttons},
	})

	return nil
}

func callbackWeather(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.CallbackQuery == nil || ctx.CallbackQuery.Message == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	chatLang, err := localization.GetChatLanguage(ctx.Update)
	if err != nil {
		return nil
	}

	callbackData := strings.Split(ctx.CallbackQuery.Data, "|")
	if len(callbackData) < 3 {
		return nil
	}

	latitude, err := strconv.ParseFloat(callbackData[1], 64)
	if err != nil {
		return nil
	}
	longitude, err := strconv.ParseFloat(callbackData[2], 64)
	if err != nil {
		return nil
	}

	weatherResultData, err := weatherSearchResult(fmt.Sprintf("%.3f,%.3f", latitude, longitude), strings.Split(chatLang, "-")[0], i18n)
	if err != nil {
		return nil
	}

	var localNameParts []string
	if locale4 := weatherResultData.V3LocationPoint.Location.Locale.Locale4; locale4 != "" {
		localNameParts = append(localNameParts, locale4)
	}
	if locale3, ok := weatherResultData.V3LocationPoint.Location.Locale.Locale3.(string); ok && locale3 != "" {
		localNameParts = append(localNameParts, locale3)
	}
	localNameParts = append(localNameParts, weatherResultData.V3LocationPoint.Location.City, weatherResultData.V3LocationPoint.Location.AdminDistrict, weatherResultData.V3LocationPoint.Location.Country)

	chat := ctx.CallbackQuery.Message.GetChat()
	msgID := ctx.CallbackQuery.Message.GetMessageId()
	_, _, _ = b.EditMessageText(i18n("weather-details", map[string]any{
		"localname":            strings.Join(localNameParts, ", "),
		"temperature":          weatherResultData.V3WxObservationsCurrent.Temperature,
		"temperatureFeelsLike": weatherResultData.V3WxObservationsCurrent.TemperatureFeelsLike,
		"relativeHumidity":     weatherResultData.V3WxObservationsCurrent.RelativeHumidity,
		"windSpeed":            weatherResultData.V3WxObservationsCurrent.WindSpeed,
	}), &gotgbot.EditMessageTextOpts{ChatId: chat.Id, MessageId: msgID, ParseMode: gotgbot.ParseModeHTML})

	return nil
}

func weatherInlineQuery(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.InlineQuery == nil {
		return nil
	}

	i18n := localization.Get(ctx)
	chatLang, err := localization.GetChatLanguage(ctx.Update)
	if err != nil {
		return nil
	}

	weatherSearchData, err := searchWeather(ctx.InlineQuery.Query, strings.Split(chatLang, "-")[0])
	if err != nil {
		return nil
	}

	results := make([]gotgbot.InlineQueryResult, 0, 5)
	for i := 0; i < len(weatherSearchData.Location.Address) && i < 5; i++ {
		res := gotgbot.InlineQueryResultArticle{
			Id:    fmt.Sprintf("weather-%f,%f", weatherSearchData.Location.Latitude[i], weatherSearchData.Location.Longitude[i]),
			Title: weatherSearchData.Location.Address[i],
			InputMessageContent: gotgbot.InputTextMessageContent{
				MessageText: i18n("loading"),
				ParseMode:   gotgbot.ParseModeHTML,
			},
			ReplyMarkup: &gotgbot.InlineKeyboardMarkup{InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{{Text: "⏳", CallbackData: "NONE"}}}},
		}
		results = append(results, res)
	}

	if len(results) > 0 {
		cacheTime := int64(0)
		_, _ = b.AnswerInlineQuery(ctx.InlineQuery.Id, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
	}

	return nil
}

func WeatherInline(b *gotgbot.Bot, ctx *ext.Context, geocode string) error {
	i18n := localization.Get(ctx)
	chatLang, err := localization.GetChatLanguage(ctx.Update)
	if err != nil {
		slog.Error("Couldn't get chat language", "error", err.Error())
		return nil
	}

	weatherResultData, err := weatherSearchResult(geocode, strings.Split(chatLang, "-")[0], i18n)
	if err != nil {
		slog.Error("Couldn't get weather data", "error", err.Error())
		return nil
	}

	var localNameParts []string
	if locale4 := weatherResultData.V3LocationPoint.Location.Locale.Locale4; locale4 != "" {
		localNameParts = append(localNameParts, locale4)
	}
	if locale3, ok := weatherResultData.V3LocationPoint.Location.Locale.Locale3.(string); ok && locale3 != "" {
		localNameParts = append(localNameParts, locale3)
	}
	localNameParts = append(localNameParts, weatherResultData.V3LocationPoint.Location.City, weatherResultData.V3LocationPoint.Location.AdminDistrict, weatherResultData.V3LocationPoint.Location.Country)

	if ctx.ChosenInlineResult != nil {
		_, _, err = b.EditMessageText(i18n("weather-details", map[string]any{
			"localname":            strings.Join(localNameParts, ", "),
			"temperature":          weatherResultData.V3WxObservationsCurrent.Temperature,
			"temperatureFeelsLike": weatherResultData.V3WxObservationsCurrent.TemperatureFeelsLike,
			"relativeHumidity":     weatherResultData.V3WxObservationsCurrent.RelativeHumidity,
			"windSpeed":            weatherResultData.V3WxObservationsCurrent.WindSpeed,
		}), &gotgbot.EditMessageTextOpts{InlineMessageId: ctx.ChosenInlineResult.InlineMessageId, ParseMode: gotgbot.ParseModeHTML})
		if err != nil {
			slog.Error("Couldn't edit message", "Error", err.Error())
		}
	}

	return nil
}

func slapHandler(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage == nil || ctx.EffectiveUser == nil {
		return nil
	}
	if ctx.EffectiveChat == nil || (ctx.EffectiveChat.Type != gotgbot.ChatTypeGroup && ctx.EffectiveChat.Type != gotgbot.ChatTypeSupergroup) {
		return nil
	}

	i18n := localization.Get(ctx)
	if ctx.EffectiveMessage.ReplyToMessage == nil || ctx.EffectiveMessage.ReplyToMessage.From == nil {
		return nil
	}

	targetUser := ctx.EffectiveMessage.ReplyToMessage.From
	if targetUser.Id == ctx.EffectiveUser.Id {
		return nil
	}

	userName := utils.EscapeHTML(ctx.EffectiveUser.FirstName)
	targetName := utils.EscapeHTML(targetUser.FirstName)

	actionTypes := []struct {
		key        string
		variations []string
	}{
		{key: "slap-hit", variations: []string{"vodka", "bat", "shovel", "fish", "fryingpan", "penis", "baguette", "hammer"}},
		{key: "slap-throw", variations: []string{"cliff", "window", "mud", "pie", "water"}},
		{key: "slap-push", variations: []string{"lava", "stairs", "street"}},
	}

	selectedAction := actionTypes[rand.Intn(len(actionTypes))]
	selectedVariation := selectedAction.variations[rand.Intn(len(selectedAction.variations))]
	paramName := "item"
	if selectedAction.key == "slap-push" {
		paramName = "location"
	}

	message := i18n(selectedAction.key, map[string]any{"userName": userName, "targetName": targetName, paramName: selectedVariation})
	_, _ = b.SendMessage(ctx.EffectiveChat.Id, message, &gotgbot.SendMessageOpts{ParseMode: gotgbot.ParseModeHTML})
	return nil
}

func Load(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("weather", weatherHandler))
	dispatcher.AddHandler(handlers.NewCommand("clima", weatherHandler))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("_weather"), callbackWeather))
	dispatcher.AddHandler(handlers.NewCommand("translate", translateHandler))
	dispatcher.AddHandler(handlers.NewCommand("tr", translateHandler))
	dispatcher.AddHandler(handlers.NewInlineQuery(func(iq *gotgbot.InlineQuery) bool {
		return strings.HasPrefix(iq.Query, "weather") || strings.HasPrefix(iq.Query, "clima")
	}, weatherInlineQuery))
	dispatcher.AddHandler(handlers.NewCommand("slap", slapHandler))

	utils.SaveHelp("misc")
	utils.DisableableCommands = append(utils.DisableableCommands, "tr", "translate", "weather", "clima", "slap")
}
