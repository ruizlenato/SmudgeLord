# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import asyncio
import os
import sys
from io import BytesIO

import filetype
from PIL import Image, ImageOps
from hydrogram import filters
from hydrogram.errors import PeerIdInvalid, StickersetInvalid
from hydrogram.helpers import ikb
from hydrogram.raw.functions.messages import GetStickerSet, SendMedia
from hydrogram.raw.functions.stickers import AddStickerToSet, CreateStickerSet
from hydrogram.raw.types import (
    DocumentAttributeFilename,
    InputDocument,
    InputMediaUploadedDocument,
    InputStickerSetItem,
    InputStickerSetShortName,
)
from hydrogram.types import Message

from smudge.bot import Smudge
from smudge.config import config
from smudge.utils.locale import get_string, locale
from smudge.utils.utils import EMOJI_PATTERN


@Smudge.on_message(filters.command("getsticker"))
@locale("stickers")
async def getsticker(client: Smudge, message: Message, strings):
    sticker = message.reply_to_message.sticker
    if sticker:
        if sticker.is_animated:
            await message.reply_text(strings["animated-unsupported"])
        else:
            extension = ".png" if not sticker.is_video else ".webm"
            file = await message.reply_to_message.download(
                in_memory=True, file_name=f"{sticker.file_unique_id}.{extension}"
            )

        await message.reply_to_message.reply_document(
            document=file,
            caption=(strings["sticker-info"]).format(sticker.emoji, sticker.file_id),
        )
    else:
        await message.reply_text(strings["getsticker-no-args"])


@Smudge.on_message(filters.command("kang"))
@locale("stickers")
async def kang(client: Smudge, message: Message, strings):
    progress_mesage = await message.reply_text(strings["kanging"])
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
                return await progress_mesage.edit_text(strings["file-no-name"])
            if message.reply_to_message.sticker.emoji:
                emoji = message.reply_to_message.sticker.emoji
            animated = message.reply_to_message.sticker.is_animated
            videos = message.reply_to_message.sticker.is_video
            if (
                not message.reply_to_message.sticker.file_name.endswith(".tgs")
                and not videos
            ):
                resize = True
        else:
            return await progress_mesage.edit_text(strings["media-invalid"])

        pack_prefix = "anim" if animated else "vid" if videos else "a"
        packname = f"{pack_prefix}_{message.from_user.id}_by_{client.me.username}"

        if (
            len(message.command) > 1
            and message.command[1].isdigit()
            and int(message.command[1]) > 0
        ):
            # provide pack number to kang in desired pack
            packnum = message.command.pop(1)
            packname = (
                f"{pack_prefix}{packnum}_{message.from_user.id}_by_{client.me.username}"
            )
        if len(message.command) > 1:
            # matches all valid emojis in input
            emoji = (
                "".join(set(EMOJI_PATTERN.findall("".join(message.command[1:]))))
                or emoji
            )

        if convert:
            file = await client.download_media(message.reply_to_message)
        else:
            file = await client.download_media(message.reply_to_message, in_memory=True)
        if not file:
            await progress_mesage.delete()  # Failed to download
            return None
    else:
        return await progress_mesage.edit_text(strings["kang-no-reply"])

    try:
        if resize:
            file = resize_image(file)
        elif convert:
            file = await convert_video(file)
            file.name = f"sticker.{filetype.guess_extension(file)}"
            await progress_mesage.edit_text(strings["converting_video"])
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
                    packname = f"{pack_prefix}_{packnum}_{message.from_user.id}_by_\
{client.me.username}"
                else:
                    packname_found = True
            except StickersetInvalid:
                break
        ufile = await client.save_file(file)
        media = await client.invoke(
            SendMedia(
                peer=(await client.resolve_peer(int(config["LOG_CHAT"]))),
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
            await progress_mesage.edit_text(strings["existing_pack"])
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
            await progress_mesage.edit_text(strings["new-pack"])
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
                    strings["never-interacted"],
                    reply_markup=ikb(
                        [
                            [
                                (
                                    await get_string(message, "start", "start-button"),
                                    f"https://t.me/{client.me.username}?start",
                                    "url",
                                )
                            ]
                        ]
                    ),
                )
    except Exception as all_e:
        await progress_mesage.edit_text(f"{all_e.__class__.__name__} : {all_e}")
    else:
        await progress_mesage.edit_text(
            strings["sticker-stoled"].format(packname, emoji)
        )
        await client.delete_messages(
            chat_id=config["LOG_CHAT"], message_ids=msg_.id, revoke=True
        )


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


__help__ = True
