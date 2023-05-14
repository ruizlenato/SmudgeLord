# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import gettext
from functools import wraps

from pyrogram.enums import ChatType
from pyrogram.types import CallbackQuery

from ..database import database

conn = database.get_conn()


async def get_db_lang(m):
    m = m.message if isinstance(m, CallbackQuery) else m

    if m.chat.type == ChatType.PRIVATE:
        cursor = await conn.execute("SELECT lang FROM users WHERE id = (?)", (m.chat.id,))
    elif m.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        cursor = await conn.execute("SELECT lang FROM groups WHERE id = (?)", (m.chat.id,))
    try:
        row = await cursor.fetchone()
        await cursor.close()
        return row[0]
    except TypeError:
        return "en_US"


async def set_db_lang(m, code: str):
    m = m.message if isinstance(m, CallbackQuery) else m

    if m.chat.type == ChatType.PRIVATE:
        await conn.execute("UPDATE users SET lang = ? WHERE id = ?", (code, m.chat.id))
    elif m.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        await conn.execute("UPDATE groups SET lang = ? WHERE id = ?", (code, m.chat.id))
    await conn.commit()


def locale():
    def decorator(func):
        @wraps(func)
        async def wrapper(client, message):
            translation = gettext.translation(
                "bot", "locales", languages=[await get_db_lang(message)]
            )
            translation.install()
            _ = translation.gettext
            return await func(client, message, _)

        return wrapper

    return decorator
