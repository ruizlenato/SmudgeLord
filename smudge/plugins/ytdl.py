# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import io
import os
import re
import yt_dlp
import shutil
import tempfile
import datetime

from pyrogram.helpers import ikb
from pyrogram import filters, enums
from pyrogram.errors import BadRequest
from pyrogram.types import Message, CallbackQuery

from smudge import Smudge
from smudge.plugins import tld
from smudge.utils import send_logs, http, pretty_size, aiowrap
from smudge.database.start import check_sdl

SDL_REGEX_LINKS = r"^(http(s)?:\/\/(?:www\.)?(?:v\.)?(?:mobile.)?(?:instagram.com|twitter.com|vm.tiktok.com|tiktok.com)\/(?:.*?))(?:\s|$)"

YOUTUBE_REGEX = re.compile(
    r"(?m)http(?:s?):\/\/(?:www\.)?(?:music\.)?youtu(?:be\.com\/(watch\?v=|shorts/)|\.be\/|)([\w\-\_]*)(&(amp;)?‌​[\w\?‌​=]*)?"
)
TIME_REGEX = re.compile(r"[?&]t=([0-9]+)")

MAX_FILESIZE = 500000000


@aiowrap
def extract_info(instance: yt_dlp.YoutubeDL, url: str, download=True):
    return instance.extract_info(url, download)


@Smudge.on_message(filters.command("ytdl"))
async def ytdlcmd(c: Smudge, m: Message):
    user = m.from_user.id

    if m.reply_to_message and m.reply_to_message.text:
        url = m.reply_to_message.text
    elif len(m.command) > 1:
        url = m.text.split(None, 1)[1]
    else:
        await m.reply_text(await tld(m, "Misc.ytdl_missing_argument"))
        return

    ydl = yt_dlp.YoutubeDL({"noplaylist": True})

    rege = YOUTUBE_REGEX.match(url)

    t = TIME_REGEX.search(url)
    temp = t.group(1) if t else 0

    if not rege:
        yt = await extract_info(ydl, "ytsearch:" + url, download=False)
        try:
            yt = yt["entries"][0]
        except IndexError:
            return
    else:
        yt = await extract_info(ydl, rege.group(), download=False)

    for f in yt["formats"]:
        if f["format_id"] == "140":
            afsize = f["filesize"] or 0
        if f["ext"] == "mp4" and f["filesize"] is not None:
            vfsize = f["filesize"] or 0

    keyboard = [
        [
            (
                await tld(m, "Misc.ytdl_audio_button"),
                f'_aud.{yt["id"]}|{afsize}|{temp}|{user}|{m.id}',
            ),
            (
                await tld(m, "Misc.ytdl_video_button"),
                f'_vid.{yt["id"]}|{vfsize}|{temp}|{user}|{m.id}',
            ),
        ]
    ]

    if " - " in yt["title"]:
        performer, title = yt["title"].rsplit(" - ", 1)
    else:
        performer = yt.get("creator") or yt.get("uploader")
        title = yt["title"]

    text = f"🎧 <b>{performer}</b> - <i>{title}</i>\n"
    text += f"💾 <code>{pretty_size(afsize)}</code> (audio) / <code>{pretty_size(int(vfsize))}</code> (video)\n"
    text += f"⏳ <code>{datetime.timedelta(seconds=yt.get('duration'))}</code>"

    await m.reply_text(text, reply_markup=ikb(keyboard))


