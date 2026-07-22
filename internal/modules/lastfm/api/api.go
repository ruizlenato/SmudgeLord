package lastfmapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/ruizlenato/smudgelord/internal/config"
	"github.com/ruizlenato/smudgelord/internal/database/cache"
	"github.com/ruizlenato/smudgelord/internal/utils"
)

const lastFMAPI = "http://ws.audioscrobbler.com/2.0"

type lastFMRecentTrack struct {
	Track      string
	Album      string
	Artist     string
	Image      string
	Playcount  int
	Nowplaying bool
	Trackloved bool
}

func Init() *LastFM {
	return &LastFM{apiKey: config.LastFMKey}
}

func (lfm *LastFM) GetUser(username string) error {
	response, err := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":  "user.getinfo",
			"user":    username,
			"api_key": lfm.apiKey,
			"format":  "json",
		},
	})

	if err != nil {
		return fmt.Errorf("error requesting user info: %w", err)
	}
	defer response.Body.Close()

	var userInfo userInfo
	err = json.NewDecoder(response.Body).Decode(&userInfo)
	if err != nil {
		return fmt.Errorf("error unmarshalling user info: %w", err)
	}

	if userInfo.User == nil {
		return errors.New("user not found")
	}
	return nil
}

func (lfm *LastFM) GetRecentTrackAPI(username string) *recentTracks {
	response, err := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":   "user.getrecenttracks",
			"user":     username,
			"api_key":  lfm.apiKey,
			"limit":    "3",
			"extended": "1",
			"format":   "json",
		},
	})

	if err != nil || response.StatusCode != http.StatusOK {
		return nil
	}
	defer response.Body.Close()

	var recentTracks recentTracks
	err = json.NewDecoder(response.Body).Decode(&recentTracks)
	if err != nil {
		slog.Error("Couldn't unmarshal recent tracks", "Error", err.Error())
	}
	return &recentTracks
}

func (lfm *LastFM) GetRecentTrack(methodType, username string) (lastFMRecentTrack, error) {
	var track string
	var artist string
	var album string
	var image string
	var playcount int
	var nowplaying bool
	var trackloved bool

	recentTracks := lfm.GetRecentTrackAPI(username)

	if recentTracks == nil {
		return lastFMRecentTrack{}, fmt.Errorf("lastFM error")
	}
	if recentTracks.RecentTracks == nil || len(*recentTracks.RecentTracks.Track) < 1 {
		return lastFMRecentTrack{}, fmt.Errorf("no recent tracks")
	}

	image = (*recentTracks.RecentTracks.Track)[0].Image[3].Text
	artist = (*recentTracks.RecentTracks.Track)[0].Artist.Name
	nowplaying = (*recentTracks.RecentTracks.Track)[0].Attr.Nowplaying != ""
	trackloved = (*recentTracks.RecentTracks.Track)[0].Loved == "1"

	switch methodType {
	case "track":
		track = (*recentTracks.RecentTracks.Track)[0].Name
		playcount = lfm.PlayCount(recentTracks, methodType)
	case "album":
		album = (*recentTracks.RecentTracks.Track)[0].Album.Text
		playcount = lfm.PlayCount(recentTracks, methodType)
	case "artist":
		playcount = lfm.PlayCount(recentTracks, methodType)
	}

	return lastFMRecentTrack{Track: track, Album: album, Artist: artist, Image: image, Playcount: playcount, Nowplaying: nowplaying, Trackloved: trackloved}, nil
}

