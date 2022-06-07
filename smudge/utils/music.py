# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import re
import os
import orjson
import tempfile

from smudge.utils import http
from smudge.config import (
    SPOTIFY_BASIC,
    SPOTIFY_CLIENT_ID,
    SPOTIFY_CLIENT_SECRET,
    LASTFM_API_KEY,
)
from smudge.database.music import set_spot_user, get_spot_user, unreg_spot

from urllib.parse import urlparse, quote
from PIL import Image, ImageDraw, ImageFont

from asyncio import get_event_loop


class SpotifyUser:
    authorize_url = "https://accounts.spotify.com/authorize"
    token_url = "https://accounts.spotify.com/api/token"

    def __init__(self, ClientID, ClientSecret):
        self.client_id = ClientID
        self.client_secret = ClientSecret
        self.redirect_url = "https://ruizlenato.github.io/Smudge/go"

    def getAuthUrl(self):
        authorization_redirect_url = (
            self.authorize_url
            + "?response_type=code&client_id="
            + self.client_id
            + "&redirect_uri="
            + self.redirect_url
            + "&scope=user-read-currently-playing"
        )
        return authorization_redirect_url

    async def getAccessToken(self, user_id, token: str):
        r = await http.post(
            "https://accounts.spotify.com/api/token",
            headers=dict(Authorization=f"Basic {SPOTIFY_BASIC}"),
            data=dict(
                grant_type="authorization_code",
                code=token,
                redirect_uri=self.redirect_url,
            ),
        )

        if r.status_code in range(200, 299):
            b = orjson.loads(r.content)
            await set_spot_user(user_id, b["access_token"], b["refresh_token"])
            return True, b["access_token"]
        else:
            return False

    async def getCurrentyPlayingSong(self, refreshToken):
        data = {
            "grant_type": "refresh_token",
            "refresh_token": refreshToken,
            "client_id": self.client_id,
            "client_secret": self.client_secret,
        }
        token = orjson.loads((await http.post(self.token_url, data=data)).content)

        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "Authorization": "Bearer " + token["access_token"],
        }
        return await http.get(
            "https://api.spotify.com/v1/me/player/currently-playing", headers=headers
        )

    async def getCurrentUser(self, refreshToken):
        data = {
            "grant_type": "refresh_token",
            "refresh_token": refreshToken,
            "client_id": self.client_id,
            "client_secret": self.client_secret,
        }
        token = orjson.loads((await http.post(self.token_url, data=data)).content)

        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "Authorization": "Bearer " + token["access_token"],
        }
        return await http.get("https://api.spotify.com/v1/me", headers=headers)

    async def search(self, refreshToken, query, type, market, limit):
        data = {
            "grant_type": "refresh_token",
            "refresh_token": refreshToken,
            "client_id": self.client_id,
            "client_secret": self.client_secret,
        }
        token = orjson.loads((await http.post(self.token_url, data=data)).content)

        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "Authorization": "Bearer " + token["access_token"],
        }
        return await http.get(
            f"https://api.spotify.com/v1/search?q={query}&type={type}&market={market}&limit={limit}",
            headers=headers,
        )


Spotify = SpotifyUser(SPOTIFY_CLIENT_ID, SPOTIFY_CLIENT_SECRET)


async def refresh_token(user_id):
    usr = await get_spot_user(user_id)
    b = orjson.loads(
        (
            await http.post(
                "https://accounts.spotify.com/api/token",
                headers=dict(Authorization=f"Basic {SPOTIFY_BASIC}"),
                data=dict(grant_type="refresh_token", refresh_token=usr),
            )
        ).content
    )
    await set_spot_user(user_id, b["access_token"], usr)
    return b["access_token"]


class LastFMError(Exception):
    pass


class Fonts:
    JetBrainsMono = "smudge/fonts/JetBrainsMono-Regular.ttf"


