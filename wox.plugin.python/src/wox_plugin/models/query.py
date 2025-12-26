"""
Wox Query Models

This module provides models for handling user queries in Wox plugins.

Queries represent user input and the context in which they are made,
including the query text, selection (text/files), environment information,
and trigger keywords.
"""

import json
from dataclasses import dataclass, field
from enum import Enum
from typing import List, Optional


class SelectionType(str, Enum):
    """
    Enumeration of selection types for user-selected content.

    When a user makes a selection in another application and invokes Wox,
    the selection can be either text or files.
    """

    TEXT = "text"
    """
    Text selection.

    Represents selected text content from another application.
    The actual text is available in the Selection.text field.
    """

    FILE = "file"
    """
    File selection.

    Represents one or more selected file paths from the file system.
    The file paths are available in the Selection.file_paths field.
    """


class QueryType(str, Enum):
    """
    Enumeration of query types in Wox.

    Defines how the query was initiated and what content it contains.
    """

    INPUT = "input"
    """
    Input query triggered by typing in the Wox search box.

    This is the most common query type, triggered when the user
    activates Wox and types text or uses a trigger keyword.
    """

    SELECTION = "selection"
    """
    Selection query triggered by selecting content and invoking Wox.

    This query type occurs when the user selects text or files in
    another application and then invokes Wox (e.g., via hotkey).
    The selection data is available in Query.selection.
    """


@dataclass
class MetadataCommand:
    """
    Command metadata for registering plugin commands.

    Used with `api.register_query_commands()` to register commands
    that can be invoked by typing specific keywords or patterns.

    Attributes:
        command: The command keyword/trigger
        description: Human-readable description of what the command does

    Example usage:
        # Register commands in init()
        commands = [
            MetadataCommand(
                command="todo",
                description="Manage your todo list"
            ),
            MetadataCommand(
                command="note",
                description="Quick note taking"
            ),
        ]
        await api.register_query_commands(ctx, commands)
    """

    command: str
    """
    The command keyword/trigger string.

    Users can invoke this command by typing this keyword in Wox.
    Example: "todo", "note", "calc"
    """

    description: str
    """
    Human-readable description of the command.

    This is shown to users to help them understand what the
    command does. Should be concise and informative.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        return json.dumps(
            {
                "Command": self.command,
                "Description": self.description,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "MetadataCommand":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string with "Command" and "Description"

        Returns:
            A new MetadataCommand instance
        """
        data = json.loads(json_str)
        return cls(
            command=data.get("Command", ""),
            description=data.get("Description", ""),
        )


@dataclass
class Selection:
    """
    User-selected content from another application.

    Represents text or files that the user selected before invoking Wox.
    This allows plugins to operate on content from other applications.

    Attributes:
        type: The type of selection (TEXT or FILE)
        text: The selected text content (when type is TEXT)
        file_paths: List of selected file paths (when type is FILE)

    Example usage:
        async def query(ctx: Context, query: Query) -> List[Result]:
            if query.type == QueryType.SELECTION:
                if query.selection.type == SelectionType.TEXT:
                    # Process selected text
                    text = query.selection.text
                    return [process_text(text)]
                elif query.selection.type == SelectionType.FILE:
                    # Process selected files
                    files = query.selection.file_paths
                    return [process_files(files)]
    """

    type: SelectionType = field(default=SelectionType.TEXT)
    """
    The type of selection.

    Either TEXT (selected text content) or FILE (selected file paths).
    """

    text: str = field(default="")
    """
    The selected text content.

    Contains the actual text that the user selected in another application.
    Only populated when type is TEXT.
    """

    file_paths: List[str] = field(default_factory=list)
    """
    List of selected file paths.

    Contains the full paths to files that the user selected.
    Only populated when type is FILE.
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
                "FilePaths": self.file_paths,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "Selection":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing selection data

        Returns:
            A new Selection instance
        """
        data = json.loads(json_str)

        if not data.get("Type"):
            data["Type"] = SelectionType.TEXT

        return cls(
            type=SelectionType(data.get("Type")),
            text=data.get("Text", ""),
            file_paths=data.get("FilePaths", []),
        )

    def __str__(self) -> str:
        """
        Convert selection to string representation.

        Returns:
            Text content for TEXT type, comma-joined paths for FILE type,
            or empty string if no data
        """
        if self.type == SelectionType.TEXT and self.text:
            return self.text
        elif self.type == SelectionType.FILE and self.file_paths:
            return ",".join(self.file_paths)
        return ""


