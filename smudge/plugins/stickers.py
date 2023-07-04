# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import asyncio
import gettext
import os
import sys
from io import BytesIO

import filetype
from PIL import Image, ImageOps
from pyrogram import filters
from pyrogram.errors import PeerIdInvalid, StickersetInvalid
from pyrogram.helpers import ikb
from pyrogram.raw.functions.messages import GetStickerSet, SendMedia
from pyrogram.raw.functions.stickers import AddStickerToSet, CreateStickerSet
from pyrogram.raw.types import (
    DocumentAttributeFilename,
    InputDocument,
    InputMediaUploadedDocument,
    InputStickerSetItem,
    InputStickerSetShortName,
)
from pyrogram.types import Message

from smudge.bot import Smudge
from smudge.config import config
from smudge.utils.locale import locale
from smudge.utils.utils import EMOJI_PATTERN


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


@Smudge.on_message(filters.command("kang"))
@locale()
async def kang(client: Smudge, message: Message, _):
    progress_mesage = await message.reply_text(_("<code>Kanging (Stealing) the sticker...</code>"))
    emoji = "🤔"
    packnum = 0
    packname_found = False
    resize = False
    animated = False
    videos = False
    convert = False
    if message.reply_to_message and message.reply_to_message.media:
        if (
            not message.reply_to_message.photo
            and message.reply_to_message.document
            and "image" in message.reply_to_message.document.mime_type
            or message.reply_to_message.photo
        ):
            resize = True
        elif (
            message.reply_to_message.document
            and "tgsticker" in message.reply_to_message.document.mime_type
        ):
            animated = True
        elif (
            message.reply_to_message.document
            and "video" in message.reply_to_message.document.mime_type
            or message.reply_to_message.video
            or message.reply_to_message.animation
        ):
            convert = True
            videos = True
        elif message.reply_to_message.sticker:
            if not message.reply_to_message.sticker.file_name:
                return await progress_mesage.edit_text(_("This sticker doesn't a have filename!"))
            if message.reply_to_message.sticker.emoji:
                emoji = message.reply_to_message.sticker.emoji
            animated = message.reply_to_message.sticker.is_animated
            videos = message.reply_to_message.sticker.is_video
            if not message.reply_to_message.sticker.file_name.endswith(".tgs"):
                resize = True
        else:
            return await progress_mesage.edit_text(_("<b>Erorr</b>: Invalid media!"))

        pack_prefix = "anim" if animated else "vid" if videos else "a"
        packname = f"{pack_prefix}_{message.from_user.id}_by_{client.me.username}"

        if (
            len(message.command) > 1
            and message.command[1].isdigit()
            and int(message.command[1]) > 0
        ):
            # provide pack number to kang in desired pack
            packnum = message.command.pop(1)
            packname = f"{pack_prefix}{packnum}_{message.from_user.id}_by_{client.me.username}"
        if len(message.command) > 1:
            # matches all valid emojis in input
            emoji = "".join(set(EMOJI_PATTERN.findall("".join(message.command[1:])))) or emoji

        if convert:
            file = await client.download_media(message.reply_to_message)
        else:
            file = await client.download_media(message.reply_to_message, in_memory=True)
        if not file:
            await progress_mesage.delete()  # Failed to download
            return None
    else:
        return await progress_mesage.edit_text(
            _(
                "<b>You need to use this command replying a \
sticker or a photo.</b>"
            )
        )

    try:
        if resize:
            file = resize_image(file)
        elif convert:
            file = await convert_video(file)
            file.name = f"sticker.{filetype.guess_extension(file)}"
            await progress_mesage.edit_text(_("<code>Converting video/gif to sticker...</code>"))
            if file is False:
                return await progress_mesage.edit_text("<b>Error</b>")
        max_stickers = 50 if animated else 120
        while not packname_found:
            try:
                stickerset = await client.invoke(
                    GetStickerSet(
                        stickerset=InputStickerSetShortName(short_name=packname),
                        hash=0,
                    )
                )
                if stickerset.set.count >= max_stickers:
                    packnum += 1
                    packname = (
                        f"{pack_prefix}_{packnum}_{message.from_user.id}_by_{client.me.username}"
                    )
                else:
                    packname_found = True
            except StickersetInvalid:
                break
        ufile = await client.save_file(file)
        media = await client.invoke(
            SendMedia(
                peer=(await client.resolve_peer(config["LOG_CHAT"])),
                media=InputMediaUploadedDocument(
                    file=ufile,
                    mime_type=filetype.guess_mime(file),
                    attributes=[DocumentAttributeFilename(file_name=file.name)],
                ),
                message=f"#Sticker kang by UserID -> {message.from_user.id}",
                random_id=client.rnd_id(),
            ),
        )
        msg_ = media.updates[-1].message
        stkr_file = msg_.media.document
        if packname_found:
            await progress_mesage.edit_text(_("<code>Using existing sticker pack...</code>"))
            await client.invoke(
                AddStickerToSet(
                    stickerset=InputStickerSetShortName(short_name=packname),
                    sticker=InputStickerSetItem(
                        document=InputDocument(
                            id=stkr_file.id,
                            access_hash=stkr_file.access_hash,
                            file_reference=stkr_file.file_reference,
                        ),
                        emoji=emoji,
                    ),
                )
            )
        else:
            await progress_mesage.edit_text(_("<code>Creating a new sticker package...</code>"))
            try:
                stkr_title = f"@{message.from_user.username[:32]}'s SmudgeLord"
            except TypeError:
                stkr_title = f"@{message.from_user.username[:32]}'s SmudgeLord"
            if animated:
                stkr_title += " Anim"
            elif videos:
                stkr_title += " Vid"
            if packnum != 0:
                stkr_title += f" v{packnum}"
            try:
                await client.invoke(
                    CreateStickerSet(
                        user_id=await client.resolve_peer(
                            message.from_user.username or message.from_user.id
                        ),
                        title=stkr_title,
                        short_name=packname,
                        stickers=[
                            InputStickerSetItem(
                                document=InputDocument(
                                    id=stkr_file.id,
                                    access_hash=stkr_file.access_hash,
                                    file_reference=stkr_file.file_reference,
                                ),
                                emoji=emoji,
                            )
                        ],
                        animated=animated,
                        videos=videos,
                    )
                )
            except PeerIdInvalid:
                return await progress_mesage.edit_text(
                    _(
                        "Looks like you've never interacted with me on private chat, you need \
to do that first.\nClick the button below to send me a message."
                    ),
                    reply_markup=ikb(
                        [[(_("Start"), f"https://t.me/{client.me.username}?start", "url")]]
                    ),
                )
    except Exception as all_e:
        await progress_mesage.edit_text(f"{all_e.__class__.__name__} : {all_e}")
    else:
        kanged_success_msg = _(
            "Sticker stolen <b>successfully</b>, <a href='t.me/addstickers/{}'>check out.</a>\
\n<b>Emoji:</b> {}"
        )
        await progress_mesage.edit_text(kanged_success_msg.format(packname, emoji))
        await client.delete_messages(chat_id=config["LOG_CHAT"], message_ids=msg_.id, revoke=True)


