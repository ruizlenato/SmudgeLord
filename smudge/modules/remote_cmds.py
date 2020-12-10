import html
from typing import Optional, List

from telegram import Message, Chat, Update, Bot, User, ParseMode, InlineKeyboardMarkup, ChatPermissions
from telegram.error import BadRequest
from telegram.ext import run_async, CommandHandler, Filters
from telegram.utils.helpers import mention_html

from smudge import dispatcher, CallbackContext, OWNER_ID, LOGGER
from smudge.helper_funcs.chat_status import bot_admin, user_admin, is_user_ban_protected, can_restrict, \
    is_user_admin, is_user_in_chat, is_bot_admin
from smudge.helper_funcs.extraction import extract_user_and_text
from smudge.helper_funcs.string_handling import extract_time
from smudge.helper_funcs.filters import CustomFilters

RBAN_ERRORS = {
    "User is an administrator of the chat", "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant", "Peer_id_invalid", "Group chat was deactivated",
    "Need to be inviter of a user to kick it from a basic group",
    "Chat_admin_required",
    "Only the creator of a basic group can kick group administrators",
    "Channel_private", "Not in the chat"
}

RUNBAN_ERRORS = {
    "User is an administrator of the chat", "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant", "Peer_id_invalid", "Group chat was deactivated",
    "Need to be inviter of a user to kick it from a basic group",
    "Chat_admin_required",
    "Only the creator of a basic group can kick group administrators",
    "Channel_private", "Not in the chat"
}

RKICK_ERRORS = {
    "User is an administrator of the chat", "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant", "Peer_id_invalid", "Group chat was deactivated",
    "Need to be inviter of a user to kick it from a basic group",
    "Chat_admin_required",
    "Only the creator of a basic group can kick group administrators",
    "Channel_private", "Not in the chat"
}

RMUTE_ERRORS = {
    "User is an administrator of the chat", "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant", "Peer_id_invalid", "Group chat was deactivated",
    "Need to be inviter of a user to kick it from a basic group",
    "Chat_admin_required",
    "Only the creator of a basic group can kick group administrators",
    "Channel_private", "Not in the chat"
}

RUNMUTE_ERRORS = {
    "User is an administrator of the chat", "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant", "Peer_id_invalid", "Group chat was deactivated",
    "Need to be inviter of a user to kick it from a basic group",
    "Chat_admin_required",
    "Only the creator of a basic group can kick group administrators",
    "Channel_private", "Not in the chat"
}


