# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import os
import yaml
import logging

from ..database.locales import get_db_lang
from pyrogram.types import CallbackQuery

log = logging.getLogger(__name__)

loaded_locales: dict = {}
locales_name: list = []


def load_locale(locale_path: str) -> None:
    locale = os.path.basename(locale_path).replace(".yml", "")
    locales_name.append(locale)
    with open(locale_path, "r", encoding="utf8") as file:
        content = yaml.load(file.read(), yaml.Loader)
        loaded_locales[locale] = content


locale_dir = "smudge/locales"
for file in os.listdir(locale_dir):
    if file not in ("__init__.py", "__pycache__"):
        log.info("\033[90m[!] - Language %s loadded.\033[0m", file)
        load_locale(f"{locale_dir}/{file}")


async def tld(m, t):
    if isinstance(m, CallbackQuery):
        m = m.message

    lang = await get_db_lang(m.chat.id, m.chat.type)
    m_args = t.split(".")

    try:
        return loaded_locales.get(lang, "en")["strings"][m_args[0]][m_args[1]]
    except KeyError as exc:
        raise exc
