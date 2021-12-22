import html
import httpx
import rapidjson
import dicioinformal

from gpytranslate import Translator

from smudge import SCREENSHOT_API_KEY
from smudge.locales.strings import tld
from smudge.utils import http

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


@Client.on_message(filters.command("short"))
async def short(c: Client, m: Message):
    if len(m.command) < 2:
        return await m.reply_text(await tld(m.chat.id, "short_error"))
    else:
        url = m.command[1]
        if not url.startswith("http"):
            url = "http://" + url
        try:
            short = m.command[2]
            shortRequest = await http.get(
                f"https://api.1pt.co/addURL?long={url}&short={short}"
            )
            info = rapidjson.loads(shortRequest.content)
            short = info["short"]
            return await m.reply_text(f"<code>https://1pt.co/{short}</code>")
        except IndexError:
            shortRequest = await http.get(f"https://api.1pt.co/addURL?long={url}")
            info = rapidjson.loads(shortRequest.content)
            short = info["short"]
            return await m.reply_text(f"<code>https://1pt.co/{short}</code>")
        except Exception as e:
            return await m.reply_text(f"<b>{e}</b>")


@Client.on_message(filters.command(["print", "ss"]))
async def prints(c: Client, message: Message):
    msg = message.text
    the_url = msg.split(" ", 1)
    wrong = False

    if len(the_url) == 1:
        if message.reply_to_message:
            the_url = message.reply_to_message.text
            if len(the_url) == 1:
                wrong = True
            else:
                the_url = the_url[1]
        else:
            wrong = True
    else:
        the_url = the_url[1]

    if wrong:
        await message.reply_text(
            "<b>Uso:</b> <code>/print https://example.com</code> - Tira uma captura de tela do site especificado."
        )
        return

    try:
        sent = await message.reply_text("Obtendo captura de tela...")
        res_json = await cssworker_url(target_url=the_url)
    except BaseException as e:
        await message.reply(f"**Failed due to:** `{e}`")
        return

    if res_json:
        # {"url":"image_url","response_time":"147ms"}
        image_url = res_json["url"]
        if image_url:
            try:
                await message.reply_photo(image_url)
                await sent.delete()
            except BaseException:
                # if failed to send the message, it's not API's
                # fault.
                # most probably there are some other kind of problem,
                # for example it failed to delete its message.
                # or the bot doesn't have access to send media in the chat.
                return
        else:
            await message.reply(
                "couldn't get url value, most probably API is not accessible."
            )
    else:
        await message.reply("Failed, because API is not responding, try it later.")


async def cssworker_url(target_url: str):
    url = "https://htmlcsstoimage.com/demo_run"
    my_headers = {
        "User-Agent": "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:89.0) Gecko/20100101 Firefox/89.0",
        "Accept": "*/*",
        "Accept-Language": "en-US,en;q=0.5",
        "Referer": "https://htmlcsstoimage.com/",
        "Content-Type": "application/json",
        "Origin": "https://htmlcsstoimage.com",
        "Alt-Used": "htmlcsstoimage.com",
        "Connection": "keep-alive",
    }

    data = {
        "html": "",
        "console_mode": "",
        "url": target_url,
        "css": "",
        "selector": "",
        "ms_delay": "",
        "render_when_ready": "false",
        "viewport_height": "1600",
        "viewport_width": "900",
        "google_fonts": "",
        "device_scale": "",
    }

    try:
        resp = await http.post(url, headers=my_headers, json=data)
        return resp.json()
    except httpx.NetworkError:
        return None


plugin_name = "misc_name"
plugin_help = "misc_help"
