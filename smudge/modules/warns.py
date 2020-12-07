import html
import re
from typing import Optional, List

import telegram
from telegram import InlineKeyboardButton, InlineKeyboardMarkup, ParseMode, User, CallbackQuery
from telegram import Message, Chat, Update, Bot
from telegram.error import BadRequest
from telegram.ext import CommandHandler, run_async, DispatcherHandlerStop, MessageHandler, Filters, CallbackQueryHandler
from telegram.utils.helpers import mention_html

from smudge import dispatcher, CallbackContext, SUDO_USERS
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.helper_funcs.chat_status import is_user_admin, bot_admin, user_admin_no_reply, user_admin, \
    can_restrict
from smudge.helper_funcs.extraction import extract_text, extract_user_and_text, extract_user
from smudge.helper_funcs.filters import CustomFilters
from smudge.helper_funcs.misc import split_message
from smudge.helper_funcs.string_handling import split_quotes
from smudge.modules.log_channel import loggable
from smudge.modules.sql import warns_sql as sql
from smudge.modules.translations.strings import tld

WARN_HANDLER_GROUP = 9

# Not async
def warn(user: User,
         chat: Chat,
         reason: str,
         message: Message,
         warner: User = None) -> str:
    if is_user_admin(chat, user.id):
        message.reply_text("Damn admins, can't even be warned!")
        return ""

    if not user.id or int(user.id) == 777000 or int(user.id) == 1087968824:
        message.reply_text(
            "This is the Telegram Service Bot or the Group Anonymous Bot. Kinda pointless to warn it, don't you think?"
        )
        return ""

    if warner:
        warner_tag = mention_html(warner.id, warner.first_name)
    else:
        warner_tag = "Automated warn filter."

    limit, soft_warn = sql.get_warn_setting(chat.id)
    num_warns, reasons = sql.warn_user(user.id, chat.id, reason)
    if num_warns >= limit:
        sql.reset_warns(user.id, chat.id)
        if soft_warn:  # kick
            chat.unban_member(user.id)
            reply = "{} warnings, {} has been kicked!".format(
                limit, mention_html(user.id, user.first_name))

        else:  # ban
            chat.kick_member(user.id)
            reply = "{} warnings, {} has been banned!".format(
                limit, mention_html(user.id, user.first_name))

        for warn_reason in reasons:
            reply += "\n - {}".format(html.escape(warn_reason))

        keyboard = []
        log_reason = tld(chat.id, 'warns_warn_ban_log_channel').format(html.escape(chat.title),
                                                                       warner_tag,
                                                                       mention_html(
                                                                           user.id, user.first_name),
                                                                       user.id, reason, num_warns, limit)

    else:
        keyboard = InlineKeyboardMarkup([[
            InlineKeyboardButton(tld(chat.id, 'warns_btn_remove_warn'),
                                 callback_data="rm_warn({})".format(user.id))
        ]])

        reply = tld(chat.id, 'warns_user_warned').format(
            mention_html(user.id, user.first_name), num_warns, limit)
        if reason:
            reply += tld(chat.id, 'warns_latest_warn_reason').format(
                html.escape(reason))

        log_reason = "<b>{}:</b>" \
                     "\n#WARN" \
                     "\n<b>Admin:</b> {}" \
                     "\n<b>User:</b> {} (<code>{}</code>)" \
                     "\n<b>Reason:</b> {}"\
                     "\n<b>Counts:</b> <code>{}/{}</code>".format(html.escape(chat.title),
                                                                  warner_tag,
                                                                  mention_html(user.id, user.first_name),
                                                                  user.id, reason, num_warns, limit)

    try:
        message.reply_text(reply,
                           reply_markup=keyboard,
                           parse_mode=ParseMode.HTML)
    except BadRequest as excp:
        if excp.message == "Reply message not found":
            # Do not reply
            message.reply_text(reply,
                               reply_markup=keyboard,
                               parse_mode=ParseMode.HTML,
                               quote=False)
        else:
            raise
    return log_reason