func (lfm *LastFM) PlayCount(recentTracks *recentTracks, method string) int {
	username := *recentTracks.RecentTracks.Attr.User // Dereference the pointer to get the string value
	artist := (*recentTracks.RecentTracks.Track)[0].Artist.Name
	var methodValue string

	switch method {
	case "track":
		methodValue = (*recentTracks.RecentTracks.Track)[0].Name
	case "album":
		methodValue = (*recentTracks.RecentTracks.Track)[0].Album.Text
	case "artist":
		methodValue = (*recentTracks.RecentTracks.Track)[0].Artist.Name
	}

	response, err := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":  fmt.Sprintf("%s.getinfo", method),
			"user":    username,
			"api_key": lfm.apiKey,
			"artist":  artist,
			method:    methodValue,
			"format":  "json",
		},
	})

	if err != nil {
		slog.Error("Couldn't request get info",
			"Error", err.Error())
		return 0
	}
	defer response.Body.Close()

	var getInfo getInfo
	err = json.NewDecoder(response.Body).Decode(&getInfo)
	if err != nil {
		slog.Error("Couldn't unmarshal get info",
			"Error", err.Error())
	}

	var userPlaycount int
	switch method {
	case "track":
		userPlaycount, _ = strconv.Atoi(getInfo.Track.UserPlaycount)
	case "album":
		userPlaycount = getInfo.Album.Userplaycount
	case "artist":
		userPlaycount, _ = strconv.Atoi(getInfo.Artist.Stats.Userplaycount)
	}

	if userPlaycount == 0 {
		return 1
	}

	return userPlaycount
}

func (lfm *LastFM) GetTopAlbums(username, period string, limit int) ([]topAlbum, error) {
	if limit <= 0 {
		limit = 9
	}
	if limit > 500 {
		limit = 500
	}

	body, err := lfm.getTop(methodTopAlbums, username, period, limit)
	if err != nil {
		return nil, err
	}

	var response topAlbumsResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding top albums: %w", err)
	}
	if response.TopAlbums == nil {
		return nil, errors.New("no top albums")
	}

	return response.TopAlbums.Albums, nil
}

func (lfm *LastFM) GetTopArtists(username, period string, limit int) ([]topArtist, error) {
	if limit <= 0 {
		limit = 9
	}
	if limit > 500 {
		limit = 500
	}

	body, err := lfm.getTop(methodTopArtists, username, period, limit)
	if err != nil {
		return nil, err
	}

	var response topArtistsResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding top artists: %w", err)
	}
	if response.TopArtists == nil {
		return nil, errors.New("no top artists")
	}

	return response.TopArtists.Artists, nil
}

func (lfm *LastFM) GetTopTracks(username, period string, limit int) ([]topTrack, error) {
	if limit <= 0 {
		limit = 9
	}
	if limit > 500 {
		limit = 500
	}

	body, err := lfm.getTop(methodTopTracks, username, period, limit)
	if err != nil {
		return nil, err
	}

	var response topTracksResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding top tracks: %w", err)
	}
	if response.TopTracks == nil {
		return nil, errors.New("no top tracks")
	}

	return response.TopTracks.Tracks, nil
}

