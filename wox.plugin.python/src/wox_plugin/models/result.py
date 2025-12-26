"""
Wox Result Models

This module provides models for search results displayed in Wox.

Results are the primary way plugins present information to users. Each result
can have a title, subtitle, icon, preview, actions, and additional visual
elements called tails.
"""

import json
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Awaitable, Callable, Dict, List, Optional

from .context import Context
from .image import WoxImage
from .preview import WoxPreview
from .setting import PluginSettingDefinitionItem


class ResultTailType(str, Enum):
    """
    Enumeration of result tail types.

    Tails are additional visual elements displayed with a result,
    providing extra information or quick access to features.
    """

    TEXT = "text"
    """
    String type tail.

    Displays text content next to the result. Common uses include:
    - Status indicators ("Enabled", "Disabled")
    - Metadata ("Size: 1.2 MB")
    - Quick info ("Last used: 2 hours ago")
    """

    IMAGE = "image"
    """
    WoxImage type tail.

    Displays an icon or small image next to the result.
    Common uses include:
    - Status icons (checkmarks, warning signs)
    - Platform indicators (Windows, Linux, macOS icons)
    - Visual badges (favorite star, notification bell)
    """


class ResultActionType(str, Enum):
    """
    Enumeration of result action types.

    Actions are operations that can be performed on a result,
    triggered by clicking, hotkeys, or other user interactions.
    """

    EXECUTE = "execute"
    """
    Execute action immediately.

    The action callback runs immediately when triggered without
    showing any UI. Use this for simple operations like:
    - Copying text to clipboard
    - Opening a file/URL
    - Running a command

    Example:
        ResultAction(
            name="Copy",
            action=copy_to_clipboard,
            type=ResultActionType.EXECUTE
        )
    """

    FORM = "form"
    """
    Show a form before executing.

    When triggered, displays a form with input fields defined
    in the `form` property. After the user fills and submits
    the form, the `on_submit` callback is called with the values.

    Use this for operations that require user input:
    - Renaming a file
    - Adding a new item with custom properties
    - Configuring options before execution

    Example:
        ResultAction(
            name="Rename",
            type=ResultActionType.FORM,
            form=[
                create_textbox_setting("new_name", "New Name", "old_name.txt")
            ],
            on_submit=handle_rename
        )
    """


@dataclass
class ResultTail:
    """
    Tail model for Wox results.

    Tails are additional visual elements displayed next to a result item,
    providing extra information or quick access to features. They appear
    in the result detail view and can display text or small images.

    Attributes:
        type: The type of tail (TEXT or IMAGE)
        text: Text content (for TEXT type)
        image: Image to display (for IMAGE type)
        id: Unique identifier for this tail
        context_data: Additional data for later retrieval

    Example usage:
        # Text tail
        tail = ResultTail(
            type=ResultTailType.TEXT,
            text="Size: 1.2 MB",
            id="size_info"
        )

        # Image tail
        tail = ResultTail(
            type=ResultTailType.IMAGE,
            image=WoxImage.new_emoji("⭐"),
            id="favorite_indicator"
        )

        # Add to result
        result = Result(
            title="Document.txt",
            icon=WoxImage.new_emoji("📄"),
            tails=[tail]
        )
    """

    type: ResultTailType = field(default=ResultTailType.TEXT)
    """
    The type of tail content.

    Determines whether the tail displays text (TEXT) or an image (IMAGE).
    """

    text: str = field(default="")
    """
    Text content for TEXT type tails.

    Displayed as plain text next to the result.
    Only used when type is TEXT.
    """

    image: WoxImage = field(default_factory=WoxImage)
    """
    Image for IMAGE type tails.

    A small icon or image displayed next to the result.
    Only used when type is IMAGE.
    """

    id: str = field(default="")
    """
    Unique identifier for this tail.

    Should be unique within the result's tails list.
    If not set, Wox will assign a random ID.

    Use this ID to:
    - Identify the tail in action callbacks
    - Update/remove specific tails dynamically
    - Track tail state across updates
    """

    context_data: Dict[str, str] = field(default_factory=dict)
    """
    Additional data associated with this tail.

    Store arbitrary key-value pairs for tail identification and
    metadata. Note: This data is NOT passed to action callbacks.
    Use action.context_data for data that needs to be available
    in action handlers.

    Tail context_data is primarily used for:
    - Identifying specific tails (e.g., custom IDs, tags)
    - Storing tail metadata for UI updates
    - System-level tail identification (e.g., "system:favorite")

    Example:
        context_data={"tail_type": "status", "source": "api"}

    Note: If you need to pass data to action callbacks, use
    ResultAction.context_data instead.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        return json.dumps(
            {
                "Type": self.type,
                "Text": self.text,
                "Image": json.loads(self.image.to_json()),
                "Id": self.id,
                "ContextData": self.context_data,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "ResultTail":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing tail data

        Returns:
            A new ResultTail instance
        """
        data = json.loads(json_str)
        if not data.get("Type"):
            data["Type"] = ResultTailType.TEXT
        if not data.get("Image"):
            data["Image"] = {}

        return cls(
            type=ResultTailType(data.get("Type")),
            text=data.get("Text", ""),
            image=WoxImage.from_json(json.dumps(data["Image"])),
            id=data.get("Id", ""),
            context_data=data.get("ContextData", {}) or {},
        )


