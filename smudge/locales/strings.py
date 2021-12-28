import yaml

from os import path

from smudge.database import get_db_lang
from smudge import LOGGER

LANGUAGES = ["en-US", "pt-BR"]


strings = {}
strings_folder = path.join(path.dirname(path.realpath(__file__)), "strings")

for i in LANGUAGES:
    strings[i] = yaml.full_load(open("smudge/locales/strings/" + i + ".yml", "r"))


async def tld(chat_id, t):
    LANGUAGE = await get_db_lang(chat_id)
    try:
        return strings[LANGUAGE][t]
    except KeyError:
        err = f"Warning: No string found for {t}.\nReport it in @Renatoh."
        LOGGER.warning(err)
        return err
