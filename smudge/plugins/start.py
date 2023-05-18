# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import glob
import importlib

from pyrogram import filters
from pyrogram.enums import ChatType
from pyrogram.helpers import ikb
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
            "Hello <b>{}</b>, my name is <b>SmudgeLord,</b> \
I'm a bot with some fun and useful commands for you :3\n\n📦 <b>My SourceCode:</b> \
<a href='https://github.com/ruizlenato/SmudgeLord'>GitHub</a>\n💬 If you have a <b>problem</b> \
<a href='https://t.me/RuizLenato'>click here to talk to my developer.</a>"
        ).format(union.from_user.first_name)
    else:
        text = _(
            "Hello!, I'm SmudgeLord. I have a lot of functions, \
to know more, start a conversation with me."
        )
    await reply_text(text, reply_markup=ikb(keyboard), disable_web_page_preview=True)


@Smudge.on_callback_query(filters.regex(r"^help-menu"))
@locale()
async def help_menu(client: Smudge, union: Message | CallbackQuery, _):
    reply_text = union.edit_message_text if isinstance(union, CallbackQuery) else union.reply_text
    keyboard = [
        [
            (
                _(help),
                f"help-plugin {help}",
            )
            for help in HELPABLE
        ],
    ]
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
