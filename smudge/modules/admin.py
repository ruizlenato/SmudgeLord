import html
from typing import Optional, List

from telegram import Message, Chat, Update, Bot, User
from telegram import ParseMode
from telegram.error import BadRequest
from telegram.ext import CommandHandler, MessageHandler, Filters
from telegram.ext.dispatcher import run_async
from telegram.utils.helpers import mention_html

from smudge import dispatcher
from smudge.modules.connection import connected
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.modules.helper_funcs.chat_status import bot_admin, user_admin, can_pin
from smudge.modules.helper_funcs.extraction import extract_user
from smudge.modules.log_channel import loggable
from smudge.modules.sql import admin_sql as sql
from smudge.modules.translations.strings import tld


@run_async
@bot_admin
@user_admin
@loggable
def promote(bot: Bot, update: Update, args: List[str]) -> str:
    message = update.effective_message  # type: Optional[Message]
    user = update.effective_user  # type: Optional[User]
    chat = update.effective_chat  # type: Optional[Chat]
    conn = connected(bot, update, chat, user.id)
    if conn:
        chatD = dispatcher.bot.getChat(conn)
    else:
        chatD = update.effective_chat
        if chat.type == "private":
            exit(1)

    if not chatD.get_member(bot.id).can_promote_members:
	        update.effective_message.reply_text("I can't promote/demote people here! "
                                            "Make sure I'm admin and can appoint new admins.")

    user_id = extract_user(message, args)
    if not user_id:
        message.reply_text(tld(chat.id, "You don't seem to be referring to a user."))
        return ""

    user_member = chatD.get_member(user_id)
    if user_member.status == 'administrator' or user_member.status == 'creator':
        message.reply_text(tld(chat.id, "How am I meant to promote someone who's already an admin?"))
        return ""

    if user_id == bot.id:
        message.reply_text(tld(chat.id, "I can't promote myself! Get an admin to do it for me."))
        return ""

    # set same perms as bot - bot can't assign higher perms than itself!
    bot_member = chatD.get_member(bot.id)

    bot.promoteChatMember(chatD.id, user_id,
                          can_change_info=bot_member.can_change_info,
                          can_post_messages=bot_member.can_post_messages,
                          can_edit_messages=bot_member.can_edit_messages,
                          can_delete_messages=bot_member.can_delete_messages,
                          # can_invite_users=bot_member.can_invite_users,
                          can_restrict_members=bot_member.can_restrict_members,
                          can_pin_messages=bot_member.can_pin_messages,
                          can_promote_members=bot_member.can_promote_members)

    message.reply_text(tld(chat.id, f"Successfully promoted in *{chatD.title}*!"), parse_mode=ParseMode.MARKDOWN)
    return f"<b>{html.escape(chatD.title)}:</b>" \
           "\n#PROMOTED" \
           f"\n<b>• Admin:</b> {mention_html(user.id, user.first_name)}" \
           f"\n<b>• User:</b> {mention_html(user_member.user.id, user_member.user.first_name)}"


