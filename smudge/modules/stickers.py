import os
from telegram import Message, Chat, Update
from telegram import ParseMode
from telegram.ext import run_async
from telegram.utils.helpers import escape_markdown

from smudge import dispatcher, CallbackContext
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.helper_funcs.filters import CustomFilters
from smudge.modules.translations.strings import tld

combot_stickers_url = "https://combot.org/telegram/stickers?q="


def stickerid(update: Update, context: CallbackContext):
    bot = context.bot
    msg = update.effective_message
    if msg.reply_to_message and msg.reply_to_message.sticker:
        update.effective_message.reply_text(
            "Sticker ID:\n```" + msg.reply_to_message.sticker.file_id + "```",
            parse_mode=ParseMode.MARKDOWN)
    else:
        update.effective_message.reply_text(
            "Please reply to a sticker to get its ID.")


def getsticker(update: Update, context: CallbackContext):
    bot = context.bot
    msg = update.effective_message
    chat_id = update.effective_chat.id
    if msg.reply_to_message and msg.reply_to_message.sticker:
        file_id = msg.reply_to_message.sticker.file_id
        newFile = bot.get_file(file_id)
        newFile.download('sticker.png')
        bot.sendDocument(chat_id, document=open('sticker.png', 'rb'))
        os.remove("sticker.png")

    else:
        update.effective_message.reply_text(
            "Please reply to a sticker for me to upload its PNG.")


def cb_sticker(update: Update, context: CallbackContext):
    bot = context.bot
    msg = update.effective_message
    split = msg.text.split(' ', 1)
    if len(split) == 1:
        msg.reply_text('Provide some name to search for pack.')
        return
    text = requests.get(combot_stickers_url + split[1]).text
    soup = bs(text, 'lxml')
    results = soup.find_all("a", {'class': "sticker-pack__btn"})
    titles = soup.find_all("div", "sticker-pack__title")
    if not results:
        msg.reply_text('No results found :(.')
        return
    reply = f"Stickers for *{split[1]}*:"
    for result, title in zip(results, titles):
        link = result['href']
        reply += f"\n• [{title.get_text()}]({link})"
    msg.reply_text(reply, parse_mode=ParseMode.MARKDOWN)


def kang(update: Update, context: CallbackContext):
    msg = update.effective_message
    user = update.effective_user
    args = context.args
    chat = update.effective_chat
    packnum = 0
    packname = "c" + str(user.id) + "_by_" + context.bot.username
    packname_found = 0
    max_stickers = 120
    while packname_found == 0:
        try:
            stickerset = context.bot.get_sticker_set(packname)
            if len(stickerset.stickers) >= max_stickers:
                packnum += 1
                packname = "c" + str(packnum) + "_" + str(
                    user.id) + "_by_" + context.bot.username
            else:
                packname_found = 1
        except TelegramError as e:
            if e.message == "Stickerset_invalid":
                packname_found = 1
    kangsticker = "kangsticker.png"
    if msg.reply_to_message:
        if msg.reply_to_message.sticker:
            file_id = msg.reply_to_message.sticker.file_id
        elif msg.reply_to_message.photo:
            file_id = msg.reply_to_message.photo[-1].file_id
        elif msg.reply_to_message.document:
            file_id = msg.reply_to_message.document.file_id
        else:
            msg.reply_text(tld(chat.id, 'stickers_kang_error'))
        kang_file = context.bot.get_file(file_id)
        kang_file.download('kangsticker.png')
        if args:
            sticker_emoji = str(args[0])
        elif msg.reply_to_message.sticker and msg.reply_to_message.sticker.emoji:
            sticker_emoji = msg.reply_to_message.sticker.emoji
        else:
            sticker_emoji = "🤔"
        try:
            im = Image.open(kangsticker)
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
            if not msg.reply_to_message.sticker:
                im.save(kangsticker, "PNG")
            context.bot.add_sticker_to_set(user_id=user.id,
                                           name=packname,
                                           png_sticker=open(
                                               'kangsticker.png', 'rb'),
                                           emojis=sticker_emoji)
            msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, sticker_emoji),
                           parse_mode=ParseMode.MARKDOWN)
        except OSError as e:
            msg.reply_text(tld(chat.id, 'stickers_kang_only_img'))
            print(e)
            return
        except TelegramError as e:
            if e.message == "Stickerset_invalid":
                makepack_internal(update, context, msg, user, sticker_emoji,
                                  packname, packnum, png_sticker=open("kangsticker.png", "rb"),)
            elif e.message == "Sticker_png_dimensions":
                im.save(kangsticker, "PNG")
                context.bot.add_sticker_to_set(user_id=user.id,
                                               name=packname,
                                               png_sticker=open(
                                                   'kangsticker.png', 'rb'),
                                               emojis=sticker_emoji)
                msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, sticker_emoji),
                               parse_mode=ParseMode.MARKDOWN)
            elif e.message == "Invalid sticker emojis":
                msg.reply_text(tld(chat.id, 'stickers_kang_invalid_emoji'))
            elif e.message == "Stickers_too_much":
                msg.reply_text(tld(chat.id, 'stickers_kang_too_much'))
            elif e.message == "Internal Server Error: sticker set not found (500)":
                msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, sticker_emoji),
                               parse_mode=ParseMode.MARKDOWN)
            print(e)

    elif args:
        try:
            try:
                urlemoji = msg.text.split(" ")
                png_sticker = urlemoji[1]
                sticker_emoji = urlemoji[2]
            except IndexError:
                sticker_emoji = "🤔"
            urllib.urlretrieve(png_sticker, kangsticker)
            im = Image.open(kangsticker)
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
            im.save(kangsticker, "PNG")
            msg.reply_photo(photo=open('kangsticker.png', 'rb'))
            context.bot.add_sticker_to_set(user_id=user.id, name=packname, png_sticker=open(
                "kangsticker.png", "rb"), emojis=sticker_emoji,)
            msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, sticker_emoji),
                           parse_mode=ParseMode.MARKDOWN)
        except OSError as e:
            msg.reply_text(tld(chat.id, 'stickers_kang_only_img'))
            print(e)
            return
        except TelegramError as e:
            if e.message == "Stickerset_invalid":
                makepack_internal(msg, user, open('kangsticker.png', 'rb'),
                                  sticker_emoji, bot, packname, packnum)
            elif e.message == "Sticker_png_dimensions":
                im.save(kangsticker, "PNG")
                context.bot.add_sticker_to_set(user_id=user.id,
                                               name=packname,
                                               png_sticker=open(
                                                   'kangsticker.png', 'rb'),
                                               emojis=sticker_emoji)
                msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, sticker_emoji),
                               parse_mode=ParseMode.MARKDOWN)
            elif e.message == "Invalid sticker emojis":
                msg.reply_text(tld(chat.id, 'stickers_kang_invalid_emoji'))
            elif e.message == "Stickers_too_much":
                msg.reply_text(tld(chat.id, 'stickers_kang_too_much'))
            elif e.message == "Internal Server Error: sticker set not found (500)":
                msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, sticker_emoji),
                               parse_mode=ParseMode.MARKDOWN)
            print(e)
    else:
        packs = tld(chat.id, 'stickers_kang_no_reply')
        if packnum > 0:
            firstpackname = "c" + str(user.id) + "_by_" + context.bot.username
            for i in range(0, packnum + 1):
                if i == 0:
                    packs += f"[pack](t.me/addstickers/{firstpackname})\n"
                else:
                    packs += f"[pack{i}](t.me/addstickers/{packname})\n"
        else:
            packs += f"[pack](t.me/addstickers/{packname})"
        msg.reply_text(packs, parse_mode=ParseMode.MARKDOWN)
    if os.path.isfile("kangsticker.png"):
        os.remove("kangsticker.png")


