import os
import shutil
import tempfile

from PIL import Image

from smudge.config import CHAT_LOGS
from smudge.utils import EMOJI_PATTERN
from smudge.locales.strings import tld

from pyrogram import Client, filters
from pyrogram.errors import PeerIdInvalid, StickersetInvalid
from pyrogram.raw.functions.messages import GetStickerSet, SendMedia
from pyrogram.raw.functions.stickers import AddStickerToSet, CreateStickerSet
from pyrogram.types import Message, InlineKeyboardButton, InlineKeyboardMarkup
from pyrogram.raw.types import (
    DocumentAttributeFilename,
    InputDocument,
    InputMediaUploadedDocument,
    InputStickerSetItem,
    InputStickerSetShortName,
)

SUPPORTED_TYPES = ["jpeg", "png", "webp"]


@Client.on_message(filters.command("getsticker"))
async def getsticker(c: Client, m: Message):
    sticker = m.reply_to_message.sticker
    if sticker:
        if sticker.is_animated:
            await m.reply_text(await tld(m.chat.id, "animated_not_supported"))
        elif not sticker.is_animated:
            with tempfile.TemporaryDirectory() as tempdir:
                path = os.path.join(tempdir, "getsticker")
            sticker_file = await c.download_media(
                message=m.reply_to_message,
                file_name=f"{path}/{sticker.set_name}.png",
            )
            await m.reply_to_message.reply_document(
                document=sticker_file,
                caption=(await tld(m.chat.id, "sticker_info")).format(
                    sticker.emoji, sticker.file_id
                ),
            ),
            shutil.rmtree(tempdir, ignore_errors=True)
    else:
        await m.reply_text(await tld(m.chat.id, "stickers_getsticker_no_reply"))
        return


