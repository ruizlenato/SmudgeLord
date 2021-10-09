from pyrogram import Client
from pyrogram.types import Message

from smudge.database.core import users, groups

# This is the first plugin run to guarantee
# that the actual chat is initialized in the DB.


async def add_chat(chat_id, chat_type, m_user_id):
    if chat_type == "private":
        await users.create(user_id=chat_id)
    elif (
        chat_type in "group" or "supergroup"
    ):  # groups and supergroups share the same table
        await groups.create(chat_id=chat_id)
        await users.create(user_id=m_user_id)
    else:
        raise TypeError("Unknown chat type '%s'." % chat_type)
    return True


async def chat_exists(chat_id, chat_type, m_user_id):
    if chat_type == "private":
        return await users.exists(user_id=chat_id)
    if (
        chat_type == "group" or "supergroup"
    ):  # groups and supergroups share the same table
        return await groups.exists(chat_id=chat_id)
        return await users.exists(user_id=m_user_id)
    raise TypeError("Unknown chat type '%s'." % chat_type)


@Client.on_message(group=-1)
async def check_chat(c: Client, m: Message):
    chat_id = m.chat.id
    chat_type = m.chat.type
    m_user_id = m.from_user.id
    check_the_chat = await chat_exists(chat_id, chat_type, m_user_id)

    if not check_the_chat:
        await add_chat(chat_id, chat_type, m_user_id)
