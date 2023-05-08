# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import gettext
import os
import shutil
import tempfile

from pyrogram import filters
from pyrogram.types import Message

from ..bot import Smudge
from ..utils.locale import locale


@Smudge.on_message(filters.command("getsticker"))
@locale()
async def getsticker(c: Smudge, m: Message):
    try:
        sticker = m.reply_to_message.sticker
    except AttributeError:
        return await m.reply_text(
            _(
                "Reply to a sticker using this command so I can send it to you as a <b>png or gif</b>.\n<i>It only works with video and static stickers</i>"
            )
        )

    if sticker.is_video:
        with tempfile.TemporaryDirectory() as tempdir:
            path = os.path.join(tempdir, "getsticker")
        sticker_file = await c.download_media(
            message=m.reply_to_message, file_name=f"{path}/{sticker.set_name}.gif"
        )
    elif sticker.is_animated:
        await m.reply_text(_("Animated stickers are not supported yet!"))
        return

    else:
        with tempfile.TemporaryDirectory() as tempdir:
            path = os.path.join(tempdir, "getsticker")
        sticker_file = await c.download_media(
            message=m.reply_to_message, file_name=f"{path}/{sticker.set_name}.png"
        )

    await m.reply_to_message.reply_document(
        document=sticker_file,
        caption=(_("<b>Emoji:</b> {}\n<b>Sticker ID:</b> <code>{}</code>")).format(
            sticker.emoji, sticker.file_id
        ),
    )
    shutil.rmtree(tempdir, ignore_errors=True)


__help_name__ = gettext.gettext("Stickers")
__help_text__ = gettext.gettext(
    "<b>/getsticker -</b> reply to a sticker to me to upload the file as a <b>png or gif</b> (It only works with video and static stickers).\n"
)