@dataclass
class ActionContext:
    """
    Context passed to result action callbacks.

    Contains information about which result and action triggered
    the callback, along with any custom context data.

    Attributes:
        result_id: ID of the result that triggered this action
        result_action_id: ID of the action that was triggered
        context_data: Additional data from the action or result

    Example usage:
        async def my_action(ctx: Context, action_ctx: ActionContext):
            result_id = action_ctx.result_id
            action_id = action_ctx.result_action_id
            custom_data = action_ctx.context_data.get("key")

            # Use the result_id to update the result
            await api.update_result(ctx, UpdatableResult(
                id=result_id,
                title="Action completed!"
            ))
    """

    result_id: str = field(default="")
    """
    ID of the result that triggered this action.

    This ID corresponds to the Result.id field. Use it with
    api.update_result() or api.get_updatable_result() to
    modify the result that triggered the action.
    """

    result_action_id: str = field(default="")
    """
    ID of the action that was triggered.

    This ID corresponds to the ResultAction.id field.
    Useful when you have multiple actions on a result and
    need to identify which one was triggered.
    """

    context_data: Dict[str, str] = field(default_factory=dict)
    """
    Additional data associated with this action.

    Contains the context_data from both the action and any
    associated tail (if the action was triggered from a tail).
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        return json.dumps(
            {
                "ResultId": self.result_id,
                "ResultActionId": self.result_action_id,
                "ContextData": self.context_data,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "ActionContext":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing action context data

        Returns:
            A new ActionContext instance
        """
        data = json.loads(json_str)
        context_data = data.get("ContextData", {}) or {}
        if isinstance(context_data, str):
            try:
                context_data = json.loads(context_data)
            except Exception:
                context_data = {}

        return cls(
            result_id=data.get("ResultId", ""),
            result_action_id=data.get("ResultActionId", ""),
            context_data=context_data if isinstance(context_data, dict) else {},
        )


@dataclass
class FormActionContext(ActionContext):
    """
    Context for form action submissions.

    Extends ActionContext with the form values submitted by the user.
    Used with ResultAction that have type=FORM.

    Attributes:
        result_id: ID of the result (inherited)
        result_action_id: ID of the action (inherited)
        context_data: Additional data (inherited)
        values: Form field values submitted by the user

    Example usage:
        async def handle_rename(ctx: Context, form_ctx: FormActionContext):
            new_name = form_ctx.values.get("new_name")
            if new_name:
                rename_file(form_ctx.result_id, new_name)
    """

    values: Dict[str, str] = field(default_factory=dict)
    """
    Form field values submitted by the user.

    Keys are the setting keys from the form definition,
    values are the user's input.

    Example:
        # Form defined with key "new_name"
        values = {"new_name": "document_v2.txt"}
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        return json.dumps(
            {
                "ResultId": self.result_id,
                "ResultActionId": self.result_action_id,
                "ContextData": self.context_data,
                "Values": self.values,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "FormActionContext":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing form action context data

        Returns:
            A new FormActionContext instance
        """
        data = json.loads(json_str)
        context_data = data.get("ContextData", {}) or {}
        if isinstance(context_data, str):
            try:
                context_data = json.loads(context_data)
            except Exception:
                context_data = {}
        return cls(
            result_id=data.get("ResultId", ""),
            result_action_id=data.get("ResultActionId", ""),
            context_data=context_data if isinstance(context_data, dict) else {},
            values=data.get("Values", {}) or {},
        )


