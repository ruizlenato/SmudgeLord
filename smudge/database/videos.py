# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
from typing import Optional

from .core import database

conn = database.get_conn()


async def csdl(chat_id: int) -> bool:
    cursor = await conn.execute("SELECT sdl_auto FROM groups WHERE id = ?", (chat_id,))
    row = await cursor.fetchone()
    await cursor.close()
    return row[0]


async def tsdl(chat_id: int, mode: Optional[bool]) -> None:

    await conn.execute("UPDATE groups SET sdl_auto = ? WHERE id = ?", (mode, chat_id))
    await conn.commit()


async def cisdl(chat_id: int) -> bool:
    cursor = await conn.execute(
        "SELECT sdl_images FROM groups WHERE id = ?", (chat_id,)
    )
    row = await cursor.fetchone()
    await cursor.close()
    return row[0]


async def tisdl(chat_id: int, mode: Optional[bool]) -> None:
    await conn.execute("UPDATE groups SET sdl_images = ? WHERE id = ?", (mode, chat_id))
    await conn.commit()
