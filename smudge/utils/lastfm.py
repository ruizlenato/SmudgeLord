import json
import re

from smudge.database.users import get_user_data, register_lastfm
from smudge.utils.utils import http

from ..config import config


class LastFM:
    """This class is a Python interface to Last.fm API."""

    def __init__(self):
        self.api: str = "http://ws.audioscrobbler.com/2.0"
        self.api_key: str = config["LASTFM_API_KEY"]
        self.is_connected: bool = False

    async def register_lastfm(self, id: int, username: str):
        if not await self.check_user(username):
            return False

        await register_lastfm(id, username)
        return True

    @classmethod
    async def get_username(self, id: int):
        username = (await get_user_data(id))["lastfm_username"]
        if not username:
            return False
        return username

    async def check_user(self, username: str):
        if not username:
            return False

        r = await http.get(
            self.api
            + "?method=user.getrecenttracks&limit=3"
            + f"&extended=1&user={username}"
            + f"&api_key={self.api_key}&format=json"
        )

        return r.status_code == 200

    async def track_playcount(self, artist: str, track: str, username: str):
        r = await http.get(
            self.api
            + f"?method=track.getinfo&artist={artist}"
            + f"&track={track}"
            + f"&user={username}&api_key={self.api_key}&format=json"
        )
        res = json.loads(r.content)

        try:
            scrobbles = int(res["track"]["userplaycount"]) + 1
        except KeyError:
            scrobbles = 1
        return scrobbles

    async def track(self, id: int):
        if not await self.check_user(await self.get_username(id)):
            return "No Username"

        r = await http.get(
            self.api
            + "?method=user.getrecenttracks&limit=3"
            + f"&extended=1&user={await self.get_username(id)}&api_key="
            + f"{self.api_key}&format=json"
        )

        if r.status_code != 200:
            return False

        res = json.loads(r.content)
        try:
            ftrack = res["recenttracks"]["track"][0]
        except IndexError:
            return "No Scrobbles"

        playcount = await self.track_playcount(
            ftrack["artist"]["name"], ftrack["name"], await self.get_username(id)
        )

        return {
            "artist": ftrack["artist"]["name"],
            "track": ftrack["name"],
            "loved": int(ftrack["loved"]),
            "playcount": playcount,
            "image": ftrack["image"][3]["#text"],
            "now": bool(ftrack.get("@attr")),
        }

    async def album_playcount(self, artist: str, album: str, username: str):
        r = await http.get(
            self.api
            + f"?method=album.getinfo&artist={artist}"
            + f"&album={album}"
            + f"&user={username}&api_key={self.api_key}&format=json"
        )
        res = json.loads(r.content)

        try:
            scrobbles = int(res["album"]["userplaycount"]) + 1
        except KeyError:
            scrobbles = 1
        return scrobbles

    async def album(self, id: int):
        if not await self.check_user(await self.get_username(id)):
            return "No Username"

        r = await http.get(
            self.api
            + "?method=user.getrecenttracks&limit=3"
            + f"&extended=1&user={await self.get_username(id)}&api_key="
            + f"{self.api_key}&format=json"
        )

        if r.status_code != 200:
            return False

        res = json.loads(r.content)
        try:
            ftrack = res["recenttracks"]["track"][0]
        except IndexError:
            return "No Scrobbles"

        playcount = await self.album_playcount(
            ftrack["artist"]["name"], ftrack["album"]["#text"], await self.get_username(id)
        )

        return {
            "artist": ftrack["artist"]["name"],
            "album": ftrack["album"]["#text"],
            "loved": int(ftrack["loved"]),
            "playcount": playcount,
            "image": ftrack["image"][3]["#text"],
            "now": bool(ftrack.get("@attr")),
        }

    async def artist_playcount(self, artist: str, username: str):
        r = await http.get(
            self.api
            + f"?method=artist.getInfo&artist={artist}"
            + f"&user={username}&api_key={self.api_key}&format=json"
        )
        res = json.loads(r.content)
        try:
            scrobbles = int(res["artist"]["stats"]["userplaycount"]) + 1
        except KeyError:
            scrobbles = 1
        return scrobbles

    async def artist(self, id: int):
        if not await self.check_user(await self.get_username(id)):
            return "No Username"

        r = await http.get(
            self.api
            + "?method=user.getrecenttracks&limit=3"
            + f"&extended=1&user={await self.get_username(id)}&api_key="
            + f"{self.api_key}&format=json"
        )

        if r.status_code != 200:
            return False

        res = json.loads(r.content)
        try:
            ftrack = res["recenttracks"]["track"][0]
        except IndexError:
            return "No Scrobbles"

        if found := re.search(
            'https://lastfm.freetls.fastly.net/i/u/avatar170s/.*?(?=")',
            (
                await http.get(
                    f"https://www.last.fm/music/{str(ftrack['artist']['name'])}/+images"
                )
            ).text,
        ):
            image = found.group().replace("avatar170s", "770x0") + ".jpg"
        else:
            image = ftrack["image"][3]["#text"]

        playcount = await self.artist_playcount(
            ftrack["artist"]["name"], await self.get_username(id)
        )
        return {
            "artist": ftrack["artist"]["name"],
            "loved": int(ftrack["loved"]),
            "playcount": playcount,
            "image": image,
            "now": bool(ftrack.get("@attr")),
        }
