# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import asyncio
from pyrogram import Client
from pyrogram.types import Message

from smudge.database.chats import add_chat, chat_exists

# This is the first plugin run to guarantee
# that the actual chat is initialized in the DB.
@Client.on_message(group=-1)
async def check_chat(c: Client, m: Message):
    try:
        chat_id = m.chat.id
        chat_type = m.chat.type
        chatexists = await chat_exists(chat_id, chat_type)
    except AttributeError:
        return

    if not chatexists:
        await add_chat(chat_id, chat_type)
        await asyncio.sleep(0.5)


def __list_all_plugins():
    from os.path import dirname, basename, isfile
    import glob

    mod_paths = glob.glob(f"{dirname(__file__)}/*.py")
    return [
        basename(f)[:-3]
        for f in mod_paths
        if isfile(f)
        and f.endswith(".py")
        and not f.endswith("__init__.py")
        and not f.endswith("start.py")
    ]


all_plugins = sorted(__list_all_plugins())
