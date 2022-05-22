# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2021-2022 Luiz Renato (ruizlenato@protonmail.com)
import logging

import aiosqlite

from rich import print

logger = logging.getLogger(__name__)

DATABASE_PATH = "./smudge/database/database.db"


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
            lang TEXT DEFAULT 'en-US',
            lastfm_username TEXT,
            spot_access_token TEXT,
            spot_refresh_token TEXT,
            afk_reason TEXT
        );
        CREATE TABLE IF NOT EXISTS groups(
            id INTEGER PRIMARY KEY,
            lang TEXT DEFAULT 'en-US',
            sdl_auto INTEGER
        );
        CREATE TABLE IF NOT EXISTS channels(
            id INTEGER PRIMARY KEY,
            lang TEXT DEFAULT 'en-US'
        );
        """
        )

        # Enable VACUUM
        await conn.execute("VACUUM")

        # Enable WAL
        await conn.execute("PRAGMA journal_mode=WAL")

        # Update the database
        await conn.commit()

        conn.row_factory = aiosqlite.Row

        self.conn = conn
        self.is_connected: bool = True

        print("[green]The database has been connected.")

    async def close(self):
        # Close the connection
        await self.conn.close()

        self.is_connected: bool = False

        logger.info("The database was closed.")

    def get_conn(self) -> aiosqlite.Connection:
        if not self.is_connected:
            raise RuntimeError("The database is not connected.")

        return self.conn


database = Database()
