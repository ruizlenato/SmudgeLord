import os
import re

import aiohttp
import youtube_dl
from bs4 import BeautifulSoup

from config import bot
from utils import pretty_size

ydl = youtube_dl.YoutubeDL({'outtmpl': 'dls/%(title)s.%(ext)s', 'format': '140', 'noplaylist': True})


async def search_yt(query):
    url_base = "https://www.youtube.com/results"
    url_yt = "https://www.youtube.com"
    async with aiohttp.ClientSession() as session:
        r = await session.get(url_base, params=dict(q=query))
        page = await r.text()
    soup = BeautifulSoup(page, "html.parser")
    id_url = None
    list_videos = []
    for link in soup.find_all('a'):
        url = link.get('href')
        title = link.get('title')
        if url.startswith("/watch") and (id_url != url) and (title is not None):
            id_url = url
            dic = {'title': title, 'url': url_yt + url}
            list_videos.append(dic)
        else:
            pass
    return list_videos


async def youtube(msg):
    if msg.get('text'):

        if msg['text'].startswith('/yt '):
            res = await search_yt(msg['text'][4:])

            vids = ['{}: <a href="{}">{}</a>'.format(num + 1, i['url'], i['title']) for num, i in enumerate(res)]
            await bot.sendMessage(msg['chat']['id'], '\n'.join(vids) if vids else "Nenhum resultado foi encontrado", 'HTML',
                                      reply_to_message_id=msg['message_id'],
                                      disable_web_page_preview=True)
            return True


        elif msg['text'].split()[0] == '/ytdl':
            text = msg['text'][6:]

            if text:
                sent_id = (await bot.sendMessage(msg['chat']['id'], 'Obtendo informações do vídeo...', 'Markdown',
                                                 reply_to_message_id=msg['message_id']))['message_id']
                try:
                    if re.match(r'^(https?://)?(youtu\.be/|(m\.|www\.)?youtube\.com/watch\?v=).+', text):
                        yt = ydl.extract_info(text, download=False)
                    else:
                        yt = ydl.extract_info('ytsearch:' + text, download=False)['entries'][0]
                    for f in yt['formats']:
                        if f['format_id'] == '140':
                            fsize = f['filesize'] or 0
                    name = yt['title']
                except Exception as e:
                    return await bot.editMessageText((msg['chat']['id'], sent_id), 'Ocorreu um erro.\n\n' + str(e))
                if not fsize > 52428800:
                    if ' - ' in name:
                        performer, title = name.rsplit(' - ', 1)
                    else:
                        performer = yt.get('creator') or yt.get('uploader')
                        title = name
                    await bot.editMessageText((msg['chat']['id'], sent_id),
                                              'Baixando <code>{}</code> do YouTube...\n({})'.format(name,
                                                                                                    pretty_size(fsize)),
                                              'HTML')
                    ydl.download(['https://www.youtube.com/watch?v=' + yt['id']])
                    await bot.editMessageText((msg['chat']['id'], sent_id), 'Enviando áudio...')
                    await bot.sendChatAction(msg['chat']['id'], 'upload_document')
                    await bot.sendAudio(msg['chat']['id'], open(ydl.prepare_filename(yt), 'rb'),
                                        performer=performer,
                                        title=title,
                                        duration=yt['duration'],
                                        reply_to_message_id=msg['message_id'])
                    os.remove(ydl.prepare_filename(yt))
                    await bot.deleteMessage((msg['chat']['id'], sent_id))
                else:
                    await bot.editMessageText((msg['chat']['id'], sent_id),
                                              f'Ow, o arquivo resultante ({pretty_size(fsize)}) ultrapassa o meu limite de 50 MB')

            else:
                await bot.sendMessage(msg['chat']['id'], '*Uso:* /ytdl URL do vídeo ou nome', 'Markdown',
                                      reply_to_message_id=msg['message_id'])

            return True


__help__ = """
*Test module*
*Useful tools:*
 - /yt test module

__mod_name__ = "youtube"

BMOJI_HANDLER = CommandHandler("yt", yt)
BMOJI_HANDLER = CommandHandler("ytdl", ytdl)


dispatcher.add_handler(YTDL_HANDLER)
dispatcher.add_handler(yt_HANDLER)


