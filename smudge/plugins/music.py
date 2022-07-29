# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import os
import re
import PIL
import time
import json
import httpx
import base64
import random
import shutil
import urllib.parse

from typing import Union

from smudge.utils import http
from smudge.utils.locales import tld
from smudge.config import LASTFM_API_KEY
from smudge.database.music import (
    get_last_user,
    set_last_user,
    del_last_user,
    get_spot_user,
    unreg_spot,
)
from smudge.utils.music import Spotify, LastFMImage

from pyrogram.helpers import ikb
from pyrogram import Client, filters, enums
from pyrogram.errors import UserNotParticipant, BadRequest, MessageNotModified
from pyrogram.types import Message, CallbackQuery, InputMediaPhoto


@Client.on_message(filters.command(["spoti", "spo", "spot"]))
async def spoti(c: Client, m: Message):
    if (
        m.chat.type != enums.ChatType.PRIVATE
        and m.text.split(maxsplit=1)[0] == "/spoti"
    ):
        try:
            await m.chat.get_member(796461943)  # To avoid conflict with @lyricspybot
            return
        except UserNotParticipant:
            pass
    user_id = m.from_user.id
    user = m.from_user.first_name
    tx = m.text.split(" ", 1)
    if len(tx) == 2:
        if await get_spot_user(user_id):
            return await m.reply_text(await tld(m, "Music.spotify_already_logged"))
        get = await Spotify.getAccessToken(user_id, tx[1])
        if get:
            await m.reply_text(await tld(m, "Music.spotify_login_done"))
        else:
            await m.reply_text(await tld(m, "Music.spotify_login_failed"))
    else:
        usr = await get_spot_user(m.from_user.id)
        if not usr:
            keyboard = ikb([[("Login", Spotify.getAuthUrl(), "url")]])
            await m.reply_text(
                await tld(m, "Music.spitify_no_login"), reply_markup=keyboard
            )
        else:
            sp = await Spotify.getCurrentyPlayingSong(usr)
            if sp is False:
                return await m.reply_text(await tld(m, "Music.spotify_login_failed"))

            spotify_json = json.loads(sp.content)

            if spotify_json is None:
                return

            userlink = json.loads.loads((await Spotify.getCurrentUser(usr)).content)

            rep = f"<a href='{spotify_json['item']['album']['images'][1]['url']}'>\u200c</a>"
            if spotify_json["is_playing"] is True:
                rep += (await tld(m, "Music.spotify_np")).format(
                    userlink["external_urls"]["spotify"], user
                )
            else:
                rep += (await tld(m, "Music.spotify_was_np")).format(
                    userlink["external_urls"]["spotify"], user
                )
            rep += f"<b>{spotify_json['item']['artists'][0]['name']}</b> - {spotify_json['item']['name']}"
            return await m.reply_text(rep)


@Client.on_message(filters.command(["spotf", "spotif"]))
async def spotf(c: Client, m: Message):
    usr = await get_spot_user(m.from_user.id)
    if not usr:
        keyboard = ikb([[("Login", Spotify.getAuthUrl(), "url")]])
        return await m.reply_text(
            await tld(m, "Music.spitify_no_login"), reply_markup=keyboard
        )
    else:
        spotify_json = json.loads((await Spotify.getCurrentyPlayingSong(usr)).content)
        rep = (
            f"<a href='{spotify_json['item']['album']['images'][1]['url']}'>\u200c</a>"
        )
        rep += (await tld(m, "Music.spotf_info")).format(
            spotify_json["item"]["name"],
            spotify_json["item"]["artists"][0]["name"],
            spotify_json["item"]["album"]["release_date"],
            spotify_json["item"]["album"]["name"],
            spotify_json["item"]["album"]["album_type"],
        )
        keyboard = ikb(
            [
                [
                    (
                        "Spotify Link",
                        f"{spotify_json['item']['external_urls']['spotify']}",
                        "url",
                    )
                ]
            ]
        )
        await m.reply_text(rep, reply_markup=keyboard)


@Client.on_message(filters.command(["unreg", "unregister"]))
async def unreg(c: Client, m: Message):
    user_id = m.from_user.id
    spot_user = await get_spot_user(user_id)
    if not spot_user:
        await m.reply_text(await tld(m, "Music.spotify_noclean"))
    else:
        await unreg_spot(user_id)
        await m.reply_text(
            (await tld(m, "Music.spotify_cleaned")), disable_web_page_preview=True
        )

    return


