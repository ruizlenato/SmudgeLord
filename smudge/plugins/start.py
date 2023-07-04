# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import asyncio
import glob
import importlib

from pyrogram import filters
from pyrogram.enums import ChatType
from pyrogram.helpers import array_chunk, ikb
from pyrogram.types import CallbackQuery, Message

from smudge.bot import Smudge
from smudge.utils.locale import locale

HELPABLE = {}

mod_paths = glob.glob("smudge/plugins/*.py")
for f in mod_paths:
    if not f.endswith("__init__.py"):
        imported_module = importlib.import_module((f)[:-3].replace("/", "."))
        if hasattr(imported_module, "__help_name__"):
            HELPABLE[imported_module.__help_name__] = {
                "__help_text__": imported_module.__help_text__
            }


@Smudge.on_message(filters.command("start"))
@Smudge.on_callback_query(filters.regex(r"start"))
@locale()
async def start_command(client: Smudge, union: Message | CallbackQuery, _):
    if isinstance(union, CallbackQuery):
        chat_type = union.message.chat.type
        reply_text = union.edit_message_text
    else:
        chat_type = union.chat.type
        reply_text = union.reply_text

    if chat_type == ChatType.PRIVATE:
        keyboard = [
            [
                (_("🌐Language"), "language"),
                (_("❓Help"), "help-menu"),
            ],
            [
                (
                    "Smudge News 📬",
                    "https://t.me/SmudgeNews",
                    "url",
                ),
            ],
        ]
        text = _(
            "Hello <b>{}</b>! — I'm <b>SmudgeLord,</b> a bot with some useful \
and fun commands for you.\n\n \
📦 <b>SourceCode:</b> <a href='https://github.com/ruizlenato/SmudgeLord'>GitHub</a>"
        ).format(union.from_user.first_name)
    else:
        keyboard = [[("Start", f"https://t.me/{client.me.username}?start=start", "url")]]
        text = _(
            "Hello!, I'm SmudgeLord. I have a lot of functions, \
to know more, start a conversation with me."
        )
    await reply_text(text, reply_markup=ikb(keyboard), disable_web_page_preview=True)


@Smudge.on_callback_query(filters.regex(r"^help-menu"))
@locale()
async def help_menu(client: Smudge, union: Message | CallbackQuery, _):
    reply_text = union.edit_message_text if isinstance(union, CallbackQuery) else union.reply_text
    buttons: list = []
    for help in HELPABLE:
        buttons.append((_(help), f"help-plugin {help}"))

    keyboard = array_chunk(buttons, 3)
    # This will limit the row list to having 3 buttons only.
    await reply_text(
        _(
            "Here are all my plugins, to find out more about the plugins, \
<b>just click on their name.</b>"
        ),
        reply_markup=ikb(keyboard),
    )


@Smudge.on_callback_query(filters.regex(pattern="^help-plugin (?P<module>.+)"))
@locale()
async def help_plugin(client: Smudge, callback: CallbackQuery, _):
    match = callback.matches[0]["module"]
    keyboard = [[(_("↩️ Back"), "help-menu")]]
    help_text = "__help_text__"  # To avoid problems with gettext
    text = _("<b>Avaliable Commands:</b>\n\n") + _(HELPABLE[match][help_text])
    await callback.edit_message_text(text, reply_markup=ikb(keyboard))


@Smudge.on_callback_query(filters.regex(r"config"))
@Smudge.on_message(filters.command("config"))
@locale()
async def config(client: Smudge, union: Message | CallbackQuery, _):
    reply = union.edit_message_text if isinstance(union, CallbackQuery) else union.reply_text

    if not await filters.admin(client, union):
        if isinstance(union, CallbackQuery):
            await union.answer(_("You are not a group admin."), show_alert=True, cache_time=60)
        else:
            message = await reply(_("You are not a group admin."))
            await asyncio.sleep(5.0)
            await message.delete()
        return

    keyboard = [
        [
            (_("Medias"), "media_config"),
        ],
        [
            (_("🌐Language"), "language"),
        ],
    ]

    await reply(
        _(
            "<b>Settings</b> — Here are my settings for this group, \
to know more, <b>click on the buttons below.</b>"
        ),
        reply_markup=ikb(keyboard),
    )
