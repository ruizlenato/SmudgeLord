# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import os
import yaml
from glob import glob
from pyrogram.types import CallbackQuery

from smudge.database import get_db_lang
from smudge import LOGGER

LANGUAGES = ["pt-BR", "en-US"]
strings = {}


def cache_localizations(files):
    """Get all translated strings from files."""
    ldict = {lang: {} for lang in LANGUAGES}
    for file in files:
        lang_name = (file.split(os.path.sep)[2]).replace(".yml", "")
        lang_data = yaml.load(open(file, encoding="utf-8"), Loader=yaml.FullLoader)
        ldict[lang_name] = lang_data
    return ldict


# Get all translation files
lang_files = []
for langs in LANGUAGES:
    strings[langs] = yaml.full_load(
        open(f"smudge/locales/{langs}.yml", "r", encoding="utf-8")
    )
    lang_files += glob(os.path.join("smudge/locales/", f"{langs}.yml"))
lang_dict = cache_localizations(lang_files)


async def tld(m, t):
    # Get Chat
    if isinstance(m, CallbackQuery):
        m = m.message
    LANGUAGE = await get_db_lang(m.chat.id)
    try:
        return strings[LANGUAGE][t]
    except KeyError:
        err = f"Warning: No string found for {t}.\nReport it in @ruizlenatogs."
        LOGGER.warning(err)
        return err
