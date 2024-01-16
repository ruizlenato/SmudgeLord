from hydrogram import filters
from hydrogram.enums import ChatMemberStatus, ChatType
from hydrogram.errors import ChatAdminRequired
from hydrogram.types import CallbackQuery, Message


async def FilterAdmin(_, bot, union: CallbackQuery | Message) -> bool:
    message = union.message if isinstance(union, CallbackQuery) else union
    chat = message.chat
    user = union.from_user

    if chat.type == ChatType.PRIVATE:
        return True

    if not user:
        return False

    try:
        member = await bot.get_chat_member(chat.id, user.id)
    except ChatAdminRequired:
        return False
    return member.status in (ChatMemberStatus.ADMINISTRATOR, ChatMemberStatus.OWNER)


filters.admin = filters.create(FilterAdmin, "FilterAdmin")
