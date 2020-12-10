import datetime
import importlib
from sys import argv
import re
from typing import Optional, List

from telegram import Message, Chat, Update, Bot, User
from telegram import ParseMode, InlineKeyboardMarkup, InlineKeyboardButton
from telegram.error import Unauthorized, BadRequest, TimedOut, NetworkError, ChatMigrated, TelegramError
from telegram.ext import CommandHandler, Filters, MessageHandler, CallbackQueryHandler
from telegram.ext.dispatcher import run_async, DispatcherHandlerStop, Dispatcher
from telegram.utils.helpers import escape_markdown
from smudge.modules.translations.strings import tld

from smudge import dispatcher, updater, CallbackContext, TOKEN, OWNER_ID, LOGGER, ALLOW_EXCL, tbot
# needed to dynamically load modules
# NOTE: Module order is not guaranteed, specify that in the config file!
from smudge.modules import ALL_MODULES
from smudge.helper_funcs.chat_status import is_user_admin
from smudge.helper_funcs.misc import paginate_modules

IMPORTED = {}
MIGRATEABLE = []
HELPABLE = {}
STATS = []
USER_INFO = []
DATA_IMPORT = []
DATA_EXPORT = []

GDPR = []

for module_name in ALL_MODULES:
    imported_module = importlib.import_module("smudge.modules." + module_name)
    modname = imported_module.__name__.split('.')[2]

    if not modname.lower() in IMPORTED:
        IMPORTED[modname.lower()] = imported_module
    else:
        raise Exception(
            "Can't have two modules with the same name! Please change one")

    if hasattr(imported_module, "__help__") and imported_module.__help__:
        HELPABLE[modname.lower()] = tld(0, "modname_" + modname).strip()

    # Chats to migrate on chat_migrated events
    if hasattr(imported_module, "__migrate__"):
        MIGRATEABLE.append(imported_module)

    if hasattr(imported_module, "__stats__"):
        STATS.append(imported_module)

    if hasattr(imported_module, "__gdpr__"):
        GDPR.append(imported_module)

    if hasattr(imported_module, "__user_info__"):
        USER_INFO.append(imported_module)

    if hasattr(imported_module, "__import_data__"):
        DATA_IMPORT.append(imported_module)

    if hasattr(imported_module, "__export_data__"):
        DATA_EXPORT.append(imported_module)


# do not async
def send_help(chat_id, text, keyboard=None):
    if not keyboard:
        keyboard = InlineKeyboardMarkup(
            paginate_modules(chat_id, 0, HELPABLE, "help"))        
    dispatcher.bot.send_message(chat_id=chat_id,
                                text=text,
                                parse_mode=ParseMode.MARKDOWN,
                                reply_markup=keyboard)



def start(update: Update, context: CallbackContext):
    chat = update.effective_chat
    args = context.args
    if update.effective_chat.type == "private":
        if len(args) >= 1:
            if args[0].lower() == "help":
                send_help(
                    update.effective_chat.id,
                    tld(chat.id,
                        "send-help").format(context.bot.first_name,
                                            tld(chat.id, "cmd_multitrigger")))

            elif args[0][1:].isdigit() and "rules" in IMPORTED:
                IMPORTED["rules"].send_rules(update, args[0], from_pm=True)

        else:
            send_start(update, context)
    else:
        try:
            update.effective_message.reply_text(tld(chat.id, 'main_start_group'))
        except Exception:
            print("Nut")


