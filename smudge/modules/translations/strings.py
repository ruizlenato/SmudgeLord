from smudge.modules.sql.translation import prev_locale
from smudge.modules.translations.English import EnglishStrings
from smudge.modules.translations.PortugueseBr import PortugueseBrStrings


def tld(chat_id, t, show_none=True):
    LANGUAGE = prev_locale(chat_id)
    print(chat_id, t)
    if LANGUAGE:
        LOCALE = LANGUAGE.locale_name
        if LOCALE in ('pt') and t in PortugueseBrStrings:
            return PortugueseBrStrings[t]
        else:
            if t in EnglishStrings:
                return EnglishStrings[t]
            else:
                return t
    elif show_none:
        if t in EnglishStrings:
            return EnglishStrings[t]
        else:
            return t


def tld_help(chat_id, t):
    LANGUAGE = prev_locale(chat_id)
    print("tld_help ", chat_id, t)
    if LANGUAGE:
        LOCALE = LANGUAGE.locale_name

        t = t + "_help"

        print("Test2", t)

        if LOCALE in ('pt') and t in PortugueseBrStrings:
            return PortugueseBrStrings[t]
        else:
            return False
    else:
        return False
