import html
import json
import time
import yaml
from datetime import datetime
from typing import Optional, List
from hurry.filesize import size as sizee
from bs4 import BeautifulSoup


from telegram import Message, Chat, Update, Bot, MessageEntity
from telegram import ParseMode, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import CommandHandler, run_async, Filters
from telegram.utils.helpers import escape_markdown, mention_html

from haruka import dispatcher, LOGGER
from haruka.__main__ import GDPR
from haruka.__main__ import STATS, USER_INFO
from haruka.modules.disable import DisableAbleCommandHandler
from haruka.modules.helper_funcs.extraction import extract_user
from haruka.modules.helper_funcs.filters import CustomFilters
from haruka.modules.translations.strings import tld

from requests import get

# Greeting all bot owners that is using this module,
# - RealAkito (used to be peaktogoo) [Module Maker]
# have spent so much time of their life into making this module better, stable, and well more supports.
# Please don't remove these comment, if you're still respecting me, the module maker.
#
# This module was inspired by Android Helper Bot by Vachounet.
# None of the code is taken from the bot itself, to avoid confusion.

LOGGER.info("android: Original Android Modules by @RealAkito on Telegram, modified by @Renatoh on GitHub")
DEVICES_DATA = 'https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json'

@run_async
def getfw(bot, update, args):
    if not len(args) == 2:
        reply = f'Give me something to fetch, like: <code>/getfw SM-N975F DBT</code>'
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.HTML)
        return
    temp,csc = args
    model = f'sm-'+temp if not temp.upper().startswith('SM-') else temp
    test = get(f'https://samfrew.com/model/{model.upper()}/region/{csc.upper()}/')
    if test.status_code == 404:
        reply = f"Couldn't find any firmware downloads for <code>{model.upper()} {csc.upper()}</code>, make sure you gave me the right CSC and model!"
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.HTML)
        return
    url1 = f'https://samfrew.com/model/{model.upper()}/region/{csc.upper()}/'
    url2 = f'https://www.sammobile.com/samsung/firmware/{model.upper()}/{csc.upper()}/'
    url3 = f'https://sfirmware.com/samsung-{model.lower()}/#tab=firmwares'
    url4 = f'https://samfw.com/firmware/{model.upper()}/{csc.upper()}/'
    fota = get(f'http://fota-cloud-dn.ospserver.net/firmware/{csc.upper()}/{model.upper()}/version.xml')
    page = BeautifulSoup(fota.content, 'lxml')
    os = page.find("latest").get("o")
    reply = ""
    if page.find("latest").text.strip():
        pda,csc2,phone=page.find("latest").text.strip().split('/')
        reply += f'*Latest firmware for {model.upper()} {csc.upper()}:*\n'
        reply += f' • PDA: `{pda}`\n • CSC: `{csc2}`\n'
        if phone:
            reply += f' • Phone: `{phone}`\n'
        if os:
            reply += f' • Android: `{os}`\n'
    reply += f'\n'
    reply += f'*Downloads for {model.upper()} {csc.upper()}:*\n'
    reply += f' • [samfrew.com]({url1})\n'
    reply += f' • [sammobile.com]({url2})\n'
    reply += f' • [sfirmware.com]({url3})\n'
    reply += f' • [samfw.com]({url4}) ❇️\n\n'
    reply += f'❇️,the site with this symbol is the best site to download\n'
    update.message.reply_text("{}".format(reply),
                           parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def checkfw(bot, update, args):
    if not len(args) == 2:
        reply = f'Give me something to fetch, like:\n`/checkfw SM-N975F DBT`'
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    temp,csc = args
    model = f'sm-'+temp if not temp.upper().startswith('SM-') else temp
    fota = get(f'http://fota-cloud-dn.ospserver.net/firmware/{csc.upper()}/{model.upper()}/version.xml')
    test = get(f'http://fota-cloud-dn.ospserver.net/firmware/{csc.upper()}/{model.upper()}/version.test.xml')
    if test.status_code != 200:
        reply = f"Couldn't check for {temp.upper()} {csc.upper()}, make sure you gave me the right CSC and model!"
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    page1 = BeautifulSoup(fota.content, 'lxml')
    page2 = BeautifulSoup(test.content, 'lxml')
    os1 = page1.find("latest").get("o")
    os2 = page2.find("latest").get("o")
    if page1.find("latest").text.strip():
        pda1,csc1,phone1=page1.find("latest").text.strip().split('/')
        reply = f'*Latest released firmware for {model.upper()} {csc.upper()}:*\n'
        reply += f' • PDA: `{pda1}`\n • CSC: `{csc1}`\n'
        if phone1:
            reply += f' • Phone: `{phone1}`\n'
        if os1:
            reply += f' • Android: `{os1}`\n'
        reply += f'\n'
    else:
        reply = f'*No public release found for {model.upper()} {csc.upper()}.*\n\n'
    reply += f'*Latest test firmware for {model.upper()} {csc.upper()}:*\n'
    if len(page2.find("latest").text.strip().split('/')) == 3:
        pda2,csc2,phone2=page2.find("latest").text.strip().split('/')
        reply += f' • PDA: `{pda2}`\n • CSC: `{csc2}`\n'
        if phone2:
            reply += f' • Phone: `{phone2}`\n'
        if os2:
            reply += f' • Android: `{os2}`\n'
        reply += f'\n'
    else:
        md5=page2.find("latest").text.strip()
        reply += f' • Hash: `{md5}`\n • Android: `{os2}`\n\n'
    
    update.message.reply_text("{}".format(reply),
                           parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def magisk(bot, update):
    url = 'https://raw.githubusercontent.com/topjohnwu/magisk_files/'
    releases = ""
    for type, path in {"Stable": "master/stable", "Beta": "master/beta", "Canary": "canary/release"}.items():
        data = get(url + path + '.json').json()
        releases += f'{type}: [ZIP v{data["magisk"]["version"]}]({data["magisk"]["link"]}) | ' \
                    f'[APP v{data["app"]["version"]}]({data["app"]["link"]}) | ' \
                    f'[Uninstaller]({data["uninstaller"]["link"]})\n'

    update.message.reply_text("*Latest Magisk Releases:*\n{}".format(releases),
                              parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)

@run_async
def twrp(bot, update, args):
    if len(args) == 0:
        reply='No codename provided, write a codename for fetching informations.'
        del_msg = update.effective_message.reply_text("{}".format(reply),
                               parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        time.sleep(5)
        try:
            del_msg.delete()
            update.effective_message.delete()
        except BadRequest as err:
            if (err.message == "Message to delete not found" ) or (err.message == "Message can't be deleted" ):
                return

    device = " ".join(args)
    url = get(f'https://eu.dl.twrp.me/{device}/')
    if url.status_code == 404:
        reply = f"Couldn't find twrp downloads for {device}!\n"
        del_msg = update.effective_message.reply_text("{}".format(reply),
                               parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        time.sleep(5)
        try:
            del_msg.delete()
            update.effective_message.delete()
        except BadRequest as err:
            if (err.message == "Message to delete not found" ) or (err.message == "Message can't be deleted" ):
                return
    else:
        reply = f'*Latest Official TWRP for {device}*\n'            
        db = get(DEVICES_DATA).json()
        newdevice = device.strip('lte') if device.startswith('beyond') else device
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
        for i in range(row):
            download = trs[i].find('a')
            dl_link = f"https://eu.dl.twrp.me{download['href']}"
            dl_file = download.text
            size = trs[i].find("span", {"class": "filesize"}).text
            reply += f'[{dl_file}]({dl_link}) - {size}\n'

        update.message.reply_text("{}".format(reply),
                               parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
    
@run_async
def havoc(bot: Bot, update: Update):
    cmd_name = "havoc"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    device = message.text[len(f'/{cmd_name} '):]

    fetch = get(
        f'https://raw.githubusercontent.com/Havoc-Devices/android_vendor_OTA/pie/{device}.json'
    )

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    if fetch.status_code == 200:
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

    elif fetch.status_code == 404:
        reply_text = tld(chat.id, "err_not_found")

    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def pixys(bot: Bot, update: Update):
    cmd_name = "pixys"
    message = update.effective_message
    device = message.text[len(f'/{cmd_name} '):]
    chat = update.effective_chat  # type: Optional[Chat]

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get(
        f'https://raw.githubusercontent.com/PixysOS-Devices/official_devices/master/{device}/build.json'
    )
    if fetch.status_code == 200:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        romtype = response['romtype']
        version = response['version']

        reply_text = tld(chat.id, "download").format(filename, url)
        reply_text += tld(chat.id, "build_size").format(buildsize_b)
        reply_text += tld(chat.id, "version").format(version)
        reply_text += tld(chat.id, "rom_type").format(romtype)

        keyboard = [[
            InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
        ]]
        message.reply_text(reply_text,
                           reply_markup=InlineKeyboardMarkup(keyboard),
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    elif fetch.status_code == 404:
        reply_text = tld(chat.id, "err_not_found")
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def pearl(bot: Bot, update: Update):
    cmd_name = "pearl"
    message = update.effective_message
    device = message.text[len(f'/{cmd_name} '):]
    chat = update.effective_chat  # type: Optional[Chat]

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get(
        f'https://raw.githubusercontent.com/PearlOS/OTA/master/{device}.json')
    if fetch.status_code == 200:
        usr = fetch.json()
        response = usr['response'][0]
        maintainer = response['maintainer']
        romtype = response['romtype']
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']
        xda = response['xda']

        if xda == '':
            reply_text = tld(chat.id, "download").format(filename, url)
            reply_text += tld(chat.id, "build_size").format(buildsize_b)
            reply_text += tld(chat.id, "version").format(version)
            reply_text += tld(chat.id, "rom_type").format(romtype)
            reply_text += tld(chat.id, "maintainer").format(f"`{maintainer}`")

            keyboard = [[
                InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
            ]]
            message.reply_text(reply_text,
                               reply_markup=InlineKeyboardMarkup(keyboard),
                               parse_mode=ParseMode.MARKDOWN,
                               disable_web_page_preview=True)
            return

        reply_text = tld(chat.id, "download").format(filename, url)
        reply_text += tld(chat.id, "build_size").format(buildsize_b)
        reply_text += tld(chat.id, "version").format(version)
        reply_text += tld(chat.id, "rom_type").format(romtype)
        reply_text += tld(chat.id, "maintainer").format(f"`{maintainer}`")
        reply_text += tld(chat.id, "xda_thread").format(xda)

        keyboard = [[
            InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
        ]]
        message.reply_text(reply_text,
                           reply_markup=InlineKeyboardMarkup(keyboard),
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    elif fetch.status_code == 404:
        reply_text = tld(chat.id, "err_not_found")
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def posp(bot: Bot, update: Update):
    cmd_name = "posp"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    device = message.text[len(f'/{cmd_name} '):]

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
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
def los(bot: Bot, update: Update):
    cmd_name = "los"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    device = message.text[len(f'/{cmd_name} '):]

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
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
def dotos(bot: Bot, update: Update):
    cmd_name = "dotos"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    device = message.text[len(f'/{cmd_name} '):]

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get(
        f'https://raw.githubusercontent.com/DotOS/ota_config/dot-p/{device}.json'
    )
    if fetch.status_code == 200:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']
        changelog = response['changelog_device']

        reply_text = tld(chat.id, "download").format(filename, url)
        reply_text += tld(chat.id, "build_size").format(buildsize_b)
        reply_text += tld(chat.id, "version").format(version)
        reply_text += tld(chat.id, "changelog").format(changelog)

        keyboard = [[
            InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
        ]]
        message.reply_text(reply_text,
                           reply_markup=InlineKeyboardMarkup(keyboard),
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    elif fetch.status_code == 404:
        reply_text = tld(chat.id, "err_not_found")
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def viper(bot: Bot, update: Update):
    cmd_name = "viper"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    device = message.text[len(f'/{cmd_name} '):]

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    fetch = get(
        f'https://raw.githubusercontent.com/Viper-Devices/official_devices/master/{device}/build.json'
    )
    if fetch.status_code == 200:
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

    elif fetch.status_code == 404:
        reply_text = tld(chat.id, "btn_dl")
    message.reply_text(reply_text,
                       parse_mode=ParseMode.MARKDOWN,
                       disable_web_page_preview=True)


@run_async
def evo(bot: Bot, update: Update):
    cmd_name = "evo"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    device = message.text[len(f'/{cmd_name} '):]

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

    fetch = get(
        f'https://raw.githubusercontent.com/Evolution-X-Devices/official_devices/master/builds/{device}.json'
    )

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    if device == 'gsi':
        reply_text = tld(chat.id, "evox_gsi")

        keyboard = [[
            InlineKeyboardButton(
                text=tld(chat.id, "btn_dl"),
                url="https://sourceforge.net/projects/evolution-x/files/GSI/")
        ]]
        message.reply_text(reply_text,
                           reply_markup=InlineKeyboardMarkup(keyboard),
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
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


def enesrelease(bot: Bot, update: Update, args: List[str]):
    romname = "Enes Sastim's"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]

    usr = get(
        f'https://api.github.com/repos/EnesSastim/Downloads/releases/latest'
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


def phh(bot: Bot, update: Update, args: List[str]):
    romname = "Phh's"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]

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


def descendant(bot: Bot, update: Update, args: List[str]):
    romname = "Descendant GSI"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]

    usr = get(f'https://api.github.com/repos/Descendant/InOps/releases/latest'
              ).json()
    reply_text = tld(chat.id, "cust_releases").format(romname)
    for i in range(len(usr)):
        try:
            name = usr['assets'][i]['name']
            url = usr['assets'][i]['browser_download_url']
            download_count = usr['assets'][i]['download_count']
            reply_text += f"[{name}]({url}) - Downloaded `{download_count}` Times\n\n"
        except IndexError:
            continue
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN)


def miui(bot: Bot, update: Update):
    cmd_name = "miui"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    device = message.text[len(f'/{cmd_name} '):]

    giturl = "https://raw.githubusercontent.com/XiaomiFirmwareUpdater/miui-updates-tracker/master/"

    if device == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    result = tld(chat.id, "miui_release")
    stable_all = yaml.load(get(giturl +
                               "stable_recovery/stable_recovery.yml").content,
                           Loader=yaml.FullLoader)
    data = [i for i in stable_all if device == i['codename']]
    if len(data) != 0:
        for i in data:
            result += "[" + i['filename'] + "](" + i['download'] + ")\n\n"

        result += tld(chat.id, "weekly")
        weekly_all = yaml.load(
            get(giturl + "weekly_recovery/weekly_recovery.yml").content,
            Loader=yaml.FullLoader)
        data = [i for i in weekly_all if device == i['codename']]
        for i in data:
            result += "[" + i['filename'] + "](" + i['download'] + ")"
    else:
        result = tld(chat.id, "err_not_found")

    message.reply_text(result, parse_mode=ParseMode.MARKDOWN)


@run_async
def getaex(bot: Bot, update: Update, args: List[str]):
    cmd_name = "aex"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]

    AEX_OTA_API = "https://api.aospextended.com/ota/"

    if len(args) != 2:
        reply_text = tld(chat.id, "aex_cust_str")
        message.reply_text(reply_text,
                           parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    device = args[0]
    version = args[1]
    res = get(AEX_OTA_API + device + '/' + version.lower())
    if res.status_code == 200:
        apidata = json.loads(res.text)
        if apidata.get('error'):
            message.reply_text(tld(chat.id, "err_not_found"))
            return
        else:
            developer = apidata.get('developer')
            developer_url = apidata.get('developer_url')
            xda = apidata.get('forum_url')
            filename = apidata.get('filename')
            url = "https://downloads.aospextended.com/download/" + device + "/" + version + "/" + apidata.get(
                'filename')
            builddate = datetime.strptime(apidata.get('build_date'),
                                          "%Y%m%d-%H%M").strftime("%d %B %Y")
            buildsize = sizee(int(apidata.get('filesize')))

            reply_text = tld(chat.id, "download").format(filename, url)
            reply_text += tld(chat.id, "build_size").format(buildsize)
            reply_text += tld(chat.id, "build_date").format(builddate)
            reply_text += tld(
                chat.id,
                "maintainer").format(f"[{developer}]({developer_url})")

            keyboard = [[
                InlineKeyboardButton(text=tld(chat.id, "btn_dl"), url=f"{url}")
            ]]
            message.reply_text(reply_text,
                               reply_markup=InlineKeyboardMarkup(keyboard),
                               parse_mode=ParseMode.MARKDOWN,
                               disable_web_page_preview=True)
            return
    else:
        message.reply_text(tld(chat.id, "err_not_found"))


@run_async
def bootleggers(bot: Bot, update: Update):
    cmd_name = "bootleggers"
    message = update.effective_message
    chat = update.effective_chat  # type: Optional[Chat]
    codename = message.text[len(f'/{cmd_name} '):]

    if codename == '':
        reply_text = tld(chat.id, "cmd_example").format(cmd_name)
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


__help__ = True

TWRP_HANDLER = CommandHandler("twrp", twrp, pass_args=True)
GETFW_HANDLER = CommandHandler("getfw", getfw, pass_args=True)
CHECKFW_HANDLER = CommandHandler("checkfw", checkfw, pass_args=True)
MAGISK_HANDLER = CommandHandler("magisk", magisk)
GETAEX_HANDLER = DisableAbleCommandHandler("aex",
                                           getaex,
                                           pass_args=True,
                                           admin_ok=True)
MIUI_HANDLER = DisableAbleCommandHandler("miui", miui, admin_ok=True)
EVO_HANDLER = DisableAbleCommandHandler("evo", evo, admin_ok=True)
HAVOC_HANDLER = DisableAbleCommandHandler("havoc", havoc, admin_ok=True)
VIPER_HANDLER = DisableAbleCommandHandler("viper", viper, admin_ok=True)
DESCENDANT_HANDLER = DisableAbleCommandHandler("descendant",
                                               descendant,
                                               pass_args=True,
                                               admin_ok=True)
ENES_HANDLER = DisableAbleCommandHandler("enesrelease",
                                         enesrelease,
                                         pass_args=True,
                                         admin_ok=True)
PHH_HANDLER = DisableAbleCommandHandler("phh",
                                        phh,
                                        pass_args=True,
                                        admin_ok=True)
PEARL_HANDLER = DisableAbleCommandHandler("pearl", pearl, admin_ok=True)
POSP_HANDLER = DisableAbleCommandHandler("posp", posp, admin_ok=True)
DOTOS_HANDLER = DisableAbleCommandHandler("dotos", dotos, admin_ok=True)
PIXYS_HANDLER = DisableAbleCommandHandler("pixys", pixys, admin_ok=True)
LOS_HANDLER = DisableAbleCommandHandler("los", los, admin_ok=True)
BOOTLEGGERS_HANDLER = DisableAbleCommandHandler("bootleggers",
                                                bootleggers,
                                                admin_ok=True)

dispatcher.add_handler(GETAEX_HANDLER)
dispatcher.add_handler(MIUI_HANDLER)
dispatcher.add_handler(EVO_HANDLER)
dispatcher.add_handler(HAVOC_HANDLER)
dispatcher.add_handler(VIPER_HANDLER)
dispatcher.add_handler(DESCENDANT_HANDLER)
dispatcher.add_handler(ENES_HANDLER)
# dispatcher.add_handler(PHH_HANDLER)
dispatcher.add_handler(PEARL_HANDLER)
dispatcher.add_handler(POSP_HANDLER)
dispatcher.add_handler(DOTOS_HANDLER)
dispatcher.add_handler(PIXYS_HANDLER)
dispatcher.add_handler(LOS_HANDLER)
dispatcher.add_handler(BOOTLEGGERS_HANDLER)
dispatcher.add_handler(MAGISK_HANDLER)
dispatcher.add_handler(GETFW_HANDLER)
dispatcher.add_handler(CHECKFW_HANDLER)
dispatcher.add_handler(TWRP_HANDLER)
