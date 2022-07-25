# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import sys
import aiocron
import datetime

from pyrogram import Client, enums

from smudge.config import API_HASH, API_ID, BOT_TOKEN, CHAT_LOGS

# Date
date = datetime.datetime.now().strftime("%H:%M:%S - %d/%m/%Y")


class Smudge(Client):
    def __init__(self):
        name = self.__class__.__name__.lower()

        super().__init__(
            name,
            workdir="smudge",
            api_id=API_ID,
            api_hash=API_HASH,
            bot_token=BOT_TOKEN,
            sleep_threshold=180,
            parse_mode=enums.ParseMode.HTML,
            plugins={"root": "smudge.plugins"},
        )

    async def start(self):
        print("\033[92mConnected to telegram servers.\033[0m")
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

        print("\033[92m- Started.\033[0m")

    async def stop(self, *args):
        await super().stop()
        print(f"\033[93mSmudgeLord stopped. Bye!\033[0m")