def resize_image(file: str) -> BytesIO:
    im = Image.open(file)
    im = ImageOps.contain(im, (512, 512), method=Image.ANTIALIAS)
    image = BytesIO()
    image.name = "sticker.png"
    im.save(image, "PNG")
    return image


async def convert_video(file: str) -> str:
    process = await asyncio.create_subprocess_exec(
        *[
            "ffmpeg",
            "-loglevel",
            "quiet",
            "-i",
            file,
            "-t",
            "00:00:03",
            "-vf",
            "fps=30",
            "-c:v",
            "vp9",
            "-b:v:",
            "500k",
            "-preset",
            "ultrafast",
            "-s",
            "512x512",
            "-y",
            "-an",
            "-f",
            "webm",
            "pipe:%i" % sys.stdout.fileno(),
        ],
        stdin=asyncio.subprocess.PIPE,
        stdout=asyncio.subprocess.PIPE,
    )

    (stdout, stderr) = await process.communicate()
    os.remove(file)
    # it is necessary to delete the video because
    # ffmpeg used a saved file and not bytes,
    # unfortunately it is not possible to convert
    # the video or gif to webm in the telegram
    # sticker video requirements with ffmpeg input in bytes
    return BytesIO(stdout)


__help_name__ = gettext.gettext("Stickers")
__help_text__ = gettext.gettext(
    """<b>/getsticker —</b> reply to a sticker to me to upload the file as a
<b>png or gif</b> <i>(It only works with video and static stickers).</i>\n
<b>/kang —</b> reply to a sticker to add it to your pack created by me.
"""
)
