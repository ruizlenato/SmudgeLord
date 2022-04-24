# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import logging
import datetime


from rich.panel import Panel
from rich import box, print as rprint

from .smudge import Smudge

# Enable logging
logging.basicConfig(format="%(asctime)s - %(message)s", level="WARNING")
logging.getLogger("pyrogram.client").setLevel(logging.WARNING)
log = logging.getLogger("rich")
logs = "[bold purple]SmudgeLord[/bold purple]"
logs += f"\nProject maintained by: RuizLenato"
rprint(Panel.fit(logs, border_style="turquoise2", box=box.ASCII))

if __name__ == "__main__":
    try:
        Smudge().run()
    except KeyboardInterrupt:
        log.warning("Forced stop.")
