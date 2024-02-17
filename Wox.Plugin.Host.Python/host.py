#!/usr/bin/env python

import asyncio
import pkgutil

import websockets
from loguru import logger


async def handler(websocket):
    while True:
        message = await websocket.recv()
        logger.info(message)
        # my_module = importlib.import_module('os.path')


async def start_websocket(websocket_port: int):
    async with websockets.serve(handler, "", websocket_port):
        await asyncio.Future()  # run forever


def load_plugin():
    pkgutil.iter_modules(['plugins'])
