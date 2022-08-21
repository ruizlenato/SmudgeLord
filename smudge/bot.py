# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import sys
import aiocron
import datetime
import logging

from pyrogram import Client
from pyrogram.enums import ParseMode

from smudge.utils import http
from smudge.database import database
from smudge.config import API_HASH, API_ID, BOT_TOKEN, CHAT_LOGS, IPV6, WORKERS

# Logging
logger = logging.getLogger(__name__)

# Date
date = datetime.datetime.now().strftime("%H:%M:%S - %d/%m/%Y")


class Smudge(Client):
    def __init__(self):
        name = self.__class__.__name__.lower()

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
        logger.info("\033[92mConnected to telegram servers.\033[0m")
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

        aiocron.crontab("*/60 * * * *", func=backup, start=True)

    async def stop(self) -> None:
        await http.aclose()
        if database.is_connected:
            await database.close()
        await super().stop()
        logger.warning(f"\033[93mSmudgeLord stopped. Bye!\033[0m")