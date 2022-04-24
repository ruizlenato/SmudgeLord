# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import os
import io
import sys
import html
import traceback

from pyrogram import filters
from pyrogram.types import Message

from contextlib import redirect_stdout

from smudge import Smudge
from smudge.config import SUDOERS
from smudge.database import groups

from rich import print as rprint


@Smudge.on_message(filters.command("(broadcast|announcement)") & filters.user(SUDOERS))
async def broadcast(c: Smudge, m: Message):
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


@Smudge.on_message(filters.command("restart") & filters.user(SUDOERS))
async def broadcast(c: Smudge, m: Message):
    await m.reply_text("Restarting...")
    args = [sys.executable, "-m", "smudge"]
    os.system("cls" if os.name == "nt" else "clear")
    rprint("[red]Restarting...")
    os.execl(sys.executable, *args)


@Smudge.on_message(filters.command("exec") & filters.user(SUDOERS))
async def execs(c: Smudge, m: Message):
    strio = io.StringIO()
    code = m.text.split(maxsplit=1)[1]
    exec(
        "async def __ex(c, m): " + " ".join("\n " + l for l in code.split("\n"))
    )  # skipcq: PYL-W0122
    with redirect_stdout(strio):
        try:
            await locals()["__ex"](c, m)
        except:  # skipcq
            return await m.reply_text(html.escape(traceback.format_exc()))

    if strio.getvalue().strip():
        out = f"<code>{html.escape(strio.getvalue())}</code>"
    else:
        out = "Command executed."
    await m.reply_text(out)
