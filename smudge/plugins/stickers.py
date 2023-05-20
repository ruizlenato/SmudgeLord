# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import gettext

from pyrogram import filters
from pyrogram.types import Message

from smudge.bot import Smudge
from smudge.utils.locale import locale


@Smudge.on_message(filters.command("getsticker"))
@locale()
async def getsticker(client: Smudge, message: Message, _):
    sticker = message.reply_to_message.sticker
    if sticker:
        if sticker.is_animated:
            await message.reply_text(_("Animated stickers are not supported yet!"))
        else:
            extension = ".png" if not sticker.is_video else ".webm"
            file = await message.reply_to_message.download(
                in_memory=True, file_name=f"{sticker.file_unique_id}.{extension}"
            )

        await message.reply_to_message.reply_document(
            document=file,
            caption=(_("<b>Emoji:</b> {}\n<b>Sticker ID:</b> <code>{}</code>")).format(
                sticker.emoji, sticker.file_id
            ),
        )
    else:
        await message.reply_text(
            _(
                "Reply to a sticker using this command so I can send it to you as a \
<b>png or gif</b>.\n<i>It only works with video and static stickers</i>"
            )
        )


__help_name__ = gettext.gettext("Stickers")
__help_text__ = gettext.gettext(
    """
<b>/getsticker —</b> reply to a sticker to me to upload the file as a
<b>png or gif</b> <i>(It only works with video and static stickers).</i>\n
"""
)
