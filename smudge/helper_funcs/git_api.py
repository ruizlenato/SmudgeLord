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

import urllib.request as url
import json
import datetime

VERSION = "1.0.2"
APIURL = "http://api.github.com/repos/"


def vercheck() -> str:
    return str(VERSION)


# Repo-wise stuff


def getData(repoURL):
    try:
        with url.urlopen(APIURL + repoURL + "/releases") as data_raw:
            repoData = json.loads(data_raw.read().decode())
            return repoData
    except:
        return None


def getReleaseData(repoData, index):
    if index < len(repoData):
        return repoData[index]
    else:
        return None


# Release-wise stuff


def getAuthor(releaseData):
    if releaseData is None:
        return None
    return releaseData['author']['login']


def getAuthorUrl(releaseData):
    if releaseData is None:
        return None
    return releaseData['author']['html_url']


def getReleaseName(releaseData):
    if releaseData is None:
        return None
    return releaseData['name']


def getReleaseDate(releaseData):
    if releaseData is None:
        return None
    return releaseData['published_at']


def getAssetsSize(releaseData):
    if releaseData is None:
        return None
    return len(releaseData['assets'])


def getAssets(releaseData):
    if releaseData is None:
        return None
    return releaseData['assets']


def getBody(releaseData):  # changelog stuff
    if releaseData is None:
        return None
    return releaseData['body']


# Asset-wise stuff


def getReleaseFileName(asset):
    return asset['name']


def getReleaseFileURL(asset):
    return asset['browser_download_url']


def getDownloadCount(asset):
    return asset['download_count']


def getSize(asset):
    return asset['size']
