package soundcloud

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/modules/medias/downloader"
	"github.com/ruizlenato/smudgelord/internal/telegram/helpers"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

var (
	ErrInvalidURL       = fmt.Errorf("invalid SoundCloud URL")
	ErrContentBlocked   = fmt.Errorf("content blocked in region")
	ErrContentPremium   = fmt.Errorf("content is paid/premium")
	ErrNoMediaAvailable = fmt.Errorf("no media available")
	ErrNoSuitableStream = fmt.Errorf("no suitable stream found")
)

func Handle(message string) downloader.PostInfo {
	handler := &Handler{}

	resolvedURL := resolveURL(message)
	if resolvedURL == "" {
		slog.Warn("Invalid SoundCloud URL", "URL", message)
		return downloader.PostInfo{}
	}

	if err := handler.findClientID(); err != nil {
		slog.Error(
			"Failed to find SoundCloud client ID",
			"Error", err.Error(),
		)
		return downloader.PostInfo{}
	}

	trackInfo, err := handler.getTrackInfoAPI(resolvedURL)
	if err != nil {
		slog.Error(
			"Failed to get SoundCloud track info",
			"Error", err.Error(),
			"URL", resolvedURL,
		)
		return downloader.PostInfo{}
	}

	if postInfo, err := downloader.GetMediaCache(fmt.Sprint(trackInfo.ID)); err == nil {
		return postInfo
	}

	if err := validateTrackContent(trackInfo); err != nil {
		slog.Warn(
			"Track validation failed",
			"error", err,
		)
		return downloader.PostInfo{}
	}

	media, caption, err := handler.processTrack(trackInfo)
	if err != nil {
		slog.Error(
			"Failed to process track",
			"error", err,
		)
		return downloader.PostInfo{}
	}

	return downloader.PostInfo{
		ID:      fmt.Sprint(trackInfo.ID),
		Medias:  media,
		Caption: caption,
	}
}

func resolveURL(url string) string {
	shortPattern := `^(?:https?://)?on\.soundcloud\.com/[A-Za-z0-9]+/?$`
	if matched, err := regexp.MatchString(shortPattern, url); err == nil && matched {
		retryCaller := &utils.RetryCaller{
			Caller:       utils.DefaultHTTPCaller,
			MaxAttempts:  3,
			ExponentBase: 2,
			StartDelay:   1 * time.Second,
			MaxDelay:     5 * time.Second,
		}

		response, err := retryCaller.Request(url, utils.RequestParams{
			Method:    "GET",
			Redirects: 2,
		})

		if err != nil {
			slog.Warn("Failed to resolve short URL", "URL", url, "Error", err.Error())
			return ""
		}
		defer response.Body.Close()

		finalURL := response.Request.URL.String()
		normalPattern := `^(?:https?://)?(?:www\.)?soundcloud\.com(?:/[^/\s]+)+/?$`
		if matched, err := regexp.MatchString(normalPattern, finalURL); err == nil && matched {
			return finalURL
		}
	}

	normalPattern := `^(?:https?://)?(?:www\.)?soundcloud\.com(?:/[^/\s]+)+/?$`
	if matched, err := regexp.MatchString(normalPattern, url); err == nil && matched {
		return url
	}

	return ""
}
func (h *Handler) findClientID() error {
	if clientID, err := cache.GetCache("soundcloud_client_id"); clientID != "" && err == nil {
		slog.Debug("Using cached SoundCloud client ID")
		h.clientID = clientID
		return nil
	}

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   2 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	resp, err := retryCaller.Request("https://soundcloud.com/", utils.RequestParams{
		Method: "GET",
		Headers: map[string]string{
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		},
		Redirects: 3,
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	scriptRegex := regexp.MustCompile(`<script.+src="(.+?)">`)
	scriptMatches := scriptRegex.FindAllStringSubmatch(string(body), -1)
	for _, script := range scriptMatches {
		if len(script) < 2 || !strings.HasPrefix(script[1], "https://a-v2.sndcdn.com/") {
			continue
		}
		clientID, err := extractClientIDFromScript(script[1], retryCaller)
		if err != nil {
			slog.Warn("Failed to extract client ID from script", "URL", script[1], "Error", err.Error())
			continue
		}

		if clientID != "" {
			cache.SetCache("soundcloud_client_id", clientID, 24*time.Hour)
			slog.Debug("Found and cached new SoundCloud client ID", "ClientID", clientID[:8]+"...")
			h.clientID = clientID
			return nil
		}
	}

	return fmt.Errorf("client ID not found")
}

func extractClientIDFromScript(scriptURL string, retryCaller *utils.RetryCaller) (string, error) {
	scriptResp, err := retryCaller.Request(scriptURL, utils.RequestParams{
		Method:    "GET",
		Headers:   downloader.GenericHeaders,
		Redirects: 3,
	})
	if err != nil {
		return "", err
	}
	defer scriptResp.Body.Close()

	scriptBody, err := io.ReadAll(scriptResp.Body)
	if err != nil {
		return "", err
	}

	clientIDRegex := regexp.MustCompile(`\("client_id=([A-Za-z0-9]{32})"\)`)
	clientIDMatch := clientIDRegex.FindStringSubmatch(string(scriptBody))
	if len(clientIDMatch) >= 2 {
		return clientIDMatch[1], nil
	}

	return "", fmt.Errorf("client ID not found in script")
}

func (h *Handler) getTrackInfoAPI(trackURL string) (*SoundCloudAPI, error) {
	apiURL, _ := url.Parse("https://api-v2.soundcloud.com/resolve")
	params := apiURL.Query()
	params.Set("url", trackURL)
	params.Set("client_id", h.clientID)
	apiURL.RawQuery = params.Encode()

	retryCaller := &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   2 * time.Second,
		MaxDelay:     5 * time.Second,
	}

	resp, err := retryCaller.Request(apiURL.String(), utils.RequestParams{
		Method:    "GET",
		Headers:   downloader.GenericHeaders,
		Redirects: 3,
	})
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	var soundCloudAPI SoundCloudAPI
	if err := json.NewDecoder(resp.Body).Decode(&soundCloudAPI); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %w", err)
	}

	return &soundCloudAPI, nil
}

