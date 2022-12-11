# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
import regex
import html
import json

from typing import Union
from gpytranslate import Translator

from ..bot import Smudge
from ..utils import http
from ..utils.locales import tld
from ..utils.misc import get_tr_lang, cssworker_url, dicio_def

from pyrogram import filters
from pyrogram.helpers import ikb
from pyrogram.types import Message, CallbackQuery

# Translator
tr = Translator()


@Smudge.on_message(filters.command(["tr", "tl"]))
async def translate(c: Smudge, m: Message):
    text = m.text[4:]
    lang = get_tr_lang(text)

    text = text.replace(lang, "", 1).strip() if text.startswith(lang) else text

    if not text and m.reply_to_message:
        text = m.reply_to_message.text or m.reply_to_message.caption

    if not text:
        return await m.reply_text(await tld(m, "Misc.noargs_tr"))
    sent = await m.reply_text(await tld(m, "Misc.tr_translating"))
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
        f"<b>{trres.lang}</b> -> <b>{to_lang}:</b>\n<code>{res}</code>"
    )


@Smudge.on_message(filters.regex(r"^s/(.+)?/(.+)?(/.+)?") & filters.reply)
async def sed(c: Smudge, m: Message):
    exp = regex.split(r"(?<![^\\]\\)/", m.text)
    pattern = exp[1]
    replace_with = exp[2].replace(r"\/", "/")
    rflags = 0

    flags = exp[3] if len(exp) > 3 else ""
    count = 0 if "g" in flags else 1
    if "i" in flags and "s" in flags:
        rflags = regex.I | regex.S
    elif "i" in flags:
        rflags = regex.I
    elif "s" in flags:
        rflags = regex.S

    text = m.reply_to_message.text or m.reply_to_message.caption

    if not text:
        return

    try:
        res = regex.sub(
            pattern, replace_with, text, count=count, flags=rflags, timeout=1
        )
    except TimeoutError:
        await m.reply_text(await tld(m, "Misc.regex_timeout"))
    except regex.error as e:
        await m.reply_text(str(e))
    else:
        await c.send_message(
            m.chat.id,
            f"{html.escape(res)}",
            reply_to_message_id=m.reply_to_message.id,
        )


@Smudge.on_message(filters.command("dicio"))
async def dicio(c: Smudge, m: Message):
    txt = m.text.split(" ", 1)[1]
    if a := await dicio_def(txt):
        frase = f'<b>{a[0]["title"]}:</b>\n{a[0]["tit"]}\n\n<i>{a[0]["desc"]}</i>'
    else:
        frase = "sem resultado"
    await m.reply(frase)


@Smudge.on_message(filters.command(["print", "ss"]))
async def prints(c: Smudge, m: Message):
    msg = m.text
    the_url = msg.split(" ", 1)
    wrong = False

    if len(the_url) == 1:
        wrong = True
    else:
        the_url = the_url[1]

    if wrong:
        await m.reply_text(await tld(m, "Misc.noargs_print"))
        return

    try:
        sent = await m.reply_text(await tld(m, "Misc.print_printing"))
        res_json = await cssworker_url(target_url=the_url)
    except BaseException as e:
        await m.reply(f"<b>Error:</b> <code>{e}</code>")
        return

    if res_json:
        if image_url := res_json["url"]:
            try:
                await m.reply_photo(image_url)
                await sent.delete()
            except BaseException as e:
                await m.reply(f"<b>Error:</b> <code>{e}</code>")
                return
        else:
            await m.reply(
                "couldn't get url value, most probably API is not accessible."
            )
    else:
        await m.reply(await tld(m, "Misc.print_api_dead"))


