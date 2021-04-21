#    SmudgeLord (A telegram bot project)
#    Copyright (C) 2017-2019 Paul Larsen
#    Copyright (C) 2019-2021 A Haruka Aita and Intellivoid Technologies project
#    Copyright (C) 2021 Renatoh

#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU Affero General Public License as published by
#    the Free Software Foundation, either version 3 of the License, or
#    (at your option) any later version.

#    You should have received a copy of the GNU Affero General Public License
#    along with this program.  If not, see <https://www.gnu.org/licenses/>.

import re
import html
import pafy
import sys
import os
import wikipedia
from datetime import datetime
import urllib.parse as urlparse
from typing import Optional, List
import urllib.parse as urlparse
from covid import Covid

import requests
import urllib.request
from telegram import Message, Chat, Update, Bot, MessageEntity
from telegram import ParseMode, ReplyKeyboardRemove, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import CommandHandler, CallbackContext, run_async, Filters
from telegram.utils.helpers import escape_markdown, mention_html
from telegram.error import BadRequest

from smudge import dispatcher, OWNER_ID, SUDO_USERS, WHITELIST_USERS, sw, SCREENSHOT_API_KEY
from smudge.__main__ import GDPR
from smudge.__main__ import STATS, USER_INFO
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.helper_funcs.extraction import extract_user

from smudge.modules.translations.strings import tld

from requests import get

cvid = Covid(source="worldometers")


def screenshot(update: Update, context: CallbackContext):
    args = context.args
    bot = context.bot
    msg = update.effective_message
    chat = update.effective_chat
    filename = "screencapture.png"
    if args:
        user = update.effective_user.id
        txt = " ".join(args)
        image_url = f"https://api.screenshotlayer.com/api/capture?access_key={SCREENSHOT_API_KEY}&url={args}&fullpage=1&viewport=2560x1440&format=PNG&force=1"
        if not SCREENSHOT_API_KEY:
            msg.reply_text(tld(chat.id, "lastfm_usernotset"))
            return
        urllib.request.urlretrieve(image_url, filename)
        dispatcher.bot.send_document(chat_id=chat.id,  document=open(
            'screencapture.png', 'rb'), caption=txt)
    else:
        msg.reply_text(tld(chat.id, "lastfm_usernotset"))


def get_bot_ip(update: Update, context: CallbackContext):
    res = requests.get("http://ipinfo.io/ip")
    update.message.reply_text(res.text)


def get_id(update: Update, context: CallbackContext):
    args = context.args
    bot = context.bot
    user_id = extract_user(update.effective_message, args)
    chat = update.effective_chat  # type: Optional[Chat]
    if user_id:
        if update.effective_message.reply_to_message and update.effective_message.reply_to_message.forward_from:
            user1 = update.effective_message.reply_to_message.from_user
            user2 = update.effective_message.reply_to_message.forward_from
            update.effective_message.reply_markdown(tld(chat.id, "misc_get_id_1").format(escape_markdown(user2.first_name), user2.id,
                                                                                         escape_markdown(user1.first_name), user1.id))
        else:
            user = bot.get_chat(user_id)
            update.effective_message.reply_markdown(tld(chat.id, "misc_get_id_2").format(escape_markdown(user.first_name),
                                                                                         user.id))
    else:
        chat = update.effective_chat  # type: Optional[Chat]
        if chat.type == "private":
            update.effective_message.reply_markdown(
                tld(chat.id, "misc_id_1").format(chat.id))

        else:
            update.effective_message.reply_markdown(
                tld(chat.id, "misc_id_2").format(chat.id))


