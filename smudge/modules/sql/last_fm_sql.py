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

import threading

from sqlalchemy import Column, String

from smudge.modules.sql import BASE, SESSION


class LastFMUsers(BASE):
    __tablename__ = "last_fm"
    user_id = Column(String(14), primary_key=True)
    username = Column(String(15))

    def __init__(self, user_id, username):
        self.user_id = user_id
        self.username = username


LastFMUsers.__table__.create(checkfirst=True)

INSERTION_LOCK = threading.RLock()


def set_user(user_id, username):
    with INSERTION_LOCK:
        user = SESSION.query(LastFMUsers).get(str(user_id))
        if not user:
            user = LastFMUsers(str(user_id), str(username))
        else:
            user.username = str(username)

        SESSION.add(user)
        SESSION.commit()


def get_user(user_id):
    user = SESSION.query(LastFMUsers).get(str(user_id))
    rep = ""
    if user:
        rep = str(user.username)

    SESSION.close()
    return rep
