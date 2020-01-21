import html
from io import BytesIO
from typing import Optional, List

from telegram import Message, Update, Bot, User, Chat, ParseMode
from telegram.error import BadRequest, TelegramError
from telegram.ext import run_async, CommandHandler, MessageHandler, Filters
from telegram.utils.helpers import mention_html

import smudge.modules.sql.antispam_sql as sql
from smudge import dispatcher, OWNER_ID, SUDO_USERS, SUPPORT_USERS, MESSAGE_DUMP, STRICT_ANTISPAM
from smudge.modules.helper_funcs.chat_status import user_admin, is_user_admin
from smudge.modules.helper_funcs.extraction import extract_user, extract_user_and_text
from smudge.modules.helper_funcs.filters import CustomFilters
from smudge.modules.sql.users_sql import get_all_chats
from smudge.modules.translations.strings import tld

GBAN_ENFORCE_GROUP = 6

GBAN_ERRORS = {
    "User is an administrator of the chat",
    "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant",
    "Peer_id_invalid",
    "Group chat was deactivated",
    "Need to be inviter of a user to kick it from a basic group",
    "Chat_admin_required",
    "Only the creator of a basic group can kick group administrators",
    "Channel_private",
    "Not in the chat"
}

UNGBAN_ERRORS = {
    "User is an administrator of the chat",
    "Chat not found",
    "Not enough rights to restrict/unrestrict chat member",
    "User_not_participant",
    "Method is available for supergroup and channel chats only",
    "Not in the chat",
    "Channel_private",
    "Chat_admin_required",
    "User not found",
}


@run_async
def gban(bot: Bot, update: Update, args: List[str]):
    message = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    user_id, reason = extract_user_and_text(message, args)

    if not user_id or int(user_id)==777000:
        message.reply_text("You don't seem to be referring to a user.")
        return

    if int(user_id) in SUDO_USERS:
        message.reply_text(tld(chat.id, "I spy with my little eyes... a sudo user war! Why are you guys turning on each other?"))
        return

    if int(user_id) in SUPPORT_USERS:
        message.reply_text(tld(chat.id, "OOOH, someone's trying to gban a support user! *grabs popcorn*"))
        return

    if user_id == bot.id:
        message.reply_text(tld(chat.id, "-_- So funny, let me gban myself, why don't I? Nice try."))
        return

    try:
        user_chat = bot.get_chat(user_id)
    except BadRequest as excp:
        message.reply_text(excp.message)
        return

    if user_chat.type != 'private':
        message.reply_text(tld(chat.id, "That's not a user!"))
        return

    if user_chat.first_name == '':
        message.reply_text(tld(chat.id, "That's a deleted account! Why even bother gbanning them?"))
        return

    if sql.is_user_gbanned(user_id):
        if not reason:
            message.reply_text(tld(chat.id, "This user is already gbanned. I'd change the reason, but you haven't given me one..."))
            return

        old_reason = sql.update_gban_reason(user_id, user_chat.username or user_chat.first_name, reason)
        user_id, new_reason = extract_user_and_text(message, args)

        if old_reason:
            banner = update.effective_user  # type: Optional[User]
            bannerid = banner.id

            if int(bannerid) in [172811422, 214416808]:
                return

            bannername = banner.first_name
            new_reason = f"{new_reason}"

            bot.send_message(
                MESSAGE_DUMP,
                "<b>GBan Reason Update</b>\n" \
                "#GBAN" \
                "\n<b>Sudo Admin:</b> {}" \
                "\n<b>User:</b> {}" \
                "\n<b>ID:</b> <code>{}</code>" \
                "\n<b>Previous Reason:</b> {}" \
                "\n<b>New Reason:</b> {}".format(mention_html(banner.id, banner.first_name),
                                                 mention_html(user_chat.id, user_chat.first_name or "Deleted Account"),
                                                 user_chat.id, old_reason, new_reason),
                parse_mode=ParseMode.HTML
            )

            message.reply_text("This user is already gbanned, for the following reason:\n"
                               "<code>{}</code>\n"
                               "I've gone and updated it with the new reason!".format(html.escape(old_reason)),
                               parse_mode=ParseMode.HTML)
        else:
            banner = update.effective_user  # type: Optional[User]
            bannerid = banner.id

            if int(bannerid) in [172811422, 214416808]:
                return

            bannername = banner.first_name
            new_reason = f"{new_reason}"

            bot.send_message(
                MESSAGE_DUMP,
                "<b>GBan Reason Update</b>\n" \
                "#GBAN" \
                "\n<b>Sudo Admin:</b> {}" \
                "\n<b>User:</b> {}" \
                "\n<b>ID:</b> <code>{}</code>" \
                "\n<b>New Reason:</b> {}".format(mention_html(banner.id, banner.first_name or "Deleted Account"),
                                                 mention_html(user_chat.id, user_chat.first_name),
                                                 user_chat.id, new_reason),
                parse_mode=ParseMode.HTML
            )
            message.reply_text(tld(chat.id, "This user is already gbanned, but had no reason set; I've gone and updated it!"))

        return

    starting = "Global Banning {} with the id <code>{}</code>".format(
        mention_html(user_chat.id, user_chat.first_name or "Deleted Account"), user_chat.id)
    message.reply_text(starting, parse_mode=ParseMode.HTML)

    banner = update.effective_user  # type: Optional[User]
    bannerid = banner.id
    bannername = banner.first_name
    reason = f"{reason}"
    try:
        bot.send_message(
            MESSAGE_DUMP,
            "<b>New Global Ban</b>\n#GBAN\n<b>Sudo Admin:</b> {}\n<b>User:</b> {}\n<b>ID:</b> <code>{}</code> "
            "\n<b>Reason:</b> {}".format(mention_html(banner.id, banner.first_name),
                                  mention_html(user_chat.id, user_chat.first_name), user_chat.id,
                                  reason or "No reason given"),
            parse_mode=ParseMode.HTML
        )
    except TelegramError:
        print("nut")

    sql.gban_user(user_id, user_chat.username or user_chat.first_name, reason)

    try:
        if int(bannerid) in [302253890]:
            return
        chat.kick_member(user_chat.id)
    except TelegramError:
        print("Meh")


