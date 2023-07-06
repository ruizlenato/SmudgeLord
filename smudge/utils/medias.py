# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import contextlib
import io
import json
import re
import uuid

import esprima
import filetype
from bs4 import BeautifulSoup as bs
from httpx import AsyncClient
from yt_dlp import YoutubeDL

from ..config import config
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

    async def download(self, url: str, captions: bool):
        self.files: list = []
        if re.search(r"instagram.com/", url):
            await self.Instagram(url, captions)
        elif re.search(r"tiktok.com/", url):
            await self.TikTok(url, captions)
        elif re.search(r"twitter.com/", url):
            await self.Twitter(url, captions)
        elif re.search(r"threads.net/", url):
            await self.Threads(url, captions)

        if not captions:
            self.caption = f"<a href='{url}'>🔗 Link</a>"
        return self.files, self.caption

    async def httpx(self, url: str):
        if (await http.get(url)).status_code != 200:
            for proxy in config["PROXIES"]:
                http_client = AsyncClient(proxies=proxy)
                response = await http_client.get(url)
                if response.status_code == 200:
                    break
            return http_client
        else:  # noqa: RET505
            return http

    async def Instagram(self, url: str, captions: str):
        headers = {
            "authority": "www.instagram.com",
            "accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp\
,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.9",
            "accept-language": "en-US,en;q=0.9",
            "cache-control": "max-age=0",
            "sec-fetch-mode": "navigate",
            "upgrade-insecure-requests": "1",
            "referer": "https://www.instagram.com/",
            "user-agent": "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 \
(KHTML, like Gecko) Chrome/100.0.4896.60 Safari/537.36",
            "viewport-width": "1280",
        }

        post_id = re.findall(r"/(?:reel|p)/([a-zA-Z0-9_-]+)/", url)[0]

        httpx = await self.httpx("https://www.instagram.com/")

        r = await httpx.get(
            f"https://www.instagram.com/p/{post_id}/embed/captioned",
            headers=headers,
            follow_redirects=True,
        )
        soup = bs(r.text, "html.parser")
        medias = []

        if soup.find("div", {"data-media-type": "GraphImage"}):
            caption = re.sub(
                r'.*</a><br/><br/>(.*)(<div class="CaptionComments">.*)',
                r"\1",
                str(soup.find("div", {"class": "Caption"})),
            ).replace("<br/>", "\n")
            self.caption = f"{caption}\n<a href='{url}'>🔗 Link</a>"
            file = soup.find("img", {"class": "EmbeddedMediaImage"}).get("src")
            medias.append({"p": file, "w": 0, "h": 0})

        data = re.findall(r'<script>(requireLazy\(\["TimeSliceImpl".*)<\/script>', r.text)
        if data and "shortcode_media" in data[0]:
            tokenized = esprima.tokenize(data[0])
            for token in tokenized:
                if "shortcode_media" in token.value:
                    jsoninsta = json.loads(json.loads(token.value))["gql_data"]["shortcode_media"]

                    if caption := jsoninsta["edge_media_to_caption"]["edges"]:
                        self.caption = f"{caption[0]['node']['text']}\n<a href='{url}'>🔗 Link</a>"
                    else:
                        self.caption = f"\n<a href='{url}'>🔗 Link</a>"

                    if jsoninsta["__typename"] == "GraphVideo":
                        url = jsoninsta["video_url"]
                        dimensions = jsoninsta["dimensions"]
                        medias.append(
                            {"p": url, "w": dimensions["width"], "h": dimensions["height"]}
                        )
                    else:
                        for post in jsoninsta["edge_sidecar_to_children"]["edges"]:
                            url = post["node"]["display_url"]
                            if post["node"]["is_video"] is True:
                                with contextlib.suppress(KeyError):
                                    url = post["node"]["video_url"]
                            dimensions = post["node"]["dimensions"]
                            medias.append(
                                {"p": url, "w": dimensions["width"], "h": dimensions["height"]}
                            )
        else:
            r = await httpx.get(
                f"https://www.instagram.com/p/{post_id}/",
                headers=headers,
            )
            soup = bs(r.text, "html.parser")
            if content := soup.find("script", type="application/ld+json"):
                data = json.loads(content.contents[0])
            else:
                return

            if video := data["video"]:
                if len(video) == 1:
                    url = video[0]["contentUrl"]
                    medias.append(
                        {"p": url, "w": int(video[0]["width"]), "h": int(video[0]["height"])}
                    )
                else:
                    for v in video:
                        url = v["contentUrl"]
                        medias.append({"p": url, "w": v["width"], "h": v["height"]})

        for m in medias:
            file = io.BytesIO((await httpx.get(m["p"])).content)
            file.name = f"{m['p'][60:80]}.{filetype.guess_extension(file)}"
            self.files.append({"p": file, "w": m["w"], "h": m["h"]})
        return

    async def Twitter(self, url: str, captions: str):
        # Twitter Bearer Token
        bearer: str = "Bearer AAAAAAAAAAAAAAAAAAAAAPYXBAAAAAAACLXUNDekMxqa8h%2F40K4moUkGsoc%3DTYfb\
DKbT3jJPCEVnMYqilB28NHfOPqkca3qaAxGfsyKCs0wRbw"
        # Extract the tweet ID from the URL
        tweet_id = re.match(".*twitter.com/.+status/([A-Za-z0-9]+)", url)[1]
        params: str = ".json?tweet_mode=extended&cards_platform=Web-12&include_cards=1\
&include_user_entities=0"
        csrfToken = str(uuid.uuid4()).replace("-", "")
        res = (
            await http.get(
                f"https://api.twitter.com/1.1/statuses/show/{tweet_id}{params}",
                headers={
                    "Authorization": bearer,
                    "Cookie": f"auth_token=ee4ebd1070835b90a9b8016d1e6c6130ccc89637;\
 ct0={csrfToken};",
                    "x-twitter-active-user": "yes",
                    "x-twitter-auth-type": "OAuth2Session",
                    "x-csrf-token": csrfToken,
                    "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0)\
 Gecko/20100101 Firefox/116.0",
                },
            )
        ).json()

        self.caption = f"<b>{res['user']['screen_name']}</b>\n{res['full_text']}"
        for media in res["extended_entities"]["media"]:
            width = media["original_info"]["width"]
            height = media["original_info"]["height"]
            if media["type"] == "photo":
                path = io.BytesIO((await http.get(media["media_url_https"])).content)
                path.name = f"{media['id_str']}.{filetype.guess_extension(path)}"
            else:
                bitrate = [
                    a["bitrate"]
                    for a in media["video_info"]["variants"]
                    if a["content_type"] == "video/mp4"
                ]
                for a in media["video_info"]["variants"]:
                    if a["content_type"] == "video/mp4" and a["bitrate"] == max(bitrate):
                        path = io.BytesIO((await http.get(a["url"])).content)
                        path.name = f"{media['id_str']}.{filetype.guess_extension(path)}"

        self.files.append({"p": path, "w": width, "h": height})

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

    async def Threads(self, url: str, captions: str):
        httpx = await self.httpx("https://www.threads.net/")

        post_id = re.findall("/t/([a-zA-Z0-9_-]+)", url)[0]
        r = await httpx.get(
            f"https://www.threads.net/t/{post_id}/embed/",
            follow_redirects=True,
        )
        soup = bs(r.text, "html.parser")

        medias = []
        self.caption = f"<a href='{url}'>🔗 Link</a>"

        # Scrapper of the post js to get the Thread caption.
        pattern = re.compile(r'\[\["__markup.+{"__html".+span>"},1]]')
        script_tag = soup.find("script", text=re.compile(pattern))
        if script_tag:
            matches = re.findall(pattern, script_tag.string)[0]
            data = json.loads(matches)
            if comment := re.findall(r"<span.+>(.+)</span>", str(data[0][1])):
                self.caption = f"{comment[0]}\n<a href='{url}'>🔗 Link</a>"

        # This method is for when a media has more than one media.
        if find_all := soup.find_all("div", class_="MediaScrollImageContainer"):
            for div in find_all:
                if video := div.find("video"):
                    url = video.find("source").get("src")
                if image := div.find("img", class_="img"):
                    url = image.get("src")
                medias.append({"p": url, "w": 0, "h": 0})

        # This method only works when a "Thread" has only one medium.
        if div := soup.find("div", class_="SingleInnerMediaContainer"):
            if video := div.find("video"):
                url = video.find("source").get("src")
            if image := div.find("img", class_="img"):
                url = image.get("src")
            medias.append({"p": url, "w": 0, "h": 0})

        for m in medias:
            file = io.BytesIO((await httpx.get(m["p"])).content)
            file.name = f"{m['p'][60:80]}.{filetype.guess_extension(file)}"
            self.files.append({"p": file, "w": m["w"], "h": m["h"]})
        return
