# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import sys
import asyncio
import logging

from pyrogram import idle

from rich.panel import Panel
from rich import box, print as rprint

from .smudge import Smudge
from smudge.utils import http
from smudge.database import database

# Enable logging
logging.basicConfig(format="%(asctime)s - %(message)s", level="WARNING")
logging.getLogger("pyrogram.client").setLevel(logging.WARNING)
log = logging.getLogger("rich")
logs = "[bold purple]SmudgeLord[/bold purple]"
logs += f"\nProject maintained by: RuizLenato"
rprint(Panel.fit(logs, border_style="turquoise2", box=box.ASCII))


async def main():
    smudge = Smudge()

    try:
        # start the bot
        await database.connect()
        await smudge.start()

        if "justtest" not in sys.argv:
            await idle()
    except KeyboardInterrupt:
        # exit gracefully
        rprint("[red]Forced stop... Bye!")
    finally:
        # close https connections and the DB if open
        await smudge.stop()
        await http.aclose()
        if database.is_connected:
            await database.close()


if __name__ == "__main__":
    # open new asyncio event loop
    event_policy = asyncio.get_event_loop_policy()
    event_loop = event_policy.new_event_loop()
    asyncio.set_event_loop(event_loop)

    # start the bot
    event_loop.run_until_complete(main())

    # close asyncio event loop
    event_loop.close()
