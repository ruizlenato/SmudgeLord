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
from smudge.utils.locale import get_string, locale


@Smudge.on_message(filters.command(["setlang", "language"]))
@Smudge.on_callback_query(filters.regex(r"^language"))
@locale("locale")
async def language(client: Smudge, union: Message | CallbackQuery, strings):
    reply = (
        union.edit_message_text
        if isinstance(union, CallbackQuery)
        else union.reply_text
    )
    buttons: list = []
    for lang in list(Languages):
        text, data = (Locale.parse(lang).display_name.title(), f"lang_set {lang}")
        buttons.append((text, data))

    keyboard = array_chunk(buttons, 2)
    keyboard.append(
        [
            (
                strings["crowdin-button"],
                "https://crowdin.com/project/smudgelord",
                "url",
            )
        ]
    )

    if isinstance(union, CallbackQuery) and union.message.chat.type == ChatType.PRIVATE:
        keyboard += [
            [(await get_string(union, "start", "back-button"), "start_command")]
        ]

    await reply(strings["select-language"], reply_markup=ikb(keyboard))


@Smudge.on_callback_query(filters.regex("^lang_set (?P<code>.+)"))
@locale("locale")
async def change_language(client: Smudge, callback: CallbackQuery, _):
    lang = callback.matches[0]["code"]
    if not await filters.admin(client, callback):
        return await callback.answer(
            await get_string(callback, "config", "no-admin"),
            show_alert=True,
            cache_time=60,
        )

    await set_db_lang(callback, lang)
    await change_language_edit(client, callback)
    return None


@locale("locale")
async def change_language_edit(client, callback, strings):
    keyboard = [[(await get_string(callback, "start", "back-button"), "start_command")]]
    if (
        isinstance(callback, CallbackQuery)
        and callback.message.chat.type != ChatType.PRIVATE
    ):
        keyboard = [[(await get_string(callback, "start", "back-button"), "config")]]
    with suppress(MessageNotModified):
        await callback.edit_message_text(
            strings["language-changed"], reply_markup=ikb(keyboard)
        )