func findBestForPreset(transcodes []Transcoding) *Transcoding {
	if len(transcodes) == 0 {
		return nil
	}

	priorities := []string{"aac_160k", "opus_0_0", "abr_sq", "mp3_1_0"}

	for _, preset := range priorities {
		var inferior *Transcoding

		for i := range transcodes {
			if transcodes[i].Snipped {
				continue
			}

			protocol := transcodes[i].Format.Protocol
			if strings.Contains(protocol, "encrypted") {
				continue
			}

			if strings.HasPrefix(transcodes[i].Preset, preset+"_") {
				if protocol == "progressive" {
					return &transcodes[i]
				}

				inferior = &transcodes[i]
			}
		}

		if inferior != nil {
			return inferior
		}
	}

	for i := range transcodes {
		if !transcodes[i].Snipped && !strings.Contains(transcodes[i].Format.Protocol, "encrypted") {
			return &transcodes[i]
		}
	}

	return nil
}

func (h *Handler) processTrack(trackInfo *SoundCloudAPI) ([]telegram.InputMedia, string, error) {
	retryCaller := createRetryCaller()

	stream := findBestForPreset(trackInfo.Media.Transcodings)
	if stream == nil {
		return nil, "", ErrNoSuitableStream
	}

	fileURL, _ := url.Parse(stream.URL)
	params := fileURL.Query()
	params.Set("client_id", h.clientID)
	params.Set("track_authorization", trackInfo.TrackAuthorization)
	fileURL.RawQuery = params.Encode()

	streamResp, err := retryCaller.Request(fileURL.String(), utils.RequestParams{
		Method:    "GET",
		Headers:   downloader.GenericHeaders,
		Redirects: 3,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to get stream URL: %w", err)
	}
	defer streamResp.Body.Close()

	var streamData StreamResponse
	if err := json.NewDecoder(streamResp.Body).Decode(&streamData); err != nil {
		return nil, "", fmt.Errorf("failed to decode stream response: %w", err)
	}

	audioBytes, err := downloader.FetchBytesFromURL(streamData.URL)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download audio: %w", err)
	}

	metadata := extractMetadata(trackInfo)

	var thumbnail []byte
	if coverURL, ok := metadata["cover"].(string); ok && coverURL != "" {
		if thumb, err := downloader.FetchBytesFromURL(coverURL); err == nil {
			thumbnail = thumb
		} else {
			slog.Debug("Failed to fetch thumbnail", "error", err)
		}
	}

	if idx := strings.Index(stream.Format.MimeType, ";"); idx != -1 {
		stream.Format.MimeType = stream.Format.MimeType[:idx]
	}

	uploadedAudio, err := helpers.UploadAudio(helpers.UploadAudioParams{
		File:     audioBytes,
		Thumb:    thumbnail,
		MimeType: stream.Format.MimeType,
		Filename: utils.SanitizeString(
			fmt.Sprintf("Smudge-SoundCloud_%s_%s",
				metadata["artist"], metadata["title"],
			),
		),
		Title:     metadata["title"].(string),
		Performer: metadata["artist"].(string),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to upload audio: %w", err)
	}

	caption := fmt.Sprintf("🎵 <b>%s</b>\n👤 %s", metadata["title"].(string), metadata["artist"].(string))
	medias := []telegram.InputMedia{&uploadedAudio}

	return medias, caption, nil
}

func extractMetadata(trackInfo *SoundCloudAPI) map[string]any {
	metadata := map[string]any{
		"service": "soundcloud",
		"id":      strconv.FormatInt(trackInfo.ID, 10),
		"title":   strings.TrimSpace(trackInfo.Title),
		"artist":  strings.TrimSpace(trackInfo.User.Username),
	}

	if trackInfo.ArtworkURL != "" {
		metadata["cover"] = strings.Replace(trackInfo.ArtworkURL, "-large", "-t1080x1080", 1)

	}

	return metadata
}

func validateTrackContent(trackInfo *SoundCloudAPI) error {
	switch trackInfo.Policy {
	case "BLOCK":
		return ErrContentBlocked
	case "SNIP":
		return ErrContentPremium
	}

	if len(trackInfo.Media.Transcodings) == 0 {
		return ErrNoMediaAvailable
	}

	return nil
}

func createRetryCaller() *utils.RetryCaller {
	return &utils.RetryCaller{
		Caller:       utils.DefaultHTTPCaller,
		MaxAttempts:  3,
		ExponentBase: 2,
		StartDelay:   2 * time.Second,
		MaxDelay:     5 * time.Second,
	}
}
