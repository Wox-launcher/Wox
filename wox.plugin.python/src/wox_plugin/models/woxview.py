"""Generated WoxView wire types. Keys match the JSON protocol."""

from __future__ import annotations

from typing import Any, Literal, NoReturn, TypeAlias, TypedDict


class WoxViewAnimationRequired(TypedDict):
    type: Literal["pulse", "spin", "shimmer"]


class WoxViewAnimation(WoxViewAnimationRequired, total=False):
    duration: int | None


class WoxViewColorVariant3(TypedDict):
    dark: str
    light: str


class WoxViewEdgesVariant2(TypedDict, total=False):
    bottom: WoxViewSize
    horizontal: WoxViewSize
    left: WoxViewSize
    right: WoxViewSize
    top: WoxViewSize
    vertical: WoxViewSize


class WoxViewEventData(TypedDict):
    Args: list[Any]
    Handler: str
    Key: str


WoxViewHandler = TypedDict("WoxViewHandler", {"$handler": str})


class WoxViewPresenceAnimationVariant2Required(TypedDict):
    type: Literal["fade", "slideUp", "scale"]


class WoxViewPresenceAnimationVariant2(WoxViewPresenceAnimationVariant2Required, total=False):
    duration: int | None


class WoxViewStateStyle(TypedDict, total=False):
    backgroundColor: WoxViewColor | None
    borderBottomWidth: WoxViewSize | None
    borderColor: WoxViewColor | None
    borderLeftWidth: WoxViewSize | None
    borderRadius: WoxViewSize | None
    borderRightWidth: WoxViewSize | None
    borderTopWidth: WoxViewSize | None
    borderWidth: WoxViewSize | None
    color: WoxViewColor | None
    opacity: float | None


class WoxViewStyle(TypedDict, total=False):
    alignItems: Literal["", "start", "center", "end", "stretch"]
    alignSelf: Literal["", "start", "center", "end", "stretch"]
    aspectRatio: float | None
    backgroundColor: WoxViewColor
    borderBottomWidth: WoxViewSize | None
    borderColor: WoxViewColor
    borderLeftWidth: WoxViewSize | None
    borderRadius: WoxViewSize
    borderRightWidth: WoxViewSize | None
    borderTopWidth: WoxViewSize | None
    borderWidth: WoxViewSize
    bottom: WoxViewSize | None
    color: WoxViewColor
    flex: float
    floating: bool
    fontFamily: Literal["", "ui", "monospace"]
    fontSize: WoxViewSize
    fontWeight: Literal["", "regular", "semibold", "bold"]
    gap: WoxViewSize
    height: WoxViewSize
    italic: bool
    justifyContent: Literal["", "start", "center", "end", "space-between"]
    left: WoxViewSize | None
    lineHeight: WoxViewSize
    margin: WoxViewEdges
    maxHeight: WoxViewSize
    maxLines: int
    maxWidth: WoxViewSize
    minHeight: WoxViewSize
    minWidth: WoxViewSize
    opacity: float | None
    overflow: Literal["", "visible", "hidden"]
    padding: WoxViewEdges
    right: WoxViewSize | None
    rotate: float | None
    scale: float | None
    shadow: Literal["", "sm", "md", "lg"]
    textAlign: Literal["", "start", "center"]
    top: WoxViewSize | None
    translateX: WoxViewOffset
    translateY: WoxViewOffset
    width: WoxViewSize


class WoxViewTransitionRequired(TypedDict):
    properties: list[Literal["opacity", "backgroundColor", "width", "height", "translateX", "translateY", "scale"]]


class WoxViewTransition(WoxViewTransitionRequired, total=False):
    curve: Literal["", "linear", "easeOutBack", "easeInOut"]
    duration: int | None


class WoxViewBadgeProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewBadgePropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewBadgePropsShortcutsItem]
    style: WoxViewStyle
    text: str
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewButtonProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewButtonPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    icon: str
    iconColorMode: Literal["", "tint", "original"]
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewButtonPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    variant: Literal["", "secondary", "primary", "outline", "text", "surface"]


class WoxViewCheckboxProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewCheckboxPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewCheckboxPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: bool


class WoxViewCodeProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    code: str
    contextMenu: list[WoxViewCodePropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    language: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewCodePropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewColumnProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewColumnPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewColumnPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewDividerProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewDividerPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewDividerPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    vertical: bool


class WoxViewFormProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewFormPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onSubmit: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewFormPropsShortcutsItem]
    style: WoxViewStyle
    submitting: bool
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewGridPropsRequired(TypedDict):
    columns: int


class WoxViewGridProps(WoxViewGridPropsRequired, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewGridPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    gap: WoxViewSize
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewGridPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewHotkeyProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewHotkeyPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    placeholder: str
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewHotkeyPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: str


class WoxViewIconButtonPropsRequired(TypedDict):
    icon: str


class WoxViewIconButtonProps(WoxViewIconButtonPropsRequired, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewIconButtonPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    iconColorMode: Literal["", "tint", "original"]
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewIconButtonPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewImagePropsVariant1Required(TypedDict):
    icon: str


class WoxViewImagePropsVariant1(WoxViewImagePropsVariant1Required, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewImagePropsVariant1ContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    fit: Literal["", "contain", "cover", "fill"]
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    iconColorMode: Literal["", "tint", "original"]
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewImagePropsVariant1ShortcutsItem]
    source: None
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewImagePropsVariant2Required(TypedDict):
    source: WoxViewImagePropsVariant2SourceVariant1 | None


class WoxViewImagePropsVariant2(WoxViewImagePropsVariant2Required, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewImagePropsVariant2ContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    fit: Literal["", "contain", "cover", "fill"]
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    icon: Literal[""]
    iconColorMode: Literal[""]
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewImagePropsVariant2ShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewLazyListPropsRequired(TypedDict):
    itemHeight: WoxViewSize


class WoxViewLazyListProps(WoxViewLazyListPropsRequired, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewLazyListPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onEndReached: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewLazyListPropsShortcutsItem]
    stickToEnd: bool
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewLinkPropsRequired(TypedDict):
    url: str


class WoxViewLinkProps(WoxViewLinkPropsRequired, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewLinkPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewLinkPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewListProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewListPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    options: list[WoxViewListPropsOptionsItem]
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewListPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: str


class WoxViewLoadingProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewLoadingPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewLoadingPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewMarkdownProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    content: str
    contextMenu: list[WoxViewMarkdownPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onLinkPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewMarkdownPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewMorphPathPropsRequired(TypedDict):
    keyframes: list[WoxViewMorphPathPropsKeyframesItem]


class WoxViewMorphPathProps(WoxViewMorphPathPropsRequired, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewMorphPathPropsContextMenuItem]
    curve: Literal["", "linear", "easeOutBack", "easeInOut"]
    disabledStyle: WoxViewStateStyle | None
    duration: int | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    progress: float
    shortcuts: list[WoxViewMorphPathPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewMultiSelectProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewMultiSelectPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    options: list[WoxViewMultiSelectPropsOptionsItem]
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewMultiSelectPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: list[str]


class WoxViewBadgeNodeRequired(TypedDict):
    type: Literal["badge"]


class WoxViewBadgeNode(WoxViewBadgeNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewBadgeProps | None


class WoxViewButtonNodeRequired(TypedDict):
    type: Literal["button"]


class WoxViewButtonNode(WoxViewButtonNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewButtonProps | None


class WoxViewCheckboxNodeRequired(TypedDict):
    type: Literal["checkbox"]


class WoxViewCheckboxNode(WoxViewCheckboxNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewCheckboxProps | None


class WoxViewCodeNodeRequired(TypedDict):
    type: Literal["code"]


class WoxViewCodeNode(WoxViewCodeNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewCodeProps | None


class WoxViewColumnNodeRequired(TypedDict):
    type: Literal["column"]


class WoxViewColumnNode(WoxViewColumnNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str
    props: WoxViewColumnProps | None


class WoxViewDividerNodeRequired(TypedDict):
    type: Literal["divider"]


class WoxViewDividerNode(WoxViewDividerNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewDividerProps | None


class WoxViewFormNodeRequired(TypedDict):
    type: Literal["form"]


class WoxViewFormNode(WoxViewFormNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str
    props: WoxViewFormProps | None


class WoxViewGridNodeRequired(TypedDict):
    props: WoxViewGridProps
    type: Literal["grid"]


class WoxViewGridNode(WoxViewGridNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str


class WoxViewHotkeyNodeRequired(TypedDict):
    type: Literal["hotkey"]


class WoxViewHotkeyNode(WoxViewHotkeyNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewHotkeyProps | None


class WoxViewIconButtonNodeRequired(TypedDict):
    props: WoxViewIconButtonProps
    type: Literal["icon-button"]


class WoxViewIconButtonNode(WoxViewIconButtonNodeRequired, total=False):
    children: list[NoReturn]
    key: str


class WoxViewImageNodeRequired(TypedDict):
    props: WoxViewImageProps
    type: Literal["image"]


class WoxViewImageNode(WoxViewImageNodeRequired, total=False):
    children: list[NoReturn]
    key: str


class WoxViewLazyListNodeRequired(TypedDict):
    props: WoxViewLazyListProps
    type: Literal["lazy-list"]


class WoxViewLazyListNode(WoxViewLazyListNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str


class WoxViewLinkNodeRequired(TypedDict):
    props: WoxViewLinkProps
    type: Literal["link"]


class WoxViewLinkNode(WoxViewLinkNodeRequired, total=False):
    children: list[NoReturn]
    key: str


class WoxViewListNodeRequired(TypedDict):
    type: Literal["list"]


class WoxViewListNode(WoxViewListNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewListProps | None


class WoxViewLoadingNodeRequired(TypedDict):
    type: Literal["loading"]


class WoxViewLoadingNode(WoxViewLoadingNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewLoadingProps | None


class WoxViewMarkdownNodeRequired(TypedDict):
    type: Literal["markdown"]


class WoxViewMarkdownNode(WoxViewMarkdownNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewMarkdownProps | None


class WoxViewMorphPathNodeRequired(TypedDict):
    props: WoxViewMorphPathProps
    type: Literal["morph-path"]


class WoxViewMorphPathNode(WoxViewMorphPathNodeRequired, total=False):
    children: list[NoReturn]
    key: str


class WoxViewMultiSelectNodeRequired(TypedDict):
    type: Literal["multi-select"]


class WoxViewMultiSelectNode(WoxViewMultiSelectNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewMultiSelectProps | None


class WoxViewNumberFieldNodeRequired(TypedDict):
    type: Literal["number-field"]


class WoxViewNumberFieldNode(WoxViewNumberFieldNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewNumberFieldProps | None


class WoxViewProgressNodeRequired(TypedDict):
    type: Literal["progress"]


class WoxViewProgressNode(WoxViewProgressNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewProgressProps | None


class WoxViewRadioGroupNodeRequired(TypedDict):
    type: Literal["radio-group"]


class WoxViewRadioGroupNode(WoxViewRadioGroupNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewRadioGroupProps | None


class WoxViewRowNodeRequired(TypedDict):
    type: Literal["row"]


class WoxViewRowNode(WoxViewRowNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str
    props: WoxViewRowProps | None


class WoxViewScrollNodeRequired(TypedDict):
    type: Literal["scroll"]


class WoxViewScrollNode(WoxViewScrollNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str
    props: WoxViewScrollProps | None


class WoxViewSelectNodeRequired(TypedDict):
    type: Literal["select"]


class WoxViewSelectNode(WoxViewSelectNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewSelectProps | None


class WoxViewSliderNodeRequired(TypedDict):
    type: Literal["slider"]


class WoxViewSliderNode(WoxViewSliderNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewSliderProps | None


class WoxViewSpacerNodeRequired(TypedDict):
    type: Literal["spacer"]


class WoxViewSpacerNode(WoxViewSpacerNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewSpacerProps | None


class WoxViewStackNodeRequired(TypedDict):
    type: Literal["stack"]


class WoxViewStackNode(WoxViewStackNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str
    props: WoxViewStackProps | None


class WoxViewSubmitNodeRequired(TypedDict):
    type: Literal["submit"]


class WoxViewSubmitNode(WoxViewSubmitNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewSubmitProps | None


class WoxViewSwitchNodeRequired(TypedDict):
    type: Literal["switch"]


class WoxViewSwitchNode(WoxViewSwitchNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewSwitchProps | None


class WoxViewTableNodeRequired(TypedDict):
    props: WoxViewTableProps
    type: Literal["table"]


class WoxViewTableNode(WoxViewTableNodeRequired, total=False):
    children: list[NoReturn]
    key: str


class WoxViewTabsNodeRequired(TypedDict):
    type: Literal["tabs"]


class WoxViewTabsNode(WoxViewTabsNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewTabsProps | None


class WoxViewTagNodeRequired(TypedDict):
    type: Literal["tag"]


class WoxViewTagNode(WoxViewTagNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewTagProps | None


class WoxViewTagsNodeRequired(TypedDict):
    type: Literal["tags"]


class WoxViewTagsNode(WoxViewTagsNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewTagsProps | None


class WoxViewTextNodeRequired(TypedDict):
    type: Literal["text"]


class WoxViewTextNode(WoxViewTextNodeRequired, total=False):
    children: list[WoxViewSpanNode]
    key: str
    props: WoxViewTextProps | None


class WoxViewTextFieldNodeRequired(TypedDict):
    type: Literal["text-field"]


class WoxViewTextFieldNode(WoxViewTextFieldNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewTextFieldProps | None


class WoxViewViewNodeRequired(TypedDict):
    type: Literal["view"]


class WoxViewViewNode(WoxViewViewNodeRequired, total=False):
    children: list[WoxViewNode]
    key: str
    props: WoxViewViewProps | None


class WoxViewNumberFieldProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    autofocus: bool
    contextMenu: list[WoxViewNumberFieldPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    max: float | None
    min: float | None
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onSubmit: WoxViewHandler | None
    placeholder: str
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewNumberFieldPropsShortcutsItem]
    step: float
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: float


class WoxViewProgressProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewProgressPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    indeterminate: bool
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewProgressPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: float


class WoxViewRadioGroupProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewRadioGroupPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    options: list[WoxViewRadioGroupPropsOptionsItem]
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewRadioGroupPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: str


class WoxViewRowProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewRowPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewRowPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    wrap: bool


class WoxViewScrollProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewScrollPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    endThreshold: WoxViewSize
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    horizontal: bool
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onEndReached: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewScrollPropsShortcutsItem]
    stickToEnd: bool
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewSelectProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewSelectPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    options: list[WoxViewSelectPropsOptionsItem]
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewSelectPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: str


class WoxViewSliderProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewSliderPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    max: float
    min: float
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewSliderPropsShortcutsItem]
    step: float
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: float


class WoxViewSpacerProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewSpacerPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewSpacerPropsShortcutsItem]
    size: WoxViewSize
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewSpanNodeRequired(TypedDict):
    type: Literal["span"]


class WoxViewSpanNode(WoxViewSpanNodeRequired, total=False):
    children: list[NoReturn]
    key: str
    props: WoxViewSpanProps | None


class WoxViewSpanProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewSpanPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewSpanPropsShortcutsItem]
    style: WoxViewSpanPropsStyle
    text: str
    tooltip: str
    transition: None | WoxViewTransition


class WoxViewStackProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewStackPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewStackPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewSubmitProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewSubmitPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    icon: str
    iconColorMode: Literal["", "tint", "original"]
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewSubmitPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    variant: Literal["", "secondary", "primary", "outline", "text", "surface"]


class WoxViewSwitchProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewSwitchPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewSwitchPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: bool


class WoxViewTablePropsRequired(TypedDict):
    columns: list[WoxViewTablePropsColumnsItem]


class WoxViewTableProps(WoxViewTablePropsRequired, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewTablePropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    rows: list[WoxViewTablePropsRowsItem]
    shortcuts: list[WoxViewTablePropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewTabsProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewTabsPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    items: list[WoxViewTabsPropsItemsItem]
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewTabsPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: str


class WoxViewTagProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewTagPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewTagPropsShortcutsItem]
    style: WoxViewStyle
    text: str
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewTagsProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewTagsPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    options: list[WoxViewTagsPropsOptionsItem]
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewTagsPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: list[str]


class WoxViewTextFieldProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    autofocus: bool
    changeDebounce: int
    contextMenu: list[WoxViewTextFieldPropsContextMenuItem]
    disabled: bool
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    error: str
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    maxLines: int
    name: str
    onChange: WoxViewHandler | None
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onSubmit: WoxViewHandler | None
    password: bool
    placeholder: str
    pressedStyle: WoxViewStateStyle | None
    required: bool
    revision: int
    shortcuts: list[WoxViewTextFieldPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None
    value: str


class WoxViewTextProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewTextPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    selectable: bool
    shortcuts: list[WoxViewTextPropsShortcutsItem]
    style: WoxViewStyle
    text: str
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewViewProps(TypedDict, total=False):
    animation: WoxViewAnimation | None
    contextMenu: list[WoxViewViewPropsContextMenuItem]
    disabledStyle: WoxViewStateStyle | None
    enter: WoxViewPresenceAnimation | None
    exit: WoxViewPresenceAnimation | None
    focusStyle: WoxViewStateStyle | None
    hoverStyle: WoxViewStateStyle | None
    inputRegion: bool
    label: str
    onContextMenu: WoxViewHandler | None
    onDoubleTap: WoxViewHandler | None
    onFocusChange: WoxViewHandler | None
    onHoverChange: WoxViewHandler | None
    onPress: WoxViewHandler | None
    pressedStyle: WoxViewStateStyle | None
    shortcuts: list[WoxViewViewPropsShortcutsItem]
    style: WoxViewStyle
    tooltip: str
    transition: WoxViewTransition | None


class WoxViewTree(TypedDict):
    root: WoxViewNode
    version: Literal[1]


class WoxViewBadgePropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewBadgePropsContextMenuItem(WoxViewBadgePropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewBadgePropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewButtonPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewButtonPropsContextMenuItem(WoxViewButtonPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewButtonPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewCheckboxPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewCheckboxPropsContextMenuItem(WoxViewCheckboxPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewCheckboxPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewCodePropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewCodePropsContextMenuItem(WoxViewCodePropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewCodePropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewColumnPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewColumnPropsContextMenuItem(WoxViewColumnPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewColumnPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewDividerPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewDividerPropsContextMenuItem(WoxViewDividerPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewDividerPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewFormPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewFormPropsContextMenuItem(WoxViewFormPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewFormPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewGridPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewGridPropsContextMenuItem(WoxViewGridPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewGridPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewHotkeyPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewHotkeyPropsContextMenuItem(WoxViewHotkeyPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewHotkeyPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewIconButtonPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewIconButtonPropsContextMenuItem(WoxViewIconButtonPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewIconButtonPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewImagePropsVariant1ContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewImagePropsVariant1ContextMenuItem(WoxViewImagePropsVariant1ContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewImagePropsVariant1ShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewImagePropsVariant2ContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewImagePropsVariant2ContextMenuItem(WoxViewImagePropsVariant2ContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewImagePropsVariant2ShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewImagePropsVariant2SourceVariant1(TypedDict):
    imageData: str
    imageType: Literal["base64", "svg", "absolute"]


class WoxViewLazyListPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewLazyListPropsContextMenuItem(WoxViewLazyListPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewLazyListPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewLinkPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewLinkPropsContextMenuItem(WoxViewLinkPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewLinkPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewListPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewListPropsContextMenuItem(WoxViewListPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewListPropsOptionsItemRequired(TypedDict):
    value: str


class WoxViewListPropsOptionsItem(WoxViewListPropsOptionsItemRequired, total=False):
    disabled: bool
    label: str


class WoxViewListPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewLoadingPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewLoadingPropsContextMenuItem(WoxViewLoadingPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewLoadingPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewMarkdownPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewMarkdownPropsContextMenuItem(WoxViewMarkdownPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewMarkdownPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewMorphPathPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewMorphPathPropsContextMenuItem(WoxViewMorphPathPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewMorphPathPropsKeyframesItem(TypedDict):
    at: float
    path: list[WoxViewMorphPathPropsKeyframesItemPathItem]


class WoxViewMorphPathPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewMultiSelectPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewMultiSelectPropsContextMenuItem(WoxViewMultiSelectPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewMultiSelectPropsOptionsItemRequired(TypedDict):
    value: str


class WoxViewMultiSelectPropsOptionsItem(WoxViewMultiSelectPropsOptionsItemRequired, total=False):
    disabled: bool
    label: str


class WoxViewMultiSelectPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewNumberFieldPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewNumberFieldPropsContextMenuItem(WoxViewNumberFieldPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewNumberFieldPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewProgressPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewProgressPropsContextMenuItem(WoxViewProgressPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewProgressPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewRadioGroupPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewRadioGroupPropsContextMenuItem(WoxViewRadioGroupPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewRadioGroupPropsOptionsItemRequired(TypedDict):
    value: str


class WoxViewRadioGroupPropsOptionsItem(WoxViewRadioGroupPropsOptionsItemRequired, total=False):
    disabled: bool
    label: str


class WoxViewRadioGroupPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewRowPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewRowPropsContextMenuItem(WoxViewRowPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewRowPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewScrollPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewScrollPropsContextMenuItem(WoxViewScrollPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewScrollPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewSelectPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewSelectPropsContextMenuItem(WoxViewSelectPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewSelectPropsOptionsItemRequired(TypedDict):
    value: str


class WoxViewSelectPropsOptionsItem(WoxViewSelectPropsOptionsItemRequired, total=False):
    disabled: bool
    label: str


class WoxViewSelectPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewSliderPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewSliderPropsContextMenuItem(WoxViewSliderPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewSliderPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewSpacerPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewSpacerPropsContextMenuItem(WoxViewSpacerPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewSpacerPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewSpanPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewSpanPropsContextMenuItem(WoxViewSpanPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewSpanPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewSpanPropsStyle(TypedDict, total=False):
    color: WoxViewColor
    fontFamily: Literal["", "ui", "monospace"]
    fontWeight: Literal["", "regular", "semibold", "bold"]
    italic: bool | None
    underline: bool | None


class WoxViewStackPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewStackPropsContextMenuItem(WoxViewStackPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewStackPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewSubmitPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewSubmitPropsContextMenuItem(WoxViewSubmitPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewSubmitPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewSwitchPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewSwitchPropsContextMenuItem(WoxViewSwitchPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewSwitchPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewTablePropsColumnsItemRequired(TypedDict):
    key: str


class WoxViewTablePropsColumnsItem(WoxViewTablePropsColumnsItemRequired, total=False):
    label: str
    width: WoxViewSize


class WoxViewTablePropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewTablePropsContextMenuItem(WoxViewTablePropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewTablePropsRowsItemRequired(TypedDict):
    cells: list[str]
    key: str


class WoxViewTablePropsRowsItem(WoxViewTablePropsRowsItemRequired, total=False):
    actions: list[WoxViewTablePropsRowsItemActionsItem]


class WoxViewTablePropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewTabsPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewTabsPropsContextMenuItem(WoxViewTabsPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewTabsPropsItemsItemRequired(TypedDict):
    value: str


class WoxViewTabsPropsItemsItem(WoxViewTabsPropsItemsItemRequired, total=False):
    disabled: bool
    label: str


class WoxViewTabsPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewTagPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewTagPropsContextMenuItem(WoxViewTagPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewTagPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewTagsPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewTagsPropsContextMenuItem(WoxViewTagsPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewTagsPropsOptionsItemRequired(TypedDict):
    value: str


class WoxViewTagsPropsOptionsItem(WoxViewTagsPropsOptionsItemRequired, total=False):
    disabled: bool
    label: str


class WoxViewTagsPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewTextFieldPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewTextFieldPropsContextMenuItem(WoxViewTextFieldPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewTextFieldPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewTextPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewTextPropsContextMenuItem(WoxViewTextPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewTextPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewViewPropsContextMenuItemRequired(TypedDict):
    label: str
    onPress: WoxViewHandler


class WoxViewViewPropsContextMenuItem(WoxViewViewPropsContextMenuItemRequired, total=False):
    disabled: bool
    icon: str


class WoxViewViewPropsShortcutsItem(TypedDict):
    key: str
    onPress: WoxViewHandler


class WoxViewMorphPathPropsKeyframesItemPathItemRequired(TypedDict):
    op: Literal["M", "L", "Q", "C", "Z"]


class WoxViewMorphPathPropsKeyframesItemPathItem(WoxViewMorphPathPropsKeyframesItemPathItemRequired, total=False):
    values: list[float]


class WoxViewTablePropsRowsItemActionsItemRequired(TypedDict):
    key: str
    label: str
    onPress: WoxViewHandler


class WoxViewTablePropsRowsItemActionsItem(WoxViewTablePropsRowsItemActionsItemRequired, total=False):
    disabled: bool
    icon: str


WoxViewColor: TypeAlias = str | None | WoxViewColorVariant3
WoxViewOffset: TypeAlias = (
    float
    | Literal["$radiusSm", "$radiusMd", "$space1", "$space2", "$space3", "$space4", "$fontSm", "$fontMd", "$fontLg", "$fontTitle"]
    | None
)
WoxViewPresenceAnimation: TypeAlias = Literal["fade", "slideUp", "scale"] | WoxViewPresenceAnimationVariant2
WoxViewSize: TypeAlias = (
    float
    | Literal["$radiusSm", "$radiusMd", "$space1", "$space2", "$space3", "$space4", "$fontSm", "$fontMd", "$fontLg", "$fontTitle"]
    | None
)
WoxViewImageProps: TypeAlias = WoxViewImagePropsVariant1 | WoxViewImagePropsVariant2
WoxViewNode: TypeAlias = (
    WoxViewBadgeNode
    | WoxViewButtonNode
    | WoxViewCheckboxNode
    | WoxViewCodeNode
    | WoxViewColumnNode
    | WoxViewDividerNode
    | WoxViewFormNode
    | WoxViewGridNode
    | WoxViewHotkeyNode
    | WoxViewIconButtonNode
    | WoxViewImageNode
    | WoxViewLazyListNode
    | WoxViewLinkNode
    | WoxViewListNode
    | WoxViewLoadingNode
    | WoxViewMarkdownNode
    | WoxViewMorphPathNode
    | WoxViewMultiSelectNode
    | WoxViewNumberFieldNode
    | WoxViewProgressNode
    | WoxViewRadioGroupNode
    | WoxViewRowNode
    | WoxViewScrollNode
    | WoxViewSelectNode
    | WoxViewSliderNode
    | WoxViewSpacerNode
    | WoxViewStackNode
    | WoxViewSubmitNode
    | WoxViewSwitchNode
    | WoxViewTableNode
    | WoxViewTabsNode
    | WoxViewTagNode
    | WoxViewTagsNode
    | WoxViewTextNode
    | WoxViewTextFieldNode
    | WoxViewViewNode
)
WoxViewEdges: TypeAlias = WoxViewSize | WoxViewEdgesVariant2
