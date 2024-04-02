package lastfm

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"

	"smudgelord/smudgelord/config"
	"smudgelord/smudgelord/utils"
)

const lastFMAPI = "http://ws.audioscrobbler.com/2.0"

type LastFM struct {
	apiKey string
}

func Init() *LastFM {
	return &LastFM{apiKey: config.LastFMKey}
}

func (lfm *LastFM) GetUser(username string) error {
	body := utils.RequestGET(lastFMAPI, utils.RequestGETParams{Query: map[string]string{"method": "user.getinfo", "user": username, "api_key": lfm.apiKey, "format": "json"}})
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

func (lfm *LastFM) GetRecentTrack(username string) *recentTracks {
	body := utils.RequestGET(lastFMAPI, utils.RequestGETParams{Query: map[string]string{"method": "user.getrecenttracks", "user": username, "api_key": lfm.apiKey, "limit": "3", "extended": "1", "format": "json"}})
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

func (lfm *LastFM) PlayCount(recentTracks *recentTracks, method string) int {
	var methodValue string
	username := *recentTracks.RecentTracks.Attr.User // Dereference the pointer to get the string value
	artist := recentTracks.RecentTracks.Track[0].Artist.Name
	if method == "track" {
		methodValue = recentTracks.RecentTracks.Track[0].Name
	} else if method == "album" {
		methodValue = recentTracks.RecentTracks.Track[0].Album.Text
	} else if method == "artist" {
		methodValue = recentTracks.RecentTracks.Track[0].Artist.Name
	}

	body := utils.RequestGET(lastFMAPI, utils.RequestGETParams{Query: map[string]string{"method": fmt.Sprintf("%s.getinfo", method), "user": username, "api_key": lfm.apiKey, "artist": artist, method: methodValue, "format": "json"}}).Body()
	var getInfo getInfo
	err := json.Unmarshal(body, &getInfo)
	if err != nil {
		log.Print("[lastfm/PlayCount] Error unmarshalling get info:", err)
	}
	var userPlaycount int
	switch method {
	case "track":
		userPlaycount, err = strconv.Atoi(getInfo.Track.UserPlaycount)
	case "album":
		userPlaycount = getInfo.Album.Userplaycount
	case "artist":
		userPlaycount, err = strconv.Atoi(getInfo.Artist.Stats.Userplaycount)
	}
	if err != nil {
		return 0
	}

	if userPlaycount == 0 {
		return 1
	}

	return userPlaycount
}