@run_async
@bot_admin
@user_admin
@loggable
def demote(bot: Bot, update: Update, args: List[str]) -> str:
    chat = update.effective_chat  # type: Optional[Chat]
    message = update.effective_message  # type: Optional[Message]
    user = update.effective_user  # type: Optional[User]
    conn = connected(bot, update, chat, user.id)
    if conn:
        chatD = dispatcher.bot.getChat(conn)
    else:
        chatD = update.effective_chat
        if chat.type == "private":
            exit(1)

    if not chatD.get_member(bot.id).can_promote_members:
        update.effective_message.reply_text("I can't promote/demote people here! "
                                            "Make sure I'm admin and can appoint new admins.")
        exit(1)

    user_id = extract_user(message, args)
    if not user_id:
        message.reply_text(tld(chat.id, "You don't seem to be referring to a user."))
        return ""

    user_member = chatD.get_member(user_id)
    if user_member.status == 'creator':
        message.reply_text(tld(chat.id, "This person is the CREATOR of the chat, how would I demote them?"))
        return ""

    if not user_member.status == 'administrator':
        message.reply_text(tld(chat.id, "Can't demote those who weren't promoted!"))
        return ""

    if user_id == bot.id:
        message.reply_text(tld(chat.id, "I can't demote myself!"))
        return ""

    try:
        bot.promoteChatMember(int(chatD.id), int(user_id),
                              can_change_info=False,
                              can_post_messages=False,
                              can_edit_messages=False,
                              can_delete_messages=False,
                              can_invite_users=False,
                              can_restrict_members=False,
                              can_pin_messages=False,
                              can_promote_members=False)
        message.reply_text(tld(chat.id, f"Successfully demoted in *{chatD.title}*!"), parse_mode=ParseMode.MARKDOWN)
        return f"<b>{html.escape(chatD.title)}:</b>" \
               "\n#DEMOTED" \
               f"\n<b>• Admin:</b> {mention_html(user.id, user.first_name)}" \
               f"\n<b>• User:</b> {mention_html(user_member.user.id, user_member.user.first_name)}"

    except BadRequest:
        message.reply_text(
            tld(chat.id,
                "Could not demote. I might not be admin, or the admin status was appointed by another user, so I can't act upon them!")
        )
        return ""


@run_async
@bot_admin
@can_pin
@user_admin
@loggable
def pin(bot: Bot, update: Update, args: List[str]) -> str:
    user = update.effective_user  # type: Optional[User]
    chat = update.effective_chat  # type: Optional[Chat]

    is_group = chat.type != "private" and chat.type != "channel"

    prev_message = update.effective_message.reply_to_message

    is_silent = True
    if len(args) >= 1:
        is_silent = not (args[0].lower() == 'notify' or args[0].lower() == 'loud' or args[0].lower() == 'violent')

    if prev_message and is_group:
        try:
            bot.pinChatMessage(chat.id, prev_message.message_id, disable_notification=is_silent)
        except BadRequest as excp:
            if excp.message == "Chat_not_modified":
                pass
            else:
                raise
        return f"<b>{html.escape(chat.title)}:</b>" \
               "\n#PINNED" \
               f"\n<b>• Admin:</b> {mention_html(user.id, user.first_name)}"

    return ""


@run_async
@bot_admin
@can_pin
@user_admin
@loggable
def unpin(bot: Bot, update: Update) -> str:
    chat = update.effective_chat
    user = update.effective_user  # type: Optional[User]

    try:
        bot.unpinChatMessage(chat.id)
    except BadRequest as excp:
        if excp.message == "Chat_not_modified":
            pass
        else:
            raise

    return f"<b>{html.escape(chat.title)}:</b>" \
           "\n#UNPINNED" \
           f"\n<b>• Admin:</b> {mention_html(user.id, user.first_name)}"


@run_async
@bot_admin
@user_admin
def invite(bot: Bot, update: Update):
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    conn = connected(bot, update, chat, user.id, need_admin=False)
    if conn:
        chatP = dispatcher.bot.getChat(conn)
    else:
        chatP = update.effective_chat
        if chat.type == "private":
            exit(1)

    if chatP.username:
        update.effective_message.reply_text(chatP.username)
    elif chatP.type == chatP.SUPERGROUP or chatP.type == chatP.CHANNEL:
        bot_member = chatP.get_member(bot.id)
        if bot_member.can_invite_users:
            invitelink = chatP.invite_link
            # print(invitelink)
            if not invitelink:
                invitelink = bot.exportChatInviteLink(chatP.id)

            update.effective_message.reply_text(invitelink)
        else:
            update.effective_message.reply_text(
                tld(chat.id, "I don't have access to the invite link, try changing my permissions!"))
    else:
        update.effective_message.reply_text(
            tld(chat.id, "Sorry, but I can give invite links only for supergroups and channels!"))


