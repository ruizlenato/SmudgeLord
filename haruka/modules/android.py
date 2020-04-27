#    Haruka Aya (A telegram bot project)
#    Copyright (C) 2017-2019 Paul Larsen
#    Copyright (C) 2019-2020 Akito Mizukito (Haruka Network Development)

#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU Affero General Public License as published by
#    the Free Software Foundation, either version 3 of the License, or
#    (at your option) any later version.

#    This program is distributed in the hope that it will be useful,
#    but WITHOUT ANY WARRANTY; without even the implied warranty of
#    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
#    GNU Affero General Public License for more details.

#    You should have received a copy of the GNU Affero General Public License
#    along with this program.  If not, see <https://www.gnu.org/licenses/>.

from datetime import datetime
from typing import List
from hurry.filesize import size as sizee

from telegram import Update, Bot
from telegram import ParseMode, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import run_async

from haruka import dispatcher, LOGGER
from haruka.modules.disable import DisableAbleCommandHandler
from haruka.modules.tr_engine.strings import tld

from requests import get

# Greeting all bot owners that is using this module,
# - RealAkito (used to be peaktogoo) [Module Maker]
# have spent so much time of their life into making this module better, stable, and well more supports.
# Please don't remove these comment, if you're still respecting me, the module maker.
#
# This module was inspired by Android Helper Bot by Vachounet.
# None of the code is taken from the bot itself, to avoid confusion.

LOGGER.info("android: Original Android Modules by @RealAkito on Telegram")


