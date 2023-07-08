from pyrogram import filters
from pyrogram.enums import MessageEntityType
from pyrogram.types import Message

from smudge.bot import Smudge
from smudge.utils.locale import locale
from smudge.utils.utils import screenshot_page


@Smudge.on_message(filters.command("print"))
@locale("misc")
async def prints(client: Smudge, message: Message, strings):
    for entity in message.entities or message.caption_entities:
        if entity.type == MessageEntityType.URL:
            if message.text:
                target_url = message.text[entity.offset : entity.offset + entity.length]
            else:
                target_url = message.caption[entity.offset : entity.offset + entity.length]
            break
        if entity.type == MessageEntityType.TEXT_LINK:
            target_url = entity.url
            break
    else:
        if message.reply_to_message:
            for entity in (
                message.reply_to_message.entities or message.reply_to_message.caption_entities
            ):
                if entity.type == MessageEntityType.URL:
                    if message.reply_to_message.text:
                        target_url = message.reply_to_message.text[
                            entity.offset : entity.offset + entity.length
                        ]
                    else:
                        target_url = message.reply_to_message.caption[
                            entity.offset : entity.offset + entity.length
                        ]
                    break
                if entity.type == MessageEntityType.TEXT_LINK:
                    target_url = entity.url
                    break
            else:
                await message.reply_text(strings["print-no-args"])
                return
        else:
            await message.reply_text(strings["print-no-args"])
            return

    sent = await message.reply_text(strings["taking-screenshot"])

    try:
        response = await screenshot_page(target_url)
    except BaseException as e:
        await sent.edit_text(f"<b>API returned an error:</b> <code>{e}</code>")
        return

    if response:
        try:
            await message.reply_photo(response)
        except BaseException as e:
            # if failed to send the message, it's not API's fault.
            # most probably there are some other kind of problem,
            # for example it failed to delete its message.
            # or the bot doesn't have access to send media in the chat.
            await sent.edit_text(f"Failed to send the screenshot due to: {e!s}")
        else:
            await sent.delete()
    else:
        await message.reply_text(strings["error-api"])


__help__ = True
