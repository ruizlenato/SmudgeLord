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
from typing import Optional, List

from telegram import Message, Chat, Update, User, ChatPermissions, ParseMode

from telegram.error import BadRequest
from telegram.ext import CommandHandler, Filters
from telegram.ext.dispatcher import run_async
from telegram.utils.helpers import mention_html

from smudge import dispatcher, CallbackContext, LOGGER
from smudge.helper_funcs.chat_status import bot_admin, user_admin, is_user_admin, can_restrict
from smudge.helper_funcs.extraction import extract_user, extract_user_and_text
from smudge.helper_funcs.string_handling import extract_time
from smudge.modules.log_channel import loggable
from smudge.modules.translations.strings import tld
from smudge.modules.disable import DisableAbleCommandHandler


@bot_admin
@user_admin
@loggable
def mute(update: Update, context: CallbackContext) -> str:
    args = context.args
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    message = update.effective_message  # type: Optional[Message]

    user_id = extract_user(message, args)
    if not user_id:
        message.reply_text(tld(chat.id, "mute_invalid"))
        return ""

    if user_id == context.bot.id:
        message.reply_text(tld(chat.id, "mute_not_myself"))
        return ""

    member = chat.get_member(int(user_id))

    if member:

        if is_user_admin(chat, user_id, member=member):
            message.reply_text(tld(chat.id, "mute_not_m_admin"))

        elif member.can_send_messages is None or member.can_send_messages:
            context.bot.restrict_chat_member(
                chat.id,
                user_id,
                permissions=ChatPermissions(can_send_messages=False))

            reply = tld(chat.id, "mute_success").format(
                mention_html(member.user.id, member.user.first_name))
            message.reply_text(reply, parse_mode=ParseMode.HTML)
            return "<b>{}:</b>" \
                   "\n#MUTE" \
                   "\n<b>Admin:</b> {}" \
                   "\n<b>User:</b> {}".format(html.escape(chat.title),
                                              mention_html(
                                                  user.id, user.first_name),
                                              mention_html(member.user.id, member.user.first_name))

        else:
            message.reply_text(
                tld(chat.id, "mute_already_mute").format(chat.title))
    else:
        message.reply_text(tld(chat.id, "mute_not_in_chat").format(chat.title))

    return ""


