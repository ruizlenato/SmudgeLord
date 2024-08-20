package lastFMAPI

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	body := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":  "user.getinfo",
			"user":    username,
			"api_key": lfm.apiKey,
			"format":  "json",
		},
	})

	if body.StatusCode() != 200 {
		return nil
	}

	var userInfo userInfo
	err := json.Unmarshal(body.Body(), &userInfo)
	if err != nil {
		log.Print("[lastfm/GetUser] Error unmarshalling user info:", err)
	}

	if userInfo.User == nil {
		return errors.New("user not found")
	}
	return nil
}

func (lfm *LastFM) GetRecentTrackAPI(username string) *recentTracks {
	body := utils.Request(lastFMAPI, utils.RequestParams{
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

	if body.StatusCode() != 200 {
		return nil
	}

	var recentTracks recentTracks
	err := json.Unmarshal(body.Body(), &recentTracks)
	if err != nil {
		log.Print("[lastfm/GetRecentTrack] Error unmarshalling recent tracks:", err)
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

	// Check if recentTracks is nil or empty
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

	body := utils.Request(lastFMAPI, utils.RequestParams{
		Method: "GET",
		Query: map[string]string{
			"method":  fmt.Sprintf("%s.getinfo", method),
			"user":    username,
			"api_key": lfm.apiKey,
			"artist":  artist,
			method:    methodValue,
			"format":  "json",
		},
	}).Body()

	var getInfo getInfo
	err := json.Unmarshal(body, &getInfo)
	if err != nil {
		log.Print("[lastfm/PlayCount] Error unmarshalling get info:", err)
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
