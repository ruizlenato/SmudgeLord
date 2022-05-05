# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import asyncio
from pyrogram.types import Message

from smudge import Smudge
from smudge.database.chats import add_chat, chat_exists

# This is the first plugin run to guarantee
# that the actual chat is initialized in the DB.


@Smudge.on_message(group=-1)
async def check_chat(c: Smudge, m: Message):
    try:
        chat_id = m.chat.id
        chat_type = m.chat.type
        chatexists = await chat_exists(chat_id, chat_type)
    except AttributeError:
        return

    if not chatexists:
        await add_chat(chat_id, chat_type)
        await asyncio.sleep(0.5)
