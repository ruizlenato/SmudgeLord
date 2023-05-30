# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from pyrogram.enums import ChatType

from . import database

conn = database.get_conn()
GROUPS = (ChatType.GROUP, ChatType.SUPERGROUP)


async def get_chat_data(id: int):
    cursor = await conn.execute("SELECT * FROM chats WHERE id = ?", (id,))
    row = await cursor.fetchone()
    await cursor.close()
    return row


async def register_chat(id: int, language: str):
    await conn.execute("INSERT INTO chats (id, language) values (?, ?)", (id, language))
    await conn.commit()