@run_async
def adminlist(bot, update):
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    conn = connected(bot, update, chat, user.id, need_admin=False)
    if conn:
        chatP = dispatcher.bot.getChat(conn)
    else:
        chatP = update.effective_chat
        if chat.type == "private":
            exit(1)

    administrators = chatP.get_administrators()

    text = tld(chat.id, "Admins in") + " *{}*:".format(chatP.title or tld(chat.id, "this chat"))
    for admin in administrators:
        user = admin.user
        status = admin.status
        if status == "creator":
            name = user.first_name + (user.last_name or "") + tld(chat.id, " (Creator)")
        else:
            name = user.first_name + (user.last_name or "")
        text += f"\n• `{name}`"

    update.effective_message.reply_text(text, parse_mode=ParseMode.MARKDOWN)


@can_pin
@user_admin
@run_async
def permanent_pin_set(bot: Bot, update: Update, args: List[str]) -> str:
    user = update.effective_user  # type: Optional[User]
    chat = update.effective_chat  # type: Optional[Chat]

    conn = connected(bot, update, chat, user.id, need_admin=True)
    if conn:
        chat = dispatcher.bot.getChat(conn)
        chat_id = conn
        chat_name = dispatcher.bot.getChat(conn).title
        if not args:
            get_permapin = sql.get_permapin(chat_id)
            text_maker = "Permanent pin is currently set: `{}`".format(bool(int(get_permapin)))
            if get_permapin:
                if chat.username:
                    old_pin = "https://t.me/{}/{}".format(chat.username, get_permapin)
                else:
                    old_pin = "https://t.me/c/{}/{}".format(str(chat.id)[4:], get_permapin)
                text_maker += "\nTo disable a permanent pin: `/permanentpin off`"
                text_maker += "\n\n[The permanent pin message is here]({})".format(old_pin)
            update.effective_message.reply_text(text_maker, parse_mode="markdown")
            return ""
        prev_message = args[0]
        if prev_message == "off":
            sql.set_permapin(chat_id, 0)
            update.effective_message.reply_text("The permanent pin has been disabled!")
            return
        if "/" in prev_message:
            prev_message = prev_message.split("/")[-1]
    else:
        if update.effective_message.chat.type == "private":
            update.effective_message.reply_text("You can do this command in the group, not in PM")
            return ""
        chat = update.effective_chat
        chat_id = update.effective_chat.id
        chat_name = update.effective_message.chat.title
        if update.effective_message.reply_to_message:
            prev_message = update.effective_message.reply_to_message.message_id
        elif len(args) >= 1 and args[0] == "off":
            sql.set_permapin(chat.id, 0)
            update.effective_message.reply_text("The permanent pin has been disabled!")
            return
        else:
            get_permapin = sql.get_permapin(chat_id)
            text_maker = "Permanent pin is currently set: `{}`".format(bool(int(get_permapin)))
            if get_permapin:
                if chat.username:
                    old_pin = "https://t.me/{}/{}".format(chat.username, get_permapin)
                else:
                    old_pin = "https://t.me/c/{}/{}".format(str(chat.id)[4:], get_permapin)
                text_maker += "\nTo disable the permanent pin: `/permanentpin off`"
                text_maker += "\n\n[The permanent pin message is here]({})".format(old_pin)
            update.effective_message.reply_text(text_maker, parse_mode="markdown")
            return ""

    is_group = chat.type != "private" and chat.type != "channel"

    if prev_message and is_group:
        sql.set_permapin(chat.id, prev_message)
        update.effective_message.reply_text("Permanent pin successfully set!")
        return "<b>{}:</b>" \
               "\n#PERMANENT_PIN" \
               "\n<b>• Admin:</b> {}".format(html.escape(chat.title), mention_html(user.id, user.first_name))

    return ""


