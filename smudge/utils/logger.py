# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import logging

logging.basicConfig(
    level=logging.INFO,
    format="%(levelname)s | \u001B[35m%(name)s |\u001B[33m %(asctime)s | \u001B[37m%(message)s",
    datefmt="%m/%d %H:%M:%S",
)

# To avoid some annoying log
logging.getLogger("pyrogram").setLevel(logging.WARNING)
logging.getLogger("apscheduler").setLevel(logging.WARNING)

log: logging.Logger = logging.getLogger(__name__)

log.info("\033[1m\033[35mSmudgeLord\033[0m")
log.info("\033[96mProject maintained by:\033[0m RuizLenato")