class LastFMImage:
    def __init__(self):
        self.url = "http://ws.audioscrobbler.com/2.0/"
        self.url_image = "https://www.last.fm/music/"
        self.api_key = LASTFM_API_KEY

    async def _get_body(self):
        return f"{self.url}?method={self.method}&user={self.user}&api_key={self.api_key}&period={self.period}&limit={self.limit}&format=json"

    async def get_artists(self):
        r = await http.get(await self._get_body())
        b = orjson.loads(r.content)
        if r.status_code == 403:
            print("cannot access")
            return False
        if "error" in b:
            raise LastFMError(b["message"])
        artists = b["topartists"]["artist"]
        return artists

    async def get_tracks(self):
        r = await http.get(await self._get_body())
        b = orjson.loads(r.content)
        if r.status_code == 403:
            print("cannot access")
            return False
        if "error" in b:
            raise LastFMError(b["message"])
        tracks = b["toptracks"]["track"]
        return tracks

    async def get_albums(self):
        r = await http.get(await self._get_body())
        b = orjson.loads(r.content)
        if r.status_code == 403:
            print("cannot access")
            return False
        if "error" in b:
            raise LastFMError(b["message"])
        album = b["topalbums"]["album"]
        return album

    @staticmethod
    async def _download_file(url, path):
        with open(path, "wb") as f:
            try:
                res = await http.get(url)
                f.write(res.content)
            except:
                image = Image.new("RGB", (500, 500))
                await image.save(path)
        return path

    async def _get_image_from_cache(self, url):
        url_parts = urlparse(url)
        cache_name = str(url_parts.path).replace("/", "")
        cache_name = cache_name or "empty.png"
        if os.path.isfile(self.cache_path + "/" + cache_name):
            return self.cache_path + "/" + cache_name
        path = await self._download_file(url, self.cache_path + "/" + cache_name)
        return path

    async def _get_artists_images(self, artists):
        image_info = []
        for artist in artists:
            response = await http.get(
                self.url_image + str(quote(artist["name"])) + "/+images"
            )
            found = re.search(
                'https://lastfm.freetls.fastly.net/i/u/avatar170s/.*?(?=")',
                response.text,
            )
            if found:
                image_url = found.group().replace("avatar170s", "770x0") + ".jpg"

            path = await self._get_image_from_cache(image_url)
            spot_info = {
                "name": artist["name"],
                "playcount": artist["playcount"],
                "path": path,
            }
            image_info.append(spot_info)
        return image_info

    async def _get_tracks_images(self, tracks):
        image_info = []
        for track in tracks:
            artist_name = track["artist"]["name"]
            track_name = track["name"]
            info = orjson.loads(
                (
                    await http.get(
                        f"{self.url}?method=track.getInfo&api_key={self.api_key}&artist={quote(artist_name)}&track={quote(track_name)}&format=json"
                    )
                ).content
            )

            try:
                usr = await get_spot_user("1032274246")
                spotify_json = orjson.loads(
                    (
                        await Spotify.search(
                            usr, f"{artist_name}+{track_name}", "track", "US", 20
                        )
                    ).content
                )
                json = spotify_json["tracks"]["items"]
                for i in json:
                    if i["album"]["name"] == info["track"]["album"]["title"]:
                        if i["album"]["album_type"] == "album" or "single":
                            if i["album"]["artists"][0]["name"] in artist_name:
                                url = i["album"]["images"][1]["url"]
                            url = i["album"]["images"][1]["url"]
                    else:
                        if i["album"]["album_type"] == "album" or "single":
                            url = i["album"]["images"][1]["url"]
                    break
            except KeyError:
                res = await http.get(track["url"], follow_redirects=True)
                found = re.search(
                    r"(?s)<span class=\"cover-art\"*?>.*?<img.*?src=\"([^\"]+)\"",
                    res.text,
                )
                if found:
                    url = found.groups()[0]

            path = await self._get_image_from_cache(url)
            spot_info = {
                "name": track["name"],
                "artist": track["artist"]["name"],
                "playcount": track["playcount"],
                "path": path,
            }
            image_info.append(spot_info)
        return image_info

    async def _get_albums_images(self, albums):
        image_info = []
        for album in albums:
            url = album["image"][3]["#text"]
            path = await self._get_image_from_cache(url)
            spot_info = {
                "name": album["name"],
                "artist": album["artist"]["name"],
                "playcount": album["playcount"],
                "path": path,
            }
            image_info.append(spot_info)
        return image_info

    async def _insert_name(self, w, h, image, name, artist, playcount, cursor):
        if w and h > 700 < 1000:
            font = ImageFont.truetype(Fonts.JetBrainsMono, size=50)
        elif w and h > 200 < 400:
            font = ImageFont.truetype(Fonts.JetBrainsMono, size=15)
        draw = ImageDraw.Draw(image, "RGBA")
        x = cursor[0]
        y = cursor[1]
        if artist == None:
            draw.multiline_text(
                (x + 8, y + 1),
                f"{name}\n{playcount} plays",
                font=font,
                fill="white",
                stroke_width=3,
                stroke_fill="black",
                spacing=5,
            )
        else:
            draw.multiline_text(
                (x + 5, y + 1),
                f"{name}\n{artist}\n{playcount} plays",
                font=font,
                fill="white",
                stroke_width=3,
                stroke_fill="black",
                spacing=0,
            )

    async def create_collage(
        self,
        username: str,
        method: str,
        period: str,
        col: int = 3,
        row: int = 3,
    ):
        self.user = username
        self.period = period
        self.method = f"user.gettop{method}"
        self.limit = int(col) * int(row)
        self.cache_path = os.path.join(tempfile.mkdtemp())

        if self.method == "user.gettopartists":
            images = await self._get_artists_images(await self.get_artists())
            artist_name = False
        elif self.method == "user.gettoptracks":
            images = await self._get_tracks_images(await self.get_tracks())
            artist_name = True
        elif self.method == "user.gettopalbums":
            images = await self._get_albums_images(await self.get_albums())
            artist_name = True

        w, h = Image.open(images[0]["path"]).size
        collage_width = int(col) * int(w)
        collage_height = int(row) * int(h)
        final_image = Image.new("RGB", (collage_width, collage_height))
        cursor = (0, 0)
        for image in images:
            # place image
            final_image.paste(Image.open(image["path"]), cursor)

            # add name
            if artist_name:
                await self._insert_name(
                    w,
                    h,
                    final_image,
                    image["name"],
                    image["artist"],
                    image["playcount"],
                    cursor,
                )
            else:
                await self._insert_name(
                    w,
                    h,
                    final_image,
                    image["name"],
                    None,
                    image["playcount"],
                    cursor,
                )

            # move cursor
            y = cursor[1]
            x = cursor[0] + w
            if cursor[0] >= (collage_width - w):
                y = cursor[1] + h
                x = 0
            cursor = (x, y)

        final_image.save(self.cache_path + "/lastfm-final.jpg")
        return self.cache_path
        # final_image.show()
