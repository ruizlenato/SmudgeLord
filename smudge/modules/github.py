import html
from typing import Optional, List
import requests

from requests import get
from telegram.ext import CommandHandler, run_async, Filters, RegexHandler
from telegram import Message, Chat, Update, Bot, User, ParseMode, MAX_MESSAGE_LENGTH

import smudge.helper_funcs.git_api as api
import smudge.modules.sql.github_sql as sql

from smudge import dispatcher
from smudge.helper_funcs.chat_status import user_admin
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.modules.translations.strings import tld


# do not async
def getData(bot, update, url, index):
    chat = update.effective_chat
    if not api.getData(url):
        return "Invalid <user>/<repo> combo"
    recentRelease = api.getReleaseData(api.getData(url), index)
    if recentRelease is None:
        return "The specified release could not be found"
    author = api.getAuthor(recentRelease)
    authorUrl = api.getAuthorUrl(recentRelease)
    name = api.getReleaseName(recentRelease)
    assets = api.getAssets(recentRelease)
    releaseName = api.getReleaseName(recentRelease)
    (tld(chat.id, "repo_saved"))
    message = (tld(chat.id, "author_github")).format(authorUrl, author)
    message += (tld(chat.id, "release_name_github")).format(releaseName)
    for asset in assets:
        message += (tld(chat.id, "asset_github"))
        fileName = api.getReleaseFileName(asset)
        fileURL = api.getReleaseFileURL(asset)
        assetFile = "<a href='{}'>{}</a>".format(fileURL, fileName)
        sizeB = ((api.getSize(asset))/1024)/1024
        size = "{0:.2f}".format(sizeB)
        downloadCount = api.getDownloadCount(asset)
        message += assetFile + "\n"
        message += (tld(chat.id, "size_github")).format(size)
        message += (tld(chat.id, "download_count_github")
                    ).format(downloadCount)
    return message


# likewise, aux function, not async
def getRepo(bot, update, reponame):
    chat_id = update.effective_chat.id
    repo = sql.get_repo(str(chat_id), reponame)
    if repo:
        return repo.value, repo.backoffset
    return None, None


@run_async
def getRelease(bot: Bot, update: Update, args: List[str]):
    msg = update.effective_message
    chat = update.effective_chat
    if len(args) == 0:
        msg.reply_text(tld(chat.id, "github_releases_arguments"))
        return
    if(len(args) != 1 and not (len(args) == 2 and args[1].isdigit()) and not ("/" in args[0])):
        msg.reply_text("Please specify a valid combination of <user>/<repo>")
        return
    index = 0
    if len(args) == 2:
        index = int(args[1])
    url = args[0]
    text = getData(bot, update, url, index)
    msg.reply_text(text, parse_mode=ParseMode.HTML,
                   disable_web_page_preview=True)
    return


@run_async
def hashFetch(bot: Bot, update: Update):  # kanged from notes
    message = update.effective_message.text
    msg = update.effective_message
    fst_word = message.split()[0]
    no_hash = fst_word[1:]
    url, index = getRepo(bot, update, no_hash)
    if url is None and index is None:
        msg.reply_text("There was a problem parsing your request. Likely this is not a saved repo shortcut",
                       parse_mode=ParseMode.HTML, disable_web_page_preview=True)
        return
    text = getData(bot, update, url, index)
    msg.reply_text(text, parse_mode=ParseMode.HTML,
                   disable_web_page_preview=True)
    return


@run_async
def cmdFetch(bot: Bot, update: Update, args: List[str]):
    msg = update.effective_message
    chat = update.effective_chat
    if(len(args) != 1):
        msg.reply_text(tld(chat.id, "invalid_repo_github"))
        return
    url, index = getRepo(bot, update, args[0])
    if url is None and index is None:
        msg.reply_text("There was a problem parsing your request. Likely this is not a saved repo shortcut",
                       parse_mode=ParseMode.HTML, disable_web_page_preview=True)
        return
    text = getData(bot, update, url, index)
    msg.reply_text(text, parse_mode=ParseMode.HTML,
                   disable_web_page_preview=True)
    return


@run_async
def changelog(bot: Bot, update: Update, args: List[str]):
    msg = update.effective_message
    chat = update.effective_chat
    if(len(args) != 1):
        msg.reply_text(tld(chat.id, "invalid_repo_github"))
        return
    url, index = getRepo(bot, update, args[0])
    if not api.getData(url):
        msg.reply_text("Invalid <user>/<repo> combo")
        return
    data = api.getData(url)
    release = api.getReleaseData(data, index)
    body = api.getBody(release)
    msg.reply_text(body)
    return