@dataclass
class QueryEnv:
    """
    Environment information at the time of the query.

    Provides context about the user's current environment when the
    query was made, including the active window and browser URL.

    Attributes:
        active_window_title: Title of the currently active window
        active_window_pid: Process ID of the active window (0 if unavailable)
        active_window_icon: Icon of the active window (as WoxImage dict)
        active_browser_url: URL from the active browser tab

    Example usage:
        # Get context about the current application
        window_title = query.env.active_window_title
        if "Chrome" in window_title:
            # User is browsing, show browser-related results
    """

    active_window_title: str = field(default="")
    """
    Title of the active window when the query was made.

    Contains the window title of the application that was active
    when the user invoked Wox. Can be used to provide context-aware results.

    Examples: "Visual Studio Code", "Google Chrome", "Untitled - Notepad"
    """

    active_window_pid: int = field(default=0)
    """
    Process ID of the active window.

    The PID of the application that was active when the query was made.
    Can be used to interact with the active application programmatically.

    Note: May be 0 if the PID cannot be determined.
    """

    active_window_icon: dict = field(default_factory=dict)
    """
    Icon of the active window as a WoxImage dictionary.

    Contains the icon of the active application window. The format
    is a WoxImage serialized to dictionary format.

    Note: May be empty if no icon is available.
    """

    active_browser_url: str = field(default="")
    """
    URL from the active browser tab.

    Contains the URL of the currently open tab if:
    1. The active window is a supported browser (Chrome, Edge, etc.)
    2. The Wox Chrome Extension is installed

    The extension is available at:
    https://github.com/Wox-launcher/Wox.Chrome.Extension

    Note: Only available when both conditions above are met.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        return json.dumps(
            {
                "ActiveWindowTitle": self.active_window_title,
                "ActiveWindowPid": self.active_window_pid,
                "ActiveWindowIcon": self.active_window_icon,
                "ActiveBrowserUrl": self.active_browser_url,
            }
        )

    @classmethod
    def from_json(cls, json_str: str) -> "QueryEnv":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing environment data

        Returns:
            A new QueryEnv instance
        """
        data = json.loads(json_str)
        return cls(
            active_window_title=data.get("ActiveWindowTitle", ""),
            active_window_pid=data.get("ActiveWindowPid", 0),
            active_window_icon=data.get("ActiveWindowIcon", {}),
            active_browser_url=data.get("ActiveBrowserUrl", ""),
        )


