# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

from pyrogram import Client, filters
from pyrogram.types import Message

from smudge import LOGGER
from smudge.utils import extract_user
from smudge.locales.strings import tld
from tortoise.exceptions import DoesNotExist
from smudge.database.core import users


async def set_afk_user(user_id: int, reason: str):
    await users.filter(id=user_id).update(afk_reason=reason)
    return


async def get_afk_user(user_id: int):
    try:
        return (await users.get(id=user_id)).afk_reason
    except DoesNotExist:
        return None


async def del_afk_user(user_id: int):
    await users.filter(id=user_id).update(afk_reason="False")
    return


@Client.on_message(filters.command("afk"))
async def set_afk(_, m: Message):

    afkmsg = (await tld(m.chat.id, "user_now_afk")).format(
        m.from_user.id, m.from_user.first_name
    )

    if len(m.command) > 1:
        reason = m.text.split(None, 1)[1]
        reason_txt = (await tld(m.chat.id, "afk_reason")).format(reason)
    else:
        reason = "No reason"
        reason_txt = ""

    await set_afk_user(m.from_user.id, reason)
    await m.reply_text(afkmsg + reason_txt)

    await m.stop_propagation()


@Client.on_message(filters.group, group=11)
async def rem_afk(c: Client, m: Message):
    if not m.from_user:
        return

    user_afk = await get_afk_user(m.from_user.id)

    if user_afk == "False":
        return

    await del_afk_user(m.from_user.id)
    await m.reply_text(
        (await tld(m.chat.id, "no_longer_afk")).format(m.from_user.first_name)
    )


@Client.on_message(filters.group & ~filters.bot, group=12)
async def afk_mentioned(c: Client, m: Message):
    if not m.from_user:
        return

    get_me = await c.get_me()
    user_id, user_first_name = await extract_user(c, m)
    user_afk = await get_afk_user(user_id)

    if user_id == get_me.id:
        return

    if user_afk == "False":
        return
    else:
        afkmsg = (await tld(m.chat.id, "user_afk")).format(user_first_name)
        if not await get_afk_user(user_id) == "No reason":
            afkmsg += (await tld(m.chat.id, "afk_reason")).format(
                await get_afk_user(user_id)
            )
        else:
            pass
        await m.reply_text(afkmsg)

    await m.stop_propagation()