@Client.on_message(filters.command("kang"))
async def kang_sticker(c: Client, m: Message):
    prog_msg = await m.reply_text(await tld(m.chat.id, "stickers_kanging"))
    user = await c.get_me()
    bot_username = user.username
    sticker_emoji = "🤔"
    packnum = 0
    packname_found = False
    resize = False
    animated = False
    reply = m.reply_to_message
    user = await c.resolve_peer(m.from_user.username or m.from_user.id)

    if reply and reply.media:
        if reply.photo:
            resize = True
        elif reply.document:
            if "image" in reply.document.mime_type:
                # mime_type: image/webp
                resize = True
            elif "tgsticker" in reply.document.mime_type:
                # mime_type: application/x-tgsticker
                animated = True
        elif reply.sticker:
            if not reply.sticker.file_name:
                return await prog_msg.edit_text(
                    await tld(m.chat.id, "err_sticker_no_file_name")
                )
            if reply.sticker.emoji:
                sticker_emoji = reply.sticker.emoji
            animated = reply.sticker.is_animated
            if not reply.sticker.file_name.endswith(".tgs"):
                resize = True
        else:
            return await prog_msg.edit_text(
                await tld(m.chat.id, "invalid_media_string")
            )
        pack_prefix = "anim" if animated else "a"
        packname = f"{pack_prefix}_{m.from_user.id}_by_{bot_username}"

        if len(m.command) > 1:
            if m.command[1].isdigit() and int(m.command[1]) > 0:
                # provide pack number to kang in desired pack
                packnum = m.command.pop(1)
                packname = f"{pack_prefix}{packnum}_{m.from_user.id}_by_{bot_username}"
            if len(m.command) > 1:
                # matches all valid emojis in input
                sticker_emoji = (
                    "".join(set(EMOJI_PATTERN.findall("".join(m.command[1:]))))
                    or sticker_emoji
                )
        filename = await c.download_media(m.reply_to_message)
        if not filename:
            # Failed to download
            await prog_msg.delete()
            return
    elif m.entities and len(m.entities) > 1:
        packname = f"c{m.from_user.id}_by_{bot_username}"
        pack_prefix = "a"
        # searching if image_url is given
        img_url = None
        filename = "sticker.png"
        for y in m.entities:
            if y.type == "url":
                img_url = m.text[y.offset : (y.offset + y.length)]
                break
        if not img_url:
            await prog_msg.delete()
            return
        try:
            r = await http.get(img_url)
            if r.status_code == 200:
                with open(filename, mode="wb") as f:
                    f.write(r.read())
        except Exception as r_e:
            return await prog_msg.edit_text(f"{r_e.__class__.__name__} : {r_e}")
        if len(m.command) > 2:
            # m.command[1] is image_url
            if m.command[2].isdigit() and int(m.command[2]) > 0:
                packnum = m.command.pop(2)
                packname = f"a{packnum}_{m.from_user.id}_by_{bot_username}"
            if len(m.command) > 2:
                sticker_emoji = (
                    "".join(set(EMOJI_PATTERN.findall("".join(m.command[2:]))))
                    or sticker_emoji
                )
            resize = True
    else:
        return await prog_msg.edit_text(await tld(m.chat.id, "stickers_kang_noreply"))
    try:
        if resize:
            filename = resize_image(filename)
        max_stickers = 50 if animated else 120
        while not packname_found:
            try:
                stickerset = await c.send(
                    GetStickerSet(
                        stickerset=InputStickerSetShortName(short_name=packname)
                    )
                )
                if stickerset.set.count >= max_stickers:
                    packnum += 1
                    packname = (
                        f"{pack_prefix}_{packnum}_{m.from_user.id}_by_{bot_username}"
                    )
                else:
                    packname_found = True
            except StickersetInvalid:
                break
        file = await c.save_file(filename)
        media = await c.send(
            SendMedia(
                peer=(await c.resolve_peer(CHAT_LOGS)),
                media=InputMediaUploadedDocument(
                    file=file,
                    mime_type=c.guess_mime_type(filename),
                    attributes=[DocumentAttributeFilename(file_name=filename)],
                ),
                message=f"#Sticker kang by UserID -> {m.from_user.id}",
                random_id=c.rnd_id(),
            ),
        )
        stkr_file = media.updates[-1].message.media.document
        if packname_found:
            await prog_msg.edit_text(await tld(m.chat.id, "use_existing_pack"))
            await c.send(
                AddStickerToSet(
                    stickerset=InputStickerSetShortName(short_name=packname),
                    sticker=InputStickerSetItem(
                        document=InputDocument(
                            id=stkr_file.id,
                            access_hash=stkr_file.access_hash,
                            file_reference=stkr_file.file_reference,
                        ),
                        emoji=sticker_emoji,
                    ),
                )
            )
        else:
            await prog_msg.edit_text(await tld(m.chat.id, "create_new_pack_string"))
            stkr_title = f"{m.from_user.first_name[:32]}'s "
            if animated:
                stkr_title += "Anim. "
            stkr_title += "Smudge Pack"
            if packnum != 0:
                stkr_title += f" v{packnum}"
            try:
                await c.send(
                    CreateStickerSet(
                        user_id=user,
                        title=stkr_title,
                        short_name=packname,
                        stickers=[
                            InputStickerSetItem(
                                document=InputDocument(
                                    id=stkr_file.id,
                                    access_hash=stkr_file.access_hash,
                                    file_reference=stkr_file.file_reference,
                                ),
                                emoji=sticker_emoji,
                            )
                        ],
                        animated=animated,
                    )
                )
            except PeerIdInvalid:
                return await prog_msg.edit_text(
                    await tld(m.chat.id, "stickers_pack_contact_pm"),
                    reply_markup=InlineKeyboardMarkup(
                        [
                            [
                                InlineKeyboardButton(
                                    "/start", url=f"https://t.me/{bot_username}?start"
                                )
                            ]
                        ]
                    ),
                )
    except Exception as all_e:
        await prog_msg.edit_text(f"{all_e.__class__.__name__} : {all_e}")
    else:
        await prog_msg.edit_text(
            (await tld(m.chat.id, "sticker_kanged_string")).format(
                packname, sticker_emoji
            )
        )
        # Cleanup
        try:
            os.remove(filename)
        except OSError:
            pass


def resize_image(filename: str) -> str:
    im = Image.open(filename)
    maxsize = 512
    scale = maxsize / max(im.width, im.height)
    sizenew = (int(im.width * scale), int(im.height * scale))
    im = im.resize(sizenew, Image.NEAREST)
    downpath, f_name = os.path.split(filename)
    # not hardcoding png_image as "sticker.png"
    png_image = os.path.join(downpath, f"{f_name.split('.', 1)[0]}.png")
    im.save(png_image, "PNG")
    if png_image != filename:
        os.remove(filename)
    return png_image


plugin_name = "stickers_name"
plugin_help = "stickers_help"
