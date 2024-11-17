package misc

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"strings"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

type Translation struct {
	Sentences []struct {
		Trans string `json:"trans"`
	} `json:"sentences"`
	Source string `json:"src"`
	Target string `json:"ld_target"`
}

func Translator(text string, message *telegram.NewMessage) (*Translation, error) {
	var translation Translation

	sourceLang, targetLang, text := parseAndGetTranslateLang(text, message)
	userAgent := fmt.Sprintf(
		`GoogleTranslate/6.28.0.05.421483610 (%s)`,
		getRandomDevice(),
	)

	url := fmt.Sprintf(
		"https://translate.google.com/translate_a/single?client=at&dt=t&dj=1&sl=%s&tl=%s&q=%s",
		sourceLang, targetLang,
		url.QueryEscape(text),
	)

	response, err := utils.Request(url, utils.RequestParams{
		Method: "POST",
		Headers: map[string]string{
			`User-Agent`:   userAgent,
			`Content-Type`: `application/x-www-form-urlencoded;charset=utf-8`,
		},
	})
	defer response.Body.Close()

	if err != nil || response.Body == nil {
		return &translation, err
	}

	err = json.NewDecoder(response.Body).Decode(&translation)
	if err != nil {
		return &translation, err
	}

	translation.Target = targetLang
	return &translation, err
}

func parseAndGetTranslateLang(text string, message *telegram.NewMessage) (sourceLang, targetLang, parsedText string) {
	languages := []string{
		`af`, `sq`, `am`, `ar`, `hy`, `as`, `ay`, `az`, `bm`, `eu`,
		`be`, `bn`, `bho`, `bs`, `bg`, `ca`, `ceb`, `zh`, `co`, `hr`,
		`cs`, `da`, `dv`, `doi`, `nl`, `en`, `eo`, `et`, `ee`, `fil`,
		`fi`, `fr`, `fy`, `gl`, `ka`, `de`, `el`, `gn`, `gu`, `ht`,
		`ha`, `haw`, `he`, `iw`, `hi`, `hmn`, `hu`, `is`, `ig`, `ilo`,
		`id`, `ga`, `it`, `ja`, `jv`, `jw`, `kn`, `kk`, `km`, `rw`,
		`gom`, `ko`, `kri`, `ku`, `ckb`, `ky`, `lo`, `la`, `lv`, `ln`,
		`lt`, `lg`, `lb`, `mk`, `mai`, `mg`, `ms`, `ml`, `mt`, `mi`,
		`mr`, `mni`, `lus`, `mn`, `my`, `ne`, `no`, `ny`, `or`, `om`,
		`ps`, `fa`, `pl`, `pt`, `pa`, `qu`, `ro`, `ru`, `sm`, `sa`,
		`gd`, `nso`, `sr`, `st`, `sn`, `sd`, `si`, `sk`, `sl`, `so`,
		`es`, `su`, `sw`, `sv`, `tl`, `tg`, `ta`, `tt`, `te`, `th`,
		`ti`, `ts`, `tr`, `tk`, `ak`, `uk`, `ur`, `ug`, `uz`, `vi`,
		`cy`, `xh`, `yi`, `yo`, `zu`,
	}

	contains := func(item string) bool {
		for _, s := range languages {
			if s == item {
				return true
			}
		}
		return false
	}

	chatLang, err := localization.GetChatLanguage(message.ChatID(), message.ChatType())
	if err != nil {
		chatLang = "en"
	}

	language := strings.Fields(text)[0]
	languageSplit := strings.Split(language, "-")

	if !contains(languageSplit[0]) {
		language = strings.Split(chatLang, "-")[0]
	}

	if len(languageSplit) > 1 && !contains(languageSplit[1]) {
		language = strings.Split(chatLang, "-")[0]
	}

	if strings.HasPrefix(text, language) {
		text = strings.Replace(text, language, "", 1)
		text = strings.TrimSpace(text)
	}

	if languageSplit := strings.Split(language, "-"); len(languageSplit) > 1 {
		return languageSplit[0], languageSplit[1], text
	} else {
		return "auto", language, text
	}
}

func getRandomDevice() string {
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
	return devices[rand.Intn(len(devices))]
}
