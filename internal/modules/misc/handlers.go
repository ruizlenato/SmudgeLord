package misc

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/url"
	"strings"

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
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("misc.translatorNoArgs"),
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
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("misc.translatorNoArgs"),
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

	body := utils.Request(fmt.Sprintf("https://translate.google.com/translate_a/single?client=at&dt=t&dj=1&sl=%s&tl=%s&q=%s",
		sourceLang, targetLang, url.QueryEscape(text)), utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			`User-Agent`:   fmt.Sprintf(`GoogleTranslate/6.28.0.05.421483610 (%s)`, devices[rand.Intn(len(devices))]),
			`Content-Type`: `application/x-www-form-urlencoded;charset=utf-8`,
		},
	}).Body()

	err := json.Unmarshal(body, &translation)
	if err != nil {
		log.Println("[Misc/gTranslate] Error unmarshalling translation data:", err)
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

func handleWeather(bot *telego.Bot, message telego.Message) {
	var weatherQuery string

	lang, _ := localization.GetChatLanguage(message.Chat)
	i18n := localization.Get(message)

	if len(strings.Fields(message.Text)) > 1 {
		_, _, args := telegoutil.ParseCommand(message.Text)
		weatherQuery = strings.Join(args, " ")
	} else {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("weather.noLocation"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	var weatherData weatherSearch

	body := utils.Request("https://api.weather.com/v3/location/search", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"apiKey":   weatherAPIKey,
			"query":    weatherQuery,
			"language": strings.Split(lang, "-")[0],
			"format":   "json",
		},
	}).Body()

	err := json.Unmarshal(body, &weatherData)
	if err != nil || len(weatherData.Location.Address) == 0 {
		bot.SendMessage(&telego.SendMessageParams{
			ChatID:    telegoutil.ID(message.From.ID),
			Text:      i18n("weather.locationUnknown"),
			ParseMode: "HTML",
			ReplyParameters: &telego.ReplyParameters{
				MessageID: message.MessageID,
			},
		})
		return
	}

	var weatherResult weatherResult
	body = utils.Request("https://api.weather.com/v3/aggcommon/v3-wx-observations-current",
		utils.RequestParams{
			Method: "GET",
			Query: map[string]string{
				"apiKey":   weatherAPIKey,
				"geocode":  fmt.Sprintf("%.3f,%.3f", weatherData.Location.Latitude[0], weatherData.Location.Longitude[0]),
				"language": strings.Split(lang, "-")[0],
				"units":    i18n("weather.measurementUnit"),
				"format":   "json",
			},
		}).Body()

	err = json.Unmarshal(body, &weatherResult)
	if err != nil {
		log.Print("[misc/weather] Error unmarshalling weather data:", err)
		return
	}

	bot.SendMessage(&telego.SendMessageParams{
		ChatID:    telegoutil.ID(message.Chat.ID),
		Text:      fmt.Sprintf(i18n("weather.details"), weatherData.Location.Address[0], weatherResult.V3WxObservationsCurrent.Temperature, weatherResult.V3WxObservationsCurrent.TemperatureFeelsLike, weatherResult.V3WxObservationsCurrent.RelativeHumidity, weatherResult.V3WxObservationsCurrent.WindSpeed),
		ParseMode: "HTML",
		ReplyParameters: &telego.ReplyParameters{
			MessageID: message.MessageID,
		},
	})
}

func Load(bh *telegohandler.BotHandler, bot *telego.Bot) {
	helpers.Store("misc")
	bh.HandleMessage(handleWeather, telegohandler.Or(telegohandler.CommandEqual("weather"), telegohandler.CommandEqual("clima")))
	bh.HandleMessage(handleTranslate, telegohandler.Or(
		telegohandler.CommandEqual("translate"),
		telegohandler.CommandEqual("tr")),
	)

	helpers.DisableableCommands = append(helpers.DisableableCommands, "tr", "translate", "weather", "clima")
}
