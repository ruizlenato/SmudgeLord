from smudge import pbot
import asyncio
from pyrogram import Client, filters, errors
from pyrogram.types import Update
from smudge.modules.translations.strings import tld


async def admin_check(c: Client, update: Update) -> bool:
    chat_id = update.chat.id
    user_id = update.from_user.id

    user = await c.get_chat_member(chat_id=chat_id, user_id=user_id)
    admin_strings = ["creator", "administrator"]

    if user.status not in admin_strings:
        await update.reply_text(
            "This is an Admin Restricted command and you're not allowed to use it."
        )
        return False

    return True


@pbot.on_message(filters.command("purge"))
async def purge(c: Client, update: Update):

    chat_id = update.chat.id
    res = await admin_check(c, update)
    if not res:
        return

    if update.chat.type != "supergroup":
        await update.reply_text("purge_gruop")
        return
    purge_msg = await update.reply_text(tld(chat_id, "purge_msg_deleting"))
    message_ids = []

    if update.reply_to_message:
        for a_msg in range(update.reply_to_message.message_id, update.message_id):
            message_ids.append(a_msg)

    if (
        not update.reply_to_message
        and len(update.text.split()) == 2
        and isinstance(update.text.split()[1], int)
    ):
        c_msg_id = update.message_id
        first_msg = (update.message_id) - (update.text.split()[1])
        for a_msg in range(first_msg, c_msg_id):
            message_ids.append(a_msg)

    try:
        await c.delete_messages(chat_id=update.chat.id, message_ids=message_ids, revoke=True)
    except errors.MessageDeleteForbidden:
        await update.edit_text(tld(chat_id, "purge_old_fail"))
        return

    count_del_msg = len(message_ids)

    await purge_msg.edit_text(tld(chat_id, "purge_msg_success"))
    await asyncio.sleep(3)
    await purge_msg.delete()
    return


@pbot.on_message(filters.command("del"))
async def del_msg(c: Client, update: Update):

    res = await admin_check(c, update)
    if not res:
        return

        await c.delete_messages(
            chat_id=update.chat.id, message_ids=update.reply_to_message.message_id
        )
        await asyncio.sleep(0.4)
        await update.delete()
    else:
        delm = await update.reply_text("Finish")
        delm
        await delm.delete()
    return
