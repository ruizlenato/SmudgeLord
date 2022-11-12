# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
from . import database
from pyrogram.enums import ChatType

conn = database.get_conn()
GROUPS = (ChatType.GROUP, ChatType.SUPERGROUP)


async def add_chat(id, lang, type):
    if type is ChatType.PRIVATE:
        await conn.execute(
            "INSERT INTO users (id, lang) values (?, ?)",
            (
                id,
                lang,
            ),
        )
        await conn.commit()
    elif type in GROUPS:  # groups and supergroups share the same table
        await conn.execute(
            "INSERT INTO groups (id, lang) values (?, ?)",
            (
                id,
                lang,
            ),
        )
        await conn.commit()
    else:
        return


async def get_chat(id, type):
    if type is ChatType.PRIVATE:
        cursor = await conn.execute("SELECT * FROM users WHERE id = ?", (id,))
        row = await cursor.fetchone()
        await cursor.close()
        return row
    elif type in GROUPS:  # groups and supergroups share the same table
        cursor = await conn.execute("SELECT * FROM groups where id = ?", (id,))
        return await cursor.fetchone()
    else:
        return