def makepack_internal(update, context, msg, user, emoji, packname, packnum, png_sticker=None, tgs_sticker=None):
    message = update.effective_message
    chat = update.effective_chat
    name = user.first_name
    name = name[:50]
    try:
        extra_version = ""
        if packnum > 0:
            extra_version = " " + str(packnum)
        success = success = context.bot.create_new_sticker_set(
            user.id, packname, f"{name}'s kang pack" + extra_version, png_sticker=png_sticker, emojis=emoji)
    except TelegramError as e:
        print(e)
        if e.message == "Sticker set name is already occupied":
            msg.reply_text(
                tld(chat.id, 'stickers_pack_name_exists') % packname,
                parse_mode=ParseMode.MARKDOWN)
        elif e.message == "Peer_id_invalid":
            msg.reply_text(tld(chat.id, 'stickers_pack_contact_pm'),
                           reply_markup=InlineKeyboardMarkup([[
                               InlineKeyboardButton(text="Start",
                                                    url=f"t.me/{context.bot.username}")
                           ]]))
        elif e.message == "Internal Server Error: created sticker set not found (500)":
            msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, sticker_emoji),
                           parse_mode=ParseMode.MARKDOWN)
        return

    if success:
        msg.reply_text(tld(chat.id, 'stickers_kang_success').format(packname, emoji),
                       parse_mode=ParseMode.MARKDOWN)
    else:
        msg.reply_text(tld(chat.id, 'stickers_pack_create_error'))


__help__ = True

STICKERID_HANDLER = DisableAbleCommandHandler(
    "stickerid", stickerid, run_async=True)
GETSTICKER_HANDLER = DisableAbleCommandHandler(
    "getsticker", getsticker, filters=CustomFilters.sudo_filter, run_async=True)
KANG_HANDLER = DisableAbleCommandHandler(
    "kang", kang, admin_ok=True, run_async=True)
STICKERS_HANDLER = DisableAbleCommandHandler(
    "stickers", cb_sticker, run_async=True)

dispatcher.add_handler(STICKERID_HANDLER)
dispatcher.add_handler(GETSTICKER_HANDLER)
dispatcher.add_handler(KANG_HANDLER)
dispatcher.add_handler(STICKERS_HANDLER)
