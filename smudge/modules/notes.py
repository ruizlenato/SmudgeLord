import re

from telegram import MAX_MESSAGE_LENGTH, ParseMode, InlineKeyboardMarkup
from telegram import Bot, Update
from telegram.error import BadRequest
from telegram.ext import CallbackContext, CommandHandler, CallbackQueryHandler, Filters, MessageHandler
from telegram.ext.dispatcher import run_async
import smudge.modules.sql.notes_sql as sql
from smudge import dispatcher, MESSAGE_DUMP, LOGGER
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.helper_funcs.chat_status import user_admin
from smudge.helper_funcs.misc import build_keyboard, revert_buttons
from smudge.helper_funcs.msg_types import get_note_type

from smudge.modules.translations.strings import tld
from smudge.modules.connection import connected

FILE_MATCHER = re.compile(r"^###file_id(!photo)?###:(.*?)(?:\s|$)")
STICKER_MATCHER = re.compile(r"^###sticker(!photo)?###:")
BUTTON_MATCHER = re.compile(r"^###button(!photo)?###:(.*?)(?:\s|$)")
MYFILE_MATCHER = re.compile(r"^###file(!photo)?###:")
MYPHOTO_MATCHER = re.compile(r"^###photo(!photo)?###:")
MYAUDIO_MATCHER = re.compile(r"^###audio(!photo)?###:")
MYVOICE_MATCHER = re.compile(r"^###voice(!photo)?###:")
MYVIDEO_MATCHER = re.compile(r"^###video(!photo)?###:")

ENUM_FUNC_MAP = {
    sql.Types.TEXT.value: dispatcher.bot.send_message,
    sql.Types.BUTTON_TEXT.value: dispatcher.bot.send_message,
    sql.Types.STICKER.value: dispatcher.bot.send_sticker,
    sql.Types.DOCUMENT.value: dispatcher.bot.send_document,
    sql.Types.PHOTO.value: dispatcher.bot.send_photo,
    sql.Types.AUDIO.value: dispatcher.bot.send_audio,
    sql.Types.VOICE.value: dispatcher.bot.send_voice,
    sql.Types.VIDEO.value: dispatcher.bot.send_video,
}


# Do not async
def get(update: Update, context: CallbackContext, notename, show_none=True, no_format=False):
    bot, args = context.bot, context.args
    chat = update.effective_chat
    user = update.effective_user
    conn = connected(update, context, chat, user.id, need_admin=False)
    if conn:
        chat_id = conn
        send_id = user.id
    else:
        chat_id = update.effective_chat.id
        send_id = chat_id

    note = sql.get_note(chat_id, notename)
    message = update.effective_message

    if note:
        pass
    elif show_none:
        message.reply_text(tld(chat.id, "note_not_existed"))
        return

    # If we're replying to a message, reply to that message (unless it's an error)
    if message.reply_to_message:
        reply_id = message.reply_to_message.message_id
    else:
        reply_id = message.message_id

    if note and note.is_reply:
        if MESSAGE_DUMP:
            try:
                context.bot.forward_message(chat_id=chat_id,
                                            from_chat_id=MESSAGE_DUMP,
                                            message_id=note.value)
            except BadRequest as excp:
                if excp.message == "Message to forward not found":
                    message.reply_text(tld(chat.id, "note_lost"))
                    sql.rm_note(chat_id, notename)
                else:
                    raise
        else:
            try:
                context.bot.forward_message(chat_id=chat_id,
                                            from_chat_id=chat_id,
                                            message_id=note.value)

            except BadRequest as excp:
                if excp.message == "Message to forward not found":
                    message.reply_text(tld(chat.id, "note_msg_del"))
                sql.rm_note(chat_id, notename)

            else:
                raise
    else:
        if note:
            text = note.value
        else:
            text = None

        keyb = []
        parseMode = ParseMode.MARKDOWN
        buttons = sql.get_buttons(chat_id, notename)
        if no_format:
            parseMode = None
            text += revert_buttons(buttons)
        else:
            keyb = build_keyboard(buttons)

        keyboard = InlineKeyboardMarkup(keyb)

        try:
            if note and note.msgtype in (sql.Types.BUTTON_TEXT,
                                         sql.Types.TEXT):
                try:
                    context.bot.send_message(send_id,
                                             text,
                                             reply_to_message_id=reply_id,
                                             parse_mode=parseMode,
                                             disable_web_page_preview=True,
                                             reply_markup=keyboard)
                except BadRequest as excp:
                    if excp.message == "Wrong http url":
                        failtext = tld(chat.id, "note_url_invalid")
                        failtext += "\n\n```\n{}```".format(
                            note.value + revert_buttons(buttons))
                        message.reply_text(failtext, parse_mode="markdown")

            else:
                if note:
                    ENUM_FUNC_MAP[note.msgtype](send_id,
                                                note.file,
                                                caption=text,
                                                reply_to_message_id=reply_id,
                                                parse_mode=parseMode,
                                                disable_web_page_preview=True,
                                                reply_markup=keyboard)

        except BadRequest as excp:
            if excp.message == "Entity_mention_user_invalid":
                message.reply_text(tld(chat.id, "note_mention_invalid"))

            elif FILE_MATCHER.match(note.value):
                message.reply_text(tld(chat.id, "note_incorrect_import"))
                sql.rm_note(chat_id, notename)
            else:
                message.reply_text(tld(chat.id, "note_cannot_send"))
                LOGGER.exception("Could not parse message #%s in chat %s",
                                 notename, str(chat_id))
                LOGGER.warning("Message was: %s", str(note.value))

    return


