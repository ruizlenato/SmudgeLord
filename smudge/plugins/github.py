# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

from requests import get
from ujson import loads

from smudge.database.core import groups
from smudge.plugins import tld

from pyrogram.types import Message
from pyrogram import Client, filters
from pyrogram.types import InlineKeyboardButton, InlineKeyboardMarkup

from tortoise.exceptions import DoesNotExist, IntegrityError


async def add_repo(chat_id: int, git_repo_name: str, git_repo: str):
    check_repo_exists = await groups.exists(
        id=chat_id, git_repo_name=git_repo_name, git_repo=git_repo
    )
    try:
        if check_repo_exists:
            await groups.filter(id=chat_id).update(
                git_repo_name=git_repo_name, git_repo=git_repo
            )
            return True
        else:
            await groups.filter(id=chat_id).update(
                git_repo_name=git_repo_name, git_repo=git_repo
            )
            return False
    except IntegrityError:
        return


async def del_repo(chat_id: int, git_repo_name: str):
    try:
        return await groups.filter(id=chat_id, git_repo_name=git_repo_name).delete()
    except DoesNotExist:
        return False


async def get_repo(chat_id: int, git_repo_name: str):
    try:
        return (
            await groups.get(
                id=chat_id,
                git_repo_name=git_repo_name,
            )
        ).git_repo
    except DoesNotExist:
        return None


async def get_repos(chat_id: int):
    return await groups.filter(id=chat_id)


@Client.on_message(filters.command(["gitr"]))
async def git_on_message(c: Client, m: Message):
    if not len(m.command) == 2:
        await m.reply_text(await tld(m, "GitHub.gitr_noargs"))
        return
    repo = m.command[1]
    page = await check_repo(repo)
    if not page:
        await m.reply_text((await tld(m, "GitHub.repo_errorreleases")).format(repo))

    else:
        await git(c, m, repo, page)


@Client.on_message(filters.command(["repos"]) & filters.group)
async def git_repos(c: Client, m: Message):
    repos = await get_repos(m.chat.id)
    for i in repos:
        keyword = i.git_repo_name

        if keyword is None:
            await m.reply_text(await tld(m, "GitHub.nothing_save"))
        else:
            message = (await tld(m, "GitHub.repos_saved")).format(keyword)
            message += await tld(m, "GitHub.repos_savedhelp")
            await m.reply_text(message)


@Client.on_message(filters.command(["fetch"]) & filters.group)
@Client.on_message(filters.regex(pattern=r"^&\w*") & filters.group)
async def fetch_repo(c: Client, m: Message):
    try:
        repo = m.command[1]
    except TypeError:
        repo = m.text[1:]
    repo_db = await get_repo(m.chat.id, repo)
    if not await get_repo(m.chat.id, repo):
        message = "Repo <b>{}</b> doesn't exist!".format(repo)
        return await m.reply_text(message)
    else:
        page = get(f"https://api.github.com/repos/{repo_db}/releases/latest")
        if not page.status_code == 200:
            await m.reply_text((await tld(m, "GitHub.repo_errorreleases")).format(repo))

        else:
            await git(c, m, repo_db, page)


@Client.on_message(filters.command(["gitadd"]) & filters.group)
async def save_repo(c: Client, m: Message):
    if not len(m.command) == 3:
        await m.reply(await tld(m, "GitHub.add_noargs"))
        return
    name = m.command[1]
    repo = m.command[2]
    page = await check_repo(repo)
    if not page:
        await m.reply(await tld(m, "GitHub.repo_noreleases"))
        return
    msg = await tld(m, "GitHub.repo_added")
    if await add_repo(m.chat.id, name, repo) is True:
        message = msg.format(await tld(m, "Main.updated"), name)
    else:
        message = msg.format(await tld(m, "Main.added"), name)
    await m.reply_text(message)


@Client.on_message(filters.command(["gitdel"]) & filters.group)
async def rm_repo(c: Client, m: Message):
    name = m.text.split(maxsplit=1)[1]
    if await del_repo(m.chat.id, name) is False:
        message = await tld(m, "GitHub.repo_faildelete")
    else:
        message = await tld(m, "GitHub.repo_deleted")
    await m.reply_text(message.format(name))


async def check_repo(repo):
    page = get(f"https://api.github.com/repos/{repo}/releases/latest")
    if not page.status_code == 200:
        return False

    else:
        return page


async def git(c: Client, m: Message, repo, page):
    db = loads(page.content)
    name = db["name"]
    date = db["published_at"]
    tag = db["tag_name"]
    date = db["published_at"]
    changelog = db["body"]
    dev, repo = repo.split("/")
    message = "<b>Name:</b> <code>{}</code>\n".format(name)
    message += "<b>Tag:</b> <code>{}</code>\n".format(tag)
    message += "<b>Released on:</b> <code>{}</code>\n".format(date[: date.rfind("T")])
    message += "<b>By:</b> <code>{}@github.com</code>\n".format(dev)
    message += "<b>Changelog:</b>\n<code>{}</code>\n\n".format(changelog)
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
    )


plugin_name = "GitHub.name"
plugin_help = "GitHub.help"
