# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)

import importlib
import re
import asyncio


from pyrogram import Client, filters
from pyrogram.types import (
    Message,
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    CallbackQuery,
)
from pyrogram.helpers import ikb

from smudge.locales.strings import tld, lang_dict
from smudge.utils.help_menu import help_buttons
from smudge.database import set_db_lang
from smudge.database.core import groups
from smudge.plugins import all_plugins

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


@Client.on_callback_query(filters.regex("^set_lang (?P<code>.+)"))
async def portuguese(c: Client, m: Message):
    lang = m.matches[0]["code"]
    if m.message.chat.type == "private":
        pass
    else:
        member = await c.get_chat_member(
            chat_id=m.message.chat.id, user_id=m.from_user.id
        )
        if member.status in ["administrator", "creator"]:
            pass
        else:
            return
    keyboard = InlineKeyboardMarkup(
        inline_keyboard=[
            [
                InlineKeyboardButton(
                    text=(await tld(m, "main_btn_back")),
                    callback_data="setchatlang",
                )
            ],
        ]
    )
    if m.message.chat.type == "private":
        await set_db_lang(m.from_user.id, lang)
    elif m.message.chat.type == "supergroup" or "group":
        await set_db_lang(m.message.chat.id, lang)
    text = await tld(m, "lang_save")
    await m.edit_message_text(text, reply_markup=keyboard)


@Client.on_message(filters.command(["setlang"]))
@Client.on_callback_query(filters.regex(r"setchatlang"))
async def setlang(c: Client, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        chat_id = m.message.chat.id
        chat_type = m.message.chat.type
        reply_text = m.edit_message_text
    else:
        chat_id = m.chat.id
        chat_type = m.chat.type
        reply_text = m.reply_text
    langs = sorted(list(lang_dict.keys()))
    keyboard = [
        [
            (
                f'{lang_dict[lang]["main"]["language_flag"]} {lang_dict[lang]["main"]["language_name"]} ({lang_dict[lang]["main"]["language_code"]})',
                f"set_lang {lang}",
            )
            for lang in langs
        ],
        [
            (
                await tld(m, "lang_crowdin"),
                "https://crowdin.com/project/smudgelord",
                "url",
            ),
        ],
    ]
    if chat_type == "private":
        keyboard += [[(await tld(m, "main_btn_back"), f"start_command")]]
    else:
        try:
            member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
            if member.status in ["administrator", "creator"]:
                pass
            else:
                return
        except AttributeError:
            message = await reply_text(await tld(m, "change_lang_uchannel"))
            await asyncio.sleep(10.0)
            await message.delete()
            return
    await reply_text(await tld(m, "main_select_lang"), reply_markup=ikb(keyboard))
    return


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


@Client.on_callback_query(filters.regex(r"setsdl"))
async def setsdl(c: Client, m: Union[Message, CallbackQuery]):
    chat_id = m.message.chat.id
    chat_type = m.message.chat.type
    reply_text = m.edit_message_text

    try:
        member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
        if member.status in ["administrator", "creator"]:
            pass
        else:
            return
    except AttributeError:
        message = await reply_text(await tld(m, "change_lang_uchannel"))
        await asyncio.sleep(10.0)
        await message.delete()
        return

    if (await groups.get(id=chat_id)).sdl_autodownload == "Off":
        await groups.filter(id=chat_id).update(sdl_autodownload="On")
        text = await tld(m, "sdl_config_auto")
    else:
        await groups.filter(id=chat_id).update(sdl_autodownload="Off")
        text = await tld(m, "sdl_config_noauto")

    await reply_text(text)
    return


@Client.on_message(filters.command("config", prefixes="/") & filters.group)
async def config(c: Client, m: Union[Message, CallbackQuery]):
    chat_type = m.chat.type
    reply_text = m.reply_text
    chat_id = m.chat.id

    try:
        member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
        if member.status in ["administrator", "creator"]:
            pass
        else:
            return
    except AttributeError:
        message = await reply_text(await tld(m, "change_lang_uchannel"))
        await asyncio.sleep(10.0)
        await message.delete()
        return

    if (await groups.get(id=chat_id)).sdl_autodownload == "Off":
        emoji = "❌"
    else:
        emoji = "✅"

    me = await c.get_me()
    keyboard = [
        [
            (
                "SDL Auto: {}".format(emoji),
                "setsdl",
            ),
            (
                await tld(m, "main_start_btn_lang"),
                "setchatlang",
            ),
        ]
    ]

    text = await tld(m, "config_text")
    await reply_text(text, reply_markup=ikb(keyboard))
