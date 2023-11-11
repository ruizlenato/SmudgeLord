# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import asyncio
import math
import re
import uuid
from collections.abc import Callable
from functools import partial, wraps
from json import JSONDecodeError

import httpx
from pyrogram import emoji

timeout = httpx.Timeout(30, pool=None)
http = httpx.AsyncClient(http2=True, timeout=timeout)


def aiowrap(func: Callable) -> Callable:
    @wraps(func)
    async def run(*args, loop=None, executor=None, **kwargs):
        if loop is None:
            loop = asyncio.get_event_loop()
        pfunc = partial(func, *args, **kwargs)
        return await loop.run_in_executor(executor, pfunc)

    return run


def pretty_size(size_bytes):
    if size_bytes == 0:
        return "0B"
    size_name = ("B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB")
    i = int(math.floor(math.log(size_bytes, 1024)))
    p = math.pow(1024, i)
    s = round(size_bytes / p, 2)
    return f"{s} {size_name[i]}"


def get_emoji_regex():
    e_list = [
        getattr(emoji, e).encode("unicode-escape").decode("ASCII")
        for e in dir(emoji)
        if not e.startswith("_")
    ]
    # to avoid re.error excluding char that start with '*'
    e_sort = sorted([x for x in e_list if not x.startswith("*")], reverse=True)
    # Sort emojis by length to make sure multi-character emojis are
    # matched first
    pattern_ = f"({'|'.join(e_sort)})"
    return re.compile(pattern_)


async def screenshot_page(target_url: str) -> str:
    """This function is used to get a screenshot of a website using htmlcsstoimage.com API.

    :param target_url: The URL of the website to get a screenshot of.
    :return: The URL of the screenshot.
    """
    headers = {
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:108.0) Gecko/20100101 Firefox/108.0",
    }

    data = {
        "url": target_url,
        # Sending a random CSS to make the API to generate a new screenshot.
        "css": f"random-tag: {uuid.uuid4()}",
        "render_when_ready": False,
        "viewport_width": 1366,
        "viewport_height": 768,
        "device_scale": 1,
    }

    try:
        resp = await http.post(
            "https://htmlcsstoimage.com/demo_run", headers=headers, json=data
        )
        return resp.json()["url"]
    except (JSONDecodeError, KeyError) as e:
        raise Exception("Screenshot API returned an invalid response.") from e
    except httpx.HTTPError as e:
        raise Exception("Screenshot API seems offline. Try again later.") from e


EMOJI_PATTERN = get_emoji_regex()
