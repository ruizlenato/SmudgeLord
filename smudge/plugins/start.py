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
        chat_id = m.message.chat.id
        chat_type = m.message.chat.type
        reply_text = m.edit_message_text
    else:
        chat_id = m.chat.id
        chat_type = m.chat.type
        reply_text = m.reply_text

    me = await c.get_me()
    if chat_type == "private":
        keyboard = InlineKeyboardMarkup(
            inline_keyboard=[
                [
                    InlineKeyboardButton(
                        text=(await tld(chat_id, "main_start_btn_lang")),
                        callback_data="setchatlang",
                    ),
                    InlineKeyboardButton(
                        text=(await tld(chat_id, "main_start_btn_help")),
                        callback_data="menu"
                    )
                ],
            ]
        )
        text = (await tld(chat_id, "start_message_private")).format(
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
        text = await tld(m.chat.id, "start_message")
        await reply_text(text, reply_markup=keyboard, disable_web_page_preview=True)

@Client.on_callback_query(filters.regex("menu"))
async def button(c: Client, m: Union[Message, CallbackQuery]):
    keyboard = InlineKeyboardMarkup(await help_buttons(m, HELP))
    text = await tld(m.message.chat.id, "main_help_text")
    await m.edit_message_text(text, reply_markup=keyboard)

async def help_menu(c, m, text):
    keyboard = [[InlineKeyboardButton("Back", callback_data="menu")]]
    text = "<b> Avaliable Commands:</b>\n" + text
    await m.edit_message_text(text, reply_markup=InlineKeyboardMarkup(keyboard))

@Client.on_callback_query(filters.regex(pattern=".*help_plugin.*"))
async def but(c: Client, m: Union[Message, CallbackQuery]):
    plug_match = re.match(r"help_plugin\((.+?)\)", m.data)
    plug = plug_match.group(1)
    text = (await tld(m.message.chat.id, str(HELP[plug][0]["help"])))
    await help_(c, m, text)