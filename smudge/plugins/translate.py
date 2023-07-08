import html

from gpytranslate import Translator
from pyrogram import filters
from pyrogram.types import Message

from smudge.bot import Smudge
from smudge.database.users import get_user_data
from smudge.utils.locale import locale

tr = Translator()

# See https://cloud.google.com/translate/docs/languages
# fmt: off
LANGUAGES = [
    "af", "sq", "am", "ar", "hy",
    "as", "ay", "az", "bm", "eu",
    "be", "bn", "bho", "bs", "bg",
    "ca", "ceb", "zh", "co", "hr",
    "cs", "da", "dv", "doi", "nl",
    "en", "eo", "et", "ee", "fil",
    "fi", "fr", "fy", "gl", "ka",
    "de", "el", "gn", "gu", "ht",
    "ha", "haw", "he", "iw", "hi",
    "hmn", "hu", "is", "ig", "ilo",
    "id", "ga", "it", "ja", "jv",
    "jw", "kn", "kk", "km", "rw",
    "gom", "ko", "kri", "ku", "ckb",
    "ky", "lo", "la", "lv", "ln",
    "lt", "lg", "lb", "mk", "mai",
    "mg", "ms", "ml", "mt", "mi",
    "mr", "mni", "lus", "mn", "my",
    "ne", "no", "ny", "or", "om",
    "ps", "fa", "pl", "pt", "pa",
    "qu", "ro", "ru", "sm", "sa",
    "gd", "nso", "sr", "st", "sn",
    "sd", "si", "sk", "sl", "so",
    "es", "su", "sw", "sv", "tl",
    "tg", "ta", "tt", "te", "th",
    "ti", "ts", "tr", "tk", "ak",
    "uk", "ur", "ug", "uz", "vi",
    "cy", "xh", "yi", "yo", "zu"
]
# fmt: on


async def get_tr_lang(text, id):
    user_lang = ((await get_user_data(id))["language"]).split("_")[0]
    if len(text.split()) > 0:
        lang = text.split()[0]
        if lang.split("-")[0] not in LANGUAGES:
            if lang := user_lang:
                pass
            else:
                lang: str = "pt"
        else:
            lang = lang.split("-")[0]
        if len(lang.split("-")) > 1 and lang.split("-")[1] not in LANGUAGES:
            lang: str = "pt"
    else:
        lang = user_lang
    return lang


@Smudge.on_message(filters.command(["tr", "tl"]))
@locale("translate")
async def translate(client: Smudge, message: Message, strings):
    text = message.text[4:]
    lang = await get_tr_lang(text, message.from_user.id)

    text = (
        text.replace(text.split()[0] if len(text.split()) > 0 else text, "", 1).strip()
        if text.startswith(text.split()[0] if len(text.split()) > 0 else text)
        else text
    )

    if not text and message.reply_to_message:
        text = message.reply_to_message.text or message.reply_to_message.caption

    if not text:
        return await message.reply_text(strings["no-args"])
    sent = await message.reply_text(strings["translating"])
    langs = {}

    if len(lang.split("-")) > 1:
        langs["sourcelang"] = lang.split("-")[0]
        langs["targetlang"] = lang.split("-")[1]
    else:
        to_lang = langs["targetlang"] = lang

    trres = await tr.translate(text, **langs)
    text = trres.text

    res = html.escape(text)
    await sent.edit_text(f"<b>{trres.lang}</b> -> <b>{to_lang}:</b>\n<code>{res}</code>")
    return None
