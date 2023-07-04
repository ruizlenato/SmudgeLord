# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import os

from babel import Locale
from pyrogram.enums import ChatType
from pyrogram.types import Message

from smudge.bot import Smudge
from smudge.database.chats import get_chat_data, register_chat
from smudge.database.users import get_user_data, register_user

Languages: list[str] = []  # Loaded Locales
Languages.append("en_US")  # The en_US language doesn't have a file

for file in os.listdir("locales"):
    if not file.endswith(".rst") and not file.endswith(".pot"):
        Languages.append(file)


@Smudge.on_message(group=-1)
async def check_chat(client: Smudge, message: Message):
    chat = message.chat
    user = message.from_user
    if not user:
        return

    try:
        language_code = str(Locale.parse(user.language_code, sep="-"))

    except (AttributeError, TypeError):
        language_code: str = "en_US"

    if language_code not in Languages:
        language_code: str = "en_US"

    if user and await get_user_data(user.id) is None:
        await register_user(user.id, language_code)

    if chat.type in (ChatType.GROUP, ChatType.SUPERGROUP) and await get_chat_data(chat.id) is None:
        await register_chat(chat.id, language_code)
