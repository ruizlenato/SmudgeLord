# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import os
import re
import httpx
import shutil
import base64
import asyncio
import urllib.parse
import urllib.request
import rapidjson as json

from smudge.config import LASTFM_API_KEY
from smudge.plugins import tld
from smudge.database.core import users
from smudge.utils import http

from pyrogram.helpers import ikb
from pyrogram import Client, filters
from pyrogram.errors import UserNotParticipant
from pyrogram.types import Message, CallbackQuery

from tortoise.exceptions import IntegrityError, DoesNotExist


async def set_last_user(user_id: int, lastfm_username: str):
    await users.update_or_create(id=user_id)
    await users.filter(id=user_id).update(lastfm_username=lastfm_username)
    return


async def get_last_user(user_id: int):
    try:
        return (await users.get(id=user_id)).lastfm_username
    except DoesNotExist:
        return None


async def del_last_user(chat_id: int, lastfm_username: str):
    try:
        return await users.filter(id=chat_id, lastfm_username=lastfm_username).delete()
    except DoesNotExist:
        return False


@Client.on_message(filters.command(["clearuser", "deluser"]))
async def clear(c: Client, m: Message):
    user_id = m.from_user.id
    username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m, "LastFM.no_username_to_clear"))
        return
    else:
        await del_last_user(user_id, username)
        await m.reply_text(
            (await tld(m, "LastFM.username_clear")), disable_web_page_preview=True
        )
        return


@Client.on_message(filters.command(["setuser", "setlast"]))
async def setuser(c: Client, m: Message):
    user_id = m.from_user.id
    if m.reply_to_message and m.reply_to_message.text:
        username = m.reply_to_message.text
    elif len(m.command) > 1:
        username = m.text.split(None, 1)[1]
    else:
        await m.reply_text(
            (await tld(m, "LastFM.no_username_save")).format(m.text.split(None, 1)[0])
        )
        return

    if username:
        base_url = "http://ws.audioscrobbler.com/2.0"
        res = await http.get(
            f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
        )
        if not res.status_code == 200:
            await m.reply_text((await tld(m, "LastFM.username_wrong")))
            return
        else:
            await set_last_user(user_id, username)
            await m.reply_text((await tld(m, "LastFM.username_save")))
    else:
        rep = "Você esquceu do username"
        await m.reply_text(rep)
    return


@Client.on_message(filters.command(["lastfm", "lmu", "lt"], prefixes="/"))
async def lastfm(c: Client, m: Message):
    if m.chat.type == "private":
        pass
    else:
        if m.text.split(maxsplit=1)[0] == "/lt":
            try:
                await m.chat.get_member(1993314727)
                return
            except UserNotParticipant:
                pass
        else:
            pass
    user = m.from_user.first_name
    user_id = m.from_user.id
    username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m, "LastFM.no_username"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = await http.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    if not res.status_code == 200:
        await m.reply_text((await tld(m, "LastFM.username_wrong")))
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
    try:
        info = json.loads(fetch.content)
        last_user = info["track"]
        if int(last_user.get("userplaycount")) == 0:
            scrobbles = int(last_user.get("userplaycount")) + 1
        else:
            scrobbles = int(last_user.get("userplaycount"))
    except KeyError:
        scrobbles = "none"

    if image:
        rep = f"<a href='{image}'>\u200c</a>"

    if first_track.get("@attr"):
        if scrobbles == "none":
            rep += (await tld(m, "LastFM.scrobble_none_is")).format(username, user)
        else:
            rep += (await tld(m, "LastFM.scrobble_is")).format(
                username, user, scrobbles
            )
    else:
        if scrobbles == "none":
            rep += (await tld(m, "LastFM.scrobble_none_was")).format(username, user)
        else:
            rep += (await tld(m, "LastFM.scrobble_was")).format(
                username, user, scrobbles
            )

    if not loved:
        rep += f"<b>{artist}</b> - {song}"
    else:
        rep += f"<b>{artist}</b> - {song}❤️"

    await m.reply_text(rep)


