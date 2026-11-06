"""Static typing examples for the shared WoxView wire contract."""

from wox_plugin import WoxViewEventData, WoxViewTree
from wox_plugin.models.woxview import WoxViewImageProps

counter: WoxViewTree = {
    "version": 1,
    "root": {
        "type": "column",
        "children": [
            {"type": "text", "props": {"text": "Count: 0"}},
            {"type": "button", "key": "increment", "props": {"label": "Add", "onPress": {"$handler": "increment"}}},
        ],
    },
}
event: WoxViewEventData = {"Handler": "increment", "Args": [], "Key": "increment"}

bad_image: WoxViewImageProps = {}  # type: ignore[assignment]

stopwatch: WoxViewTree = {
    "version": 1,
    "root": {"type": "timer", "props": {"direction": "up", "format": "HH:MM:SS", "elapsed": 1234, "running": True, "animate": True}},
}
countdown: WoxViewTree = {
    "version": 1,
    "root": {
        "type": "timer",
        "props": {
            "direction": "down",
            "format": "MM:SS.SSS",
            "duration": 1500000,
            "elapsed": 120000,
            "running": True,
            "paused": False,
            "playbackId": "focus-1",
            "onComplete": {"$handler": "done"},
        },
    },
}