@dataclass
class ResultAction:
    """
    Action model for Wox results.

    Actions are operations that can be performed on a result.
    They appear as clickable items in the result detail view.

    Attributes:
        name: Display name of the action
        action: Callback function for EXECUTE type actions
        id: Unique identifier for this action
        type: Action type (EXECUTE or FORM)
        form: Form field definitions for FORM type
        on_submit: Callback function for FORM type submissions
        icon: Icon to display for the action
        is_default: Whether this is the default action
        prevent_hide_after_action: Keep Wox visible after action
        hotkey: Keyboard shortcut to trigger the action
        context_data: Additional data for later retrieval

    Example usage:
        # Simple execute action
        ResultAction(
            name="Copy to Clipboard",
            icon=WoxImage.new_emoji("📋"),
            action=lambda ctx, ac: copy_text(ac.context_data["text"])
        )

        # Form action
        ResultAction(
            name="Rename",
            type=ResultActionType.FORM,
            form=[create_textbox_setting("new_name", "New Name")],
            on_submit=handle_rename
        )
    """

    name: str
    """
    Display name of the action.

    This is the text shown to the user in the action list.
    Can be internationalized using translation keys.
    """

    action: Optional[Callable[[Context, ActionContext], Awaitable[None]]] = None
    """
    Callback function for EXECUTE type actions.

    Called when the action is triggered. Receives the plugin context
    and action context containing the result_id and other data.

    Example:
        async def my_callback(ctx: Context, action_ctx: ActionContext):
            # Handle the action
            pass
    """

    id: str = field(default="")
    """
    Unique identifier for this action.

    Should be unique within the result's actions list.
    If not set, Wox will assign a random ID.
    """

    type: ResultActionType = field(default=ResultActionType.EXECUTE)
    """
    Action type determining how it's executed.

    EXECUTE: Runs immediately without UI
    FORM: Shows a form before executing
    """

    form: List[PluginSettingDefinitionItem] = field(default_factory=list)
    """
    Form field definitions for FORM type actions.

    Defines the input fields shown to the user.
    Only used when type is FORM.

    Example:
        form=[
            create_textbox_setting("name", "Name"),
            create_checkbox_setting("confirm", "Confirm")
        ]
    """

    on_submit: Optional[Callable[[Context, FormActionContext], Awaitable[None]]] = None
    """
    Callback function for FORM type submissions.

    Called after the user submits the form. Receives the form values
    in the FormActionContext.values field.

    Only used when type is FORM.
    """

    icon: WoxImage = field(default_factory=WoxImage)
    """
    Icon to display for the action.

    Shown next to the action name in the UI.
    """

    is_default: bool = field(default=False)
    """
    Whether this is the default action.

    The default action is executed when the user presses Enter
    on the result (or clicks the result directly).

    If no action is marked as default, the first action is used.
    Only one action per result should be default.
    """

    prevent_hide_after_action: bool = field(default=False)
    """
    Keep Wox visible after executing this action.

    When True, Wox remains open after the action completes.
    When False (default), Wox hides after the action.

    Use this for actions that:
    - Update the result state
    - Require user confirmation
    - Are part of a multi-step process

    Example:
        # After marking as favorite, keep Wox open to see the update
        ResultAction(
            name="Toggle Favorite",
            prevent_hide_after_action=True,
            action=toggle_favorite
        )
    """

    hotkey: str = field(default="")
    """
    Keyboard shortcut to trigger this action.

    Format: Modifier keys separated by '+' or space, case-insensitive.
    Examples: "Ctrl+K", "Command+Shift+Space", "ctrl+1"

    Wox normalizes hotkeys for each platform (e.g., "Ctrl" becomes
    "Command" on macOS automatically).

    If is_default is True, the hotkey is automatically set to Enter.
    """

    context_data: Dict[str, str] = field(default_factory=dict)
    """
    Additional data associated with this action.

    Stored key-value pairs that are passed to the action callback
    in the ActionContext. Useful for storing operation-specific data.

    Example:
        context_data={"file_id": "123", "operation": "delete"}
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Note: Callback functions (action, on_submit) are excluded
        from JSON serialization.

        Returns:
            JSON string representation
        """
        data: Dict[str, Any] = {
            "Name": self.name,
            "Id": self.id,
            "Type": self.type,
            "IsDefault": self.is_default,
            "PreventHideAfterAction": self.prevent_hide_after_action,
            "Hotkey": self.hotkey,
            "Icon": json.loads(self.icon.to_json()),
            "ContextData": self.context_data,
        }

        if self.type == ResultActionType.FORM:
            data["Form"] = [item.to_dict() for item in self.form]

        return json.dumps(data)

    @classmethod
    def from_json(cls, json_str: str) -> "ResultAction":
        """
        Create from JSON string with camelCase naming.

        Note: Callback functions cannot be restored from JSON and
        will be None. You need to set them manually after deserialization.

        Args:
            json_str: JSON string containing action data

        Returns:
            A new ResultAction instance
        """
        data = json.loads(json_str)

        action_type = ResultActionType(data.get("Type", ResultActionType.EXECUTE))
        form: List[PluginSettingDefinitionItem] = []
        if action_type == ResultActionType.FORM and data.get("Form"):
            form = [PluginSettingDefinitionItem.from_dict(item) for item in data.get("Form", [])]

        context_data = data.get("ContextData", {}) or {}
        if isinstance(context_data, str):
            try:
                context_data = json.loads(context_data)
            except Exception:
                context_data = {}

        return cls(
            name=data.get("Name", ""),
            id=data.get("Id", ""),
            type=action_type,
            form=form,
            icon=WoxImage.from_json(json.dumps(data.get("Icon", {}))),
            is_default=data.get("IsDefault", False),
            prevent_hide_after_action=data.get("PreventHideAfterAction", False),
            hotkey=data.get("Hotkey", ""),
            context_data=context_data if isinstance(context_data, dict) else {},
        )


