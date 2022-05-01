# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
from requests import get
from ujson import loads

from smudge import Smudge
from smudge.utils import http
from smudge.plugins import tld

from pyrogram import filters
from pyrogram.types import Message
from pyrogram.enums import ParseMode
from pyrogram.types import InlineKeyboardButton, InlineKeyboardMarkup


@Smudge.on_message(filters.command(["gitr", "ghr"]))
async def git_on_message(c: Smudge, m: Message):
    if not len(m.command) == 2:
        await m.reply_text(await tld(m, "Misc.gitr_noargs"))
        return
    repo = m.command[1]
    page = await check_repo(repo)
    if not page:
        await m.reply_text((await tld(m, "Misc.repo_errorreleases")).format(repo))

    else:
        await git(c, m, repo, page)


async def check_repo(repo):
    page = await http.get(f"https://api.github.com/repos/{repo}/releases/latest")
    if not page.status_code == 200:
        return False

    else:
        return page


async def git(c: Smudge, m: Message, repo, page):
    db = loads(page.content)
    name = db["name"]
    date = db["published_at"]
    tag = db["tag_name"]
    date = db["published_at"]
    changelog = db["body"]
    dev, repo = repo.split("/")
    message = "**Name:** `{}`\n".format(name)
    message += "**Tag:** `{}`\n".format(tag)
    message += "**Released on:** `{}`\n".format(date[: date.rfind("T")])
    message += "**By:** `{}@github.com`\n".format(dev)
    message += "**Changelog:**\n{}\n\n".format(changelog)
    keyboard = []
    for i in range(len(db)):
        try:
            file_name = db["assets"][i]["name"]
            url = db["assets"][i]["browser_download_url"]
            dls = db["assets"][i]["download_count"]
            size_bytes = db["assets"][i]["size"]
            size = float("{:.2f}".format((size_bytes / 1024) / 1024))
            text = "{}\n💾 {}MB | 📥 {}".format(file_name, size, dls)
            keyboard += [[InlineKeyboardButton(text=text, url=url)]]
        except IndexError:
            continue
    await m.reply_text(
        text=message,
        reply_markup=InlineKeyboardMarkup(keyboard),
        disable_web_page_preview=True,
        parse_mode=ParseMode.MARKDOWN,
    )
