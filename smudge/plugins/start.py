# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import importlib
import re

from pyrogram import Client, filters
from pyrogram.types import (
    Message,
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    CallbackQuery,
)
from smudge.locales.strings import tld
from smudge.plugins import all_plugins
from smudge.utils.help_menu import help_buttons

from typing import Union

HELP = {}

for plugin in all_plugins:
    imported_plugin = importlib.import_module("smudge.plugins." + plugin)
    if hasattr(imported_plugin, "plugin_help") and hasattr(
        imported_plugin, "plugin_name"
    ):
        plugin_name = imported_plugin.plugin_name
        plugin_help = imported_plugin.plugin_help
        HELP.update({plugin: [{"name": plugin_name, "help": plugin_help}]})


@Client.on_message(filters.command("start", prefixes="/"))
@Client.on_callback_query(filters.regex(r"start"))
async def start_command(c: Client, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        chat_type = m.message.chat.type
        reply_text = m.edit_message_text
    else:
        chat_type = m.chat.type
        reply_text = m.reply_text

    me = await c.get_me()
    if chat_type == "private":
        keyboard = InlineKeyboardMarkup(
            inline_keyboard=[
                [
                    InlineKeyboardButton(
                        text=(await tld(m, "main_start_btn_lang")),
                        callback_data="setchatlang",
                    ),
                    InlineKeyboardButton(
                        text=(await tld(m, "main_start_btn_help")),
                        callback_data="menu",
                    ),
                ],
            ]
        )
        text = (await tld(m, "start_message_private")).format(m.from_user.first_name)
        await reply_text(text, reply_markup=keyboard, disable_web_page_preview=True)
    else:
        keyboard = InlineKeyboardMarkup(
            inline_keyboard=[
                [
                    InlineKeyboardButton(
                        text="Start", url=f"https://t.me/{me.username}?start=start"
                    )
                ]
            ]
        )
        text = await tld(m, "start_message")
        await reply_text(text, reply_markup=keyboard, disable_web_page_preview=True)


@Client.on_callback_query(filters.regex("menu"))
async def button(c: Client, cq: CallbackQuery):
    keyboard = InlineKeyboardMarkup(await help_buttons(cq, HELP))
    text = await tld(cq, "main_help_text")
    await cq.edit_message_text(text, reply_markup=keyboard)


async def help_menu(c, cq, text):
    keyboard = [
        [InlineKeyboardButton(await tld(cq, "main_btn_back"), callback_data="menu")]
    ]
    text = (await tld(cq, "avaliable_commands")).format(text)
    await cq.edit_message_text(text, reply_markup=InlineKeyboardMarkup(keyboard))


@Client.on_callback_query(filters.regex(pattern=".*help_plugin.*"))
async def but(c: Client, cq: CallbackQuery):
    plug_match = re.match(r"help_plugin\((.+?)\)", cq.data)
    plug = plug_match.group(1)
    text = await tld(cq, str(HELP[plug][0]["help"]))
    await help_menu(c, cq, text)


@Client.on_message(filters.new_chat_members)
async def logging(c: Client, m: Message):
    bot = await c.get_me()
    bot_id = bot.id
    if bot_id in [z.id for z in m.new_chat_members]:
        await c.send_message(
            chat_id=m.chat.id,
            text=(
                "/ᐠ. ｡.ᐟ\ᵐᵉᵒʷ  Olá, obrigado por me adicionar aqui!\n"
                "Não se esqueça de <b>mudar meu idioma usando /setlang</b>\n\n"
                "/ᐠ. ｡.ᐟ\ᵐᵉᵒʷ  Hi, thanks for adding me here!\n"
                "Don't forget to <b>change my language using /setlang</b>\n"
            ),
            disable_notification=True,
        )
