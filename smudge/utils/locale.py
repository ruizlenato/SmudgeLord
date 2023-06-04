# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import gettext
from functools import wraps

from pyrogram.enums import ChatType
from pyrogram.types import CallbackQuery

from smudge.database.locale import get_db_lang


def locale():
    def decorator(func):
        @wraps(func)
        async def wrapper(client, message, *args, **kwargs):
            message = message.message if isinstance(message, CallbackQuery) else message
            if message.chat.type == ChatType.CHANNEL:
                return None

            translation = gettext.translation(
                "bot", "locales", languages=[await get_db_lang(message)], fallback=True
            )
            _ = translation.gettext
            return await func(client, message, _, *args, **kwargs)

        return wrapper

    return decorator