func (lfm *LastFM) GetTopCollageItems(collageType, username, period string, limit int) ([]TopCollageItem, error) {
	if from, to, ok := parseCustomPeriodRange(period); ok {
		switch collageType {
		case "album":
			albums, err := lfm.GetWeeklyAlbums(username, from, to)
			if err != nil {
				return nil, err
			}
			type albumResult struct {
				idx  int
				item TopCollageItem
			}
			items := make([]TopCollageItem, len(albums))
			results := make(chan albumResult, len(albums))
			sem := make(chan struct{}, 8)
			var wg sync.WaitGroup

			for i, a := range albums {
				wg.Add(1)
				go func(idx int, album weeklyAlbum) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					plays, _ := strconv.Atoi(album.Playcount)
					artistName := strings.TrimSpace(album.Artist.Text)
					if artistName == "" {
						artistName = strings.TrimSpace(album.Artist.Name)
					}
					imageURL := lfm.ResolveAlbumImage(artistName, album.Name, album.MBID, bestImageURL(album.Image))
					results <- albumResult{idx: idx, item: TopCollageItem{
						Title:     album.Name,
						Subtitle:  artistName,
						Playcount: plays,
						ImageURL:  imageURL,
					}}
				}(i, a)
			}

			go func() {
				wg.Wait()
				close(results)
			}()

			for res := range results {
				items[res.idx] = res.item
			}
			return items, nil
		case "artist":
			artists, err := lfm.GetWeeklyArtists(username, from, to)
			if err != nil {
				return nil, err
			}
			type artistResult struct {
				idx  int
				item TopCollageItem
			}
			items := make([]TopCollageItem, len(artists))
			results := make(chan artistResult, len(artists))
			sem := make(chan struct{}, 8)
			var wg sync.WaitGroup

			for i, a := range artists {
				wg.Add(1)
				go func(idx int, artist weeklyArtist) {
					defer wg.Done()
					sem <- struct{}{}
					defer func() { <-sem }()

					plays, _ := strconv.Atoi(artist.Playcount)
					imageURL := lfm.ResolveArtistImage(artist.Name, bestImageURL(artist.Image))
					results <- artistResult{idx: idx, item: TopCollageItem{
						Title:     artist.Name,
						Playcount: plays,
						ImageURL:  imageURL,
					}}
				}(i, a)
			}

			go func() {
				wg.Wait()
				close(results)
			}()

			for res := range results {
				items[res.idx] = res.item
			}
			return items, nil
		case "track":
			tracks, err := lfm.GetWeeklyTracks(username, from, to)
			if err != nil {
				return nil, err
			}
			items := make([]TopCollageItem, 0, len(tracks))
			for _, t := range tracks {
				plays, _ := strconv.Atoi(t.Playcount)
				artistName := strings.TrimSpace(t.Artist.Text)
				if artistName == "" {
					artistName = strings.TrimSpace(t.Artist.Name)
				}
				items = append(items, TopCollageItem{
					Title:     t.Name,
					Subtitle:  artistName,
					Playcount: plays,
					ImageURL:  bestImageURL(t.Image),
				})
			}
			return items, nil
		default:
			return nil, fmt.Errorf("unsupported collage type")
		}
	}

	switch collageType {
	case "album":
		albums, err := lfm.GetTopAlbums(username, period, limit)
		if err != nil {
			return nil, err
		}
		items := make([]TopCollageItem, 0, len(albums))
		for _, a := range albums {
			plays, _ := strconv.Atoi(a.Playcount)
			items = append(items, TopCollageItem{
				Title:     a.Name,
				Subtitle:  a.Artist.Name,
				Playcount: plays,
				ImageURL:  bestImageURL(a.Image),
			})
		}
		return items, nil
	case "artist":
		artists, err := lfm.GetTopArtists(username, period, limit)
		if err != nil {
			return nil, err
		}
		type artistResult struct {
			idx  int
			item TopCollageItem
		}
		items := make([]TopCollageItem, len(artists))
		results := make(chan artistResult, len(artists))
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup

		for i, a := range artists {
			wg.Add(1)
			go func(idx int, artist topArtist) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				plays, _ := strconv.Atoi(artist.Playcount)
				imageURL := lfm.ResolveArtistImage(artist.Name, bestImageURL(artist.Image))
				results <- artistResult{idx: idx, item: TopCollageItem{
					Title:     artist.Name,
					Playcount: plays,
					ImageURL:  imageURL,
				}}
			}(i, a)
		}

		go func() {
			wg.Wait()
			close(results)
		}()

		for res := range results {
			items[res.idx] = res.item
		}
		return items, nil
	case "track":
		tracks, err := lfm.GetTopTracks(username, period, limit)
		if err != nil {
			return nil, err
		}
		items := make([]TopCollageItem, 0, len(tracks))
		for _, t := range tracks {
			plays, _ := strconv.Atoi(t.Playcount)
			items = append(items, TopCollageItem{
				Title:     t.Name,
				Subtitle:  t.Artist.Name,
				Playcount: plays,
				ImageURL:  bestImageURL(t.Image),
			})
		}
		return items, nil
	default:
		return nil, fmt.Errorf("unsupported collage type")
	}
}

const (
	methodTopAlbums  = "user.gettopalbums"
	methodTopArtists = "user.gettopartists"
	methodTopTracks  = "user.gettoptracks"
)

func (lfm *LastFM) getTop(method, username, period string, limit int) ([]byte, error) {
	if period == "" {
		period = "overall"
	}

	response, err := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":  method,
			"user":    username,
			"api_key": lfm.apiKey,
			"period":  period,
			"limit":   strconv.Itoa(limit),
			"format":  "json",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error requesting top items: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading top items body: %w", err)
	}

	if bytes.Contains(body, []byte("\"error\"")) {
		return nil, errors.New("lastFM error")
	}

	return body, nil
}

func (lfm *LastFM) GetWeeklyAlbums(username string, from, to int64) ([]weeklyAlbum, error) {
	body, err := lfm.getWeekly("user.getweeklyalbumchart", username, from, to)
	if err != nil {
		return nil, err
	}

	var response weeklyAlbumsResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding weekly albums: %w", err)
	}
	if response.WeeklyAlbumChart == nil {
		return nil, errors.New("no weekly albums")
	}
	return response.WeeklyAlbumChart.Albums, nil
}

