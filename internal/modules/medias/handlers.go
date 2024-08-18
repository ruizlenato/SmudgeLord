package medias

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/kkdai/youtube/v2"
	"github.com/ruizlenato/smudgelord/internal/database"
	"github.com/ruizlenato/smudgelord/internal/localization"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/instagram"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/tiktok"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/twitter"
	yt "github.com/ruizlenato/smudgelord/internal/modules/medias/downloader/youtube"
	"github.com/ruizlenato/smudgelord/internal/telegram/handlers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const (
	regexMedia     = `(?:http(?:s)?://)?(?:m|vm|www|mobile)?(?:.)?(?:instagram|twitter|x|tiktok|reddit|twitch).(?:com|net|tv)/(?:\S*)`
	maxSizeCaption = 1024
)

func handlerMedias(message *telegram.NewMessage) error {
	var mediaItems []telegram.InputMedia
	var caption string
	var postID string

	if !regexp.MustCompile(`^/dl`).MatchString(message.Text()) && message.ChatType() != telegram.EntityUser {
		var mediasAuto bool
		if err := database.DB.QueryRow("SELECT mediasAuto FROM chats WHERE id = ?;", message.ChatID()).Scan(&mediasAuto); err != nil || !mediasAuto {
			return nil
		}
	}
	i18n := localization.Get(message)

	url := regexp.MustCompile(regexMedia).FindStringSubmatch(message.Text())
	if len(url) < 1 {
		_, err := message.Reply(i18n("medias.noURL"))
		return err
	}

	mediaHandlers := map[string]func(*telegram.NewMessage) ([]telegram.InputMedia, []string){
		"(twitter|x).com/": twitter.Handle,
		"instagram.com/":   instagram.Handle,
		"tiktok.com/":      tiktok.Handle,
	}

	for pattern, handler := range mediaHandlers {
		if match, _ := regexp.MatchString(pattern, message.Text()); match {
			var result []string
			mediaItems, result = handler(message)
			if len(result) == 2 {
				caption, postID = result[0], result[1]
			}
			break
		}
	}

	if _, InputMediaUploadedPhoto := mediaItems[0].(*telegram.InputMediaUploadedPhoto); mediaItems == nil || (len(mediaItems) == 1 &&
		InputMediaUploadedPhoto &&
		message.Media() != nil &&
		message.Media().(*telegram.MessageMediaWebPage) != nil) {
		return nil
	}

	if media, ok := mediaItems[0].(*telegram.InputMediaUploadedDocument); ok {
		fmt.Printf("%+v\n", media.File)
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
	err = downloader.SetMediaCache(replied, postID)
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
		_, err := message.Reply(i18n("medias.youtubeNoURL"))
		return err
	}

	ytClient := youtube.Client{}
	video, err := ytClient.GetVideo(videoURL)
	if err != nil {
		_, err := message.Reply(i18n("medias.youtubeInvalidURL"))
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

	var audioStream youtube.Format
	if len(video.Formats.Itag(140)) > 0 {
		audioStream = video.Formats.Itag(140)[0]
	} else {
		audioStream = video.Formats.WithAudioChannels().Type("audio/mp4")[1]
	}

	text := fmt.Sprintf(i18n("medias.youtubeVideoInfo"),
		video.Title, video.Author,
		float64(audioStream.ContentLength)/(1024*1024),
		float64(videoStream.ContentLength+audioStream.ContentLength)/(1024*1024),
		video.Duration.String())

	keyboard := telegram.Button{}.Keyboard(
		telegram.Button{}.Row(
			telegram.Button{}.Data(
				i18n("medias.youtubeDownloadAudio"),
				fmt.Sprintf("_aud|%s|%d|%d|%d", video.ID, audioStream.ItagNo, audioStream.ContentLength, message.SenderID()),
			),
			telegram.Button{}.Data(
				i18n("medias.youtubeDownloadVideo"),
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
		_, err := update.Answer(i18n("medias.youtubeDenied"), &telegram.CallbackOptions{
			Alert: true,
		})
		return err
	}

	if size, _ := strconv.ParseInt(callbackData[3], 10, 64); size > int64(1572864000) {
		_, err := update.Answer(i18n("medias.youtubeBigFile"), &telegram.CallbackOptions{
			Alert: true,
		})
		return err
	}

	_, err := update.Edit(i18n("medias.downloading"))
	if err != nil {
		return err
	}

	outputFile, video, err := yt.Downloader(callbackData)
	if err != nil {
		_, err := update.Edit(i18n("medias.youtubeError"))
		return err
	}
	itag, _ := strconv.Atoi(callbackData[2])

	_, err = update.Edit(i18n("medias.uploading"))
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

	_, err = outputFile.Seek(0, 0)
	if err != nil {
		_, err := update.Edit(i18n("medias.youtubeError"))
		return err
	}
	thumbURL := strings.Replace(video.Thumbnails[len(video.Thumbnails)-1].URL, "sddefault", "maxresdefault", 1)
	thumbnail, err := downloader.Downloader(thumbURL)
	if err != nil {
		_, err := update.Edit(i18n("medias.youtubeError"))
		return err
	}

	defer func() {
		if err := os.Remove(thumbnail.Name()); err != nil {
			log.Printf("Failed to remove thumbnail: %v", err)
		}
		if err := os.Remove(outputFile.Name()); err != nil {
			log.Printf("Failed to remove outputFile: %v", err)
		}
	}()

	switch callbackData[0] {
	case "_aud":
		_, err := update.ReplyMedia(outputFile.Name(), &telegram.MediaOptions{
			Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeAudio{
				Title:     video.Title,
				Performer: video.Author,
			}},
			Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			Thumb:   thumbnail.Name(),
		})
		if err != nil {
			_, err := update.Edit(i18n("medias.youtubeError"))
			return err
		}
	case "_vid":
		_, err := update.ReplyMedia(outputFile.Name(), &telegram.MediaOptions{
			Attributes: []telegram.DocumentAttribute{&telegram.DocumentAttributeVideo{
				SupportsStreaming: true,
				W:                 int32(video.Formats.Itag(itag)[0].Width),
				H:                 int32(video.Formats.Itag(itag)[0].Height),
			}},
			Caption: fmt.Sprintf("<b>%s:</b> %s", video.Author, video.Title),
			Thumb:   thumbnail.Name(),
		})
		if err != nil {
			_, err := update.Edit(i18n("medias.youtubeError"))
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
