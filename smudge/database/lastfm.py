# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
from . import database

conn = database.get_conn()


async def get_last_user(user_id: int):
    cursor = await conn.execute(
        "SELECT lastfm_username FROM users WHERE id = (?)", (user_id,)
    )
    try:
        row = await cursor.fetchone()
        return row[0]
    except (IndexError, TypeError):
        return None


async def set_last_user(user_id: int, username: str):
    await conn.execute(
        "UPDATE users SET lastfm_username = ? WHERE id = ?", (username, user_id)
    )
    await conn.commit()


async def del_last_user(user_id: int):
    await conn.execute(
        "UPDATE users SET lastfm_username = ? WHERE id = ?", ("", user_id)
    )
    await conn.commit()


async def set_spot_user(user_id, access_token: str, refresh_token: str):
    await conn.execute(
        "UPDATE users SET spot_access_token = ?, spot_refresh_token = ? WHERE id = ?",
        (access_token, refresh_token, user_id),
    )
    return await conn.commit()


async def get_spot_user(user_id: int):
    cursor = await conn.execute(
        "SELECT spot_refresh_token FROM users WHERE id = ?", (user_id,)
    )
    try:
        row = await cursor.fetchone()
        return row[0]
    except (IndexError, TypeError):
        return None


async def unreg_spot(user_id: int):
    await conn.execute(
        "UPDATE users SET spot_refresh_token = ? and spot_access_token = ? WHERE id = ?",
        ("", "", user_id),
    )
    await conn.commit()
