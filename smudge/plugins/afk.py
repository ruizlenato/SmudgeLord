# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import re
import asyncio

from pyrogram.types import Message
from pyrogram import filters, enums
from pyrogram.errors import FloodWait, UserNotParticipant, BadRequest

from smudge import Smudge
from smudge.plugins import tld

from smudge.plugins import tld
from smudge.database.core import database

conn = database.get_conn()


async def set_afk_user(user_id: int, reason: str):
    cursor = await conn.execute("SELECT id FROM users where id = ?", (user_id,))
    row = await cursor.fetchone()
    if not bool(row):
        await conn.execute("INSERT INTO users (id) values (?)", (user_id,))

    await conn.execute(
        "UPDATE users SET afk_reason = ? WHERE id = ?", (reason, user_id)
    )
    await conn.commit()


async def get_afk_user(user_id: int):
    cursor = await conn.execute(
        "SELECT afk_reason FROM users WHERE id = (?)", (user_id,)
    )
    try:
        row = await cursor.fetchone()
        return row[0]
    except (IndexError, TypeError):
        return None


async def del_afk_user(user_id: int):
    await conn.execute("UPDATE users SET afk_reason = ? WHERE id = ?", ("", user_id))
    await conn.commit()


@Smudge.on_message(filters.command("afk"))
@Smudge.on_message(filters.regex(r"^(?i)brb(\s(?P<args>.+))?"))
async def set_afk(_, m: Message):
    afkmsg = (await tld(m, "Misc.user_now_afk")).format(
        m.from_user.id, m.from_user.first_name
    )

    if m.matches:
        if m.matches[0]["args"]:
            reason = m.matches[0]["args"]
            reason_txt = (await tld(m, "Misc.afk_reason")).format(reason)
        else:
            reason = "No reason"
            reason_txt = ""
    else:
        if len(m.command) > 1:
            reason = m.text.split(None, 1)[1]
            reason_txt = (await tld(m, "Misc.afk_reason")).format(reason)
        else:
            reason = "No reason"
            reason_txt = ""

    await set_afk_user(m.from_user.id, reason)
    await m.reply_text(afkmsg + reason_txt)

    await m.stop_propagation()


@Smudge.on_message(filters.all & ~filters.bot, group=2)
async def rem_afk(c: Smudge, m: Message):
    if not m.from_user:
        return

    user_afk = await get_afk_user(m.from_user.id)

    if not user_afk:
        return

    await del_afk_user(m.from_user.id)
    await m.reply_text(
        (await tld(m, "Misc.no_longer_afk")).format(m.from_user.first_name)
    )


@Smudge.on_message(filters.all & ~filters.bot, group=3)
async def afk_mentioned(c: Smudge, m: Message):
    if m.entities:
        for y in m.entities:
            if y.type == enums.MessageEntityType.MENTION:
                x = re.search("@(\w+)", m.text)  # Regex to get @username
                try:
                    user = await c.get_users(x.group(1))
                except FloodWait as e:  # Avoid FloodWait
                    await asyncio.sleep(e.value)
                    user = await c.get_users(x.group(1))
                except (IndexError, BadRequest, KeyError):
                    return

                user_id = user.id
                user_first_name = user.first_name
            else:
                return
    elif m.reply_to_message and m.reply_to_message.from_user:
        try:
            user_id = m.reply_to_message.from_user.id
            user_first_name = m.reply_to_message.from_user.first_name
        except AttributeError:
            return
    else:
        return

    try:
        if user_id == m.from_user.id:
            return
    except AttributeError:
        return
    except FloodWait as e:  # Avoid FloodWait
        await asyncio.sleep(e.value)

    try:
        await m.chat.get_member(user_id)  # Check if the user is in the group
        pass
    except UserNotParticipant:
        return

    user_afk = await get_afk_user(user_id)

    if not user_afk:
        return
    else:
        afkmsg = (await tld(m, "Misc.user_afk")).format(user_first_name)
        if not await get_afk_user(user_id) == "No reason":
            afkmsg += (await tld(m, "Misc.afk_reason")).format(
                await get_afk_user(user_id)
            )
        else:
            pass
        await m.reply_text(afkmsg)
