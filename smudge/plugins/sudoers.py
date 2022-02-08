# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import os
import sys

from pyrogram import Client, filters
from pyrogram.types import Message
from smudge.database import groups
from smudge.config import SUDOERS


@Client.on_message(filters.command("(broadcast|announcement)") & filters.user(SUDOERS))
async def broadcast(c: Client, m: Message):
    sm = await m.reply_text("Broadcasting...")
    command = m.text.split()[0]
    text = m.text[len(command) + 1 :]
    chats = await groups.all()
    success = []
    fail = []
    for chat in chats:
        try:
            if await c.send_message(chat.id, text):
                success.append(chat.id)
            else:
                fail.append(chat.id)
        except:
            fail.append(chat.id)
    await sm.edit_text(
        f"Anúncio feito com sucesso! Sua mensagem foi enviada em um total de <code>{len(success)}</code> grupos e falhou o envio em <code>{len(fail)}</code> grupos."
    )


@Client.on_message(filters.command("restart") & filters.user(SUDOERS))
async def broadcast(c: Client, m: Message):
    await m.reply_text("Restarting...")
    args = [sys.executable, "-m", "smudge"]
    os.execl(sys.executable, *args)
