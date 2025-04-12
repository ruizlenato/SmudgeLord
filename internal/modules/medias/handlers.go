package medias

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/gabriel-vasile/mimetype"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/bluesky"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/threads"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/tiktok"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/twitter"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/youtube"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
	"github.com/steino/youtubedl"
)

const (
	regexMedia     = `(?:http(?:s)?://)?(?:m|vm|vt|www|mobile)?(?:.)?(?:(?:bsky|threads|instagram|tiktok|twitter|x)\.(?:com|net|app)|youtube\.com/shorts)/(?:\S*)`
	maxSizeCaption = 1024
)

func handlerMedias(message *telegram.NewMessage) error {
	var mediaItems []telegram.InputMedia
	var result []string
	var caption string

	if !regexp.MustCompile(`^/dl`).MatchString(message.Text()) && message.ChatType() != telegram.EntityUser {
		var mediasAuto bool
		if err := database.DB.QueryRow("SELECT mediasAuto FROM chats WHERE id = ?;", message.ChatID()).Scan(&mediasAuto); err != nil || !mediasAuto {
			return nil
		}
	}
	i18n := localization.Get(message)

	url := regexp.MustCompile(regexMedia).FindStringSubmatch(message.Text())
	if len(url) < 1 {
		_, err := message.Reply(i18n("no-link-provided"))
		return err
	}

	mediaHandlers := map[string]func(*telegram.NewMessage) ([]telegram.InputMedia, []string){
		"bsky.app/":        bluesky.Handle,
		"threads.net/":     threads.Handle,
		"instagram.com/":   instagram.Handle,
		"tiktok.com/":      tiktok.Handle,
		"(twitter|x).com/": twitter.Handle,
		"youtube.com/":     youtube.Handle,
	}

	for pattern, handler := range mediaHandlers {
		if match, _ := regexp.MatchString(pattern, message.Text()); match {
			mediaItems, result = handler(message)
			if len(result) == 2 {
				caption = result[0]
			}
			break
		}
	}

	if len(mediaItems) == 0 {
		return nil
	}

	if _, InputMediaUploadedPhoto := mediaItems[0].(*telegram.InputMediaUploadedPhoto); len(mediaItems) == 1 &&
		InputMediaUploadedPhoto &&
		message.Media() != nil &&
		message.Media().(*telegram.MessageMediaWebPage) != nil {
		return nil
	}

	if utf8.RuneCountInString(caption) > maxSizeCaption {
		caption = downloader.TruncateUTF8Caption(
			caption,
			regexp.MustCompile(regexMedia).FindStringSubmatch(message.Text())[0],
		)
	}

	_, err := message.SendAction("upload_document")
	if err != nil {
		return err
	}
	replied, err := message.ReplyAlbum(mediaItems, &telegram.MediaOptions{Caption: caption})
	if err != nil {
		return err
	}
	err = downloader.SetMediaCache(replied, result)
	return err
}

func handleYoutubeDownload(message *telegram.NewMessage) error {
	var videoURL string
	i18n := localization.Get(message)

	if message.IsReply() {
		reply, err := message.GetReplyMessage()
		if err != nil {
			return err
		}
		videoURL = reply.Text()
	} else if len(strings.Fields(message.Text())) > 1 {
		videoURL = strings.Fields(message.Text())[1]
	} else {
		_, err := message.Reply(i18n("youtube-no-url"))
		return err
	}

	ytClient := youtube.ConfigureYoutubeClient()
	video, err := ytClient.GetVideo(videoURL)
	if err != nil {
		_, err := message.Reply(i18n("youtube-invalid-url"))
		return err
	}

	desiredQualityLabels := func(qualityLabel string) bool {
		supportedQualities := []string{"1080p", "720p", "480p", "360p", "240p", "144p"}
		for _, supported := range supportedQualities {
			if strings.Contains(qualityLabel, supported) {
				return true
			}
		}
		return false
	}

	var maxBitrate int
	var maxBitrateIndex int
	for i, format := range video.Formats.Type("video/mp4") {
		if format.Bitrate > maxBitrate && desiredQualityLabels(format.QualityLabel) {
			maxBitrate = format.Bitrate
			maxBitrateIndex = i
		}
	}
	videoStream := video.Formats.Type("video/mp4")[maxBitrateIndex]

	var audioStream youtubedl.Format
	if len(video.Formats.Itag(140)) > 0 {
		audioStream = video.Formats.Itag(140)[0]
	} else {
		audioStream = video.Formats.WithAudioChannels().Type("audio/mp4")[1]
	}

	text := i18n("youtube-video-info",
		map[string]any{
			"title":     video.Title,
			"author":    video.Author,
			"audioSize": fmt.Sprintf("%.2f", float64(audioStream.ContentLength)/(1024*1024)),
			"videoSize": fmt.Sprintf("%.2f", float64(videoStream.ContentLength+audioStream.ContentLength)/(1024*1024)),
			"duration":  video.Duration.String(),
		})

	keyboard := telegram.ButtonBuilder{}.Keyboard(
		telegram.ButtonBuilder{}.Row(
			telegram.ButtonBuilder{}.Data(
				i18n("youtube-download-audio-button"),
				fmt.Sprintf("_aud|%s|%d|%d|%d", video.ID, audioStream.ItagNo, audioStream.ContentLength, message.SenderID()),
			),
			telegram.ButtonBuilder{}.Data(
				i18n("youtube-download-video-button"),
				fmt.Sprintf("_vid|%s|%d|%d|%d", video.ID, videoStream.ItagNo, videoStream.ContentLength+audioStream.ContentLength, message.SenderID()),
			),
		),
	)

	_, err = message.Reply(text, telegram.SendOptions{
		ReplyMarkup: keyboard,
	})

	return err
}