@Client.on_message(filters.command(["clearuser", "deluser"]))
async def clear(c: Client, m: Message):
    user_id = m.from_user.id
    username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m, "Music.no_username_to_clear"))
    else:
        await del_last_user(user_id)
        await m.reply_text(
            (await tld(m, "Music.username_deleted")), disable_web_page_preview=True
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
            (await tld(m, "Music.no_username_save")).format(m.text.split(None, 1)[0])
        )
        return

    if username:
        base_url = "http://ws.audioscrobbler.com/2.0"
        res = await http.get(
            f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json"
        )
        if res.status_code != 200:
            await m.reply_text((await tld(m, "Music.username_wrong")))
            return
        else:
            await set_last_user(user_id, username)
            await m.reply_text((await tld(m, "Music.username_save")))
    else:
        rep = "Você esquceu do username"
        await m.reply_text(rep)
    return


@Client.on_message(filters.command(["lastinfo", "linfo"]))
async def lastfm_info(c: Client, m: Message):
    user = m.from_user.first_name
    user_id = m.from_user.id
    username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m, "Music.no_username"))
        return

    res = await http.get(
        "http://ws.audioscrobbler.com/2.0"
        + f"?method=user.getInfo&user={username}"
        + f"&api_key={LASTFM_API_KEY}&format=json"
    )

    db = json.loads(res.content)

    if res.status_code != 200:
        return

    luser = db["user"]
    photo = luser["image"][3]["#text"]
    rname = luser["realname"]
    pcount = luser["playcount"]
    rtime = time.strftime("%m/%Y", time.localtime(luser["registered"]["#text"]))

    rep = f"<a href='{photo}'>\u200c</a>"
    rep += (await tld(m, "Music.lastfm_info")).format(
        user, username, rname, pcount, rtime
    )
    keyboard = ikb(
        [
            [
                (
                    await tld(m, "Music.lastfm_info_btn"),
                    f"https://last.fm/user/{username}",
                    "url",
                ),
            ],
        ]
    )

    await m.reply_text(rep, reply_markup=keyboard)


