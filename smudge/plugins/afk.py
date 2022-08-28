# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import re
import asyncio

from pyrogram.types import Message
from pyrogram import Client, filters
from pyrogram.enums import MessageEntityType
from pyrogram.errors import (
    FloodWait,
    UserNotParticipant,
    BadRequest,
    PeerIdInvalid,
    ChatWriteForbidden,
)

from smudge.utils.locales import tld
from smudge.database.afk import set_uafk, get_uafk, del_uafk


@Client.on_message(filters.command("afk") | filters.regex(r"^(?i)brb(\s(?P<args>.+))?"))
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
    try:
        await m.reply_text(afkmsg + reason_txt)
    except ChatWriteForbidden:
        return


async def check_afk(m, user_id, user_fn, user):
    if user_id == user.id:
        return

    if await get_uafk(user_id) is not None:
        try:
            await m.chat.get_member(int(user_id))  # Check if the user is in the group
        except (UserNotParticipant, PeerIdInvalid):
            return

        afkmsg = (await tld(m, "Misc.user_afk")).format(user_fn[:25])
        if await get_uafk(user_id) != "No reason":
            afkmsg += (await tld(m, "Misc.afk_reason")).format(await get_uafk(user_id))
        try:
            return await m.reply_text(afkmsg)
        except ChatWriteForbidden:
            return


@Client.on_message(~filters.private & ~filters.bot & filters.all, group=3)
async def afk(c: Client, m: Message):
    user = m.from_user
    if m.sender_chat:
        return

    try:
        if m.text.startswith(("brb", "/afk")):
            return
    except AttributeError:
        return

    if user and await get_uafk(user.id) is not None:
        await del_uafk(user.id)
        try:
            return await m.reply_text(
                (await tld(m, "Misc.no_longer_afk")).format(user.first_name)
            )
        except ChatWriteForbidden:
            return

    elif m.reply_to_message:
        await check_afk(
            m,
            m.reply_to_message.from_user.id,
            m.reply_to_message.from_user.first_name,
            user,
        )

    elif m.entities:
        for y in m.entities:
            if y.type is MessageEntityType.MENTION:
                try:
                    ent = await c.get_users(m.text[y.offset : y.offset + y.length])
                except (IndexError, KeyError, BadRequest):
                    return
                except FloodWait as e:
                    await asyncio.sleep(e.value)

            elif y.type is MessageEntityType.TEXT_MENTION:
                try:
                    ent = y.user
                except UnboundLocalError:
                    return
            else:
                return

            return await check_afk(m, ent.id, ent.first_name, user)
