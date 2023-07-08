# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import contextlib
import datetime
import io
import re

import filetype
from pyrogram import filters
from pyrogram.enums import ChatAction, ChatType
from pyrogram.errors import BadRequest
from pyrogram.helpers import ikb
from pyrogram.raw.functions import channels, messages
from pyrogram.raw.types import InputMessageID
from pyrogram.types import CallbackQuery, InputMediaPhoto, InputMediaVideo, Message
from yt_dlp import YoutubeDL

from smudge.bot import Smudge
from smudge.database.chats import get_chat_data
from smudge.database.medias import toggle_media
from smudge.database.users import get_user_data
from smudge.utils.locale import get_string, locale
from smudge.utils.medias import DownloadMedia, extract_info
from smudge.utils.utils import http, pretty_size

# Regex to get link
DL_REGEX = r"(?:htt.+?//)?(?:.+?)?(?:instagram|twitter|tiktok|threads).(com|net)\/(?:\S*)"

# Regex to get the video ID from the URL
YOUTUBE_REGEX = re.compile(
    r"(?m)http(?:s?):\/\/(?:www\.)?(?:music\.)?youtu(?:be\.com\/(watch\?v=|shorts/|embed/)|\.be\/|)([\w\-\_]*)(&(amp;)?‌​[\w\?‌​=]*)?"
)


@Smudge.on_message(filters.command("ytdl"))
@locale("medias")
async def ytdlcmd(client: Smudge, message: Message, strings):
    user = message.from_user.id

    if message.reply_to_message and message.reply_to_message.text:
        url = message.reply_to_message.text
    elif len(message.command) > 1:
        url = message.text.split(None, 1)[1]
    else:
        await message.reply_text(strings["ytdl-no-args"])
        return

    ydl = YoutubeDL({"noplaylist": True})
    if rege := YOUTUBE_REGEX.match(url):
        yt = await extract_info(ydl, rege.group(), download=False)

    else:
        yt = await extract_info(ydl, f"ytsearch:{url}", download=False)
        try:
            yt = yt["entries"][0]
        except IndexError:
            return
    for f in yt["formats"]:
        if f["format_id"] == "140":
            afsize = f["filesize"] or 0
        if f["ext"] == "mp4" and f["filesize"] is not None:
            vfsize = f["filesize"] or 0
            vformat = f["format_id"]
    keyboard = [
        [
            (
                strings["audio-button"],
                f'_aud.{yt["id"]}|{afsize}|{vformat}|{user}|{message.id}',
            ),
            (
                strings["video-button"],
                f'_vid.{yt["id"]}|{vfsize}|{vformat}|{user}|{message.id}',
            ),
        ]
    ]

    if " - " in yt["title"]:
        performer, title = yt["title"].rsplit(" - ", 1)
    else:
        performer = yt.get("creator") or yt.get("uploader")
        title = yt["title"]

    text = f"🎧 <b>{performer}</b> - <i>{title}</i>\n"
    text += f"💾 <code>{pretty_size(afsize)}</code> (audio) / "
    text += f"<code>{pretty_size(int(vfsize))}</code> (video)\n"
    text += f"⏳ <code>{datetime.timedelta(seconds=yt.get('duration'))}</code>"

    await message.reply_text(text, reply_markup=ikb(keyboard))


@Smudge.on_callback_query(filters.regex("^(_(vid|aud))"))
@locale("media")
async def cli_ytdl(client: Smudge, callback: CallbackQuery, strings):
    try:
        data, fsize, vformat, userid, mid = callback.data.split("|")
    except ValueError:
        return print(callback.data)
    if callback.from_user.id != int(userid):
        return await callback.answer(strings["button-answer"], cache_time=60)
    if int(fsize) > 2147483648:
        return await callback.answer(
            strings["big-file"],
            show_alert=True,
            cache_time=60,
        )

    vid = re.sub(r"^\_(vid|aud)\.", "", data)
    url = f"https://www.youtube.com/watch?v={vid}"
    await callback.message.edit(strings["downloading"])

    try:  # Downloader
        file = io.BytesIO()
        with contextlib.redirect_stdout(file), YoutubeDL({"outtmpl": "-"}) as ydl:
            format = f"{vformat}+140" if "vid" in data else "ba[ext=m4a]"
            ydl.params.update({"format": format, "noplaylist": True})
            yt = await extract_info(ydl, url, download=True)
        file.name = yt["title"]
    except BaseException as e:
        return await callback.message.edit_text(strings["sending-error"].format(errmsg=e))
    await callback.message.edit(strings["sending"])
    await client.send_chat_action(callback.message.chat.id, ChatAction.UPLOAD_VIDEO)

    thumb = io.BytesIO((await http.get(yt["thumbnail"])).content)
    thumb.name = "thumbnail.png"
    caption = f"<a href='{yt['webpage_url']}'>{yt['title']}</a></b>"

    try:
        if "vid" in data:
            await client.send_video(
                callback.message.chat.id,
                video=file,
                width=1920,
                height=1080,
                caption=caption,
                duration=yt["duration"],
                thumb=thumb,
                reply_to_message_id=int(mid),
            )
        else:
            if " - " in yt["title"]:
                performer, title = yt["title"].rsplit(" - ", 1)
            else:
                performer = yt.get("creator") or yt.get("uploader")
                title = yt["title"]
                await client.send_audio(
                    callback.message.chat.id,
                    audio=file,
                    title=title,
                    performer=performer,
                    caption=caption,
                    duration=yt["duration"],
                    thumb=thumb,
                    reply_to_message_id=int(mid),
                )
    except BadRequest as e:
        await callback.message.edit_text(strings["sending-error"].format(errmsg=e))
    await callback.message.delete()
    return None


