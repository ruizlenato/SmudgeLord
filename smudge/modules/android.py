from smudge import pbot
from ujson import loads
from requests import get
from bs4 import BeautifulSoup
from pyrogram import Client, filters
from smudge.modules.translations.strings import tld
from pyrogram.types import Update, InlineKeyboardButton, InlineKeyboardMarkup

fw_links = {"SAMMOBILE":"https://www.sammobile.com/samsung/firmware/{}/{}/",
            "SAMFW":"https://samfw.com/firmware/{}/{}/",
            "SAMFREW":"https://samfrew.com/model/{}/region/{}/",
           }.items()

class GetDevice:
    def __init__(self, device):
        """Get device info by codename or model!"""
        self.device = device

    def get(self):
        if self.device.lower().startswith('sm-'):
            data = get(
                'https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_model.json').content
            db = loads(data)
            try:
                name = db[self.device.upper()][0]['name']
                device = db[self.device.upper()][0]['device']
                brand = db[self.device.upper()][0]['brand']
                model = self.device.lower()
                return {'name': name,
                        'device': device,
                        'model': model,
                        'brand': brand
                        }
            except KeyError:
                return False
        else:
            data = get(
                'https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json').content
            db = loads(data)
            newdevice = self.device.strip('lte').lower() if self.device.startswith(
                'beyond') else self.device.lower()
            try:
                name = db[newdevice][0]['name']
                model = db[newdevice][0]['model']
                brand = db[newdevice][0]['brand']
                device = self.device.lower()
                return {'name': name,
                        'device': device,
                        'model': model,
                        'brand': brand
                        }
            except KeyError:
                return False

async def git(c: Client, update: Update, repo, page):
    db = loads(page.content)
    name = db['name']
    date = db['published_at']
    tag = db['tag_name']
    date = db['published_at']
    changelog = db['body']
    dev, repo = repo.split('/')
    message = "<b>Name:</b> <code>{}</code>\n".format(name)
    message += "<b>Tag:</b> <code>{}</code>\n".format(tag)
    message += "<b>Released on:</b> <code>{}</code>\n".format(date[:date.rfind("T")])
    message += "<b>By:</b> <code>{}@github.com</code>\n".format(dev)
    message += "<b>Changelog:</b>\n<code>{}</code>\n\n".format(changelog)
    keyboard = []
    for i in range(len(db)):
        try:
            file_name = db['assets'][i]['name']
            url = db['assets'][i]['browser_download_url']
            dls = db['assets'][i]['download_count']
            size_bytes = db['assets'][i]['size']
            size = float("{:.2f}".format((size_bytes/1024)/1024))
            text = "{}\n💾 {}MB | 📥 {}".format(file_name, size, dls)
            keyboard += [[InlineKeyboardButton(text=text, url=url)]]
        except IndexError:
            continue
    await c.send_message(
                chat_id=update.chat.id,
                text=message,
                reply_markup=InlineKeyboardMarkup(keyboard),
                disable_web_page_preview=True
            )
@pbot.on_message(filters.command(["magisk", "root"]))
async def magisk(c: Client, update: Update):
    url = 'https://raw.githubusercontent.com/topjohnwu/magisk_files/'
    chat_id=update.chat.id
    message = tld(chat_id, "magisk_releases")
    for magisk_type, path  in {"Stable":"master/stable", "Beta":"master/beta", "Canary":"canary/canary"}.items():
        canary = "https://github.com/topjohnwu/magisk_files/raw/canary/" if magisk_type == "Canary" else ""
        data = get(url + path + '.json').json()
        message += f'<b>• {magisk_type}</b>:\n<a href="{canary + data["magisk"]["link"]}">Magisk - V{data["magisk"]["version"]}</a> |' \
                    f'<a href="{canary + data["app"]["link"]}"> App - v{data["app"]["version"]}</a> |' \
                    f'<a href="{canary + data["uninstaller"]["link"]}"> Uninstaller</a> \n'
    await c.send_message(
                chat_id=update.chat.id,
                text=message,
                disable_web_page_preview=True
            )

