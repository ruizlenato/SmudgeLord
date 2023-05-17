import gettext
import re

import filetype
from pyrogram import filters
from pyrogram.enums import ChatAction, ChatType
from pyrogram.raw.functions import channels, messages
from pyrogram.raw.types import InputMessageID
from pyrogram.types import InputMediaPhoto, InputMediaVideo, Message

from ..bot import Smudge
from ..utils.locale import locale
from ..utils.medias import DownloadMedia

REGEX_LINKS = r"(?:htt.+?//)?(?:.+?)?(?:instagram|twitter|tiktok|facebook).com\/(?:\S*)"


@Smudge.on_message(filters.command(["dl", "sdl"]) | filters.regex(REGEX_LINKS), group=1)
@locale()
async def sdl(client: Smudge, message: Message, _):
    analise = False
    if message.matches:
        if message.chat.type is ChatType.PRIVATE or analise:
            url = message.matches[0].group(0)
        else:
            return None
    elif not message.matches and len(message.command) > 1:
        url = message.text.split(None, 1)[1]
        if not re.match(REGEX_LINKS, url, re.M):
            return await message.reply_text(
                _(
                    "<b>System glitch someone disconnected me.</b>\nThe link you sent is invalid, \
currently I only support links from TikTok, Twitter and Instagram."
                )
            )
    elif message.reply_to_message and message.reply_to_message.text:
        url = message.reply_to_message.text
    else:
        return await message.reply_text(
            _(
                "<b>Usage:</b> <code>/dl [link]</code>\n\nSpecify a link from Instagram, TikTok \
or Twitter so I can download the video."
            )
        )

    if message.chat.type == ChatType.PRIVATE:
        method = messages.GetMessages(id=[InputMessageID(id=(message.id))])
    else:
        method = channels.GetMessages(
            channel=await client.resolve_peer(message.chat.id),
            id=[InputMessageID(id=(message.id))],
        )

    rawM = (await client.invoke(method)).messages[0].media
    files, caption = await DownloadMedia().download(url)

    medias = []
    for media in files:
        if filetype.is_video(media["p"]) and len(files) == 1:
            await client.send_chat_action(message.chat.id, ChatAction.UPLOAD_VIDEO)
            return await message.reply_video(
                video=media["p"],
                width=media["h"],
                height=media["h"],
                caption=caption,
            )

        if filetype.is_video(media["p"]):
            if medias:
                medias.append(InputMediaVideo(media["p"], width=media["w"], height=media["h"]))
            else:
                medias.append(
                    InputMediaVideo(
                        media["p"],
                        width=media["w"],
                        height=media["h"],
                        caption=caption,
                    )
                )
        elif not medias:
            medias.append(InputMediaPhoto(media["p"], caption=caption))
        else:
            medias.append(InputMediaPhoto(media["p"]))

    if medias:
        if (
            rawM
            and not re.search(r"instagram.com/", url)
            and len(medias) == 1
            and "InputMediaPhoto" in str(medias[0])
        ):
            return None

        await client.send_chat_action(message.chat.id, ChatAction.UPLOAD_DOCUMENT)
        await message.reply_media_group(media=medias)
        return None
    return None


__help_name__ = gettext.gettext("Videos")
__help_text__ = gettext.gettext(
    "<b>/dl|/sdl -</b> <i>Downloads videos from <b>Instagram, TikTok and Twitter.</b></i>\n"
)
