# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import os
import yaml
from glob import glob
from functools import reduce
from operator import getitem
from pyrogram.types import CallbackQuery

from smudge import LOGGER
from smudge.database.locales import get_db_lang

LANGUAGES = ["pt-BR", "en-US"]
strings = {}


def cache_localizations(files):
    # Get all translated strings from files
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

    lang = await get_db_lang(m.chat.id, m.chat.type)

    m_args = t.split(".")
    # Get lang
    m_args.insert(0, lang)
    m_args.insert(1, "strings")

    try:
        txt = reduce(getitem, m_args, lang_dict)
        return txt
    except KeyError:
        err = f"Warning: No string found for {t}.\nChatID: {m.chat.id}\nReport it in @ruizlenato."
        LOGGER.warning(err)
        return err
