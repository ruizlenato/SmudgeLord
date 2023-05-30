# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)
import aiosqlite
from config import DATABASE_PATH

from smudge.utils.logger import log


class Database:
    def __init__(self):
        self.conn: aiosqlite.Connection = None
        self.path: str = DATABASE_PATH
        self.is_connected: bool = False

    async def connect(self):
        # Open the connection
        conn = await aiosqlite.connect(self.path)

        # Define the tables
        await conn.executescript(
            """
        CREATE TABLE IF NOT EXISTS users(
            id INTEGER PRIMARY KEY,
            language TEXT DEFAULT 'en_US',
            medias_captions INTEGER
        );

        CREATE TABLE IF NOT EXISTS chats(
            id INTEGER PRIMARY KEY,
            language TEXT DEFAULT 'en_US',
            medias_adownloads INTEGER DEFAULT 1,
            medias_captions INTEGER
        );

        CREATE TABLE IF NOT EXISTS afk(
            user_id INTEGER,
            reason TEXT,
            time INTEGER
        );
        """
        )

        # Enable VACUUM and WAL
        await conn.execute("VACUUM")
        await conn.execute("PRAGMA journal_mode=WAL")

        # Update the database
        await conn.commit()

        conn.row_factory = aiosqlite.Row

        self.conn = conn
        self.is_connected: bool = True
        log.info("\033[92mThe database has been connected.\033[0m")

    async def close(self):
        # Close the connection
        await self.conn.close()

        self.is_connected: bool = False
        log.warning("\033[93mThe database was closed.\033[0m")

    def get_conn(self) -> aiosqlite.Connection:
        if not self.is_connected:
            raise RuntimeError("The database is not connected.")

        return self.conn


database = Database()
