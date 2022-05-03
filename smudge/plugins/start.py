# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import importlib
import re
import asyncio

from typing import Union

from pyrogram import filters
from pyrogram.helpers import ikb
from pyrogram.errors import FloodWait
from pyrogram.enums import ChatType, ChatMemberStatus
from pyrogram.types import (
    Message,
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    CallbackQuery,
)

from smudge import Smudge
from smudge.plugins import all_plugins
from smudge.plugins import tld, lang_dict
from smudge.utils.help_menu import help_buttons
from smudge.database.start import toggle_sdl, check_sdl
from smudge.database.locales import set_db_lang

HELP = {}

for plugin in all_plugins:
    imported_plugin = importlib.import_module("smudge.plugins." + plugin)
    if hasattr(imported_plugin, "plugin_help") and hasattr(
        imported_plugin, "plugin_name"
    ):
        plugin_name = imported_plugin.plugin_name
        plugin_help = imported_plugin.plugin_help
        HELP.update({plugin: [{"name": plugin_name, "help": plugin_help}]})


@Smudge.on_message(filters.command("start", prefixes="/"))
@Smudge.on_callback_query(filters.regex(r"start"))
async def start_command(c: Smudge, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        chat_type = m.message.chat.type
        reply_text = m.edit_message_text
    else:
        chat_type = m.chat.type
        reply_text = m.reply_text

    try:
        me = await c.get_me()
    except FloodWait as e:
        await asyncio.sleep(e.value)
    if chat_type == ChatType.PRIVATE:
        keyboard = InlineKeyboardMarkup(
            inline_keyboard=[
                [
                    InlineKeyboardButton(
                        text=(await tld(m, "Main.start_btn_lang")),
                        callback_data="setchatlang",
                    ),
                    InlineKeyboardButton(
                        text=(await tld(m, "Main.start_btn_help")),
                        callback_data="menu",
                    ),
                ],
            ]
        )
        text = (await tld(m, "Main.start_message_private")).format(
            m.from_user.first_name
        )
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
        text = await tld(m, "Main.start_message")
        await reply_text(text, reply_markup=keyboard, disable_web_page_preview=True)


@Smudge.on_callback_query(filters.regex("^set_lang (?P<code>.+)"))
async def portuguese(c: Smudge, m: Message):
    lang = m.matches[0]["code"]
    if m.message.chat.type == ChatType.PRIVATE:
        pass
    else:
        member = await c.get_chat_member(
            chat_id=m.message.chat.id, user_id=m.from_user.id
        )
        if member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER):
            pass
        else:
            return
    keyboard = InlineKeyboardMarkup(
        inline_keyboard=[
            [
                InlineKeyboardButton(
                    text=(await tld(m, "Main.btn_back")),
                    callback_data="setchatlang",
                )
            ],
        ]
    )
    if m.message.chat.type == ChatType.PRIVATE:
        await set_db_lang(m.from_user.id, lang, m.message.chat.type)
    elif m.message.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        await set_db_lang(m.message.chat.id, lang, m.message.chat.type)
    elif m.message.chat.type == ChatType.CHANNEL:
        await set_db_lang(m.from_user.id, lang, m.message.chat.type)
    text = await tld(m, "Main.lang_save")
    await m.edit_message_text(text, reply_markup=keyboard)


@Smudge.on_message(filters.command(["setlang"]))
@Smudge.on_callback_query(filters.regex(r"setchatlang"))
async def setlang(c: Smudge, m: Union[Message, CallbackQuery]):
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
                f'{lang_dict[lang]["core"]["language_flag"]} {lang_dict[lang]["core"]["language_name"]} ({lang_dict[lang]["core"]["language_code"]})',
                f"set_lang {lang}",
            )
            for lang in langs
        ],
        [
            (
                await tld(m, "Main.lang_crowdin"),
                "https://crowdin.com/project/smudgelord",
                "url",
            ),
        ],
    ]
    if chat_type == ChatType.PRIVATE:
        keyboard += [[(await tld(m, "Main.btn_back"), f"start_command")]]
    else:
        try:
            member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
            if member.status in (
                ChatMemberStatus.ADMINISTRATOR,
                ChatMemberStatus.OWNER,
            ):
                pass
            else:
                return
        except AttributeError:
            message = await reply_text(await tld(m, "Main.change_lang_uchannel"))
            await asyncio.sleep(10.0)
            await message.delete()
            return
    await reply_text(await tld(m, "Main.select_lang"), reply_markup=ikb(keyboard))
    return


@Smudge.on_callback_query(filters.regex("menu"))
async def button(c: Smudge, cq: CallbackQuery):
    keyboard = InlineKeyboardMarkup(await help_buttons(cq, HELP))
    text = await tld(cq, "Main.help_text")
    await cq.edit_message_text(text, reply_markup=keyboard)


async def help_menu(c, cq, text):
    keyboard = [
        [InlineKeyboardButton(await tld(cq, "Main.btn_back"), callback_data="menu")]
    ]
    text = (await tld(cq, "Main.avaliable_commands")).format(text)
    await cq.edit_message_text(text, reply_markup=InlineKeyboardMarkup(keyboard))


@Smudge.on_callback_query(filters.regex(pattern=".*help_plugin.*"))
async def but(c: Smudge, cq: CallbackQuery):
    plug_match = re.match(r"help_plugin\((.+?)\)", cq.data)
    plug = plug_match.group(1)
    text = await tld(cq, str(HELP[plug][0]["help"]))
    await help_menu(c, cq, text)


@Smudge.on_message(filters.new_chat_members)
async def logging(c: Smudge, m: Message):
    bot = await c.get_me()
    bot_id = bot.id
    if bot_id in [z.id for z in m.new_chat_members]:
        await c.send_message(
            chat_id=m.chat.id,
            text=(
                ":3 (🇧🇷 pt-BR) Olá, obrigado por me adicionar aqui!\n"
                "Não se esqueça de <b>mudar meu idioma usando /config</b>\n\n"
                ":3 (🇺🇸 en-US) Hi, thanks for adding me here!\n"
                "Don't forget to <b>change my language using /config</b>\n"
            ),
            disable_notification=True,
        )


@Smudge.on_callback_query(filters.regex(r"setsdl"))
async def setsdl(c: Smudge, m: Union[Message, CallbackQuery]):
    chat_id = m.message.chat.id
    reply_text = m.edit_message_text

    try:
        member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
        if member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER):
            pass
        else:
            return
    except AttributeError:
        message = await reply_text(await tld(m, "Main.change_lang_uchannel"))
        await asyncio.sleep(10.0)
        await message.delete()
        return

    if await check_sdl(chat_id) is None:
        await toggle_sdl(chat_id, True)
        text = await tld(m, "Misc.sdl_config_auto")
    else:
        await toggle_sdl(chat_id, None)
        text = await tld(m, "Misc.sdl_config_noauto")

    await reply_text(text)
    return


@Smudge.on_message(filters.command("config", prefixes="/") & filters.group)
async def config(c: Smudge, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        chat_id = m.message.chat.id
        reply_text = m.edit_message_text
    else:
        chat_id = m.chat.id
        reply_text = m.reply_text

    try:
        member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
        if member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER):
            pass
        else:
            return
    except AttributeError:
        message = await reply_text(await tld(m, "Main.change_lang_uchannel"))
        await asyncio.sleep(10.0)
        await message.delete()
        return

    if await check_sdl(m.chat.id) is None:
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
                await tld(m, "Main.start_btn_lang"),
                "setchatlang",
            ),
        ]
    ]

    text = await tld(m, "Main.config_text")
    await reply_text(text, reply_markup=ikb(keyboard))