func callbackYoutubeDownload(update *telegram.CallbackQuery) error {
	i18n := localization.Get(update)
	callbackData := strings.Split(update.DataString(), "|")

	if userID, _ := strconv.Atoi(callbackData[4]); update.SenderID != int64(userID) {
		_, err := update.Answer(i18n("denied-button-alert"), &telegram.CallbackOptions{
			Alert: true,
		})
		return err
	}

	if size, _ := strconv.ParseInt(callbackData[3], 10, 64); size > int64(1572864000) {
		_, err := update.Answer(i18n("video-exceeds-limit", map[string]any{
			"size": int64(1572864000),
		}), &telegram.CallbackOptions{
			Alert: true,
		})
		return err
	}

	_, err := update.Edit(i18n("downloading"))
	if err != nil {
		return err
	}

	outputFile, video, err := youtube.Downloader(callbackData)
	if err != nil {
		_, err := update.Edit(i18n("youtube-error"))
		return err
	}
	itag, _ := strconv.Atoi(callbackData[2])

	_, err = update.Edit(i18n("uploading"))
	if err != nil {
		return err
	}
	switch callbackData[0] {
	case "_aud":
		_, err := update.Client.SendAction(update.Sender.ID, "upload_audio")
		if err != nil {
			return err
		}
	case "_vid":
		_, err := update.Client.SendAction(update.Sender.ID, "upload_video")
		if err != nil {
			return err
		}
	}

	thumbURL := strings.Replace(video.Thumbnails[len(video.Thumbnails)-1].URL, "sddefault", "maxresdefault", 1)
	thumbnail, err := downloader.Downloader(thumbURL)
	if err != nil {
		_, err := update.Edit(i18n("youtube-error"))
		return err
	}

	filename := utils.SanitizeString(fmt.Sprintf("SmudgeLord-%s_%s", video.Author, video.Title))
	mimeType, err := mimetype.DetectReader(bytes.NewReader(outputFile))
	if err != nil {
		return err
	}
	switch callbackData[0] {
	case "_aud":
		_, err := update.ReplyMedia(outputFile, &telegram.MediaOptions{
			FileName: filename + mimeType.Extension(),
			MimeType: mimeType.String(),
			Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeAudio{
				Title:     video.Title,
				Performer: video.Author,
			}},
			Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			Thumb:   thumbnail.Name(),
		})
		if err != nil {
			_, err := update.Edit(i18n("youtube-error"))
			return err
		}
	case "_vid":
		_, err := update.ReplyMedia(outputFile, &telegram.MediaOptions{
			FileName: filename + mimeType.Extension(),
			MimeType: mimeType.String(),
			Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
				SupportsStreaming: true,
				W:                 int32(video.Formats.Itag(itag)[0].Width),
				H:                 int32(video.Formats.Itag(itag)[0].Height),
			}},
			Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			Thumb:   thumbnail.Name(),
		})
		if err != nil {
			_, err := update.Edit(i18n("youtube-error"))
			return err
		}
	}
	_, err = update.Delete()
	return err
}

func Load(client *telegram.Client) {
	utils.SotreHelp("medias")
	client.On("message:"+regexMedia, handlers.HandleCommand(handlerMedias))
	client.On("command:dl", handlers.HandleCommand(handlerMedias))
	client.On("command:ytdl", handlers.HandleCommand(handleYoutubeDownload))
	client.On("callback:^(_(vid|aud))", callbackYoutubeDownload)

	handlers.DisableableCommands = append(handlers.DisableableCommands, "ytdl", "dl")
}
