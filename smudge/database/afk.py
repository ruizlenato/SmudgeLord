# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from time import time

from . import database

conn = database.get_conn()


async def is_afk(user_id: int) -> bool:
    cursor = await conn.execute("SELECT * FROM afk WHERE user_id = ?", (user_id,))
    return await cursor.fetchone()


async def set_afk(user_id: int, reason: str):
    await conn.execute(
        "INSERT INTO afk(user_id, reason, time) VALUES(?, ?, ?)",
        (user_id, reason, time()),
    )
    await conn.commit()


async def rm_afk(user_id: int) -> bool:
    await conn.execute("DELETE FROM afk WHERE user_id = ?", (user_id,))
    await conn.commit()
