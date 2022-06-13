# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import uuid

import httpx
import orjson

from smudge.utils import http, aiowrap

from bs4 import BeautifulSoup

dicio_link = "https://www.dicionarioinformal.com.br/"

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


async def cssworker_url(target_url: str):
    url = "https://htmlcsstoimage.com/demo_run"
    my_headers = {
        "User-Agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:95.0) Gecko/20100101 Firefox/95.0",
    }

    data = {
        "url": target_url,
        # Sending a random CSS to make the API to generate a new screenshot.
        "css": f"random-tag: {uuid.uuid4()}",
        "render_when_ready": False,
        "viewport_width": 900,
        "viewport_height": 1600,
        "device_scale": 1,
    }

    try:
        return orjson.loads(
            (await http.post(url, headers=my_headers, json=data)).content
        )
    except httpx.NetworkError:
        return None


async def dicio_def(query):
    r = await http.get(dicio_link + query, follow_redirects=True)
    soup = BeautifulSoup(r.text, "html.parser")
    tit = soup.find_all("h3", "di-blue")
    if tit is None:
        tit = soup.find_all("h3", "di-blue-link")
    title = []
    for i in tit:
        a = i.find("a")
        if a != None:
            title.append(a.get("title"))
    if a is None:
        tit = soup.find_all("h3", "di-blue-link")
    for i in tit:
        a = i.find("a")
        if a != None:
            title.append(f'você quiz dizer: {a.get("title")}')
    ti = soup.find_all("p", "text-justify")
    tit = []
    for i in ti:
        ti = i.get_text()[17:].replace("""\n                """, "")
        tit.append(ti)
    des = soup.find_all("blockquote", "text-justify")
    des.append(" ")
    desc = []
    for i in des:
        try:
            des = (
                i.get_text()
                .replace("\n"[0], "")
                .replace("                 ", "")
                .replace("""\n                """, "")
            )
        except:
            if i == " ":
                des = ""
        desc.append(des)
    result = []
    max = 0
    for i in title:
        try:
            b = {
                "title": i.replace("\t", ""),
                "tit": tit[max].replace("\t", ""),
                "desc": desc[max].replace("\t", "").replace("                ", ""),
            }
            max += 1
            result.append(b)
        except:
            pass

    return result
