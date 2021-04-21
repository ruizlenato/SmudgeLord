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

import html
from typing import List

from telegram import Update, ChatPermissions, ParseMode
from telegram.error import BadRequest
from telegram.ext import Filters, MessageHandler, CommandHandler, run_async, CallbackContext
from telegram.utils.helpers import mention_html

from smudge import dispatcher
from smudge.helper_funcs.chat_status import is_user_admin, user_admin, can_restrict, user_can_changeinfo
from smudge.modules.log_channel import loggable
from smudge.modules.sql import antiflood_sql as sql

from smudge.modules.translations.strings import tld
from smudge.modules.connection import connected

FLOOD_GROUP = 5


@loggable
def check_flood(update, context) -> str:
    user = update.effective_user
    chat = update.effective_chat
    msg = update.effective_message

    if not user:  # ignore channels
        return ""

    if user.id == 777000:  # ignore telegram
        return ""

    # ignore admins
    if is_user_admin(chat, user.id):
        sql.update_flood(chat.id, None)
        return ""

    should_ban = sql.update_flood(chat.id, user.id)
    if not should_ban:
        return ""

    try:
        context.bot.restrict_chat_member(
            chat.id, user.id, permissions=ChatPermissions(can_send_messages=False))
        msg.reply_text(tld(chat.id, "flood_mute"),
                       parse_mode=ParseMode.MARKDOWN)
        return tld(chat.id, "flood_logger_success").format(
            html.escape(chat.title), mention_html(user.id, user.first_name))

    except BadRequest:
        msg.reply_text(tld(chat.id, "flood_err_no_perm"))
        sql.set_flood(chat.id, 0)
        return tld(chat.id, "flood_logger_fail").format(chat.title)


@user_admin
@can_restrict
@loggable
@user_can_changeinfo
def set_flood(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    chat = update.effective_chat
    user = update.effective_user
    message = update.effective_message

    if len(args) >= 1:
        val = args[0].lower()
        if val in ("off", "no", "0"):
            sql.set_flood(chat.id, 0)
            message.reply_text(tld(chat.id, "flood_set_off"))

        elif val.isdigit():
            amount = int(val)
            if amount <= 0:
                sql.set_flood(chat.id, 0)
                message.reply_text(tld(chat.id, "flood_set_off"))
                return tld(chat.id, "flood_logger_set_off").format(
                    html.escape(chat.title),
                    mention_html(user.id, user.first_name))

            elif amount < 3:
                message.reply_text(tld(chat.id, "flood_err_num"))
                return ""

            else:
                sql.set_flood(chat.id, amount)
                message.reply_text(tld(chat.id, "flood_set").format(amount))
                return tld(chat.id, "flood_logger_set_on").format(
                    html.escape(chat.title),
                    mention_html(user.id, user.first_name), amount)

        else:
            message.reply_text(tld(chat.id, "flood_err_args"))

    return ""


def flood(update: Update, context: CallbackContext):
    bot = context.bot
    chat = update.effective_chat

    limit = sql.get_flood_limit(chat.id)
    if limit == 0:
        update.effective_message.reply_text(tld(chat.id, "flood_status_off"))
    else:
        update.effective_message.reply_text(
            tld(chat.id, "flood_status_on").format(limit))


def __migrate__(old_chat_id, new_chat_id):
    sql.migrate_chat(old_chat_id, new_chat_id)


__help__ = True

# TODO: Add actions: ban/kick/mute/tban/tmute

FLOOD_BAN_HANDLER = MessageHandler(Filters.all & Filters.chat_type.groups,
                                   check_flood,
                                   run_async=True)
SET_FLOOD_HANDLER = CommandHandler("setflood",
                                   set_flood,
                                   filters=Filters.chat_type.groups,
                                   pass_args=True,
                                   run_async=True)
FLOOD_HANDLER = CommandHandler("flood",
                               flood,
                               filters=Filters.chat_type.groups,
                               run_async=True)

dispatcher.add_handler(FLOOD_BAN_HANDLER, FLOOD_GROUP)
dispatcher.add_handler(SET_FLOOD_HANDLER)
dispatcher.add_handler(FLOOD_HANDLER)
