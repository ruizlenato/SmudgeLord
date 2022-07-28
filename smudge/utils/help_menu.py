# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
from pyrogram.types import InlineKeyboardButton
from smudge.utils.locales import tld


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
            (
                await tld(m, str(HELP[plugin][0]["name"] + ".name")),
                f"help_plugin({plugin.lower()})",
            )
            for plugin in HELP.keys()
        ]
    )

    buttons = [plugins[i * 3 : (i + 1) * 3] for i in range((len(plugins) + 3 - 1) // 3)]
    round_num = len(plugins) / 3
    calc = len(plugins) - round(round_num)
    if calc == 1:
        buttons.append((plugins[-1],))

    return buttons
