import spotipy

from spotipy.client import SpotifyException

from tortoise.exceptions import DoesNotExist

from smudge.database import users
from smudge.config import SPOTIFY_BASIC
from smudge.utils import http


async def check_spotify_token(user_id):
    try:
        Token = (await users.get(id=user_id)).spot_refresh_token
        if Token == None:
            return False
        else:
            return True
    except DoesNotExist:
        return False


async def set_spot_user(user_id: int, access_token: int, refresh_token: int):
    await users.update_or_create(id=user_id)
    await users.filter(id=user_id).update(
        spot_access_token=access_token, spot_refresh_token=refresh_token
    )
    return


async def get_spot_user(user_id: int):
    try:
        return (await users.get(id=user_id)).spot_refresh_token
    except DoesNotExist:
        return None


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
        print("AAAAAAAAA")
        return a
    except SpotifyException:
        try:
            new_token = await refresh_token(user_id)
        except:
            await unreg_spot(user_id)
            return False
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


async def set_last_user(user_id: int, lastfm_username: str):
    await users.update_or_create(id=user_id)
    await users.filter(id=user_id).update(lastfm_username=lastfm_username)
    return


async def get_last_user(user_id: int):
    try:
        return (await users.get(id=user_id)).lastfm_username
    except DoesNotExist:
        return None


async def del_last_user(user_id: int, lastfm_username: str):
    try:
        await users.filter(id=user_id, lastfm_username=lastfm_username).delete()
        await users.update_or_create(id=user_id)
        return
    except DoesNotExist:
        return False


async def unreg_spot(user_id: int):
    try:
        refresh = (await users.get(id=user_id)).spot_refresh_token
        acess = (await users.get(id=user_id)).spot_access_token
        await users.filter(
            id=user_id, spot_access_token=acess, spot_refresh_token=refresh
        ).delete()
        await users.update_or_create(id=user_id)
        return
    except DoesNotExist:
        return False
