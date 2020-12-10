import json
import time
import os
from io import BytesIO

from telegram import ParseMode
from telegram import Update, Bot
from telegram.error import BadRequest
from telegram.ext import CommandHandler, run_async

import smudge.modules.sql.notes_sql as sql
from smudge import dispatcher, LOGGER, MESSAGE_DUMP
from smudge.__main__ import DATA_IMPORT
from smudge.helper_funcs.chat_status import user_admin
import smudge.modules.sql.rules_sql as rulessql
import smudge.modules.sql.blacklist_sql as blacklistsql
from smudge.modules.sql import disable_sql as disabledsql
import smudge.modules.sql.locks_sql as locksql
from smudge.modules.connection import connected
from smudge.modules.translations.strings import tld


@run_async
@user_admin
def import_data(bot: Bot, update):
    msg = update.effective_message
    chat = update.effective_chat
    user = update.effective_user
    # TODO: allow uploading doc with command, not just as reply
    # only work with a doc

    conn = connected(bot, update, chat, user.id, need_admin=True)
    if conn:
        chat = dispatcher.bot.getChat(conn)
        chat_name = dispatcher.bot.getChat(conn).title
    else:
        if update.effective_message.chat.type == "private":
            update.effective_message.reply_text(
                tld(chat.id, "common_cmd_group_only"))
            return ""

        chat = update.effective_chat
        chat_name = update.effective_message.chat.title

    if msg.reply_to_message and msg.reply_to_message.document:
        try:
            file_info = bot.get_file(msg.reply_to_message.document.file_id)
        except BadRequest:
            msg.reply_text(tld(chat.id, "backups_file_corrupted"))
            return

        with BytesIO() as file:
            file_info.download(out=file)
            file.seek(0)
            data = json.load(file)

        # only import one group
        if len(data) > 1 and str(chat.id) not in data:
            msg.reply_text(tld(chat.id, "backups_contains_multiple_chats"))
            return

        # Check if backup is this chat
        try:
            if data.get(str(chat.id)) == None:
                if conn:
                    text = tld(
                        chat.id,
                        "backups_from_another_chat").format(f"*{chat_name}*")
                else:
                    text = tld(chat.id,
                               "backups_from_another_chat").format("this chat")
                return msg.reply_text(text, parse_mode="markdown")
        except Exception:
            return msg.reply_text(tld(chat.id, "backups_err_unknown"))
        # Check if backup is from self
        try:
            if str(bot.id) != str(data[str(chat.id)]['bot']):
                return msg.reply_text(tld(chat.id, "backups_from_another_bot"))
        except Exception:
            pass
        # Select data source
        if str(chat.id) in data:
            data = data[str(chat.id)]['hashes']
        else:
            data = data[list(data.keys())[0]]['hashes']

        try:
            for mod in DATA_IMPORT:
                mod.__import_data__(str(chat.id), data)
        except Exception:
            msg.reply_text(tld(chat.id, "backups_restore_err"))

            LOGGER.exception("Imprt for the chat %s with the name %s failed.",
                             str(chat.id), str(chat.title))
            return

        # TODO: some of that link logic
        # NOTE: consider default permissions stuff?
        if conn:
            text = tld(chat.id,
                       "backups_fully_restored").format(f" to *{chat_name}*")
        else:
            text = tld(chat.id, "backups_fully_restored").format("")
        msg.reply_text(text, parse_mode="markdown")


