# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import sys
import aiocron
import datetime

from pyrogram import Client, enums

from smudge.config import API_HASH, API_ID, BOT_TOKEN, CHAT_LOGS

from rich import print as rprint

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
        rprint("[green]Connected to telegram servers.[/]")
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

        rprint("[bold green]- Started.[/]")

    async def stop(self, *args):
        await super().stop()
        rprint("[red]SmudgeLord stopped. Bye.")
