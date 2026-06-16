"""
Wox Query Response Models

This module defines the structured response returned by plugin.query().
QueryResponse keeps result rows together with query-scoped refinements and
layout hints so the host can forward one normalized payload to Wox core.
QueryResponse requires Wox >= 2.0.4; plugins that need to run on older Wox
releases should return List[Result] directly instead.
"""

import json
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Dict, List, Optional

from .image import WoxImage
from .result import Result


class QueryRefinementType(str, Enum):
    """Available control shapes for query refinements."""

    SINGLE_SELECT = "singleSelect"
    MULTI_SELECT = "multiSelect"
    TOGGLE = "toggle"
    SORT = "sort"


@dataclass
class QueryRefinementOption:
    """One selectable value inside a query refinement control."""

    value: str
    title: str
    icon: Optional[WoxImage] = None
    keywords: List[str] = field(default_factory=list)
    count: Optional[int] = None

    def to_json(self) -> str:
        data: Dict[str, Any] = {
            "Value": self.value,
            "Title": self.title,
            "Keywords": self.keywords,
        }
        if self.icon is not None:
            data["Icon"] = json.loads(self.icon.to_json())
        if self.count is not None:
            data["Count"] = self.count
        return json.dumps(data)


@dataclass
class QueryRefinement:
    """Query-scoped UI control returned alongside results."""

    id: str
    title: str
    type: QueryRefinementType
    hotkey: str
    options: List[QueryRefinementOption] = field(default_factory=list)
    default_value: List[str] = field(default_factory=list)
    persist: bool = False

    def to_json(self) -> str:
        return json.dumps(
            {
                "Id": self.id,
                "Title": self.title,
                "Type": self.type,
                "Options": [json.loads(option.to_json()) for option in self.options],
                "DefaultValue": self.default_value,
                "Hotkey": self.hotkey,
                "Persist": self.persist,
            }
        )


@dataclass
class QueryGridLayout:
    """
    Optional grid presentation hints for the current query response.

    Prefer this QueryResponse layout field over plugin.json gridLayout. The
    metadata feature is deprecated because it only describes static plugin or
    command defaults, while QueryResponse can choose the layout per query.
    """

    columns: int = 0
    image_width: int = 0
    image_height: int = 0
    item_padding: int = 0
    item_margin: int = 0
    aspect_ratio: float = 0.0
    commands: List[str] = field(default_factory=list)

    def to_json(self) -> str:
        return json.dumps(
            {
                "Columns": self.columns,
                "ImageWidth": self.image_width,
                "ImageHeight": self.image_height,
                "ItemPadding": self.item_padding,
                "ItemMargin": self.item_margin,
                "AspectRatio": self.aspect_ratio,
                "Commands": self.commands,
            }
        )


@dataclass
class QueryLayout:
    """
    Optional presentation hints that apply to one query response.

    Use this object for result preview width and grid layout. The older
    plugin.json resultPreviewWidthRatio and gridLayout metadata features are
    deprecated and remain only for compatibility with existing plugins.
    """

    icon: Optional[WoxImage] = None
    result_preview_width_ratio: Optional[float] = None
    grid_layout: Optional[QueryGridLayout] = None

    def to_json(self) -> str:
        data: Dict[str, Any] = {}
        if self.icon is not None:
            data["Icon"] = json.loads(self.icon.to_json())
        if self.result_preview_width_ratio is not None:
            # Zero is a valid preview-only ratio. Omitting the field means
            # unset; sending 0.0 means the plugin intentionally overrides it.
            data["ResultPreviewWidthRatio"] = self.result_preview_width_ratio
        if self.grid_layout is not None:
            data["GridLayout"] = json.loads(self.grid_layout.to_json())
        return json.dumps(data)


@dataclass
class QueryResponse:
    """
    Complete response from plugin.query().

    Requires Wox >= 2.0.4. Set plugin.json MinWoxVersion to at least 2.0.4
    when returning this shape.

    Returning List[Result] directly is still accepted by the host for
    compatibility, but it is deprecated because it cannot carry refinements or
    layout hints with the same query update.
    """

    results: List[Result] = field(default_factory=list)
    refinements: List[QueryRefinement] = field(default_factory=list)
    layout: QueryLayout = field(default_factory=QueryLayout)

    def to_json(self) -> str:
        return json.dumps(
            {
                "Results": [json.loads(result.to_json()) for result in self.results],
                "Refinements": [json.loads(refinement.to_json()) for refinement in self.refinements],
                "Layout": json.loads(self.layout.to_json()),
            }
        )