@Client.on_message(filters.command(["lastfm", "lmu", "lt", "whl"]))
async def lastfm(c: Client, m: Message):
    if m.chat.type != enums.ChatType.PRIVATE and m.text.split(maxsplit=1)[0] == "/lt":
        try:
            await m.chat.get_member(
                1993314727
            )  # To avoid conflict with @MyScrobblesbot
            return
        except UserNotParticipant:
            pass

    if m.text.split(maxsplit=1)[0] == "/whl":
        try:
            user = m.reply_to_message.from_user.first_name
            user_id = m.reply_to_message.from_user.id
            username = await get_last_user(user_id)
        except AttributeError:
            return
    else:
        user = m.from_user.first_name
        user_id = m.from_user.id
        username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m, "Music.no_username"))
        return

    res = await http.get(
        "http://ws.audioscrobbler.com/2.0"
        + "?method=user.getrecenttracks&limit=3"
        + f"&extended=1&user={username}&api_key="
        + f"{LASTFM_API_KEY}&format=json"
    )

    db = json.loads(res.content)

    if res.status_code != 200:
        await m.reply_text((await tld(m, "Music.username_wrong")))
        return

    try:
        first_track = db["recenttracks"]["track"][0]
    except IndexError:
        await m.reply_text("Você não parece ter scrobblado(escutado) nenhuma música...")
        return

    image = first_track["image"][3]["#text"]
    artist = first_track["artist"]["name"]
    song = first_track["name"]
    loved = int(first_track["loved"])

    fetch = await http.get(
        "http://ws.audioscrobbler.com/2.0"
        + f"?method=track.getinfo&artist={urllib.parse.quote(artist)}"
        + f"&track={urllib.parse.quote(song)}&user={username}"
        + f"&api_key={LASTFM_API_KEY}&format=json"
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

    rep = f"<a href='{image}'>\u200c</a>"
    if first_track.get("@attr"):  # Check if track is now playing
        rep += (
            (await tld(m, "Music.scrobble_none_is")).format(username, user)
            if scrobbles == "none"
            else (await tld(m, "Music.scrobble_is")).format(username, user, scrobbles)
        )

    elif scrobbles == "none":
        rep += (await tld(m, "Music.scrobble_none_was")).format(username, user)
    else:
        rep += (await tld(m, "Music.scrobble_was")).format(username, user, scrobbles)

    rep += f"<b>{artist}</b> - {song}❤️" if loved else f"<b>{artist}</b> - {song}"
    await m.reply_text(rep)


@Client.on_message(filters.command(["lalbum", "lalb", "album"]))
async def album(c: Client, m: Message):
    if (
        m.chat.type != enums.ChatType.PRIVATE
        and m.text.split(maxsplit=1)[0] == "/album"
    ):
        try:
            await m.chat.get_member(
                1993314727
            )  # To avoid conflict with @MyScrobblesbot
            return
        except UserNotParticipant:
            pass
    user = m.from_user.first_name
    username = await get_last_user(m.from_user.id)

    if not username:
        await m.reply_text(await tld(m, "Music.no_username"))
        return

    res = await http.get(
        "http://ws.audioscrobbler.com/2.0"
        + "?method=user.getrecenttracks&limit=3"
        + f"&extended=1&user={username}&api_key="
        + f"{LASTFM_API_KEY}&format=json"
    )
    db = json.loads(res.content)

    if res.status_code != 200:
        await m.reply_text((await tld(m, "Music.username_wrong")))
        return

    try:
        first_track = db["recenttracks"]["track"][0]
    except IndexError:
        await m.reply_text("Você não parece ter scrobblado(escutado) nenhuma música...")
        return
    image = first_track["image"][3]["#text"]
    artist = first_track["artist"]["name"]
    album = first_track["album"]["#text"]
    loved = int(first_track["loved"])
    fetch = await http.get(
        "http://ws.audioscrobbler.com/2.0"
        + f"?method=album.getinfo&artist={urllib.parse.quote(artist)}"
        + f"&album={urllib.parse.quote(album)}&user={username}"
        + f"&api_key={LASTFM_API_KEY}&format=json"
    )

    info = json.loads(fetch.content)
    last_user = info["album"]
    if int(last_user.get("userplaycount")) == 0:
        scrobbles = int(last_user.get("userplaycount")) + 1
    else:
        scrobbles = int(last_user.get("userplaycount"))

    rep = f"<a href='{image}'>\u200c</a>"

    if first_track.get("@attr"):  # Check if track is now playing
        rep += (
            (await tld(m, "Music.scrobble_none_is")).format(username, user)
            if scrobbles == "none"
            else (await tld(m, "Music.scrobble_is")).format(username, user, scrobbles)
        )

    elif scrobbles == "none":
        rep += (await tld(m, "Music.scrobble_none_was")).format(username, user)
    else:
        rep += (await tld(m, "Music.scrobble_was")).format(username, user, scrobbles)

    rep += (
        f"🎙 <strong>{artist}</strong>\n📀 {album} ❤️"
        if loved
        else f"🎙 <strong>{artist}</strong>\n📀 {album}"
    )

    await m.reply(rep)


@Client.on_message(filters.command(["lartist", "lart", "artist"]))
async def artist(c: Client, m: Message):
    if (
        m.chat.type != enums.ChatType.PRIVATE
        and m.text.split(maxsplit=1)[0] == "artist"
    ):
        try:
            await m.chat.get_member(
                1993314727
            )  # To avoid conflict with @MyScrobblesbot
            return
        except UserNotParticipant:
            pass
    user = m.from_user.first_name
    username = await get_last_user(m.from_user.id)

    if not username:
        await m.reply_text(await tld(m, "Music.no_username"))
        return

    res = await http.get(
        "http://ws.audioscrobbler.com/2.0"
        + "?method=user.getrecenttracks&limit=3"
        + f"&extended=1&user={username}&api_key="
        + f"{LASTFM_API_KEY}&format=json"
    )
    db = json.loads(res.content)

    if res.status_code != 200:
        await m.reply_text((await tld(m, "Music.username_wrong")))
        return

    try:
        first_track = db["recenttracks"]["track"][0]
    except IndexError:
        await m.reply_text("Você não parece ter scrobblado(escutado) nenhuma música...")
        return
    image = first_track["image"][3]["#text"]
    artist = first_track["artist"]["name"]
    loved = int(first_track["loved"])
    fetch = await http.get(
        "http://ws.audioscrobbler.com/2.0"
        + f"?method=artist.getinfo&artist={urllib.parse.quote(artist)}"
        + f"&user={username}&api_key={LASTFM_API_KEY}&format=json"
    )
    info = json.loads(fetch.content)
    last_user = info["artist"]["stats"]
    if int(last_user.get("userplaycount")) == 0:
        scrobbles = int(last_user.get("userplaycount")) + 1
    else:
        scrobbles = int(last_user.get("userplaycount"))

    rep = f"<a href='{image}'>\u200c</a>"

    if first_track.get("@attr"):  # Check if track is now playing
        rep += (
            (await tld(m, "Music.scrobble_none_is")).format(username, user)
            if scrobbles == "none"
            else (await tld(m, "Music.scrobble_is")).format(username, user, scrobbles)
        )

    elif scrobbles == "none":
        rep += (await tld(m, "Music.scrobble_none_was")).format(username, user)
    else:
        rep += (await tld(m, "Music.scrobble_was")).format(username, user, scrobbles)

    rep += (
        f"🎙 <strong>{artist}</strong> ❤️" if loved else f"🎙 <strong>{artist}</strong>"
    )

    await m.reply(rep)


CollageLastFM = LastFMImage()


@Client.on_message(filters.command(["collage", "lastcllg"]))
@Client.on_callback_query(filters.regex("^(_(collage))"))
async def collage(c: Client, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        chat_type = m.message.chat.type
    else:
        chat_type = m.chat.type

    if not isinstance(m, CallbackQuery) and (
        enums.ChatType.PRIVATE != chat_type
        and m.text.split(maxsplit=1)[0] == "/collage"
    ):
        try:
            await m.chat.get_member(296635833)  # To avoid conflict with @lastfmrobot
            return
        except UserNotParticipant:
            pass

    if isinstance(m, CallbackQuery):
        data, colNumData, rowNumData, user_id, username, style, period = m.data.split(
            "|"
        )
        user_name = m.from_user.first_name
        if m.from_user.id != int(user_id):
            await m.answer("🚫")
            return

        if "plus" in data:
            colNum = int(colNumData) + 1 if int(colNumData) < 20 else colNumData
            rowNum = int(rowNumData) + 1 if int(rowNumData) < 20 else rowNumData
        else:
            colNum = int(colNumData) - 1 if int(colNumData) > 1 else colNumData
            rowNum = int(rowNumData) - 1 if int(rowNumData) > 1 else rowNumData
        if int(rowNum) and int(colNum) < 1:
            return m.answer("🚫")
    else:
        user_id = m.from_user.id
        user_name = m.from_user.first_name
        username = await get_last_user(user_id)
        if len(m.command) <= 1:
            return await m.reply_text(await tld(m, "Music.collage_noargs"))

        args = m.text.split(None, 1)[1]
        if re.search("[A-a]rt", args):
            style = "artists"
        elif re.search("[A-a]lb", args):
            style = "albums"
        elif re.search("[M-m]us|[T-t]ra|[S-s]ongs", args):
            style = "tracks"
        else:
            style = "artists"

        try:
            args = args.lower()
            if x := re.search(r"(\d+m|\d+y|\d+d|\d+w|overall)", args):
                uwu = (
                    str(x[1]).replace("12m", "1y").replace("30d", "1m").replace(" ", "")
                )
                if uwu in {"1m", "3m", "6m"}:
                    period = f"{uwu}onth"
                elif uwu in {"7d", "1w"} or uwu not in "1y" and uwu != "overall":
                    period = "7day"
                elif uwu in "1y":
                    period = "12month"
                else:
                    period = "overall"
            else:
                period = "1month"
        except UnboundLocalError:
            return

        try:
            args = args.lower()
            if x := re.search(r"(\d+)x(\d+)", args):
                colNum = x[1]
                rowNum = x[2]
            else:
                colNum = "3"
                rowNum = "3"
        except UnboundLocalError:
            return
    if not username:
        return await m.reply_text(await tld(m, "Music.no_username"))

    keyboard = ikb(
        [
            [
                (
                    "➕",
                    f"_collage.plus|{colNum}|{rowNum}|{user_id}|{username}|{style}|{period}",
                ),
                (
                    "➖",
                    f"_collage.minus|{colNum}|{rowNum}|{user_id}|{username}|{style}|{period}",
                ),
            ]
        ]
    )

    caption = (await tld(m, "Music.collage_caption")).format(
        username, user_name, period, colNum, rowNum, style
    )

    if isinstance(m, CallbackQuery):
        try:
            filename = await CollageLastFM.create_collage(
                username, style, period, colNum, rowNum
            )
            await m.edit_message_media(
                InputMediaPhoto(f"{filename}/lastfm-final.jpg", caption=caption),
                reply_markup=keyboard,
            )
        except (UnboundLocalError, BadRequest):
            await m.answer("🚫")
        except PIL.UnidentifiedImageError:
            await m.answer("🚫")
    else:
        filename = await CollageLastFM.create_collage(
            username, style, period, colNum, rowNum
        )
        await m.reply_photo(
            photo=f"{filename}/lastfm-final.jpg", caption=caption, reply_markup=keyboard
        )
        shutil.rmtree(filename, ignore_errors=True)


@Client.on_message(filters.command(["duotone", "dualtone"]))
async def duotone(c: Client, m: Message):
    user_id = m.from_user.id
    username = await get_last_user(user_id)

    if not username:
        await m.reply_text(await tld(m, "Music.no_username"))
        return

    if len(m.command) <= 1:
        return await m.reply_text(
            (await tld(m, "Music.dualtone_noargs")).format(
                command=m.text.split(None, 1)[0]
            )
        )

    args = m.text.split(None, 1)[1]
    if re.search("[A-a]rt", args):
        top = "artists"
    elif re.search("[A-a]lb", args):
        top = "albums"
    elif re.search("[M-m]us|[T-t]ra", args):
        top = "tracks"
    else:
        top = "albums"
    try:
        args = args.lower()
        x = re.search(r"(\d+d)", args)
        y = re.search(r"(\d+m|\d+y)", args)
        z = re.search("(overall)", args)
        if x:
            uwu = str(x[1]).replace("30d", "1m").replace(" ", "")
            if uwu in {"1m", "7d", "9d", "3d"}:
                period = f"{uwu}ounth" if uwu in "1m" else f"{uwu}ay"
            else:
                period = "1month"
        elif y:
            uwu = str(y[1]).replace("1y", "12m")
            period = "1month" if uwu not in ["1y", "1m", "3m", "12m"] else f"{uwu}onth"
        elif z:
            period = "overall"
        else:
            period = "1month"
    except UnboundLocalError:
        return
    keyboard = [
        [
            ("🟣+🟦", f"_duton.divergent|{top}|{period}|{user_id}|{username}"),
            ("⬛️+🔴", f"_duton.horror|{top}|{period}|{user_id}|{username}"),
            ("🟢+🟩", f"_duton.natural|{top}|{period}|{user_id}|{username}"),
        ],
        [
            ("🟨+🔴", f"_duton.sun|{top}|{period}|{user_id}|{username}"),
            ("⚫️+🟨", f"_duton.yellish|{top}|{period}|{user_id}|{username}"),
            ("🔵+🟦", f"_duton.sea|{top}|{period}|{user_id}|{username}"),
            ("🟣+🟪", f"_duton.purplish|{top}|{period}|{user_id}|{username}"),
        ],
    ]

    await m.reply_text(
        await tld(m, "Music.dualtone_choose"), reply_markup=ikb(keyboard)
    )


@Client.on_callback_query(filters.regex("^(_duton)"))
async def create_duotone(c: Client, cq: CallbackQuery):
    try:
        await cq.edit_message_text(await tld(cq, "Main.loading"))
    except BadRequest:
        return
    color, top, period, user_id, username = cq.data.split("|")
    if cq.from_user.id == int(user_id):
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
                        (await tld(cq, f"Music.dualtone_{tld_string}")).format(
                            period_tld_num
                        ),
                    ],
                    "subtitle": (await tld(cq, f"Music.dualtone_{tld_string}")).format(
                        period_tld_num
                    ),
                    "title": (await tld(cq, f"Music.dualtone_{top}")),
                },
            },
            "source": "web",
        }
        try:
            resp = await http.post(url, headers=my_headers, json=data)
            res = json.loads(resp.content)
            data = (
                str(res["base64"])
                .replace(" ", "+")
                .replace("data:image/jpeg;base64,", "")
                .replace("'}", "")
                .replace("'{", "")
            )
            imgdata = base64.b64decode(data)

            filename = f"({top})%s%s(%s).png" % (
                user_id,
                username,
                random.randint(0, 300),
            )
            with open(filename, "wb") as f:
                f.write(imgdata)
                keyboard = [
                    [("👤 LastFM User", f"https://last.fm/user/{username}", "url")]
                ]
                await c.send_photo(
                    cq.message.chat.id,
                    photo=filename,
                    reply_markup=ikb(keyboard),
                )
                await cq.message.delete()
                os.remove(filename)
        except httpx.NetworkError:
            return None
    else:
        await cq.answer("🚫")


__help__ = "Music"
