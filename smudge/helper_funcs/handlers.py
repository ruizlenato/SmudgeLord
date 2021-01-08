#    SmudgeLord (A telegram bot project)
#    Copyright (C) 2017-2019 Paul Larsen
#    Copyright (C) 2019-2021 A Haruka Aita and Intellivoid Technologies project
#    Copyright (C) 2021 Renatoh 

#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU Affero General Public License as published by
#    the Free Software Foundation, either version 3 of the License, or
#    (at your option) any later version.

#    You should have received a copy of the GNU Affero General Public License
#    along with this program.  If not, see <https://www.gnu.org/licenses/>.

import telegram.ext as tg
from telegram import Update
import smudge.modules.sql.antispam_sql as sql

CMD_STARTERS = ('/', '!')


class CustomCommandHandler(tg.CommandHandler):
    def __init__(self, command, callback, run_async=True, **kwargs):
        if "admin_ok" in kwargs:
            del kwargs["admin_ok"]
        super().__init__(command, callback, run_async=run_async, **kwargs)

    def check_update(self, update):
        if isinstance(update, Update) and update.effective_message:
            message = update.effective_message

            try:
                user_id = update.effective_user.id
            except:
                user_id = None

            if message.text and len(message.text) > 1:
                fst_word = message.text.split(None, 1)[0]
                if len(fst_word) > 1 and any(
                    fst_word.startswith(start) for start in CMD_STARTERS
                ):
                    args = message.text.split()[1:]
                    command = fst_word[1:].split("@")
                    command.append(
                        message.bot.username
                    )  # in case the command was sent without a username

                    if not (
                        command[0].lower() in self.command
                        and command[1].lower() == message.bot.username.lower()
                    ):
                        return None

                    filter_result = self.filters(update)
                    if filter_result:
                        return args, filter_result
                    else:
                        return False

class GbanLockHandler(tg.CommandHandler):
    def __init__(self, command, callback, **kwargs):
        super().__init__(command, callback, **kwargs)

    def check_update(self, update):
        if (isinstance(update, Update) and
            (update.message or update.edited_message and self.allow_edited)):
            message = update.message or update.edited_message
            if sql.is_user_gbanned(update.effective_user.id):
                return False
            if message.text and message.text.startswith('/') and len(
                    message.text) > 1:
                first_word = message.text_html.split(None, 1)[0]
                if len(first_word) > 1 and first_word.startswith('/'):
                    command = first_word[1:].split('@')
                    command.append(
                        message.bot.username
                    )  # in case the command was sent without a username
                    if not (command[0].lower() in self.command
                            and command[1].lower()
                            == message.bot.username.lower()):
                        return False
                    if self.filters is None:
                        res = True
                    elif isinstance(self.filters, list):
                        res = any(func(message) for func in self.filters)
                    else:
                        res = self.filters(message)
                    return res
            return False
