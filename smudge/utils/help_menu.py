from pyrogram.types import InlineKeyboardButton
from smudge.locales.strings import tld

class EqInlineKeyboardButton(InlineKeyboardButton):
    def __eq__(self, other):
        return self.text == other.text

    def __lt__(self, other):
        return self.text < other.text

    def __gt__(self, other):
        return self.text > other.text


async def help_buttons(m, HELP):
    plugins = sorted(
        [
            EqInlineKeyboardButton(
                await tld(m.message.chat.id, str(HELP[plugin][0]["name"])),
                callback_data="help_plugin({})".format(plugin.lower()),
            )
            for plugin in HELP.keys()
        ]
    )
    buttons = [
        plugins[i * 3:(i + 1) * 3] for i in range((len(plugins) + 3 - 1) // 3)
    ]
    round_num = len(plugins) / 3
    calc = len(plugins) - round(round_num)
    if calc == 1:
        buttons.append((plugins[-1],))
    elif calc == 2:
        buttons.append((plugins[-1],))

    return buttons

async def help_plugin(c, m, text):
    keyboard = [[InlineKeyboardButton("Back", callback_data="menu")]]
    text = "<b> Avaliable Commands:</b>\n" + text
    await m.edit_message_text(text, reply_markup=InlineKeyboardMarkup(keyboard))