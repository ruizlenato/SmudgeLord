# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)

import io
import os
import re
import json
import shutil
import asyncio
import tempfile
import datetime
import gallery_dl

from yt_dlp import YoutubeDL
from bs4 import BeautifulSoup

from pyrogram import filters
from pyrogram.helpers import ikb
from pyrogram.enums import ChatAction, ChatType
from pyrogram.types import Message, CallbackQuery, InputMediaVideo, InputMediaPhoto
from pyrogram.errors import (
    BadRequest,
    FloodWait,
    Forbidden,
    MediaEmpty,
    UserNotParticipant,
)

from ..bot import Smudge
from smudge.utils.locales import tld
from smudge.utils import http, pretty_size, aiowrap
from smudge.database.videos import sdl_c

# Regex to get link
SDL_REGEX_LINKS = r"http(?:s)?:\/\/(?:www.|mobile.|m.|vm.)?(?:instagram|twitter|reddit|tiktok|facebook).com\/(?:\S*)"

# Regex to get the video ID from the URL
YOUTUBE_REGEX = re.compile(
    r"(?m)http(?:s?):\/\/(?:www\.)?(?:music\.)?youtu(?:be\.com\/(watch\?v=|shorts/|embed/)|\.be\/|)([\w\-\_]*)(&(amp;)?‌​[\w\?‌​=]*)?"
)

# Twitter regex
TWITTER_REGEX_LINKS = (
    r"(http(s)?:\/\/(?:www\.)?(?:v\.)?(?:mobile.)?(?:twitter.com)\/(?:.*?))(?:\s|$)"
)


@aiowrap
def gallery_down(path, url: str):
    gallery_dl.config.set(("output",), "mode", "null")
    gallery_dl.config.set((), "directory", [])
    gallery_dl.config.set((), "base-directory", [path])
    gallery_dl.config.load()
    return gallery_dl.job.DownloadJob(url).run()


@aiowrap
def extract_info(instance: YoutubeDL, url: str, download=True):
    return instance.extract_info(url, download)


async def search_yt(query):
    page = await http.get(
        "https://www.youtube.com/results",
        params=dict(search_query=query, pbj="1"),
        headers={
            "x-youtube-Smudge-name": "1",
            "x-youtube-Smudge-version": "2.20200827",
        },
    )
    page = json.loads(page.content)
    list_videos = []
    for video in page[1]["response"]["contents"]["twoColumnSearchResultsRenderer"][
        "primaryContents"
    ]["sectionListRenderer"]["contents"][0]["itemSectionRenderer"]["contents"]:
        if video.get("videoRenderer"):
            dic = {
                "title": video["videoRenderer"]["title"]["runs"][0]["text"],
                "url": "https://www.youtube.com/watch?v="
                + video["videoRenderer"]["videoId"],
            }
            list_videos.append(dic)
    return list_videos


@Smudge.on_message(filters.command("yt"))
async def yt_search_cmd(c: Smudge, m: Message):
    if m.reply_to_message and m.reply_to_message.text:
        args = m.reply_to_message.text
    elif len(m.command) > 1:
        args = m.text.split(None, 1)[1]
    else:
        await m.reply_text(await tld(m, "Misc.noargs_yt"))
        return
    vids = [
        f'{num + 1}: <a href="{i["url"]}">{i["title"]}</a>'
        for num, i in enumerate(await search_yt(args))
    ]

    await m.reply_text(
        "\n".join(vids) if vids else r"¯\_(ツ)_/¯", disable_web_page_preview=True
    )


