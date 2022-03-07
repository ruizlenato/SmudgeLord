# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import io
import os
import re
import html
import httpx
import shutil
import yt_dlp
import tempfile
import datetime
import rapidjson
import dicioinformal

from typing import Union

from gpytranslate import Translator

from tortoise.exceptions import DoesNotExist

from smudge.utils import send_logs
from smudge.plugins import tld
from smudge.database.core import groups
from smudge.utils import http, pretty_size, aiowrap

from pyrogram.types import Message, CallbackQuery
from pyrogram.errors import BadRequest
from pyrogram import Client, filters
from pyrogram.helpers import ikb


@aiowrap
def extract_info(instance, url, download=True):
    return instance.extract_info(url, download)


tr = Translator()

# See https://cloud.google.com/translate/docs/languages
# fmt: off
LANGUAGES = [
    "af", "sq", "am", "ar", "hy",
    "az", "eu", "be", "bn", "bs",
    "bg", "ca", "ceb", "zh", "co",
    "hr", "cs", "da", "nl", "en",
    "eo", "et", "fi", "fr", "fy",
    "gl", "ka", "de", "el", "gu",
    "ht", "ha", "haw", "he", "iw",
    "hi", "hmn", "hu", "is", "ig",
    "id", "ga", "it", "ja", "jv",
    "kn", "kk", "km", "rw", "ko",
    "ku", "ky", "lo", "la", "lv",
    "lt", "lb", "mk", "mg", "ms",
    "ml", "mt", "mi", "mr", "mn",
    "my", "ne", "no", "ny", "or",
    "ps", "fa", "pl", "pt", "pa",
    "ro", "ru", "sm", "gd", "sr",
    "st", "sn", "sd", "si", "sk",
    "sl", "so", "es", "su", "sw",
    "sv", "tl", "tg", "ta", "tt",
    "te", "th", "tr", "tk", "uk",
    "ur", "ug", "uz", "vi", "cy",
    "xh", "yi", "yo", "zu",
]
# fmt: on


def get_tr_lang(text):
    if len(text.split()) > 0:
        lang = text.split()[0]
        if lang.split("-")[0] not in LANGUAGES:
            lang = "pt"
        if len(lang.split("-")) > 1 and lang.split("-")[1] not in LANGUAGES:
            lang = "pt"
    else:
        lang = "pt"
    return lang


@Client.on_message(filters.command(["tr", "tl"]))
async def translate(c: Client, m: Message):
    text = m.text[4:]
    lang = get_tr_lang(text)

    text = text.replace(lang, "", 1).strip() if text.startswith(lang) else text

    if not text and m.reply_to_message:
        text = m.reply_to_message.text or m.reply_to_message.caption

    if not text:
        return await m.reply_text(await tld(m, "tr_error"))
    sent = await m.reply_text(await tld(m, "tr_translating"))
    langs = {}

    if len(lang.split("-")) > 1:
        langs["sourcelang"] = lang.split("-")[0]
        langs["targetlang"] = lang.split("-")[1]
    else:
        to_lang = langs["targetlang"] = lang

    trres = await tr.translate(text, **langs)
    text = trres.text

    res = html.escape(text)
    await sent.edit_text(
        ("<b>{from_lang}</b> -> <b>{to_lang}:</b>\n<code>{translation}</code>").format(
            from_lang=trres.lang, to_lang=to_lang, translation=res
        )
    )


@Client.on_message(filters.command("dicio"))
async def dicio(c: Client, m: Message):
    txt = m.text.split(" ", 1)[1]
    a = dicioinformal.definicao(txt)["results"]
    if a:
        frase = f'<b>{a[0]["title"]}:</b>\n{a[0]["tit"]}\n\n<i>{a[0]["desc"]}</i>'
    else:
        frase = "sem resultado"
    await m.reply(frase)


