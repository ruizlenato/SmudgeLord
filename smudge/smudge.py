# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import sys
import aiocron
import asyncio
import logging
import datetime

from pyrogram import Client, enums
from pyrogram.errors import FloodWait

from smudge.config import API_HASH, API_ID, BOT_TOKEN, CHAT_LOGS

from rich import print as rprint

# Date
date = datetime.datetime.now().strftime("%H:%M:%S - %d/%m/%Y")


class Smudge(Client):
    def __init__(self):
        name = self.__class__.__name__.lower()

        super().__init__(
            name=name,
            api_hash=API_HASH,
            api_id=API_ID,
            bot_token=BOT_TOKEN,
            parse_mode=enums.ParseMode.HTML,
            workers=24,
            workdir="smudge",
            plugins={"root": "smudge.plugins"},
            sleep_threshold=0.5,
        )

    async def start(self):
        rprint("[green]Connected to telegram servers.[/]")
        await super().start()  # Connect to telegram's servers

        try:
            self.me = await self.get_me()
        except FloodWait as e:
            await asyncio.sleep(e.value)

        if "test" not in sys.argv:
            await self.send_message(
                chat_id=CHAT_LOGS,
                text="<b>{} started!</b>\n<b>Date:</b> {}".format(
                    self.me.first_name, date
                ),
            )

        # Backup the database every 1h
        async def backup() -> None:
            await self.send_document(
                CHAT_LOGS,
                "smudge/database/database.db",
                caption="<b>Database backuped!</b>\n<b>- Date:</b> {}".format(date),
            )
            logging.warning("[SmudgeLord] Database backuped!")

        aiocron.crontab("*/60 * * * *", func=backup, start=True)

        rprint("[bold green]- Started.[/]")

    async def stop(self, *args):
        await super().stop()
        rprint("[red]SmudgeLord stopped. Bye.")
