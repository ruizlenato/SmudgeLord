# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import sys
import aiocron
import logging
import datetime

from pyrogram import Client, enums
from tortoise import Tortoise

from smudge.utils import http
from smudge.database import connect_database
from smudge.config import API_HASH, API_ID, BOT_TOKEN, CHAT_LOGS

from rich import box, print as rprint

# Enable logging
logging.basicConfig(format="%(asctime)s - %(message)s", level="WARNING")
logging.getLogger("pyrogram.client").setLevel(logging.WARNING)
logging.getLogger("spotipy").setLevel(logging.CRITICAL)

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
        )

    async def start(self):
        rprint(f"[yellow] Connecting to telegram's servers...")
        await super().start()  # Connect to telegram's servers
        rprint(f"[yellow] Connecting to the database...")
        await connect_database()  # Connect to the database
        rprint(f"[green] SmudgeLord Started.")

        # Backup the database every 1h
        @aiocron.crontab("*/60 * * * *")
        async def backup() -> None:
            await self.send_document(
                CHAT_LOGS,
                "smudge/database/database.db",
                caption="<b>Database backuped!</b>\n<b>- Date:</b> {}".format(date),
            )
            logging.warning("[SmudgeLord] Database backuped!")

    async def stop(self, *args):
        await Tortoise.close_connections()
        await super().stop()
        await http.aclose()  # Close httpx session
        rprint("[red]SmudgeLord stopped. Bye.")
