import html
import dicioinformal

from gpytranslate import Translator

from smudge import SCREENSHOT_API_KEY
from smudge.locales.strings import tld

from pyrogram.types import Message
from pyrogram import Client, filters
from pyrogram.errors import ImageProcessFailed

tr = Translator()

# See https://cloud.google.com/translate/docs/languages
# fmt: off
LANGUAGES = [
    "af", "sq", "am", "ar", "hy",
    "az", "eu", "be", "bn", "bs",
    "bg", "ca", "ceb", "zh", "co",
    "hr", "cs", "da", "nl", "en",
    "eo", "et", "fi", "fr", "fy",
    "gl", "ka", "de", "el", "gu",
    "ht", "ha", "haw", "he", "iw",
    "hi", "hmn", "hu", "is", "ig",
    "id", "ga", "it", "ja", "jv",
    "kn", "kk", "km", "rw", "ko",
    "ku", "ky", "lo", "la", "lv",
    "lt", "lb", "mk", "mg", "ms",
    "ml", "mt", "mi", "mr", "mn",
    "my", "ne", "no", "ny", "or",
    "ps", "fa", "pl", "pt", "pa",
    "ro", "ru", "sm", "gd", "sr",
    "st", "sn", "sd", "si", "sk",
    "sl", "so", "es", "su", "sw",
    "sv", "tl", "tg", "ta", "tt",
    "te", "th", "tr", "tk", "uk",
    "ur", "ug", "uz", "vi", "cy",
    "xh", "yi", "yo", "zu",
]
# fmt: on


def get_tr_lang(text):
    if len(text.split()) > 0:
        lang = text.split()[0]
        if lang.split("-")[0] not in LANGUAGES:
            lang = "pt"
        if len(lang.split("-")) > 1 and lang.split("-")[1] not in LANGUAGES:
            lang = "pt"
    else:
        lang = "pt"
    return lang


@Client.on_message(filters.command(["print", "ss"]))
async def print(c: Client, m: Message):
    try:
        if m.reply_to_message and m.reply_to_message.text:
            url = m.reply_to_message.text
        elif m.text and m.text.split(maxsplit=1)[1]:
            url = m.text.split(maxsplit=1)[1]
    except IndexError:
        await m.reply_text(await tld(m.chat.id, "print_error"))
        return
    try:
        image_url = f"https://api.screenshotlayer.com/api/capture?access_key={SCREENSHOT_API_KEY}&url={url}&viewport=1920x1080&width=1000"
        await m.reply_photo(image_url)
    except ImageProcessFailed:
        await m.reply_text(tld(m.chat.id, "print_image_prcoessfailed"))
        return


@Client.on_message(filters.command(["tr", "tl"]))
async def translate(c: Client, m: Message):
    text = m.text[4:]
    lang = get_tr_lang(text)

    text = text.replace(lang, "", 1).strip() if text.startswith(lang) else text

    if not text and m.reply_to_message:
        text = m.reply_to_message.text or m.reply_to_message.caption

    if not text:
        return await m.reply_text(await tld(m.chat.id, "tr_error"))
    sent = await m.reply_text(await tld(m.chat.id, "tr_translating"))
    langs = {}

    if len(lang.split("-")) > 1:
        langs["sourcelang"] = lang.split("-")[0]
        langs["targetlang"] = lang.split("-")[1]
    else:
        to_lang = langs["targetlang"] = lang

    trres = await tr.translate(text, **langs)
    text = trres.text

    res = html.escape(text)
    await sent.edit_text(
        ("<b>{from_lang}</b> -> <b>{to_lang}:</b>\n<code>{translation}</code>").format(
            from_lang=trres.lang, to_lang=to_lang, translation=res
        )
    )


@Client.on_message(filters.command("dicio"))
async def dicio(c: Client, m: Message):
    txt = m.text.split(" ", 1)[1]
    a = dicioinformal.definicao(txt)["results"]
    if a:
        frase = f'<b>{a[0]["title"]}:</b>\n{a[0]["tit"]}\n\n<i>{a[0]["desc"]}</i>'
    else:
        frase = "sem resultado"
    await m.reply(frase)

plugin_name = "misc_name"
plugin_help = "misc_help"