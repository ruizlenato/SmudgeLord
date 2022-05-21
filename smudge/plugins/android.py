# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import rapidjson
from bs4 import BeautifulSoup

from pyrogram import filters
from pyrogram.helpers import ikb
from pyrogram.types import Message
from smudge import Smudge
from smudge.utils import http
from smudge.plugins import tld

# Port from SamsungGeeksBot.


class GetDevice:
    def __init__(self, device):
        """Get device info by codename or model!"""
        self.device = device

    async def get(self):
        if self.device.lower().startswith("sm-"):
            data = await http.get(
                "https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_model.json"
            )
            db = rapidjson.loads(data)
            try:
                name = db[self.device.upper()][0]["name"]
                device = db[self.device.upper()][0]["device"]
                brand = db[self.device.upper()][0]["brand"]
                model = self.device.lower()
                return {"name": name, "device": device, "model": model, "brand": brand}
            except KeyError:
                return False
        else:
            data = await http.get(
                "https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json"
            )
            database = rapidjson.loads(data.content)
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
                return {"name": name, "device": device, "model": model, "brand": brand}
            except KeyError:
                return False


@Smudge.on_message(filters.command(["magisk"]))
async def magisk(c: Smudge, m: Message):
    repo_url = "https://raw.githubusercontent.com/topjohnwu/magisk-files/master/"
    text = await tld(m, "Android.magisk_releases")
    for magisk_type in ["stable", "beta", "canary"]:
        fetch = await http.get(repo_url + magisk_type + ".json")
        data = rapidjson.loads(fetch.content)
        text += (
            f"<b>{magisk_type.capitalize()}</b>:\n"
            f'<a href="{data["magisk"]["link"]}" >Magisk - V{data["magisk"]["version"]}</a>'
            f' | <a href="{data["magisk"]["note"]}" >Changelog</a> \n'
        )
    await m.reply_text(text, disable_web_page_preview=True)


@Smudge.on_message(filters.command(["twrp"]))
async def twrp(c: Smudge, m: Message):
    if not len(m.command) == 2:
        message = "Please write your codename into it, i.e <code>/twrp herolte</code>"
        await m.reply_text(message)
        return
    device = m.command[1]
    url = await http.get(f"https://eu.dl.twrp.me/{device}/")
    if url.status_code == 404:
        await m.reply_text(f"TWRP currently is not avaliable for <code>{device}</code>")
    else:
        message = f"<b>Latest TWRP Recovery For {device}</b>\n"
        page = BeautifulSoup(url.content, "html.parser")
        date = page.find("em").text.strip()
        message += f"<b>Updated:</b> <code>{date}</code>\n"
        trs = page.find("table").find_all("tr")
        row = 2 if trs[0].find("a").text.endswith("tar") else 1
        keyboard = []
        for i in range(row):
            download = trs[i].find("a")
            dl_link = f"https://eu.dl.twrp.me{download['href']}"
            dl_file = download.text
            keyboard += [[(dl_file.upper(), dl_link, "url")]]
        size = trs[i].find("span", {"class": "filesize"}).text
        message += f"<b>Size:</b> <code>{size}</code>\n"
        await m.reply_text(
            message,
            reply_markup=ikb(keyboard),
        )


@Smudge.on_message(filters.command(["whatis", "device", "codename"]))
async def models(c: Smudge, m: Message):
    if not len(m.command) == 2:
        message = await tld(m, "Android.whatis_nocodename")
        await m.reply_text(message)
        return
    device = m.command[1]
    data = await GetDevice(device).get()
    if data:
        name = data["name"]
        device = data["device"]
        brand = data["brand"]
        model = data["model"]
    else:
        message = await tld(m, "Android.codename_notfound")
        await m.reply_text(message)
        return
    message = (await tld(m, "Android.whatis_device")).format(
        model, model.upper(), brand, name
    )
    await m.reply_text(message)


@Smudge.on_message(filters.command(["variants", "models"]))
async def variants(c: Smudge, m: Message):
    if not len(m.command) == 2:
        message = await tld(m, "Android.models_nocodename")
        await m.reply_text(message)
        return
    cdevice = m.command[1]
    data = await GetDevice(cdevice).get()
    if data:
        name = data["name"]
        device = data["device"]
    else:
        message = await tld(m, "Android.codename_notfound")
        await m.reply_text(message)
        return
    data = await http.get(
        "https://raw.githubusercontent.com/androidtrackers/certified-android-devices/master/by_device.json"
    )
    db = rapidjson.loads(data.content)
    device = db[device]
    message = (await tld(m, "Android.models_variant")).format(cdevice)
    for i in device:
        name = i["name"]
        model = i["model"]
        message += (await tld(m, "Android.models_list")).format(model, name)

    await m.reply_text(message)


plugin_name = "Android.name"
plugin_help = "Android.help"