def cmd_get(update: Update, context: CallbackContext):
    bot, args = context.bot, context.args
    chat = update.effective_chat
    if len(args) >= 2 and args[1].lower() == "noformat":
        get(update, context, args[0].lower(), show_none=True, no_format=True)
    elif len(args) >= 1:
        get(update, context, args[0].lower(), show_none=True)
    else:
        update.effective_message.reply_text(tld(chat.id, "get_invalid"))


def hash_get(update: Update, context: CallbackContext):
    message = update.effective_message.text
    fst_word = message.split()[0]
    no_hash = fst_word[1:].lower()
    get(update, context, no_hash, show_none=False)


# TODO: FIX THIS
@user_admin
def save(update: Update, context: CallbackContext):
    chat = update.effective_chat
    user = update.effective_user
    conn = connected(update, context, chat, user.id)
    if conn:
        chat_id = conn
        chat_name = dispatcher.bot.getChat(conn).title
    else:
        chat_id = update.effective_chat.id
        if chat.type == "private":
            chat_name = tld(chat.id, "note_is_local")
        else:
            chat_name = chat.title

    msg = update.effective_message

    note_name, text, data_type, content, buttons = get_note_type(msg)
    note_name = note_name.lower()

    if data_type is None:
        msg.reply_text(tld(chat.id, "save_invalid"))
        return

    if not sql.get_note(chat_id, note_name):
        sql.add_note_to_db(chat_id,
                           note_name,
                           text,
                           data_type,
                           buttons=buttons,
                           file=content)
        msg.reply_text(tld(chat.id,
                           "save_success").format(note_name, chat_name,
                                                  note_name, note_name),
                       parse_mode=ParseMode.MARKDOWN)
    else:
        sql.add_note_to_db(chat_id,
                           note_name,
                           text,
                           data_type,
                           buttons=buttons,
                           file=content)
        msg.reply_text(tld(chat.id,
                           "save_updated").format(note_name, chat_name,
                                                  note_name, note_name),
                       parse_mode=ParseMode.MARKDOWN)


