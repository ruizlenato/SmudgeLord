import spotipy

from spotipy.client import SpotifyException

from smudge.database import get_spot_user, set_spot_user
from smudge.config import SPOTIFY_BASIC
from smudge.utils import http


async def gen_spotify_token(user_id, token):
    r = await http.post(
        "https://accounts.spotify.com/api/token",
        headers=dict(Authorization=f"Basic {SPOTIFY_BASIC}"),
        data=dict(
            grant_type="authorization_code",
            code=token,
            redirect_uri="https://ruizlenato.github.io/Smudge/go",
        ),
    )
    b = r.json()
    if b.get("error"):
        return False, b["error"]
    else:
        await set_spot_user(user_id, b["access_token"], b["refresh_token"])
        return True, b["access_token"]


async def get_spoti_session(user_id):
    usr = await get_spot_user(user_id)
    a = spotipy.Spotify(auth=usr)
    try:
        a.devices()
        return a
    except SpotifyException:
        new_token = await refresh_token(user_id)
        a = spotipy.Spotify(auth=new_token)
        return a


async def refresh_token(user_id):
    usr = await get_spot_user(user_id)
    r = await http.post(
        "https://accounts.spotify.com/api/token",
        headers=dict(Authorization=f"Basic {SPOTIFY_BASIC}"),
        data=dict(grant_type="refresh_token", refresh_token=usr),
    )
    b = r.json()
    await set_spot_user(user_id, b["access_token"], usr)
    return b["access_token"]
