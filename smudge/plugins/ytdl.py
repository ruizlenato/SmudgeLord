import os
import asyncio

import aiohttp
import youtube_dl

from smudge.locales.strings import tld

from functools import wraps, partial
from typing import Coroutine, Callable

from pyromod.helpers import ikb
from pyrogram.types import Message
from pyrogram import Client, filters
from pyrogram.errors import MessageNotModified

loop = asyncio.get_event_loop()


def aiowrap(fn: Callable) -> Coroutine:
    @wraps(fn)
    def decorator(*args, **kwargs):
        wrapped = partial(fn, *args, **kwargs)

        return loop.run_in_executor(None, wrapped)

    return decorator


@aiowrap
def extract_info(instance, url, download=True):
    return instance.extract_info(url, download)


@Client.on_message(filters.command(["ytdl", "vdl"]))
async def ytdl(c: Client, m: Message):
    url = m.text.split(maxsplit=1)[1]
    if "-m4a" in url:
        url = url.replace(" -m4a", "")
        ydl = youtube_dl.YoutubeDL(
            {
                "outtmpl": "stuff/%(title)s-%(id)s.%(ext)s",
                "format": "140",
                "noplaylist": True,
            }
        )
        vid = False
    else:
        url = url.replace(" -mp4", "")
        ydl = youtube_dl.YoutubeDL(
            {
                "outtmpl": "stuff/%(title)s-%(id)s.%(ext)s",
                "format": "mp4",
                "noplaylist": True,
            }
        )
        vid = True
    if "http" not in url and "https" not in url:
        yt = await extract_info(ydl, "ytsearch:" + url, download=False)
        yt = yt["entries"][0]
        url = "https://www.youtube.com/watch?v=" + yt["id"]
    else:
        yt = await extract_info(ydl, url, download=False)

    formats = yt["formats"]
    format = formats[0]

    for f in yt["formats"]:
        if f["format_id"] == "140":
            fsize = f["filesize"] or 0
        if f["ext"] == "mp4" and f["filesize"] is not None:
            fsize = f["filesize"] or 0

    try:
        if int(fsize) > 209715200:
            return await m.reply_text(await tld(m.chat.id, "ytdl_max_size"))
    except:
        pass

    yt = await extract_info(ydl, url, download=True)
    a = f'Downloading <code>{yt["title"]}</code>'
    reply = await m.reply_text(a)

    a = f'Sending <code>{yt["title"]}</code>'
    await reply.edit(a)
    await c.send_chat_action(m.chat.id, "UPLOAD_VIDEO")
    filename = ydl.prepare_filename(yt)

    if vid:
        keyboard = [[("🔗 Link", yt["webpage_url"], "url")]]
        await c.send_video(
            m.chat.id,
            filename,
            width=int(1920),
            height=int(1080),
            reply_markup=ikb(keyboard),
            reply_to_message_id=m.message_id,
        )
    else:
        keyboard = [[("🔗 Link", yt["webpage_url"], "url")]]
        if " - " in yt["title"]:
            performer, title = yt["title"].rsplit(" - ", 1)
        else:
            performer = yt.get("creator") or yt.get("uploader")
            title = yt["title"]
        await c.send_audio(
            m.chat.id,
            filename,
            title=title,
            performer=performer,
            duration=yt["duration"],
            reply_markup=ikb(keyboard),
            reply_to_message_id=m.message_id,
        )

    await reply.delete()
    os.remove(filename)


plugin_name = "ytdl_name"
plugin_help = "ytdl_help"
