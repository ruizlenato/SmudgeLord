import re
import os
import json
import contextlib
import gallery_dl

from yt_dlp import YoutubeDL
from bs4 import BeautifulSoup
from urllib.parse import unquote

from pyrogram.types import InputMediaPhoto, InputMediaVideo

from ..utils import aiowrap, http
from ..config import BARRER_TOKEN


@aiowrap
def gallery_down(path, url: str):
    gallery_dl.config.set(("output",), "mode", "null")
    gallery_dl.config.set((), "directory", [])
    gallery_dl.config.set((), "base-directory", [path])
    gallery_dl.config.load()
    return gallery_dl.job.DownloadJob(url).run()


@aiowrap
def extract_info(instance: YoutubeDL, url: str, download=True):
    return instance.extract_info(url, download)


async def search_yt(query):
    page = await http.get(
        "https://www.youtube.com/results",
        params=dict(search_query=query, pbj="1"),
        headers={
            "x-youtube-Smudge-name": "1",
            "x-youtube-Smudge-version": "2.20200827",
        },
    )
    page = json.loads(page.content)
    list_videos = []
    for video in page[1]["response"]["contents"]["twoColumnSearchResultsRenderer"][
        "primaryContents"
    ]["sectionListRenderer"]["contents"][0]["itemSectionRenderer"]["contents"]:
        if video.get("videoRenderer"):
            dic = {
                "title": video["videoRenderer"]["title"]["runs"][0]["text"],
                "url": "https://www.youtube.com/watch?v="
                + video["videoRenderer"]["videoId"],
            }
            list_videos.append(dic)
    return list_videos


class DownloadMedia:
    def __init__(self):
        self.cors: str = "https://cors-bypass.amanoteam.com/"
        self.TwitterAPI: str = "https://api.twitter.com/2/"

    async def download(self, url: str, id: str):
        self.files: list = []
        if re.search(r"instagram.com\/", url, re.M):
            await self.instagram(url, id)
        elif re.search(r"tiktok.com\/", url, re.M):
            await self.TikTok(url)
        elif re.search(r"twitter.com\/", url, re.M):
            await self.Twitter(url, id)
        return self.files

    async def instagram(self, url: str, id: str):
        caption = f"<a href='{url}'>🔗 Link</a>"
        url = re.sub(
            r"(?:www.|m.)?instagram.com/(?:reel|p)(.*)/", r"imginn.com/p\1/", url
        )
        res = await http.get(f"{self.cors}{url}")

        if res.status_code != 200:
            url = re.sub(r"imginn.com", r"imginn.org", url)
            res = await http.get(f"{url}")

        soup = BeautifulSoup(res.text, "html.parser")
        with contextlib.suppress(FileExistsError):
            os.mkdir(f"./downloads/{id}/")
        if swiper := soup.find_all("div", "swiper-slide"):
            for i in swiper:
                urlmedia = re.sub(r".*url=", r"", unquote(i["data-src"]))
                media = f"{self.cors}{urlmedia}"
                path = f"./downloads/{id}/{media[90:120]}.{'mp4' if re.search(r'.mp4', media, re.M) else 'jpg'}"
                await self.downloader(media, caption, path)
        else:
            media = f"{self.cors}{soup.find('a', 'download', href=True)['href']}"
            path = f"./downloads/{id}/{media[90:120]}.{'mp4' if re.search(r'.mp4', media, re.M) else 'jpg'}"
            await self.downloader(media, caption, path)
        return self.files

    async def Twitter(self, url: str, id: str):
        caption = f"<a href='{url}'>🔗 Link</a>"
        x = re.match(".*twitter.com/.+status/([A-Za-z0-9]+)", url)
        params: str = "?expansions=attachments.media_keys,author_id&media.fields=type,variants,url&tweet.fields=entities"
        res = await http.get(
            f"{self.TwitterAPI}tweets/{x[1]}{params}",
            headers={"Authorization": f"Bearer {BARRER_TOKEN}"},
        )
        tweet = json.loads(res.content)

        for media in tweet["includes"]["media"]:
            if media["type"] in ("animated_gif", "video"):
                key = media["media_key"]
                bitrate = [
                    a["bit_rate"]
                    for a in media["variants"]
                    if a["content_type"] == "video/mp4"
                ]
                for a in media["variants"]:
                    with contextlib.suppress(FileExistsError):
                        os.mkdir(f"./downloads/{id}/")
                    if a["content_type"] == "video/mp4" and a["bit_rate"] == max(
                        bitrate
                    ):
                        path = f"./downloads/{id}/{key}.mp4"
                        await self.downloader(a["url"], caption, path)
            else:
                self.files += [InputMediaPhoto(media["url"], caption=caption)]

        return self.files

    async def TikTok(self, url: str):
        caption = f"<a href='{url}'>🔗 Link</a>"
        x = re.match(r".*tiktok.com\/.*?(:?@[A-Za-z0-9]+\/video\/)?([A-Za-z0-9]+)", url)
        res = await http.get(f"https://proxitok.marcopisco.com/video/{x[2]}")
        soup = BeautifulSoup(res.text, "html.parser")
        self.files.append(
            InputMediaVideo(
                str(soup.find("a", string="No watermark")["href"]), caption=caption
            )
        )
        return self.files

    async def downloader(self, url: str, caption: str, path: str):
        with open(path, "wb") as f:
            f.write((await http.get(url)).content)
        InputType = (
            InputMediaVideo if re.search(r".mp4", path, re.M) else InputMediaPhoto
        )
        if len(self.files) > 0:
            self.files.append(InputType(path))
        else:
            self.files.append(InputType(path, caption=caption))
