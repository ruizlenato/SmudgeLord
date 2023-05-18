# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from pyrogram.enums import ChatType

from . import database

conn = database.get_conn()
GROUPS = (ChatType.GROUP, ChatType.SUPERGROUP)


async def add_chat(id, language, type):
    if type is ChatType.PRIVATE:
        await conn.execute(
            "INSERT INTO users (user_id, language) values (?, ?)",
            (
                id,
                language,
            ),
        )
        await conn.commit()
    elif type in GROUPS:  # groups and supergroups share the same table
        await conn.execute(
            "INSERT INTO groups (chat_id, language) values (?, ?)",
            (
                id,
                language,
            ),
        )
        await conn.commit()
    else:
        return


async def get_chat(id, type):
    if type is ChatType.PRIVATE:
        cursor = await conn.execute("SELECT * FROM users WHERE user_id = ?", (id,))
        row = await cursor.fetchone()
        await cursor.close()
        return row

    if type in GROUPS:  # groups and supergroups share the same table
        cursor = await conn.execute("SELECT * FROM groups where chat_id = ?", (id,))
        return await cursor.fetchone()

    return None
