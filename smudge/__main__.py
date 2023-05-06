# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import asyncio

import pyrogram
import uvloop

from .bot import Smudge
from .utils.logger import log

uvloop.install()  # https://docs.pyrogram.org/topics/speedups


async def main():
    client = Smudge()
    await client.start()
    log.info("\033[92m[🚀] - Bot started.\033[0m")
    await pyrogram.idle()
    await client.stop()
    log.warning("\033[93m[⚠️] - Bye!\033[0m")


if __name__ == "__main__":
    event_policy = asyncio.get_event_loop_policy()
    event_loop = event_policy.new_event_loop()
    try:
        event_loop.run_until_complete(main())
    except KeyboardInterrupt:
        log.warning("\033[31mForced stop!\033[0m")
    finally:
        event_loop.close()
