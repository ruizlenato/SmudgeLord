# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import re
from datetime import datetime

import humanize
from pyrogram import filters
from pyrogram.enums import MessageEntityType
from pyrogram.errors.exceptions import PeerIdInvalid
from pyrogram.types import Message

from smudge.bot import Smudge
from smudge.database.afk import is_afk, rm_afk, set_afk
from smudge.database.locale import get_db_lang
from smudge.database.users import get_user_data_from_username
from smudge.utils.locale import locale


@Smudge.on_message(
    filters.command("afk") | filters.regex(r"(?i)\b^brb\b(\s(?P<args>.+))?")
)
@locale("afk")
async def afk(client: Smudge, message: Message, strings):
    if not message.from_user:
        return

    if await is_afk(message.from_user.id):
        await stop_afk(message, strings)
        return

    if matches := message.matches and message.matches[0]["args"]:
        reason = matches
    elif message.command and len(message.command) > 1:
        reason = message.text.split(None, 1)[1]
    else:
        reason = ""

    await set_afk(message.from_user.id, reason)
    await message.reply_text(
        strings["now-unavailable"].format(message.from_user.first_name)
    )


@Smudge.on_message(~filters.private & ~filters.bot & filters.all, group=2)
@locale("afk")
async def reply_afk(client: Smudge, message: Message, strings):
    if (
        not message.from_user
        or message.text
        and re.findall(r"^\/\bafk\b|^\bbrb\b", message.text)
    ):
        return None

    if message.from_user and await is_afk(message.from_user.id) is not None:
        return await stop_afk(message, strings)
    if message.entities:
        for ent in message.entities:
            if ent.type == MessageEntityType.MENTION:
                if data := (
                    await get_user_data_from_username(
                        message.text[ent.offset : ent.offset + ent.length]
                    )
                ):
                    try:
                        user = await client.get_chat(int(data["id"]))
                    except PeerIdInvalid:
                        return None
                else:
                    return None
            elif ent.type == MessageEntityType.TEXT_MENTION:
                user = ent.user
            elif ent.type == MessageEntityType.BOT_COMMAND:
                return None
            else:
                return None

            await check_afk(message, user.id, user.first_name, strings)

    elif message.reply_to_message and message.reply_to_message.from_user:
        await check_afk(
            message,
            message.reply_to_message.from_user.id,
            message.reply_to_message.from_user.first_name,
            strings,
        )
    return None


async def stop_afk(message: Message, strings):
    if not message.from_user:
        return

    await rm_afk(message.from_user.id)
    await message.reply_text(
        strings["user-available"].format(
            message.from_user.id, message.from_user.first_name
        )
    )
    return


async def check_afk(message: Message, user_id: int, first_name: str, strings):
    if user := await is_afk(user_id):
        if user_id == message.from_user.id:
            return
        if (lang := await get_db_lang(message)) != "en_US":
            humanize.i18n.activate(lang)
        time = humanize.naturaldelta(
            datetime.now() - datetime.fromtimestamp(user["time"])
        )
        res = strings["user-unavailable"].format(user_id, first_name, time)
        if user["reason"]:
            res += strings["user-reason"].format(user["reason"])

        await message.reply_text(res)


__help__ = True
