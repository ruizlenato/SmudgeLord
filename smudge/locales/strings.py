import os
import yaml

from smudge.database import get_db_lang
from smudge import LOGGER

LANGUAGES = ["en-US", "pt-BR"]
strings = {}

for langs in LANGUAGES:
    strings[langs] = yaml.full_load(open(f"smudge/locales/strings/{langs}.yml", "r"))
    parsed_yaml_file = yaml.load(
        open(f"smudge/locales/strings/{langs}.yml"), Loader=yaml.FullLoader
    )
    print(parsed_yaml_file["language_name"])


async def tld(chat_id, t):
    LANGUAGE = await get_db_lang(chat_id)
    try:
        return strings[LANGUAGE][t]
    except KeyError:
        err = f"Warning: No string found for {t}.\nReport it in @Renatoh."
        LOGGER.warning(err)
        return err