@user_admin_no_reply
@bot_admin
@loggable
def button(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    query = update.callback_query  # type: Optional[CallbackQuery]
    user = update.effective_user  # type: Optional[User]
    chat = update.effective_chat
    admin = chat.get_member(int(user.id))
    if (admin.status != 'creator') and (not admin.can_restrict_members) and (
            not int(user.id) in SUDO_USERS):
        query.answer(
            text="You don't have sufficient permissions to remove warns!")
        return ""

    match = re.match(r"rm_warn\((.+?)\)", query.data)
    if match:
        user_id = match.group(1)
        chat = update.effective_chat  # type: Optional[Chat]
        res = sql.remove_warn(user_id, chat.id)
        if res:
            update.effective_message.edit_text(tld(chat.id, 'warns_remove_success').format(
                mention_html(user.id, user.first_name)),
                                               parse_mode=ParseMode.HTML)
            user_member = chat.get_member(user_id)
            return "<b>{}:</b>" \
                   "\n#UNWARN" \
                   "\n<b>Admin:</b> {}" \
                   "\n<b>User:</b> {} (<code>{}</code>)".format(html.escape(chat.title),
                                                                mention_html(user.id, user.first_name),
                                                                mention_html(user_member.user.id, user_member.user.first_name),
                                                                user_member.user.id)
        else:
            update.effective_message.edit_text(
                "User has already has no warns.".format(
                    mention_html(user.id, user.first_name)),
                parse_mode=ParseMode.HTML)

    return ""


@user_admin
@can_restrict
@loggable
def warn_user(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    message = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    warner = update.effective_user  # type: Optional[User]

    user_id, reason = extract_user_and_text(message, args)
    conn = connected(update, context, chat, user.id, need_admin=False)
    if conn:
        chatP = dispatcher.bot.getChat(conn)
    else:
        chatP = update.effective_chat
        if chat.type == "private":
            return

    if user_id:
        if message.reply_to_message and message.reply_to_message.from_user.id == user_id:
            return warn(message.reply_to_message.from_user, chat, reason,
                        message.reply_to_message, warner)
        else:
            return warn(
                chat.get_member(user_id).user, chat, reason, message, warner)
    else:
        message.reply_text(tld(chat.id, 'common_err_no_user'))
    return ""


@user_admin
@bot_admin
@loggable
def reset_warns(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    message = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]

    user_id = extract_user(message, args)

    if user_id:
        sql.reset_warns(user_id, chat.id)
        message.reply_text(tld(chat.id, 'warns_reset_success'))
        warned = chat.get_member(user_id).user
        return "<b>{}:</b>" \
               "\n#RESETWARNS" \
               "\n<b>Admin:</b> {}" \
               "\n<b>User:</b> {} (<code>{}</code>)".format(html.escape(chat.title),
                                                            mention_html(user.id, user.first_name),
                                                            mention_html(warned.id, warned.first_name),
                                                            warned.id)
    else:
        message.reply_text(tld(chat.id, 'common_err_no_user'))
    return ""


@user_admin
@bot_admin
@loggable
def remove_warns(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    message = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]

    user_id = extract_user(message, args)

    if user_id:
        sql.remove_warn(user_id, chat.id)
        message.reply_text(tld(chat.id, 'warns_latest_remove_success'))
        warned = chat.get_member(user_id).user
        return "<b>{}:</b>" \
               "\n#UNWARN" \
               "\n<b>• Admin:</b> {}" \
               "\n<b>• User:</b> {}" \
               "\n<b>• ID:</b> <code>{}</code>".format(html.escape(chat.title),
                                                       mention_html(user.id, user.first_name),
                                                       mention_html(warned.id, warned.first_name),
                                                       warned.id)
    else:
        message.reply_text(tld(chat.id, 'common_err_no_user'))
    return ""


def warns(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    message = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    user_id = extract_user(message, args) or update.effective_user.id
    result = sql.get_warns(user_id, chat.id)

    if result and result[0] != 0:
        num_warns, reasons = result
        limit, soft_warn = sql.get_warn_setting(chat.id)

        if reasons:
            text = tld(chat.id, 'warns_list_warns').format(
                num_warns, limit)
            for reason in reasons:
                text += "\n - {}".format(reason)

            msgs = split_message(text)
            for msg in msgs:
                update.effective_message.reply_text(msg)
        else:
            update.effective_message.reply_text(
                tld(chat.id, 'warns_list_warns_no_reason').
                format(num_warns, limit))
    else:
        update.effective_message.reply_text(
            tld(chat.id, 'warns_list_warns_none'))


# Dispatcher handler stop - do not async
@user_admin
def add_warn_filter(update: Update, context: CallbackContext):
    bot = context.bot
    chat = update.effective_chat  # type: Optional[Chat]
    msg = update.effective_message  # type: Optional[Message]

    args = msg.text.split(
        None,
        1)  # use python's maxsplit to separate Cmd, keyword, and reply_text

    if len(args) < 2:
        return

    extracted = split_quotes(args[1])

    if len(extracted) >= 2:
        # set trigger -> lower, so as to avoid adding duplicate filters with different cases
        keyword = extracted[0].lower()
        content = extracted[1]

    else:
        return

    # Note: perhaps handlers can be removed somehow using sql.get_chat_filters
    for handler in dispatcher.handlers.get(WARN_HANDLER_GROUP, []):
        if handler.filters == (keyword, chat.id):
            dispatcher.remove_handler(handler, WARN_HANDLER_GROUP)

    sql.add_warn_filter(chat.id, keyword, content)

    update.effective_message.reply_text(
        tld(chat.id, 'warns_handler_add_success').format(keyword))
    raise DispatcherHandlerStop


@user_admin
def remove_warn_filter(update: Update, context: CallbackContext):
    bot = context.bot
    chat = update.effective_chat  # type: Optional[Chat]
    msg = update.effective_message  # type: Optional[Message]

    args = msg.text.split(
        None,
        1)  # use python's maxsplit to separate Cmd, keyword, and reply_text

    if len(args) < 2:
        return

    extracted = split_quotes(args[1])

    if len(extracted) < 1:
        return

    to_remove = extracted[0]

    chat_filters = sql.get_chat_warn_triggers(chat.id)

    if not chat_filters:
        msg.reply_text(tld(chat.id, 'warns_filters_list_empty'))
        return

    for filt in chat_filters:
        if filt == to_remove:
            sql.remove_warn_filter(chat.id, to_remove)
            msg.reply_text(tld(chat.id, 'warns_filters_remove_success'))
            raise DispatcherHandlerStop

    msg.reply_text(tld(chat.id, 'warns_filter_doesnt_exist'))



def list_warn_filters(update: Update, context: CallbackContext):
    bot = context.bot
    chat = update.effective_chat  # type: Optional[Chat]
    all_handlers = sql.get_chat_warn_triggers(chat.id)

    if not all_handlers:
        update.effective_message.reply_text(
            tld(chat.id, 'warns_filters_list_empty'))
        return

    filter_list = CURRENT_WARNING_FILTER_STRING
    for keyword in all_handlers:
        entry = " - {}\n".format(html.escape(keyword))
        if len(entry) + len(filter_list) > telegram.MAX_MESSAGE_LENGTH:
            update.effective_message.reply_text(filter_list,
                                                parse_mode=ParseMode.HTML)
            filter_list = entry
        else:
            filter_list += entry

    if not filter_list == tld(chat.id, 'warns_filters_list'):
        update.effective_message.reply_text(filter_list,
                                            parse_mode=ParseMode.HTML)


@loggable
def reply_filter(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    chat = update.effective_chat  # type: Optional[Chat]
    message = update.effective_message  # type: Optional[Message]

    chat_warn_filters = sql.get_chat_warn_triggers(chat.id)
    to_match = extract_text(message)
    if not to_match:
        return ""

    for keyword in chat_warn_filters:
        pattern = r"( |^|[^\w])" + re.escape(keyword).replace(
            "\*", "(.*)").replace("\\(.*)", "*") + r"( |$|[^\w])"
        if re.search(pattern, to_match, flags=re.IGNORECASE):
            user = update.effective_user  # type: Optional[User]
            warn_filter = sql.get_warn_filter(chat.id, keyword)
            return warn(user, chat, warn_filter.reply, message)
    return ""


@user_admin
@loggable
def set_warn_limit(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    msg = update.effective_message  # type: Optional[Message]

    if args:
        if args[0].isdigit():
            if int(args[0]) < 3:
                msg.reply_text("The minimum warn limit is 3!")
            else:
                sql.set_warn_limit(chat.id, int(args[0]))
                msg.reply_text("Updated the warn limit to {}".format(args[0]))
                return "<b>{}:</b>" \
                       "\n#SET_WARN_LIMIT" \
                       "\n<b>Admin:</b> {}" \
                       "\nSet the warn limit to <code>{}</code>".format(html.escape(chat.title),
                                                                        mention_html(user.id, user.first_name), args[0])
        else:
            msg.reply_text("Give me a number as an arg!")
    else:
        limit, soft_warn = sql.get_warn_setting(chat.id)

        msg.reply_text("The current warn limit is {}".format(limit))
    return ""


@user_admin
def set_warn_strength(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    msg = update.effective_message  # type: Optional[Message]

    if args:
        if args[0].lower() in ("on", "yes"):
            sql.set_warn_strength(chat.id, False)
            msg.reply_text("Too many warns will now result in a ban!")
            return "<b>{}:</b>\n" \
                   "<b>Admin:</b> {}\n" \
                   "Has enabled strong warns. Users will be banned.".format(html.escape(chat.title),
                                                                            mention_html(user.id, user.first_name))

        elif args[0].lower() in ("off", "no"):
            sql.set_warn_strength(chat.id, True)
            msg.reply_text(
                "Too many warns will now result in a kick! Users will be able to join again after."
            )
            return "<b>{}:</b>\n" \
                   "<b>Admin:</b> {}\n" \
                   "Has disabled strong warns. Users will only be kicked.".format(html.escape(chat.title),
                                                                                  mention_html(user.id,
                                                                                               user.first_name))

        else:
            msg.reply_text("I only understand on/yes/no/off!")
    else:
        limit, soft_warn = sql.get_warn_setting(chat.id)
        if soft_warn:
            msg.reply_text(
                "Warns are currently set to *kick* users when they exceed the limits.",
                parse_mode=ParseMode.MARKDOWN)
        else:
            msg.reply_text(
                "Warns are currently set to *ban* users when they exceed the limits.",
                parse_mode=ParseMode.MARKDOWN)
    return ""


def __stats__():
    return "{} overall warns, across {} chats.\n" \
           "{} warn filters, across {} chats.".format(sql.num_warns(), sql.num_warn_chats(),
                                                      sql.num_warn_filters(), sql.num_warn_filter_chats())


def __import_data__(chat_id, data):
    for user_id, count in data.get('warns', {}).items():
        for x in range(int(count)):
            sql.warn_user(user_id, chat_id)


def __migrate__(old_chat_id, new_chat_id):
    sql.migrate_chat(old_chat_id, new_chat_id)


def __chat_settings__(chat_id, user_id):
    num_warn_filters = sql.num_warn_chat_filters(chat_id)
    limit, soft_warn = sql.get_warn_setting(chat_id)
    return "This chat has `{}` warn filters. It takes `{}` warns " \
           "before the user gets *{}*.".format(num_warn_filters, limit, "kicked" if soft_warn else "banned")


__help__ = True

WARN_HANDLER = CommandHandler("warn",
                              warn_user,
                              run_async=True,
                              filters=Filters.chat_type.groups)
RESET_WARN_HANDLER = CommandHandler(["resetwarn", "resetwarns"],
                                    reset_warns,
                                    run_async=True,
                                    filters=Filters.chat_type.groups)
REMOVE_WARNS_HANDLER = CommandHandler(["rmwarn", "unwarn"],
                                      remove_warns,
                                      run_async=True,
                                      filters=Filters.chat_type.groups)
CALLBACK_QUERY_HANDLER = CallbackQueryHandler(button, pattern=r"rm_warn")
MYWARNS_HANDLER = DisableAbleCommandHandler("warns",
                                            warns,
                                            run_async=True,
                                            filters=Filters.chat_type.groups)

ADD_WARN_HANDLER = CommandHandler("addwarn",
                                  add_warn_filter,
                                  filters=Filters.chat_type.groups)
RM_WARN_HANDLER = CommandHandler(["nowarn", "stopwarn"],
                                 remove_warn_filter,
                                 filters=Filters.chat_type.groups)

LIST_WARN_HANDLER = DisableAbleCommandHandler(["warnlist", "warnfilters"],
                                              list_warn_filters,
                                              filters=Filters.chat_type.groups,
                                              admin_ok=True)
WARN_FILTER_HANDLER = MessageHandler(
    CustomFilters.has_text & Filters.chat_type.groups, reply_filter)
WARN_LIMIT_HANDLER = CommandHandler("warnlimit",
                                    set_warn_limit,
                                    run_async=True,
                                    filters=Filters.chat_type.groups)
WARN_STRENGTH_HANDLER = CommandHandler("strongwarn",
                                       set_warn_strength,
                                       run_async=True,
                                       filters=Filters.chat_type.groups)

dispatcher.add_handler(WARN_HANDLER)
dispatcher.add_handler(CALLBACK_QUERY_HANDLER)
dispatcher.add_handler(RESET_WARN_HANDLER)
dispatcher.add_handler(REMOVE_WARNS_HANDLER)
dispatcher.add_handler(MYWARNS_HANDLER)
dispatcher.add_handler(ADD_WARN_HANDLER)
dispatcher.add_handler(RM_WARN_HANDLER)
dispatcher.add_handler(LIST_WARN_HANDLER)
dispatcher.add_handler(WARN_LIMIT_HANDLER)
dispatcher.add_handler(WARN_STRENGTH_HANDLER)
dispatcher.add_handler(WARN_FILTER_HANDLER, WARN_HANDLER_GROUP)