@Smudge.on_message(filters.command(["dl", "sdl"]) | filters.regex(DL_REGEX), group=1)
@locale("media")
async def medias_download(client: Smudge, message: Message, strings):
    if message.matches:
        if (
            message.chat.type is ChatType.PRIVATE
            or (await get_chat_data(message.chat.id))["medias_adownloads"]
        ):
            url = message.matches[0].group(0)
        else:
            return None
    elif not message.matches and len(message.command) > 1:
        url = message.text.split(None, 1)[1]
        if not re.match(DL_REGEX, url, re.M):
            return await message.reply_text(strings["unsupported-link"])
    elif message.reply_to_message and message.reply_to_message.text:
        url = message.reply_to_message.text
    else:
        return await message.reply_text(strings["sdl-no-args"])

    if message.chat.type == ChatType.PRIVATE:
        captions = (await get_user_data(message.chat.id))["medias_captions"]
        method = messages.GetMessages(id=[InputMessageID(id=(message.id))])
    else:
        captions = (await get_chat_data(message.chat.id))["medias_captions"]
        method = channels.GetMessages(
            channel=await client.resolve_peer(message.chat.id),
            id=[InputMessageID(id=(message.id))],
        )

    rawM = (await client.invoke(method)).messages[0].media
    files, caption = await DownloadMedia().download(url, captions)

    medias = []
    for media in files:
        if filetype.is_video(media["p"]) and len(files) == 1:
            await client.send_chat_action(message.chat.id, ChatAction.UPLOAD_VIDEO)
            return await message.reply_video(
                video=media["p"],
                width=media["h"],
                height=media["h"],
                caption=caption,
            )

        if filetype.is_video(media["p"]):
            if medias:
                medias.append(InputMediaVideo(media["p"], width=media["w"], height=media["h"]))
            else:
                medias.append(
                    InputMediaVideo(
                        media["p"],
                        width=media["w"],
                        height=media["h"],
                        caption=caption,
                    )
                )
        elif not medias:
            medias.append(InputMediaPhoto(media["p"], caption=caption))
        else:
            medias.append(InputMediaPhoto(media["p"]))

    if medias:
        if (
            rawM
            and not re.search(r"(instagram.com/|threads.net)", url)
            and len(medias) == 1
            and "InputMediaPhoto" in str(medias[0])
        ):
            return None

        await client.send_chat_action(message.chat.id, ChatAction.UPLOAD_DOCUMENT)
        await message.reply_media_group(media=medias)
        return None
    return None


@Smudge.on_callback_query(filters.regex(r"^media_config"))
@locale("config")
async def media_config(client: Smudge, callback: CallbackQuery, strings):
    if not await filters.admin(client, callback):
        return await callback.answer(
            await get_string(callback, "config", "no-admin"), show_alert=True, cache_time=60
        )

    state = ["☑️", "✅"]
    chat = callback.message.chat
    mode = get_user_data if chat.type == ChatType.PRIVATE else get_chat_data

    if "+" in callback.data and not (await mode(chat.id))["medias_captions"]:
        await toggle_media(callback, "medias_captions", True)
    elif "+" in callback.data and (await mode(chat.id))["medias_captions"]:
        await toggle_media(callback, "medias_captions", False)

    keyboard = [
        [
            (strings["medias-captions-button"], "media_config"),
            (state[(await mode(chat.id))["medias_captions"]], "media_config+"),
        ],
    ]

    if chat.type != ChatType.PRIVATE:
        if "-" in callback.data and not (await mode(chat.id))["medias_adownloads"]:
            await toggle_media(callback, "medias_adownloads", True)
        elif "-" in callback.data and (await mode(chat.id))["medias_adownloads"]:
            await toggle_media(callback, "medias_adownloads", False)

        keyboard += [
            [
                (strings["medias-auto-button"], "media_config"),
                (state[(await mode(chat.id))["medias_adownloads"]], "media_config-"),
            ]
        ]

    keyboard += [[(await get_string(callback, "start", "back-button"), "config")]]
    return await callback.edit_message_text(
        strings["medias-config-text"], reply_markup=ikb(keyboard)
    )


__help__ = True