@Client.on_message(filters.command(["lalbum", "lalb", "album"], prefixes="/"))
async def album(c: Client, m: Message):
    if m.chat.type == "private":
        pass
    else:
        if m.text.split(maxsplit=1)[0] == "/lt":
            try:
                await m.chat.get_member(1993314727)
                return
            except UserNotParticipant:
                pass
        else:
            pass
    user = m.from_user.first_name
    username = await get_last_user(m.from_user.id)

    if not username:
        await m.reply_text(await tld(m, "LastFM.no_username"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = await http.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    if not res.status_code == 200:
        await m.reply_text((await tld(m, "LastFM.username_wrong")))
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
        f"{base_url}?method=album.getinfo&api_key={LASTFM_API_KEY}&artist={artist1}&album={album1}&user={username}&format=json"
    )
    info = json.loads(fetch.content)
    last_user = info["album"]
    if int(last_user.get("userplaycount")) == 0:
        scrobbles = int(last_user.get("userplaycount")) + 1
    else:
        scrobbles = int(last_user.get("userplaycount"))

    if image:
        rep = f"<a href='{image}'>\u200c</a>"

    if first_track.get("@attr"):
        if scrobbles == "none":
            rep += (await tld(m, "LastFM.scrobble_none_is")).format(username, user)
        else:
            rep += (await tld(m, "LastFM.scrobble_is")).format(
                username, user, scrobbles
            )
    else:
        if scrobbles == "none":
            rep += (await tld(m, "LastFM.scrobble_none_was")).format(username, user)
        else:
            rep += (await tld(m, "LastFM.scrobble_was")).format(
                username, user, scrobbles
            )

    if not loved:
        rep += f"🎙 <strong>{artist}</strong>\n📀 {album}"
    else:
        rep += f"🎙 <strong>{artist}</strong>\n📀 {album} ❤️"

    await m.reply(rep)


@Client.on_message(filters.command(["lartist", "lart", "artist"], prefixes="/"))
async def artist(c: Client, m: Message):
    if m.chat.type == "private":
        pass
    else:
        if m.text.split(maxsplit=1)[0] == "/lt":
            try:
                await m.chat.get_member(1993314727)
                return
            except UserNotParticipant:
                pass
        else:
            pass
    user = m.from_user.first_name
    username = await get_last_user(m.from_user.id)

    if not username:
        await m.reply_text(await tld(m, "LastFM.no_username"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = await http.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    if not res.status_code == 200:
        await m.reply_text((await tld(m, "LastFM.username_wrong")))
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

    if image:
        rep = f"<a href='{image}'>\u200c</a>"

    if first_track.get("@attr"):
        if scrobbles == "none":
            rep += (await tld(m, "LastFM.scrobble_none_is")).format(username, user)
        else:
            rep += (await tld(m, "LastFM.scrobble_is")).format(
                username, user, scrobbles
            )
    else:
        if scrobbles == "none":
            rep += (await tld(m, "LastFM.scrobble_none_was")).format(username, user)
        else:
            rep += (await tld(m, "LastFM.scrobble_was")).format(
                username, user, scrobbles
            )

    if not loved:
        rep += f"🎙 <strong>{artist}</strong>"
    else:
        rep += f"🎙 <strong>{artist}</strong> ❤️"

    await m.reply(rep)


@Client.on_message(filters.command(["duotone"], prefixes="/"))
async def duotone(c: Client, m: Message):
    user_id = m.from_user.id
    username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m, "LastFM.no_username"))
        return

    if len(m.command) > 1:
        args = m.text.split(None, 1)[1]
        if re.search("[A-a]rt", args):
            top = "artists"
        elif re.search("[A-a]lb", args):
            top = "albums"
        elif re.search("[M-m]us|[T-t]ra", args):
            top = "tracks"
        else:
            return
    else:
        top = "albums"

    try:
        args = args.lower()
        x = re.search(r"(\d+d)", args)
        y = re.search(r"(\d+m|\d+y)", args)
        z = re.search(r"(overall)", args)
        if x:
            uwu = str(x.group(1)).replace("30d", "1m").replace(" ", "")
            if uwu in "1m":
                period = f"{uwu}ounth"
            else:
                period = f"{uwu}ay"
            if uwu not in ["1m", "7d", "9d", "3d"]:
                period = f"1month"
        elif y:
            uwu = str(y.group(1)).replace("1y", "12m")
            period = f"{uwu}onth"
            if uwu not in ["1y", "1m", "3m", "12m"]:
                period = f"1month"
        elif z:
            period = f"overall"
        else:
            period = "1month"
    except UnboundLocalError:
        return

    keyboard = [
        [
            (
                f"🟣+🟦",
                f"_duton.divergent|{top}|{period}|{user_id}|{username}",
            ),
            (
                f"⬛️+🔴",
                f"_duton.horror|{top}|{period}|{user_id}|{username}",
            ),
            (
                f"🟢+🟩",
                f"_duton.natural|{top}|{period}|{user_id}|{username}",
            ),
        ],
        [
            (
                f"🟨+🔴",
                f"_duton.sun|{top}|{period}|{user_id}|{username}",
            ),
            (
                f"⚫️+🟨",
                f"_duton.yellish|{top}|{period}|{user_id}|{username}",
            ),
            (
                f"🔵+🟦",
                f"_duton.sea|{top}|{period}|{user_id}|{username}",
            ),
            (
                f"🟣+🟪",
                f"_duton.purplish|{top}|{period}|{user_id}|{username}",
            ),
        ],
    ]

    await m.reply_text(
        await tld(m, "LastFM.dualtone_choose"), reply_markup=ikb(keyboard)
    )


@Client.on_callback_query(filters.regex("^(_duton)"))
async def create_duotone(c: Client, cq: CallbackQuery):
    color, top, period, user_id, username = cq.data.split("|")
    period_tld_num = re.sub("[A-z]", "", period)
    tld_string = re.sub("[0-9]", "", period)
    url = "https://generator.musicorumapp.com/generate"
    my_headers = {
        "Content-Type": "application/json",
    }
    color = re.sub(r"^\_(duton)\.", "", color)
    data = {
        "theme": "duotone",
        "options": {
            "user": username,
            "top": top,
            "pallete": color,
            "period": period,
            "names": "true",
            "playcount": True,
            "story": False,
            "messages": {
                "scrobbles": [
                    "scrobbles",
                    (await tld(cq, f"LastFM.dualtone_{tld_string}")).format(
                        period_tld_num
                    ),
                ],
                "subtitle": (await tld(cq, f"LastFM.dualtone_{tld_string}")).format(
                    period_tld_num
                ),
                "title": (await tld(cq, f"LastFM.dualtone_{top}")),
            },
        },
        "source": "web",
    }
    try:
        resp = await http.post(url, headers=my_headers, json=data)
        resp = resp.json()
        data = (
            str(resp["base64"])
            .replace(" ", "+")
            .replace("data:image/jpeg;base64,", "")
            .replace("'}", "")
            .replace("'{", "")
        )
        imgdata = base64.b64decode(data)

        filename = f"({top})%s%s.png" % (user_id, username)
        with open(filename, "wb") as f:
            f.write(imgdata)
        with open(filename, "rb") as image:
            keyboard = [[(f"👤 LastFM User", f"https://last.fm/user/{username}", "url")]]
            await c.send_photo(
                cq.message.chat.id,
                photo=filename,
                reply_markup=ikb(keyboard),
            )
            await cq.message.delete()
            await asyncio.sleep(0.2)
            os.remove(filename)
    except httpx.NetworkError:
        return None


plugin_name = "LastFM.name"
plugin_help = "LastFM.help"
