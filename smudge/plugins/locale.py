from contextlib import suppress

from babel import Locale
from pyrogram import filters
from pyrogram.enums import ChatMemberStatus, ChatType
from pyrogram.errors import MessageNotModified, UserNotParticipant
from pyrogram.helpers import ikb
from pyrogram.types import CallbackQuery, Message

from ..bot import Smudge
from ..utils.locale import locale, set_db_lang
from . import Languages


@Smudge.on_message(filters.command(["setlang", "language"]))
@Smudge.on_callback_query(filters.regex(r"^language"))
@locale()
async def language(client: Smudge, union: Message | CallbackQuery, _):
    if isinstance(union, CallbackQuery):
        reply = union.edit_message_text
        union = union.message
    else:
        reply = union.reply_text

    keyboard = [
        [
            (Locale.parse(lang).display_name.title(), f"lang_set {lang}")
            for lang in list(Languages)
        ],
        [
            (
                _("🌎 Help us with translations!"),
                "https://crowdin.com/project/smudgelord",
                "url",
            ),
        ],
    ]

    if isinstance(union, CallbackQuery):
        keyboard += [[(_("↩️ Back"), "start_command")]]

    await reply(
        _("Select below the language you want to use the bot."),
        reply_markup=ikb(keyboard),
    )


@Smudge.on_callback_query(filters.regex("^lang_set (?P<code>.+)"))
@locale()
async def change_language(client: Smudge, callback: CallbackQuery, _):
    lang = callback.matches[0]["code"]
    if callback.message.chat.type is not ChatType.PRIVATE:
        try:
            member = await client.get_chat_member(
                chat_id=callback.message.chat.id, user_id=callback.from_user.id
            )
            if member.status not in (
                ChatMemberStatus.ADMINISTRATOR,
                ChatMemberStatus.OWNER,
            ):
                return
        except UserNotParticipant:
            return

    await set_db_lang(callback, lang)
    await change_language_edit(client, callback)


@locale()
async def change_language_edit(client, callback, _):
    keyboard = [[(_("↩️ Back"), "start")]]
    with suppress(MessageNotModified):
        text = _("Language changed successfully.")
        await callback.edit_message_text(text, reply_markup=ikb(keyboard))
