# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
from contextlib import suppress

from babel import Locale
from pyrogram import filters
from pyrogram.enums import ChatType
from pyrogram.errors import MessageNotModified
from pyrogram.helpers import array_chunk, ikb
from pyrogram.types import CallbackQuery, Message

from smudge.bot import Smudge
from smudge.database.locale import set_db_lang
from smudge.plugins import Languages
from smudge.utils.locale import locale


@Smudge.on_message(filters.command(["setlang", "language"]))
@Smudge.on_callback_query(filters.regex(r"^language"))
@locale()
async def language(client: Smudge, union: Message | CallbackQuery, _):
    reply = union.edit_message_text if isinstance(union, CallbackQuery) else union.reply_text
    buttons: list = []
    for lang in list(Languages):
        text, data = (Locale.parse(lang).display_name.title(), f"lang_set {lang}")
        buttons.append((text, data))

    keyboard = array_chunk(buttons, 2)
    keyboard.append(
        [
            (
                _("🌎 Help us with translations!"),
                "https://crowdin.com/project/smudgelord",
                "url",
            )
        ]
    )

    if isinstance(union, CallbackQuery) and union.message.chat.type == ChatType.PRIVATE:
        keyboard += [[(_("↩️ Back"), "start_command")]]

    await reply(
        _("Select below the language you want to use the bot."),
        reply_markup=ikb(keyboard),
    )


@Smudge.on_callback_query(filters.regex("^lang_set (?P<code>.+)"))
@locale()
async def change_language(client: Smudge, callback: CallbackQuery, _):
    lang = callback.matches[0]["code"]
    if not await filters.admin(client, callback):
        return await callback.answer(
            _("You are not a group admin."), show_alert=True, cache_time=60
        )

    await set_db_lang(callback, lang)
    await change_language_edit(client, callback)
    return None


@locale()
async def change_language_edit(client, callback, _):
    text = _("Language changed successfully.")
    keyboard = [[(_("↩️ Back"), "start_command")]]
    if isinstance(callback, CallbackQuery) and callback.message.chat.type != ChatType.PRIVATE:
        keyboard = [[(_("↩️ Back"), "config")]]
    with suppress(MessageNotModified):
        await callback.edit_message_text(text, reply_markup=ikb(keyboard))
