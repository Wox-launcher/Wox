import asyncio
import sys
import uuid

from loguru import logger

from wox import start_websocket

if len(sys.argv) != 3:
    print('Usage: wox.py <port> <logDirectory>')
    sys.exit(1)

port = int(sys.argv[1])
log_directory = (sys.argv[2])

logger.remove()
logger.add(f"{log_directory}/python.log", format="{time:YYYY-MM-DD HH:mm:ss.SSS} [{level}] {message}", rotation="100 MB", retention="3 days")

logger.info("----------------------------------------")
logger.info(f"Start python host: {uuid.uuid4()}")
logger.info(f"port: {port}")

asyncio.run(start_websocket(port))