@user_admin
def clear(update: Update, context: CallbackContext):
    args = context.args
    user = update.effective_user
    chat = update.effective_chat
    conn = connected(update, context, chat, user.id)
    if conn:
        chat_id = conn
        chat_name = dispatcher.bot.getChat(conn).title
    else:
        chat_id = update.effective_chat.id
        if chat.type == "private":
            chat_name = tld(chat.id, "note_is_local")
        else:
            chat_name = chat.title

    if len(args) >= 1:
        notename = args[0].lower()

        if sql.rm_note(chat_id, notename):
            update.effective_message.reply_text(tld(
                chat.id, "clear_success").format(chat_name),
                parse_mode=ParseMode.MARKDOWN)
        else:
            update.effective_message.reply_text(
                tld(chat.id, "note_not_existed"))


def list_notes(update: Update, context: CallbackContext):
    chat = update.effective_chat
    user = update.effective_user
    conn = connected(update, context, chat, user.id, need_admin=False)
    if conn:
        chat_id = conn
        chat_name = dispatcher.bot.getChat(conn).title
        msg = tld(chat.id, "note_in_chat")
    else:
        chat_id = update.effective_chat.id
        if chat.type == "private":
            chat_name = tld(chat.id, "note_is_local")
            msg = tld(chat.id, "note_in_local")
        else:
            chat_name = chat.title
            msg = tld(chat.id, "note_in_chat")

    note_list = sql.get_all_chat_notes(chat_id)

    for note in note_list:
        note_name = " • `{}`\n".format(note.name.lower())
        if len(msg) + len(note_name) > MAX_MESSAGE_LENGTH:
            update.effective_message.reply_text(msg,
                                                parse_mode=ParseMode.MARKDOWN)
            msg = ""
        msg += note_name

    if not note_list:
        update.effective_message.reply_text(tld(
            chat.id, "note_none_in_chat").format(chat_name),
            parse_mode=ParseMode.MARKDOWN)

    elif len(msg) != 0:
        msg += tld(chat.id, "note_get")
        update.effective_message.reply_text(msg.format(chat_name),
                                            parse_mode=ParseMode.MARKDOWN)


@user_admin
def remove_all_notes(update: Update, context: CallbackContext):
    chat = update.effective_chat
    user = update.effective_user
    message = update.effective_message

    if chat.type == "private":
        chat.title = tld(chat.id, "note_is_local")
    else:
        owner = chat.get_member(user.id)
        chat.title = chat.title
        if owner.status != 'creator':
            message.reply_text(tld(chat.id, "notes_must_be_creator"))
            return

    note_list = sql.get_all_chat_notes(chat.id)
    if not note_list:
        message.reply_text(tld(chat.id,
                               "note_none_in_chat").format(chat.title),
                           parse_mode=ParseMode.MARKDOWN)
        return

    x = 0
    a_note = []
    for notename in note_list:
        x += 1
        note = notename.name.lower()
        a_note.append(note)

    for note in a_note:
        sql.rm_note(chat.id, note)

    message.reply_text(tld(chat.id, "notes_cleanup_success").format(x))


def __stats__():
    return "• `{}` notes, accross `{}` chats.".format(sql.num_notes(),
                                                      sql.num_chats())


def __migrate__(old_chat_id, new_chat_id):
    sql.migrate_chat(old_chat_id, new_chat_id)


__help__ = True

GET_HANDLER = DisableAbleCommandHandler(
    "get", cmd_get, pass_args=True, run_async=True)
HASH_GET_HANDLER = MessageHandler(
    Filters.regex(r"^#[^\s]+"), hash_get, run_async=True)
SAVE_HANDLER = CommandHandler("save", save, run_async=True)
REMOVE_ALL_NOTES_HANDLER = CommandHandler("clearall", remove_all_notes)
DELETE_HANDLER = CommandHandler("clear", clear, pass_args=True, run_async=True)

LIST_HANDLER = DisableAbleCommandHandler(["notes", "saved"],
                                         list_notes,
                                         admin_ok=True, run_async=True)

dispatcher.add_handler(GET_HANDLER)
dispatcher.add_handler(SAVE_HANDLER)
dispatcher.add_handler(LIST_HANDLER)
dispatcher.add_handler(DELETE_HANDLER)
dispatcher.add_handler(HASH_GET_HANDLER)
dispatcher.add_handler(REMOVE_ALL_NOTES_HANDLER)
