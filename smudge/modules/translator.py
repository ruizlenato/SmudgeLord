from typing import Optional, List

from telegram import Message, Update, Bot
from telegram.ext import run_async

from smudge import dispatcher
from smudge.modules.disable import DisableAbleCommandHandler

from googletrans import Translator

from smudge.modules.translations.strings import tld

@run_async
def do_translate(bot: Bot, update: Update, args: List[str]):
    chat = update.effective_chat  # type: Optional[Chat]
    msg = update.effective_message  # type: Optional[Message]
    lan = " ".join(args)
    try:
        if msg.reply_to_message:
            args = update.effective_message.text.split(None, 1)
        if msg.reply_to_message.text:
                to_translate_text = msg.reply_to_message.text
        elif msg.reply_to_message.caption:
                to_translate_text = msg.reply_to_message.caption
    except:
        return
    translator = Translator()
    try:
        translated = translator.translate(to_translate_text, dest=lan)
        src_lang = translated.src
        translated_text = translated.text
        msg.reply_text(tld(chat.id, 'translator_translated').format(
            src_lang, lan, translated_text))
    except Exception as e:
        msg.reply_text(tld(chat.id, 'translator_err').format(e))

__help__ = True

dispatcher.add_handler(
    DisableAbleCommandHandler("tr", do_translate, pass_args=True))
