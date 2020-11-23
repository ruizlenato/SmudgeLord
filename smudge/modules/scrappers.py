import re
import pafy
from telethon import custom

from smudge import LOGGER
from smudge.events import register
from smudge.modules.translations.strings import tld


LOGGER.info("YTDownloader: By @Nick80835 (modified by @Renatoh on Telegram)")


@register(pattern=r"^/yt(?: |)([\S]*)(?: |)([\s\S]*)")
async def youtube_cmd(event):
    youtube_link = event.pattern_match.group(1)
    video = pafy.new(youtube_link)
    video_stream = video.getbest()
    try:
        await event.client.send_file(event.chat_id, file=video_stream.url)
    except:
        await event.reply(f"`Download failed: `[URL]({video_stream.url})")
