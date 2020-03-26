import html
import json
import time
import yaml
from datetime import datetime
from typing import Optional, List
from hurry.filesize import size as sizee

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

LOGGER.info("android: Original Android Modules by @RealAkito on Telegram")


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
