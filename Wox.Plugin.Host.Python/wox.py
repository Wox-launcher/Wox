#!/usr/bin/env python

import asyncio

import websockets
from loguru import logger


async def handler(websocket):
    while True:
        message = await websocket.recv()
        logger.info(message)


async def start_websocket(websocket_port: int):
    async with websockets.serve(handler, "", websocket_port):
        await asyncio.Future()  # run forever
