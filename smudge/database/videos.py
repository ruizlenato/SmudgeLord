# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
from typing import Optional

from . import database

conn = database.get_conn()


async def sdl_c(mode: str, id: int):
    cursor = await conn.execute(
        f"SELECT {mode} FROM groups WHERE id = ?",
        (id,),
    )
    row = await cursor.fetchone()
    await cursor.close()
    return row[0]


async def sdl_t(mode: str, id: int, config: Optional[bool]) -> None:
    await conn.execute(f"UPDATE groups SET {mode} = ? WHERE id = ?", (config, id))
    await conn.commit()
