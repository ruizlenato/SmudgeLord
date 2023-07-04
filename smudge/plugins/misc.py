import gettext

from pyrogram import filters
from pyrogram.enums import MessageEntityType
from pyrogram.types import Message

from smudge.bot import Smudge
from smudge.utils.locale import locale
from smudge.utils.utils import screenshot_page


@Smudge.on_message(filters.command("print"))
@locale()
async def prints(client: Smudge, message: Message, _):
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
                await message.reply_text(
                    _(
                        "<b>Usage:</b> <code>/print https://example.com</code> — \
Take a screenshot of the specified website."
                    )
                )
                return
        else:
            await message.reply_text(
                _(
                    "<b>Usage:</b> <code>/print https://example.com</code> — \
Take a screenshot of the specified website."
                )
            )
            return

    sent = await message.reply_text(_("Taking screenshot…"))

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
        await message.reply_text(_("Couldn't get url value, most probably API is not accessible."))


__help_name__ = gettext.gettext("Misc")
__help_text__ = gettext.gettext(
    """<b>/print —</b> Take a screenshot of the specified website.
<b>/tr —</b> Translates the text into the given language \
(the default is the user's default language).
"""
)
