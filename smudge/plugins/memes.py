import random

from smudge.locales.strings import tld
from pyrogram import Client, filters
from pyrogram.types import Message


@Client.on_message(filters.command("slap"))
async def slap(c: Client, m: Message):
    if m.reply_to_message:
        try:
            user1 = (
                f"<a href='tg://user?id={m.from_user.id}'>{m.from_user.first_name}</a>"
            )
        except:
            user1 = m.chat.title
        try:
            user2 = f"<a href='tg://user?id={m.reply_to_message.from_user.id}'>{m.reply_to_message.from_user.first_name}</a>"
        except:
            user2 = m.chat.title

        temp = random.choice(await tld(m.chat.id, "memes_slaps_templates_list"))
        item = random.choice(await tld(m.chat.id, "memes_items_list"))
        hit = random.choice(await tld(m.chat.id, "memes_hit_list"))
        throw = random.choice(await tld(m.chat.id, "memes_throw_list"))

        reply = temp.format(user1=user1, user2=user2, item=item, hits=hit, throws=throw)

        await m.reply_text(reply)
    else:
        await m.reply_text("Bruuuh")
