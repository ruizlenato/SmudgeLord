# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
import os
import sys
import shutil


from .utils import http
from .database import database
from .config import API_HASH, API_ID, BOT_TOKEN, CHAT_LOGS

from pyrogram import Client
from pyrogram.enums import ParseMode
from pyrogram.types import CallbackQuery

from apscheduler.schedulers.asyncio import AsyncIOScheduler


class Smudge(Client):
    def __init__(self):
        name = self.__class__.__name__.lower()
        self.scheduler = AsyncIOScheduler()

        super().__init__(
            name,
            bot_token=BOT_TOKEN,
            api_hash=API_HASH,
            api_id=API_ID,
            workers=24,
            parse_mode=ParseMode.HTML,
            workdir="smudge",
            sleep_threshold=180,
            plugins={"root": "smudge.plugins"},
        )

    async def start(self):
        await database.connect()
        shutil.rmtree("./downloads/", ignore_errors=True)
        os.mkdir("./downloads/")
        await super().start()  # Connect to telegram's servers

        if "test" not in sys.argv:
            await self.send_message(CHAT_LOGS, f"<b>{self.me.first_name} started!</b>")

        # Backup the database every 1h
        async def backup() -> None:
            await self.send_document(
                CHAT_LOGS,
                "smudge/database/database.db",
                caption="<b>Database backuped!</b>",
            )

        self.scheduler.add_job(backup, "interval", minutes=30)
        self.scheduler.start()

    async def stop(self) -> None:
        await http.aclose()
        if database.is_connected:
            await database.close()
        await super().stop()

    @staticmethod
    async def send_logs(self, m, e):
        if isinstance(m, CallbackQuery):
            m = m.message

        user_mention = m.from_user.mention(m.from_user.first_name)
        user_id = m.from_user.id
        return await self.send_message(
            chat_id=CHAT_LOGS,
            text=f"<b>⚠️ Error</b>\n<b>User:</b>{user_mention} (<code>{user_id}</code>)\n<b>Log:</b>\n<code>{e}</code></b>",
        )
