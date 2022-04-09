# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import re
import math
import httpx
import asyncio

from typing import Tuple, Callable
from functools import wraps, partial

from pyrogram import emoji, Client
from smudge.config import CHAT_LOGS

timeout = httpx.Timeout(120)
http = httpx.AsyncClient(http2=True, timeout=timeout)


def pretty_size(size_bytes):
    if size_bytes == 0:
        return "0B"
    size_name = ("B", "KB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB")
    i = int(math.floor(math.log(size_bytes, 1024)))
    p = math.pow(1024, i)
    s = round(size_bytes / p, 2)
    return "%s %s" % (s, size_name[i])


def aiowrap(func: Callable) -> Callable:
    @wraps(func)
    async def run(*args, loop=None, executor=None, **kwargs):
        if loop is None:
            loop = asyncio.get_event_loop()
        pfunc = partial(func, *args, **kwargs)
        return await loop.run_in_executor(executor, pfunc)

    return run


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


EMOJI_PATTERN = get_emoji_regex()


async def extract_user(c, m) -> Tuple[int, str]:
    """Extract the user from the provided message."""
    user_id = None
    user_first_name = None

    if m.reply_to_message:
        if m.reply_to_message.from_user:
            user_id = m.reply_to_message.from_user.id
            user_first_name = m.reply_to_message.from_user.first_name

        elif m.reply_to_message.sender_chat:
            user_id = m.reply_to_message.sender_chat.id
            user_first_name = m.reply_to_message.sender_chat.first_name

    elif m.command and len(m.command) > 1:
        if m.entities:
            if len(m.entities) > 1:
                required_entity = m.entities[1]
                if required_entity.type == "text_mention":
                    user_id = required_entity.user.id
                    user_first_name = required_entity.user.first_name
                elif required_entity.type == "mention":
                    user_id = m.text[
                        required_entity.offset : required_entity.offset
                        + required_entity.length
                    ]
                    user_first_name = user_id
        else:
            user_id = m.command[1]
            user_first_name = user_id

    else:
        user_id = m.from_user.id
        user_first_name = m.from_user.first_name

    user = await c.get_users(user_id)
    user_id = user.id
    user_first_name = user.first_name

    return user_id, user_first_name


async def send_logs(c, user_mention, user_id, e):
    await c.send_message(
        chat_id=CHAT_LOGS,
        text=(
            "<b>⚠️ Error</b>\n<b>User:</b>{} (<code>{}</code>)\n<b>Log:</b>\n<code>{}</code></b>"
        ).format(user_mention, user_id, e),
    )
    return
