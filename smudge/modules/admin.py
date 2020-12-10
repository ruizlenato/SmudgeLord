import html
from typing import Optional

from telegram import Message, Chat, Update, User, ChatPermissions
from telegram import ParseMode
from telegram.error import BadRequest
from telegram.ext import CommandHandler, Filters
from telegram.ext.dispatcher import run_async
from telegram.utils.helpers import escape_markdown, mention_html

from smudge import dispatcher, CallbackContext, SUDO_USERS
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.helper_funcs.chat_status import bot_admin, user_admin, can_pin, can_promote, user_can_promote, user_can_pin
from smudge.helper_funcs.extraction import extract_user_and_text, extract_user
from smudge.modules.log_channel import loggable
from smudge.modules.translations.strings import tld

@user_can_promote
@bot_admin
@can_promote
@user_admin
@loggable
def promote(update: Update, context: CallbackContext) -> str:
    args = context.args
    chat_id = update.effective_chat.id
    message = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]

    user_id, title = extract_user_and_text(message, args)

    if not user_id or int(user_id) == 777000 or int(user_id) == 1087968824:
        message.reply_text(tld(chat.id, "common_err_no_user"))
        return ""

    user_member = chat.get_member(user_id)
    if user_member.status == 'administrator' or user_member.status == 'creator':
        message.reply_text(tld(chat.id, "admin_err_user_admin"))
        return ""

    if user_id == context.bot.id:
        message.reply_text(tld(chat.id, "admin_err_self_promote"))
        return ""

    bot_member = chat.get_member(context.bot.id)

    context.bot.promoteChatMember(
        chat_id,
        user_id,
        can_change_info=bot_member.can_change_info,
        can_post_messages=bot_member.can_post_messages,
        can_edit_messages=bot_member.can_edit_messages,
        can_delete_messages=bot_member.can_delete_messages,
        can_invite_users=bot_member.can_invite_users,
        can_restrict_members=bot_member.can_restrict_members,
        can_promote_members=bool(False if user_id not in SUDO_USERS else
                                 bot_member.can_restrict_members),
        can_pin_messages=bot_member.can_pin_messages)

    text = ""
    if title:
        context.bot.set_chat_administrator_custom_title(chat_id, user_id, title[:16])
        text = " with title <code>{}</code>".format(title[:16])

    message.reply_text(tld(chat.id, "admin_promote_success").format(
        mention_html(user_member.user.id, user_member.user.first_name)),
        parse_mode=ParseMode.HTML)
    return "<b>{}:</b>" \
           "\n#PROMOTED" \
           "\n<b>Admin:</b> {}" \
           "\n<b>User:</b> {}".format(html.escape(chat.title),
                                      mention_html(user.id, user.first_name),
                                      mention_html(user_member.user.id, user_member.user.first_name))
@user_can_promote
@bot_admin
@can_promote
@user_admin
@loggable
def demote(update: Update, context: CallbackContext) -> str:
    args = context.args
    chat = update.effective_chat  # type: Optional[Chat]
    message = update.effective_message  # type: Optional[Message]
    user = update.effective_user  # type: Optional[User]

    user_id = extract_user(message, args)

    if not user_id or int(user_id) == 777000 or int(user_id) == 1087968824:
        message.reply_text(tld(chat.id, "common_err_no_user"))
        return ""

    user_member = chat.get_member(user_id)
    if user_member.status == 'creator':
        message.reply_text(tld(chat.id, "admin_err_demote_creator"))
        return ""

    if not user_member.status == 'administrator':
        message.reply_text(tld(chat.id, "admin_err_demote_noadmin"))
        return ""

    if user_id == context.bot.id:
        message.reply_text(tld(chat.id, "admin_err_self_demote"))
        return ""

    try:
        context.bot.restrict_chat_member(chat.id,
                                 int(user_id),
                                 permissions=ChatPermissions(
                                     can_send_messages=True,
                                     can_send_media_messages=True,
                                     can_send_other_messages=True,
                                     can_add_web_page_previews=True)
                                 )  #restrict incase you're demoting a bot
        context.bot.promoteChatMember(int(chat.id),
                              int(user_id),
                              can_change_info=False,
                              can_post_messages=False,
                              can_edit_messages=False,
                              can_delete_messages=False,
                              can_invite_users=False,
                              can_restrict_members=False,
                              can_pin_messages=False,
                              can_promote_members=False)

        message.reply_text(tld(chat.id, "admin_demote_success").format(
            mention_html(user_member.user.id, user_member.user.first_name)),
            parse_mode=ParseMode.HTML)
        return f"<b>{html.escape(chat.title)}:</b>" \
            "\n#DEMOTED" \
               f"\n<b>Admin:</b> {mention_html(user.id, user.first_name)}" \
               f"\n<b>User:</b> {mention_html(user_member.user.id, user_member.user.first_name)}"

    except BadRequest:
        message.reply_text(tld(chat.id, "admin_err_cant_demote"))
        return ""

