import asyncio
import sys
import uuid
import os

import logger
from host import start_websocket

if len(sys.argv) != 4:
    print('Usage: python python-host.pyz <port> <logDirectory> <woxPid>')
    sys.exit(1)

port = int(sys.argv[1])
log_directory = sys.argv[2]
wox_pid = int(sys.argv[3])

trace_id = str(uuid.uuid4())
host_id = f"python-{uuid.uuid4()}"
logger.update_log_directory(log_directory)

def check_wox_process():
    """Check if Wox process is still alive"""
    try:
        os.kill(wox_pid, 0)
        return True
    except OSError:
        return False

async def monitor_wox_process():
    """Monitor Wox process and exit if it's not alive"""
    await logger.info(trace_id, "start monitor wox process")
    while True:
        if not check_wox_process():
            await logger.error(trace_id, "wox process is not alive, exit")
            sys.exit(1)
        await asyncio.sleep(1)

async def main():
    """Main function"""
    # Log startup information
    await logger.info(trace_id, "----------------------------------------")
    await logger.info(trace_id, f"start python host: {host_id}")
    await logger.info(trace_id, f"port: {port}")
    await logger.info(trace_id, f"wox pid: {wox_pid}")
    
    # Start tasks
    monitor_task = asyncio.create_task(monitor_wox_process())
    websocket_task = asyncio.create_task(start_websocket(port))
    await asyncio.gather(monitor_task, websocket_task)

asyncio.run(main())
