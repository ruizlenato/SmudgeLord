# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
from .core import database
from pyrogram.enums import ChatType

conn = database.get_conn()


async def set_uafk(id: int, reason: str):
    cursor = await conn.execute("SELECT id FROM users where id = ?", (id,))
    row = await cursor.fetchone()
    await cursor.close()
    if row is None:
        await conn.execute("INSERT INTO users (id) values (?)", (id,))

    await conn.execute("UPDATE users SET afk_reason = ? WHERE id = ?", (reason, id))
    await conn.commit()


async def get_uafk(id: int):
    cursor = await conn.execute("SELECT afk_reason FROM users WHERE id = ?", (id,))
    row = await cursor.fetchone()
    await cursor.close()
    return row[0]


async def del_uafk(id: int):
    await conn.execute("UPDATE users SET afk_reason = ? WHERE id = ?", ("", id))
    await conn.commit()
