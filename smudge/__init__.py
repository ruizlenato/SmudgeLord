# SPDX-License-Identifier: GPL-3.0
# Copyright (c) 2023 Luiz Renato (ruizlenato@proton.me)

from subprocess import run

__version__ = (
    run(["git", "rev-parse", "--short", "HEAD"], capture_output=True)
    .stdout.decode("utf-8")
    .strip()
)