@bot_admin
@user_admin
@loggable
def unmute(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    message = update.effective_message  # type: Optional[Message]

    user_id = extract_user(message, args)
    if not user_id:
        message.reply_text(tld(chat.id, "unmute_invalid"))
        return ""

    member = chat.get_member(int(user_id))

    if member:
        if member.status !=  'kicked' or 'left':
            if member.can_send_messages and  member.can_send_media_messages \
                    and member.can_send_other_messages and member.can_add_web_page_previews:
                message.reply_text(
                    tld(chat.id, "unmute_not_muted").format(chat.title))
                return ""
            else:
                context.bot.restrict_chat_member(chat.id,
                                                 int(user_id),
                                                 permissions=ChatPermissions(
                                                     can_send_messages=True,
                                                     can_send_media_messages=True,
                                                     can_send_other_messages=True,
                                                     can_add_web_page_previews=True))
            reply = tld(chat.id, "unmute_success").format(
                mention_html(member.user.id, member.user.first_name), chat.title)
            message.reply_text(reply, parse_mode=ParseMode.HTML)
            return "<b>{}:</b>" \
                   "\n#UNMUTE" \
                   "\n<b>• Admin:</b> {}" \
                   "\n<b>• User:</b> {}" \
                   "\n<b>• ID:</b> <code>{}</code>".format(html.escape(chat.title), mention_html(
                       user.id, user.first_name), mention_html(member.user.id, member.user.first_name), user_id)
    else:
        message.reply_text(tld(chat.id, "unmute_not_in_chat"))

    return ""


@bot_admin
@can_restrict
@user_admin
@loggable
def temp_mute(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    message = update.effective_message  # type: Optional[Message]

    user_id, reason = extract_user_and_text(message, args)

    if not user_id:
        message.reply_text(tld(chat.id, "mute_not_refer"))
        return ""

    try:
        member = chat.get_member(user_id)
    except BadRequest as excp:
        if excp.message == "User not found":
            message.reply_text(tld(chat.id, "mute_not_existed"))
            return ""
        else:
            raise

    if is_user_admin(chat, user_id, member):
        message.reply_text(tld(chat.id, "mute_is_admin"))
        return ""

    if user_id == bot.id:
        message.reply_text(tld(chat.id, "mute_is_bot"))
        return ""

    if not reason:
        message.reply_text(tld(chat.id, "tmute_no_time"))
        return ""

    split_reason = reason.split(None, 1)

    time_val = split_reason[0].lower()
    if len(split_reason) > 1:
        reason = split_reason[1]
    else:
        reason = ""

    mutetime = extract_time(message, time_val)

    if not mutetime:
        return ""

    log = "<b>{}:</b>" \
          "\n#TEMP MUTED" \
          "\n<b>Admin:</b> {}" \
          "\n<b>User:</b> {}" \
          "\n<b>Time:</b> {}".format(html.escape(chat.title), mention_html(
              user.id, user.first_name), mention_html(member.user.id, member.user.first_name), time_val)
    if reason:
        log += "\n<b>Reason:</b> {}".format(reason)

    try:
        if member.can_send_messages is None or member.can_send_messages:
            context.bot.restrict_chat_member(
                chat.id,
                user_id,
                until_date=mutetime,
                permissions=ChatPermissions(can_send_messages=False))

            message.reply_text(
                tld(chat.id, "tmute_success").format(time_val, chat.title))
            return log
        else:
            message.reply_text(
                tld(chat.id, "mute_already_mute").format(chat.title))

    except BadRequest as excp:
        if excp.message == "Reply message not found":
            # Do not reply
            message.reply_text(tld(chat.id, "tmute_success").format(
                time_val), quote=False)
            return log
        else:
            LOGGER.warning(update)
            LOGGER.exception("ERROR muting user %s in chat %s (%s) due to %s",
                             user_id, chat.title, chat.id, excp.message)
            message.reply_text(tld(chat.id, "mute_cant_mute"))

    return ""


@bot_admin
@can_restrict
def muteme(update: Update, context: CallbackContext) -> str:
    user = update.effective_message.from_user
    chat = update.effective_chat
    user = update.effective_user
    if is_user_admin(update.effective_chat, user.id):
        update.effective_message.reply_text(tld(chat.id, "mute_is_admin"))
        return

    res = context.bot.restrict_chat_member(
        chat.id, user.id, permissions=ChatPermissions(can_send_messages=False))
    if res:
        update.effective_message.reply_text(tld(chat.id, "muteme_muted"))
        log = "<b>{}:</b>" \
            "\n#MUTEME" \
            "\n<b>User:</b> {}" \
            "\n<b>ID:</b> <code>{}</code>".format(html.escape(chat.title),
                                                  mention_html(user.id, user.first_name), user.id)
        return log
    update.effective_message.reply_text(tld(chat.id, "mute_cant_mute"))


MUTE_HANDLER = DisableAbleCommandHandler("mute",
                                         mute,
                                         run_async=True,
                                         filters=Filters.chat_type.groups)
UNMUTE_HANDLER = DisableAbleCommandHandler("unmute",
                                           unmute,
                                           run_async=True,
                                           filters=Filters.chat_type.groups)
TEMPMUTE_HANDLER = DisableAbleCommandHandler(["tmute", "tempmute"],
                                             temp_mute,
                                             run_async=True,
                                             filters=Filters.chat_type.groups)
MUTEME_HANDLER = DisableAbleCommandHandler("muteme",
                                           muteme,
                                           pass_args=True,
                                           filters=Filters.chat_type.groups,
                                           admin_ok=True)

dispatcher.add_handler(MUTE_HANDLER)
dispatcher.add_handler(UNMUTE_HANDLER)
dispatcher.add_handler(TEMPMUTE_HANDLER)
dispatcher.add_handler(MUTEME_HANDLER)