@dataclass
class Query:
    """
    A user query in Wox.

    Represents a complete query from the user, including the query text,
    type, selection, environment context, and parsed components like
    trigger keyword and search terms.

    Attributes:
        id: Unique identifier for this query instance
        type: The type of query (INPUT or SELECTION)
        raw_query: The full raw query text as typed by the user
        selection: Selected content (for SELECTION type queries)
        env: Environment context information
        trigger_keyword: The trigger keyword if present (empty for global queries)
        command: The command keyword after trigger keyword
        search: The search text after command (empty if only command)

    Example usage:
        async def query(ctx: Context, query: Query) -> List[Result]:
            # For a query like ">todo add buy groceries"
            # trigger_keyword = "todo"
            # command = "add"
            # search = "buy groceries"

            if query.trigger_keyword == "todo":
                return handle_todo_command(query.command, query.search)
            elif query.is_global_query():
                return handle_global_search(query.search)
    """

    id: str
    """
    Unique identifier for this query instance.

    Can be used to track and correlate requests, especially useful
    for logging and debugging.
    """

    type: QueryType
    """
    The type of query.

    INPUT: User typed in the Wox search box
    SELECTION: User selected content and invoked Wox
    """

    raw_query: str
    """
    The full raw query text as typed by the user.

    Contains the complete query string including trigger keyword,
    command, and search text. For example: ">todo add buy groceries"
    """

    selection: Selection
    """
    Selected content for SELECTION type queries.

    Contains the text or file paths that the user selected before
    invoking Wox. Empty for INPUT type queries.
    """

    env: QueryEnv
    """
    Environment context at query time.

    Contains information about the active window, browser URL,
    and other contextual information.
    """

    trigger_keyword: str = field(default="")
    """
    The trigger keyword for plugin-specific queries.

    When a plugin registers with a trigger keyword (e.g., ">todo"),
    this field contains that keyword. Empty for global queries.

    Example: For ">todo add item", trigger_keyword = "todo"
    """

    command: str = field(default="")
    """
    The command keyword after the trigger keyword.

    For queries with a command structure, this contains the first
    word after the trigger keyword.

    Example: For ">todo add item", command = "add"
    """

    search: str = field(default="")
    """
    The search text after the command.

    Contains the remaining text after extracting the trigger keyword
    and command. Used as the actual search/query parameter.

    Example: For ">todo add buy groceries", search = "buy groceries"
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        return json.dumps(
            {
                "QueryId": self.id,
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
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing query data

        Returns:
            A new Query instance
        """
        data = json.loads(json_str)

        if not data.get("Type"):
            data["Type"] = QueryType.INPUT

        return cls(
            id=data.get("QueryId", ""),
            type=QueryType(data.get("Type")),
            raw_query=data.get("RawQuery", ""),
            selection=Selection.from_json(data.get("Selection", Selection().to_json())),
            env=QueryEnv.from_json(data.get("Env", QueryEnv().to_json())),
            trigger_keyword=data.get("TriggerKeyword", ""),
            command=data.get("Command", ""),
            search=data.get("Search", ""),
        )

    def is_global_query(self) -> bool:
        """
        Check if this is a global query without trigger keyword.

        Global queries are queries that don't have a trigger keyword,
        meaning they're handled by the global search system rather
        than a specific plugin.

        Returns:
            True if this is a global INPUT query without trigger keyword

        Example:
            if query.is_global_query():
                # Handle global search
                return search_everywhere(query.search)
        """
        return self.type == QueryType.INPUT and not self.trigger_keyword

    def __str__(self) -> str:
        """
        Convert query to string representation.

        Returns:
            The raw query text for INPUT queries, the selection string
            for SELECTION queries, or empty string
        """
        if self.type == QueryType.INPUT:
            return self.raw_query
        elif self.type == QueryType.SELECTION:
            return str(self.selection)
        return ""


