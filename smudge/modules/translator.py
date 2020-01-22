from typing import Optional, List

from googletrans import Translator
from telegram import message, Update, Bot
from telegram.ext import run_async
from telegram.ext import CommandHandler

from smudge import dispatcher
from smudge.modules.disable import DisableAbleCommandHandler


@run_async
def do_translate(bot: Bot, update: Update, args: List[str]):
    msg = update.effective_message  # type: Optional[Message]
    lan = " ".join(args)
    try:
        to_translate_text = msg.reply_to_message.text
    except:
        return
    translator = Translator()
    try:
        translated = translator.translate(to_translate_text, dest=lan)
        src_lang = translated.src
        translated_text = translated.text
        msg.reply_text("Translated from {} to {}.\n{}".format(src_lang, lan, translated_text))
    except Exception as e:
        msg.reply_text(f"Error occured while translating:\n{e}")


__help__ = """ 
*This module uses Google Translate to do the translations.*

 - /tr <language code>: as reply to a long message.
"""
__mod_name__ = "Translator"

dispatcher.add_handler(CommandHandler("tr", do_translate, pass_args=True))