@run_async
def permanent_pin(bot: Bot, update: Update):
    user = update.effective_user  # type: Optional[User]
    chat = update.effective_chat  # type: Optional[Chat]
    message = update.effective_message

    get_permapin = sql.get_permapin(chat.id)
    if get_permapin and not user.id == bot.id:
        try:
            to_del = bot.pinChatMessage(chat.id, get_permapin, disable_notification=True)
        except BadRequest:
            sql.set_permapin(chat.id, 0)
            if chat.username:
                old_pin = "https://t.me/{}/{}".format(chat.username, get_permapin)
            else:
                old_pin = "https://t.me/c/{}/{}".format(str(chat.id)[4:], get_permapin)
            message.reply_text("*Permanent pin error:*\nI can't pin messages here!\nMake sure I am an admin and can pin messages.\n\nPin permanently disabled, [old permanent pin message is here]({})".format(old_pin), parse_mode="markdown")
            return

        if to_del:
            try:
                bot.deleteMessage(chat.id, message.message_id+1)
            except BadRequest:
                print("Permanent pin error: cannot delete pin msg")


# TODO: Finalize this command, add automatic message deleting
@user_admin
@run_async
def reaction(bot: Bot, update: Update, args: List[str]) -> str:
    chat = update.effective_chat  # type: Optional[Chat]
    if len(args) >= 1:
        var = args[0]
        print(var)
        if var == "False":
            sql.set_command_reaction(chat.id, False)
            update.effective_message.reply_text("Disabled response to user-triggered admin commands.")
        elif var == "True":
            sql.set_command_reaction(chat.id, True)
            update.effective_message.reply_text("Enabled response to user-triggered admin commands.")
        else:
            update.effective_message.reply_text("Please enter True or False!", parse_mode=ParseMode.MARKDOWN)
    else:
        status = sql.command_reaction(chat.id)
        update.effective_message.reply_text(
            "Response for user-triggered admin commands is currently "
            f"`{'enabled' if status == True else 'disabled'}`!",
            parse_mode=ParseMode.MARKDOWN
        )


__help__ = """
*Make it easy to promote and demote users and keep your chat up to date on the latest news with a simple pinned message!*

Available commands:
 - /adminlist: list the admins in the current chat.

*Admin only:*
 - /promote: promote a user.
 - /demote: demote a user.
 - /pin: pin the message you replied to; add 'loud' or 'notify' to send a notification to group members.
 - /unpin: Unpin the currently pinned message.
 - /permanentpin: Set a permanent pin for supergroup chat, when an admin or telegram channel change pinned message, bot will change pinned message immediatelly
 
An example of promoting someone to admins:
`/promote @username`; this promotes a user to admins.
"""

__mod_name__ = "Admin"

PIN_HANDLER = DisableAbleCommandHandler("pin", pin, pass_args=True, filters=Filters.group)
UNPIN_HANDLER = DisableAbleCommandHandler("unpin", unpin, filters=Filters.group)

INVITE_HANDLER = CommandHandler("invitelink", invite)

PROMOTE_HANDLER = DisableAbleCommandHandler("promote", promote, pass_args=True)
DEMOTE_HANDLER = DisableAbleCommandHandler("demote", demote, pass_args=True)

PERMANENT_PIN_SET_HANDLER = CommandHandler("permanentpin", permanent_pin_set, pass_args=True, filters=Filters.group)
PERMANENT_PIN_HANDLER = MessageHandler(Filters.status_update.pinned_message | Filters.user(777000), permanent_pin)

REACT_HANDLER = DisableAbleCommandHandler("reaction", reaction, pass_args=True, filters=Filters.group)

ADMINLIST_HANDLER = DisableAbleCommandHandler(["adminlist", "admins"], adminlist)

dispatcher.add_handler(PIN_HANDLER)
dispatcher.add_handler(UNPIN_HANDLER)
dispatcher.add_handler(INVITE_HANDLER)
dispatcher.add_handler(PROMOTE_HANDLER)
dispatcher.add_handler(DEMOTE_HANDLER)
dispatcher.add_handler(PERMANENT_PIN_SET_HANDLER)
dispatcher.add_handler(PERMANENT_PIN_HANDLER)
dispatcher.add_handler(ADMINLIST_HANDLER)
dispatcher.add_handler(REACT_HANDLER)
