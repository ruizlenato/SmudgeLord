# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import sentry_sdk
from pyrogram import Client
from pyrogram.enums import ParseMode

from smudge.database import database
from smudge.utils.utils import http

from .config import config
from .utils.logger import log


class Smudge(Client):
    def __init__(self):
        name = self.__class__.__name__.lower()

        super().__init__(
            name,
            api_id=config["API_ID"],
            api_hash=config["API_HASH"],
            bot_token=config["BOT_TOKEN"],
            workers=int(config["WORKERS"]),
            parse_mode=ParseMode.HTML,
            plugins={"root": "smudge.plugins"},
            sleep_threshold=180,
        )

    async def start(self):
        if SENTRY_KEY := config["SENTRY_KEY"]:
            log.info("\033[94m[📉] - Starting Sentry integration...\033[0m")
            sentry_sdk.init(SENTRY_KEY)
        await database.connect()
        await super().start()

    async def stop(self) -> None:
        await http.aclose()
        if database.is_connected:
            await database.close()
        await super().stop()
