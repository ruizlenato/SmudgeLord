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
import time
from io import BytesIO
from typing import List

from telegram import Update, ParseMode
from telegram.error import BadRequest  # ,  TelegramError
from telegram.ext import CommandHandler, MessageHandler, Filters, CallbackContext
from telegram.utils.helpers import mention_html

import smudge.modules.sql.antispam_sql as sql
from smudge import dispatcher, OWNER_ID, SUDO_USERS, STRICT_ANTISPAM, sw
from smudge.helper_funcs.chat_status import user_admin, is_user_admin
from smudge.helper_funcs.extraction import extract_user_and_text
from smudge.helper_funcs.filters import CustomFilters
#from smudge.helper_funcs.misc import send_to_list
# from smudge.sql.users_sql import get_all_chats

from smudge.modules.translations.strings import tld
from smudge.modules.disable import DisableAbleCommandHandler
GBAN_ENFORCE_GROUP = 6

GBAN_ERRORS = {
    "User is an administrator of the chat", "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant", "Peer_id_invalid", "Group chat was deactivated",
    "Need to be inviter of a user to kick it from a basic group",
    "Chat_admin_required",
    "Only the creator of a basic group can kick group administrators",
    "Channel_private",                                  "Not in the chat"
}

UNGBAN_ERRORS = {
    "User is an administrator of the chat",
    "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant",
    "Method is available for supergroup and channel chats only",
    "Not in the chat",
    "Channel_private",
    "Chat_admin_required",
}


def check_and_ban(update, user_id, should_message=True):
    chat = update.effective_chat
    message = update.effective_message
    try:
        if sw != None:
            sw_ban = sw.get_ban(user_id)
            if sw_ban:
                spamwatch_reason = sw_ban.reason
                chat.kick_member(user_id)
                if should_message:
                    message.reply_text(tld(
                        chat.id,
                        "antispam_spamwatch_banned").format(spamwatch_reason),
                        parse_mode=ParseMode.HTML)
                    return
                else:
                    return
    except Exception:
        pass

    if sql.is_user_gbanned(user_id):
        chat.kick_member(user_id)
        if should_message:
            userr = sql.get_gbanned_user(user_id)
            usrreason = userr.reason
            if not usrreason:
                usrreason = tld(chat.id, "antispam_no_reason")

            message.reply_text(tld(
                chat.id, "antispam_checkban_user_removed").format(usrreason),
                parse_mode=ParseMode.MARKDOWN)
            return


def enforce_gban(update: Update, context: CallbackContext):
    # Not using @restrict handler to avoid spamming - just ignore if cant gban.
    bot = context.bot
    try:
        if sql.does_chat_gban(
                update.effective_chat.id) and update.effective_chat.get_member(
                    bot.id).can_restrict_members:
            user = update.effective_user
            chat = update.effective_chat
            msg = update.effective_message

            if user and not is_user_admin(chat, user.id):
                check_and_ban(update, user.id)
                return

            if msg.new_chat_members:
                new_members = update.effective_message.new_chat_members
                for mem in new_members:
                    check_and_ban(update, mem.id)
                    return

            if msg.reply_to_message:
                user = msg.reply_to_message.from_user
                if user and not is_user_admin(chat, user.id):
                    check_and_ban(update, user.id, should_message=False)
                    return
    except Exception as f:
        print(f"err {f}")


@user_admin
def antispam(update: Update, context: CallbackContext):
    args = context.args
    chat = update.effective_chat
    if len(args) > 0:
        if args[0].lower() in ["on", "yes"]:
            sql.enable_antispam(chat.id)
            update.effective_message.reply_text(tld(chat.id, "antispam_on"))
        elif args[0].lower() in ["off", "no"]:
            sql.disable_antispam(chat.id)
            update.effective_message.reply_text(tld(chat.id, "antispam_off"))
    else:
        update.effective_message.reply_text(
            tld(chat.id,
                "antispam_err_wrong_arg").format(sql.does_chat_gban(chat.id)))


def __migrate__(old_chat_id, new_chat_id):
    sql.migrate_chat(old_chat_id, new_chat_id)


__help__ = True

ANTISPAM_STATUS = DisableAbleCommandHandler("antispam",
                                            antispam,
                                            pass_args=True,
                                            filters=Filters.chat_type.groups,
                                            run_async=True)

GBAN_ENFORCER = MessageHandler(Filters.all & Filters.chat_type.groups,
                               enforce_gban,
                               run_async=True)

dispatcher.add_handler(ANTISPAM_STATUS)

if STRICT_ANTISPAM:  # enforce GBANS if this is set
    dispatcher.add_handler(GBAN_ENFORCER, GBAN_ENFORCE_GROUP)