@run_async
@user_admin
def saveRepo(bot: Bot, update: Update, args: List[str]):
    chat_id = update.effective_chat.id
    msg = update.effective_message
    chat = update.effective_chat
    if(len(args) != 2 and (len(args) != 3 and not args[2].isdigit()) or not ("/" in args[1])):
        msg.reply_text(
            "Invalid data, use <reponame> <user>/<repo> <value (optional)>")
        return
    index = 0
    if len(args) == 3:
        index = int(args[2])
    sql.add_repo_to_db(str(chat_id), args[0], args[1], index)
    msg.reply_text(tld(chat.id, "repo_saved"))
    return


@run_async
@user_admin
def delRepo(bot: Bot, update: Update, args: List[str]):
    chat_id = update.effective_chat.id
    msg = update.effective_message
    if(len(args) != 1):
        msg.reply_text(tld(chat.id, "invalid_repo_github"))
        return
    sql.rm_repo(str(chat_id), args[0])
    msg.reply_text("Repo shortcut deleted successfully!")
    return


@run_async
def listRepo(bot: Bot, update: Update):
    chat_id = update.effective_chat.id
    chat = update.effective_chat
    chat_name = chat.title or chat.first or chat.username
    repo_list = sql.get_all_repos(str(chat_id))
    msg = (tld(chat.id, "github_sortcuts")).format(chat_name)
    des = (tld(chat.id, "github_help_list"))
    for repo in repo_list:
        repo_name = (" • <code>&{}</code>\n".format(repo.name))
        if len(msg) + len(repo_name) > MAX_MESSAGE_LENGTH:
            update.effective_message.reply_text(msg, parse_mode=ParseMode.HTML)
            msg = ""
        msg += repo_name
    if msg == "<b>List of repo shotcuts in {}:</b>\n":
        update.effective_message.reply_text("No repo shortcuts in this chat!")
    elif len(msg) != 0:
        update.effective_message.reply_text(msg.format(
            chat_name) + des, parse_mode=ParseMode.HTML)


def getVer(bot: Bot, update: Update):
    msg = update.effective_message
    ver = api.vercheck()
    msg.reply_text("GitHub API version: "+ver)
    return


@run_async
def repo(bot: Bot, update: Update, args: List[str]):
    message = update.effective_message
    text = message.text[len('/repo '):]
    usr = get(f'https://api.github.com/users/{text}/repos?per_page=40').json()
    reply_text = "*Repo*\n"
    for i in range(len(usr)):
        reply_text += f"[{usr[i]['name']}]({usr[i]['html_url']})\n"
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)

# GitHub module. This module will help you to fetch GitHub releases.
#
# *Available Commands:*
# - /git <user>/<repo>: will fetch the most recent release from that repo.
# - /git <user>/<repo> <number>: will fetch releases in past.
# - /fetch <reponame> or &reponame: same as /git, but you can use a saved repo shortcut
# - /listrepo: lists all repo shortcuts in chat
# - /gitver: returns the current API version
# - /changelog <reponame>: gets the changelog of a saved repo shortcut
#
# *Admin only:*
# - /saverepo <name> <user>/<repo> <number (optional)>: saves a repo value as shortcut
# - /delrepo <name>: deletes a repo shortcut


__help__ = True

REPO_HANDLER = DisableAbleCommandHandler("repo",
                                         repo,
                                         pass_args=True,
                                         admin_ok=True)
RELEASE_HANDLER = DisableAbleCommandHandler("gitr", getRelease, pass_args=True,
                                            admin_ok=True)
FETCH_HANDLER = DisableAbleCommandHandler("fetch", cmdFetch, pass_args=True,
                                          admin_ok=True)
SAVEREPO_HANDLER = DisableAbleCommandHandler(
    "saverepo", saveRepo, pass_args=True)
DELREPO_HANDLER = DisableAbleCommandHandler("delrepo", delRepo, pass_args=True)
LISTREPO_HANDLER = DisableAbleCommandHandler("listrepo", listRepo,
                                             admin_ok=True)
CHANGELOG_HANDLER = DisableAbleCommandHandler("changelog", changelog,
                                              pass_args=True,
                                              admin_ok=True)

HASHFETCH_HANDLER = RegexHandler(r"^&[^\s]+", hashFetch)

dispatcher.add_handler(RELEASE_HANDLER)
dispatcher.add_handler(FETCH_HANDLER)
dispatcher.add_handler(SAVEREPO_HANDLER)
dispatcher.add_handler(DELREPO_HANDLER)
dispatcher.add_handler(LISTREPO_HANDLER)
dispatcher.add_handler(HASHFETCH_HANDLER)
dispatcher.add_handler(CHANGELOG_HANDLER)
dispatcher.add_handler(REPO_HANDLER)
