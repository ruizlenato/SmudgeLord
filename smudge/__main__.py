import asyncio
import logging

from pyrogram import Client, idle

from rich.panel import Panel
from rich import box, print as rprint

from tortoise import run_async

from smudge.config import *
from smudge.database import connect_database

# Enable logging
logging.basicConfig(format="%(asctime)s - %(message)s", level="INFO")
logging.getLogger("pyrogram.client").setLevel(logging.WARNING)

log = logging.getLogger("rich")
logs = "[bold purple]SmudgeLord Running[/bold purple]"
logs += f"\n[SmudgeLord] Project maintained by: Renatoh"
rprint(Panel.fit(logs, border_style="turquoise2", box=box.ASCII))

# Pyrogram Client
plugins = dict(root="smudge.plugins")

client = Client(
    "smudge",
    workers=20,
    api_id=API_ID,
    api_hash=API_HASH,
    bot_token=BOT_TOKEN,
    parse_mode="html",
    plugins=plugins,
)


async def main():
    await client.start()
    print("[SmudgeLord] Starting...")
    await connect_database()
    await client.send_message(chat_id=CHAT_LOGS, text="<b>SmudgeLord started!</b>")
    await idle()


if __name__ == "__main__":
    try:
        run_async(main())
        loop = asyncio.get_event_loop()
        loop.run_until_complete(main())
    except KeyboardInterrupt:
        Tortoise.close_connections()
