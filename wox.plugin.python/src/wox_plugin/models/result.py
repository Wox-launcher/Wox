from typing import Optional, List, Dict, Callable, Awaitable
from pydantic import BaseModel

from ..types import WoxImageType, WoxPreviewType, ResultTailType


class WoxImage(BaseModel):
    """Image model for Wox"""

    ImageType: WoxImageType
    ImageData: str


class WoxPreview(BaseModel):
    """Preview model for Wox results"""

    PreviewType: WoxPreviewType
    PreviewData: str
    PreviewProperties: Dict[str, str]


class ResultTail(BaseModel):
    """Tail model for Wox results"""

    Type: ResultTailType
    Text: Optional[str] = None
    Image: Optional[WoxImage] = None


class ActionContext(BaseModel):
    """Context for result actions"""

    ContextData: str


class ResultAction(BaseModel):
    """Action model for Wox results"""

    Name: str
    Action: Callable[[ActionContext], Awaitable[None]]
    Id: Optional[str] = None
    Icon: Optional[WoxImage] = None
    IsDefault: Optional[bool] = None
    PreventHideAfterAction: Optional[bool] = None
    Hotkey: Optional[str] = None

    class Config:
        arbitrary_types_allowed = True


class Result(BaseModel):
    """Result model for Wox"""

    Title: str
    Icon: WoxImage
    Id: Optional[str] = None
    SubTitle: Optional[str] = None
    Preview: Optional[WoxPreview] = None
    Score: Optional[float] = None
    Group: Optional[str] = None
    GroupScore: Optional[float] = None
    Tails: Optional[List[ResultTail]] = None
    ContextData: Optional[str] = None
    Actions: Optional[List[ResultAction]] = None
    RefreshInterval: Optional[int] = None
    OnRefresh: Optional[
        Callable[["RefreshableResult"], Awaitable["RefreshableResult"]]
    ] = None

    class Config:
        arbitrary_types_allowed = True


class RefreshableResult(BaseModel):
    """Result that can be refreshed periodically"""

    Title: str
    SubTitle: str
    Icon: WoxImage
    Preview: WoxPreview
    Tails: List[ResultTail]
    ContextData: str
    RefreshInterval: int
    Actions: List[ResultAction]

    def __await__(self):
        # Make RefreshableResult awaitable by returning itself
        async def _awaitable():
            return self

        return _awaitable().__await__()
