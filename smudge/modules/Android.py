import re
import html
import time
from bs4 import BeautifulSoup
import rapidjson as json
from requests import get
from telegram import Message, Update, Bot, User, Chat, ParseMode, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.error import BadRequest
from telegram.ext import run_async
from telegram.utils.helpers import escape_markdown, mention_html

from smudge import dispatcher, updater, CallbackContext
from smudge.modules.disable import DisableAbleCommandHandler
from smudge.modules.translations.strings import tld

DEVICES_DATA = 'https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json'


def magisk(update: Update, context: CallbackContext):
    bot = context.bot
    url = 'https://raw.githubusercontent.com/topjohnwu/magisk-files/master/'
    chat = update.effective_chat
    releases = tld(chat.id, "magisk_releases")
    for type, branch in {
            "Stable": "stable",
            "Beta": "beta",
            "Canary": "canary"
    }.items():
        data = get(url + branch + '.json').json()

        releases += f'*{type}:* [Magisk v{data["magisk"]["version"]}]({data["magisk"]["link"]}) | ' \
                    f'[Changelog]({data["magisk"]["note"]})\n'
                        
    update.message.reply_text(
        releases, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


def device(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    chat = update.effective_chat
    if len(args) == 0:
        update.effective_message.reply_text(tld(
            chat.id, "whatis_no_device"), parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    device = " ".join(args)
    db = get(DEVICES_DATA).json()
    newdevice = device.strip('lte') if device.startswith('beyond') else device
    try:
        brand = db[newdevice][0]['brand']
        name = db[newdevice][0]['name']
        model = db[newdevice][0]['model']
        codename = newdevice
        reply = tld(chat.id, "whatis_device").format(codename, brand, name)
    except KeyError as err:
        reply = f"Couldn't find info about {device}!\n"
        update.effective_message.reply_text(
            "{}".format(reply),
            parse_mode=ParseMode.MARKDOWN,
            disable_web_page_preview=True)
        return
    update.message.reply_text("{}".format(reply),
                              parse_mode=ParseMode.MARKDOWN,
                              disable_web_page_preview=True)


def twrp(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    chat = update.effective_chat
    if len(args) == 0:
        reply = 'No codename provided, write a codename for fetching informations.'
        update.effective_message.reply_text(
            "{}".format(reply),
            parse_mode=ParseMode.MARKDOWN,
            disable_web_page_preview=True)
        return

    device = " ".join(args)
    url = get(f'https://eu.dl.twrp.me/{device}/')
    if url.status_code == 404:
        reply = f"Couldn't find twrp downloads for {device}!\n"
        del_msg = update.effective_message.reply_text(
            "{}".format(reply),
            parse_mode=ParseMode.MARKDOWN,
            disable_web_page_preview=True)
        time.sleep(5)
        try:
            del_msg.delete()
            update.effective_message.delete()
        except BadRequest as err:
            if (err.message == "Message to delete not found") or (
                    err.message == "Message can't be deleted"):
                return
    else:
        reply = f'*Latest Official TWRP for {device}*\n'
        db = get(DEVICES_DATA).json()
        newdevice = device.strip('lte') if device.startswith(
            'beyond') else device
        try:
            brand = db[newdevice][0]['brand']
            name = db[newdevice][0]['name']
            reply += f'*{brand} - {name}*\n'
        except KeyError as err:
            pass
        page = BeautifulSoup(url.content, 'lxml')
        date = page.find("em").text.strip()
        reply += f'*Updated:* {date}\n'
        trs = page.find('table').find_all('tr')
        row = 2 if trs[0].find('a').text.endswith('tar') else 1
        if trs[0].find('a').text.endswith('tar'):
            for i in range(row):
                download = trs[i].find('a')
                dl_link = f"https://eu.dl.twrp.me{download['href']}"
                dl_file = download.text
                size = trs[i].find("span", {"class": "filesize"}).text
                keyboard = [
                    [InlineKeyboardButton(text=dl_file, url=dl_link)],
                    [InlineKeyboardButton(text=dl_file, url=dl_link)]]

        else:
            download = page.find('table').find('tr').find('a')
            dl_link = f"https://eu.dl.twrp.me{download['href']}"
            dl_file = download.text
            size = page.find("span", {"class": "filesize"}).text
            keyboard = [[InlineKeyboardButton(text=dl_file, url=dl_link)]]

        update.message.reply_text(tld(chat.id, "twrp_release").format(device, brand, name, date),
                                  parse_mode=ParseMode.MARKDOWN,
                                  reply_markup=InlineKeyboardMarkup(keyboard),
                                  disable_web_page_preview=True)


__help__ = True

__mod_name__ = "Android"

MAGISK_HANDLER = DisableAbleCommandHandler("magisk", magisk, run_async=True)
DEVICE_HANDLER = DisableAbleCommandHandler("device", device, run_async=True)
TWRP_HANDLER = DisableAbleCommandHandler("twrp", twrp, run_async=True)

dispatcher.add_handler(MAGISK_HANDLER)
dispatcher.add_handler(DEVICE_HANDLER)
dispatcher.add_handler(TWRP_HANDLER)