@pbot.on_message(filters.command("twrp"))
async def twrp(c: Client, update: Update):
    if not len(update.command) == 2:
        m = "Type the device codename, example: <code>/twrp whyred</code>"
        await c.send_message(
            chat_id=update.chat.id,
            text=m)
        return

    device = update.command[1]
    url = get(f'https://dl.twrp.me/{device}/')
    if url.status_code == 404:
        m = "TWRP is not available for <code>{device}</code>"
        await c.send_message(
            chat_id=update.chat.id,
            text=m)
        return

    else:
        chat_id=update.chat.id
        m = f'<b>Latest TWRP for {device}</b>\n'
        page = BeautifulSoup(url.content, 'lxml')
        date = page.find("em").text.strip()
        m += tld(chat_id, "recovery_release_date").format(date)
        trs = page.find('table').find_all('tr')
        row = 2 if trs[0].find('a').text.endswith('tar') else 1

        for i in range(row):
            download = trs[i].find('a')
            dl_link = f"https://dl.twrp.me{download['href']}"
            dl_file = download.text
            size = trs[i].find("span", {"class": "filesize"}).text
        m += tld(chat_id, "recovery_release_size").format(size)
        m += f'📦 <b>File:</b> <code>{dl_file.lower()}</code>'
        keyboard = [[InlineKeyboardButton(
            text="Download", url=dl_link)]]
        await c.send_message(
            chat_id=update.chat.id,
            text=m,
            reply_markup=InlineKeyboardMarkup(keyboard))

@pbot.on_message(filters.command(["ofox", "ofx", "orangefox", "fox", "ofx_recovery"]))
async def ofox(c: Client, update: Update):
    chat_id=update.chat.id
    if not len(update.command) == 2:
        message = tld(chat_id, "fox_get_release")
        await c.send_message(
                chat_id=update.chat.id,
                text=message,
                disable_web_page_preview=True
            )
        return
    device = update.command[1]
    data = GetDevice(device).get()
    if data:
        name = data['name']
        device = data['device']
        brand = data['brand']
    else:
        message = tld(chat_id, "fox_device_not_found")
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    page = get(f'https://api.orangefox.download/v2/device/{device}/releases/stable/last')
    if page.status_code == 404:
        message = f"OrangeFox currently is not avaliable for <code>{device}</code>"
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    else:
        message = tld(chat_id, "fox_release_title").format(device)
        message += f'<b>📱Device:</b> {brand.upper()} {name.upper()}\n'
        page = loads(page.content)
        version = page['version']
        size = page['size_human']
        dl_link = page['url']
        date = page['date']
        md5 = page['md5']
        message += tld(chat_id, "fox_stable")
        message += tld(chat_id, "fox_release_version").format(version)
        message += tld(chat_id, "recovery_release_size").format(size)
        message += tld(chat_id, "recovery_release_date").format(date)
        message += tld(chat_id, "reovery_release_md5").format(md5)
        keyboard = [[InlineKeyboardButton(text="Download", url=dl_link)]]
        await c.send_message(
                chat_id=update.chat.id,
                text=message,
                reply_markup=InlineKeyboardMarkup(keyboard))

@pbot.on_message(filters.command(["ofoxbeta", "ofxbeta", "orangefoxbeta", "foxbeta", "ofx_recoverybeta"]))
async def ofox(c: Client, update: Update):
    chat_id=update.chat.id
    if not len(update.command) == 2:
        message = tld(chat_id, "fox_get_release")
        await c.send_message(
                chat_id=update.chat.id,
                text=message,
                disable_web_page_preview=True
            )
        return
    device = update.command[1]
    data = GetDevice(device).get()
    if data:
        name = data['name']
        device = data['device']
        brand = data['brand']
    else:
        message = tld(chat_id, "fox_device_not_found")
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    page = get(f'https://api.orangefox.download/v2/device/{device}/releases/beta/last')
    if page.status_code == 404:
        message = f"OrangeFox currently is not avaliable for <code>{device}</code>"
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    else:
        message = tld(chat_id, "fox_release_title").format(device)
        message += f'<b>📱Device:</b> {brand.upper()} {name.upper()}\n'
        page = loads(page.content)
        version = page['version']
        size = page['size_human']
        dl_link = page['url']
        date = page['date']
        md5 = page['md5']
        message += tld(chat_id, "fox_beta")
        message += tld(chat_id, "fox_release_version").format(version)
        message += tld(chat_id, "recovery_release_size").format(size)
        message += tld(chat_id, "recovery_release_date").format(date)
        message += tld(chat_id, "reovery_release_md5").format(md5)
        keyboard = [[InlineKeyboardButton(text="Download", url=dl_link)]]
        await c.send_message(
                chat_id=update.chat.id,
                text=message,
                reply_markup=InlineKeyboardMarkup(keyboard))

@pbot.on_message(filters.command(["phh", "gsi"]))
async def phh(c: Client, update: Update):
    repo = "phhusson/treble_experimentations"
    page = get(f'https://api.github.com/repos/{repo}/releases/latest')
    if not page.status_code == 200:
        return
    await git(c, update , repo, page)

