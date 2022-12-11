# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@proton.me)
import re
import json
from bs4 import BeautifulSoup

from pyrogram import filters
from pyrogram.helpers import ikb
from pyrogram.types import Message

from ..bot import Smudge
from ..utils import http
from ..utils.locales import tld

# Port from SamsungGeeksBot.
DEVICE_DATA: str = "https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_model.json"
DEVICE_DATA_NAME: str = "https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_name.json"
DEVICE_DATA_MODEL: str = "https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json"


class GetDevice:
    def __init__(self, device):
        """Get device info by codename or model!"""
        self.device = device

    async def get(self):
        try:
            db = json.loads((await http.get(DEVICE_DATA_NAME)).content)
            name = self.device.lower()
            device = db[self.device][0][
                "device"
            ]  # To-Do: Add support for case-insensitive search.
            brand = db[self.device][0]["brand"]
            model = db[self.device][0]["model"]
            return {"name": name, "device": device, "model": model, "brand": brand}
        except KeyError:
            if re.search(r"^(ASUS.*|sm-.*)", self.device):
                argdevice = (
                    self.device.upper()
                    if re.search(r"(sm-.*)", self.device)
                    else self.device
                )
                db = json.loads((await http.get(DEVICE_DATA)).content)
                try:
                    name = db[argdevice][0]["name"]
                    device = db[argdevice][0]["device"]
                    brand = db[argdevice][0]["brand"]
                    model = self.device.lower()
                    return {
                        "name": name,
                        "device": device,
                        "model": model,
                        "brand": brand,
                    }
                except KeyError:
                    return False
            else:
                database = json.loads((await http.get(DEVICE_DATA_MODEL)).content)
                newdevice = (
                    self.device.strip("lte").lower()
                    if self.device.startswith("beyond")
                    else self.device.lower()
                )
                try:
                    name = database[newdevice][0]["name"]
                    model = database[newdevice][0]["model"]
                    brand = database[newdevice][0]["brand"]
                    device = newdevice
                    return {
                        "name": name,
                        "device": device,
                        "model": model,
                        "brand": brand,
                    }
                except KeyError:
                    return False


@Smudge.on_message(filters.command(["magisk"]))
async def magisk(c: Smudge, m: Message):
    repo_url = "https://raw.githubusercontent.com/topjohnwu/magisk-files/master/"
    text = await tld(m, "Android.magisk_releases")
    for magisk_type in ["stable", "beta", "canary"]:
        data = json.loads((await http.get(repo_url + magisk_type + ".json")).content)
        text += (
            f"<b>{magisk_type.capitalize()}</b>:\n"
            f'<a href="{data["magisk"]["link"]}" >Magisk - V{data["magisk"]["version"]}</a>'
            f' | <a href="{data["magisk"]["note"]}" >Changelog</a> \n'
        )
    await m.reply_text(text, disable_web_page_preview=True)


@Smudge.on_message(filters.command(["twrp"]))
async def twrp(c: Smudge, m: Message):
    if len(m.command) != 2:
        message = "Please write your codename into it, i.e <code>/twrp herolte</code>"
        await m.reply_text(message)
        return
    device = m.command[1]
    url = await http.get(f"https://eu.dl.twrp.me/{device}/")
    if url.status_code == 404:
        await m.reply_text((await tld(m, "Android.twrp_404")).format(device))
    else:
        message = (await tld(m, "Android.twrp_found")).format(device)
        page = BeautifulSoup(url.content, "html.parser")
        date = page.find("em").text.strip()
        message += (await tld(m, "Android.twrp_date")).format(date)
        trs = page.find("table").find_all("tr")
        row = 2 if trs[0].find("a").text.endswith("tar") else 1
        keyboard = []
        for i in range(row):
            download = trs[i].find("a")
            dl_link = f"https://eu.dl.twrp.me{download['href']}"
            dl_file = download.text
            keyboard += [[(dl_file.upper(), dl_link, "url")]]
        size = trs[i].find("span", {"class": "filesize"}).text
        message += (await tld(m, "Android.twrp_size")).format(size)
        await m.reply_text(
            message,
            reply_markup=ikb(keyboard),
        )


@Smudge.on_message(
    filters.command(["variants", "models", "whatis", "device", "codename"])
)
async def variants(c: Smudge, m: Message):
    if m.reply_to_message and m.reply_to_message.text:
        cdevice = m.reply_to_message.text
    elif len(m.command) > 1:
        cdevice = m.text.split(None, 1)[1]
    else:
        await m.reply_text(
            (await tld(m, "Android.models_nocodename")).format(m.text.split(None, 1)[0])
        )
        return

    data = await GetDevice(cdevice).get()
    if data:
        name = data["name"]
        device = data["device"]
    else:
        message = await tld(m, "Android.codename_notfound")
        await m.reply_text(message)
        return
    db = json.loads(
        (
            await http.get(
                "https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json"
            )
        ).content
    )
    device = db[device]
    message = (await tld(m, "Android.models_variant")).format(cdevice)
    for i in device:
        name = i["name"]
        model = i["model"]
        brand = i["brand"]
        message += (await tld(m, "Android.models_list")).format(brand, name, model)

    await m.reply_text(message)


__help__ = "Android"