@run_async
def posp(bot: Bot, update: Update, args: List[str]):
    message = update.effective_message
    chat = update.effective_chat
    try:
        device = args[0]
    except:
        device = ''

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format("posp")
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get(
        f'https://api.potatoproject.co/checkUpdate?device={device}&type=weekly'
    )
    if fetch.status_code == 200 and len(fetch.json()['response']) != 0:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']

        reply_text = tld(chat.id, "download").format(filename, url)
        reply_text += tld(chat.id, "build_size").format(buildsize_b)
        reply_text += tld(chat.id, "version").format(version)

        keyboard = [[
            InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
        ]]
        message.reply_text(reply_text,
                           reply_markup=InlineKeyboardMarkup(keyboard),
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    else:
        reply_text = tld(chat.id, "err_not_found")
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def los(bot: Bot, update: Update, args: List[str]):
    message = update.effective_message
    chat = update.effective_chat
    try:
        device = args[0]
    except:
        device = ''

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format("los")
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get(f'https://download.lineageos.org/api/v1/{device}/nightly/*')
    if fetch.status_code == 200 and len(fetch.json()['response']) != 0:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']

        reply_text = tld(chat.id, "download").format(filename, url)
        reply_text += tld(chat.id, "build_size").format(buildsize_b)
        reply_text += tld(chat.id, "version").format(version)

        keyboard = [[
            InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
        ]]
        message.reply_text(reply_text,
                           reply_markup=InlineKeyboardMarkup(keyboard),
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    else:
        reply_text = tld(chat.id, "err_not_found")
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def evo(bot: Bot, update: Update, args: List[str]):
    message = update.effective_message
    chat = update.effective_chat
    try:
        device = args[0]
    except:
        device = ''

    if device == "example":
        reply_text = tld(chat.id, "err_example_device")
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    if device == "x00t":
        device = "X00T"

    if device == "x01bd":
        device = "X01BD"

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format("evo")
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get(
        f'https://raw.githubusercontent.com/Evolution-X-Devices/official_devices/master/builds/{device}.json'
    )

    if fetch.status_code in [500, 504, 505]:
        message.reply_text(
            "Haruka Aya have been trying to connect to Github User Content, It seem like Github User Content is down"
        )
        return

    if fetch.status_code == 200:
        try:
            usr = fetch.json()
            filename = usr['filename']
            url = usr['url']
            version = usr['version']
            maintainer = usr['maintainer']
            maintainer_url = usr['telegram_username']
            size_a = usr['size']
            size_b = sizee(int(size_a))

            reply_text = tld(chat.id, "download").format(filename, url)
            reply_text += tld(chat.id, "build_size").format(size_b)
            reply_text += tld(chat.id, "android_version").format(version)
            reply_text += tld(chat.id, "maintainer").format(
                f"[{maintainer}](https://t.me/{maintainer_url})")

            keyboard = [[
                InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
            ]]
            message.reply_text(reply_text,
                               reply_markup=InlineKeyboardMarkup(keyboard),
                               parse_mode=ParseMode.MARKDOWN,
                               disable_web_page_preview=True)
            return

        except ValueError:
            reply_text = tld(chat.id, "err_json")
            message.reply_text(reply_text,
                               parse_mode=ParseMode.MARKDOWN,
                               disable_web_page_preview=True)
            return

    elif fetch.status_code == 404:
        reply_text = tld(chat.id, "err_not_found")
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return


def phh(bot: Bot, update: Update):
    romname = "Phh's"
    message = update.effective_message
    chat = update.effective_chat

    usr = get(
        f'https://api.github.com/repos/phhusson/treble_experimentations/releases/latest'
    ).json()
    reply_text = tld(chat.id, "cust_releases").format(romname)
    for i in range(len(usr)):
        try:
            name = usr['assets'][i]['name']
            url = usr['assets'][i]['browser_download_url']
            reply_text += f"[{name}]({url})\n"
        except IndexError:
            continue
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN)


@run_async
def bootleggers(bot: Bot, update: Update, args: List[str]):
    message = update.effective_message
    chat = update.effective_chat
    try:
        codename = args[0]
    except:
        codename = ''

    if codename == '':
        reply_text = tld(chat.id, "cmd_example").format("bootleggers")
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get('https://bootleggersrom-devices.github.io/api/devices.json')
    if fetch.status_code == 200:
        nestedjson = fetch.json()

        if codename.lower() == 'x00t':
            devicetoget = 'X00T'
        else:
            devicetoget = codename.lower()

        reply_text = ""
        devices = {}

        for device, values in nestedjson.items():
            devices.update({device: values})

        if devicetoget in devices:
            for oh, baby in devices[devicetoget].items():
                dontneedlist = ['id', 'filename', 'download', 'xdathread']
                peaksmod = {
                    'fullname': 'Device name',
                    'buildate': 'Build date',
                    'buildsize': 'Build size',
                    'downloadfolder': 'SourceForge folder',
                    'mirrorlink': 'Mirror link',
                    'xdathread': 'XDA thread'
                }
                if baby and oh not in dontneedlist:
                    if oh in peaksmod:
                        oh = peaksmod[oh]
                    else:
                        oh = oh.title()

                    if oh == 'SourceForge folder':
                        reply_text += f"\n*{oh}:* [Here]({baby})"
                    elif oh == 'Mirror link':
                        reply_text += f"\n*{oh}:* [Here]({baby})"
                    else:
                        reply_text += f"\n*{oh}:* `{baby}`"

            reply_text += tld(chat.id, "xda_thread").format(
                devices[devicetoget]['xdathread'])
            reply_text += tld(chat.id, "download").format(
                devices[devicetoget]['filename'],
                devices[devicetoget]['download'])
        else:
            reply_text = tld(chat.id, "err_not_found")

    elif fetch.status_code == 404:
        reply_text = tld(chat.id, "err_api")
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def magisk(bot: Bot, update: Update):
    message = update.effective_message
    url = 'https://raw.githubusercontent.com/topjohnwu/magisk_files/'
    releases = '*Latest Magisk Releases:*\n'
    variant = [
        'master/stable', 'master/beta', 'canary/release', 'canary/debug'
    ]
    for variants in variant:
        data = get(url + variants + '.json').json()
        if variants == "master/stable":
            name = "*Stable*"
        elif variants == "master/beta":
            name = "*Beta*"
        elif variants == "canary/release":
            name = "*Canary*"
        elif variants == "canary/debug":
            name = "*Canary (Debug)*"

        releases += f'{name}: [ZIP v{data["magisk"]["version"]}]({data["magisk"]["link"]}) | ' \
                    f'[APK v{data["app"]["version"]}]({data["app"]["link"]}) | ' \
                    f'[Uninstaller]({data["uninstaller"]["link"]})\n'

    message.reply_text(releases,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


__help__ = True

EVO_HANDLER = DisableAbleCommandHandler("evo",
                                        evo,
                                        pass_args=True,
                                        admin_ok=True)
PHH_HANDLER = DisableAbleCommandHandler("phh", phh, admin_ok=True)
POSP_HANDLER = DisableAbleCommandHandler("posp",
                                         posp,
                                         pass_args=True,
                                         admin_ok=True)
LOS_HANDLER = DisableAbleCommandHandler("los",
                                        los,
                                        pass_args=True,
                                        admin_ok=True)
BOOTLEGGERS_HANDLER = DisableAbleCommandHandler("bootleggers",
                                                bootleggers,
                                                pass_args=True,
                                                admin_ok=True)
MAGISK_HANDLER = DisableAbleCommandHandler("magisk", magisk, admin_ok=True)

dispatcher.add_handler(EVO_HANDLER)
dispatcher.add_handler(PHH_HANDLER)
# dispatcher.add_handler(POSP_HANDLER)
dispatcher.add_handler(LOS_HANDLER)
dispatcher.add_handler(BOOTLEGGERS_HANDLER)
dispatcher.add_handler(MAGISK_HANDLER)
