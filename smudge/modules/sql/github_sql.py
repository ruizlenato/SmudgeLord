import threading

from sqlalchemy import Column, String, UnicodeText, Integer, func, distinct

from smudge.modules.sql import SESSION, BASE


class GitHub(BASE):
    __tablename__ = "github"
    chat_id = Column(String(14), primary_key=True)
    name = Column(UnicodeText, primary_key=True)
    value = Column(UnicodeText, nullable=False)
    backoffset = Column(Integer, nullable=False, default=0)

    def __init__(self, chat_id, name, value, backoffset):
        self.chat_id = str(chat_id)
        self.name = name
        self.value = value
        self.backoffset = backoffset

    def __repr__(self):
        return "<Git Repo %s>" % self.name


GitHub.__table__.create(checkfirst=True)

GIT_LOCK = threading.RLock()


def add_repo_to_db(chat_id, name, value, backoffset):
    with GIT_LOCK:
        prev = SESSION.query(GitHub).get((str(chat_id), name))
        if prev:
            SESSION.delete(prev)
        repo = GitHub(str(chat_id), name, value, backoffset)
        SESSION.add(repo)
        SESSION.commit()


def get_repo(chat_id, name):
    try:
        return SESSION.query(GitHub).get((str(chat_id), name))
    finally:
        SESSION.close()


def rm_repo(chat_id, name):
    with GIT_LOCK:
        repo = SESSION.query(GitHub).get((str(chat_id), name))
        if repo:
            SESSION.delete(repo)
            SESSION.commit()
            return True
        SESSION.close()
        return False


def get_all_repos(chat_id):
    try:
        return SESSION.query(GitHub).filter(GitHub.chat_id == str(chat_id)).order_by(GitHub.name.asc()).all()
    finally:
        SESSION.close()


def num_repos():
    try:
        return SESSION.query(GitHub).count()
    finally:
        SESSION.close()


def num_chats():
    try:
        return SESSION.query(func.count(distinct(GitHub.chat_id))).scalar()
    finally:
        SESSION.close()