@Smudge.on_callback_query(filters.regex("^(_(vid|aud))"))
async def cli_ytdl(c: Smudge, cq: CallbackQuery):
    data, fsize, temp, userid, mid = cq.data.split("|")
    if not cq.from_user.id == int(userid):
        return await cq.answer(await tld("Misc.ytdl_button_denied"), cache_time=60)
    if int(fsize) > MAX_FILESIZE:
        return await cq.answer(
            await tld(cq, "Misc.ytdl_file_too_big"),
            show_alert=True,
            cache_time=60,
        )
    vid = re.sub(r"^\_(vid|aud)\.", "", data)
    url = "https://www.youtube.com/watch?v=" + vid
    await cq.message.edit(await tld(cq, "Misc.ytdl_downloading"))
    with tempfile.TemporaryDirectory() as tempdir:
        path = os.path.join(tempdir, "ytdl")

    ttemp = ""
    if int(temp):
        ttemp = f"⏰ {datetime.timedelta(seconds=int(temp))} | "

    if "vid" in data:
        ydl = yt_dlp.YoutubeDL(
            {
                "outtmpl": f"{path}/%(title)s-%(id)s.%(ext)s",
                "format": "best[ext=mp4]",
                "max_filesize": MAX_FILESIZE,
                "noplaylist": True,
            }
        )
    else:
        ydl = yt_dlp.YoutubeDL(
            {
                "outtmpl": f"{path}/%(title)s-%(id)s.%(ext)s",
                "format": "bestaudio[ext=m4a]",
                "max_filesize": MAX_FILESIZE,
                "noplaylist": True,
            }
        )
    try:
        yt = await extract_info(ydl, url, download=True)
    except BaseException as e:
        user_mention = cq.message.from_user.mention(cq.message.from_user.first_name)
        user_id = cq.message.from_user.id
        await send_logs(c, user_mention, user_id, e)
        await cq.message.edit((await tld(cq, "Misc.ytdl_send_error")).format(e))
        return
    await cq.message.edit(await tld(cq, "Misc.ytdl_sending"))
    await c.send_chat_action(cq.message.chat.id, enums.ChatAction.UPLOAD_VIDEO)

    filename = ydl.prepare_filename(yt)
    thumb = io.BytesIO((await http.get(yt["thumbnail"])).content)
    thumb.name = "thumbnail.png"
    if "vid" in data:
        try:
            await c.send_video(
                cq.message.chat.id,
                video=filename,
                width=1920,
                height=1080,
                caption=ttemp + yt["title"],
                duration=yt["duration"],
                thumb=thumb,
                reply_to_message_id=int(mid),
            )
            await cq.message.delete()
        except BadRequest as e:
            user_mention = cq.message.from_user.mention(cq.message.from_user.first_name)
            user_id = cq.message.from_user.id
            await send_logs(c, user_mention, user_id, e)
            await c.send_message(
                chat_id=cq.message.chat.id,
                text=(await tld(cq, "Misc.ytdl_send_error")).format(errmsg=e),
                reply_to_message_id=int(mid),
            )
    else:
        if " - " in yt["title"]:
            performer, title = yt["title"].rsplit(" - ", 1)
        else:
            performer = yt.get("creator") or yt.get("uploader")
            title = yt["title"]
        try:
            await c.send_audio(
                cq.message.chat.id,
                audio=filename,
                title=title,
                performer=performer,
                caption=ttemp[:-2],
                duration=yt["duration"],
                thumb=thumb,
                reply_to_message_id=int(mid),
            )
        except BadRequest as e:
            await cq.message.edit_text(
                await tld(cq, "ytdl_send_error").format(errmsg=e)
            )
        else:
            await cq.message.delete()

    shutil.rmtree(tempdir, ignore_errors=True)


@Smudge.on_message(filters.command(["sdl", "mdl"]), group=1)
@Smudge.on_message(filters.regex(SDL_REGEX_LINKS))
async def sdl(c: Smudge, m: Message):
    yt_dlp.utils.std_headers["User-Agent"] = "facebookexternalhit/1.1"

    try:
        if len(m.command) > 1:
            url = m.text.split(None, 1)[1]
        elif m.reply_to_message and m.reply_to_message.text:
            url = m.reply_to_message.text
    except IndexError:
        await m.reply_text(await tld(m, "Misc.sdl_missing_arguments"))
        return
    except TypeError:
        if m.chat.type == enums.ChatType.PRIVATE:
            url = m.matches[0].group(0)
            pass
        elif await check_sdl(m.chat.id) is None:
            return
        else:
            url = m.matches[0].group(0)
            pass

    link = re.match(
        SDL_REGEX_LINKS,
        url,
        re.M,
    )

    if not link:
        await m.reply_text(await tld(m, "Misc.sdl_invalid_link"))
        return

    with tempfile.TemporaryDirectory() as tempdir:
        path = os.path.join(tempdir, "ytdl")
    filename = f"{path}/%s%s.mp4" % (m.chat.id, m.id)
    ydl_opts = {
        "outtmpl": filename,
        "cookiefile": "~/instagram.com_cookies.txt",
        "extractor_retries": "3",
        "noplaylist": False,
        "logger": MyLogger(),
    }

    with yt_dlp.YoutubeDL(ydl_opts) as ydl:
        try:
            await extract_info(ydl, url, download=True)
        except BaseException as e:
            return

    with open(filename, "rb") as video:
        await c.send_chat_action(m.chat.id, enums.ChatAction.UPLOAD_VIDEO)
        await m.reply_video(video=video)
    shutil.rmtree(tempdir, ignore_errors=True)


class MyLogger:
    def debug(self, msg):
        # For compatibility with youtube-dl, both debug and info are passed into debug
        # You can distinguish them by the prefix '[debug] '
        if msg.startswith("[debug] "):
            pass
        else:
            self.info(msg)

    def info(self, msg):
        pass

    def warning(self, msg):
        pass

    @staticmethod
    def error(msg):
        if "There's no video" in msg:
            pass
        else:
            print(msg)
