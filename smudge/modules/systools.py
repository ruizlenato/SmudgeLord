import subprocess
import os
import requests
import speedtest

import smudge.modules.helper_funcs.cas_api as cas
import smudge.modules.helper_funcs.git_api as git

from platform import python_version
from telegram import Update, Bot, Message, Chat, ParseMode
from telegram.ext import CommandHandler, run_async, Filters

from smudge import dispatcher, OWNER_ID, SUDO_USERS
from smudge.modules.helper_funcs.filters import CustomFilters
from smudge.modules.helper_funcs.extraction import extract_text
from smudge.modules.disable import DisableAbleCommandHandler


@run_async
def status(bot: Bot, update: Update):
    reply = "<b>System Status:</b> <code>operational</code>\n\n"
    reply += "<b>smudge version:</b> <code>2.5</code>\n"
    reply += "<b>Python version:</b> <code>"+python_version()+"</code>\n"
    reply += "<b>CAS API version:</b> <code>"+str(cas.vercheck())+"</code>\n"
    reply += "<b>GitHub API version:</b> <code>"+str(git.vercheck())+"</code>\n"
    update.effective_message.reply_text(reply, parse_mode=ParseMode.HTML)


def speed_convert(size):
    """
    Hi human, you can't read bytes?
    """
    power = 2 ** 10
    zero = 0
    units = {0: '', 1: 'Kb/s', 2: 'Mb/s', 3: 'Gb/s', 4: 'Tb/s'}
    while size > power:
        size /= power
        zero += 1
    return f"{round(size, 2)} {units[zero]}"


@run_async
def get_bot_ip(bot: Bot, update: Update):
    """ Sends the bot's IP address, so as to be able to ssh in if necessary.
        OWNER ONLY.
    """
    res = requests.get("http://ipinfo.io/ip")
    update.message.reply_text(res.text)


def ping(bot: Bot, update: Update):
    message = update.effective_message
    parsing = extract_text(message).split(' ')
    if (len(parsing) < 2):
        message.reply_text("Give me an address to ping!")
        return
    elif (len(parsing) > 2):
        message.reply_text("Too many arguments!")
        return
    dns = (parsing)[1]
    out = ""
    under = False
    if os.name == 'nt':
        try:
            output = subprocess.check_output("ping -n 1 " + dns + " | findstr time*", shell=True).decode()
        except:
            message.reply_text("There was a problem parsing the IP/Hostname")
            return
        outS = output.splitlines()
        out = outS[0]
    else:
        try:
            out = subprocess.check_output("ping -c 1 " + dns + " | grep time=", shell=True).decode()
        except:
            message.reply_text("There was a problem parsing the IP/Hostname")
            return
    splitOut = out.split(' ')
    stringtocut = ""
    for line in splitOut:
        if (line.startswith('time=') or line.startswith('time<')):
            stringtocut = line
            break
    newstra = stringtocut.split('=')
    if len(newstra) == 1:
        under = True
        newstra = stringtocut.split('<')
    newstr = ""
    if os.name == 'nt':
        newstr = newstra[1].split('ms')
    else:
        newstr = newstra[1].split(' ')  # redundant split, but to try and not break windows ping
    ping_time = float(newstr[0])
    if os.name == 'nt' and under:
        update.effective_message.reply_text(" Ping speed of " + dns + " is <{}ms".format(ping_time))
    else:
        update.effective_message.reply_text(" Ping speed of " + dns + ": {}ms".format(ping_time))


@run_async
def speedtst(bot: Bot, update: Update):
    chat = update.effective_chat
    del_msg = bot.send_message(chat.id, "<code>🔄 Running speedtest...</code>",
                               parse_mode=ParseMode.HTML)
    test = speedtest.Speedtest()
    test.get_best_server()
    test.download()
    test.upload()
    test.results.share()
    result = test.results.dict()
    del_msg.delete()
    update.effective_message.reply_text("<b>SpeedTest Results</b> \n\n"
                                        "<b>Download:</b> "
                                        f"<code>{speed_convert(result['download'])}</code> \n"
                                        "<b>Upload:</b> "
                                        f"<code>{speed_convert(result['upload'])}</code> \n"
                                        "<b>Ping:</b> "
                                        f"<code>{result['ping']}</code> \n"
                                        "<b>ISP:</b> "
                                        f"<code>{result['client']['isp']}</code>",
                                        parse_mode=ParseMode.HTML)


STATUS_HANDLER = CommandHandler("status", status)
IP_HANDLER = CommandHandler("ip", get_bot_ip, filters=Filters.chat(OWNER_ID))
PING_HANDLER = CommandHandler("cping", ping, filters=CustomFilters.sudo_filter)
SPEED_HANDLER = CommandHandler("speedtest", speedtst, filters=CustomFilters.sudo_filter)

dispatcher.add_handler(STATUS_HANDLER)
dispatcher.add_handler(IP_HANDLER)
dispatcher.add_handler(SPEED_HANDLER)
dispatcher.add_handler(PING_HANDLER)
