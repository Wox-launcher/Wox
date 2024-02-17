from loguru import logger as loguru_logger


def update_log_directory(log_directroy: str) -> None:
    loguru_logger.remove()
    loguru_logger.add(f"{log_directroy}/python.log", format="{time:YYYY-MM-DD HH:mm:ss.SSS} [{level}] {message}", rotation="100 MB", retention="3 days")


def debug(trace_id: str, message: str) -> None:
    __inner_log(trace_id, message, "debug")


def info(trace_id: str, message: str) -> None:
    __inner_log(trace_id, message, "info")


def error(trace_id: str, message: str) -> None:
    __inner_log(trace_id, message, "error")


def __inner_log(trace_id: str, message: str, level: str) -> None:
    loguru_logger.log(__level=level, __message=f"{trace_id} {message}")
