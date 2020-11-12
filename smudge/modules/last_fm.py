import re
import requests
import urllib.request
import urllib.parse
import lyricsgenius
from telegraph import Telegraph

from telegram import Bot, Update, ParseMode
from telegram.ext import run_async, CommandHandler

import smudge.modules.sql.last_fm_sql as sql
from smudge import dispatcher, LASTFM_API_KEY, GENIUS
from smudge.modules.translations.strings import tld
from smudge.modules.disable import DisableAbleCommandHandler

@run_async
def set_user(bot: Bot, update: Update, args):
    msg = update.effective_message
    chat = update.effective_chat
    if args:
        user = update.effective_user.id
        username = " ".join(args)
        sql.set_user(user, username)
        msg.reply_text(tld(chat.id, "setuser_lastfm").format(username))
    else:
        msg.reply_text(
            tld(chat.id, "setuser_lastfm_error"))


@run_async
def clear_user(bot: Bot, update: Update):
    user = update.effective_user.id
    chat = update.effective_chat
    sql.set_user(user, "")
    update.effective_message.reply_text(
        tld(chat.id, "learuser_lastfm"))


@run_async
def last_fm(bot: Bot, update: Update):
    msg = update.effective_message
    user = update.effective_user.first_name
    user_id = update.effective_user.id
    username = sql.get_user(user_id)
    chat = update.effective_chat
    if not username:
        msg.reply_text(tld(chat.id, "lastfm_usernotset"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = requests.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json")
    if not res.status_code == 200:
        msg.reply_text(tld(chat.id, "lastfm_userwrong"))
        return

    try:
        first_track = res.json().get("recenttracks").get("track")[0]
    except IndexError:
        msg.reply_text(tld(chat.id, "lastfm_nonetracks"))
        return
    if first_track.get("@attr"):
        # Ensures the track is now playing
        image = first_track.get("image")[3].get("#text")  # Grab URL of 300x300 image
        artist = first_track.get("artist").get("name")
        artist1 = urllib.parse.quote(artist)
        song = first_track.get("name")
        song1 = album = urllib.parse.quote(song)
        loved = int(first_track.get("loved"))
        last_user = requests.get(
            f"{base_url}?method=track.getinfo&artist={artist1}&track={song1}&user={username}&api_key={LASTFM_API_KEY}&format=json").json().get("track")
        scrobbles = int(last_user.get("userplaycount"))+1
        rep = tld(chat.id, "lastfm_listening").format(user, scrobbles)
        if not loved:
            rep += tld(chat.id, "lastfm_scrb").format(artist, song)
        else:
            rep += tld(chat.id, "lastfm_scrb_loved").format(artist, song)
        if image:
            rep += f"<a href='{image}'>\u200c</a>"
    else:
        tracks = res.json().get("recenttracks").get("track")
        track_dict = {tracks[i].get("artist").get("name"): tracks[i].get("name") for i in range(3)}
        rep = tld(chat.id, "lastfm_old_scrb").format(user)
        for artist, song in track_dict.items():
            rep += tld(chat.id, "lastfm_scrr").format(artist, song)
        last_user = requests.get(
            f"{base_url}?method=user.getinfo&user={username}&api_key={LASTFM_API_KEY}&format=json").json().get("user")
        scrobbles = last_user.get("playcount")
        rep += tld(chat.id, "lastfm_scr").format(scrobbles)

    msg.reply_text(rep, parse_mode=ParseMode.HTML)

@run_async
def album(bot: Bot, update: Update):
    msg = update.effective_message
    user = update.effective_user.first_name
    user_id = update.effective_user.id
    username = sql.get_user(user_id)
    chat = update.effective_chat
    if not username:
        msg.reply_text(tld(chat.id, "lastfm_usernotset"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = requests.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json")
    if not res.status_code == 200:
        msg.reply_text(tld(chat.id, "lastfm_userwrong"))
        return

    try:
        first_track = res.json().get("recenttracks").get("track")[0]
    except IndexError:
        msg.reply_text(tld(chat.id, "lastfm_nonetracks"))
        return
    if first_track.get("@attr"):
        # Ensures the track is now playing
        artist = first_track.get("artist").get("name")
        artist1 = album = urllib.parse.quote(artist)
        album = first_track.get("album").get("#text")
        album1 = urllib.parse.quote(album)
        loved = int(first_track.get("loved"))
        album_info = requests.get(
            f"{base_url}?method=album.getinfo&album={album1}&artist={artist1}&user={username}&api_key={LASTFM_API_KEY}&format=json").json().get("album")
        scrobbles = int(album_info.get("userplaycount"))+1
        image = first_track.get("image")[3].get("#text")
        rep = tld(chat.id, "lastfm_listening_album").format(user, scrobbles)
        if not loved:
            rep += tld(chat.id, "lastfm_scrb_album").format(artist, album)
        else:
            rep += tld(chat.id, "lastfm_scrb_album").format(artist, album)
        if image:
            rep += f"<a href='{image}'>\u200c</a>"
    else:
        msg.reply_text(tld(chat.id, "lastfm_nonetracks_now"))

    msg.reply_text(rep, parse_mode=ParseMode.HTML)

@run_async
def artist(bot: Bot, update: Update):
    msg = update.effective_message
    user = update.effective_user.first_name
    user_id = update.effective_user.id
    username = sql.get_user(user_id)
    chat = update.effective_chat
    if not username:
        msg.reply_text(tld(chat.id, "lastfm_usernotset"))
        return

    base_url = "http://ws.audioscrobbler.com/2.0"
    res = requests.get(
        f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json")
    if not res.status_code == 200:
        msg.reply_text(tld(chat.id, "lastfm_userwrong"))
        return

    try:
        first_track = res.json().get("recenttracks").get("track")[0]
    except IndexError:
        msg.reply_text(tld(chat.id, "lastfm_nonetracks"))
        return
    if first_track.get("@attr"):
        # Ensures the track is now playing
        artist = first_track.get("artist").get("name")
        song = first_track.get("name")
        album = first_track.get("album").get("#text")
        loved = int(first_track.get("loved"))
        album_info = requests.get(
            f"{base_url}?method=artist.getinfo&artist={artist}&autocorrect=1&user={username}&api_key={LASTFM_API_KEY}&format=json").json().get("artist").get("stats")
        scrobbles = int(album_info.get("userplaycount"))+1
        image = first_track.get("image")[3].get("#text")
        rep = tld(chat.id, "lastfm_listening").format(user, scrobbles)
        if not loved:
            rep += tld(chat.id, "lastfm_scrb_artist").format(artist)
        else:
            rep += tld(chat.id, "lastfm_scrb_artist").format(artist)
        if image:
            rep += f"<a href='{image}'>\u200c</a>"
    else:
        msg.reply_text(tld(chat.id, "lastfm_nonetracks_now"))

    msg.reply_text(rep, parse_mode=ParseMode.HTML)
    
@run_async
def collage(bot: Bot, update: Update):
    user_id = update.effective_user.id
    user = update.effective_user.first_name
    username = sql.get_user(user_id)
    chat = update.effective_chat
    filename = "test.png"
    image_url = f"https://www.tapmusic.net/collage.php?user={username}&type=7day&size=4x4&caption=true&playcount=true"
    if not username:
        msg.reply_text(tld(chat.id, "lastfm_usernotset"))
        return
     
    else:
        urllib.request.urlretrieve(image_url, filename)
        bot.send_photo(chat_id=chat.id,  photo=open('test.png', 'rb'), caption= tld(chat.id, "lastfm_collage").format(user))

@run_async
def lyrics(bot: Bot, update: Update, args):
    msg = update.effective_message
    user = update.effective_user.first_name
    user_id = update.effective_user.id
    username = sql.get_user(user_id)
    telegraph = Telegraph()
    telegraph.create_account(short_name='SmudgeLord', author_name='Renatoh')
    genius = lyricsgenius.Genius(GENIUS)
    chat = update.effective_chat
    if args:
        search = " ".join(args)
        songs = genius.search_song(search)
        if songs is None:
            msg.reply_text("Sem resultados")
        else:
            page_content = songs.lyrics.replace("\n", "<br class='inline'>")
            response = telegraph.create_page(
                f'{search}',
                author_name="SmudgeLordBot",
                html_content=(f"<p> {page_content} </p>")
            )
            lyricsurl = ('https://telegra.ph/{}'.format(response['path']))
            msg.reply_text(lyricsurl, parse_mode=ParseMode.HTML)
    else:
        if not username:
            msg.reply_text(tld(chat.id, "lastfm_usernotset"))
            return
        base_url = "http://ws.audioscrobbler.com/2.0"
        res = requests.get(
            f"{base_url}?method=user.getrecenttracks&limit=3&extended=1&user={username}&api_key={LASTFM_API_KEY}&format=json")
        if not res.status_code == 200:
            msg.reply_text(tld(chat.id, "lastfm_userwrong"))
            return
        
        try:
            first_track = res.json().get("recenttracks").get("track")[0]
        except IndexError:
            msg.reply_text(tld(chat.id, "lastfm_nonetracks"))
            return
        artist = first_track.get("artist").get("name")
        song = first_track.get("name")
        songs = genius.search_song(song, artist)
        #name_song = tld(chat.id, "song_test").format(artist, song, songs.lyrics)
        if songs is None:
            msg.reply_text(tld(chat.id, "lyrics_not_found").format(song, artist))
        else:
            page_content = songs.lyrics.replace("\n", "<br class='inline'>")
            response = telegraph.create_page(
                f'{song} - {artist}',
                author_name="SmudgeLordBot",
                html_content=(f"<p> {page_content} </p>")
            )
            lyricsurl = ('https://telegra.ph/{}'.format(response['path']))
            msg.reply_text(lyricsurl, parse_mode=ParseMode.HTML)
    
__help__ = """
Share what you're what listening to with the help of this module!

*Available commands:*
 - /setuser <username>: sets your last.fm username.
 - /clearuser: removes your last.fm username from the bot's database.
 - /lastfm: returns what you're scrobbling on last.fm.
 - /toptracks: returns the songs you most listened to.
"""

__mod_name__ = "Last.FM"
    

SET_USER_HANDLER = CommandHandler("setuser", set_user, pass_args=True)
CLEAR_USER_HANDLER = CommandHandler("clearuser", clear_user)
LASTFM_HANDLER = DisableAbleCommandHandler(["lastfm", "lt", "last", "l"], last_fm)
LYRICS_HANDLER = CommandHandler("lyrics", lyrics, pass_args=True)
ALBUM_HANDLER = DisableAbleCommandHandler(["album", "albuns"], album)
ARTIST_HANDLER = DisableAbleCommandHandler("artist", artist)
#COLLAGE_HANDLER = CommandHandler("collage", collage)

#dispatcher.add_handler(COLLAGE_HANDLER)
dispatcher.add_handler(SET_USER_HANDLER)
dispatcher.add_handler(CLEAR_USER_HANDLER)
dispatcher.add_handler(LASTFM_HANDLER)
dispatcher.add_handler(LYRICS_HANDLER)
dispatcher.add_handler(ALBUM_HANDLER)
dispatcher.add_handler(ARTIST_HANDLER)
