# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)


from smudge.locales.strings import tld, lang_dict
from smudge.database import set_db_lang

from typing import Union

from pyrogram.types import (
    CallbackQuery,
    InlineKeyboardButton,
    InlineKeyboardMarkup,
    Message,
)
from pyrogram import Client, filters
from pyrogram.helpers import ikb


async def gen_langs_kb(m):
    langs = sorted(list(lang_dict.keys()))
    return [
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

    if chat_type == "private":
        keyboard = InlineKeyboardMarkup(
            inline_keyboard=[
                [
                    InlineKeyboardButton(
                        text="🇧🇷 PT-BR (Português)",
                        callback_data=f"set_lang pt-BR",
                    )
                ],
                [
                    InlineKeyboardButton(
                        text="🇺🇸 EN-US (American English)",
                        callback_data=f"set_lang en-US",
                    )
                ],
                [
                    InlineKeyboardButton(
                        text=(await tld(m, "main_btn_back")),
                        callback_data="start_command",
                    )
                ],
            ]
        )
    else:
        member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
        if member.status in ["administrator", "creator"]:
            pass
        else:
            return
    text = await tld(m, "main_select_lang")
    await reply_text(text, reply_markup=ikb(await gen_langs_kb(m)))
    return


# @Client.on_callback_query(filters.regex(r"setchatlang"))
# async def setchatlang(c: Client, m: Message):
#    keyboard = InlineKeyboardMarkup(
#        inline_keyboard=[
#            [InlineKeyboardButton(text="Portuguese", callback_data="pt-BR")],
#            [InlineKeyboardButton(text="English", callback_data="en-US")],
#        ]
#    )
#    text = await tld(m, "main_select_lang")
#    await m.edit_message_text(text, reply_markup=keyboard)
#    return
