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

from smudge.modules.sql.translation import switch_to_locale, prev_locale
from smudge.modules.translations.strings import tld
from telegram.ext import CommandHandler, CallbackContext
from telegram import Update, ParseMode, InlineKeyboardMarkup, InlineKeyboardButton
from smudge import dispatcher
from smudge.modules.translations.list_locale import list_locales
from smudge.helper_funcs.chat_status import user_admin
from telegram.ext import CallbackQueryHandler
import re

from smudge.modules.connection import connected


@user_admin
def locale(update: Update, context: CallbackContext):
    args = context.args
    chat = update.effective_chat
    if len(args) > 0:
        locale = args[0].lower()
        if locale in list_locales:
            if locale in ('en', 'pt'):
                switch_to_locale(chat.id, locale)
                update.message.reply_text(
                    tld(chat.id, 'language_switch_success').format(
                        list_locales[locale]))
            else:
                update.message.reply_text(
                    tld(chat.id,
                        "language_not_supported").format(list_locales[locale]),
                        parse_mode=ParseMode.HTML)
        else:
            update.message.reply_text(tld(chat.id, "language_code_not_valid"))
    else:
        LANGUAGE = prev_locale(chat.id)
        if LANGUAGE:
            locale = LANGUAGE.locale_name
            native_lang = list_locales[locale]
            update.message.reply_text(tld(
                chat.id, "language_current_locale").format(native_lang),
                parse_mode=ParseMode.MARKDOWN)
        else:
            update.message.reply_text(tld(
                chat.id, "language_current_locale").format("English"),
                parse_mode=ParseMode.MARKDOWN)


@user_admin
def locale_button(update: Update, context: CallbackContext):
    bot = context.bot
    chat = update.effective_chat
    user = update.effective_user
    query = update.callback_query
    lang_match = re.findall(r"en|pt", query.data)
    if lang_match:
        if lang_match[0]:
            switch_to_locale(chat.id, lang_match[0])
            query.answer(text=tld(chat.id, 'language_switch_success').format(
                list_locales[lang_match[0]]))
        else:
            query.answer(text="Error!", show_alert=True)

    try:
        LANGUAGE = prev_locale(chat.id)
        locale = LANGUAGE.locale_name
        curr_lang = list_locales[locale]
    except:
        curr_lang = "English"

    text = tld(chat.id, "language_select_language")
    text += tld(chat.id, "language_user_language").format(curr_lang)

    conn = connected(update, context, chat, user.id, need_admin=False)

    if conn:
        try:
            chatlng = prev_locale(conn).locale_name
            chatlng = list_locales[chatlng]
            text += tld(chat.id, "language_chat_language").format(chatlng)
        except:
            chatlng = "English"

    text += tld(chat.id, "language_sel_user_lang")

    query.message.reply_text(
        text,
        parse_mode=ParseMode.MARKDOWN,
        reply_markup=InlineKeyboardMarkup([[
            InlineKeyboardButton("English 🇺🇸", callback_data="set_lang_en")
        ]] + [[
            InlineKeyboardButton("Portuguese Brazil 🇧🇷",
                                 callback_data="set_lang_pt")
        ]] + [[
            InlineKeyboardButton(f"⬅️ {tld(chat.id, 'btn_go_back')}",
                                 callback_data="bot_start")
        ]]))

    print(lang_match)
    query.message.delete()
    bot.answer_callback_query(query.id)


LOCALE_HANDLER = CommandHandler(["set_locale", "locale", "lang", "setlang"],
                                locale,
                                pass_args=True)
locale_handler = CallbackQueryHandler(locale_button, pattern="chng_lang")
set_locale_handler = CallbackQueryHandler(locale_button, pattern=r"set_lang_")

dispatcher.add_handler(LOCALE_HANDLER)
dispatcher.add_handler(locale_handler)
dispatcher.add_handler(set_locale_handler)
