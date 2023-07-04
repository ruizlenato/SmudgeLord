import gettext

from pyrogram import filters
from pyrogram.types import ForceReply, Message

from smudge.bot import Smudge
from smudge.utils.lastfm import LastFM
from smudge.utils.locale import locale


@Smudge.on_message(filters.command(["setuser", "setlast"]))
@locale()
async def setuser(client: Smudge, message: Message, _):
    if message.reply_to_message and message.reply_to_message.text:
        username = message.reply_to_message.text
        mesid = message.id
    elif len(message.command) > 1:
        username = message.text.split(None, 1)[1]
        mesid = message.id
    else:
        answer = await message.chat.ask(
            _("<a href='tg://user?id={}'>{}</a>, Send me your last.fm username.").format(
                message.from_user.id, message.from_user.first_name
            ),
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
            await message.reply_text(
                _("<b>Error</b>\nYour last.fm username is wrong"), reply_to_message_id=mesid
            )
            return

        await message.reply_text(_("<b>Done, last.fm user saved!</b>"), reply_to_message_id=mesid)


@Smudge.on_message(filters.command(["lastfm", "lmu", "lt"]))
@locale()
async def track(client: Smudge, message: Message, _):
    LastAPI = await LastFM().track(message.from_user.id)
    if LastAPI == "No Username":
        return await message.reply_text(
            _("<b>You have not set your last.fm username.</b>\nUse the command /setuser to set")
        )

    if LastAPI == "No Scrobbles":
        await message.reply_text(
            _(
                "<b>Apparently you have never scrobbled a song on LastFM.</b>\n\nIf you are having\
 trouble, go to last.fm/about/trackmymusic and see how to connect your account to your music app."
            )
        )

    rep = f"<a href='{LastAPI['image']}'>\u200c</a>"
    if LastAPI["now"]:
        rep += _(
            "<b><a href='https://last.fm/user/{}'>{}</a></b> is listening for the \
<b>{}nd time</b>:\n\n"
        ).format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    else:
        rep += _(
            "<b><a href='https://last.fm/user/{}'>{}</a></b> was listening for the \
<b>{}nd time</b>:\n\n"
        ).format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )

    rep += f"<b>{LastAPI['artist']}</b> - {LastAPI['track']}"
    rep += "❤️" if LastAPI["loved"] else ""

    await message.reply_text(rep)
    return None


@Smudge.on_message(filters.command(["lalbum", "lalb", "album"]))
@locale()
async def album(client: Smudge, message: Message, _):
    LastAPI = await LastFM().album(message.from_user.id)
    if LastAPI == "No Username":
        return await message.reply_text(
            _("<b>You have not set your last.fm username.</b>\nUse the command /setuser to set")
        )

    if LastAPI == "No Scrobbles":
        await message.reply_text(
            _(
                "<b>Apparently you have never scrobbled a song on LastFM.</b>\n\nIf you are having\
 trouble, go to last.fm/about/trackmymusic and see how to connect your account to your music app."
            )
        )

    rep = f"<a href='{LastAPI['image']}'>\u200c</a>"
    if LastAPI["now"]:
        rep += _(
            "<b><a href='https://last.fm/user/{}'>{}</a></b> is listening for the \
<b>{}nd time</b>:\n\n"
        ).format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    else:
        rep += _(
            "<b><a href='https://last.fm/user/{}'>{}</a></b> was listening for the \
<b>{}nd time</b>:\n\n"
        ).format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    rep += f"<b>🎙 {LastAPI['artist']}</b>\n📀 {LastAPI['album']}"
    rep += "❤️" if LastAPI["loved"] else ""
    await message.reply_text(rep)
    return None


@Smudge.on_message(filters.command(["lartist", "lart", "artist"]))
@locale()
async def artist(client: Smudge, message: Message, _):
    LastAPI = await LastFM().artist(message.from_user.id)
    if LastAPI == "No Username":
        return await message.reply_text(
            _("<b>You have not set your last.fm username.</b>\nUse the command /setuser to set")
        )

    if LastAPI == "No Scrobbles":
        await message.reply_text(
            _(
                "<b>Apparently you have never scrobbled a song on LastFM.</b>\n\nIf you are having\
 trouble, go to last.fm/about/trackmymusic and see how to connect your account to your music app."
            )
        )

    rep = f"<a href='{LastAPI['image']}'>\u200c</a>"
    if LastAPI["now"]:
        rep += _(
            "<b><a href='https://last.fm/user/{}'>{}</a></b> is listening for the \
<b>{}nd time</b>:\n\n"
        ).format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    else:
        rep += _(
            "<b><a href='https://last.fm/user/{}'>{}</a></b> was listening for the \
<b>{}nd time</b>:\n\n"
        ).format(
            await LastFM().get_username(message.from_user.id),
            message.from_user.first_name,
            LastAPI["playcount"],
        )
    rep += f"🎙<b>{LastAPI['artist']}</b>"
    rep += "❤️" if LastAPI["loved"] else ""
    await message.reply_text(rep)
    return None


__help_name__ = gettext.gettext("LastFM")
__help_text__ = gettext.gettext(
    """Last.fm is an online service that offers various music-related features. You can record the\
 music you listen to on different streaming platforms and thus create a personalized music profile\
, providing recommendations of songs and artists based on your tastes.\n\n
<b>/setuser —</b> Sets your last.fm username.
<b>/lastfm|/lt —</b> Shows the song you are scrobbling on last.fm.
<b>/lalbum|/lalb —</b> Shows the album you are scrobbling on last.fm.
<b>/lartist|/lart —</b> Shows the artist you are scrobbling on last.fm.
"""
)
