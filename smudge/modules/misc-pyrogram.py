import re
import sys
import json
import time
import html
import regex
import requests
import traceback
import subprocess
from datetime import datetime
from pyrogram.types import Message
from smudge import pbot, SUDO_USERS
from pyrogram import Client, filters

@pbot.on_message(filters.command("ping", prefixes="/"))
async def ping(c: Client, m: Message):
    first = datetime.now()
    sent = await m.reply_text("**Pong!**")
    second = datetime.now()
    await sent.edit_text(f"**Pong!** `{(second - first).microseconds / 1000}`ms")

@pbot.on_message(filters.regex(r'^s/(.+)?/(.+)?(/.+)?') & filters.reply)
async def sed(c: Client, m: Message):
    exp = regex.split(r'(?<![^\\]\\)/', m.text)
    pattern = exp[1]
    replace_with = exp[2].replace(r'\/', '/')
    flags = exp[3] if len(exp) > 3 else ''

    count = 1
    rflags = 0

    if 'g' in flags:
        count = 0
    if 'i' in flags and 's' in flags:
        rflags = regex.I | regex.S
    elif 'i' in flags:
        rflags = regex.I
    elif 's' in flags:
        rflags = regex.S

    text = m.reply_to_message.text or m.reply_to_message.caption

    if not text:
        return

    try:
        res = regex.sub(pattern, replace_with, text, count=count, flags=rflags, timeout=1)
    except TimeoutError:
        await m.reply_text(_("sed.regex_timeout"))
    except regex.error as e:
        await m.reply_text(str(e))
    else:
        await c.send_message(m.chat.id, f'{html.escape(res)}',
                             reply_to_message_id=m.reply_to_message.message_id)

@pbot.on_message(filters.user(SUDO_USERS) & filters.command("term"))
async def terminal(client, message):
    if len(message.text.split()) == 1:
        await message.reply("Usage: `/term echo owo`")
        return
    args = message.text.split(None, 1)
    teks = args[1]
    if "\n" in teks:
        code = teks.split("\n")
        output = ""
        for x in code:
            shell = re.split(''' (?=(?:[^'"]|'[^']*'|"[^"]*")*$)''', x)
            try:
                process = subprocess.Popen(
                    shell,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE
                )
            except Exception as err:
                print(err)
                await message.reply("""
**Error:**
```{}```
""".format(err))
            output += "**{}**\n".format(code)
            output += process.stdout.read()[:-1].decode("utf-8")
            output += "\n"
    else:
        shell = re.split(''' (?=(?:[^'"]|'[^']*'|"[^"]*")*$)''', teks)
        for a in range(len(shell)):
            shell[a] = shell[a].replace('"', "")
        try:
            process = subprocess.Popen(
                shell,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE
            )
        except Exception as err:
            exc_type, exc_obj, exc_tb = sys.exc_info()
            errors = traceback.format_exception(etype=exc_type, value=exc_obj, tb=exc_tb)
            await message.reply("""**Error:**\n```{}```""".format("".join(errors)))
            return
        output = process.stdout.read()[:-1].decode("utf-8")
    if str(output) == "\n":
        output = None
    if output:
        if len(output) > 4096:
            with open("nana/cache/output.txt", "w+") as file:
                file.write(output)
            await client.send_document(message.chat.id, "nana/cache/output.txt", reply_to_message_id=message.message_id,
                                    caption="`Output file`")
            os.remove("nana/cache/output.txt")
            return
        await message.reply(f"**Output:**\n```{output}```", parse_mode='markdown')
    else:
        await message.reply("**Output:**\n`No Output`")

@pbot.on_message(filters.command("json", prefixes="/") & filters.user(SUDO_USERS))
async def jsonify(client, message):
    the_real_message = None
    reply_to_id = None

    if message.reply_to_message:
        the_real_message = message.reply_to_message
    else:
        the_real_message = message

    try:
        await message.reply_text(f"<code>{the_real_message}</code>")
    except Exception as e:
        with open("json.text", "w+", encoding="utf8") as out_file:
            out_file.write(str(the_real_message))
        await message.reply_document(
            document="json.text",
            caption=str(e),
            disable_notification=True,
            reply_to_message_id=reply_to_id
        )
        os.remove("json.text")