@Smudge.on_message(filters.command("ytdl"))
async def ytdlcmd(c: Smudge, m: Message):
    user = m.from_user.id

    if m.reply_to_message and m.reply_to_message.text:
        url = m.reply_to_message.text
    elif len(m.command) > 1:
        url = m.text.split(None, 1)[1]
    else:
        await m.reply_text(await tld(m, "Misc.noargs_ytdl"))
        return

    ydl = YoutubeDL({"noplaylist": True, "logger": MyLogger()})
    rege = YOUTUBE_REGEX.match(url)

    if not rege:
        yt = await extract_info(ydl, f"ytsearch:{url}", download=False)
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
            vformat = f["format_id"]

    keyboard = [
        [
            (
                await tld(m, "Misc.ytdl_audio_button"),
                f'_aud.{yt["id"]}|{afsize}|{vformat}|{user}|{m.id}',
            ),
            (
                await tld(m, "Misc.ytdl_video_button"),
                f'_vid.{yt["id"]}|{vfsize}|{vformat}|{user}|{m.id}',
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
    try:
        data, fsize, vformat, userid, mid = cq.data.split("|")
    except ValueError:
        return print(cq.data)
    if cq.from_user.id != int(userid):
        return await cq.answer(await tld(cq, "Misc.ytdl_button_denied"), cache_time=60)
    if int(fsize) > 2147483648:
        return await cq.answer(
            await tld(cq, "Misc.ytdl_file_too_big"),
            show_alert=True,
            cache_time=60,
        )
    vid = re.sub(r"^\_(vid|aud)\.", "", data)
    url = f"https://www.youtube.com/watch?v={vid}"
    await cq.message.edit(await tld(cq, "Main.downloading"))
    with tempfile.TemporaryDirectory() as tempdir:
        path = os.path.join(tempdir, "ytdl")

    if "vid" in data:
        ydl = YoutubeDL(
            {
                "outtmpl": f"{path}/%(title)s-%(id)s.%(ext)s",
                "format": f"{vformat}+140",
                "max_filesize": 500000000,
                "noplaylist": True,
                "logger": MyLogger(),
            }
        )

    else:
        ydl = YoutubeDL(
            {
                "outtmpl": f"{path}/%(title)s-%(id)s.%(ext)s",
                "format": "bestaudio[ext=m4a]",
                "max_filesize": 500000000,
                "noplaylist": True,
                "logger": MyLogger(),
            }
        )

    try:
        yt = await extract_info(ydl, url, download=True)
    except BaseException as e:
        await c.send_logs(cq, e)
        await cq.message.edit((await tld(cq, "Misc.ytdl_send_error")).format(e))
        return
    await cq.message.edit(await tld(cq, "Main.sending"))
    await c.send_chat_action(cq.message.chat.id, ChatAction.UPLOAD_VIDEO)

    filename = ydl.prepare_filename(yt)
    thumb = io.BytesIO((await http.get(yt["thumbnail"])).content)
    thumb.name = "thumbnail.png"
    caption = f"<a href='{yt['webpage_url']}'>{yt['title']}</a></b>"
    if "vid" in data:
        try:
            await c.send_video(
                cq.message.chat.id,
                video=filename,
                width=1920,
                height=1080,
                caption=caption,
                duration=yt["duration"],
                thumb=thumb,
                reply_to_message_id=int(mid),
            )
            await cq.message.delete()
        except BadRequest as e:
            await c.send_logs(cq, e)
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
                caption=caption,
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


@Smudge.on_message(
    filters.command(["sdl", "mdl"]) | filters.regex(SDL_REGEX_LINKS), group=1
)
async def sdl(c: Smudge, m: Message):
    if m.matches:
        if m.chat.type is ChatType.PRIVATE or await sdl_c("sdl_auto", m.chat.id):
            url = m.matches[0].group(0)
        else:
            return
    elif len(m.command) > 1:
        url = m.text.split(None, 1)[1]
    elif m.reply_to_message and m.reply_to_message.text:
        url = m.reply_to_message.text
    else:
        return await m.reply_text(
            (await tld(m, "Misc.noargs_sdl")).format(m.text.split(None, 1)[0])
        )

    if not re.match(SDL_REGEX_LINKS, url, re.M):
        return await m.reply_text(await tld(m, "Misc.sdl_invalid_link"))
    with tempfile.TemporaryDirectory() as tempdir:
        path = os.path.join(tempdir)

    if m.chat.type is not ChatType.PRIVATE and re.match(TWITTER_REGEX_LINKS, url, re.M):
        try:
            await m.chat.get_member(
                1703426201
            )  # To avoid conflict with @TwitterGramRobot
            return
        except UserNotParticipant:
            pass
    ydl_opts = {
        "outtmpl": f"{path}/%(extractor)s.%(ext)s",
        "wait-for-video": "1",
        "noplaylist": True,
        "logger": MyLogger(),
    }
    files = []
    caption = f"<a href='{str(url)}'>🔗 Link</a> "
    if re.match(
        r"http(?:s)?:\/\/(?:www|vm|vt)?.(?:m.)?(?:instagram|tiktok).com\/(?:\S*)",
        url,
        re.M,
    ):
        if re.search(r"instagram.com\/", url, re.M):
            bibliogram = re.sub(
                "(?:www.|m.)?instagram.com/", "bibliogram.froth.zone/", url
            )
            soup = BeautifulSoup(
                (await http.get(bibliogram, follow_redirects=True)).text,
                "html.parser",
            )
            for images in soup.find_all("img", "sized-image"):
                files += [
                    InputMediaPhoto(
                        f"https://bibliogram.froth.zone{images.get('src')}",
                        caption=caption,
                    )
                ]
            for videos in soup.find_all("video", "sized-video"):
                if not re.match(r"\S*url=undefined", videos.get("src"), re.M):
                    os.mkdir(path)
                    open(f"{path}/insta.mp4", "wb").write(
                        (
                            await http.get(
                                f"https://bibliogram.froth.zone{videos.get('src')}"
                            )
                        ).content
                    )  # Avoid "Telegram says: [400 WEBPAGE_MEDIA_EMPTY]"
        elif re.match(r"http(?:s)?://(?:vm|vt|www)\.tiktok\.com(?:\S*)", url, re.M):
            r = await http.head(url, follow_redirects=True)
            try:
                await extract_info(YoutubeDL(ydl_opts), str(r.url), download=True)
            except BaseException:
                return
        else:
            await gallery_down(path, str(url))

    try:
        files += [
            InputMediaVideo(os.path.join(path, video), caption=caption)
            for video in os.listdir(path)
            if video.endswith(".mp4")
        ]
    except FileNotFoundError:
        pass

    if m.chat.type is ChatType.PRIVATE or await sdl_c("sdl_images", m.chat.id):
        try:
            files += [
                InputMediaPhoto(os.path.join(path, photo), caption=caption)
                for photo in os.listdir(path)
                if photo.endswith((".jpg", ".png", ".jpeg"))
            ]
        except FileNotFoundError:
            pass

    if files:
        try:
            await c.send_chat_action(m.chat.id, ChatAction.UPLOAD_DOCUMENT)
            await m.reply_media_group(media=files)
        except FloodWait as e:
            await asyncio.sleep(e.value)
        except MediaEmpty:
            return
        except Forbidden:
            return shutil.rmtree(tempdir, ignore_errors=True)

    await asyncio.sleep(2)
    return shutil.rmtree(tempdir, ignore_errors=True)


class MyLogger:
    def debug(self, msg):
        # For compatibility with youtube-dl, both debug and info are passed into debug
        # You can distinguish them by the prefix '[debug] '
        if not msg.startswith("[debug] "):
            self.info(msg)

    def info(self, msg):
        pass

    def warning(self, msg):
        pass

    @staticmethod
    def error(msg):
        if "There's no video" not in msg:
            print(msg)
