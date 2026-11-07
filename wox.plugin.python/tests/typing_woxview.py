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

bar: WoxViewTree = {"version": 1, "root": {"type": "bar-chart", "props": {"values": [-5, 10], "pointLabels": ["loss", "profit"], "chart": {"barWidth": 0.7, "showGrid": True}}}}
scatter: WoxViewTree = {"version": 1, "root": {"type": "scatter-chart", "props": {"points": [{"x": 2, "y": 4, "label": "walk"}], "chart": {"showXAxis": True, "xValueSuffix": " h"}}}}
pie: WoxViewTree = {"version": 1, "root": {"type": "pie-chart", "props": {"values": [60, 40], "chart": {"innerRadius": 0.5, "colors": ["$accent", "$textSecondary"]}}}}

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