def send_start(update: Update, context: CallbackContext):
    chat = update.effective_chat  # type: Optional[Chat]
    # Try to remove old message
    try:
        query = update.callback_query
        query.message.delete()
    except:
        pass

    # chat = update.effective_chat  # type: Optional[Chat] and unused variable
    text = tld(chat.id, 'main_start_pm')

    keyboard = [[
        InlineKeyboardButton(text=tld(chat.id, 'main_start_btn_news'),
                             url="https://t.me/SmudgeNews")
    ]]
    keyboard += [[
        InlineKeyboardButton(
            text=tld(chat.id, 'main_start_btn_lang'), callback_data="set_lang_"),
        InlineKeyboardButton(text=tld(chat.id, 'btn_help'),
                             callback_data="help_back")
    ]]

    update.effective_message.reply_text(
        text,
        reply_markup=InlineKeyboardMarkup(keyboard),
        parse_mode=ParseMode.MARKDOWN,
        disable_web_page_preview=True)
# for test purposes



# for test purposes
def error_callback(update, context):
    bot = context.bot
    error = context.error
    try:
        raise error
    except Unauthorized:
        print("no nono1")
        print(error)
        # remove update.message.chat_id from conversation list
    except BadRequest:
        print("no nono2")
        print("BadRequest caught")
        print(error)

        # handle malformed requests - read more below!
    except TimedOut:
        print("no nono3")
        # handle slow connection problems
    except NetworkError:
        print("no nono4")
        # handle other connection problems
    except ChatMigrated as err:
        print("no nono5")
        print(err)
        # the chat_id of a group has changed, use e.new_chat_id instead
    except TelegramError:
        print(error)
        # handle all other telegram related errors


def help_button(update, context):
    bot = context.bot
    query = update.callback_query
    chat = update.effective_chat
    back_match = re.match(r"help_back", query.data)
    mod_match = re.match(r"help_module\((.+?)\)", query.data)
    try:
        if mod_match:
            module = mod_match.group(1)
            mod_name = tld(chat.id, "modname_" + module).strip()
            help_txt = tld(
                chat.id, module +
                "_help")  # tld_help(chat.id, HELPABLE[module].__mod_name__)

            if not help_txt:
                LOGGER.exception(f"Help string for {module} not found!")

            text = tld(chat.id, "here_is_help").format(mod_name, help_txt)

            bot.edit_message_text(chat_id=query.message.chat_id,
                                  message_id=query.message.message_id,
                                  text=text,
                                  parse_mode=ParseMode.MARKDOWN,
                                  reply_markup=InlineKeyboardMarkup([[
                                      InlineKeyboardButton(
                                          text=tld(chat.id, "btn_go_back"),
                                          callback_data="help_back")
                                  ]]),
                                  disable_web_page_preview=True)

        elif back_match:
            bot.edit_message_text(chat_id=query.message.chat_id,
                                  message_id=query.message.message_id,
                                  text=tld(chat.id, "send-help").format(
                                      dispatcher.bot.first_name,
                                      tld(chat.id, "cmd_multitrigger")),
                                  parse_mode=ParseMode.MARKDOWN,
                                  reply_markup=InlineKeyboardMarkup(
                                      paginate_modules(chat.id, 0, HELPABLE,
                                                       "help")),
                                  disable_web_page_preview=True)

    except BadRequest:
        pass



def get_help(update: Update, context: CallbackContext):
    chat = update.effective_chat
    args = update.effective_message.text.split(None, 1)
    bot = context.bot

    # ONLY send help in PM
    if chat.type != chat.PRIVATE:
        update.effective_message.reply_text(
            tld(chat.id, 'help_pm_only'),
            reply_markup=InlineKeyboardMarkup([[
                InlineKeyboardButton(text=tld(chat.id, 'btn_help'),
                                     url="t.me/{}?start=help".format(
                                         bot.username))
            ]]))
        return

    if len(args) >= 2:
        mod_name = None
        for x in HELPABLE:
            if args[1].lower() == HELPABLE[x].lower():
                mod_name = tld(chat.id, "modname_" + x).strip()
                module = x
                break

        if mod_name:
            help_txt = tld(chat.id, module + "_help")

            if not help_txt:
                LOGGER.exception(f"Help string for {module} not found!")

            text = tld(chat.id, "here_is_help").format(mod_name, help_txt)
            send_help(
                chat.id, text,
                InlineKeyboardMarkup([[
                    InlineKeyboardButton(text=tld(chat.id, 'main_start_btn_news'),
                                         url="https://t.me/SmudgeNews")
                ]]))

            return

        update.effective_message.reply_text(tld(
            chat.id, "help_not_found").format(args[1]),
            parse_mode=ParseMode.HTML)
        return

    send_help(
        chat.id,
        tld(chat.id, "send-help").format(dispatcher.bot.first_name,
                                         tld(chat.id, "cmd_multitrigger")))