def info(update: Update, context: CallbackContext):
    args = context.args
    bot = context.bot
    msg = update.effective_message  # type: Optional[Message]
    user_id = extract_user(update.effective_message, args)
    chat = update.effective_chat  # type: Optional[Chat]

    if user_id:
        user = bot.get_chat(user_id)

    elif not msg.reply_to_message and not args:
        user = msg.from_user

    elif not msg.reply_to_message and (
            not args or
        (len(args) >= 1 and not args[0].startswith("@")
         and not args[0].isdigit()
         and not msg.parse_entities([MessageEntity.TEXT_MENTION]))):
        msg.reply_text(tld(chat.id, "misc_info_extract_error"))
        return

    else:
        return

    text = tld(chat.id, "misc_info_1")
    text += tld(chat.id, "misc_info_id").format(user.id)
    text += tld(chat.id,
                "misc_info_first").format(html.escape(user.first_name))

    if user.last_name:
        text += tld(chat.id,
                    "misc_info_name").format(html.escape(user.last_name))

    if user.username:
        text += tld(chat.id,
                    "misc_info_username").format(html.escape(user.username))

    text += tld(chat.id,
                "misc_info_user_link").format(mention_html(user.id, "link"))

    try:
        spamwatch = sw.get_ban(int(user.id))
        if spamwatch:
            text += tld(chat.id, "misc_info_swban1")
            text += tld(chat.id, "misc_info_swban2").format(spamwatch.reason)
            text += tld(chat.id, "misc_info_swban3")
        else:
            pass
    except:
        pass

    if user.id == OWNER_ID:
        text += tld(chat.id, "misc_info_is_owner")
    else:
        if user.id == int(254318997):
            text += tld(chat.id, "misc_info_is_original_owner")

        if user.id in SUDO_USERS:
            text += tld(chat.id, "misc_info_is_sudo")
        else:
            if user.id in WHITELIST_USERS:
                text += tld(chat.id, "misc_info_is_whitelisted")

    for mod in USER_INFO:

        try:
            mod_info = mod.__user_info__(user.id)
        except TypeError:
            mod_info = mod.__user_info__(user.id, chat.id)
        if mod_info:
            text += "\n" + mod_info

    update.effective_message.reply_text(text, parse_mode=ParseMode.HTML)


def echo(update: Update, context: CallbackContext):
    message = update.effective_message
    message.delete()
    args = update.effective_message.text.split(None, 1)
    if message.reply_to_message:
        message.reply_to_message.reply_text(args[1])
    else:
        message.reply_text(args[1], quote=False)


def reply_keyboard_remove(update: Update, context: CallbackContext):
    reply_keyboard = []
    reply_keyboard.append([ReplyKeyboardRemove(remove_keyboard=True)])
    reply_markup = ReplyKeyboardRemove(remove_keyboard=True)
    old_message = bot.send_message(
        chat_id=update.message.chat_id,
        text='trying',  # This text will not get translated
        reply_markup=reply_markup,
        reply_to_message_id=update.message.message_id)
    bot.delete_message(chat_id=update.message.chat_id,
                       message_id=old_message.message_id)


def gdpr(update: Update, context: CallbackContext):
    update.effective_message.reply_text(
        tld(update.effective_chat.id, "misc_gdpr"))
    for mod in GDPR:
        mod.__gdpr__(update.effective_user.id)

    update.effective_message.reply_text(tld(update.effective_chat.id,
                                            "send_gdpr"),
                                        parse_mode=ParseMode.MARKDOWN)


def markdown_help(update: Update, context: CallbackContext):
    chat = update.effective_chat  # type: Optional[Chat]
    update.effective_message.reply_text(tld(chat.id, "misc_md_list"),
                                        parse_mode=ParseMode.HTML)
    update.effective_message.reply_text(
        tld(chat.id, "misc_md_try"))
    update.effective_message.reply_text(
        tld(
            chat.id, "misc_md_help"))


def stats(update: Update, context: CallbackContext):
    update.effective_message.reply_text(
        # This text doesn't get translated as it is internal message.
        "*Current Stats:*\n" + "\n".join([mod.__stats__() for mod in STATS]),
        parse_mode=ParseMode.MARKDOWN)


