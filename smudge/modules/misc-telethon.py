from asyncio import sleep
from telethon import custom
from smudge import LOGGER
from smudge.events import register
from smudge.modules.translations.strings import tld
from smudge.helper_funcs.telethon.chat_status import user_is_admin


@register(group_only=True, pattern="^/delusers(?: |$)(.*)")
async def rm_deletedacc(event):
    chat = event.chat_id

    if not await user_is_admin(user_id=event.from_id, message=event):
        await event.reply(tld(chat, "helpers_user_not_admin"))
        return
    chat_id = event.chat_id
    con = event.pattern_match.group(1)
    del_u = 0
    del_status = (tld(chat_id, "deleted_acc_false"))

    if con != "clean":
        message1 = await event.reply(tld(chat_id, "zombies_searching_accounts"))
        async for user in event.client.iter_participants(event.chat_id):
            if user.deleted:
                del_u += 1
                await sleep(1)

        if del_u > 0:
            del_status = tld(chat_id, "zombies_result_accounts").format(del_u)
        await message1.edit(del_status)
        return

    # Here laying the sanity check
    chat = await event.get_chat()
    admin = chat.admin_rights
    creator = chat.creator

    await event.reply(tld(chat_id, "zombies_cleaning"))
    del_u = 0
    del_a = 0

    async for user in event.client.iter_participants(event.chat_id):
        if user.deleted:
            try:
                await event.client(
                    EditBannedRequest(event.chat_id, user.id, BANNED_RIGHTS))
            except ChatAdminRequiredError:
                await event.edit(tld(chat_id, "helpers_bot_not_admin"))
                return
            except UserAdminInvalidError:
                del_u -= 1
                del_a += 1
            await event.client(
                EditBannedRequest(event.chat_id, user.id, UNBAN_RIGHTS))
            del_u += 1
            await sleep(1)
    if del_u > 0:
        del_status = f"cleaned **{del_u}** deleted account(s)"

    if del_a > 0:
        del_status = f"cleaned **{del_u}** deleted account(s) \
\n**{del_a}** deleted admin accounts are not removed"

    await event.edit(del_status)