@user_can_pin
@bot_admin
@can_pin
@user_admin
@loggable
def pin(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    user = update.effective_user  # type: Optional[User]
    chat = update.effective_chat  # type: Optional[Chat]

    is_group = chat.type != "private" and chat.type != "channel"

    prev_message = update.effective_message.reply_to_message

    is_silent = True
    if len(args) >= 1:
        is_silent = not (args[0].lower() == 'notify' or args[0].lower()
                         == 'loud' or args[0].lower() == 'violent')

    if prev_message and is_group:
        try:
            bot.pinChatMessage(chat.id,
                               prev_message.message_id,
                               disable_notification=is_silent)
        except BadRequest as excp:
            if excp.message == "Chat_not_modified":
                pass
            else:
                raise
        return "<b>{}:</b>" \
               "\n#PINNED" \
               "\n<b>Admin:</b> {}".format(html.escape(chat.title), mention_html(user.id, user.first_name))

    return ""

@user_can_pin
@bot_admin
@can_pin
@user_admin
@loggable
def unpin(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    chat = update.effective_chat
    user = update.effective_user  # type: Optional[User]
    args = {}

    if update.effective_message.reply_to_message:
        args[
            "message_id"] = update.effective_message.reply_to_message.message_id

    try:
        bot.unpinChatMessage(chat.id, **args)
    except BadRequest as excp:
        if excp.message == "Chat_not_modified":
            pass
        else:
            raise

    return "<b>{}:</b>" \
           "\n#UNPINNED" \
           "\n<b>Admin:</b> {}".format(html.escape(chat.title),
                                       mention_html(user.id, user.first_name))


@bot_admin
@can_pin
@user_admin
@loggable
def unpinall(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    chat = update.effective_chat
    user = update.effective_user  # type: Optional[User]

    try:
        bot.unpinAllChatMessages(chat.id)
        update.effective_message.reply_text(
            "Successfully unpinned all messages!")
    except BadRequest as excp:
        if excp.message == "Chat_not_modified":
            pass
        else:
            raise

    return "<b>{}:</b>" \
           "\n#UNPINNED" \
           "\n<b>Admin:</b> {}".format(html.escape(chat.title),
                                       mention_html(user.id, user.first_name))


@bot_admin
@user_admin
def invite(update: Update, context: CallbackContext):
    bot = context.bot
    chat = update.effective_chat  # type: Optional[Chat]
    if chat.username:
        update.effective_message.reply_text("@{}".format(chat.username))
    elif chat.type == chat.SUPERGROUP or chat.type == chat.CHANNEL:
        bot_member = chat.get_member(bot.id)
        if bot_member.can_invite_users:
            invitelink = bot.exportChatInviteLink(chat.id)
            update.effective_message.reply_text(invitelink)
        else:
            update.effective_message.reply_text(
                "I don't have access to the invite link, try changing my permissions!"
            )
    else:
        update.effective_message.reply_text(
            "I can only give you invite links for supergroups and channels, sorry!"
        )


def adminlist(update: Update, context: CallbackContext):
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]

    administrators = chatP.get_administrators()

    text = tld(chat.id,
               "admin_list").format(chatP.title
                                    or tld(chat.id, "admin_this_chat"))
    for admin in administrators:
        user = admin.user
        status = admin.status
        name = user.first_name
        name += f" {user.last_name}" if user.last_name else ""
        name += tld(chat.id,
                    "admin_list_creator") if status == "creator" else ""
        text += f"\n• `{name}`"

    update.effective_message.reply_text(text, parse_mode=ParseMode.MARKDOWN)


__help__ = True

PIN_HANDLER = CommandHandler("pin", pin, pass_args=True, filters=Filters.chat_type.groups, run_async=True)
UNPIN_HANDLER = CommandHandler("unpin", unpin, filters=Filters.chat_type.groups, run_async=True)
UNPINALL_HANDLER = CommandHandler("unpinall", unpinall, filters=Filters.chat_type.groups, run_async=True)
INVITE_HANDLER = CommandHandler(["invitelink", "link"], invite, filters=Filters.chat_type.groups, run_async=True)

PROMOTE_HANDLER = CommandHandler("promote", promote, pass_args=True, filters=Filters.chat_type.groups, run_async=True)
DEMOTE_HANDLER = CommandHandler("demote", demote, pass_args=True, filters=Filters.chat_type.groups, run_async=True)

ADMINLIST_HANDLER = DisableAbleCommandHandler(["adminlist", "admins"], adminlist, filters=Filters.chat_type.groups, run_async=True)

dispatcher.add_handler(PIN_HANDLER)
dispatcher.add_handler(UNPIN_HANDLER)
dispatcher.add_handler(UNPINALL_HANDLER)
dispatcher.add_handler(INVITE_HANDLER)
dispatcher.add_handler(PROMOTE_HANDLER)
dispatcher.add_handler(DEMOTE_HANDLER)
dispatcher.add_handler(ADMINLIST_HANDLER)
