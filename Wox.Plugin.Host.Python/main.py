#!/usr/bin/env python

import asyncio
import sys
import uuid

import websockets
from loguru import logger


def init_log(log_directory: str):
    logger.remove()
    logger.add(f"{log_directory}/python.log", format="{time:YYYY-MM-DD HH:mm:ss.SSS} [{level}] {message}", rotation="100 MB", retention="3 days")


async def handler(websocket):
    while True:
        message = await websocket.recv()
        logger.info(message)


async def main(websocket_port: int):
    async with websockets.serve(handler, "", websocket_port):
        await asyncio.Future()  # run forever


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print('Usage: main.py <port> <logDirectory>')
        sys.exit(1)

    port = int(sys.argv[1])
    init_log(sys.argv[2])

    logger.info("----------------------------------------")
    logger.info(f"Start python host: {uuid.uuid4()}")
    logger.info(f"port: {port}")

    asyncio.run(main(port))
