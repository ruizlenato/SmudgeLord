# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)

import sys
import uvloop
import datetime
import logging

from pyrogram import Client
from pyrogram.enums import ParseMode
from pyrogram.types import CallbackQuery

from .utils import http
from .database import database
from .config import API_HASH, API_ID, BOT_TOKEN, CHAT_LOGS, IPV6, WORKERS

from apscheduler.schedulers.asyncio import AsyncIOScheduler

# Logging
log = logging.getLogger(__name__)

# Date
date = datetime.datetime.now().strftime("%H:%M:%S - %d/%m/%Y")

uvloop.install()


class Smudge(Client):
    def __init__(self):
        name = self.__class__.__name__.lower()
        self.scheduler = AsyncIOScheduler()

        super().__init__(
            name,
            bot_token=BOT_TOKEN,
            api_hash=API_HASH,
            api_id=API_ID,
            ipv6=IPV6,
            workers=WORKERS,
            parse_mode=ParseMode.HTML,
            workdir="smudge",
            sleep_threshold=180,
            plugins={"root": "smudge.plugins"},
        )

    async def start(self):
        await database.connect()
        log.info("\033[92mConnected to telegram servers.\033[0m")
        await super().start()  # Connect to telegram's servers

        if "test" not in sys.argv:
            await self.send_message(
                CHAT_LOGS, f"<b>{self.me.first_name} started!</b>\n<b>Date:</b> {date}"
            )

        # Backup the database every 1h
        async def backup() -> None:
            await self.send_document(
                CHAT_LOGS,
                "smudge/database/database.db",
                caption=f"<b>Database backuped!</b>\n<b>- Date:</b> {date}",
            )

        self.scheduler.add_job(backup, "interval", minutes=30)
        self.scheduler.start()

    async def stop(self) -> None:
        await http.aclose()
        if database.is_connected:
            await database.close()
        await super().stop()
        log.warning("\033[93mSmudgeLord stopped. Bye!\033[0m")

    @staticmethod
    async def send_logs(self, m, e):
        if isinstance(m, CallbackQuery):
            m = m.message

        user_mention = m.from_user.mention(m.from_user.first_name)
        user_id = m.from_user.id
        return await Smudge.send_message(
            chat_id=CHAT_LOGS,
            text=f"<b>⚠️ Error</b>\n<b>User:</b>{user_mention} (<code>{user_id}</code>)\n<b>Log:</b>\n<code>{e}</code></b>",
        )