@Client.on_message(filters.command("short"))
async def short(c: Client, m: Message):
    if len(m.command) < 2:
        return await m.reply_text(await tld(m, "short_error"))
    else:
        url = m.command[1]
        if not url.startswith("http"):
            url = "http://" + url
        try:
            short = m.command[2]
            shortRequest = await http.get(
                f"https://api.1pt.co/addURL?long={url}&short={short}"
            )
            info = rapidjson.loads(shortRequest.content)
            short = info["short"]
            return await m.reply_text(f"<code>https://1pt.co/{short}</code>")
        except IndexError:
            shortRequest = await http.get(f"https://api.1pt.co/addURL?long={url}")
            info = rapidjson.loads(shortRequest.content)
            short = info["short"]
            return await m.reply_text(f"<code>https://1pt.co/{short}</code>")
        except Exception as e:
            return await m.reply_text(f"<b>{e}</b>")


@Client.on_message(filters.command(["print", "ss"]))
async def prints(c: Client, m: Message):
    msg = m.text
    the_url = msg.split(" ", 1)
    wrong = False

    if len(the_url) == 1:
        wrong = True
    else:
        the_url = the_url[1]

    if wrong:
        await m.reply_text(await tld(m, "print_error"))
        return

    try:
        sent = await m.reply_text(await tld(m, "print_printing"))
        res_json = await cssworker_url(target_url=the_url)
    except BaseException as e:
        user_mention = m.from_user.mention(m.from_user.first_name)
        user_id = m.from_user.id
        await send_logs(c, user_mention, user_id, e)
        await m.reply(f"<b>Failed due to:</b> <code>{e}</code>")
        return

    if res_json:
        # {"url":"image_url","response_time":"147ms"}
        image_url = res_json["url"]
        if image_url:
            try:
                await m.reply_photo(image_url)
                await sent.delete()
            except BaseException as e:
                user_mention = m.from_user.mention(m.from_user.first_name)
                user_id = m.from_user.id
                await send_logs(c, user_mention, user_id, e)
                return
        else:
            await m.reply(
                "couldn't get url value, most probably API is not accessible."
            )
    else:
        await m.reply(await tld(m, "print_api_dead"))


async def cssworker_url(target_url: str):
    url = "https://htmlcsstoimage.com/demo_run"
    my_headers = {
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.2564.97 Safari/537.36",
        "Accept": "*/*",
        "Accept-Language": "en-US,en;q=0.5",
        "Referer": "https://htmlcsstoimage.com/",
        "Content-Type": "application/json",
        "Origin": "https://htmlcsstoimage.com",
        "Alt-Used": "htmlcsstoimage.com",
        "Connection": "keep-alive",
    }

    data = {
        "html": "",
        "console_mode": "",
        "url": target_url,
        "css": "",
        "selector": "",
        "ms_delay": "",
        "render_when_ready": "false",
        "viewport_height": "900",
        "viewport_width": "1600",
        "google_fonts": "",
        "device_scale": "",
    }

    try:
        resp = await http.post(url, headers=my_headers, json=data)
        return resp.json()
    except httpx.NetworkError:
        return None


async def search_yt(query):
    page = (
        await http.get(
            "https://www.youtube.com/results",
            params=dict(search_query=query, pbj="1"),
            headers={
                "x-youtube-client-name": "1",
                "x-youtube-client-version": "2.20200827",
            },
        )
    ).json()
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


@Client.on_message(filters.command("yt"))
async def yt_search_cmd(c: Client, m: Message):
    vids = [
        '{}: <a href="{}">{}</a>'.format(num + 1, i["url"], i["title"])
        for num, i in enumerate(await search_yt(m.text.split(None, 1)[1]))
    ]
    await m.reply_text(
        "\n".join(vids) if vids else r"¯\_(ツ)_/¯", disable_web_page_preview=True
    )


