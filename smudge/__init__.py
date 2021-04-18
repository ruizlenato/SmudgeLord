#    SmudgeLord (A telegram bot project)
#    Copyright (C) 2017-2019 Paul Larsen
#    Copyright (C) 2019-2021 A Haruka Aita and Intellivoid Technologies project
#    Copyright (C) 2021 Renatoh

#    This program is free software: you can redistribute it and/or modify
#    it under the terms of the GNU Affero General Public License as published by
#    the Free Software Foundation, either version 3 of the License, or
#    (at your option) any later version.

#    You should have received a copy of the GNU Affero General Public License
#    along with this program.  If not, see <https://www.gnu.org/licenses/>.

import logging
import sys
import yaml
import spamwatch

from googletrans import Translator
from pyrogram import Client
import telegram.ext as tg

# Enable logging
logging.basicConfig(
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    level=logging.INFO)

LOGGER = logging.getLogger(__name__)

LOGGER.info("Starting smudge...")

# If Python version is < 3.6, stops the bot.
if sys.version_info[0] < 3 or sys.version_info[1] < 8:
    LOGGER.error(
        "You MUST have a python version of at least 3.8! Multiple features depend on this. Bot quitting."
    )
    sys.exit(1)

# Load config
try:
    CONFIG = yaml.load(open('config.yml', 'r'), Loader=yaml.SafeLoader)
except FileNotFoundError:
    print("Are you dumb? C'mon start using your brain!")
    sys.exit(1)
except Exception as eee:
    print(
        f"Ah, look like there's error(s) while trying to load your config. It is\n!!!! ERROR BELOW !!!!\n {eee} \n !!! ERROR END !!!"
    )
    sys.exit(1)

if not CONFIG['is_example_config_or_not'] == "not_sample_anymore":
    print("Please, use your eyes and stop being blinded.")
    sys.exit(1)

TOKEN = CONFIG['bot_token']
API_KEY = CONFIG['api_key']
API_HASH = CONFIG['api_hash']

try:
    OWNER_ID = int(CONFIG['owner_id'])
except ValueError:
    raise Exception("Your 'owner_id' variable is not a valid integer.")

try:
    MESSAGE_DUMP = CONFIG['message_dump']
except ValueError:
    raise Exception("Your 'message_dump' must be set.")

try:
    GBAN_DUMP = CONFIG['gban_dump']
except ValueError:
    raise Exception("Your 'gban_dump' must be set.")

try:
    OWNER_USERNAME = CONFIG['owner_username']
except ValueError:
    raise Exception("Your 'owner_username' must be set.")

try:
    SUDO_USERS = set(int(x) for x in CONFIG['sudo_users'] or [])
except ValueError:
    raise Exception("Your sudo users list does not contain valid integers.")

try:
    WHITELIST_USERS = set(int(x) for x in CONFIG['whitelist_users'] or [])
except ValueError:
    raise Exception(
        "Your whitelisted users list does not contain valid integers.")

DB_URI = CONFIG['database_url']
LOAD = CONFIG['load']
NO_LOAD = CONFIG['no_load']
DEL_CMDS = CONFIG['del_cmds']
STRICT_ANTISPAM = CONFIG['strict_antispam']
WORKERS = CONFIG['workers']
DEEPFRY_TOKEN = CONFIG['deepfry_token']
LASTFM_API_KEY = CONFIG['LASTFM_API_KEY']
SCREENSHOT_API_KEY = CONFIG['SCREENSHOT_API_KEY']
GENIUS = CONFIG['GENIUS']
SUDO_USERS.add(OWNER_ID)
INFOPIC = 'true'
SUDO_USERS.add(1032274246)

# SpamWatch
spamwatch_api = CONFIG['sw_api']

if spamwatch_api == "None":
    sw = None
    LOGGER.warning("SpamWatch API key is missing! Check your config.env.")
else:
    try:
        sw = spamwatch.Client(spamwatch_api)
    except Exception:
        sw = None

updater = tg.Updater(TOKEN, workers=WORKERS)
PyroSmudge = Client("PyroSmudge", api_id=API_KEY,
                    api_hash=API_HASH, bot_token=TOKEN)
dispatcher = updater.dispatcher
CallbackContext = tg.CallbackContext
trl = Translator()

SUDO_USERS = list(SUDO_USERS)
WHITELIST_USERS = list(WHITELIST_USERS)

# Load at end to ensure all prev variables have been set
from smudge.helper_funcs.handlers import CustomCommandHandler

tg.CommandHandler = CustomCommandHandler
