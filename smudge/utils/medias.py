import contextlib
import io
import json
import re
from urllib.parse import unquote

import filetype
from config import BARRER_TOKEN
from yt_dlp import YoutubeDL

from .utils import aiowrap, http


@aiowrap
def extract_info(instance: YoutubeDL, url: str, download=True):
    instance.params.update({"logger": MyLogger()})
    return instance.extract_info(url, download)


class MyLogger:
    def debug(self, msg):
        if not msg.startswith("[debug] "):
            self.info(msg)

    def info(self, msg):
        pass

    def warning(self, msg):
        pass

    @staticmethod
    def error(msg):
        if "There's no video" not in msg:
            print(msg)


class DownloadMedia:
    def __init__(self):
        self.cors: str = "https://cors-bypass.amanoteam.com/"
        self.TwitterAPI: str = "https://api.twitter.com/2/"

    async def download(self, url: str, captions):
        self.files: list = []
        if re.search(r"instagram.com/", url):
            await self.instagram(url, captions)
        elif re.search(r"tiktok.com/", url):
            await self.TikTok(url, captions)
        elif re.search(r"twitter.com/", url):
            await self.Twitter(url, captions)

        if captions is False:
            self.caption = f"<a href='{url}'>🔗 Link</a>"

        return self.files, self.caption

    async def instagram(self, url: str, captions: str):
        res = await http.post("https://igram.world/api/convert", data={"url": url})
        data = res.json()

        self.caption = f"\n<a href='{url}'>🔗 Link</a>"

        if data:
            data = [data] if isinstance(data, dict) else data

            for media in data:
                url = re.sub(
                    r".*(htt.+?//)(:?ins.+?.fna.f.+?net|s.+?.com)?(.+?)(&file.*)",
                    r"\1scontent.cdninstagram.com\3",
                    unquote(media["url"][0]["url"]),
                )
                file = io.BytesIO((await http.get(url)).content)
                file.name = f"{url[60:80]}.{filetype.guess_extension(file)}"
                self.files.append({"p": file, "w": 0, "h": 0})
            return

    async def Twitter(self, url: str, captions: str):
        # Extract the tweet ID from the URL
        tweet_id = re.match(".*twitter.com/.+status/([A-Za-z0-9]+)", url)[1]
        params: str = "?expansions=attachments.media_keys,author_id&media.fields=\
type,variants,url,height,width&tweet.fields=entities"
        # Send the request and parse the response as JSON
        res = await http.get(
            f"{self.TwitterAPI}tweets/{tweet_id}{params}",
            headers={"Authorization": f"Bearer {BARRER_TOKEN}"},
        )
        tweet = json.loads(res.content)
        self.caption = f"<b>{tweet['includes']['users'][0]['name']}</b>\n{tweet['data']['text']}"

        # Iterate over the media attachments in the tweet
        for media in tweet["includes"]["media"]:
            if media["type"] in ("animated_gif", "video"):
                bitrate = [
                    a["bit_rate"] for a in media["variants"] if a["content_type"] == "video/mp4"
                ]
                media["media_key"]
                for a in media["variants"]:
                    if a["content_type"] == "video/mp4" and a["bit_rate"] == max(bitrate):
                        path = io.BytesIO((await http.get(a["url"])).content)
                        path.name = f"{media['media_key']}.{filetype.guess_extension(path)}"
            else:
                path = media["url"]
            self.files.append({"p": path, "w": media["width"], "h": media["height"]})

    async def TikTok(self, url: str, captions: str):
        path = io.BytesIO()
        with contextlib.redirect_stdout(path):
            ydl = YoutubeDL({"outtmpl": "-"})
            yt = await extract_info(ydl, url, download=True)
        path.name = yt["title"]
        self.caption = f"{yt['title']}\n\n<a href='{url}'>🔗 Link</a>"
        self.files.append(
            {
                "p": path,
                "w": yt["formats"][0]["width"],
                "h": yt["formats"][0]["height"],
            }
        )
