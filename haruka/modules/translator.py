from typing import Optional, List

from telegram import Message, Update, Bot, User, ParseMode
from telegram import MessageEntity
from telegram.ext import Filters, MessageHandler, run_async

from haruka import dispatcher, LOGGER
from haruka.modules.disable import DisableAbleCommandHandler
from haruka.modules.helper_funcs.string_handling import remove_emoji

from googletrans import LANGUAGES, Translator

from haruka.modules.translations.strings import tld

@run_async
def do_translate(bot: Bot, update: Update, args: List[str]):
    chat = update.effective_chat  # type: Optional[Chat]
    msg = update.effective_message  # type: Optional[Message]
    lan = " ".join(args)
    if msg.reply_to_message:
        to_translate_text = remove_emoji(msg.reply_to_message.text)
    else:
        msg.reply_text(tld(chat.id, "translator_no_str"))
        return
    
    if not args:
        msg.reply_text(tld(chat.id, 'translator_no_args'))

    translator = Translator()
    try:
        translated = translator.translate(to_translate_text, dest=lan)
    except ValueError as e: 
        msg.reply_text(tld(chat.id, 'translator_err').format(e))

    src_lang = LANGUAGES[f'{translated.src.lower()}'].title()
    dest_lang = LANGUAGES[f'{translated.dest.lower()}'].title()
    translated_text = translated.text
    msg.reply_text(tld(chat.id, 'translator_translated').format(
        src_lang, to_translate_text, dest_lang, translated_text), parse_mode=ParseMode.MARKDOWN)


__help__ = True

dispatcher.add_handler(
    DisableAbleCommandHandler("tr", do_translate, pass_args=True))
