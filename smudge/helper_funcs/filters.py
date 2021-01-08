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

from telegram import Message
from telegram.ext import MessageFilter
from emoji import UNICODE_EMOJI

from smudge import SUDO_USERS


class CustomFilters(object):

    class _Sudoers(MessageFilter):
        def filter(self, message: Message):
            return bool(message.from_user
                        and message.from_user.id in SUDO_USERS)

    sudo_filter = _Sudoers()

    class _MimeType(MessageFilter):
        def __init__(self, mimetype):
            self.mime_type = mimetype
            self.name = "CustomFilters.mime_type({})".format(self.mime_type)

        def filter(self, message: Message):
            return bool(message.document
                        and message.document.mime_type == self.mime_type)

    mime_type = _MimeType

    class _HasText(MessageFilter):
        def filter(self, message: Message):
            return bool(message.text or message.sticker or message.photo
                        or message.document or message.video)

    has_text = _HasText()

    class _HasEmoji(MessageFilter):
        def filter(self, message: Message):
            text = ""
            if (message.text):
                text = message.text
            for emoji in UNICODE_EMOJI:
                for letter in text:
                    if (letter == emoji):
                        return True
            return False

    has_emoji = _HasEmoji()

    class _IsEmoji(MessageFilter):
        def filter(self, message: Message):
            if (message.text and len(message.text) == 1):
                for emoji in UNICODE_EMOJI:
                    for letter in message.text:
                        if (letter == emoji):
                            return True
            return False

    is_emoji = _IsEmoji()