@Smudge.on_message(filters.command("cep"))
async def cep(c: Smudge, m: Message):
    try:
        if len(m.command) > 1:
            cep = m.text.split(None, 1)[1]
        elif m.reply_to_message and m.reply_to_message.text:
            cep = m.reply_to_message.text
    except IndexError:
        await m.reply_text(await tld(m, "Misc.noargs_cep"))
        return

    base_url = "https://brasilapi.com.br/api/cep/v1"
    res = await http.get(f"{base_url}/{cep}")
    db = json.loads(res.content)
    try:
        city = db["city"]
        state = db["state"]
    except KeyError:
        return await m.reply_text((await tld(m, "Misc.cep_error")))
    state_name = json.loads(
        (await http.get(f"https://brasilapi.com.br/api/ibge/uf/v1/{state}")).content
    )["nome"]
    if res.status_code == 404:
        return await m.reply_text((await tld(m, "Misc.cep_error")))
    neighborhood = db["neighborhood"]
    street = db["street"]

    rep = (await tld(m, "Misc.cep_strings")).format(
        cep, city, state_name, state, neighborhood, street
    )
    await m.reply_text(rep)


@Smudge.on_message(filters.command("ddd"))
@Smudge.on_callback_query(filters.regex("ddd_(?P<num>.+)"))
async def ddd(c: Smudge, m: Union[Message, CallbackQuery]):
    try:
        if isinstance(m, CallbackQuery):
            ddd = m.matches[0]["num"]
        else:
            ddd = m.text.split(maxsplit=1)[1]
    except IndexError:
        await m.reply_text(await tld(m, "Misc.noargs_ddd"))
        return
    res = await http.get(f"https://brasilapi.com.br/api/ddd/v1/{ddd}")
    db = json.loads(res.content)
    try:
        state = db["state"]
    except KeyError:
        return await m.reply_text((await tld(m, "Misc.ddd_error")))
    if res.status_code == 404:
        return await m.reply_text((await tld(m, "Misc.ddd_error")))
    state_name = json.loads(
        (await http.get(f"https://brasilapi.com.br/api/ibge/uf/v1/{state}")).content
    )["nome"]
    cities = db["cities"]
    if isinstance(m, CallbackQuery):
        cities.reverse()
        cities = (
            str(cities)
            .replace("'", "")
            .replace("]", "")
            .replace("[", "")
            .lower()
            .title()
        )
        await m.edit_message_text(
            (await tld(m, "Misc.fddd_strings")).format(ddd, state_name, state, cities)
        )
    else:
        rep = (await tld(m, "Misc.ddd_strings")).format(ddd, state_name, state)
        keyboard = [[(await tld(m, "Misc.ddd_cities"), f"ddd_{ddd}")]]
        await m.reply_text(rep, reply_markup=ikb(keyboard))


@Smudge.on_message(filters.command(["gitr", "ghr"]))
async def git_on_message(c: Smudge, m: Message):
    if len(m.command) != 2:
        await m.reply_text(await tld(m, "Misc.noargs_gitr"))
        return
    repo = m.command[1]
    page = await http.get(f"https://api.github.com/repos/{repo}/releases/latest")
    if page.status_code != 200:
        return await m.reply_text((await tld(m, "Misc.gitr_noreleases")).format(repo))
    else:
        await git(c, m, repo, page)


async def git(c: Smudge, m: Message, repo, page):
    db = json.loads(page.content)
    date = db["published_at"]
    message = (
        f"<b>Name:</b> <i>{db['name']}</i>\n"
        + f"<b>Tag:</b> <i>{db['tag_name']}</i>\n"
        + f"<b>Released on:</b> <i>{date[: date.rfind('T')]}</i>\n"
        + f"<b>By:</b> <i>{repo.split('/')[0]}@github.com</i>\n"
    )
    keyboard = []
    for i in range(len(db)):
        try:
            file_name = db["assets"][i]["name"]
            url = db["assets"][i]["browser_download_url"]
            dls = db["assets"][i]["download_count"]
            size_bytes = db["assets"][i]["size"]
            size = float("{:.2f}".format((size_bytes / 1024) / 1024))
            text = f"{file_name}\n💾 {size}MB | 📥 {dls}"
            keyboard += [[(text, url, "url")]]

        except IndexError:
            continue
    await m.reply_text(message, reply_markup=ikb(keyboard))


__help__ = "Misc"