@run_async
def ungban(bot: Bot, update: Update, args: List[str]):
    message = update.effective_message  # type: Optional[Message]

    user_id = extract_user(message, args)
    if not user_id or int(user_id)==777000:
        message.reply_text("You don't seem to be referring to a user.")
        return

    user_chat = bot.get_chat(user_id)
    if user_chat.type != 'private':
        message.reply_text("That's not a user!")
        return

    if not sql.is_user_gbanned(user_id):
        message.reply_text("This user is not gbanned!")
        return

    banner = update.effective_user  # type: Optional[User]

    # message.reply_text("{}, will be unbanned globally.".format(user_chat.first_name or "Deleted Account"))

    bot.send_message(MESSAGE_DUMP,
                 "<b>Regression of Global Ban</b>" \
                 "\n#UNGBAN" \
                 "\n<b>Sudo Admin:</b> {}" \
                 "\n<b>User:</b> {}" \
                 "\n<b>ID:</b> <code>{}</code>".format(mention_html(banner.id, banner.first_name),
                                                       mention_html(user_chat.id, user_chat.first_name or "Deleted Account"), 
                                                                    user_chat.id),
                 parse_mode=ParseMode.HTML)

    chats = get_all_chats()
    for chat in chats:
        chat_id = chat.chat_id

        # Check if this group has disabled gbans
        if not sql.does_chat_gban(chat_id):
            continue

        try:
            member = bot.get_chat_member(chat_id, user_id)
            if member.status == 'kicked':
                bot.unban_chat_member(chat_id, user_id)

        except BadRequest as excp:
            if excp.message in UNGBAN_ERRORS:
                pass
            else:
                message.reply_text("Could not un-gban due to: {}".format(excp.message))
                bot.send_message(OWNER_ID, "Could not un-gban due to: {}".format(excp.message))
                return
        except TelegramError:
            pass

    sql.ungban_user(user_id)

    send_to_list(bot, SUDO_USERS + SUPPORT_USERS, 
                  "{} has been unbanned globally!".format(mention_html(user_chat.id, 
                                                                         user_chat.first_name or "Deleted Account")),
                  html=True)

    message.reply_text("Person has been un-gbanned.")
                                                           

@run_async
def gbanlist(bot: Bot, update: Update):
    banned_users = sql.get_gban_list()

    if not banned_users:
        update.effective_message.reply_text("There aren't any gbanned users! You're kinder than I expected...")
        return

    banfile = 'Screw these guys.\n'
    for user in banned_users:
        banfile += "[x] {} - {}\n".format(user["name"], user["user_id"])
        if user["reason"]:
            banfile += "Reason: {}\n".format(user["reason"])

    with BytesIO(str.encode(banfile)) as output:
        output.name = "gbanlist.txt"
        update.effective_message.reply_document(document=output, filename="gbanlist@Lordsmudge_BOT.txt",
                                                caption="Here is the list of currently gbanned users.")


def check_and_ban(update, user_id, should_message=True):
    if sql.is_user_gbanned(user_id):
        update.effective_chat.kick_member(user_id)
        if should_message:
            userr = sql.get_gbanned_user(user_id)
            usrreason = userr.reason
            if not usrreason:
                usrreason = "No reason given"

            update.effective_message.reply_text(f"*This user is gbanned and has been removed.*\nReason: `{usrreason}`",
                                                parse_mode=ParseMode.MARKDOWN)


