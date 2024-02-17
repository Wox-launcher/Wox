import asyncio
import sys
import uuid

import logger
from host import start_websocket

if len(sys.argv) != 4:
    print('Usage: python python-host.pyz <port> <logDirectory>')
    sys.exit(1)

port = int(sys.argv[1])
log_directory = (sys.argv[2])

trace_id = f"{uuid.uuid4()}"
host_id = f"python-{uuid.uuid4()}"
logger.update_log_directory(log_directory)
logger.info(trace_id, "----------------------------------------")
logger.info(trace_id, f"start python host: {host_id}")
logger.info(trace_id, f"port: {port}")

asyncio.run(start_websocket(port))
