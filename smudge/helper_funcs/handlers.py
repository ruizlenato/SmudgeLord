import telegram.ext as tg
from telegram import Update
from smudge import ALLOW_EXCL

CMD_STARTERS = ('/', '!')


class CustomCommandHandler(tg.CommandHandler):
    def __init__(self, command, callback, **kwargs):
        if "admin_ok" in kwargs:
            del kwargs["admin_ok"]
        super().__init__(command, callback, **kwargs)


class CustomRegexHandler(tg.MessageHandler):
    def __init__(self, pattern, callback, friendly="", **kwargs):
        super().__init__(tg.Filters.regex(pattern), callback, **kwargs)
