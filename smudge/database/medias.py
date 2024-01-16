# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from hydrogram.enums import ChatType
from hydrogram.types import CallbackQuery

from . import database

conn = database.get_conn()


async def toggle_media(message, config: str, mode: bool):
    message = message.message if isinstance(message, CallbackQuery) else message
    id = message.chat.id

    if message.chat.type == ChatType.PRIVATE:
        await conn.execute(f"UPDATE users SET {config} = ? WHERE id = ?", (mode, id))
    else:
        await conn.execute(f"UPDATE chats SET {config} = ? WHERE id = ?", (mode, id))

    await conn.commit()
