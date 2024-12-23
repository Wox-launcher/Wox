from typing import List
from dataclasses import dataclass, field
from enum import Enum
import json


class SelectionType(str, Enum):
    """Selection type enum"""

    TEXT = "text"
    FILE = "file"


class QueryType(str, Enum):
    """Query type enum"""

    INPUT = "input"
    SELECTION = "selection"


@dataclass
class MetadataCommand:
    """Metadata command"""

    command: str
    description: str

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Command": self.command,
                "Description": self.description,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "MetadataCommand":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            command=data.get("Command", ""),
            description=data.get("Description", ""),
        )


@dataclass
class Selection:
    """Selection model representing text or file selection"""

    type: SelectionType = field(default=SelectionType.TEXT)
    text: str = field(default="")
    file_paths: List[str] = field(default_factory=list)

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Type": self.type,
                "Text": self.text,
                "FilePaths": self.file_paths,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "Selection":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)

        if not data.get("Type"):
            data["Type"] = SelectionType.TEXT

        return cls(
            type=SelectionType(data.get("Type")),
            text=data.get("Text", ""),
            file_paths=data.get("FilePaths", []),
        )

    def __str__(self) -> str:
        """Convert selection to string"""
        if self.type == SelectionType.TEXT and self.text:
            return self.text
        elif self.type == SelectionType.FILE and self.file_paths:
            return ",".join(self.file_paths)
        return ""


@dataclass
class QueryEnv:
    """
    Query environment information
    """

    active_window_title: str = field(default="")
    """Active window title when user query"""

    active_window_pid: int = field(default=0)
    """Active window pid when user query, 0 if not available"""

    active_browser_url: str = field(default="")
    """
    Active browser url when user query
    Only available when active window is browser and https://github.com/Wox-launcher/Wox.Chrome.Extension is installed
    """

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "ActiveWindowTitle": self.active_window_title,
                "ActiveWindowPid": self.active_window_pid,
                "ActiveBrowserUrl": self.active_browser_url,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "QueryEnv":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)
        return cls(
            active_window_title=data.get("ActiveWindowTitle", ""),
            active_window_pid=data.get("ActiveWindowPid", 0),
            active_browser_url=data.get("ActiveBrowserUrl", ""),
        )


@dataclass
class Query:
    """
    Query model representing a user query
    """

    type: QueryType
    raw_query: str
    selection: Selection
    env: QueryEnv
    trigger_keyword: str = field(default="")
    command: str = field(default="")
    search: str = field(default="")

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        return json.dumps(
            {
                "Type": self.type,
                "RawQuery": self.raw_query,
                "Selection": json.loads(self.selection.to_json()),
                "Env": json.loads(self.env.to_json()),
                "TriggerKeyword": self.trigger_keyword,
                "Command": self.command,
                "Search": self.search,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "Query":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)

        if not data.get("Type"):
            data["Type"] = QueryType.INPUT

        return cls(
            type=QueryType(data.get("Type")),
            raw_query=data.get("RawQuery", ""),
            selection=Selection.from_json(data.get("Selection", Selection().to_json())),
            env=QueryEnv.from_json(data.get("Env", QueryEnv().to_json())),
            trigger_keyword=data.get("TriggerKeyword", ""),
            command=data.get("Command", ""),
            search=data.get("Search", ""),
        )

    def is_global_query(self) -> bool:
        """Check if this is a global query without trigger keyword"""
        return self.type == QueryType.INPUT and not self.trigger_keyword

    def __str__(self) -> str:
        """Convert query to string"""
        if self.type == QueryType.INPUT:
            return self.raw_query
        elif self.type == QueryType.SELECTION:
            return str(self.selection)
        return ""


@dataclass
class ChangeQueryParam:
    """Change query parameter"""

    query_type: QueryType
    query_text: str = field(default="")
    query_selection: Selection = field(default_factory=Selection)

    def to_json(self) -> str:
        """Convert to JSON string with camelCase naming"""
        data = {
            "QueryType": self.query_type,
            "QueryText": self.query_text,
        }
        if self.query_selection:
            data["QuerySelection"] = json.loads(self.query_selection.to_json())
        return json.dumps(data)

    @classmethod
    def from_json(cls, json_str: str) -> "ChangeQueryParam":
        """Create from JSON string with camelCase naming"""
        data = json.loads(json_str)

        if not data.get("QueryType"):
            data["QueryType"] = QueryType.INPUT

        return cls(
            query_type=QueryType(data.get("QueryType")),
            query_text=data.get("QueryText", ""),
            query_selection=Selection.from_json(data.get("QuerySelection", Selection().to_json())),
        )
