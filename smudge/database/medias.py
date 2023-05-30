# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from . import database

conn = database.get_conn()


async def auto_downloads(chat_id: int) -> bool:
    cursor = await conn.execute(
        "SELECT auto_downloads FROM medias WHERE chat_id = (?)", (chat_id,)
    )
    try:
        row = await cursor.fetchone()
        return row[0]
    except (IndexError, TypeError):
        return True


async def captions(chat_id: int) -> bool:
    cursor = await conn.execute("SELECT captions FROM medias WHERE chat_id = ?", (chat_id,))
    try:
        row = await cursor.fetchone()
        await cursor.close()
        return row[0]
    except (IndexError, TypeError):
        return False