@dataclass
class ChangeQueryParam:
    """
    Parameters for changing the current query in Wox.

    Used with `api.change_query()` to programmatically change the
    query text and type.

    Attributes:
        query_type: The type of query to change to (INPUT or SELECTION)
        query_text: The new query text (for INPUT type)
        query_selection: The new selection (for SELECTION type)

    Example usage:
        # Change to a new input query
        await api.change_query(ctx, ChangeQueryParam(
            query_type=QueryType.INPUT,
            query_text="new search text"
        ))

        # Change to a selection query
        await api.change_query(ctx, ChangeQueryParam(
            query_type=QueryType.SELECTION,
            query_selection=Selection(
                type=SelectionType.TEXT,
                text="selected text"
            )
        ))
    """

    query_type: QueryType
    """
    The type of query to change to.

    Either INPUT (for text-based queries) or SELECTION (for selection-based).
    """

    query_text: str = field(default="")
    """
    The new query text for INPUT type queries.

    The text that will appear in the Wox search box.
    """

    query_selection: Selection = field(default_factory=Selection)
    """
    The new selection for SELECTION type queries.

    Contains the text or file selection to set.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        data = {
            "QueryType": self.query_type,
            "QueryText": self.query_text,
        }
        if self.query_selection:
            data["QuerySelection"] = json.loads(self.query_selection.to_json())
        return json.dumps(data)

    @classmethod
    def from_json(cls, json_str: str) -> "ChangeQueryParam":
        """
        Create from JSON string with camelCase naming.

        Args:
            json_str: JSON string containing change query parameters

        Returns:
            A new ChangeQueryParam instance
        """
        data = json.loads(json_str)

        if not data.get("QueryType"):
            data["QueryType"] = QueryType.INPUT

        return cls(
            query_type=QueryType(data.get("QueryType")),
            query_text=data.get("QueryText", ""),
            query_selection=Selection.from_json(data.get("QuerySelection", Selection().to_json())),
        )


@dataclass
class RefreshQueryParam:
    """
    Parameters for refreshing the current query.

    Used with `api.refresh_query()` to re-execute the current query
    with the same text, optionally preserving the selected result index.

    This is useful when plugin data changes and you want to update the
    displayed results without the user having to retype the query.

    Attributes:
        preserve_selected_index: Whether to keep the current selection position

    Example usage:
        # After deleting an item, refresh and reset selection
        await api.refresh_query(ctx, RefreshQueryParam(
            preserve_selected_index=False  # Reset to first item
        ))

        # After updating an item, refresh and keep selection
        await api.refresh_query(ctx, RefreshQueryParam(
            preserve_selected_index=True  # Keep current position
        ))
    """

    preserve_selected_index: bool = field(default=False)
    """
    Controls whether to maintain the previously selected item index after refresh.

    When True, the user's current selection index in the results list is preserved.
    This is useful when updating results without disrupting the user's position.

    When False, the selection resets to the first item (index 0).
    Use this when the selected item may have been removed or repositioned.

    Examples:
        - preserve_selected_index=True: After marking an item as favorite,
          the results list updates but the user stays on the same item
        - preserve_selected_index=False: After deleting the selected item,
          the selection moves to the first item
    """


class CopyType(str, Enum):
    """
    Enumeration of clipboard copy types.

    Defines the type of content to copy to the system clipboard.
    """

    TEXT = "text"
    """
    Copy text content.

    The text will be copied as plain text to the clipboard.
    """

    IMAGE = "image"
    """
    Copy image content.

    The image (as WoxImage) will be copied to the clipboard as image data.
    """


@dataclass
class CopyParams:
    """
    Parameters for copying content to the system clipboard.

    Used with `api.copy()` to copy text or images to the clipboard.

    Attributes:
        type: The type of content to copy (TEXT or IMAGE)
        text: The text content to copy (for TEXT type)
        wox_image: The WoxImage dict to copy (for IMAGE type)

    Example usage:
        # Copy text to clipboard
        await api.copy(ctx, CopyParams(
            type=CopyType.TEXT,
            text="Hello, World!"
        ))

        # Copy image to clipboard
        await api.copy(ctx, CopyParams(
            type=CopyType.IMAGE,
            wox_image=WoxImage.new_absolute("/path/to/image.png").to_dict()
        ))
    """

    type: CopyType = field(default=CopyType.TEXT)
    """
    The type of content to copy.

    Either TEXT (plain text) or IMAGE (WoxImage).
    """

    text: str = field(default="")
    """
    The text content to copy.

    Contains the text that will be copied to the clipboard.
    Only used when type is TEXT.
    """

    wox_image: Optional[dict] = field(default=None)
    """
    The WoxImage dictionary to copy.

    Contains a WoxImage serialized to dictionary format.
    Only used when type is IMAGE.
    """

    def to_json(self) -> str:
        """
        Convert to JSON string with camelCase naming.

        Returns:
            JSON string representation
        """
        return json.dumps(
            {
                "type": self.type,
                "text": self.text,
                "woxImage": self.wox_image,
            }
        )
