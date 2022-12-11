# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
import re
import asyncio
import importlib
import contextlib
from typing import Union

from pyrogram import filters
from pyrogram.helpers import ikb
from pyrogram.types import Message, CallbackQuery
from pyrogram.enums import ChatType, ChatMemberStatus
from pyrogram.errors import FloodWait, MessageNotModified, UserNotParticipant

from ..bot import Smudge
from ..plugins import all_plugins
from ..locales import tld, loaded_locales
from ..database.locales import set_db_lang
from ..utils.help_menu import help_buttons
from ..database.videos import sdl_c, sdl_t

# Help plugins
HELP = {}

for plugins in all_plugins:
    plugin = importlib.import_module(f"smudge.plugins.{plugins}")
    if hasattr(plugin, "__help__"):
        HELP[plugins] = [{"name": plugin.__help__}]


@Smudge.on_message(filters.command("start"))
@Smudge.on_callback_query(filters.regex(r"start"))
async def start_command(c: Smudge, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        chat_type = m.message.chat.type
        reply_text = m.edit_message_text
    else:
        chat_type = m.chat.type
        reply_text = m.reply_text

    if chat_type == ChatType.PRIVATE:
        keyboard = [
            [
                (await tld(m, "Main.btn_lang"), "setchatlang"),
                (await tld(m, "Main.btn_help"), "menu"),
            ],
            [
                (
                    "Smudge News 📬",
                    "https://t.me/SmudgeNews",
                    "url",
                ),
            ],
        ]

        text = (await tld(m, "Main.start_message_private")).format(
            m.from_user.first_name
        )
    else:
        keyboard = [[("Start", f"https://t.me/{c.me.username}?start=start", "url")]]
        text = await tld(m, "Main.start_message")

    await reply_text(text, reply_markup=ikb(keyboard), disable_web_page_preview=True)


@Smudge.on_callback_query(filters.regex("^set_lang (?P<code>.+)"))
async def set_lang(c: Smudge, m: Message):
    lang = m.matches[0]["code"]
    if m.message.chat.type is not ChatType.PRIVATE:
        try:
            member = await c.get_chat_member(
                chat_id=m.message.chat.id, user_id=m.from_user.id
            )
            if member.status not in (
                ChatMemberStatus.ADMINISTRATOR,
                ChatMemberStatus.OWNER,
            ):
                return
        except UserNotParticipant:
            return

    keyboard = [[(await tld(m, "Main.back_btn"), "setchatlang")]]
    if m.message.chat.type == ChatType.PRIVATE:
        await set_db_lang(m.from_user.id, lang, m.message.chat.type)
    elif m.message.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        await set_db_lang(m.message.chat.id, lang, m.message.chat.type)
    text = await tld(m, "Main.lang_saved")
    with contextlib.suppress(MessageNotModified):
        await m.edit_message_text(text, reply_markup=ikb(keyboard))


@Smudge.on_message(filters.command(["setlang", "language"]))
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
    langs = sorted(list(loaded_locales.keys()))
    keyboard = [
        [
            (
                f'{loaded_locales.get(lang)["core"]["flag"]} {loaded_locales.get(lang)["core"]["name"]} ({loaded_locales.get(lang)["core"]["code"]})',
                f"set_lang {lang}",
            )
            for lang in langs
        ],
        [
            (
                await tld(m, "Main.crowdin"),
                "https://crowdin.com/project/smudgelord",
                "url",
            ),
        ],
    ]
    if chat_type == ChatType.PRIVATE:
        keyboard += [[(await tld(m, "Main.back_btn"), "start_command")]]
    else:
        try:
            member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
            if member.status not in (
                ChatMemberStatus.ADMINISTRATOR,
                ChatMemberStatus.OWNER,
            ):
                if isinstance(m, CallbackQuery):
                    await m.answer(
                        await tld(m, "Admin.not_admin"), show_alert=True, cache_time=60
                    )
                else:
                    message = await reply_text(await tld(m, "Admin.not_admin"))
                    await asyncio.sleep(5.0)
                    await message.delete()
                return
        except AttributeError:
            message = await reply_text(await tld(m, "Main.change_lang_uchannel"))
            await asyncio.sleep(5.0)
            return await message.delete()
        except UserNotParticipant:
            return
    await reply_text(await tld(m, "Main.select_lang"), reply_markup=ikb(keyboard))
    return


@Smudge.on_callback_query(filters.regex("menu"))
@Smudge.on_message(filters.command("help") & filters.private)
async def button(c: Smudge, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        reply_text = m.edit_message_text
        args = None
    else:
        reply_text = m.reply_text
        try:
            args = m.text.split(maxsplit=1)[1]
        except IndexError:
            args = None

    if args:
        with contextlib.suppress(KeyError):
            text = await tld(m, str(HELP[args][0]["help"]))
            return await help_menu(m, text)
    text = await tld(m, "Main.help_text")
    await reply_text(text, reply_markup=ikb(await help_buttons(m, HELP)))


async def help_menu(m, text):
    if isinstance(m, CallbackQuery):
        reply_text = m.edit_message_text
    else:
        reply_text = m.reply_text
    keyboard = [[(await tld(m, "Main.back_btn"), "menu")]]
    text = (await tld(m, "Main.avaliable_commands")).format(text)
    await reply_text(text, reply_markup=ikb(keyboard), disable_web_page_preview=True)


@Smudge.on_callback_query(filters.regex(pattern="help_plugin.*"))
async def but(c: Smudge, cq: CallbackQuery):
    plug_match = re.match(r"help_plugin\((.+?)\)", cq.data)
    plug = plug_match[1]
    text = await tld(cq, str(HELP[plug][0]["name"] + ".help"))
    await help_menu(cq, text)


@Smudge.on_message(filters.new_chat_members)
async def logging(c: Smudge, m: Message):
    try:
        if c.me.id in [x.id for x in m.new_chat_members]:
            await c.send_message(
                chat_id=m.chat.id,
                text=(
                    "(🇧🇷) Olá, obrigado por me adicionar em seu grupo!\n"
                    "<b>Não se esqueça de mudar as configurações de idioma usando o comando /config</b>.\n\n"
                    "(🇺🇸) Hi, thanks for adding me to your group!\n"
                    "<b>Don't forget to change the language settings using the /config command.</b>\n"
                ),
                disable_notification=True,
            )
    except FloodWait as e:
        await asyncio.sleep(e.value)


@Smudge.on_callback_query(filters.regex(r"^show_alert (?P<code>\w+)"))
async def show_alert(c: Smudge, m: Union[Message, CallbackQuery]):
    plugin = m.matches[0]["code"]
    await m.answer(
        await tld(m, f"Config.{plugin}_help"), show_alert=True, cache_time=60
    )


@Smudge.on_callback_query(filters.regex(r"config"))
@Smudge.on_message(filters.command("config") & filters.group)
async def config(c: Smudge, m: Union[Message, CallbackQuery]):
    if isinstance(m, CallbackQuery):
        chat_id = m.message.chat.id
        reply_text = m.edit_message_text
    else:
        chat_id = m.chat.id
        reply_text = m.reply_text

    try:
        member = await c.get_chat_member(chat_id=chat_id, user_id=m.from_user.id)
        if member.status not in (
            ChatMemberStatus.ADMINISTRATOR,
            ChatMemberStatus.OWNER,
        ):
            if isinstance(m, CallbackQuery):
                await m.answer(
                    await tld(m, "Admin.not_admin"), show_alert=True, cache_time=60
                )
            else:
                message = await reply_text(await tld(m, "Admin.not_admin"))
                await asyncio.sleep(5.0)
                await message.delete()
            return
    except UserNotParticipant:
        return
    keyboard = [
        [
            (await tld(m, "Config.videos"), "confsdl"),
        ],
        [
            (
                await tld(m, "Main.btn_lang"),
                "setchatlang",
            ),
        ],
    ]

    text = await tld(m, "Config.text")
    await reply_text(text, reply_markup=ikb(keyboard))


@Smudge.on_callback_query(filters.regex(r"confsdl"))
async def confsdl(c: Smudge, cq: CallbackQuery):
    try:
        member = await c.get_chat_member(
            chat_id=cq.message.chat.id, user_id=cq.from_user.id
        )
        if member.status not in (
            ChatMemberStatus.ADMINISTRATOR,
            ChatMemberStatus.OWNER,
        ):
            return await cq.answer(
                await tld(cq, "Admin.not_admin"),
                show_alert=True,
                cache_time=60,
            )
    except UserNotParticipant:
        return

    keyboard = [
        [
            (
                f"{await tld(cq, 'Config.btn_image')}:",
                "show_alert sdl_image",
            ),
            (
                f"{'✅' if await sdl_c('sdl_images', cq.message.chat.id) else '☑️'}",
                "setsdl sdl_images",
            ),
        ],
        [
            (
                f"{await tld(cq, 'Config.btn_auto')}:",
                "show_alert sdl_auto",
            ),
            (
                f"{'✅' if await sdl_c('sdl_auto', cq.message.chat.id) else '☑️'}",
                "setsdl sdl_auto",
            ),
        ],
        [(await tld(cq, "Main.back_btn"), "config")],
    ]

    return await cq.edit_message_text(
        await tld(cq, "Config.media_text"), reply_markup=ikb(keyboard)
    )


@Smudge.on_callback_query(filters.regex("^setsdl (?P<code>.+)"))
async def setsdl(c: Smudge, cq: CallbackQuery):
    method = cq.matches[0]["code"]
    if cq.message.chat.type is not ChatType.PRIVATE:
        member = await c.get_chat_member(
            chat_id=cq.message.chat.id, user_id=cq.from_user.id
        )
        if member.status not in (
            ChatMemberStatus.ADMINISTRATOR,
            ChatMemberStatus.OWNER,
        ):
            return await cq.answer(
                await tld(cq, "Admin.not_admin"), show_alert=True, cache_time=60
            )

    if not await sdl_c(method, cq.message.chat.id):
        await sdl_t(method, cq.message.chat.id, True)
        text = await tld(cq, f"Config.{method}")
    else:
        await sdl_t(method, cq.message.chat.id, None)
        text = await tld(cq, f"Config.no_{method}")

    keyboard = [[(await tld(cq, "Main.back_btn"), "confsdl")]]
    return await cq.edit_message_text(text, reply_markup=ikb(keyboard))