def github(update: Update, context: CallbackContext):
    message = update.effective_message
    text = message.text[len('/git '):]
    usr = get(f'https://api.github.com/users/{text}').json()
    if usr.get('login'):
        text = f"*Username:* [{usr['login']}](https://github.com/{usr['login']})"

        whitelist = [
            'name', 'id', 'type', 'location', 'blog', 'bio', 'followers',
            'following', 'hireable', 'public_gists', 'public_repos', 'email',
            'company', 'updated_at', 'created_at'
        ]

        difnames = {
            'id': 'Account ID',
            'type': 'Account type',
            'created_at': 'Account created at',
            'updated_at': 'Last updated',
            'public_repos': 'Public Repos',
            'public_gists': 'Public Gists'
        }

        goaway = [None, 0, 'null', '']

        for x, y in usr.items():
            if x in whitelist:
                if x in difnames:
                    x = difnames[x]
                else:
                    x = x.title()

                if x == 'Account created at' or x == 'Last updated':
                    y = datetime.strptime(y, "%Y-%m-%dT%H:%M:%SZ")

                if y not in goaway:
                    if x == 'Blog':
                        x = "Website"
                        y = f"[Here!]({y})"
                        text += ("\n*{}:* {}".format(x, y))
                    else:
                        text += ("\n*{}:* `{}`".format(x, y))
        reply_text = text
    else:
        reply_text = "User not found. Make sure you entered valid username!"
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


def ud(update: Update, context: CallbackContext):
    message = update.effective_message
    text = message.text[len('/ud '):]
    results = get(
        f'http://api.urbandictionary.com/v0/define?term={text}').json()
    definition = f'{results["list"][0]["definition"]}'
    definition1 = definition.replace("[", "").replace("]", "")
    exemple = f'{results["list"][0]["example"]}'
    exemple1 = exemple.replace("[", "").replace("]", "")
    reply_text = f'<strong>{text}</strong>\n<strong>Definition:</strong> {definition1}\n\n<strong>Exemple: </strong>{exemple1}'
    message.reply_text(reply_text, parse_mode=ParseMode.HTML)


def wiki(update: Update, context: CallbackContext):
    chat_id = update.effective_chat.id
    msg = update.effective_message

    msg.reply_text(tld(chat_id, "wiki_lang"), parse_mode=ParseMode.HTML)


def wikien(update: Update, context: CallbackContext):
    bot = context.bot
    chat_id = update.effective_chat.id
    kueri = re.split(pattern="wikien", string=update.effective_message.text)
    wikipedia.set_lang("en")
    if len(str(kueri[1])) == 0:
        update.effective_message.reply_text("Enter keywords!")
    else:
        try:
            pertama = update.effective_message.reply_text("🔄 Loading...")
            keyboard = InlineKeyboardMarkup([[
                InlineKeyboardButton(text="🔧 More Info...",
                                     url=wikipedia.page(kueri).url)
            ]])
            textresult = tld(chat_id, "wiki_result").format(
                wikipedia.summary(kueri, sentences=3))
            bot.editMessageText(chat_id=update.effective_chat.id,
                                message_id=pertama.message_id,
                                text=textresult,
                                reply_markup=keyboard,
                                parse_mode=ParseMode.HTML)
        except wikipedia.PageError as e:
            update.effective_message.reply_text("⚠ Error: {}".format(e))
        except BadRequest as et:
            update.effective_message.reply_text("⚠ Error: {}".format(et))
        except wikipedia.exceptions.DisambiguationError as eet:
            update.effective_message.reply_text(
                "⚠ Error\n There are too many query! Express it more!\nPossible query result:\n{}".format(
                    eet)
            )


def wikipt(update: Update, context: CallbackContext):
    bot = context.bot
    chat_id = update.effective_chat.id
    kueri = re.split(pattern="wikipt", string=update.effective_message.text)
    wikipedia.set_lang("pt")
    if len(str(kueri[1])) == 0:
        update.effective_message.reply_text(
            "Escreva o que você quer procurar no Wikipedia!")
    else:
        try:
            pertama = update.effective_message.reply_text("🔄 Carregando...")
            keyboard = InlineKeyboardMarkup([[
                InlineKeyboardButton(text="🔧 Mais Informações...",
                                     url=wikipedia.page(kueri).url)
            ]])
            textresult = tld(chat_id, "wiki_result").format(
                wikipedia.summary(kueri, sentences=2))
            bot.editMessageText(chat_id=update.effective_chat.id,
                                message_id=pertama.message_id,
                                text=textresult,
                                reply_markup=keyboard,
                                parse_mode=ParseMode.HTML)
        except wikipedia.PageError as e:
            update.effective_message.reply_text("⚠ Erro: {}".format(e))
        except BadRequest as et:
            update.effective_message.reply_text("⚠ Erro: {}".format(et))
        except wikipedia.exceptions.DisambiguationError as eet:
            update.effective_message.reply_text(
                "⚠ Error\n Há muitas coisa! Expresse melhor para achar o resultado!\nPossíveis resultados da consulta:\n{}".format(
                    eet)
            )


