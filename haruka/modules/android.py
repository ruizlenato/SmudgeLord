import time
import yaml
from typing import Optional, List
from bs4 import BeautifulSoup


from telegram import Chat, Update, Bot
from telegram import ParseMode, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.ext import CommandHandler, run_async

from haruka import dispatcher, LOGGER
from haruka.modules.disable import DisableAbleCommandHandler
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
def device(bot, update, args):
    if len(args) == 0:
        reply = '🇺🇸No codename provided, write a codename for fetching informations.\n\n🇧🇷Nenhum codinome fornecido, escreva um codinome para obter informações'
        update.effective_message.reply_text("{}".format(reply),
                                            parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    device = " ".join(args)
    db = get(DEVICES_DATA).json()
    newdevice = device.strip('lte') if device.startswith('beyond') else device
    try:
        reply = f'\n\n'
        brand = db[newdevice][0]['brand']
        name = db[newdevice][0]['name']
        model = db[newdevice][0]['model']
        codename = newdevice
        reply += f'<b>{brand} {name}</b>\n' \
                 f'Model: <code>{model}</code>\n' \
                 f'Codename: <code>{codename}</code>\n\n'
    except KeyError:
        reply = f"Couldn't find info about {device}!\n"
        update.effective_message.reply_text("{}".format(reply),
                                            parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
        return
    update.message.reply_text("{}".format(reply),
                              parse_mode=ParseMode.HTML, disable_web_page_preview=True)

@run_async
def odin(bot, update, args):
    message = "*🇺🇸Tool to flash the stock firmware of your Samsung Galaxy*\n\n🇧🇷Ferramenta para atualizar o firmware padrão do seu Samsung Galaxy!"
    keyboard = [
        [InlineKeyboardButton("Odin", url="https://odin3download.com/tool/Odin3-v3.14.1.zip"),
         InlineKeyboardButton("USB Drivers", url="https://developer.samsung.com/mobile/android-usb-driver.html")]
    ]
    reply_markup = InlineKeyboardMarkup(keyboard)
    update.effective_message.bot.send_message(chat_id=update.message.chat_id, text=message,
                                              reply_to_message_id=update.message.message_id,
                                              reply_markup=reply_markup, parse_mode=ParseMode.MARKDOWN)


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
    for type, path in {"Stable": "master/stable", "Beta": "master/beta", "Canary": "canary/debug"}.items():
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
        reply = f'\n'            
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

__help__ = True

TWRP_HANDLER = CommandHandler("twrp", twrp, pass_args=True)
GETFW_HANDLER = CommandHandler("getfw", getfw, pass_args=True)
CHECKFW_HANDLER = CommandHandler("checkfw", checkfw, pass_args=True)
MAGISK_HANDLER = CommandHandler("magisk", magisk)
MIUI_HANDLER = DisableAbleCommandHandler("miui", miui, admin_ok=True)
DESCENDANT_HANDLER = DisableAbleCommandHandler("descendant",
                                               descendant,
                                               pass_args=True,
                                               admin_ok=True)
ODIN_HANDLER = CommandHandler("odin", odin, pass_args=True)
DEVICE_HANDLER = CommandHandler("device", device, pass_args=True)
PHH_HANDLER = DisableAbleCommandHandler("phh",
                                        phh,
                                        pass_args=True,
                                        admin_ok=True)

dispatcher.add_handler(MIUI_HANDLER)
dispatcher.add_handler(DESCENDANT_HANDLER)
# dispatcher.add_handler(PHH_HANDLER)
dispatcher.add_handler(MAGISK_HANDLER)
dispatcher.add_handler(GETFW_HANDLER)
dispatcher.add_handler(CHECKFW_HANDLER)
dispatcher.add_handler(TWRP_HANDLER)
dispatcher.add_handler(DEVICE_HANDLER)
dispatcher.add_handler(ODIN_HANDLER)