@Client.on_message(filters.command("ytdl"))
async def ytdlcmd(c: Client, m: Message):
    user = m.from_user.id

    if m.reply_to_message and m.reply_to_message.text:
        url = m.reply_to_message.text
    elif len(m.command) > 1:
        url = m.text.split(None, 1)[1]
    else:
        await m.reply_text(await tld(m, "ytdl_missing_argument"))
        return

    ydl = yt_dlp.YoutubeDL(
        {"outtmpl": "dls/%(title)s-%(id)s.%(ext)s", "format": "mp4", "noplaylist": True}
    )
    rege = re.match(
        r"http(?:s?):\/\/(?:www\.)?youtu(?:be\.com\/watch\?v=|\.be\/)([\w\-\_]*)(&(amp;)?‌​[\w\?‌​=]*)?",
        url,
        re.M,
    )

    if "t=" in url:
        temp = url.split("t=")[1].split("&")[0]
    else:
        temp = 0

    if not rege:
        yt = await extract_info(ydl, "ytsearch:" + url, download=False)
        yt = yt["entries"][0]
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
                await tld(m, "ytdl_audio_button"),
                f'_aud.{yt["id"]}|{afsize}|{temp}|{vformat}|{user}|{m.message_id}',
            ),
            (
                await tld(m, "ytdl_video_button"),
                f'_vid.{yt["id"]}|{vfsize}|{temp}|{vformat}|{user}|{m.message_id}',
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


@Client.on_callback_query(filters.regex("^(_(vid|aud))"))
async def cli_ytdl(c: Client, cq: CallbackQuery):
    data, fsize, temp, vformat, userid, mid = cq.data.split("|")
    if not cq.from_user.id == int(userid):
        return await cq.answer("ytdl_button_denied", cache_time=60)
    if int(fsize) > 500000000:
        return await cq.answer(
            await tld(cq, "ytdl_file_too_big"),
            show_alert=True,
            cache_time=60,
        )
    vid = re.sub(r"^\_(vid|aud)\.", "", data)
    url = "https://www.youtube.com/watch?v=" + vid
    await cq.message.edit(await tld(cq, "ytdl_downloading"))
    with tempfile.TemporaryDirectory() as tempdir:
        path = os.path.join(tempdir, "ytdl")

    ttemp = ""
    if int(temp):
        ttemp = f"⏰ {datetime.timedelta(seconds=int(temp))} | "

    if "vid" in data:
        ydl = yt_dlp.YoutubeDL(
            {
                "outtmpl": f"{path}/%(title)s-%(id)s.%(ext)s",
                "format": f"{vformat}+140",
                "noplaylist": True,
            }
        )
    else:
        ydl = yt_dlp.YoutubeDL(
            {
                "outtmpl": f"{path}/%(title)s-%(id)s.%(ext)s",
                "format": "140",
                "extractaudio": True,
                "noplaylist": True,
            }
        )
    try:
        yt = await extract_info(ydl, url, download=True)
    except BaseException as e:
        user_mention = cq.message.from_user.mention(cq.message.from_user.first_name)
        user_id = cq.message.from_user.id
        await send_logs(c, user_mention, user_id, e)
        await cq.message.edit((await tld(cq, "ytdl_send_error")).format(e))
        return
    await cq.message.edit(await tld(cq, "ytdl_sending"))
    await c.send_chat_action(cq.message.chat.id, "upload_video")

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
        except BadRequest as e:
            user_mention = cq.message.from_user.mention(cq.message.from_user.first_name)
            user_id = cq.message.from_user.id
            await send_logs(c, user_mention, user_id, e)
            await c.send_message(
                chat_id=cq.message.chat.id,
                text=(await tld(cq, "ytdl_send_error")).format(errmsg=e),
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
            user_mention = cq.message.from_user.mention(cq.message.from_user.first_name)
            user_id = cq.message.from_user.id
            await send_logs(c, user_mention, user_id, e)
            await c.send_message(
                chat_id=cq.message.chat.id,
                text=("ytdl_send_error{}").format(errmsg=e),
                reply_to_message_id=int(mid),
            )
    await cq.message.delete()

    shutil.rmtree(tempdir, ignore_errors=True)


async def sdl_autodownload(chat_id: int):
    try:
        return (await groups.get(id=chat_id)).sdl_autodownload
    except DoesNotExist:
        return None


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

    def error(self, msg):
        if "There's no video" in msg:
            pass
        else:
            print(msg)


REGEX_LINKS = r"^(http(s)?:\/\/(?:www\.)?(?:v\.)?(?:mobile.)?(?:instagram.com|twitter.com|vm.tiktok.com|tiktok.com)\/(?:.*?))(?:\s|$)"


@Client.on_message(filters.command(["sdl", "mdl"]), group=1)
@Client.on_message(filters.regex(REGEX_LINKS))
async def sdl(c: Client, m: Message):
    yt_dlp.utils.std_headers["User-Agent"] = "facebookexternalhit/1.1"

    try:
        if len(m.command) > 1:
            url = m.text.split(None, 1)[1]
        elif m.reply_to_message and m.reply_to_message.text:
            url = m.reply_to_message.text
    except IndexError:
        await m.reply_text(await tld(m, "sdl_missing_arguments"))
        return
    except TypeError:
        if await sdl_autodownload(m.chat.id) == "Off":
            return
        else:
            url = m.matches[0].group(0)
            pass

    link = re.match(
        REGEX_LINKS,
        url,
        re.M,
    )

    if not link:
        await m.reply_text(await tld(m, "sdl_invalid_link"))
        return

    print(url)

    with tempfile.TemporaryDirectory() as tempdir:
        path = os.path.join(tempdir, "ytdl")
    filename = f"{path}/%s%s.mp4" % (m.chat.id, m.message_id)
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
        await c.send_chat_action(m.chat.id, "upload_video")
        await m.reply_video(video=video)
    shutil.rmtree(tempdir, ignore_errors=True)


@Client.on_message(filters.command(["cep"], prefixes="/"))
async def lastfm(c: Client, m: Message):
    try:
        if len(m.command) > 1:
            cep = m.text.split(None, 1)[1]
        elif m.reply_to_message and m.reply_to_message.text:
            cep = m.reply_to_message.text
    except IndexError:
        await m.reply_text(await tld(m, "no_cep"))
        return

    base_url = "https://brasilapi.com.br/api/cep/v1"
    res = await http.get(f"{base_url}/{cep}")
    city = res.json().get("city")
    state = res.json().get("state")
    states = await http.get(f"https://brasilapi.com.br/api/ibge/uf/v1/{state}")
    state_name = states.json().get("nome")
    neighborhood = res.json().get("neighborhood")
    street = res.json().get("street")

    if res.status_code == 404:
        await m.reply_text((await tld(m, "cep_error")))
        return
    else:
        rep = (await tld(m, "cep_strings")).format(
            cep, city, state_name, state, neighborhood, street
        )
        await m.reply_text(rep)


@Client.on_message(filters.command(["ddd"], prefixes="/"))
@Client.on_callback_query(filters.regex("ddd_(?P<num>.+)"))
async def ddd(c: Client, m: Union[Message, CallbackQuery]):
    try:
        if isinstance(m, CallbackQuery):
            ddd = m.matches[0]["num"]
        else:
            ddd = m.text.split(maxsplit=1)[1]
    except IndexError:
        await m.reply_text(await tld(m, "no_ddd"))
        return

    base_url = "https://brasilapi.com.br/api/ddd/v1"
    res = await http.get(f"{base_url}/{ddd}")
    state = res.json().get("state")
    states = await http.get(f"https://brasilapi.com.br/api/ibge/uf/v1/{state}")
    state_name = states.json().get("nome")
    cities = res.json().get("cities")
    if isinstance(m, CallbackQuery):
        cities.reverse()
        cities = (
            str(cities)
            .replace("'", "")
            .replace("]", "")
            .replace("[", "")
            .lower()
            .title()
        )
        await m.edit_message_text(
            (await tld(m, "fddd_strings")).format(ddd, state_name, state, cities)
        )
    else:
        rep = (await tld(m, "ddd_strings")).format(ddd, state_name, state)
        keyboard = [[(await tld(m, "ddd_cities"), f"ddd_{ddd}")]]
        await m.reply_text(rep, reply_markup=ikb(keyboard))


plugin_name = "misc_name"
plugin_help = "misc_help"