@pbot.on_message(filters.command(["checkfw", "check", "fw", "getfw"]))
async def check(c: Client, update: Update):
    chat_id=update.chat.id
    if not len(update.command) == 3:
        message = "Please type your device <b>MODEL</b> and <b>CSC</b> into it!\ni.e <code>/fw SM-G975F XSG!</code>"
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    cmd,temp,csc = update.command
    model = 'sm-'+temp if not temp.upper().startswith('SM-') else temp
    fota = get(f'http://fota-cloud-dn.ospserver.net/firmware/{csc.upper()}/{model.upper()}/version.xml')
    test = get(f'http://fota-cloud-dn.ospserver.net/firmware/{csc.upper()}/{model.upper()}/version.test.xml')
    if test.status_code != 200:
        message = f"Couldn't find any firmwares for {temp.upper()} - {csc.upper()}, please refine your search or try again later!"
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    page1 = BeautifulSoup(fota.content, 'lxml')
    page2 = BeautifulSoup(test.content, 'lxml')
    os1 = page1.find("latest").get("o")
    os2 = page2.find("latest").get("o")
    if page1.find("latest").text.strip():
        pda1,csc1,phone1=page1.find("latest").text.strip().split('/')
        message = tld(chat_id, "samsung_releases_details").format(model.upper(), csc.upper())
        message += tld(chat_id, "samsung_releases")
        message += f'-PDA: <code>{pda1}</code>\n-CSC: <code>{csc1}</code>\n'
        if phone1:
            message += f'-Phone: <code>{phone1}</code>\n'
        if os1:
            message += f'-Android: <code>{os1}</code>\n'
        message += '\n'
    else:
        message = f'<b>No public release found for {model.upper()} and {csc.upper()}.</b>\n\n'
    message += tld(chat_id, "samsung_test_firmware")
    if len(page2.find("latest").text.strip().split('/')) == 3:
        pda2,csc2,phone2=page2.find("latest").text.strip().split('/')
        message += f'-PDA: <code>{pda2}</code>\n-CSC: <code>{csc2}</code>\n'
        if phone2:
            message += f'• Phone: <code>{phone2}</code>\n'
        if os2:
            message += f'• Android: <code>{os2}</code>\n'
    else:
        md5=page2.find("latest").text.strip()
        message += f'\n-Hash: <code>{md5}</code>\n-Android: <code>{os2}</code>\n\n'
    cmd.split()
    if cmd in ("checkfw", "check"):
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
    elif cmd in ("getfw", "fw"):
        keyboard = []
        message += tld(chat_id, "download_below")
        for site_name, fw_link  in fw_links:
            keyboard += [[InlineKeyboardButton(site_name, url=fw_link.format(model.upper(), csc.upper()))]]
        await c.send_message(
                chat_id=update.chat.id,
                text=message,
                reply_markup=InlineKeyboardMarkup(keyboard))
            
@pbot.on_message(filters.command(["whatis", "device", "codename"]))
async def models(c: Client, update: Update):
    chat_id=update.chat.id
    if not len(update.command) == 2:
        message = tld(chat_id, "cmd_example").format("device")
        await c.send_message(
                chat_id=update.chat.id,
                text=message,
                disable_web_page_preview=True
            )
        return
    device = update.command[1]
    data = GetDevice(device).get()
    if data:
        name = data['name']
        device = data['device']
        brand = data['brand']
        model = data['model']
    else:
        message = tld(chat_id, "err_not_found")
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    message = tld(chat_id, "device_result").format(brand, name, device, model.upper())
    await c.send_message(
                chat_id=update.chat.id,
                text=message,
                disable_web_page_preview=True
            )

@pbot.on_message(filters.command(["variants", "models"]))
async def variants(c: Client, update: Update):
    chat_id=update.chat.id
    if not len(update.command) == 2:
        message = tld(chat_id, "cmd_example").format("variants")
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    device = update.command[1]
    data = GetDevice(device).get()
    if data:
        name = data['name']
        device = data['device']
    else:
        message = tld(chat_id, "err_not_found")
        await c.send_message(
                chat_id=update.chat.id,
                text=message)
        return
    data = get('https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json').content
    db = loads(data)
    device=db[device]
    message = f'<b>{name}</b> variants:\n\n'
    for i in device:
        name =  i['name']
        model = i['model']
        message += tld(chat_id, "results_device4").format(name, model)

    await c.send_message(
        chat_id=update.chat.id,
        text=message)