@run_async
@user_admin
def export_data(bot: Bot, update: Update, chat_data):
    msg = update.effective_message
    user = update.effective_user

    chat_id = update.effective_chat.id
    chat = update.effective_chat
    current_chat_id = update.effective_chat.id

    conn = connected(bot, update, chat, user.id, need_admin=True)
    if conn:
        chat = dispatcher.bot.getChat(conn)
        chat_id = conn
    else:
        if update.effective_message.chat.type == "private":
            update.effective_message.reply_text(
                tld(chat.id, "common_cmd_group_only"))
            return ""
        chat = update.effective_chat
        chat_id = update.effective_chat.id

    jam = time.time()
    new_jam = jam + 10800
    checkchat = get_chat(chat_id, chat_data)
    if checkchat.get('status'):
        if jam <= int(checkchat.get('value')):
            timeformatt = time.strftime("%H:%M:%S %d/%m/%Y",
                                        time.localtime(checkchat.get('value')))
            update.effective_message.reply_text(tld(
                chat.id, "backups_err_timelimit").format(timeformatt),
                parse_mode=ParseMode.MARKDOWN)
            return
        if user.id != 654839744:
            put_chat(chat_id, new_jam, chat_data)
    else:
        if user.id != 654839744:
            put_chat(chat_id, new_jam, chat_data)

    note_list = sql.get_all_chat_notes(chat_id)
    backup = {}
    notes = {}
    buttonlist = []
    namacat = ""
    isicat = ""
    rules = ""
    count = 0
    countbtn = 0
    # Notes
    for note in note_list:
        count += 1
        namacat += '{}<###splitter###>'.format(note.name)
        if note.msgtype == 1:
            tombol = sql.get_buttons(chat_id, note.name)
            for btn in tombol:
                countbtn += 1
                if btn.same_line:
                    buttonlist.append(
                        ('{}'.format(btn.name), '{}'.format(btn.url), True))
                else:
                    buttonlist.append(
                        ('{}'.format(btn.name), '{}'.format(btn.url), False))
            isicat += '###button###: {}<###button###>{}<###splitter###>'.format(
                note.value, str(buttonlist))
            buttonlist.clear()
        elif note.msgtype == 2:
            isicat += '###sticker###:{}<###splitter###>'.format(note.file)
        elif note.msgtype == 3:
            isicat += '###file###:{}<###TYPESPLIT###>{}<###splitter###>'.format(
                note.file, note.value)
        elif note.msgtype == 4:
            isicat += '###photo###:{}<###TYPESPLIT###>{}<###splitter###>'.format(
                note.file, note.value)
        elif note.msgtype == 5:
            isicat += '###audio###:{}<###TYPESPLIT###>{}<###splitter###>'.format(
                note.file, note.value)
        elif note.msgtype == 6:
            isicat += '###voice###:{}<###TYPESPLIT###>{}<###splitter###>'.format(
                note.file, note.value)
        elif note.msgtype == 7:
            isicat += '###video###:{}<###TYPESPLIT###>{}<###splitter###>'.format(
                note.file, note.value)
        elif note.msgtype == 8:
            isicat += '###video_note###:{}<###TYPESPLIT###>{}<###splitter###>'.format(
                note.file, note.value)
        else:
            isicat += '{}<###splitter###>'.format(note.value)
    for x in range(count):
        notes['#{}'.format(
            namacat.split("<###splitter###>")[x])] = '{}'.format(
                isicat.split("<###splitter###>")[x])
    # Rules
    rules = rulessql.get_rules(chat_id)
    # Blacklist
    bl = list(blacklistsql.get_chat_blacklist(chat_id))
    # Disabled command
    disabledcmd = list(disabledsql.get_all_disabled(chat_id))
    # Locks
    locks = locksql.get_locks(chat_id)
    locked = []
    if locks:
        if locks.sticker:
            locked.append('sticker')
        if locks.document:
            locked.append('document')
        if locks.contact:
            locked.append('contact')
        if locks.audio:
            locked.append('audio')
        if locks.game:
            locked.append('game')
        if locks.bots:
            locked.append('bots')
        if locks.gif:
            locked.append('gif')
        if locks.photo:
            locked.append('photo')
        if locks.video:
            locked.append('video')
        if locks.voice:
            locked.append('voice')
        if locks.location:
            locked.append('location')
        if locks.forward:
            locked.append('forward')
        if locks.url:
            locked.append('url')
        restr = locksql.get_restr(chat_id)
        if restr.other:
            locked.append('other')
        if restr.messages:
            locked.append('messages')
        if restr.preview:
            locked.append('preview')
        if restr.media:
            locked.append('media')
    # TODO: Warnings
    # warns = warnssql.get_warns(chat_id)
    # Backing up
    backup[chat_id] = {
        'bot': bot.id,
        'hashes': {
            'info': {
                'rules': rules
            },
            'extra': notes,
            'blacklist': bl,
            'disabled': disabledcmd,
            'locks': locked
        }
    }
    baccinfo = json.dumps(backup, indent=4)
    f = open("smudgeb{}.backup".format(chat_id), "w")
    f.write(str(baccinfo))
    f.close()
    bot.sendChatAction(current_chat_id, "upload_document")
    tgl = time.strftime("%H:%M:%S - %d/%m/%Y", time.localtime(time.time()))
    try:
        bot.sendMessage(MESSAGE_DUMP,
                        tld(chat.id,
                            "backups_success").format(chat.title, chat_id,
                                                      tgl),
                        parse_mode=ParseMode.MARKDOWN)
    except BadRequest:
        pass
    bot.sendDocument(current_chat_id,
                     document=open('smudgeb{}.backup'.format(chat_id), 'rb'),
                     caption=tld(chat.id, "backups_success").format(
                         chat.title, chat_id, tgl),
                     timeout=360,
                     reply_to_message_id=msg.message_id,
                     parse_mode=ParseMode.MARKDOWN)
    os.remove("smudgeb{}.backup".format(chat_id))  # Cleaning file


# Temporary data
def put_chat(chat_id, value, chat_data):
    # print(chat_data)
    if value == False:
        status = False
    else:
        status = True
    chat_data[chat_id] = {'backups': {"status": status, "value": value}}


def get_chat(chat_id, chat_data):
    # print(chat_data)
    try:
        value = chat_data[chat_id]['backups']
        return value
    except KeyError:
        return {"status": False, "value": False}


__help__ = True

IMPORT_HANDLER = CommandHandler("import", import_data)
EXPORT_HANDLER = CommandHandler("export", export_data, pass_chat_data=True)

dispatcher.add_handler(IMPORT_HANDLER)
dispatcher.add_handler(EXPORT_HANDLER)
