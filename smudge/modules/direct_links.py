# Copyright (C) 2019 The Raphielscape Company LLC.
#
# Licensed under the Raphielscape Public License, Version 1.c (the "License");
# you may not use this file except in compliance with the License.
#

import re
import requests
from random import choice
from telegram import Update
from bs4 import BeautifulSoup
from smudge import dispatcher, CallbackContext
from telegram.ext import run_async, CommandHandler


def direct_link_generator(update: Update, context: CallbackContext):
    message = update.effective_message
    text = message.text[len('/direct '):]

    if text:
        links = re.findall(r'\bhttps?://.*\.\S+', text)
    else:
        message.reply_text("Usage: /direct <url>")
        return
    reply = []
    if not links:
        message.reply_text("No links found!")
        return
    for link in links:
        if 'drive.google.com' in link:
            reply.append(gdrive(link))
        elif 'mediafire.com' in link:
            reply.append(mediafire(link))
        elif 'sourceforge.net' in link:
            reply.append(sourceforge(link))
        else:
            reply.append(
                re.findall(
                    r"\bhttps?://(.*?[^/]+)",
                    link)[0] +
                ' is not supported')

    message.reply_html("\n".join(reply))


def mediafire(url: str) -> str:
    try:
        link = re.findall(r'\bhttps?://.*mediafire\.com\S+', url)[0]
    except IndexError:
        reply = "<code>No MediaFire links found</code>\n"
        return reply
    reply = ''
    page = BeautifulSoup(requests.get(link).content, 'lxml')
    info = page.find('a', {'aria-label': 'Download file'})
    dl_url = info.get('href')
    size = re.findall(r'\(.*\)', info.text)[0]
    name = page.find('div', {'class': 'filename'}).text
    reply += f'<a href="{dl_url}">{name} ({size})</a>\n'
    return reply


def sourceforge(url: str) -> str:
    try:
        link = re.findall(r'\bhttps?://.*sourceforge\.net\S+', url)[0]
    except IndexError:
        reply = "<code>No SourceForge links found</code>\n"
        return reply
    file_path = re.findall(r'/files(.*)/download', link)
    if not file_path:
        file_path = re.findall(r'/files(.*)', link)
    file_path = file_path[0]
    reply = f"Mirrors for <i>{file_path.split('/')[-1]}</i>\n"
    project = re.findall(r'projects?/(.*?)/files', link)[0]
    mirrors = f'https://sourceforge.net/settings/mirror_choices?' \
        f'projectname={project}&filename={file_path}'
    page = BeautifulSoup(requests.get(mirrors).content, 'lxml')
    info = page.find('ul', {'id': 'mirrorList'}).findAll('li')
    for mirror in info[1:]:
        name = re.findall(r'\((.*)\)', mirror.text.strip())[0]
        dl_url = f'https://{mirror["id"]}.dl.sourceforge.net/project/{project}/{file_path}'
        reply += f'<a href="{dl_url}">{name}</a> '
    return reply


def useragent():
    useragents = BeautifulSoup(
        requests.get(
            'https://developers.whatismybrowser.com/'
            'useragents/explore/operating_system_name/android/').content,
        'lxml').findAll('td', {'class': 'useragent'})
    user_agent = choice(useragents)
    return user_agent.text


__help__ = "directlinks_help"

__mod_name__ = "Direct Links"

DIRECT_HANDLER = CommandHandler("direct", direct_link_generator)

dispatcher.add_handler(DIRECT_HANDLER)
