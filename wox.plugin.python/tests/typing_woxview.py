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
