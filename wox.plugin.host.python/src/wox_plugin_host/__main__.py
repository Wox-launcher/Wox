import asyncio
import sys
import uuid
import os
import platform
from typing import Optional

from . import logger
from .host import start_websocket

if len(sys.argv) != 4:
    print("Usage: python python-host.pyz <port> <logDirectory> <woxPid>")
    sys.exit(1)

port = int(sys.argv[1])
log_directory = sys.argv[2]
wox_pid = int(sys.argv[3])

trace_id = str(uuid.uuid4())
host_id = f"python-{uuid.uuid4()}"
logger.update_log_directory(log_directory)
wox_process_handle: Optional[int] = None


def check_wox_process() -> bool:
    """Check if Wox process is still alive"""
    if platform.system() != "Windows":
        try:
            os.kill(wox_pid, 0)
            return True
        except OSError:
            return False

    # Prefer checking the original Wox process handle. This avoids PID-reuse false positives.
    if wox_process_handle:
        import ctypes  # type: ignore[import-not-found]

        WAIT_OBJECT_0 = 0x00000000
        WAIT_TIMEOUT = 0x00000102

        wait_result = ctypes.windll.kernel32.WaitForSingleObject(wox_process_handle, 0)  # type: ignore[attr-defined]
        if wait_result == WAIT_TIMEOUT:
            return True
        if wait_result == WAIT_OBJECT_0:
            return False

    # Fallback Windows PID-based check (for handle init failure case).
    import ctypes  # type: ignore[import-not-found]
    import ctypes.wintypes  # type: ignore[import-not-found]

    PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
    handle = ctypes.windll.kernel32.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, False, ctypes.wintypes.DWORD(wox_pid))  # type: ignore[attr-defined]
    if handle:
        ctypes.windll.kernel32.CloseHandle(handle)  # type: ignore[attr-defined]
        return True

    return False


async def monitor_wox_process() -> None:
    """Monitor Wox process and exit if it's not alive"""
    await logger.info(trace_id, "start monitor wox process")
    while True:
        if not check_wox_process():
            await logger.error(trace_id, "wox process is not alive, exit")
            sys.exit(1)
        await asyncio.sleep(1)


async def main() -> None:
    """Main function"""
    global wox_process_handle

    if platform.system() == "Windows":
        import ctypes  # type: ignore[import-not-found]
        import ctypes.wintypes  # type: ignore[import-not-found]

        PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
        SYNCHRONIZE = 0x00100000
        wox_process_handle = ctypes.windll.kernel32.OpenProcess(  # type: ignore[attr-defined]
            PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE, False, ctypes.wintypes.DWORD(wox_pid)
        )
        if not wox_process_handle:
            await logger.error(trace_id, "failed to get wox process handle, fallback to PID check")

    # Log startup information

    await logger.info(trace_id, "----------------------------------------")
    await logger.info(trace_id, f"start python host: {host_id}")
    await logger.info(trace_id, f"port: {port}")
    await logger.info(trace_id, f"wox pid: {wox_pid}")

    # Start tasks
    monitor_task = asyncio.create_task(monitor_wox_process())
    websocket_task = asyncio.create_task(start_websocket(port))
    try:
        await asyncio.gather(monitor_task, websocket_task)
    finally:
        if platform.system() == "Windows" and wox_process_handle:
            import ctypes  # type: ignore[import-not-found]

            ctypes.windll.kernel32.CloseHandle(wox_process_handle)  # type: ignore[attr-defined]
            wox_process_handle = None


def run() -> None:
    asyncio.run(main())
