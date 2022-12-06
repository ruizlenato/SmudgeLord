import re
import os
import json
import contextlib
import gallery_dl

from yt_dlp import YoutubeDL
from bs4 import BeautifulSoup

from pyrogram.types import InputMediaPhoto, InputMediaVideo

from smudge.utils import aiowrap, http
from smudge.config import consumer_key, consumer_secret


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

    async def download(self, url: str, id: str):
        self.files: list = []
        if re.search(r"instagram.com\/", url, re.M):
            await self.instagram(url, id)
        elif re.search(r"tiktok.com\/", url, re.M):
            await self.TikTok(url)
        elif re.search(r"twitter.com\/", url, re.M):
            await self.Twitter(url)
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

        if swiper := soup.find_all("div", "swiper-slide"):
            for i in swiper:
                media = f"{self.cors}{i['data-src']}"
                await self.downloader(media, caption, id)
        else:
            media = f"{self.cors}{soup.find('a', 'download', href=True)['href']}"
            await self.downloader(media, caption, id)
        return self.files

    async def Twitter(self, url: str):
        caption = f"<a href='{url}'>🔗 Link</a>"
        x = re.match(".*twitter.com/.+status/([A-Za-z0-9]+)", url)
        resp = await http.post(
            "https://api.twitter.com/oauth2/token",
            auth=(consumer_key, consumer_secret),
            data={"grant_type": "client_credentials"},
        )
        headers = {"Authorization": f"Bearer {resp.json()['access_token']}"}
        res = await http.get(
            f"https://api.twitter.com/1.1/statuses/show.json?id={x[1]}",
            headers=headers,
        )
        db = json.loads(res.content)
        media = db["extended_entities"]["media"]
        if media[0]["type"] in ("animated_gif", "video"):
            bitrate = [
                a["bitrate"]
                for a in media[0]["video_info"]["variants"]
                if a["content_type"] == "video/mp4"
            ]
            for a in media[0]["video_info"]["variants"]:
                if a["content_type"] == "video/mp4" and a["bitrate"] == max(bitrate):
                    self.files.append(InputMediaVideo(a["url"], caption=caption))
        else:
            self.files.extend(
                InputMediaPhoto(a["media_url_https"], caption=caption) for a in media
            )
        return self.files

    async def TikTok(self, url: str):
        caption = f"<a href='{url}'>🔗 Link</a>"
        x = re.match(".*tiktok.com\/.*?(:?@[A-Za-z0-9]+\/video\/)?([A-Za-z0-9]+)", url)
        res = await http.get(f"https://proxitok.marcopisco.com/video/{x[2]}")
        soup = BeautifulSoup(res.text, "html.parser")
        self.files.append(
            InputMediaVideo(
                str(soup.find("a", string="No watermark")["href"]), caption=caption
            )
        )
        return self.files

    async def downloader(self, url: str, caption: str, id: str):
        with contextlib.suppress(FileExistsError):
            os.mkdir(f"./downloads/{id}/")
        path = f"./downloads/{id}/{url[90:120]}.{'mp4' if re.search(r'.mp4', url, re.M) else 'jpg'}"
        with open(path, "wb") as f:
            f.write((await http.get(url)).content)
        InputType = (
            InputMediaVideo if re.search(r".mp4", url, re.M) else InputMediaPhoto
        )
        self.files += [
            InputType(path, caption=caption),
        ]
