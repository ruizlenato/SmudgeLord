# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import asyncio
import glob
from importlib import import_module

from pyrogram import filters
from pyrogram.enums import ChatType
from pyrogram.helpers import array_chunk, ikb
from pyrogram.types import CallbackQuery, Message

from smudge import __commit__, __version__
from smudge.bot import Smudge
from smudge.utils.locale import get_string, locale

HELPABLE: list[str] = []

for modules in glob.glob("smudge/plugins/*.py"):
    imported_module = import_module((modules)[:-3].replace("/", "."))
    if hasattr(imported_module, "__help__"):
        HELPABLE.append((modules.replace("/", ".")).split(".")[-2])


@Smudge.on_message(filters.command("start"))
@Smudge.on_callback_query(filters.regex(r"start"))
@locale("start")
async def start_command(client: Smudge, union: Message | CallbackQuery, strings):
    if isinstance(union, CallbackQuery):
        chat_type = union.message.chat.type
        reply_text = union.edit_message_text
    else:
        chat_type = union.chat.type
        reply_text = union.reply_text

    if chat_type == ChatType.PRIVATE:
        keyboard = [
            [
                (strings["language-button"], "language"),
                (strings["help-button"], "help-menu"),
            ],
            [
                (strings["about-button"], "about-menu"),
            ],
        ]
        text = strings["start-text"].format(union.from_user.first_name)
    else:
        keyboard = [
            [(strings["start-button"], f"https://t.me/{client.me.username}?start=start", "url")]
        ]
        text = strings["start-text-privae"]
    await reply_text(text, reply_markup=ikb(keyboard), disable_web_page_preview=True)


@Smudge.on_callback_query(filters.regex(r"^help-menu"))
@locale("start")
async def help_menu(client: Smudge, union: Message | CallbackQuery, strings):
    reply_text = union.edit_message_text if isinstance(union, CallbackQuery) else union.reply_text
    buttons: list = []
    for help in sorted(HELPABLE):
        buttons.append((await get_string(union, help, "name"), f"help-plugin {help}"))

    # This will limit the row list to having 3 buttons only
    keyboard = array_chunk(buttons, 3)
    #Add a back button
    keyboard += [[(strings["back-button"], "start")]]
    
    await reply_text(
        strings["help-menu-text"],
        reply_markup=ikb(keyboard),
    )


@Smudge.on_callback_query(filters.regex(pattern="^help-plugin (?P<module>.+)"))
@locale("start")
async def help_plugin(client: Smudge, callback: CallbackQuery, strings):
    match = callback.matches[0]["module"]
    keyboard = [[(strings["back-button"], "help-menu")]]
    text = strings["help-commands-text"] + await get_string(callback, match, "help")
    await callback.edit_message_text(text, reply_markup=ikb(keyboard))


@Smudge.on_callback_query(filters.regex(r"config"))
@Smudge.on_message(filters.command("config"))
@locale("config")
async def config(client: Smudge, union: Message | CallbackQuery, strings):
    reply = union.edit_message_text if isinstance(union, CallbackQuery) else union.reply_text

    if not await filters.admin(client, union):
        if isinstance(union, CallbackQuery):
            await union.answer(strings["no-admin"], show_alert=True, cache_time=60)
        else:
            message = await reply(strings["no-admin"])
            await asyncio.sleep(5.0)
            await message.delete()
        return

    keyboard = [
        [
            (strings["medias-button"], "media_config"),
        ],
        [
            (await get_string(union, "start", "language-button"), "language"),
        ],
    ]

    await reply(strings["config-text"], reply_markup=ikb(keyboard))


@Smudge.on_callback_query(filters.regex(r"about-menu"))
@locale("start")
async def about_menu(client: Smudge, union: Message | CallbackQuery, strings):
    keyboard = [[((strings["donate-button"]), "https://ko-fi.com/ruizlenato", "url")]]
    text = strings["about-text"].format(__version__, __commit__)
    await union.edit_message_text(text, reply_markup=ikb(keyboard), disable_web_page_preview=True)
