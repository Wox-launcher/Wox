from typing import Optional, List
from pydantic import BaseModel

from ..types import SelectionType, QueryType


class Selection(BaseModel):
    """Selection model representing text or file selection"""

    Type: SelectionType
    Text: Optional[str] = None
    FilePaths: Optional[List[str]] = None


class QueryEnv(BaseModel):
    """
    Query environment information
    """

    ActiveWindowTitle: str
    """Active window title when user query"""

    ActiveWindowPid: int
    """Active window pid when user query, 0 if not available"""

    ActiveBrowserUrl: str
    """
    Active browser url when user query
    Only available when active window is browser and https://github.com/Wox-launcher/Wox.Chrome.Extension is installed
    """


class Query(BaseModel):
    """
    Query model representing a user query
    """

    Type: QueryType
    RawQuery: str
    TriggerKeyword: Optional[str]
    Command: Optional[str]
    Search: str
    Selection: Selection
    Env: QueryEnv

    def is_global_query(self) -> bool:
        """Check if this is a global query without trigger keyword"""
        return self.Type == QueryType.INPUT and not self.TriggerKeyword
