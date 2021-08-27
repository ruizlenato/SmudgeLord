import yaml
import logging

from os import path

from smudge.database import get_db_lang

LANGUAGES = ["en-US", "pt-BR"]

LOGGER = logging.getLogger(__name__)

strings = {}
strings_folder = path.join(path.dirname(path.realpath(__file__)), "strings")

for i in LANGUAGES:
    strings[i] = yaml.full_load(open("smudge/locales/strings/" + i + ".yml", "r"))


async def tld(chat_id, t, show_none=True):
    LANGUAGE = await get_db_lang(chat_id)
    try:
        if LANGUAGE:
            if LANGUAGE in ("en-US"):
                return strings["en-US"][t]
            if LANGUAGE in ("pt-BR"):
                return strings["pt-BR"][t]
    except KeyError:
        err = f"Warning: No string found for {t}.\nReport it in @Renatoh."
        LOGGER.warning(err)
        return err
