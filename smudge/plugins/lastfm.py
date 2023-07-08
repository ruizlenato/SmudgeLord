from pyrogram import filters
from pyrogram.types import ForceReply, Message

from smudge.bot import Smudge
from smudge.utils.lastfm import LastFM
from smudge.utils.locale import locale


@Smudge.on_message(filters.command(["setuser", "setlast"]))
@locale("lastfm")
async def setuser(client: Smudge, message: Message, strings):
    if message.reply_to_message and message.reply_to_message.text:
        username = message.reply_to_message.text
        mesid = message.id
    elif len(message.command) > 1:
        username = message.text.split(None, 1)[1]
        mesid = message.id
    else:
        answer = await message.chat.ask(
            strings["ask-username"].format(message.from_user.id, message.from_user.first_name),
            filters=filters.user(message.from_user.id) & filters.incoming,
            reply_markup=ForceReply(selective=True),
        )

        if not answer.reply_to_message:
            return

        username = answer.text
        mesid = answer.id

    if username:
        LastAPI = await LastFM().register_lastfm(message.from_user.id, username)

        if not LastAPI:
            await message.reply_text(strings["wrong-username"], reply_to_message_id=mesid)
            return

        await message.reply_text(strings["saved-username"], reply_to_message_id=mesid)


@Smudge.on_message(filters.command(["lastfm", "lmu", "lt"]))
@locale("lastfm")
async def track(client: Smudge, message: Message, strings):
    LastAPI = await LastFM().track(message.from_user.id)
    if LastAPI == "No Username":
        return await message.reply_text(strings["no-username"])

    if LastAPI == "No Scrobbles":
        await message.reply_text(strings["no-scrobbles"])

    rep = f"<a href='{LastAPI['image']}'>\u200c</a>"
    if LastAPI["now"]:
        rep += strings["is-listening"].format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    else:
        rep += strings["was-listening"].format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )

    rep += f"<b>{LastAPI['artist']}</b> - {LastAPI['track']}"
    rep += "❤️" if LastAPI["loved"] else ""

    await message.reply_text(rep)
    return None


@Smudge.on_message(filters.command(["lalbum", "lalb", "album"]))
@locale("lastfm")
async def album(client: Smudge, message: Message, strings):
    LastAPI = await LastFM().album(message.from_user.id)
    if LastAPI == "No Username":
        return await message.reply_text(strings["no-username"])

    if LastAPI == "No Scrobbles":
        await message.reply_text(strings["no-scrobbles"])

    rep = f"<a href='{LastAPI['image']}'>\u200c</a>"
    if LastAPI["now"]:
        rep += strings["is-listening"].format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    else:
        rep += strings["was-listening"].format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    rep += f"<b>🎙 {LastAPI['artist']}</b>\n📀 {LastAPI['album']}"
    rep += "❤️" if LastAPI["loved"] else ""
    await message.reply_text(rep)
    return None


@Smudge.on_message(filters.command(["lartist", "lart", "artist"]))
@locale("lastfm")
async def artist(client: Smudge, message: Message, strings):
    LastAPI = await LastFM().artist(message.from_user.id)
    if LastAPI == "No Username":
        return await message.reply_text(strings["no-username"])

    if LastAPI == "No Scrobbles":
        await message.reply_text(strings["no-scrobbles"])

    rep = f"<a href='{LastAPI['image']}'>\u200c</a>"
    if LastAPI["now"]:
        rep += strings["is-listening"].format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    else:
        rep += strings["was-listening"].format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    rep += f"🎙<b>{LastAPI['artist']}</b>"
    rep += "❤️" if LastAPI["loved"] else ""
    await message.reply_text(rep)
    return None


__help__ = True
