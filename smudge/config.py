import json
from pathlib import Path


def load_config():
    global config
    with Path.open("config.json", encoding="utf-8") as f:
        config = json.load(f)
        return config


config = load_config()
