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

from emoji import UNICODE_EMOJI
from googletrans import LANGUAGES, Translator
from telegram import Bot, Update, ParseMode
from telegram.ext import CallbackContext, run_async, CommandHandler
from smudge import dispatcher, trl
from smudge.modules.translations.strings import tld


def do_translate(update: Update, context: CallbackContext):
    chat = update.effective_chat  # type: Optional[Chat]
    msg = update.effective_message  # type: Optional[Message]

    try:
        if msg.reply_to_message:
            args = update.effective_message.text.split(None, 1)
            if msg.reply_to_message.text:
                text = msg.reply_to_message.text
            elif msg.reply_to_message.caption:
                text = msg.reply_to_message.caption

            message = update.effective_message
            dest_lang = None
            
            try:
                source_lang = args[1].split(None, 1)[0]
            except:
                source_lang = "en"

            if source_lang.count('-') == 2:
                for lang in problem_lang_code:
                    if lang in source_lang:
                        if source_lang.startswith(lang):
                            dest_lang = source_lang.rsplit("-", 1)[1]
                            source_lang = source_lang.rsplit("-", 1)[0]
                        else:
                            dest_lang = source_lang.split("-", 1)[1]
                            source_lang = source_lang.split("-", 1)[0]
            elif source_lang.count('-') == 1:
                for lang in problem_lang_code:
                    if lang in source_lang:
                        dest_lang = source_lang
                        source_lang = None
                        break
                if dest_lang == None:
                    dest_lang = source_lang.split("-")[1]
                    source_lang = source_lang.split("-")[0]
            else:
                dest_lang = source_lang
                source_lang = None

            exclude_list = UNICODE_EMOJI.keys()
            for emoji in exclude_list:
                if emoji in text:
                    text = text.replace(emoji, '')

            if source_lang == None:
                detection = trl.detect(text)
                tekstr = trl.translate(text, dest=dest_lang)
                return message.reply_text(
                    f"Translated from `{detection.lang}` to `{dest_lang}`:\n`{tekstr.text}`",
                    parse_mode=ParseMode.MARKDOWN)
            else:
                tekstr = trl.translate(text, dest=dest_lang, src=source_lang)
                message.reply_text(f"Translated from `{source_lang}` to `{dest_lang}`:\n`{tekstr.text}`",
                                   parse_mode=ParseMode.MARKDOWN)
        else:
            args = update.effective_message.text.split(None, 2)
            message = update.effective_message
            source_lang = args[1]
            text = args[2]
            exclude_list = UNICODE_EMOJI.keys()
            for emoji in exclude_list:
                if emoji in text:
                    text = text.replace(emoji, '')
            dest_lang = None
            temp_source_lang = source_lang
            if temp_source_lang.count('-') == 2:
                for lang in problem_lang_code:
                    if lang in temp_source_lang:
                        if temp_source_lang.startswith(lang):
                            dest_lang = temp_source_lang.rsplit("-", 1)[1]
                            source_lang = temp_source_lang.rsplit("-", 1)[0]
                        else:
                            dest_lang = temp_source_lang.split("-", 1)[1]
                            source_lang = temp_source_lang.split("-", 1)[0]
            elif temp_source_lang.count('-') == 1:
                for lang in problem_lang_code:
                    if lang in temp_source_lang:
                        dest_lang = None
                        break
                    else:
                        dest_lang = temp_source_lang.split("-")[1]
                        source_lang = temp_source_lang.split("-")[0]
            if dest_lang == None:
                detection = trl.detect(text)
                tekstr = trl.translate(text, dest=source_lang)
                return message.reply_text(
                    "Translated from `{}` to `{}`:\n`{}`".format(
                        detection.lang, source_lang, tekstr.text),
                    parse_mode=ParseMode.MARKDOWN)
            else:
                tekstr = trl.translate(text, dest=dest_lang, src=source_lang)
                message.reply_text("Translated from `{}` to `{}`:\n`{}`".format(source_lang, dest_lang, tekstr.text),
                                   parse_mode=ParseMode.MARKDOWN)

    except IndexError:
        pass
    except ValueError:
        update.effective_message.reply_text(
            "The intended language is not found!")
    else:
        return


__help__ = True

dispatcher.add_handler(
    CommandHandler("tr", do_translate, pass_args=True))
