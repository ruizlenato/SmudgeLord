# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import gettext
from typing import Union

from pyrogram import Client
from pyrogram.enums import ChatType
from pyrogram.types import CallbackQuery, Message

from ..database import database

conn = database.get_conn()


async def get_db_lang(id: int, type: str):
    if type == ChatType.PRIVATE:
        cursor = await conn.execute("SELECT lang FROM users WHERE id = (?)", (id,))
    elif type in (ChatType.GROUP, ChatType.SUPERGROUP):
        cursor = await conn.execute("SELECT lang FROM groups WHERE id = (?)", (id,))
    try:
        row = await cursor.fetchone()
        await cursor.close()
        return row[0]
    except TypeError:
        return "en-us"


def l10n():
    def decorator(func):
        async def wrapper(
            client: Client, m: Union[CallbackQuery, Message], *args, **kwargs
        ):
            if isinstance(m, CallbackQuery):
                m = m.message

            lang = await get_db_lang(m.chat.id, m.chat.type)
            load = gettext.translation("bot", "locales", languages=["pt_BR"])
            load.install()
            return await func(client, m, *args, **kwargs)

        return wrapper

    return decorator
