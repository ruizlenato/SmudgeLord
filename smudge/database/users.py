# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from . import database

conn = database.get_conn()


async def get_user_data(id: int):
    cursor = await conn.execute("SELECT * FROM users WHERE id = ?", (id,))
    row = await cursor.fetchone()
    await cursor.close()
    return row


async def register_user(id: int, language: str):
    await conn.execute("INSERT INTO users (id, language) values (?, ?)", (id, language))
    await conn.commit()


async def register_lastfm(user_id: int, username: str):
    await conn.execute("UPDATE users SET lastfm_username = ? WHERE id = ?", (username, user_id))
    await conn.commit()
