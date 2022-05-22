# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import asyncio
from pyrogram import filters
from pyrogram.types import Message
from pyrogram.enums import ChatType, ChatMemberStatus, ChatMembersFilter
from pyrogram.errors import BadRequest, Forbidden, FloodWait

from smudge import Smudge
from smudge.plugins import tld


@Smudge.on_message(filters.command("cleanup"))
async def cleanup(c: Smudge, m: Message):
    try:
        me = await c.get_me()
    except FloodWait as e:
        await asyncio.sleep(e.value)

    if m.chat.type == ChatType.PRIVATE:
        return await m.reply_text(await tld(m, "Admin.err_private"))
    bot = await c.get_chat_member(chat_id=m.chat.id, user_id=me.id)
    member = await c.get_chat_member(chat_id=m.chat.id, user_id=m.from_user.id)
    if member.status not in (
        ChatMemberStatus.ADMINISTRATOR,
        ChatMemberStatus.OWNER,
    ):
        return await m.reply_text(await tld(m, "Admin.noadmin"))
    if bot.status != ChatMemberStatus.ADMINISTRATOR:
        return await m.reply_text(await tld(m, "Admin.botnotadmin"))
    deleted_users = []
    mes = await m.reply_text(await tld(m, "Admin.cleanup_start"))
    async for a in c.get_chat_members(chat_id=m.chat.id):
        if not a.user.is_deleted:
            return await mes.edit_text(await tld(m, "Admin.cleanup_no_deleted"))
        try:
            await c.ban_chat_member(m.chat.id, a.user.id)
            deleted_users.append(a)
            await mes.edit_text(await tld(m, "Admin.cleanup_start_d")).format(
                {len(deleted_users)}
            )
        except BadRequest:
            pass
        except Forbidden as e:
            return await m.reply_text(f"<b>Erro:</b> {e}")
