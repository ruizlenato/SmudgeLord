import asyncio
from collections.abc import Callable
from functools import partial, wraps

import httpx

timeout = httpx.Timeout(30, pool=None)
http = httpx.AsyncClient(http2=True, timeout=timeout)


def aiowrap(func: Callable) -> Callable:
    @wraps(func)
    async def run(*args, loop=None, executor=None, **kwargs):
        if loop is None:
            loop = asyncio.get_event_loop()
        pfunc = partial(func, *args, **kwargs)
        return await loop.run_in_executor(executor, pfunc)

    return run
