from telethon import custom


def build_keyboard(buttons):
    if len(buttons) != 0:
        keyb = []
        for btn in buttons:
            if btn.same_line and keyb:
                keyb[-1].append(custom.Button.url(btn.name, url=btn.url))
            else:
                keyb.append([custom.Button.url(btn.name, url=btn.url)])
    else:
        keyb = None

    return keyb


def revert_buttons(buttons):
    res = ""
    for btn in buttons:
        if btn.same_line:
            res += "\n[{}](buttonurl://{}:same)".format(btn.name, btn.url)
        else:
            res += "\n[{}](buttonurl://{})".format(btn.name, btn.url)

    return res