# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import re
import asyncio

from pyrogram.types import Message
from pyrogram import Client, filters, enums
from pyrogram.errors import FloodWait, UserNotParticipant, BadRequest, PeerIdInvalid

from smudge.utils.locales import tld
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


@Client.on_message(filters.command("afk"))
@Client.on_message(filters.regex(r"^(?i)brb(\s(?P<args>.+))?"))
async def set_afk(_, m: Message):
    try:
        afkmsg = (await tld(m, "Misc.user_now_afk")).format(
            m.from_user.id, m.from_user.first_name
        )
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
    await set_afk_user(m.from_user.id, reason)
    await m.reply_text(afkmsg + reason_txt)


@Client.on_message(filters.group & ~filters.bot, group=1)
async def afk(c: Client, m: Message):
    try:
        if await get_afk_user(m.from_user.id):
            user_afk = await get_afk_user(m.from_user.id)
            if not user_afk:
                return
            else:
                await del_afk_user(m.from_user.id)
                await m.reply_text(
                    (await tld(m, "Misc.no_longer_afk")).format(m.from_user.first_name)
                )
        else:
            if m.entities:
                for y in m.entities:
                    if y.type == enums.MessageEntityType.MENTION:
                        x = re.search(r"@(\w+)", m.text)  # Regex to get @username
                        try:
                            user = await c.get_users(x[1])
                            user_id = user.id
                            user_first_name = user.first_name
                        except FloodWait as e:  # Avoid FloodWait
                            await asyncio.sleep(e.value)
                        except (IndexError, BadRequest, KeyError):
                            return
                    elif y.type == enums.MessageEntityType.TEXT_MENTION:
                        try:
                            user_id = y.user.id
                            user_first_name = y.user.first_name
                        except UnboundLocalError:
                            return
                        except FloodWait as e:  # Avoid FloodWait
                            await asyncio.sleep(e.value)
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
            except (AttributeError, UnboundLocalError):
                return
            except FloodWait as e:  # Avoid FloodWait
                await asyncio.sleep(e.value)

            try:
                await m.chat.get_member(
                    int(user_id)
                )  # Check if the user is in the group
            except (UserNotParticipant, PeerIdInvalid):
                return

            user_afk = await get_afk_user(user_id)

            if not user_afk:
                return
            afkmsg = (await tld(m, "Misc.user_afk")).format(user_first_name)
            if await get_afk_user(user_id) != "No reason":
                afkmsg += (await tld(m, "Misc.afk_reason")).format(
                    await get_afk_user(user_id)
                )
            await m.reply_text(afkmsg)
    except FloodWait as e:  # Avoid FloodWait
        await asyncio.sleep(e.value)
    except AttributeError:
        return
