from tortoise import Tortoise, fields
from tortoise.models import Model
from tortoise.exceptions import DoesNotExist, IntegrityError


class users(Model):
    user_id = fields.IntField(pk=True)
    lastfm_username = fields.TextField()


class lang(Model):
    chat_id = fields.IntField(pk=True)
    chat_lang = fields.TextField(default="en-US")


async def set_last_user(user_id: int, lastfm_username: str):
    try:
        await users.update_or_create(
            user_id=user_id, defaults=dict(lastfm_username=lastfm_username)
        )
        return
    except IntegrityError:
        await users.filter(user_id=user_id, lastfm_username=lastfm_username).delete()
        await users.create(user_id=user_id, lastfm_username=lastfm_username)
        return


async def get_last_user(user_id: int):
    try:
        return (await users.get(user_id=user_id)).lastfm_username
    except DoesNotExist:
        return None


async def set_db_lang(chat_id: int, lang_code: str):
    check_lang_exists = await lang.exists(chat_id=chat_id, chat_lang=lang_code)
    try:
        if check_lang_exists:
            await lang.create(chat_id=chat_id, chat_lang=lang_code)
            return
        else:
            await lang.filter(chat_id=chat_id).update(chat_lang=lang_code)
            return
    except IntegrityError:
        await lang.filter(chat_id=chat_id, chat_lang=lang_code).delete()
        await lang.create(chat_id=chat_id, chat_lang=lang_code)
        return


async def get_db_lang(chat_id: int):
    try:
        return (await lang.get_or_create({"chat_lang": "en-US"}, chat_id=chat_id))[
            0
        ].chat_lang
    except DoesNotExist:
        return None


async def connect_database():
    await Tortoise.init(
        db_url="sqlite://smudge/database/database.db", modules={"models": [__name__]}
    )
    await Tortoise.generate_schemas()
