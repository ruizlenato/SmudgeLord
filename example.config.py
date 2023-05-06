# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)

from typing import List

# Telegram Bot token get it from Bot Father
BOT_TOKEN: str = ""

# Get it from https://my.telegram.org/apps/
API_ID: int = 12345
API_HASH: str = ""

# SUDOERS (to use some special commands)
SUDOERS: List[int] = [1032274246]

# IPV6 support
IPV6: bool = True

# Workers count
WORKERS: int = 24

# Database file path
DATABASE_PATH: str = "smudge/database/database.db"
