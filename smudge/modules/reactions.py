import random

from telegram import Update, Bot
from telegram.ext import run_async

from smudge import dispatcher
from smudge.modules.disable import DisableAbleCommandHandler

reactions = ["( ͡° ͜ʖ ͡°)", "¯_(ツ)_/¯", "\'\'̵͇З= ( ▀ ͜͞ʖ▀) =Ε/̵͇/’’", "▄︻̷┻═━一", "( ͡°( ͡° ͜ʖ( ͡° ͜ʖ ͡°)ʖ ͡°) ͡°)",
             "ʕ•ᴥ•ʔ", "(▀Ĺ̯▀ )", "(ง ͠° ͟ل͜ ͡°)ง", "༼ つ ◕_◕ ༽つ", "ಠ_ಠ", "(づ｡◕‿‿◕｡)づ", "\'\'̵͇З=( ͠° ͟ʖ ͡°)=Ε/̵͇/\'",
             "(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧ ✧ﾟ･: *ヽ(◕ヮ◕ヽ)", "[̲̅$̲̅(̲̅5̲̅)̲̅$̲̅]", "┬┴┬┴┤ ͜ʖ ͡°) ├┬┴┬┴", "( ͡°╭͜ʖ╮͡° )",
             "(͡ ͡° ͜ つ ͡͡°)", "(• Ε •)", "(ง\'̀-\'́)ง", "(ಥ﹏ಥ)", "﴾͡๏̯͡๏﴿ O\'RLY?", "(ノಠ益ಠ)ノ彡┻━┻",
             "[̲̅$̲̅(̲̅ ͡° ͜ʖ ͡°̲̅)̲̅$̲̅]", "(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧", "(☞ﾟ∀ﾟ)☞", "| (• ◡•)| (❍ᴥ❍Ʋ)", "(◕‿◕✿)", "(ᵔᴥᵔ)",
             "(╯°□°)╯︵ ꞰOOQƎƆⱯɟ", "(¬‿¬)", "(☞ﾟヮﾟ)☞ ☜(ﾟヮﾟ☜)", "(づ￣ ³￣)づ", "ლ(ಠ益ಠლ)", "ಠ╭╮ಠ", "\'\'̵͇З=(•_•)=Ε/̵͇/\'\'",
             "/╲/╭( ͡° ͡° ͜ʖ ͡° ͡°)╮/╱", "(;´༎ຶД༎ຶ)", "♪~ ᕕ(ᐛ)ᕗ", "♥️‿♥️", "༼ つ ͡° ͜ʖ ͡° ༽つ", "༼ つ ಥ_ಥ ༽つ",
             "(╯°□°）╯︵ ┻━┻", "( ͡ᵔ ͜ʖ ͡ᵔ )", "ヾ(⌐■_■)ノ♪", "~(˘▾˘~)", "◉_◉", "(•◡•) /", "(~˘▾˘)~",
             "(._.) ( L: ) ( .-. ) ( :L ) (._.)", "༼ʘ̚ل͜ʘ̚༽", "༼ ºل͟º ༼ ºل͟º ༼ ºل͟º ༽ ºل͟º ༽ ºل͟º ༽", "┬┴┬┴┤(･_├┬┴┬┴",
             "ᕙ(⇀‸↼‶)ᕗ", "ᕦ(Ò_Óˇ)ᕤ", "┻━┻ ︵ヽ(Д´)ﾉ︵ ┻━┻", "⚆ _ ⚆", "(•_•) ( •_•)>⌐■-■ (⌐■_■)", "(｡◕‿‿◕｡)", "ಥ_ಥ",
             "ヽ༼ຈل͜ຈ༽ﾉ", "⌐╦╦═─", "(☞ຈل͜ຈ)☞", "˙ ͜ʟ˙", "☜(˚▽˚)☞", "(•Ω•)", "(ง°ل͜°)ง", "(｡◕‿◕｡)", "（╯°□°）╯︵( .O.)",
             ":\')", "┬──┬ ノ( ゜-゜ノ)", "(っ˘ڡ˘Σ)", "ಠ⌣ಠ", "ლ(´ڡლ)", "(°ロ°)☝️", "｡◕‿‿◕｡", "( ಠ ͜ʖರೃ)", "╚(ಠ_ಠ)=┐",
             "(─‿‿─)", "ƪ(˘⌣˘)Ʃ", "(；一_一)", "(¬_¬)", "( ⚆ _ ⚆ )", "(ʘᗩʘ\')", "☜(⌒▽⌒)☞", "｡◕‿◕｡", "¯(°_O)/¯", "(ʘ‿ʘ)",
             "ლ,ᔑ•ﺪ͟͠•ᔐ.ლ", "(´・Ω・)", "ಠ~ಠ", "(° ͡ ͜ ͡ʖ ͡ °)", "┬─┬ノ( º _ ºノ)", "(´・Ω・)っ由", "ಠ_ಥ", "Ƹ̵̡Ӝ̵̨Ʒ", "(>ლ)",
             "ಠ‿↼", "ʘ‿ʘ", "(ღ˘⌣˘ღ)", "ಠOಠ", "ರ_ರ", "(▰˘◡˘▰)", "◔̯◔", "◔ ⌣ ◔", "(✿´‿`)", "¬_¬", "ب_ب", "｡゜(｀Д´)゜｡",
             "(Ó Ì_Í)=ÓÒ=(Ì_Í Ò)", "°Д°", "( ﾟヮﾟ)", "┬─┬﻿ ︵ /(.□. ）", "٩◔̯◔۶", "≧☉_☉≦", "☼.☼", "^̮^", "(>人<)",
             "〆(・∀・＠)", "(~_^)", "^̮^", "^̮^", ">_>", "(^̮^)", "(/) (°,,°) (/)", "^̮^", "^̮^", "=U", "(･.◤)"]

