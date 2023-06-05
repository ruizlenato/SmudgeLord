from pyrogram import filters
from pyrogram.enums import ChatMemberStatus, ChatType
from pyrogram.types import CallbackQuery, Message


async def FilterAdmin(_, bot, union: CallbackQuery | Message) -> bool:
    message = union.message if isinstance(union, CallbackQuery) else union
    chat = message.chat
    user = union.from_user

    if chat.type == ChatType.PRIVATE:
        return True

    if not user:
        return False

    member = await bot.get_chat_member(chat.id, user.id)
    return member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER)


filters.admin = filters.create(FilterAdmin, "FilterAdmin")
