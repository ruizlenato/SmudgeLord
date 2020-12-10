import html
from typing import Optional, List

from telegram import Message, Chat, Update, Bot, User, ParseMode
from telegram.error import BadRequest, Unauthorized
from telegram.ext import CommandHandler, RegexHandler, run_async, Filters
from telegram.utils.helpers import mention_html

from smudge import dispatcher, CallbackContext, LOGGER
from smudge.helper_funcs.chat_status import user_not_admin, user_admin
from smudge.modules.log_channel import loggable
from smudge.modules.sql import reporting_sql as sql
from smudge.modules.translations.strings import tld

REPORT_GROUPS = 5


@user_admin
def report_setting(update: Update, context: CallbackContext):
    bot = context.bot
    args = context.args
    chat = update.effective_chat  # type: Optional[Chat]
    msg = update.effective_message  # type: Optional[Message]

    if chat.type == chat.PRIVATE:
        if len(args) >= 1:
            if args[0] in ("yes", "on"):
                sql.set_user_setting(chat.id, True)
                msg.reply_text(
                    "Turned on reporting! You'll be notified whenever anyone reports something."
                )

            elif args[0] in ("no", "off"):
                sql.set_user_setting(chat.id, False)
                msg.reply_text(
                    "Turned off reporting! You wont get any reports.")
        else:
            msg.reply_text("Your current report preference is: `{}`".format(
                sql.user_should_report(chat.id)),
                           parse_mode=ParseMode.MARKDOWN)

    else:
        if len(args) >= 1:
            if args[0] in ("yes", "on"):
                sql.set_chat_setting(chat.id, True)
                msg.reply_text(
                    "Turned on reporting! Admins who have turned on reports will be notified when /report "
                    "or @admin are called.")

            elif args[0] in ("no", "off"):
                sql.set_chat_setting(chat.id, False)
                msg.reply_text(
                    "Turned off reporting! No admins will be notified on /report or @admin."
                )
        else:
            msg.reply_text("This chat's current setting is: `{}`".format(
                sql.chat_should_report(chat.id)),
                           parse_mode=ParseMode.MARKDOWN)


@user_not_admin
@loggable
def report(update: Update, context: CallbackContext) -> str:
    bot = context.bot
    message = update.effective_message  # type: Optional[Message]
    chat = update.effective_chat  # type: Optional[Chat]
    user = update.effective_user  # type: Optional[User]
    ping_list = ""

    if chat and message.reply_to_message and sql.chat_should_report(chat.id):
        reported_user = message.reply_to_message.from_user  # type: Optional[User]
        if reported_user.id == bot.id:
            message.reply_text("Haha nope, not gonna report myself.")
            return ""
        chat_name = chat.title or chat.first or chat.username
        admin_list = chat.get_administrators()

        for admin in admin_list:
            if admin.user.is_bot:  # can't message bots
                continue

            ping_list += f"​[​](tg://user?id={admin.user.id})"

        message.reply_text(
            f"Successfully reported [{reported_user.first_name}](tg://user?id={reported_user.id}) to admins! "
            + ping_list,
            parse_mode=ParseMode.MARKDOWN)

    return ""


def __migrate__(old_chat_id, new_chat_id):
    sql.migrate_chat(old_chat_id, new_chat_id)


def __chat_settings__(chat_id, user_id):
    return "This chat is setup to send user reports to admins, via /report and @admin: `{}`".format(
        sql.chat_should_report(chat_id))


def __user_settings__(user_id):
    return "You receive reports from chats you're admin in: `{}`.\nToggle this with /reports in PM.".format(
        sql.user_should_report(user_id))


__help__ = True

REPORT_HANDLER = CommandHandler("report",
                                report,
                                filters=Filters.chat_type.groups,
                                run_async=True)
SETTING_HANDLER = CommandHandler("reports", report_setting, run_async=True)
ADMIN_REPORT_HANDLER = RegexHandler("(?i)@admin(s)?", report, run_async=True)

dispatcher.add_handler(REPORT_HANDLER, group=REPORT_GROUPS)
dispatcher.add_handler(ADMIN_REPORT_HANDLER, group=REPORT_GROUPS)
dispatcher.add_handler(SETTING_HANDLER)
