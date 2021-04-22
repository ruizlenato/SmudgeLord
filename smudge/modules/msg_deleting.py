import html, time
from typing import Optional, List

from telegram import Message, Chat, Update, Bot, User
from telegram.error import BadRequest
from telegram.ext import CommandHandler, Filters
from telegram.ext.dispatcher import run_async
from telegram.utils.helpers import mention_html

from smudge import dispatcher, CallbackContext, LOGGER
from smudge.helper_funcs.chat_status import user_admin, can_delete, user_can_delete
from smudge.modules.log_channel import loggable
from smudge.modules.translations.strings import tld


@user_admin
@loggable
@user_can_delete
def purge(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    args = context.args
    msg = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]

    if msg.reply_to_message:
        if can_delete(chat, bot.id):
            message_id = msg.reply_to_message.message_id
            delete_to = msg.message_id - 1
            if args and args[0].isdigit():
                new_del = message_id + int(args[0])
                # No point deleting messages which haven't been written yet.
                if new_del < delete_to:
                    delete_to = new_del

            for m_id in range(delete_to, message_id - 1,
                              -1):  # Reverse iteration over message ids
                try:
                    bot.deleteMessage(chat.id, m_id)
                except BadRequest as err:
                    if err.message == "Message can't be deleted":
                        bot.send_message(tld(chat.id, "purge_msg_cant_del_too_old"))

                    elif err.message != "Message to delete not found":
                        LOGGER.exception("Error while purging chat messages.")

            try:
                msg.delete()
            except BadRequest as err:
                if err.message == "Message can't be deleted":
                    bot.send_message(tld(chat.id, "purge_msg_cant_del_too_old"))

                elif err.message != "Message to delete not found":
                    LOGGER.exception("Error while purging chat messages.")

            del_msg = bot.send_message(tld(chat.id, "purge_msg_success"))
            time.sleep(5)
            del_msg.delete()
            return "<b>{}:</b>" \
                   "\n#PURGE" \
                   "\n<b>Admin:</b> {}" \
                   "\nPurged <code>{}</code> messages.".format(html.escape(chat.title),
                                                               mention_html(user.id, user.first_name),
                                                               delete_to - message_id)

    else:
        msg.reply_text(tld(chat.id, "purge_invalid"))
    return ""


@user_admin
@loggable
@user_can_delete
def del_message(update: Update, context: CallbackContext) -> str:
    if not check_perms(update, 0):
        return
    bot = context.bot
    if update.effective_message.reply_to_message:
        user = update.effective_user  # type: Optional[User]
        chat = update.effective_chat  # type: Optional[Chat]

        if can_delete(chat, bot.id):
            update.effective_message.reply_to_message.delete()
            update.effective_message.delete()
            return "<b>{}:</b>" \
                   "\n#DEL" \
                   "\n<b>Admin:</b> {}" \
                   "\nMessage deleted.".format(html.escape(chat.title),
                                               mention_html(user.id, user.first_name))
    else:
        update.effective_message.reply_text(tld(chat.id, "purge_invalid2"))

    return ""

__help__ = True

DELETE_HANDLER = CommandHandler("del",
                                del_message,
                                filters=Filters.chat_type.groups,
                                run_async=True)
PURGE_HANDLER = CommandHandler("purge",
                               purge,
                               filters=Filters.chat_type.groups,
                               run_async=True)

dispatcher.add_handler(DELETE_HANDLER)
dispatcher.add_handler(PURGE_HANDLER)