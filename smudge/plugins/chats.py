# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

from pyrogram import Client
from pyrogram.types import Message

from smudge.database.core import users, groups

# This is the first plugin run to guarantee
# that the actual chat is initialized in the DB.


async def add_chat(chat_id, chat_type):
    try:
        if chat_type == "private":
            await users.update_or_create(id=chat_id)
        elif chat_type == "group" or "supergroup":
            await groups.update_or_create(id=chat_id)
    except (TypeError, AttributeError):
        return


@Client.on_message(group=-1)
async def check_chat(c: Client, m: Message):
    try:
        chat_id = m.chat.id
        chat_type = m.chat.type
    except (UnboundLocalError, AttributeError):
        pass

    await add_chat(chat_id, chat_type)
