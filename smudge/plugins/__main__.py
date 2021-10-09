from pyrogram.types import InlineKeyboardButton


class EqInlineKeyboardButton(InlineKeyboardButton):
    def __eq__(self, other):
        return self.text == other.text

    def __lt__(self, other):
        return self.text < other.text

    def __gt__(self, other):
        return self.text > other.text


def help_buttons(HELP):
    plugins = sorted(
        [
            EqInlineKeyboardButton(
                str(HELP[plugin][0]["name"]),
                callback_data="help_plugin({})".format(plugin.lower()),
            )
            for plugin in HELP.keys()
        ]
    )
    buttons = [plugins[i * 3 : (i + 1) * 3] for i in range((len(plugins) + 3 - 1) // 3)]

    round_num = len(plugins) / 3
    calc = len(plugins) - round(round_num)
    if calc == 1:
        buttons.append((plugins[-1],))
    elif calc == 2:
        buttons.append((plugins[-1],))

    return buttons