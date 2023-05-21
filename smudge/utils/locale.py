# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import gettext
from functools import wraps

from smudge.database.locale import get_db_lang


def locale():
    def decorator(func):
        @wraps(func)
        async def wrapper(client, message):
            translation = gettext.translation(
                "bot", "locales", languages=[await get_db_lang(message)], fallback=True
            )
            translation.install()
            _ = translation.gettext
            return await func(client, message, _)

        return wrapper

    return decorator
