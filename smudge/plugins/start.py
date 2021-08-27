from pyrogram import Client, filters
from pyrogram.types import (
    Message,
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    CallbackQuery,
)
from smudge.locales.strings import tld
from typing import Union


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


@Client.on_callback_query(filters.regex(r"help"))
async def c_help(c: Client, m: Message):
    keyboard = InlineKeyboardMarkup(
        inline_keyboard=[
            [InlineKeyboardButton(text="Portuguese", callback_data="pt-BR")],
            [InlineKeyboardButton(text="English", callback_data="en-US")],
        ]
    )
    text = await tld(m.message.chat.id, "main_select_lang")
    await m.edit_message_text(text, reply_markup=keyboard)
    return
