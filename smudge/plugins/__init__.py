# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
from pyrogram.types import Message
from pyrogram.enums import ChatType

from ..bot import Smudge
from smudge.utils.locales import LANGUAGES
from smudge.database.chats import add_chat, get_chat

# This is the first plugin run to guarantee
# that the actual chat is initialized in the DB.
@Smudge.on_message(group=-2)
async def check_chat(c: Smudge, m: Message):
    chat = m.chat
    user = m.from_user

    try:
        language_code = user.language_code
    except AttributeError:
        language_code: str = "en-us"

    if language_code not in LANGUAGES:
        language_code: str = "en-us"

    if user and await get_chat(user.id, ChatType.PRIVATE) is None:
        await add_chat(user.id, language_code, ChatType.PRIVATE)

    if await get_chat(chat.id, chat.type) is None:
        await add_chat(chat.id, language_code, chat.type)


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
