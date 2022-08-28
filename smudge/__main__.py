# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import logging
import pyrogram

from smudge.bot import Smudge

# Custom logging format
logging.basicConfig(
    level=logging.INFO,
    format=f"\u001B[35m%(name)s \u001B[31m| %(asctime)s | \u001B[37m%(message)s",
    datefmt="%m/%d %H:%M:%S",
)
logs = "\033[1m\033[35mSmudgeLord\033[0m"
logs += "\n\033[96mProject maintained by:\033[0m RuizLenato"
logs += f"\n\033[93mPyrogram Version:\033[0m {pyrogram.__version__}"
logs += "\n\033[94m------------------------------------------------------\033[0m"
print(logs)

# To avoid some annoying log
logging.getLogger("pyrogram.syncer").setLevel(logging.WARNING)
logging.getLogger("pyrogram.client").setLevel(logging.WARNING)

if __name__ == "__main__":
    Smudge().run()