def github(update: Update, context: CallbackContext):
    message = update.effective_message
    text = message.text[len('/git '):]
    usr = get(f'https://api.github.com/users/{text}').json()
    if usr.get('login'):
        text = f"*Username:* [{usr['login']}](https://github.com/{usr['login']})"

        whitelist = [
            'name', 'id', 'type', 'location', 'blog', 'bio', 'followers',
            'following', 'hireable', 'public_gists', 'public_repos', 'email',
            'company', 'updated_at', 'created_at'
        ]

        difnames = {
            'id': 'Account ID',
            'type': 'Account type',
            'created_at': 'Account created at',
            'updated_at': 'Last updated',
            'public_repos': 'Public Repos',
            'public_gists': 'Public Gists'
        }

        goaway = [None, 0, 'null', '']

        for x, y in usr.items():
            if x in whitelist:
                if x in difnames:
                    x = difnames[x]
                else:
                    x = x.title()

                if x == 'Account created at' or x == 'Last updated':
                    y = datetime.strptime(y, "%Y-%m-%dT%H:%M:%SZ")

                if y not in goaway:
                    if x == 'Blog':
                        x = "Website"
                        y = f"[Here!]({y})"
                        text += ("\n*{}:* {}".format(x, y))
                    else:
                        text += ("\n*{}:* `{}`".format(x, y))
        reply_text = text
    else:
        reply_text = "User not found. Make sure you entered valid username!"
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


def covid(update: Update, context: CallbackContext):
    message = update.effective_message
    chat = update.effective_chat
    country = str(message.text[len('/covid '):])
    if country == '':
        country = "world"
    if country.lower() in ["south korea", "korea"]:
        country = "s. korea"
    try:
        c_case = cvid.get_status_by_country_name(country)
    except Exception:
        message.reply_text(tld(chat.id, "misc_covid_error"))
        return
    active = format_integer(c_case["active"])
    confirmed = format_integer(c_case["confirmed"])
    country = c_case["country"]
    critical = format_integer(c_case["critical"])
    deaths = format_integer(c_case["deaths"])
    new_cases = format_integer(c_case["new_cases"])
    new_deaths = format_integer(c_case["new_deaths"])
    recovered = format_integer(c_case["recovered"])
    total_tests = c_case["total_tests"]
    if total_tests == 0:
        total_tests = "N/A"
    else:
        total_tests = format_integer(c_case["total_tests"])
    reply = tld(chat.id,
                "misc_covid").format(country, confirmed, new_cases, active,
                                     critical, deaths, new_deaths, recovered,
                                     total_tests)
    message.reply_markdown(reply)


def outline(update: Update, context: CallbackContext):
    args = context.args
    message = update.effective_message
    chat = update.effective_chat
    if args:
        link = " ".join(args)
    else:
        message.reply_text(tld(chat.id, "misc_paste_invalid"))
        return
    if urlparse.urlparse(link).scheme:
        update.message.reply_text("https://outline.com/" + link)
    else:
        update.message.reply_text(tld(chat.id, "misc_url_invalid"))
        return


def format_integer(number, thousand_separator=','):
    def reverse(string):
        string = "".join(reversed(string))
        return string

    s = reverse(str(number))
    count = 0
    result = ''
    for char in s:
        count = count + 1
        if count % 3 == 0:
            if len(s) == count:
                result = char + result
            else:
                result = thousand_separator + char + result
        else:
            result = char + result
    return result


