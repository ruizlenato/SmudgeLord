import httpx
import urllib.parse
import urllib.request
import rapidjson as json

from smudge.config import LASTFM_API_KEY
from smudge.locales.strings import tld
from smudge.database.core import users

from pyrogram import Client, filters
from pyrogram.types import Message

from tortoise.exceptions import IntegrityError

timeout = httpx.Timeout(20)
http = httpx.AsyncClient(http2=True, timeout=timeout)


async def set_last_user(user_id: int, lastfm_username: str):
    await users.filter(user_id=user_id).update(lastfm_username=lastfm_username)
    return


async def get_last_user(user_id: int):
    try:
        return (await users.get(user_id=user_id)).lastfm_username
    except DoesNotExist:
        return None


@Client.on_message(filters.command("setuser"))
async def setuser(c: Client, m: Message):
    user_id = m.from_user.id
    try:
        if m.reply_to_message and m.reply_to_message.text:
            username = m.reply_to_message.text
        elif m.text and m.text.split(maxsplit=1)[1]:
            username = m.text.split(maxsplit=1)[1]
    except IndexError:
        await m.reply_text(await tld(m.chat.id, "lastfm_no_username_save"))
        return

    if username:
        await set_last_user(user_id, username)
        await m.reply_text((await tld(m.chat.id, "lastfm_username_save")))
    else:
        rep = "Você esquceu do username"
        await m.reply_text(rep)
    return


@Client.on_message(filters.command(["lastfm", "lt", "lmu"], prefixes="/"))
async def lastfm(c: Client, m: Message):
    user = m.from_user.first_name
    user_id = m.from_user.id
    username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m.chat.id, "lastfm_no_username"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = await http.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    if not res.status_code == 200:
        await m.reply_text((await tld(m.chat.id, "lastfm_username_wrong")))
        return

    try:
        first_track = res.json().get("recenttracks").get("track")[0]
    except IndexError:
        await m.reply_text("Você não parece ter scrobblado(escutado) nenhuma música...")
        return
    image = first_track.get("image")[3].get("#text")
    artist = first_track.get("artist").get("name")
    artist1 = urllib.parse.quote(artist)
    song = first_track.get("name")
    song1 = urllib.parse.quote(song)
    loved = int(first_track.get("loved"))
    fetch = await http.get(
        f"{base_url}?method=track.getinfo&artist={artist1}&track={song1}&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    info = json.loads(fetch.content)
    last_user = info["track"]
    if int(last_user.get("userplaycount")) == 0:
        scrobbles = int(last_user.get("userplaycount")) + 1
    else:
        scrobbles = int(last_user.get("userplaycount"))

    if first_track.get("@attr"):
        rep = (await tld(m.chat.id, "lastfm_scrobble_is")).format(user, scrobbles)
    else:
        rep = (await tld(m.chat.id, "lastfm_scrobble_was")).format(user, scrobbles)
    if not loved:
        rep += f"<strong>{artist}</strong> - {song}"
    else:
        rep += f"<strong>{artist}</strong> - {song} ❤️"
    if image:
        rep += f"<a href='{image}'>\u200c</a>"

    await m.reply_text(rep)


@Client.on_message(filters.command(["album", "lalb"], prefixes="/"))
async def album(c: Client, m: Message):
    user = m.from_user.first_name
    username = await get_last_user(m.from_user.id)

    if not username:
        await m.reply_text(await tld(m.chat.id, "lastfm_no_username"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = await http.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    if not res.status_code == 200:
        await m.reply_text((await tld(m.chat.id, "lastfm_username_wrong")))
        return

    try:
        first_track = res.json().get("recenttracks").get("track")[0]
    except IndexError:
        await m.reply_text("Você não parece ter scrobblado(escutado) nenhuma música...")
        return
    image = first_track.get("image")[3].get("#text")
    artist = first_track.get("artist").get("name")
    artist1 = urllib.parse.quote(artist)
    album = first_track.get("album").get("#text")
    album1 = urllib.parse.quote(album)
    loved = int(first_track.get("loved"))
    fetch = await http.get(
        f"{base_url}?method=track.getinfo&artist={artist1}&track={album1}&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    info = json.loads(fetch.content)
    last_user = info["track"]
    if int(last_user.get("userplaycount")) == 0:
        scrobbles = int(last_user.get("userplaycount")) + 1
    else:
        scrobbles = int(last_user.get("userplaycount"))

    if first_track.get("@attr"):
        rep = await tld(m.chat.id, "lastfm_scrobble_is")
    else:
        rep = await tld(m.chat.id, "lastfm_scrobble_was")

    if not loved:
        rep += f"🎙 <strong>{artist}</strong>\n📀 {album}"
    else:
        rep += f"🎙 <strong>{artist}</strong>\n📀 {album} ❤️"
    if image:
        rep += f"<a href='{image}'>\u200c</a>"

    await m.reply(rep.format(user, scrobbles))


@Client.on_message(filters.command(["artist", "lart"], prefixes="/"))
async def artist(c: Client, m: Message):
    user = m.from_user.first_name
    username = await get_last_user(m.from_user.id)

    if not username:
        await m.reply_text(await tld(m.chat.id, "lastfm_no_username"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = await http.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    if not res.status_code == 200:
        await m.reply_text((await tld(m.chat.id, "lastfm_username_wrong")))
        return

    try:
        first_track = res.json().get("recenttracks").get("track")[0]
    except IndexError:
        await m.reply_text("Você não parece ter scrobblado(escutado) nenhuma música...")
        return
    image = first_track.get("image")[3].get("#text")
    artist = first_track.get("artist").get("name")
    artist1 = urllib.parse.quote(artist)
    loved = int(first_track.get("loved"))
    fetch = await http.get(
        f"{base_url}?method=artist.getinfo&artist={artist1}&autocorrect=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    info = json.loads(fetch.content)
    last_user = info["artist"]["stats"]
    if int(last_user.get("userplaycount")) == 0:
        scrobbles = int(last_user.get("userplaycount")) + 1
    else:
        scrobbles = int(last_user.get("userplaycount"))

    if first_track.get("@attr"):
        rep = await tld(m.chat.id, "lastfm_scrobble_is")
    else:
        rep = await tld(m.chat.id, "lastfm_scrobble_was")

    if not loved:
        rep += f"🎙 <strong>{artist}</strong>"
    else:
        rep += f"🎙 <strong>{artist}</strong> ❤️"
    if image:
        rep += f"<a href='{image}'>\u200c</a>"

    await m.reply(rep.format(user, scrobbles))


plugin_name = "lastfm_name"
plugin_help = "lastfm_help"
