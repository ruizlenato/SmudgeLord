# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
from pyrogram.enums import ChatType

from .core import database

conn = database.get_conn()


async def add_chat(chat_id, chat_type):
    if chat_type == ChatType.PRIVATE:
        await conn.execute("INSERT INTO users (id) values (?)", (chat_id,))
        await conn.commit()
    elif chat_type in (
        ChatType.GROUP,
        ChatType.SUPERGROUP,
    ):  # groups and supergroups share the same table
        await conn.execute(
            "INSERT INTO groups (id, lang) values (?, ?)", (chat_id, "en-US")
        )
        await conn.commit()
    elif chat_type == ChatType.CHANNEL:
        await conn.execute("INSERT INTO channels (id) values (?)", (chat_id,))
        await conn.commit()
    else:
        raise TypeError("Unknown chat type '%s'." % chat_type)
    return True


async def chat_exists(chat_id, chat_type):
    if chat_type == ChatType.PRIVATE:
        cursor = await conn.execute("SELECT id FROM users where id = ?", (chat_id,))
        row = await cursor.fetchone()
        return bool(row)
    elif chat_type in (
        ChatType.GROUP,
        ChatType.SUPERGROUP,
    ):
        cursor = await conn.execute("SELECT id FROM groups where id = ?", (chat_id,))
        row = await cursor.fetchone()
        return bool(row)
    elif chat_type == ChatType.CHANNEL:
        cursor = await conn.execute("SELECT id FROM channels where id = ?", (chat_id,))
        row = await cursor.fetchone()
        return bool(row)
    raise TypeError("Unknown chat type '%s'." % chat_type)
