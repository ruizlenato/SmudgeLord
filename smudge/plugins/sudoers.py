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
from smudge.database.core import database

from rich import print as rprint

conn = database.get_conn()


@Smudge.on_message(filters.command("restart") & filters.user(SUDOERS))
async def restart(c: Smudge, m: Message):
    await m.reply_text("Restarting...")
    args = [sys.executable, "-m", "smudge"]
    os.system("cls" if os.name == "nt" else "clear")
    rprint("[red]Restarting...")
    os.execl(sys.executable, *args)


@Smudge.on_message(filters.command("broadcast") & filters.user(SUDOERS))
async def broadcast(c: Smudge, m: Message):
    if len(m.command) > 1:
        lang = m.text.split(None, 2)[1]
        text = m.text.split(None, 2)[2]
    else:
        await m.reply_text("Você esqueceu dos argumentos!")
        return
    sm = await m.reply_text("Broadcasting...")
    cursor = await conn.execute("SELECT id FROM groups WHERE lang = ?", (lang,))
    row = await cursor.fetchall()
    success = []
    fail = []
    for keywords in row:
        keyword = keywords[0]
        try:
            if await c.send_message(keyword, text):
                success.append(keyword)
            else:
                fail.append(keyword)
        except:
            fail.append(keyword)
    await sm.edit_text(
        f"Anúncio feito com sucesso! Sua mensagem foi enviada em um total de <code>{len(success)}</code> grupos e falhou o envio em <code>{len(fail)}</code> grupos."
    )


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
