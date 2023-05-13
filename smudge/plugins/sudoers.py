# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import os
import sys
import traceback

from pyrogram import filters
from pyrogram.types import Message

from config import SUDOERS

from ..bot import Smudge


@Smudge.on_message(filters.command("restart") & filters.user(SUDOERS))
async def restart(c: Smudge, m: Message):
    await m.reply_text("Restarting...")
    args = [sys.executable, "-m", "smudge"]
    os.system("cls" if os.name == "nt" else "clear")
    print("\033[91mRestarting...\033[0m")
    os.execl(sys.executable, *args)


@Smudge.on_message(filters.command("exec") & filters.user(SUDOERS))
async def execs(c: Smudge, m: Message):
    code = m.text.split(maxsplit=1)[1]
    func = f"async def _aexec_(c: Smudge, m: Message):"
    for line in code.split("\n"):
        func += f"\n    {line}"
    exec(func)
    try:
        await locals()["_aexec_"](c, m)
    except BaseException:
        error = traceback.format_exc()
        await m.reply_text(f"<b>Error:</b>\n<code>{error}</code>")
        return
    await m.reply_text(f"<b>Input:</b>\n<code>{code}</code>")
