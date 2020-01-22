# Copyright (C) 2019 The Raphielscape Company LLC.
#
# Licensed under the Raphielscape Public License, Version 1.c (the "License");
# you may not use this file except in compliance with the License.
#

from os import popen
import re
import urllib.parse
import json
from random import choice
import requests
from bs4 import BeautifulSoup
from hurry.filesize import size as naturalsize
from telegram import Bot, Update
from telegram.ext import run_async, CommandHandler

from smudge import dispatcher


@run_async
def direct_link_generator(bot: Bot, update: Update):
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
        elif 'androidfilehost.com' in link:
            reply.append(androidfilehost(link))
        else:
            reply.append(re.findall(r"\bhttps?://(.*?[^/]+)", link)[0] + ' is not supported')

    message.reply_html("\n".join(reply))


def gdrive(url: str) -> str:
    drive = 'https://drive.google.com'
    try:
        link = re.findall(r'\bhttps?://drive\.google\.com\S+', url)[0]
    except IndexError:
        reply = "<code>No Google drive links found</code>\n"
        return reply
    file_id = ''
    reply = ''
    if link.find("view") != -1:
        file_id = link.split('/')[-2]
    elif link.find("open?id=") != -1:
        file_id = link.split("open?id=")[1].strip()
    elif link.find("uc?id=") != -1:
        file_id = link.split("uc?id=")[1].strip()
    url = f'{drive}/uc?export=download&id={file_id}'
    download = requests.get(url, stream=True, allow_redirects=False)
    cookies = download.cookies
    try:
        # In case of small file size, Google downloads directly
        dl_url = download.headers["location"]
        if 'accounts.google.com' in dl_url:  # non-public file
            reply += '<code>Link is not public!<code>\n'
            return reply
        name = 'Direct Download Link'
    except KeyError:
        # In case of download warning page
        page = BeautifulSoup(download.content, 'lxml')
        export = drive + page.find('a', {'id': 'uc-download-link'}).get('href')
        name = page.find('span', {'class': 'uc-name-size'}).text
        response = requests.get(export,
                                stream=True,
                                allow_redirects=False,
                                cookies=cookies)
        dl_url = response.headers['location']
        if 'accounts.google.com' in dl_url:
            reply += 'Link is not public!'
            return reply
    reply += f'<a href="{dl_url}">{name}</a>\n'
    return reply


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

__help__ = """
*This module allows you to generate direct links from various websites.*

/direct <url>: paste a URL to generate a direct download link.

*List of supported URLs:*
`Google Drive - MediaFire - SourceForge`
"""

__mod_name__ = "Direct Links"

DIRECT_HANDLER = CommandHandler("direct", direct_link_generator)

dispatcher.add_handler(DIRECT_HANDLER)