@dataclass
class Result:
    """
    Result model for Wox search results.

    Results are the primary way plugins present information to users.
    Each result represents a single item that can be selected,
    previewed, and acted upon.

    Attributes:
        title: Primary display text (required)
        icon: Icon to display (required)
        id: Unique identifier for this result
        sub_title: Secondary display text
        preview: Preview content for detail view
        score: Relevance score for sorting
        group: Group name for categorization
        group_score: Group relevance score
        tails: Additional visual elements
        actions: Operations that can be performed

    Example usage:
        result = Result(
            title="Document.txt",
            sub_title="/home/user/documents/Document.txt",
            icon=WoxImage.new_emoji("📄"),
            preview=WoxPreview(
                preview_type=WoxPreviewType.TEXT,
                preview_data="This is a text document..."
            ),
            score=100,
            group="Documents",
            actions=[
                ResultAction(
                    name="Open",
                    icon=WoxImage.new_emoji("📂"),
                    is_default=True
                ),
                ResultAction(
                    name="Delete",
                    icon=WoxImage.new_emoji("🗑️")
                )
            ]
        )
    """

    title: str
    """
    Primary display text for the result.

    This is the main text shown in the results list.
    Supports i18n translation keys.

    Required field.
    """

    icon: WoxImage
    """
    Icon to display for the result.

    Shown next to the title in the results list.
    Can be any WoxImage type (emoji, file path, URL, etc.).

    Required field.
    """

    id: str = field(default="")
    """
    Unique identifier for this result.

    If not set, Wox will assign a random ID.
    Use this ID to:
    - Track the result across updates
    - Update the result dynamically
    - Identify which result triggered an action

    Example: Use file path, database ID, or hash as the ID.
    """

    sub_title: str = field(default="")
    """
    Secondary display text for the result.

    Shown below the title in the results list.
    Typically used for additional context like file path,
    date, size, or description.

    Supports i18n translation keys.
    """

    preview: WoxPreview = field(default_factory=WoxPreview)
    """
    Preview content for the result.

    Displayed in the preview panel when the result is selected.
    Can show text, markdown, images, files, or web content.
    """

    score: float = field(default=0.0)
    """
    Relevance score for sorting results.

    Higher scores appear higher in the results list.
    Use this to rank results by relevance.

    Typical values:
    - 100+: Perfect match
    - 50-99: Good match
    - 1-49: Partial match
    - 0: Default/neutral
    """

    group: str = field(default="")
    """
    Group name for categorizing results.

    Wox groups results by this name in the UI.
    Results without a group are shown in the default group.

    Example groups: "Files", "Applications", "Web Results"
    """

    group_score: float = field(default=0.0)
    """
    Group relevance score for sorting groups.

    Higher scores cause the entire group to appear higher
    in the results. Use this when the group itself has
    relevance to the query.
    """

    tails: List[ResultTail] = field(default_factory=list)
    """
    Additional visual elements for the result.

    Tails display extra information or quick actions next to
    the result in the detail view. Can show text or icons.
    """

    actions: List[ResultAction] = field(default_factory=list)
    """
    Operations that can be performed on the result.

    Actions appear in the result detail view and can be
    triggered by clicking or hotkeys.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Note: Action callbacks are excluded from serialization.

        Returns:
            JSON string representation
        """
        data = {
            "Title": self.title,
            "Icon": json.loads(self.icon.to_json()),
            "Id": self.id,
            "SubTitle": self.sub_title,
            "Score": self.score,
            "Group": self.group,
            "GroupScore": self.group_score,
        }
        if self.preview:
            data["Preview"] = json.loads(self.preview.to_json())
        if self.tails:
            data["Tails"] = [json.loads(tail.to_json()) for tail in self.tails]
        if self.actions:
            data["Actions"] = [json.loads(action.to_json()) for action in self.actions]
        return json.dumps(data)

    @classmethod
    def from_json(cls, json_str: str) -> "Result":
        """
        Create from JSON string with camelCase naming.

        Note: Action callbacks will be None and must be set manually.

        Args:
            json_str: JSON string containing result data

        Returns:
            A new Result instance
        """
        data = json.loads(json_str)
        preview = WoxPreview.from_json(json.dumps(data["Preview"]))

        tails = []
        if "Tails" in data:
            tails = [ResultTail.from_json(json.dumps(tail)) for tail in data["Tails"]]

        actions = []
        if "Actions" in data:
            actions = [ResultAction.from_json(json.dumps(action)) for action in data["Actions"]]

        return cls(
            title=data.get("Title", ""),
            icon=WoxImage.from_json(json.dumps(data.get("Icon", {}))),
            id=data.get("Id", ""),
            sub_title=data.get("SubTitle", ""),
            preview=preview,
            score=data.get("Score", 0.0),
            group=data.get("Group", ""),
            group_score=data.get("GroupScore", 0.0),
            tails=tails,
            actions=actions,
        )


