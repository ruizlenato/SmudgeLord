# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import os
import io
import sys
import html
import traceback

from pyrogram.types import Message
from pyrogram import Client, filters
from pyrogram.errors import BadRequest

from contextlib import redirect_stdout

from smudge.config import SUDOERS
from smudge.database.core import database

conn = database.get_conn()


@Client.on_message(filters.command("restart") & filters.user(SUDOERS))
async def restart(c: Client, m: Message):
    await m.reply_text("Restarting...")
    args = [sys.executable, "-m", "smudge"]
    os.system("cls" if os.name == "nt" else "clear")
    print("\033[91mRestarting...\033[0m")
    os.execl(sys.executable, *args)


@Client.on_message(filters.command("exec") & filters.user(SUDOERS))
async def execs(c: Client, m: Message):
    strio = io.StringIO()
    code = m.text.split(maxsplit=1)[1]
    exec(
        "async def __ex(c, m): " + " ".join("\n " + l for l in code.split("\n"))
    )  # skipcq: PYL-W0122
    with redirect_stdout(strio):
        try:
            await locals()["__ex"](c, m)
        except BaseException:  # skipcq
            return await m.reply_text(html.escape(traceback.format_exc()))

    if strio.getvalue().strip():
        out = f"<code>{html.escape(strio.getvalue())}</code>"
    else:
        out = "Command executed."
    await m.reply_text(out)
