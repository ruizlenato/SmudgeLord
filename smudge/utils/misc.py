import httpx
import orjson

from smudge.utils import http, aiowrap

from bs4 import BeautifulSoup

dicio_link = "https://www.dicionarioinformal.com.br/"

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


async def cssworker_url(target_url: str):
    url = "https://htmlcsstoimage.com/demo_run"
    my_headers = {
        "User-Agent": "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/48.0.2564.97 Safari/537.36",
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
        "viewport_height": "900",
        "viewport_width": "1600",
        "google_fonts": "",
        "device_scale": "",
    }

    try:
        return orjson.loads(
            (await http.post(url, headers=my_headers, json=data)).content
        )
    except httpx.NetworkError:
        return None


async def dicio_def(query):
    r = await http.get(dicio_link + query, follow_redirects=True)
    soup = BeautifulSoup(r.text, "html.parser")
    tit = soup.find_all("h3", "di-blue")
    if tit is None:
        tit = soup.find_all("h3", "di-blue-link")
    title = []
    for i in tit:
        a = i.find("a")
        if a != None:
            title.append(a.get("title"))
    if a is None:
        tit = soup.find_all("h3", "di-blue-link")
    for i in tit:
        a = i.find("a")
        if a != None:
            title.append(f'você quiz dizer: {a.get("title")}')
    ti = soup.find_all("p", "text-justify")
    tit = []
    for i in ti:
        ti = i.get_text()[17:].replace("""\n                """, "")
        tit.append(ti)
    des = soup.find_all("blockquote", "text-justify")
    des.append(" ")
    desc = []
    for i in des:
        try:
            des = (
                i.get_text()
                .replace("\n"[0], "")
                .replace("                 ", "")
                .replace("""\n                """, "")
            )
        except:
            if i == " ":
                des = ""
        desc.append(des)
    result = []
    max = 0
    for i in title:
        try:
            b = {
                "title": i.replace("\t", ""),
                "tit": tit[max].replace("\t", ""),
                "desc": desc[max].replace("\t", "").replace("                ", ""),
            }
            max += 1
            result.append(b)
        except:
            pass

    return result
