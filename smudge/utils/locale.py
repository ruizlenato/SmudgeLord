# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import os
from functools import wraps

import yaml
from hydrogram.enums import ChatType
from hydrogram.types import CallbackQuery

from smudge.database.locale import get_db_lang
from smudge.utils.logger import log

LANGUAGES: dict[str] = {}

for file in os.listdir("locales"):
    if file not in ("__init__.py", "__pycache__"):
        log.info("\033[90m[!] - Language %s loadded.\033[0m", file)
        with open("locales/" + file, encoding="utf8") as f:
            content = yaml.load(f, Loader=yaml.CLoader)
            LANGUAGES[file.replace(".yaml", "")] = content


async def get_string(message, module, name):
    try:
        lang = LANGUAGES[await get_db_lang(message)]["strings"][module][name]
    except KeyError:
        lang = LANGUAGES["en_US"]["strings"][module][name]

    return lang


def locale(module):
    def decorator(func):
        @wraps(func)
        async def wrapper(client, message, *args, **kwargs):
            if (
                message.message.chat.type
                if isinstance(message, CallbackQuery)
                else message.chat.type
            ) == ChatType.CHANNEL:
                return None
            strings = LANGUAGES[await get_db_lang(message)]["strings"][module]

            return await func(client, message, *args, strings, **kwargs)

        return wrapper

    return decorator