reactionhappy = ["\'\'̵͇З= ( ▀ ͜͞ʖ▀) =Ε/̵͇/’’", "ʕ•ᴥ•ʔ", "(づ｡◕‿‿◕｡)づ", "(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧ ✧ﾟ･: *ヽ(◕ヮ◕ヽ)", "(ﾉ◕ヮ◕)ﾉ*:･ﾟ✧",
                 "(☞ﾟ∀ﾟ)☞", "| (• ◡•)| (❍ᴥ❍Ʋ)", "(◕‿◕✿)", "(ᵔᴥᵔ)", "(☞ﾟヮﾟ)☞ ☜(ﾟヮﾟ☜)", "(づ￣ ³￣)づ", "♪~ ᕕ(ᐛ)ᕗ", "♥️‿♥️",
                 "༼ つ ͡° ͜ʖ ͡° ༽つ", "༼ つ ಥ_ಥ ༽つ", "ヾ(⌐■_■)ノ♪", "~(˘▾˘~)", "◉_◉", "(•◡•) /", "(~˘▾˘)~", "(｡◕‿‿◕｡)",
                 "☜(˚▽˚)☞", "(•Ω•)", "(｡◕‿◕｡)", "(っ˘ڡ˘Σ)", "｡◕‿‿◕｡""☜(⌒▽⌒)☞", "｡◕‿◕｡", "(ღ˘⌣˘ღ)", "(▰˘◡˘▰)", "^̮^",
                 "^̮^", ">_>", "(^̮^)", "^̮^", "^̮^"]

reactionangry = ["▄︻̷┻═━一", "(▀Ĺ̯▀ )", "(ง ͠° ͟ل͜ ͡°)ง", "༼ つ ◕_◕ ༽つ", "ಠ_ಠ", "\'\'̵͇З=( ͠° ͟ʖ ͡°)=Ε/̵͇/\'",
                 "(ง\'̀-\'́)ง", "(ノಠ益ಠ)ノ彡┻━┻", "(╯°□°)╯︵ ꞰOOQƎƆⱯɟ", "ლ(ಠ益ಠლ)", "ಠ╭╮ಠ", "\'\'̵͇З=(•_•)=Ε/̵͇/\'\'",
                 "(╯°□°）╯︵ ┻━┻", "┻━┻ ︵ヽ(Д´)ﾉ︵ ┻━┻", "⌐╦╦═─", "（╯°□°）╯︵( .O.)", ":\')", "┬──┬ ノ( ゜-゜ノ)", "ლ(´ڡლ)",
                 "(°ロ°)☝️", "ლ,ᔑ•ﺪ͟͠•ᔐ.ლ", "┬─┬ノ( º _ ºノ)", "┬─┬﻿ ︵ /(.□. ）"]


@run_async
def react(bot: Bot, update: Update):
    message = update.effective_message
    react = random.choice(reactions)
    if message.reply_to_message:
        message.reply_to_message.reply_text(react)
    else:
        message.reply_text(react)


@run_async
def rhappy(bot: Bot, update: Update):
    message = update.effective_message
    rhappy = random.choice(reactionhappy)
    if message.reply_to_message:
        message.reply_to_message.reply_text(rhappy)
    else:
        message.reply_text(rhappy)


@run_async
def rangry(bot: Bot, update: Update):
    message = update.effective_message
    rangry = random.choice(reactionangry)
    if message.reply_to_message:
        message.reply_to_message.reply_text(rangry)
    else:
        message.reply_text(rangry)


__help__ = """
*Use these commands to let the bot express reactions for you!*

 - /react: reacts with normal reactions.
 - /happy: reacts with happiness.
 - /angry: reacts angrily.
"""

__mod_name__ = "Reactions"

REACT_HANDLER = DisableAbleCommandHandler("react", react)
RHAPPY_HANDLER = DisableAbleCommandHandler("happy", rhappy)
RANGRY_HANDLER = DisableAbleCommandHandler("angry", rangry)

dispatcher.add_handler(REACT_HANDLER)
dispatcher.add_handler(RHAPPY_HANDLER)
dispatcher.add_handler(RANGRY_HANDLER)