func (lfm *LastFM) GetWeeklyArtists(username string, from, to int64) ([]weeklyArtist, error) {
	body, err := lfm.getWeekly("user.getweeklyartistchart", username, from, to)
	if err != nil {
		return nil, err
	}

	var response weeklyArtistsResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding weekly artists: %w", err)
	}
	if response.WeeklyArtistChart == nil {
		return nil, errors.New("no weekly artists")
	}
	return response.WeeklyArtistChart.Artists, nil
}

func (lfm *LastFM) GetWeeklyTracks(username string, from, to int64) ([]weeklyTrack, error) {
	body, err := lfm.getWeekly("user.getweeklytrackchart", username, from, to)
	if err != nil {
		return nil, err
	}

	var response weeklyTracksResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&response); err != nil {
		return nil, fmt.Errorf("error decoding weekly tracks: %w", err)
	}
	if response.WeeklyTrackChart == nil {
		return nil, errors.New("no weekly tracks")
	}
	return response.WeeklyTrackChart.Tracks, nil
}

func (lfm *LastFM) getWeekly(method, username string, from, to int64) ([]byte, error) {
	response, err := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":  method,
			"user":    username,
			"api_key": lfm.apiKey,
			"from":    strconv.FormatInt(from, 10),
			"to":      strconv.FormatInt(to, 10),
			"format":  "json",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error requesting weekly items: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", response.StatusCode)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading weekly items body: %w", err)
	}

	if bytes.Contains(body, []byte("\"error\"")) {
		return nil, errors.New("lastFM error")
	}

	return body, nil
}

func parseCustomPeriodRange(period string) (int64, int64, bool) {
	parts := strings.Split(period, ":")
	if len(parts) != 4 || parts[0] != "range" {
		return 0, 0, false
	}
	from, err1 := strconv.ParseInt(parts[1], 10, 64)
	to, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil || to <= from {
		return 0, 0, false
	}
	return from, to, true
}

func bestImageURL(images []image) string {
	for i := len(images) - 1; i >= 0; i-- {
		if strings.TrimSpace(images[i].Text) != "" {
			return images[i].Text
		}
	}
	return ""
}

func (lfm *LastFM) GetArtistImage(artistName string) string {
	if strings.TrimSpace(artistName) == "" {
		return ""
	}

	response, err := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":  "artist.getinfo",
			"artist":  artistName,
			"api_key": lfm.apiKey,
			"format":  "json",
		},
	})
	if err != nil {
		return ""
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return ""
	}

	var payload struct {
		Artist *struct {
			Image []image `json:"image"`
		} `json:"artist"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return ""
	}
	if payload.Artist == nil {
		return ""
	}

	return bestImageURL(payload.Artist.Image)
}

func (lfm *LastFM) GetAlbumImage(artistName, albumName, albumMBID string) string {
	artistName = strings.TrimSpace(artistName)
	albumName = strings.TrimSpace(albumName)
	albumMBID = strings.TrimSpace(albumMBID)
	if albumMBID == "" && (artistName == "" || albumName == "") {
		return ""
	}

	query := map[string]string{
		"method":  "album.getinfo",
		"api_key": lfm.apiKey,
		"format":  "json",
	}
	if albumMBID != "" {
		query["mbid"] = albumMBID
	} else {
		query["artist"] = artistName
		query["album"] = albumName
	}

	response, err := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query:  query,
	})
	if err != nil {
		return ""
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return ""
	}

	var payload struct {
		Album *struct {
			Image []image `json:"image"`
		} `json:"album"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return ""
	}
	if payload.Album == nil {
		return ""
	}

	return bestImageURL(payload.Album.Image)
}