@bot_admin
def rban(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    message = update.effective_message
    #chat_name = chat.title or chat.first or chat.username
    chat = update.effective_chat  # type: Optional[Chat]
    if not args:
        message.reply_text("You don't seem to be referring to a chat/user.")
        return

    user_id, chat_id = extract_user_and_text(message, args)
    if not user_id:
        message.reply_text("You don't seem to be referring to a user.")
        return
    elif not chat_id:
        message.reply_text("You don't seem to be referring to a chat.")
        return

    try:
        chat = bot.get_chat(chat_id.split()[0])
    except BadRequest as excp:
        if excp.message == "Chat not found":
            message.reply_text(
                "Chat not found! Make sure you entered a valid chat ID and I'm part of that chat."
            )
            return
        else:
            raise

    if chat.type == 'private':
        message.reply_text("I'm sorry, but that's a private chat!")
        return

    if not is_bot_admin(chat, bot.id) or not chat.get_member(
            bot.id).can_restrict_members:
        message.reply_text(
            "I can't restrict people there! Make sure I'm admin and can ban users."
        )
        return

    try:
        member = chat.get_member(user_id)
    except BadRequest as excp:
        if excp.message == "User not found":
            message.reply_text("I can't seem to find this user")
            return
        else:
            raise

    if is_user_ban_protected(chat, user_id, member):
        message.reply_text("I really wish I could ban admins...")
        return

    if user_id == bot.id:
        message.reply_text("I'm not gonna BAN myself, are you crazy?")
        return

    try:
        chat.kick_member(user_id)
        rbanning = "Hunting again in the wild!\n{} has been remotely banned from {}! \n".format(
            mention_html(member.user.id, member.user.first_name),
            (chat.title or chat.first or chat.username))
        message.reply_text(rbanning, parse_mode=ParseMode.HTML)
    except BadRequest as excp:
        if excp.message == "Reply message not found":
            # Do not reply
            message.reply_text('Banned!', quote=False)
        elif excp.message in RBAN_ERRORS:
            message.reply_text(excp.message)
        else:
            LOGGER.warning(update)
            LOGGER.exception("ERROR banning user %s in chat %s (%s) due to %s",
                             user_id, chat.title, chat.id, excp.message)
            message.reply_text("Well damn, I can't ban that user.")


@bot_admin
def runban(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    message = update.effective_message

    if not args:
        message.reply_text("You don't seem to be referring to a chat/user.")
        return

    user_id, chat_id = extract_user_and_text(message, args)

    if not user_id:
        message.reply_text("You don't seem to be referring to a user.")
        return
    elif not chat_id:
        message.reply_text("You don't seem to be referring to a chat.")
        return

    try:
        chat = bot.get_chat(chat_id.split()[0])
    except BadRequest as excp:
        if excp.message == "Chat not found":
            message.reply_text(
                "Chat not found! Make sure you entered a valid chat ID and I'm part of that chat."
            )
            return
        else:
            raise

    if chat.type == 'private':
        message.reply_text("I'm sorry, but that's a private chat!")
        return

    if not is_bot_admin(chat, bot.id) or not chat.get_member(
            bot.id).can_restrict_members:
        message.reply_text(
            "I can't unrestrict people there! Make sure I'm admin and can unban users."
        )
        return

    try:
        member = chat.get_member(user_id)
    except BadRequest as excp:
        if excp.message == "User not found":
            message.reply_text("I can't seem to find this user there")
            return
        else:
            raise

    if is_user_in_chat(chat, user_id):
        message.reply_text(
            "Why are you trying to remotely unban someone that's already in that chat?"
        )
        return

    if user_id == bot.id:
        message.reply_text("I'm not gonna UNBAN myself, I'm an admin there!")
        return

    try:
        chat.unban_member(user_id)
        runbanning = "I am allowing {} to join {}!".format(
            mention_html(member.user.id, member.user.first_name),
            (chat.title or chat.first or chat.username))
        message.reply_text(runbanning, parse_mode=ParseMode.HTML)
    except BadRequest as excp:
        if excp.message == "Reply message not found":
            # Do not reply
            message.reply_text('Unbanned!', quote=False)
        elif excp.message in RUNBAN_ERRORS:
            message.reply_text(excp.message)
        else:
            LOGGER.warning(update)
            LOGGER.exception(
                "ERROR unbanning user %s in chat %s (%s) due to %s", user_id,
                chat.title, chat.id, excp.message)
            message.reply_text("Well damn, I can't unban that user.")


@bot_admin
def rkick(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    message = update.effective_message

    if not args:
        message.reply_text("You don't seem to be referring to a chat/user.")
        return

    user_id, chat_id = extract_user_and_text(message, args)

    if not user_id:
        message.reply_text("You don't seem to be referring to a user.")
        return
    elif not chat_id:
        message.reply_text("You don't seem to be referring to a chat.")
        return

    try:
        chat = bot.get_chat(chat_id.split()[0])
    except BadRequest as excp:
        if excp.message == "Chat not found":
            message.reply_text(
                "Chat not found! Make sure you entered a valid chat ID and I'm part of that chat."
            )
            return
        else:
            raise

    if chat.type == 'private':
        message.reply_text("I'm sorry, but that's a private chat!")
        return

    if not is_bot_admin(chat, bot.id) or not chat.get_member(
            bot.id).can_restrict_members:
        message.reply_text(
            "I can't restrict people there! Make sure I'm admin and can kick users."
        )
        return

    try:
        member = chat.get_member(user_id)
    except BadRequest as excp:
        if excp.message == "User not found":
            message.reply_text("I can't seem to find this user")
            return
        else:
            raise

    if is_user_ban_protected(chat, user_id, member):
        message.reply_text("I really wish I could kick admins...")
        return

    if user_id == bot.id:
        message.reply_text("I'm not gonna KICK myself, are you crazy?")
        return

    try:
        chat.unban_member(user_id)
        rkicking = "Hunting again in the wild!\n{} has been remotely kicked from {}! \n".format(
            mention_html(member.user.id, member.user.first_name),
            (chat.title or chat.first or chat.username))
        message.reply_text(rkicking, parse_mode=ParseMode.HTML)
    except BadRequest as excp:
        if excp.message == "Reply message not found":
            # Do not reply
            message.reply_text('Kicked!', quote=False)
        elif excp.message in RKICK_ERRORS:
            message.reply_text(excp.message)
        else:
            LOGGER.warning(update)
            LOGGER.exception("ERROR kicking user %s in chat %s (%s) due to %s",
                             user_id, chat.title, chat.id, excp.message)
            message.reply_text("Well damn, I can't kick that user.")


@bot_admin
def rmute(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    message = update.effective_message

    if not args:
        message.reply_text("You don't seem to be referring to a chat/user.")
        return

    user_id, chat_id = extract_user_and_text(message, args)

    if not user_id:
        message.reply_text("You don't seem to be referring to a user.")
        return
    elif not chat_id:
        message.reply_text("You don't seem to be referring to a chat.")
        return

    try:
        chat = bot.get_chat(chat_id.split()[0])
    except BadRequest as excp:
        if excp.message == "Chat not found":
            message.reply_text(
                "Chat not found! Make sure you entered a valid chat ID and I'm part of that chat."
            )
            return
        else:
            raise

    if chat.type == 'private':
        message.reply_text("I'm sorry, but that's a private chat!")
        return

    if not is_bot_admin(chat, bot.id) or not chat.get_member(
            bot.id).can_restrict_members:
        message.reply_text(
            "I can't restrict people there! Make sure I'm admin and can mute users."
        )
        return

    try:
        member = chat.get_member(user_id)
    except BadRequest as excp:
        if excp.message == "User not found":
            message.reply_text("I can't seem to find this user")
            return
        else:
            raise

    if is_user_ban_protected(chat, user_id, member):
        message.reply_text("I really wish I could mute admins...")
        return

    if user_id == bot.id:
        message.reply_text("I'm not gonna MUTE myself, are you crazy?")
        return

    try:
        bot.restrict_chat_member(
            chat.id,
            user_id,
            permissions=ChatPermissions(can_send_messages=False))
        rmuting = "It is so quiet...\n{} has been remotely muted from {}! \n".format(
            mention_html(member.user.id, member.user.first_name),
            (chat.title or chat.first or chat.username))
        message.reply_text(rmuting, parse_mode=ParseMode.HTML)
    except BadRequest as excp:
        if excp.message == "Reply message not found":
            # Do not reply
            message.reply_text('Muted!', quote=False)
        elif excp.message in RMUTE_ERRORS:
            message.reply_text(excp.message)
        else:
            LOGGER.warning(update)
            LOGGER.exception("ERROR mute user %s in chat %s (%s) due to %s",
                             user_id, chat.title, chat.id, excp.message)
            message.reply_text("Well damn, I can't mute that user.")


@bot_admin
def runmute(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    message = update.effective_message

    if not args:
        message.reply_text("You don't seem to be referring to a chat/user.")
        return

    user_id, chat_id = extract_user_and_text(message, args)

    if not user_id:
        message.reply_text("You don't seem to be referring to a user.")
        return
    elif not chat_id:
        message.reply_text("You don't seem to be referring to a chat.")
        return

    try:
        chat = bot.get_chat(chat_id.split()[0])
    except BadRequest as excp:
        if excp.message == "Chat not found":
            message.reply_text(
                "Chat not found! Make sure you entered a valid chat ID and I'm part of that chat."
            )
            return
        else:
            raise

    if chat.type == 'private':
        message.reply_text("I'm sorry, but that's a private chat!")
        return

    if not is_bot_admin(chat, bot.id) or not chat.get_member(
            bot.id).can_restrict_members:
        message.reply_text(
            "I can't unrestrict people there! Make sure I'm admin and can unban users."
        )
        return

    try:
        member = chat.get_member(user_id)
    except BadRequest as excp:
        if excp.message == "User not found":
            message.reply_text("I can't seem to find this user there")
            return
        else:
            raise

    if is_user_in_chat(chat, user_id):
        if member.can_send_messages and member.can_send_media_messages \
           and member.can_send_other_messages and member.can_add_web_page_previews:
            message.reply_text(
                "This user already has the right to speak in that chat.")
            return

    if user_id == bot.id:
        message.reply_text("I'm not gonna UNMUTE myself, I'm an admin there!")
        return

    try:
        bot.restrict_chat_member(chat.id,
                                 int(user_id),
                                 permissions=ChatPermissions(
                                     can_send_messages=True,
                                     can_send_media_messages=True,
                                     can_send_other_messages=True,
                                     can_add_web_page_previews=True))
        runmuting = "Well, I will let {} speak on {}!".format(
            mention_html(member.user.id, member.user.first_name),
            (chat.title or chat.first or chat.username))
        message.reply_text(runmuting, parse_mode=ParseMode.HTML)
    except BadRequest as excp:
        if excp.message == "Reply message not found":
            # Do not reply
            message.reply_text('Unmuted!', quote=False)
        elif excp.message in RUNMUTE_ERRORS:
            message.reply_text(excp.message)
        else:
            LOGGER.warning(update)
            LOGGER.exception(
                "ERROR unmnuting user %s in chat %s (%s) due to %s", user_id,
                chat.title, chat.id, excp.message)
            message.reply_text("Well damn, I can't unmute that user.")


# based of @1maverick1's snipe command
@bot_admin
def recho(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    try:
        parseId = str(args[0])
        del args[0]
        chat_id = int(parseId)
    except TypeError as excp:
        update.effective_message.reply_text(
            "Please give me a chat to echo to!")
    send = " ".join(args)
    if len(send) >= 2:
        try:
            bot.sendMessage(chat_id, str(send))
        except TelegramError:
            LOGGER.warning("Couldn't echo to group %s", parseId)
            update.effective_message.reply_text("Failed to send message")


__help__ = ""

__mod_name__ = "Remote Commands"

RBAN_HANDLER = CommandHandler("rban",
                              rban,
                              run_async=True,
                              filters=CustomFilters.sudo_filter)
RUNBAN_HANDLER = CommandHandler("runban",
                                runban,
                                run_async=True,
                                filters=CustomFilters.sudo_filter)
RKICK_HANDLER = CommandHandler("rkick",
                               rkick,
                               run_async=True,
                               filters=CustomFilters.sudo_filter)
RMUTE_HANDLER = CommandHandler("rmute",
                               rmute,
                               run_async=True,
                               filters=CustomFilters.sudo_filter)
RUNMUTE_HANDLER = CommandHandler("runmute",
                                 runmute,
                                 run_async=True,
                                 filters=CustomFilters.sudo_filter)
RECHO_HANDLER = CommandHandler("recho",
                               recho,
                               run_async=True,
                               filters=Filters.chat(OWNER_ID))

dispatcher.add_handler(RBAN_HANDLER)
dispatcher.add_handler(RUNBAN_HANDLER)
dispatcher.add_handler(RKICK_HANDLER)
dispatcher.add_handler(RMUTE_HANDLER)
dispatcher.add_handler(RUNMUTE_HANDLER)
dispatcher.add_handler(RECHO_HANDLER)
