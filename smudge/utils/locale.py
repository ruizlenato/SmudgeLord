# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import gettext

from pyrogram.enums import ChatType
from pyrogram.types import CallbackQuery

from ..database import database

conn = database.get_conn()


async def get_db_lang(m):
    if isinstance(m, CallbackQuery):
        m = m.message
    if m.chat.type == ChatType.PRIVATE:
        cursor = await conn.execute(
            "SELECT lang FROM users WHERE id = (?)", (m.chat.id,)
        )
    elif m.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        cursor = await conn.execute(
            "SELECT lang FROM groups WHERE id = (?)", (m.chat.id,)
        )
    try:
        row = await cursor.fetchone()
        await cursor.close()
        return row[0]
    except TypeError:
        return "en-us"


def locale():
    def decorator(func):
        async def wrapper(c, m, *args, **kwargs):
            lang = await get_db_lang(m)
            load = gettext.translation("bot", "locales", languages=["pt_BR"])
            load.install()
            return await func(c, m, *args, **kwargs)

        return wrapper

    return decorator
