from pyrogram.enums import ChatType
from pyrogram.types import CallbackQuery

from smudge.database import database

conn = database.get_conn()


async def get_db_lang(message):
    message = message.message if isinstance(message, CallbackQuery) else message

    if message.chat.type == ChatType.PRIVATE:
        cursor = await conn.execute(
            "SELECT language FROM users WHERE id = (?)", (message.chat.id,)
        )
    elif message.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        cursor = await conn.execute(
            "SELECT language FROM groups WHERE id = (?)", (message.chat.id,)
        )
    try:
        row = await cursor.fetchone()
        await cursor.close()
        return row[0]
    except TypeError:
        return "en_US"


async def set_db_lang(message, code: str):
    message = message.message if isinstance(message, CallbackQuery) else message

    if message.chat.type == ChatType.PRIVATE:
        await conn.execute("UPDATE users SET language = ? WHERE id = ?", (code, message.chat.id))
    elif message.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        await conn.execute("UPDATE groups SET language = ? WHERE id = ?", (code, message.chat.id))
    await conn.commit()
