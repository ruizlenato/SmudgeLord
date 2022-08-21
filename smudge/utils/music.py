# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import json

from smudge.utils import http
from smudge.config import (
    SPOTIFY_BASIC,
    SPOTIFY_CLIENT_ID,
    SPOTIFY_CLIENT_SECRET,
)
from smudge.database.music import set_spot_user, get_spot_user


class SpotifyUser:
    authorize_url = "https://accounts.spotify.com/authorize"
    token_url = "https://accounts.spotify.com/api/token"

    def __init__(self, ClientID, ClientSecret):
        self.client_id = ClientID
        self.client_secret = ClientSecret
        self.redirect_url = "https://ruizlenato.github.io/Smudge/go"

    def getAuthUrl(self):
        return (
            self.authorize_url
            + "?response_type=code&client_id="
            + self.client_id
            + "&redirect_uri="
            + self.redirect_url
            + "&scope=user-read-currently-playing"
        )

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
            b = json.loads(r.content)
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
        token = json.loads((await http.post(self.token_url, data=data)).content)

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
        token = json.loads((await http.post(self.token_url, data=data)).content)

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
        token = json.loads((await http.post(self.token_url, data=data)).content)

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
    b = json.loads(
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
