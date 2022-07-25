# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import re
import asyncio

from pyrogram.types import Message
from pyrogram import Client, filters, enums
from pyrogram.errors import FloodWait, UserNotParticipant, BadRequest, PeerIdInvalid

from smudge.utils.locales import tld
from smudge.database.afk import set_uafk, get_uafk, del_uafk


@Client.on_message(filters.command("afk"))
@Client.on_message(filters.regex(r"^(?i)brb(\s(?P<args>.+))?"))
async def set_afk(c: Client, m: Message):
    try:
        user = m.from_user
        afkmsg = (await tld(m, "Misc.user_now_afk")).format(user.id, user.first_name)
    except AttributeError:
        return

    if m.matches and m.matches[0]["args"]:
        reason = m.matches[0]["args"]
        reason_txt = (await tld(m, "Misc.afk_reason")).format(reason)
    elif m.matches or len(m.command) <= 1:
        reason = "No reason"
        reason_txt = ""
    else:
        reason = m.text.split(None, 1)[1]
        reason_txt = (await tld(m, "Misc.afk_reason")).format(reason)
    await set_uafk(m.from_user.id, reason)
    await m.reply_text(afkmsg + reason_txt)


@Client.on_message(filters.group & ~filters.bot, group=1)
async def afk_watcher(c: Client, m: Message):
    user = m.from_user
    if m.sender_chat:
        return
    try:
        if m.text.startswith(("brb", "/afk")):
            return
    except AttributeError:
        return

    try:
        user_afk = await get_uafk(user.id)
    except AttributeError:
        return

    if user_afk:
        await del_uafk(user.id)
        return await m.reply_text(
            (await tld(m, "Misc.no_longer_afk")).format(user.first_name)
        )

    if m.entities:
        for y in m.entities:
            if y.type == enums.MessageEntityType.MENTION:
                x = re.search(r"@(\w+)", m.text)  # Regex to get @username
                try:
                    user_id = (await c.get_users(x[1])).id
                    if user_id == user.id:
                        return
                    user_first_name = user.first_name
                except (IndexError, BadRequest, KeyError):
                    return
                except FloodWait as e:  # Avoid FloodWait
                    await asyncio.sleep(e.value)
            elif y.type == enums.MessageEntityType.TEXT_MENTION:
                try:
                    user_id = y.user.id
                    if user_id == user.id:
                        return
                    user_first_name = y.user.first_name
                except UnboundLocalError:
                    return
            else:
                return

    elif m.reply_to_message and m.reply_to_message.from_user:
        try:
            user_id = m.reply_to_message.from_user.id
            user_first_name = m.reply_to_message.from_user.first_name
        except AttributeError:
            return
    else:
        return  # No user to set afk

    if not await get_uafk(user_id):
        return

    try:
        await m.chat.get_member(int(user_id))  # Check if the user is in the group
    except (UserNotParticipant, PeerIdInvalid):
        return

    afkmsg = (await tld(m, "Misc.user_afk")).format(user_first_name[:25])
    if await get_uafk(user_id) != "No reason":
        afkmsg += (await tld(m, "Misc.afk_reason")).format(await get_uafk(user_id))
    return await m.reply_text(afkmsg)
