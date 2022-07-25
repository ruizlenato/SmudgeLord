# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
from pyrogram.enums import ChatType

from .core import database

conn = database.get_conn()


async def get_db_lang(chat_id: int, chat_type: str):
    if chat_type == ChatType.PRIVATE:
        cursor = await conn.execute("SELECT lang FROM users WHERE id = (?)", (chat_id,))
    elif chat_type in (ChatType.GROUP, ChatType.SUPERGROUP):
        cursor = await conn.execute(
            "SELECT lang FROM groups WHERE id = (?)", (chat_id,)
        )
    try:
        row = await cursor.fetchone()
        return row[0]
    except IndexError:
        return None
    except TypeError:
        return "en-US"


async def set_db_lang(chat_id: int, lang_code: str, chat_type: str):
    if chat_type == ChatType.PRIVATE:
        await conn.execute(
            "UPDATE users SET lang = ? WHERE id = ?", (lang_code, chat_id)
        )
    elif chat_type in (ChatType.GROUP, ChatType.SUPERGROUP):
        await conn.execute(
            "UPDATE groups SET lang = ? WHERE id = ?", (lang_code, chat_id)
        )
    await conn.commit()
