# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import contextlib
import datetime
import gettext
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

from ..bot import Smudge
from ..database.medias import auto_downloads, captions
from ..utils.locale import locale
from ..utils.medias import DownloadMedia, extract_info
from ..utils.utils import http, pretty_size

# Regex to get link
DL_REGEX = r"(?:htt.+?//)?(?:.+?)?(?:instagram|twitter|tiktok|facebook).com\/(?:\S*)"

# Regex to get the video ID from the URL
YOUTUBE_REGEX = re.compile(
    r"(?m)http(?:s?):\/\/(?:www\.)?(?:music\.)?youtu(?:be\.com\/(watch\?v=|shorts/|embed/)|\.be\/|)([\w\-\_]*)(&(amp;)?‌​[\w\?‌​=]*)?"
)


@Smudge.on_message(filters.command("ytdl"))
@locale()
async def ytdlcmd(client: Smudge, message: Message, _):
    user = message.from_user.id

    if message.reply_to_message and message.reply_to_message.text:
        url = message.reply_to_message.text
    elif len(message.command) > 1:
        url = message.text.split(None, 1)[1]
    else:
        await message.reply_text(
            _(
                "<b>Usage:</b> <code>/ytdl [Word or link]</code>\
\n\nSpecify a word or a link so that I can search and download a video."
            )
        )
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
                _("💿 Audio"),
                f'_aud.{yt["id"]}|{afsize}|{vformat}|{user}|{message.id}',
            ),
            (
                _("📹 Video"),
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
@locale()
async def cli_ytdl(client: Smudge, callback: CallbackQuery, _):
    try:
        data, fsize, vformat, userid, mid = callback.data.split("|")
    except ValueError:
        return print(callback.data)
    if callback.from_user.id != int(userid):
        return await callback.answer(_("This button is not for you."), cache_time=60)
    if int(fsize) > 2147483648:
        return await callback.answer(
            _(
                "The Video you want to download exceeds 2GB in size.\
\nUnable to download and upload, sorry."
            ),
            show_alert=True,
            cache_time=60,
        )

    vid = re.sub(r"^\_(vid|aud)\.", "", data)
    url = f"https://www.youtube.com/watch?v={vid}"
    await callback.message.edit(_("Downloading..."))

    try:  # Downloader
        file = io.BytesIO()
        with contextlib.redirect_stdout(file), YoutubeDL({"outtmpl": "-"}) as ydl:
            format = f"{vformat}+140" if "vid" in data else "ba[ext=m4a]"
            ydl.params.update({"format": format, "noplaylist": True})
            yt = await extract_info(ydl, url, download=True)
        file.name = yt["title"]
    except BaseException as e:
        return await callback.message.edit_text(
            _(
                "Sorry! I couldn't send the video because of an error.\
\n<b>Error:</b> <code>{}</code>"
            ).format(errmsg=e)
        )
    await callback.message.edit(_("Sending..."))
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
        await callback.message.edit_text(
            _(
                "Sorry! I couldn't send the video because of an error.\
\n<b>Error:</b> <code>{}</code>"
            ).format(errmsg=e)
        )
    await callback.message.delete()
    return None


@Smudge.on_message(filters.command(["dl", "sdl"]) | filters.regex(DL_REGEX), group=1)
@locale()
async def medias_download(client: Smudge, message: Message, _):
    if message.matches:
        if message.chat.type is ChatType.PRIVATE or await auto_downloads(message.chat.id):
            url = message.matches[0].group(0)
        else:
            return None
    elif not message.matches and len(message.command) > 1:
        url = message.text.split(None, 1)[1]
        if not re.match(DL_REGEX, url, re.M):
            return await message.reply_text(
                _(
                    "<b>System glitch someone disconnected me.</b>\nThe link you sent is invalid, \
currently I only support links from TikTok, Twitter and Instagram."
                )
            )
    elif message.reply_to_message and message.reply_to_message.text:
        url = message.reply_to_message.text
    else:
        return await message.reply_text(
            _(
                "<b>Usage:</b> <code>/dl [link]</code>\n\nSpecify a link from Instagram, TikTok \
or Twitter so I can download the video."
            )
        )

    if message.chat.type == ChatType.PRIVATE:
        method = messages.GetMessages(id=[InputMessageID(id=(message.id))])
    else:
        method = channels.GetMessages(
            channel=await client.resolve_peer(message.chat.id),
            id=[InputMessageID(id=(message.id))],
        )

    rawM = (await client.invoke(method)).messages[0].media
    files, caption = await DownloadMedia().download(url, await captions(message.chat.id))

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
            and not re.search(r"instagram.com/", url)
            and len(medias) == 1
            and "InputMediaPhoto" in str(medias[0])
        ):
            return None

        await client.send_chat_action(message.chat.id, ChatAction.UPLOAD_DOCUMENT)
        await message.reply_media_group(media=medias)
        return None
    return None


__help_name__ = gettext.gettext("Videos")
__help_text__ = gettext.gettext(
    "<b>/dl|/sdl -</b> <i>Downloads videos from <b>Instagram, TikTok and Twitter.</b></i>\n"
)