def migrate_chats(update: Update, context: CallbackContext):
    bot = context.bot
    msg = update.effective_message  # type: Optional[Message]
    if msg.migrate_to_chat_id:
        old_chat = update.effective_chat.id
        new_chat = msg.migrate_to_chat_id
    elif msg.migrate_from_chat_id:
        old_chat = msg.migrate_from_chat_id
        new_chat = update.effective_chat.id
    else:
        return

    LOGGER.info("Migrating from %s, to %s", str(old_chat), str(new_chat))
    for mod in MIGRATEABLE:
        mod.__migrate__(old_chat, new_chat)

    LOGGER.info("Successfully migrated!")
    raise DispatcherHandlerStop


def main():
    # test_handler = CommandHandler("test", test) #Unused variable
    start_handler = CommandHandler("start", start, pass_args=True, run_async=True)

    help_handler = CommandHandler("help", get_help, run_async=True)
    help_callback_handler = CallbackQueryHandler(help_button, pattern=r"help_", run_async=True)

    start_callback_handler = CallbackQueryHandler(send_start,
                                                  pattern=r"bot_start", run_async=True)

    migrate_handler = MessageHandler(Filters.status_update.migrate,
                                     migrate_chats, run_async=True)

    # dispatcher.add_handler(test_handler)
    dispatcher.add_handler(start_handler)
    dispatcher.add_handler(start_callback_handler)
    dispatcher.add_handler(help_handler)
    dispatcher.add_handler(help_callback_handler)
    dispatcher.add_handler(migrate_handler)
    # dispatcher.add_error_handler(error_callback)

    LOGGER.info("Using long polling.")
    # updater.start_polling(timeout=15, read_latency=4, clean=True)
    updater.start_polling(poll_interval=0.0,
                          timeout=10,
                          clean=True,
                          bootstrap_retries=-1,
                          read_latency=3.0)
    LOGGER.info("\n.....................................................................\n.....................................................................\n....... MMMM..............................................MMMM.......\n........ MMWMMM........................................MMWMMM .......\n........  MMMMMMM..................................MMWMMMMWN ........\n.......... MNWMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMWNXNWMMMMMMMWNN .........\n........... MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMWNN ..........\n............ MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMNN ...........\n............ MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ............\n..........  MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMWNM .............\n.......... MMMMMMMMMMMMMMMMMMMMMMMMWMMMMMMMMMMMMMMMMMM ..............\n.......... MMMk....MMMMMMMMMM......MMMMMMMMMMMMMMMMMM ...............\n......... OMMMM....MMMMMMMMMM.....MMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMMMMMMM..MWMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMMMMMMM..MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMWWMMMM  dNMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMN....MMM:..MMMMMMMMWMMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMMMMMMMMMMM.....MMMMMMMMMMMMMMMMMMMMMMMMM ..............\n......... MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n.......... MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n.......... MMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMMM ..............\n.....................................................................")
    LOGGER.info("Successfully loaded")
    if len(argv) not in (1, 3, 4):
        tbot.disconnect()
    else:
        tbot.run_until_disconnected()

    updater.idle()


CHATS_CNT = {}
CHATS_TIME = {}

if __name__ == '__main__':
    LOGGER.info("Successfully loaded modules: " + str(ALL_MODULES))
    tbot.start(bot_token=TOKEN)
    main()