func (lfm *LastFM) ResolveArtistImage(artistName, initial string) string {
	key := normalizeArtistName(artistName)
	if key == "" {
		return ""
	}

	if cached := lfm.getArtistImageCache(key); isUsableArtistImageURL(cached) {
		return cached
	}
	if cached := lfm.getArtistImageCache(key); cached == "__none__" {
		return ""
	}

	imageURL := initial
	if !isUsableArtistImageURL(imageURL) {
		imageURL = lfm.GetArtistImage(artistName)
	}
	if !isUsableArtistImageURL(imageURL) {
		imageURL = lfm.GetDeezerArtistImage(artistName)
	}

	if isUsableArtistImageURL(imageURL) {
		lfm.setArtistImageCache(key, imageURL, 24*time.Hour)
	} else {
		lfm.setArtistImageCache(key, "__none__", 30*time.Minute)
	}

	return imageURL
}

func (lfm *LastFM) ResolveAlbumImage(artistName, albumName, albumMBID, initial string) string {
	key := normalizeAlbumKey(artistName, albumName, albumMBID)
	if key == "" {
		return ""
	}

	cached := lfm.getAlbumImageCache(key)
	if isUsableArtistImageURL(cached) {
		return cached
	}
	if cached == "__none__" {
		return ""
	}

	imageURL := initial
	if !isUsableArtistImageURL(imageURL) {
		imageURL = lfm.GetAlbumImage(artistName, albumName, albumMBID)
	}

	if isUsableArtistImageURL(imageURL) {
		lfm.setAlbumImageCache(key, imageURL, 24*time.Hour)
	} else {
		lfm.setAlbumImageCache(key, "__none__", 30*time.Minute)
	}

	return imageURL
}

func (lfm *LastFM) GetDeezerArtistImage(artistName string) string {
	artistName = strings.TrimSpace(artistName)
	if artistName == "" {
		return ""
	}

	response, err := utils.Request("https://api.deezer.com/search/artist", utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"q":      artistName,
			"limit":  "5",
			"strict": "on",
		},
	})
	if err != nil {
		return ""
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return ""
	}

	var payload struct {
		Data []struct {
			Name          string `json:"name"`
			PictureXL     string `json:"picture_xl"`
			PictureBig    string `json:"picture_big"`
			PictureMedium string `json:"picture_medium"`
			PictureSmall  string `json:"picture_small"`
			Picture       string `json:"picture"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return ""
	}

	for _, node := range payload.Data {
		if !sameArtistName(artistName, node.Name) {
			continue
		}
		for _, candidate := range []string{node.PictureXL, node.PictureBig, node.PictureMedium, node.PictureSmall, node.Picture} {
			if strings.TrimSpace(candidate) != "" {
				return candidate
			}
		}
	}

	return ""
}

func sameArtistName(expected, actual string) bool {
	e := normalizeArtistName(expected)
	a := normalizeArtistName(actual)
	return e != "" && e == a
}

func normalizeArtistName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	decomposed := norm.NFKD.String(s)
	b := make([]rune, 0, len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b = append(b, r)
		}
	}
	clean := strings.Join(strings.Fields(string(b)), " ")
	return artistNameSanitizer.ReplaceAllString(clean, " ")
}

func normalizeAlbumKey(artistName, albumName, albumMBID string) string {
	mbid := strings.TrimSpace(strings.ToLower(albumMBID))
	if mbid != "" {
		return "mbid:" + mbid
	}

	artist := normalizeArtistName(artistName)
	album := normalizeArtistName(albumName)
	if artist == "" || album == "" {
		return ""
	}
	return artist + "|" + album
}

var artistNameSanitizer = regexp.MustCompile(`\s+`)

func isUsableArtistImageURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	p := strings.ToLower(parsed.Path)
	if strings.Contains(p, "2a96cbd8b46e442fc41c2b86b821562f") {
		return false
	}
	if strings.Contains(p, "4128a6eb29f94943c9d206c08e625904") {
		return false
	}
	return true
}

func (lfm *LastFM) getArtistImageCache(key string) string {
	val, err := cache.GetCache("lastfm:artist_image:" + key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

func (lfm *LastFM) setArtistImageCache(key, url string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	_ = cache.SetCache("lastfm:artist_image:"+key, url, ttl)
}

func (lfm *LastFM) getAlbumImageCache(key string) string {
	val, err := cache.GetCache("lastfm:album_image:" + key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(val)
}

func (lfm *LastFM) setAlbumImageCache(key, url string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = time.Hour
	}
	_ = cache.SetCache("lastfm:album_image:"+key, url, ttl)
}