@run_async
def enforce_gban(bot: Bot, update: Update):
    # Not using @restrict handler to avoid spamming - just ignore if cant gban.
    try:
        if sql.does_chat_gban(update.effective_chat.id) and update.effective_chat.get_member(
                bot.id).can_restrict_members:
            user = update.effective_user  # type: Optional[User]
            chat = update.effective_chat  # type: Optional[Chat]
            msg = update.effective_message  # type: Optional[Message]

            if user and not is_user_admin(chat, user.id):
                check_and_ban(update, user.id)
                return

            if msg.new_chat_members:
                new_members = update.effective_message.new_chat_members
                for mem in new_members:
                    check_and_ban(update, mem.id)
                    return

            if msg.reply_to_message:
                user = msg.reply_to_message.from_user  # type: Optional[User]
                if user and not is_user_admin(chat, user.id):
                    check_and_ban(update, user.id, should_message=False)
                    return
    except:
        print("Nut")


@run_async
@user_admin
def antispam(bot: Bot, update: Update, args: List[str]):
    chat = update.effective_chat  # type: Optional[Chat]
    if len(args) > 0:
        if args[0].lower() in ["on", "yes"]:
            sql.enable_antispam(chat.id)
            update.effective_message.reply_text(
                tld(chat.id, "I've enabled antispam security in this group. This will help to protect you "
                             "from spammers, unsavoury characters, and the biggest trolls."))
        elif args[0].lower() in ["off", "no"]:
            sql.disable_antispam(chat.id)
            update.effective_message.reply_text(
                tld(chat.id, "I've disabled antispam security in this group. GBans won't affect your users "
                             "anymore. You'll be less protected from any trolls and spammers "
                             "though!"))
    else:
        update.effective_message.reply_text(
            tld(chat.id, "Give me some arguments to choose a setting! on/off, yes/no!\n\n"
                         "Your current setting is: {}\n"
                         "When True, any gbans that happen will also happen in your group. "
                         "When False, they won't, leaving you at the possible mercy of "
                         "spammers.").format(sql.does_chat_gban(chat.id)))


def __stats__():
    return "{} gbanned users.".format(sql.num_gbanned_users())


def __user_info__(user_id, chat_id):
    is_gbanned = sql.is_user_gbanned(user_id)
    is_gmuted = sql.is_user_gmuted(user_id)

    if not user_id in SUDO_USERS:

        text = tld(chat_id, "Globally banned: <b>{}</b>")
        if is_gbanned:
            text = text.format(tld(chat_id, "Yes"))
            user = sql.get_gbanned_user(user_id)
            if user.reason:
                text += tld(chat_id, "\nReason: {}").format(html.escape(user.reason))
        else:
            text = text.format(tld(chat_id, "No"))

        return text
    else:
        return ""


def __migrate__(old_chat_id, new_chat_id):
    sql.migrate_chat(old_chat_id, new_chat_id)


def __chat_settings__(bot, update, chat, chatP, user):
    chat_id = chat.id
    return "This chat is enforcing *gbans*: `{}`.".format(sql.does_chat_gban(chat_id))


__help__ = """
*Admin only:*
 - /antispam <on/off/yes/no>: Change antispam security settings in the group, or return your \
current settings(when no arguments).

Antispam is used by the bot owners to ban spammers across all groups. This helps protect \
you and your groups by removing spam flooders as quickly as possible. This is enabled by \
default, but you can change this by using the command.

*Combot Anti Spam System:*
You can improve your Antispam protection by using the /setcas command to enable new member verification based on the [Combot Anti Spam System](https://combot.org/cas)

*CAS Commands:*
 - /casver: Returns the API version that the bot is currently running
 - /cascheck: Checks you or another user for CAS BAN

*Admin Only:*
 - /setcas <on/off/true/false>: Enables/disables CAS Checking on welcome
 - /getcas: Gets the current CAS settings
 - /setban <on/off/true/false>: Enables/disables autoban on CAS banned user detected.

*Sudo Only:*
 - /gbanlist: Will give you the complete list of globally banned users

*Credits:*
Thanks to @nunopenim for providing his proprietary Combot Anti Spam System API - [(pyCombotCAS_API)](https://github.com/nunopenim/pyCombotCAS_API)
"""

__mod_name__ = "Antispam security"

ANTISPAM_STATUS = CommandHandler("antispam", antispam, pass_args=True, filters=Filters.group)

GBAN_HANDLER = CommandHandler(["gban", "fban"], gban, pass_args=True,
                              filters=CustomFilters.sudo_filter | CustomFilters.support_filter)
UNGBAN_HANDLER = CommandHandler("ungban", ungban, pass_args=True,
                                filters=CustomFilters.sudo_filter | CustomFilters.support_filter)
GBAN_LIST = CommandHandler("gbanlist", gbanlist, filters=CustomFilters.sudo_filter | CustomFilters.support_filter)
GBAN_ENFORCER = MessageHandler(Filters.all & Filters.group, enforce_gban)

dispatcher.add_handler(ANTISPAM_STATUS)

dispatcher.add_handler(GBAN_HANDLER)
dispatcher.add_handler(UNGBAN_HANDLER)
dispatcher.add_handler(GBAN_LIST)

if STRICT_ANTISPAM:  # enforce GBANS if this is set
    dispatcher.add_handler(GBAN_ENFORCER, GBAN_ENFORCE_GROUP)
