package lastfmapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	return &LastFM{apiKey: config.LastFMAPIKey}
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

func (lfm *LastFM) GetRecentTrackAPI(username string) (*recentTracks, error) {
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

	if response.StatusCode != http.StatusOK || err != nil {
		return nil, fmt.Errorf("failed to fetch recent tracks, status code: %d", response.StatusCode)
	}
	defer response.Body.Close()

	var recentTracks recentTracks
	err = json.NewDecoder(response.Body).Decode(&recentTracks)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling recent tracks: %w", err)
	}
	return &recentTracks, nil
}

func (lfm *LastFM) GetRecentTrack(methodType, username string) (lastFMRecentTrack, error) {
	recentTracks, err := lfm.GetRecentTrackAPI(username)
	if err != nil {
		return lastFMRecentTrack{}, err
	}

	if recentTracks.RecentTracks == nil || len(*recentTracks.RecentTracks.Track) < 1 {
		return lastFMRecentTrack{}, errors.New("no recent tracks")
	}

	trackInfo := (*recentTracks.RecentTracks.Track)[0]
	image := trackInfo.Image[3].Text
	artist := trackInfo.Artist.Name
	nowplaying := trackInfo.Attr.Nowplaying != ""
	trackloved := trackInfo.Loved == "1"

	var track, album string
	var playcount int

	switch methodType {
	case "track":
		track = trackInfo.Name
		playcount = lfm.PlayCount(recentTracks, methodType)
	case "album":
		album = trackInfo.Album.Text
		playcount = lfm.PlayCount(recentTracks, methodType)
	case "artist":
		playcount = lfm.PlayCount(recentTracks, methodType)
	}

	return lastFMRecentTrack{Track: track, Album: album, Artist: artist, Image: image, Playcount: playcount, Nowplaying: nowplaying, Trackloved: trackloved}, nil
}

func (lfm *LastFM) PlayCount(recentTracks *recentTracks, method string) int {
	username := *recentTracks.RecentTracks.Attr.User
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
			"method":  method + ".getinfo",
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
s		log.Error("Couldn't unmarshal get info",
			"Error", err.Error())
		return 0
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