@dataclass
class UpdatableResult:
    """
    Result that can be updated directly in the UI.

    Used with api.update_result() to modify a currently displayed result
    without re-running the entire query. This is useful for showing
    progress updates, state changes, or dynamic content.

    All fields except id are optional. Only non-None fields will be updated.

    Example usage:
        # Update only the title
        success = await api.update_result(ctx, UpdatableResult(
            id=result_id,
            title="Downloading... 50%"
        ))

        # Update title and tails
        success = await api.update_result(ctx, UpdatableResult(
            id=result_id,
            title="Processing...",
            tails=[ResultTail(type=ResultTailType.TEXT, text="Step 1/3")]
        ))

        # Update preview
        success = await api.update_result(ctx, UpdatableResult(
            id=result_id,
            preview=WoxPreview(
                preview_type=WoxPreviewType.TEXT,
                preview_data="Updated content"
            )
        ))

    Note: When updating actions, the entire actions list is replaced.
    Include all actions you want to keep, not just the new ones.
    """

    id: str
    """
    ID of the result to update.

    Must match the Result.id of the currently displayed result.
    """

    title: Optional[str] = None
    """
    New title for the result.

    If None, the current title is kept.
    """

    sub_title: Optional[str] = None
    """
    New subtitle for the result.

    If None, the current subtitle is kept.
    """

    tails: Optional[List[ResultTail]] = None
    """
    New list of tails for the result.

    If None, the current tails are kept.
    If set (even to empty list), replaces all current tails.
    """

    preview: Optional[WoxPreview] = None
    """
    New preview for the result.

    If None, the current preview is kept.
    """

    actions: Optional[List[ResultAction]] = None
    """
    New list of actions for the result.

    If None, the current actions are kept.
    If set, replaces all current actions.

    Note: Actions in the list cannot have callbacks attached
    when updating. The callbacks from the original result are
    preserved by ID matching.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Only includes fields that are not None.

        Returns:
            JSON string representation
        """
        data: Dict[str, Any] = {"Id": self.id}

        if self.title is not None:
            data["Title"] = self.title
        if self.sub_title is not None:
            data["SubTitle"] = self.sub_title
        if self.tails is not None:
            data["Tails"] = [json.loads(tail.to_json()) for tail in self.tails]
        if self.preview is not None:
            data["Preview"] = json.loads(self.preview.to_json())
        if self.actions is not None:
            data["Actions"] = [json.loads(action.to_json()) for action in self.actions]

        return json.dumps(data)
