# EduuRobot https://github.com/AmanoTeam/EduuRobot
# Some commands were ported from EduuRobot, so I leave the credits here.
import asyncio 
import sys

from smudge import SUDO_USERS, TOKEN, pyrosmudge
from pyrogram import Client, filters
from pyrogram.types import Message

@pyrosmudge.on_message(filters.command("restart") & filters.user(SUDO_USERS))
async def restart(c: Client, m: Message):
    await m.reply_text("Restarting...")
    args = [sys.executable, "-m", "smudge"]
    os.execl(sys.executable, *args)

  
