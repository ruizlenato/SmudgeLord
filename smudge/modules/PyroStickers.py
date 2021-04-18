import os
import imghdr 

from pyrogram.errors.exceptions.bad_request_400 import PeerIdInvalid, UserIsBlocked, StickerPngNopng
from smudge.modules.translations.strings import tld
from pyrogram import Client, filters, errors, raw
from smudge import SUDO_USERS, TOKEN, PyroSmudge
from pyrogram.file_id import FileId
from pyrogram.types import Message
from typing import List
from PIL import Image

SUPPORTED_TYPES = ['jpeg', 'png', 'webp']

@PyroSmudge.on_message(filters.command("getsticker"))
async def getsticker(client, message):
    if message.reply_to_message.sticker:
        sticker = await client.download_media(message.reply_to_message.sticker)
        file_id = message.reply_to_message.sticker.file_id
        await client.send_document(document=sticker,
                                   force_document=True,
                                   chat_id=message.chat.id,
                                   reply_to_message_id=message.message_id,
                                   caption=("<strong>Sticker ID:</strong> <code>{}</code>" ).format( file_id))
    else:
        await message.reply_text(tld(msssage.chat.id, 'stickers_getsticker_no_reply'))
        return

@PyroSmudge.on_message(filters.command("kang"))
async def kang(client, message):
    if not message.reply_to_message:
        await message.reply_text(tld(msssage.chat.id, 'stickers_kang_error'))
        return
    msg = await message.reply_text("Kanging Sticker...")

    # Find the proper emoji
    args = message.text.split()
    if len(args) > 1:
        sticker_emoji = str(args[1])
    elif message.reply_to_message.sticker and message.reply_to_message.sticker.emoji:
        sticker_emoji = message.reply_to_message.sticker.emoji
    else:
        sticker_emoji = "🤔"

    # Get the corresponding fileid, resize the file if necessary
    doc = (message.reply_to_message.photo or message.reply_to_message.document)
    if message.reply_to_message.sticker:
        sticker = await create_sticker(
                await get_document_from_file_id(
                    message.reply_to_message.sticker.file_id), sticker_emoji)
    elif doc:
        temp_file_path = await PyroSmudge.download_media(doc)
        image_type = imghdr.what(temp_file_path)
        if image_type not in SUPPORTED_TYPES:
            await msg.edit("Format not supported! ({})".format(image_type))
            return
        try:
            temp_file_path = await resize_file_to_sticker_size(temp_file_path)
        except OSError as e:
            await msg.edit_text("Something wrong happened.")
            raise Exception(
                f"Something went wrong while resizing the sticker (at {temp_file_path}); {e}")
            return False
        sticker = await create_sticker(
                await upload_document(client, temp_file_path, message.chat.id),sticker_emoji)
        if os.path.isfile(temp_file_path):
            os.remove(temp_file_path)
    else:
        await msg.edit(tld(message.chat.id, 'stickers_kang_error'))
        return

    # Find an available pack & add the sticker to the pack; create a new pack if needed
    # Would be a good idea to cache the number instead of searching it every single time...
    user = await PyroSmudge.get_me()
    packnum = 0
    packname = "c" + str(message.from_user.id) + "_by_" +  user.username
    try:
        while True:
            stickerset = await get_sticker_set_by_name(client, packname)
            if not stickerset:
                stickerset = await create_sticker_set(
                        client,
                        message.from_user.id,
                        f"{message.from_user.first_name[:32]}'s kang pack",
                        packname,
                        [sticker]
                        )
            elif stickerset.set.count >= 120:
                packnum += 1
                packname = "f" + str(packnum) + "_" + \
                    str(message.from_user.id) + "_by_"+user.username
                continue
            else:
                await add_sticker_to_set(client, stickerset, sticker)
            break

        await msg.edit(tld(message.chat.id, 'stickers_kang_success').format(packname, sticker_emoji))
    except (PeerIdInvalid, UserIsBlocked):
        keyboard = InlineKeyboardMarkup(
            [[InlineKeyboardButton(text="Start", url=f"t.me/{user.username}")]])
        await msg.edit(tld(message.chat.id, 'stickers_pack_contact_pm'), reply_markup=keyboard)
    except StickerPngNopng:
        await message.reply_text("Stickers must be png files but the provided image was not a png")
        

async def get_sticker_set_by_name(client: Client, name: str) -> raw.base.messages.StickerSet:
    try:
        return await client.send(raw.functions.messages.GetStickerSet(stickerset=raw.types.InputStickerSetShortName(short_name=name)))
    except errors.exceptions.not_acceptable_406.StickersetInvalid:
        return None

async def create_sticker_set(client: Client, owner: int, title: str, short_name: str, stickers: List[raw.base.InputStickerSetItem]) -> raw.base.messages.StickerSet:
    return await client.send(raw.functions.stickers.CreateStickerSet(user_id=await client.resolve_peer(owner), title=title, short_name=short_name,stickers=stickers))


async def add_sticker_to_set(client: Client, stickerset: raw.base.messages.StickerSet, sticker: raw.base.InputStickerSetItem) -> raw.base.messages.StickerSet:
    return await client.send(raw.functions.stickers.AddStickerToSet(stickerset=raw.types.InputStickerSetShortName(short_name=stickerset.set.short_name), sticker=sticker))

async def create_sticker(sticker: raw.base.InputDocument, emoji: str) -> raw.base.InputStickerSetItem:
    return raw.types.InputStickerSetItem(document=sticker, emoji=emoji)

async def resize_file_to_sticker_size(file_path: str) -> str:
    im = Image.open(file_path)
    maxsize = (512, 512)
    if (im.width and im.height) < 512:
        size1 = im.width
        size2 = im.height
        if im.width > im.height:
            scale = 512 / size1
            size1new = 512
            size2new = size2 * scale
        else:
            scale = 512 / size2
            size1new = size1 * scale
            size2new = 512
        size1new = math.floor(size1new)
        size2new = math.floor(size2new)
        sizenew = (size1new, size2new)
        im = im.resize(sizenew)
    else:
        im.thumbnail(maxsize)
    try:
        os.remove(file_path)
        file_path = f"{file_path}.png"
        return file_path
    finally:
        im.save(file_path)
       
async def upload_document(client: Client, file_path: str, chat_id: int) -> raw.base.InputDocument:
    media = await client.send(
        raw.functions.messages.UploadMedia(
            peer=await client.resolve_peer(chat_id),
            media=raw.types.InputMediaUploadedDocument(
                mime_type=client.guess_mime_type(
                    file_path) or "application/zip",
                file=await client.save_file(file_path),
                attributes=[raw.types.DocumentAttributeFilename(
                        file_name=os.path.basename(file_path))])))
    return raw.types.InputDocument(id=media.document.id,
                                   access_hash=media.document.access_hash,
                                   file_reference=media.document.file_reference)


async def get_document_from_file_id(file_id: str) -> raw.base.InputDocument:
    decoded = FileId.decode(file_id)
    return raw.types.InputDocument(id=decoded.media_id,
                                    access_hash=decoded.access_hash,
                                   file_reference=decoded.file_reference)