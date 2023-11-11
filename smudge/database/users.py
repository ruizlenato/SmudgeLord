# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from . import database

conn = database.get_conn()


async def get_user_data(id: int):
    cursor = await conn.execute("SELECT * FROM users WHERE id = ?", (id,))
    row = await cursor.fetchone()
    await cursor.close()
    return row


async def get_user_data_from_username(username: str):
    cursor = await conn.execute("SELECT * FROM users WHERE username = ?", (username,))
    row = await cursor.fetchone()
    await cursor.close()
    return row


async def register_user(id: int, language: str, username: str):
    await conn.execute(
        "INSERT OR IGNORE INTO users (id, language, username) values (?, ?, ?)",
        (id, language, username),
    )
    await conn.commit()


async def update_username(id: int, username: str):
    await conn.execute("UPDATE users SET username = ? WHERE id = ?", (username, id))
    await conn.commit()


async def register_lastfm(id: int, username: str):
    await conn.execute(
        "UPDATE users SET lastfm_username = ? WHERE id = ?", (username, id)
    )
    await conn.commit()
