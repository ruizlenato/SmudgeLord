import json
import re
import html
import time
from datetime import datetime
from typing import List

import yaml
from bs4 import BeautifulSoup
from hurry.filesize import size as sizee
from requests import get
from telegram import ParseMode, InlineKeyboardMarkup, InlineKeyboardButton
from telegram import Update, Bot
from telegram.ext import CommandHandler
from telegram.ext import run_async

from smudge import dispatcher, LOGGER
from smudge.modules.helper_funcs.misc import split_message

# DO NOT DELETE THIS, PLEASE.
# Originally made by @RealAkito on GitHub and Telegram
# This module was inspired by Android Helper Bot by Vachounet.
# None of the code is taken from the bot itself, to avoid any more confusion.
# Command /getfw /magisk /twrp and /device were obtained thanks to corsicanu bot (originally kanged from UserBot PaperplaneExtended)
# Command /specs was only possible thanks to the help of AvinashReddy3108

LOGGER.info("Original Android Modules by @RealAkito on Telegram, modified by @Renatoh on Telegram")

GITHUB = 'https://github.com'
DEVICES_DATA = 'https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/devices.json'


@run_async
def device(bot, update, args):
    if len(args) == 0:
        reply = f'No codename provided, write a codename for fetching informations.'
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    device = " ".join(args)
    found = [
        i for i in get(DEVICES_DATA).json()
        if i["device"] == device or i["model"] == device
    ]
    if found:
        reply = f'Search results for {device}:\n\n'
        for item in found:
            brand = item['brand']
            name = item['name']
            codename = item['device']
            model = item['model']
            reply += f'<b>{brand} {name}</b>\n' \
                     f'Model: <code>{model}</code>\n' \
                     f'Codename: <code>{codename}</code>\n\n'
    else:
        reply = f"Couldn't find info about {device}!\n"
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    update.message.reply_text("{}".format(reply),
                              parse_mode=ParseMode.HTML, disable_web_page_preview=True)


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
    reply += f' • [samfw.com]({url4}) ⭐\n\n'
    reply += f'You can also receive real-time firmwares from SamFrew on the @SamFirm channel\n'
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
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    device = " ".join(args)
    url = get(f'https://dl.twrp.me/{device}/')
    if url.status_code == 404:
        reply = f"Couldn't find twrp downloads for {device}!\n"
        update.effective_message.reply_text("{}".format(reply),
                    parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    reply = f'*Latest Official TWRP for {device}*\n'
    db = get(DEVICES_DATA).json()
    newdevice = device.strip('lte') if device.startswith('beyond') else device
    for dev in db:
        if (dev['device'] == newdevice) or (dev['model'] == newdevice):
            brand = dev['brand']
            name = dev['name']
            reply += f'*{brand} - {name}*\n'
            break
    page = BeautifulSoup(url.content, 'lxml')
    date = page.find("em").text.strip()
    reply += f'*Updated:* {date}\n'
    trs = page.find('table').find_all('tr')
    row = 2 if trs[0].find('a').text.endswith('tar') else 1
    for i in range(row):
        download = trs[i].find('a')
        dl_link = f"https://dl.twrp.me{download['href']}"
        dl_file = download.text
        size = trs[i].find("span", {"class": "filesize"}).text
        reply += f'[{dl_file}]({dl_link}) - {size}\n'

    update.message.reply_text("{}".format(reply),
                              parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def aex(bot: Bot, update: Update, args: List[str]):
    AEX_OTA_API = "https://api.aospextended.com/ota/"
    message = update.effective_message

    if len(args) != 2:
        reply_text = "Please type your device **codename** and **Android Version**!\nFor example, `/aex jason pie`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    device = args[0]
    version = args[1]
    res = get(AEX_OTA_API + device + '/' + version.lower())
    if res.status_code == 200:
        apidata = json.loads(res.text)
        if apidata.get('error'):
            message.reply_text("Sorry, but there isn't any build available for " + device)
            return
        else:
            developer = apidata.get('developer')
            developer_url = apidata.get('developer_url')
            forum_url = apidata.get('forum_url')
            filename = apidata.get('filename')
            url = "https://downloads.aospextended.com/download/" + device + "/" + version + "/" + apidata.get(
                'filename')
            builddate = datetime.strptime(apidata.get('build_date'), "%Y%m%d-%H%M").strftime("%d %B %Y")
            buildsize = sizee(int(apidata.get('filesize')))

            message = (f"*Download:* [{filename}]({url})\n"
                       f"*Build date:* `{builddate}`\n"
                       f"*Build size:* `{buildsize}`\n"
                       f"*XDA Thread:* [Here]({forum_url})\n"
                       f"*By:* [{developer}]({developer_url})\n")

            keyboard = [[InlineKeyboardButton(text="Click here to download", url=f"{url}")]]
            update.effective_message.reply_text(message, reply_markup=InlineKeyboardMarkup(keyboard),
                                                parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
            return
    else:
        message.reply_text("No builds found for the provided device-version combo.")


@run_async
def bootleggers(bot: Bot, update: Update):
    message = update.effective_message
    codename = message.text[len('/bootleggers '):]

    if codename == '':
        reply_text = "Please type your device **codename**!\nFor example, `/bootleggers tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
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
                peaksmod = {'fullname': 'Device name', 'buildate': 'Build date', 'buildsize': 'Build size',
                            'downloadfolder': 'SourceForge folder', 'mirrorlink': 'Mirror link',
                            'xdathread': 'XDA thread'}
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

            reply_text += f"\n*XDA Thread:* [Here]({devices[devicetoget]['xdathread']})"
            reply_text += f"\n*Download:* [{devices[devicetoget]['filename']}]({devices[devicetoget]['download']})"
            reply_text = reply_text.strip("\n")
        else:
            reply_text = 'Device not found.'

    elif fetch.status_code == 404:
        reply_text = "Couldn't reach Bootleggers API."
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def dotos(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/dotos '):]

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/dotos tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    fetch = get(f'https://raw.githubusercontent.com/DotOS/ota_config/dot-p/{device}.json')
    if fetch.status_code == 200:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']
        changelog = response['changelog_device']

        reply_text = (f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`\n"
                      f"*Device Changelog:* `{changelog}`")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    elif fetch.status_code == 404:
        reply_text = "Device not found"
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def evo(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/evo '):]

    if device == "x00t" or device == "x01bd":
        device = device.upper()

    fetch = get(f'https://raw.githubusercontent.com/Evolution-X-Devices/official_devices/master/builds/{device}.json')

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/evo tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    if device == 'gsi':
        reply_text = "Please check Vega GSIs channel (@VegaGSIs) for unofficial but updated GSIs" \
                     " or click the button down to download the official GSIs!"

        keyboard = [[InlineKeyboardButton(text="Click to Download",
                                          url="https://sourceforge.net/projects/evolution-x/files/GSI/")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
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

            reply_text = (f"*Download:* [{filename}]({url})\n"
                          f"*Build Size:* `{size_b}`\n"
                          f"*Android Version:* `{version}`\n"
                          f"*Maintainer:* [{maintainer}](https://t.me/{maintainer_url})\n")

            keyboard = [[InlineKeyboardButton(text="⬇️ Download ⬇️", url=f"{url}")]]
            keyboard += [[InlineKeyboardButton(text="📃 Changelog 📃", url=f"https://raw.githubusercontent.com/Evolution-X-Devices/official_devices/master/changelogs/{device}/{filename}.txt")]]
            message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                               disable_web_page_preview=True)
            return

        except ValueError:
            reply_text = "Tell the rom maintainer to fix their OTA json. I'm sure this won't work with OTA and it won't work with this bot too :P"
            message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
            return

    elif fetch.status_code == 404:
        reply_text = "Device not found!"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return


@run_async
def havoc(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/havoc '):]
    fetch = get(f'https://raw.githubusercontent.com/Havoc-Devices/android_vendor_OTA/pie/{device}.json')

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/havoc tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    if fetch.status_code == 200:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']

        reply_text = (f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    elif fetch.status_code == 404:
        reply_text = "Device not found."
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def los(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/los '):]

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/los tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
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

        reply_text = (f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    else:
        reply_text = "Device not found"
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


def miui(bot: Bot, update: Update):
    giturl = "https://raw.githubusercontent.com/XiaomiFirmwareUpdater/miui-updates-tracker/master/"
    message = update.effective_message
    device = message.text[len('/miui '):]

    if device == '':
        reply_text = "Please type your device <b>codename</b>!\nFor example, <code>/miui whyred</code>!"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    result = "<b>Recovery ROM</b>\n\n"
    result += "<b>Stable</b>\n"
    stable_all = yaml.load(get(giturl + "stable_recovery/stable_recovery.yml").content, Loader=yaml.FullLoader)
    data = [i for i in stable_all if device == i['codename']]
    if len(data) != 0:
        for i in data:
            result += "<b>Device:</b> " + i['device'] + "\n"
            result += f'<a href="{i["download"]}">{i["filename"]}</a>\n'
            result += "<b>Size:</b> " + i ['size'] + "\n"
            result += "<b>Version:</b> " + i ['version'] + "\n"
            result += "<b>Android:</b> " + i ['android'] + "\n\n"

        result += "<b>Weekly</b>\n"
        weekly_all = yaml.load(get(giturl + "weekly_recovery/weekly_recovery.yml").content, Loader=yaml.FullLoader)
        data = [i for i in weekly_all if device == i['codename']]
        for i in data:
            result += "<b>Device:</b> " + i ['device'] + "\n"
            result += f'<a href="{i["download"]}">{i["filename"]}</a>\n'
            result += "<b>Size:</b> " + i ['size'] + "\n"
            result += "<b>Version:</b> " + i ['version'] + "\n"
            result += "<b>Android:</b> " + i ['android'] + "\n\n"
    else:
        result = "Couldn't find any device matching your query."

    message.reply_html(result)


@run_async
def pe(bot: Bot, update: Update):
    message = update.effective_message
    cmd = message.text.split()[0]
    device = message.text[len(cmd)+1:]

    if device == '':
        reply_text = f"Please type your device **codename**!\nFor example, `{cmd} tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    if cmd.startswith("/peplus"):
        variant = "pie_plus"
    elif cmd.startswith("/pe10"):
        variant = "ten"
    else:
        variant = "pie"

    fetch = get(f'https://download.pixelexperience.org/ota_v3/{device}/{variant}')
    if not fetch.json()['error']:
        usr = fetch.json()
        filename = usr['filename']
        url = usr['url']
        buildsize_a = usr['size']
        buildsize_b = sizee(int(buildsize_a))
        version = usr['version']
        maintainerurl = usr['maintainer_url']
        maintainer = usr['maintainer']

        reply_text = (f"*PixelExperience build for {device}*\n"
                      f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`\n"
                      f"*Maintainer:* [{maintainer}]({maintainerurl})")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=url)]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    else:
        reply_text = "Device not found"
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def pearl(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/pearl '):]

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/pearl mido`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    fetch = get(f'https://raw.githubusercontent.com/PearlOS/OTA/master/{device}.json')
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
            reply_text = (f"*Download:* [{filename}]({url})\n"
                          f"*Build size:* `{buildsize_b}`\n"
                          f"*Version:* `{version}`\n"
                          f"*Maintainer:* `{maintainer}`\n"
                          f"*ROM Type:* `{romtype}`")

            keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
            message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                               disable_web_page_preview=True)
            return

        reply_text = (f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`\n"
                      f"*Maintainer:* `{maintainer}`\n"
                      f"*ROM Type:* `{romtype}`\n"
                      f"*XDA Thread:* [Link]({xda})")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    elif fetch.status_code == 404:
        reply_text = "Device not found."
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def pixys(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/pixys '):]

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/pixys tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    fetch = get(f'https://raw.githubusercontent.com/PixysOS-Devices/official_devices/master/{device}/build.json')
    if fetch.status_code == 200:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        romtype = response['romtype']
        version = response['version']

        reply_text = (f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`\n"
                      f"*Rom Type:* `{romtype}`")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    elif fetch.status_code == 404:
        reply_text = "Device not found."
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def posp(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/posp '):]

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/posp tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    fetch = get(f'https://api.potatoproject.co/checkUpdate?device={device}&type=weekly')
    if fetch.status_code == 200 and len(fetch.json()['response']) != 0:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']

        reply_text = (f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)
        return

    else:
        reply_text = "Device not found"
    message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)


@run_async
def viper(bot: Bot, update: Update):
    message = update.effective_message
    device = message.text[len('/viper '):]

    if device == '':
        reply_text = "Please type your device **codename**!\nFor example, `/viper tissot`"
        message.reply_text(reply_text, parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return

    fetch = get(f'https://raw.githubusercontent.com/Viper-Devices/official_devices/master/{device}/build.json')
    if fetch.status_code == 200:
        usr = fetch.json()
        response = usr['response'][0]
        filename = response['filename']
        url = response['url']
        buildsize_a = response['size']
        buildsize_b = sizee(int(buildsize_a))
        version = response['version']

        reply_text = (f"*Download:* [{filename}]({url})\n"
                      f"*Build size:* `{buildsize_b}`\n"
                      f"*Version:* `{version}`")

        keyboard = [[InlineKeyboardButton(text="Click to Download", url=f"{url}")]]
        message.reply_text(reply_text, reply_markup=InlineKeyboardMarkup(keyboard), parse_mode=ParseMode.MARKDOWN,
                           disable_web_page_preview=True)

    elif fetch.status_code == 404:
        message.reply_text("Device not found")


@run_async
def specs(bot, update, args):
    if len(args) == 0:
        update.effective_message.reply_html("Please type your device <b>brand</b> and <b>name</b>!\
        \nFor example, <code>/specs Xiaomi Redmi Note 7</code>")
        return
    brand = args[0].lower()
    device = " ".join(args[1:]).lower()
    if brand and device:
        pass
    all_brands = BeautifulSoup(
        get('https://www.devicespecifications.com/en/brand-more').content,
        'lxml').find('div', {
            'class': 'brand-listing-container-news'
        }).findAll('a')
    try:
        brand_page_url = [
            i['href'] for i in all_brands if brand == i.text.strip().lower()
        ][0]
    except IndexError:
        update.effective_message.reply_html(f'<code>{brand}</code> is unknown brand!')
        return
    devices = BeautifulSoup(get(brand_page_url).content, 'lxml') \
        .findAll('div', {'class': 'model-listing-container-80'})
    device_page_url = [
        i.a['href']
        for i in BeautifulSoup(str(devices), 'lxml').findAll('h3')
        if device in i.text.strip().lower()
    ]
    if not device_page_url:
        update.effective_message.reply_html(f"Can't find <code>{device}</code>!")
        return
    if len(device_page_url) > 2:
        device_page_url = device_page_url[:2]
    reply = ''
    for url in device_page_url:
        info = BeautifulSoup(get(url).content, 'lxml')
        reply = '\n<b>' + info.title.text.split('-')[0].strip() + '</b>\n\n'
        info = info.find('div', {'id': 'model-brief-specifications'})
        specifications = re.findall(r'<b>.*?<br/>', str(info))
        for item in specifications:
            title = re.findall(r'<b>(.*?)</b>', item)[0].strip()
            data = re.findall(r'</b>: (.*?)<br/>', item)[0].strip()
            reply += f'<b>{title}</b>: {data}\n'
    update.effective_message.reply_html(reply)


__help__ = """
*Here you will have several useful commands for Android users!*

*Useful tools:*
 - /device <codename>: gets android device basic info from its codename
 - /magisk: gets the latest magisk release for Stable/Beta/Canary
 - /twrp <codename>: gets latest twrp for the android device using the codename
 - /specs <brand> <device name>: will give you the complete specifications of a device

*Specific ROM for a device*
 - /aex <device> <android version>: Get the latest AEX ROM for a device
 - /bootleggers <device>: Get the latest Bootleggers ROM for a device
 - /dotos <device>: Get the latest DotOS ROM for a device
 - /evo <device>: Get the latest Evolution X ROM for a device
 - /havoc <device>: Get the latest Havoc ROM for a device
 - /los <device>: Get the latest LineageOS ROM for a device
 - /pe <device>: Get the latest PixelExperience ROM for a device
 - /pe10 <device>: Get the latest PixelExperience 10 ROM for a device
 - /peplus <device>: Get the latest PixelExperience Plus ROM for a device
 - /pearl <device>: Get the latest Pearl ROM for a device
 - /pixys <device>: Get the latest Pixys ROM for a device
 - /posp <device>: Get the latest POSP ROM for a device
 - /viper <device>: Get the latest Viper ROM for a device

*Firmwares:*
 - /getfw <model> <csc>: (SAMSUNG ONLY) gets firmware download links from samfrew, sammobile and sfirmwares for the given device.
 - /checkfw <model> <csc>: (SAMSUNG ONLY) shows the latest firmware info for the given device, taken from samsung server.
 - /miui <device>: Get the latest MIUI ROM for a device.
"""

__mod_name__ = "Android"

DEVICE_HANDLER = DisableAbleCommandHandler("device", device, pass_args=False)
MAGISK_HANDLER = DisableAbleCommandHandler("magisk", magisk)
TWRP_HANDLER = DisableAbleCommandHandler("twrp", twrp, pass_args=False)
AEX_HANDLER = DisableAbleCommandHandler("aex", aex, pass_args=True, admin_ok=False)
BOOTLEGGERS_HANDLER = DisableAbleCommandHandler("bootleggers", bootleggers, admin_ok=False)
DOTOS_HANDLER = DisableAbleCommandHandler("dotos", dotos, admin_ok=False)
EVO_HANDLER = DisableAbleCommandHandler("evo", evo, admin_ok=False)
HAVOC_HANDLER = DisableAbleCommandHandler("havoc", havoc, admin_ok=False)
LOS_HANDLER = DisableAbleCommandHandler("los", los, admin_ok=False)
MIUI_HANDLER = DisableAbleCommandHandler("miui", miui, admin_ok=False)
PE_HANDLER = DisableAbleCommandHandler("pe", pe, admin_ok=False)
PE10_HANDLER = DisableAbleCommandHandler("pe10", pe, admin_ok=False)
PEPLUS_HANDLER = DisableAbleCommandHandler("peplus", pe, admin_ok=False)
PEARL_HANDLER = DisableAbleCommandHandler("pearl", pearl, admin_ok=False)
PIXYS_HANDLER = DisableAbleCommandHandler("pixys", pixys, admin_ok=False)
POSP_HANDLER = DisableAbleCommandHandler("posp", posp, admin_ok=False)
VIPER_HANDLER = DisableAbleCommandHandler("viper", viper, admin_ok=False)
SPECS_HANDLER = DisableAbleCommandHandler("specs", specs, pass_args=False)
GETFW_HANDLER = DisableAbleCommandHandler("getfw", getfw, pass_args=False)
CHECKFW_HANDLER = DisableAbleCommandHandler("checkfw", checkfw, pass_args=False)

dispatcher.add_handler(DEVICE_HANDLER)
dispatcher.add_handler(MAGISK_HANDLER)
dispatcher.add_handler(TWRP_HANDLER)
dispatcher.add_handler(AEX_HANDLER)
dispatcher.add_handler(BOOTLEGGERS_HANDLER)
dispatcher.add_handler(DOTOS_HANDLER)
dispatcher.add_handler(EVO_HANDLER)
dispatcher.add_handler(HAVOC_HANDLER)
dispatcher.add_handler(LOS_HANDLER)
dispatcher.add_handler(MIUI_HANDLER)
dispatcher.add_handler(PE_HANDLER)
dispatcher.add_handler(PE10_HANDLER)
dispatcher.add_handler(PEPLUS_HANDLER)
dispatcher.add_handler(PEARL_HANDLER)
dispatcher.add_handler(PIXYS_HANDLER)
dispatcher.add_handler(POSP_HANDLER)
dispatcher.add_handler(VIPER_HANDLER)
dispatcher.add_handler(SPECS_HANDLER)
dispatcher.add_handler(GETFW_HANDLER)
dispatcher.add_handler(CHECKFW_HANDLER)
