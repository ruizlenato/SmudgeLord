from typing import Union, List, Optional

from future.utils import string_types
from telegram import ParseMode, Update, Bot, Chat, User
from telegram.ext import CommandHandler, MessageHandler, Filters
from telegram.utils.helpers import escape_markdown

from smudge import dispatcher, CallbackContext
from smudge.helper_funcs.handlers import CMD_STARTERS
from smudge.helper_funcs.misc import is_module_loaded

FILENAME = __name__.rsplit(".", 1)[-1]

# If module is due to be loaded, then setup all the magical handlers
if is_module_loaded(FILENAME):
    from smudge.helper_funcs.chat_status import user_admin, is_user_admin
    from telegram.ext.dispatcher import run_async

    from smudge.modules.sql import disable_sql as sql

    DISABLE_CMDS = []
    DISABLE_OTHER = []
    ADMIN_CMDS = []

    class DisableAbleCommandHandler(CommandHandler):
        def __init__(self, command, callback, admin_ok=False, **kwargs):
            super().__init__(command, callback, **kwargs)
            self.admin_ok = admin_ok
            if isinstance(command, string_types):
                DISABLE_CMDS.append(command)
                if admin_ok:
                    ADMIN_CMDS.append(command)
            else:
                DISABLE_CMDS.extend(command)
                if admin_ok:
                    ADMIN_CMDS.extend(command)

        def check_update(self, update):
            chat = update.effective_chat  # type: Optional[Chat]
            user = update.effective_user  # type: Optional[User]
            message = update.effective_message
            if super().check_update(update):
                # Should be safe since check_update passed.
                command = update.effective_message.text_html.split(
                    None, 1)[0][1:].split('@')[0]

                # disabled, admincmd, user admin
                if sql.is_command_disabled(chat.id, command):
                    if command in ADMIN_CMDS and is_user_admin(chat, user.id):
                        pass
                    else:
                        return None

                args = message.text.split()[1:]
                filter_result = self.filters(update)
                if filter_result:
                    return args, filter_result
                else:
                    return False

            return None

    class DisableAbleRegexHandler(MessageHandler):
        def __init__(self, pattern, callback, friendly="", **kwargs):
            super().__init__(Filters.regex(pattern), callback, **kwargs)
            DISABLE_OTHER.append(friendly or pattern)
            self.friendly = friendly or pattern

        def check_update(self, update):
            chat = update.effective_chat
            return super().check_update(
                update) and not sql.is_command_disabled(
                    chat.id, self.friendly)

    @user_admin
    def disable(update: Update, context: CallbackContext):
        bot = context.bot
        args = context.args
        chat = update.effective_chat  # type: Optional[Chat]
        if len(args) >= 1:
            disable_cmd = args[0]
            if disable_cmd.startswith(CMD_STARTERS):
                disable_cmd = disable_cmd[1:]

            if disable_cmd in set(DISABLE_CMDS + DISABLE_OTHER):
                sql.disable_command(chat.id, disable_cmd)
                update.effective_message.reply_text(tld(chat.id, "disable_success").format(
                    disable_cmd), parse_mode=ParseMode.MARKDOWN)
            else:
                update.effective_message.reply_text(
                    tld(chat.id, "disable_err_undisableable"))

        else:
            update.effective_message.reply_text(
                tld(chat.id, "disable_err_no_cmd"))

    @user_admin
    def enable(update: Update, context: CallbackContext):
        bot = context.bot
        args = context.args
        chat = update.effective_chat  # type: Optional[Chat]
        if len(args) >= 1:
            enable_cmd = args[0]
            if enable_cmd.startswith(CMD_STARTERS):
                enable_cmd = enable_cmd[1:]

            if sql.enable_command(chat.id, enable_cmd):
                update.effective_message.reply_text(
                    tld(chat.id, "disable_enable_success").format(enable_cmd),
                    parse_mode=ParseMode.MARKDOWN)
            else:
                update.effective_message.reply_text(
                    tld(chat.id, "disable_already_enabled"))

        else:
            update.effective_message.reply_text(
                tld(chat.id, "disable_err_no_cmd"))

    @user_admin
    def list_cmds(update: Update, context: CallbackContext):
        bot = context.bot
        if DISABLE_CMDS + DISABLE_OTHER:
            result = ""
            for cmd in set(DISABLE_CMDS + DISABLE_OTHER):
                result += " - `{}`\n".format(escape_markdown(cmd))
            update.effective_message.reply_text(tld(chat.id, "disable_able_commands").format(
                result), parse_mode=ParseMode.MARKDOWN)
        else:
            update.effective_message.reply_text("No commands can be disabled.")

    # do not async
    def build_curr_disabled(chat_id: Union[str, int]) -> str:
        disabled = sql.get_all_disabled(chat_id)
        if not disabled:
            return tld(chat_id, "disable_chatsettings_none_disabled")

        result = ""
        for cmd in disabled:
            result += " - `{}`\n".format(escape_markdown(cmd))
        return tld(chat_id, "disable_chatsettings_list_disabled").format(result)

    def commands(update: Update, context: CallbackContext):
        bot = context.bot
        chat = update.effective_chat
        update.effective_message.reply_text(build_curr_disabled(chat.id),
                                            parse_mode=ParseMode.MARKDOWN)

    def __stats__():
        return "• `{}` disabled items, across `{}` chats.".format(
            sql.num_disabled(), sql.num_chats())

    def __migrate__(old_chat_id, new_chat_id):
        sql.migrate_chat(old_chat_id, new_chat_id)

    def __chat_settings__(chat_id, user_id):
        return build_curr_disabled(chat_id)

    __help__ = True

    DISABLE_HANDLER = CommandHandler(
        "disable", disable, filters=Filters.chat_type.groups, run_async=True)
    ENABLE_HANDLER = CommandHandler(
        "enable", enable, filters=Filters.chat_type.groups, run_async=True)
    COMMANDS_HANDLER = CommandHandler(
        ["cmds", "disabled"], commands, filters=Filters.chat_type.groups, run_async=True)
    TOGGLE_HANDLER = CommandHandler(
        "listcmds", list_cmds, filters=Filters.chat_type.groups, run_async=True)

    dispatcher.add_handler(DISABLE_HANDLER)
    dispatcher.add_handler(ENABLE_HANDLER)
    dispatcher.add_handler(COMMANDS_HANDLER)
    dispatcher.add_handler(TOGGLE_HANDLER)

else:
    DisableAbleCommandHandler = CommandHandler
    DisableAbleRegexHandler = RegexHandler
