# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import asyncio
from pyrogram.types import Message
from pyrogram import Client, filters
from pyrogram.enums import ChatType, ChatMemberStatus, ChatMembersFilter
from pyrogram.errors import BadRequest, Forbidden, FloodWait

from smudge.utils.locales import tld


@Client.on_message(filters.command("cleanup"))
async def cleanup(c: Client, m: Message):
    deleted_users = []
    chat = m.chat

    if chat.type == ChatType.PRIVATE:
        return await m.reply_text(await tld(m, "Admin.err_private"))

    bot = await c.get_chat_member(chat_id=chat.id, user_id=c.me.id)

    try:
        user = await c.get_chat_member(chat_id=chat.id, user_id=m.from_user.id)
    except AttributeError:
        message = await m.reply_text(await tld(m, "Main.change_lang_uchannel"))
        await asyncio.sleep(5.0)
        return await message.delete()

    if user.status not in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER):
        return await m.reply_text(await tld(m, "Admin.noadmin"))

    if bot.status != ChatMemberStatus.ADMINISTRATOR:
        return await m.reply_text(await tld(m, "Admin.botnotadmin"))

    mes = await m.reply_text(await tld(m, "Admin.cleanup_start"))

    async for member in c.get_chat_members(chat_id=m.chat.id):
        if member.user.is_deleted:
            await c.ban_chat_member(m.chat.id, member.user.id)
            deleted_users.append(member.user.id)

    if not deleted_users:
        return await mes.edit_text(await tld(m, "Admin.cleanup_no_deleted"))
    try:
        await mes.edit_text(
            (await tld(m, "Admin.cleanup_start_d")).format(len(deleted_users))
        )
    except BadRequest:
        pass
    except Forbidden as e:
        return await m.reply_text(f"<b>Erro:</b> {e}")
