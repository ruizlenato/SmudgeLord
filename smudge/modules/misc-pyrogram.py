from asyncio import sleep
from telethon import custom
from smudge import LOGGER
from smudge.events import register
from smudge.modules.translations.strings import tld


#@register(group_only=True, pattern="^/delusers(?: |$)(.*)")
async def rm_deletedacc(event):
    chat_id = event.chat_id
    con = event.pattern_match.group(1)
    del_u = 0
    del_status = "`No deleted accounts found, Group is cleaned as Hell`"

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

    # Well
    if not admin and not creator:
        await event.reply(tld(chat_id, "zombies_admin_noperm"))
        return

    await event.reply(tld(chat_id, "zombies_cleaning"))
    del_u = 0
    del_a = 0

    async for user in event.client.iter_participants(event.chat_id):
        if user.deleted:
            try:
                await event.client(
                    EditBannedRequest(event.chat_id, user.id, BANNED_RIGHTS))
            except ChatAdminRequiredError:
                await event.edit("`You don't have enough rights.`")
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
