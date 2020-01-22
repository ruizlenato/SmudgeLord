import time
import requests
import json

from pytz import country_names as cname
from telegram import Message, Chat, Update, Bot, ParseMode
from telegram.error import BadRequest
from telegram.ext import run_async

from smudge import dispatcher, updater, API_WEATHER as APPID
from smudge.modules.disable import DisableAbleCommandHandler


@run_async
def weather(bot, update, args):
    if len(args) == 0:
        update.effective_message.reply_text("Write a location to check the weather.")
        return


    CITY = " ".join(args)
    url = f'https://api.openweathermap.org/data/2.5/weather?q={CITY}&appid={APPID}'
    request = requests.get(url)
    result = json.loads(request.text)
    if request.status_code != 200:
        update.effective_message.reply_text("Location not valid.")
        return
    
    cityname = result['name']
    curtemp = result['main']['temp']
    feels_like = result['main']['feels_like']
    humidity = result['main']['humidity']
    wind = result['wind']['speed']
    weath = result['weather'][0]
    desc = weath['main']
    icon = weath['id']
    condmain = weath['main']
    conddet = weath['description']
    country_name = cname[f"{result['sys']['country']}"]
    if icon <= 232: # Rain storm
        icon = "⛈"
    elif icon <= 321: # Drizzle
        icon = "🌧"
    elif icon <= 504: # Light rain
        icon = "🌦"
    elif icon <= 531: # Cloudy rain
        icon = "⛈"
    elif icon <= 622: # Snow
        icon = "❄️"
    elif icon <= 781: # Atmosphere
        icon = "🌪"
    elif icon <= 800: # Bright
        icon = "☀️"
    elif icon <= 801: # A little cloudy
        icon = "⛅️"
    elif icon <= 804: # Cloudy
        icon = "☁️"
    kmph = str(wind * 3.6).split(".")
    def celsius(c):
        k = 273.15
        c = k if ( c > (k - 1) ) and ( c < k ) else c
        temp = str(round((c - k)))
        return temp
    def fahr(c):
        c1 = 9/5
        c2 = 459.67
        tF = c * c1 - c2
        if tF<0 and tF>-1:
            tF = 0
        temp = str(round(tF))
        return temp

    reply = f"*Current weather for {cityname}, {country_name} is*:\n\n*Temperature:* `{celsius(curtemp)}°C ({fahr(curtemp)}ºF), feels like {celsius(feels_like)}°C ({fahr(feels_like)}ºF) \n`*Condition:* `{condmain}, {conddet}` {icon}\n*Humidity:* `{humidity}%`\n*Wind:* `{kmph[0]} km/h`\n"
    update.effective_message.reply_text("{}".format(reply),
                parse_mode=ParseMode.MARKDOWN, disable_web_page_preview=True)
    return


__help__ = """
 - /weather <city>: gets weather info in a particular place using openweathermap.org api
"""

__mod_name__ = "Weather"

WEATHER_HANDLER = DisableAbleCommandHandler("weather", weather, pass_args=True)

dispatcher.add_handler(WEATHER_HANDLER)