def yt(update: Update, context: CallbackContext):
    args = context.args
    msg = update.effective_message
    chat = update.effective_chat
    if args:
        user = update.effective_user.id
        youtube_link = " ".join(args)
    else:
        message.reply_text(tld(chat.id, "misc_paste_invalid"))
        return
    video = pafy.new(youtube_link)
    video_stream = video.getbest()
    try:
        update.effective_message.reply_video(video=video_stream.url)
    except:
        update.message.reply_text(
            f"`Download failed: `[URL]({video_stream.url})", parse_mode=ParseMode.MARKDOWN)


def restart(update: Update, context: CallbackContext):
    user = update.effective_user
    chat_id = update.effective_chat.id

    if not user.id in SUDO_USERS:
        update.message.reply_text("User Not Sudo, Error.")
        return

    update.message.reply_text("Restarting...")
    args = [sys.executable, "-m", "smudge"]
    os.execl(sys.executable, *args)


__help__ = True

RESTART_HANDLER = CommandHandler("restart", restart, run_async=True)
YT_HANDLER = CommandHandler("yt", yt, pass_args=True, run_async=True)
OUTLINE_HANDLER = DisableAbleCommandHandler(
    "outline", outline, pass_args=True, admin_ok=True, run_async=True)
ID_HANDLER = DisableAbleCommandHandler(
    "id", get_id, pass_args=True, admin_ok=True, run_async=True)
IP_HANDLER = CommandHandler(
    "ip", get_bot_ip, filters=Filters.chat(OWNER_ID), run_async=True)

INFO_HANDLER = DisableAbleCommandHandler(
    "info", info, pass_args=True, admin_ok=True, run_async=True)
GITHUB_HANDLER = DisableAbleCommandHandler(
    "git", github, admin_ok=True, run_async=True)
ECHO_HANDLER = CommandHandler(
    "echo", echo, filters=Filters.user(OWNER_ID), run_async=True)
MD_HELP_HANDLER = CommandHandler(
    "markdownhelp", markdown_help, filters=Filters.private, run_async=True)

STATS_HANDLER = CommandHandler(
    "stats", stats, filters=Filters.user(OWNER_ID), run_async=True)
GDPR_HANDLER = CommandHandler(
    "gdpr", gdpr, filters=Filters.private, run_async=True)
UD_HANDLER = DisableAbleCommandHandler("ud", ud, run_async=True)
WIKI_HANDLER = DisableAbleCommandHandler("wiki", wiki, run_async=True)
WIKIEN_HANDLER = DisableAbleCommandHandler("wikien", wikien, run_async=True)
WIKIPT_HANDLER = DisableAbleCommandHandler("wikipt", wikipt, run_async=True)
COVID_HANDLER = DisableAbleCommandHandler(
    "covid", covid, admin_ok=True, run_async=True)
SCREENSHOT_HANDLER = CommandHandler(
    ["screenshot", "print", "ss", "screencapture"], screenshot, pass_args=True)

dispatcher.add_handler(RESTART_HANDLER)
dispatcher.add_handler(YT_HANDLER)
dispatcher.add_handler(OUTLINE_HANDLER)
dispatcher.add_handler(SCREENSHOT_HANDLER)
dispatcher.add_handler(UD_HANDLER)
dispatcher.add_handler(ID_HANDLER)
dispatcher.add_handler(IP_HANDLER)
dispatcher.add_handler(INFO_HANDLER)
dispatcher.add_handler(ECHO_HANDLER)
dispatcher.add_handler(MD_HELP_HANDLER)
dispatcher.add_handler(STATS_HANDLER)
dispatcher.add_handler(GDPR_HANDLER)
dispatcher.add_handler(GITHUB_HANDLER)
dispatcher.add_handler(
    DisableAbleCommandHandler("removebotkeyboard", reply_keyboard_remove))
dispatcher.add_handler(WIKI_HANDLER)
dispatcher.add_handler(WIKIPT_HANDLER)
dispatcher.add_handler(WIKIEN_HANDLER)
dispatcher.add_handler(COVID_HANDLER)
