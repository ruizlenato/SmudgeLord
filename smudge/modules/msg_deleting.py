from smudge.events import register
from smudge.helper_funcs.telethon.chat_status import user_is_admin, can_delete_messages
from smudge.modules.translations.strings import tld


@register(pattern="^/purge")
async def purge(event):
    if event.from_id == None:
        return

    chat = event.chat_id

    if not await user_is_admin(user_id=event.from_id, message=event):
        await event.reply(tld(chat, "helpers_user_not_admin"))
        return

    if not await can_delete_messages(message=event):
        await event.reply(tld(chat, "helpers_bot_cant_delete"))
        return

    msg = await event.get_reply_message()
    if not msg:
        await event.reply(tld(chat, "purge_invalid"))
        return
    msgs = []
    msg_id = msg.id
    delete_to = event.message.id - 1
    await event.client.delete_messages(chat, event.message.id)

    msgs.append(event.reply_to_msg_id)
    for m_id in range(delete_to, msg_id - 1, -1):
        msgs.append(m_id)
        if len(msgs) == 100:
            await event.client.delete_messages(chat, msgs)
            msgs = []

    await event.client.delete_messages(chat, msgs)
    text = tld(chat, "purge_msg_success")
    await event.respond(text, parse_mode='md')


@register(pattern="^/del$")
async def delet(event):
    if event.from_id == None:
        return

    chat = event.chat_id

    if not await user_is_admin(user_id=event.from_id, message=event):
        await event.reply(tld(chat, "helpers_user_not_admin"))
        return

    if not await can_delete_messages(message=event):
        await event.reply(tld(chat, "helpers_bot_cant_delete"))
        return

    msg = await event.get_reply_message()
    if not msg:
        await event.reply(tld(chat, "purge_invalid"))
        return
    currentmsg = event.message
    chat = await event.get_input_chat()
    delall = [msg, currentmsg]
    await event.client.delete_messages(chat, delall)


__help__ = True
