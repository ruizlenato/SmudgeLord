package lastFMAPI

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ruizlenato/smudgelord/internal/config"
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
