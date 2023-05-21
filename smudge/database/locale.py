from pyrogram.enums import ChatType
from pyrogram.types import CallbackQuery

from smudge.database import database

conn = database.get_conn()


async def get_db_lang(m):
    m = m.message if isinstance(m, CallbackQuery) else m

    if m.chat.type == ChatType.PRIVATE:
        cursor = await conn.execute("SELECT language FROM users WHERE user_id = (?)", (m.chat.id,))
    elif m.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        cursor = await conn.execute(
            "SELECT language FROM groups WHERE chat_id = (?)", (m.chat.id,)
        )
    try:
        row = await cursor.fetchone()
        await cursor.close()
        return row[0]
    except TypeError:
        return "en_US"


async def set_db_lang(m, code: str):
    m = m.message if isinstance(m, CallbackQuery) else m

    if m.chat.type == ChatType.PRIVATE:
        await conn.execute("UPDATE users SET language = ? WHERE user_id = ?", (code, m.chat.id))
    elif m.chat.type in (ChatType.GROUP, ChatType.SUPERGROUP):
        await conn.execute("UPDATE groups SET language = ? WHERE chat_id = ?", (code, m.chat.id))
    await conn.commit()
