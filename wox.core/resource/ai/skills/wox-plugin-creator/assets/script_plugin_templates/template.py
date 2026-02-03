#!/usr/bin/env python3
# {
#   "Id": "{{.PluginID}}",
#   "Name": "{{.Name}}",
#   "Author": "{{.Author}}",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.0.0",
#   "Description": "{{.Description}}",
#   "Icon": "emoji:🐍",
#   "TriggerKeywords": {{.TriggerKeywordsJSON}},
#   "SettingDefinitions": [
#     {
#       "Type": "textbox",
#       "Value": {
#         "Key": "api_key",
#         "Label": "API Key",
#         "Tooltip": "Enter your API key here (optional example)",
#         "DefaultValue": "",
#         "Style": {
#           "Width": 400
#         }
#       }
#     }
#   ],
#   "I18n": {
#     "en_US": {
#       "label1": "translation1"
#     },
#     "zh_CN": {
#       "label1": "翻译1"
#     }
#   }
# }

"""
Wox Python Script Plugin Template

This is a template for creating Wox script plugins in Python.
Script plugins are single-file plugins that are executed once per query.

IMPORTANT:
- Do not modify the base implementation in this file.
- Only edit the MyPlugin class section below.

To create your plugin:
1. Create a class that inherits from WoxAPI
2. Implement the `query` method to handle user queries
3. Optionally implement the `action` method to handle custom actions
4. Create an instance of your class at the bottom of the file

Communication with Wox is done via JSON-RPC over stdin/stdout.

Available methods in WoxAPI:
- query: Process user queries and return results (must be implemented)
- action: Handle user selection of a result (optional, only needed for custom actions)

Actions:
- Use "actions" field with an array of action objects (even for single action)
- Each action can have a "name" field (displayed in UI, defaults to "Execute")

Built-in Actions (handled automatically by Wox, no need to implement in action):
- copy-to-clipboard: Copy text to clipboard
  Usage: {"id": "copy-to-clipboard", "text": "text to copy"}
- open-url: Open URL in browser
  Usage: {"id": "open-url", "url": "https://example.com"}
- open-directory: Open directory in file manager
  Usage: {"id": "open-directory", "path": "/path/to/directory"}
- notify: Show notification
  Usage: {"id": "notify", "message": "notification message"}

Custom Actions:
- Define your own action IDs and handle them in action()
- Wox will call action() for both built-in and custom actions as a hook
- For built-in actions, you can optionally handle them in action() for additional logic

Available environment variables:
- WOX_PLUGIN_ID: Plugin ID
- WOX_PLUGIN_NAME: Plugin name
- WOX_DIRECTORY_USER_SCRIPT_PLUGINS: Directory where script plugins are stored
- WOX_DIRECTORY_USER_DATA: User data directory
- WOX_DIRECTORY_WOX_DATA: Wox application data directory
- WOX_DIRECTORY_PLUGINS: Plugin directory
- WOX_DIRECTORY_THEMES: Theme directory
- WOX_SETTING_<KEY>: Plugin settings (e.g., WOX_SETTING_API_KEY for setting key "api_key")
"""

from __future__ import annotations

import datetime
import json
import os
import sys
from typing import Any, List, TypedDict


class WoxPluginBase:
    """Wox plugin base class for script plugins. Do not modify this class."""

    class Preview(TypedDict, total=False):
        """Type hint for preview content in a result."""

        preview_type: str  # "markdown", "text"
        preview_data: str
        preview_properties: dict[str, str]

    class ActionItem(TypedDict, total=False):
        """Type hint for an action in a result."""

        id: str
        name: str
        icon: str
        """
        support following icon formats:
        - "base64:data:image/png;base64,xxx"
        - "emoji:😀"
        - "svg:<svg>...</svg>"
        - "fileicon:/absolute/path/to/file" (get system file icon)
        - "absolute:/absolute/path/to/image.png"
        - "url:https://example.com/image.png"
        """

        text: str
        url: str
        path: str
        message: str
        data: dict[str, str]

    class TailItem(TypedDict, total=False):
        """Type hint for a tail in a result."""

        id: str
        type: str  # "text" or "image"
        text: str
        image: str
        contextData: dict[str, str]

    class QueryResult(TypedDict, total=False):
        """Type hint for a query result item."""

        title: str
        subtitle: str
        icon: str
        """
        support following icon formats:
        - "base64:data:image/png;base64,xxx"
        - "emoji:😀"
        - "svg:<svg>...</svg>"
        - "fileicon:/absolute/path/to/file" (get system file icon)
        - "absolute:/absolute/path/to/image.png"
        - "url:https://example.com/image.png"
        """
        preview: WoxPluginBase.Preview
        tails: List[WoxPluginBase.TailItem]
        score: int
        actions: List[WoxPluginBase.ActionItem]

    class ActionResult(TypedDict, total=False):
        """Type hint for action method return value."""

        action: str
        message: str

    def __init__(self):
        self.log_file_path = __file__ + ".log"

    def log(self, message: str) -> None:
        """Log message to log file for debugging."""
        ts = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        message = f"[{ts}] {message}"
        if self.is_invoke_from_wox():
            with open(self.log_file_path, "a", encoding="utf-8") as f:
                f.write(f"{message}\n")
        else:
            print(f"LOG: {message}")

    def is_invoke_from_wox(self) -> bool:
        """Check if the script is invoked from Wox."""
        return "WOX_PLUGIN_ID" in os.environ

    def _build_response(self, result: Any, request_id: Any) -> dict:
        """Build a successful JSON-RPC 2.0 response."""
        return {"jsonrpc": "2.0", "result": result, "id": request_id}

    def _build_error_response(
        self, code: int, message: str, data: Any = None, request_id: Any = None
    ) -> dict:
        """Build a JSON-RPC 2.0 error response."""
        error = {"code": code, "message": message}
        if data is not None:
            error["data"] = data
        return {"jsonrpc": "2.0", "error": error, "id": request_id}

    def query(
        self, raw_query: str, trigger_keyword: str, command: str, search: str
    ) -> List[WoxPluginBase.QueryResult]:
        """
        Process user queries and return results.

        Args:
            raw_query: The full raw query string entered by the user
            trigger_keyword: The trigger keyword that activated this plugin
            command: The command part of the query (after trigger keyword)
            search: The search part of the query (after command)

            take `wpm install plugin-name` as an example
            raw_query: wpm install plugin-name
            trigger_keyword: wpm
            command: install
            search: plugin-name

        Returns:
            List of result dictionaries
        """
        # Default implementation - override in your plugin class
        return [
            {
                "title": "Hello Wox!",
                "subtitle": "This is a default result. Override the query method in your plugin class.",
                "score": 100,
                "actions": [
                    {"name": "Copy", "id": "copy-to-clipboard", "text": "Hello Wox!"}
                ],
            }
        ]

    def action(self, action_id: str, data: Any) -> None:
        """
        Handle action requests (OPTIONAL - only needed for custom actions)

        Built-in actions (copy-to-clipboard, open-url, open-directory, notify) are handled
        automatically by Wox. You only need to implement this method if you have custom actions.

        Note: This method is called as a hook for ALL actions (built-in and custom), so you can
        optionally add additional logic for built-in actions if needed.

        Args:
            action_id: The ID of the action
            data: Additional data for the action
        """
        # Default implementation - override for custom actions
        if action_id == "custom-action":
            # Custom action handling logic can be added here
            pass

    def handle_query(self, params, request_id):
        """Internal method to handle query requests"""

        search = params.get("search", "")
        trigger_keyword = params.get("trigger_keyword", "")
        command = params.get("command", "")
        raw_query = params.get("raw_query", "")

        results = self.query(raw_query, trigger_keyword, command, search)
        return self._build_response({"items": results}, request_id)

    def handle_action(self, params, request_id):
        """Internal method to handle action requests"""
        action_id = params.get("id", "")
        action_data = params.get("data", "")
        self.action(action_id, action_data)
        return self._build_response({}, request_id)

    def run(self):
        """Main entry point for the script plugin"""
        # Parse input
        if self.is_invoke_from_wox():
            # Running from Wox - read from stdin
            try:
                stdin_text = sys.stdin.read()
            except Exception:
                return 1
        else:
            # Manual testing mode
            print("Manual mode - please enter query:")
            query_input = input()
            stdin_text = (
                '{"jsonrpc": "2.0", "method": "query", "params": {"query": "'
                + query_input
                + '"}, "id": 1}'
            )

        # Parse JSON-RPC 2.0 request
        try:
            request = json.loads(stdin_text)
        except json.JSONDecodeError as e:
            error_response = self._build_error_response(
                -32700, "Parse error", str(e), None
            )
            print(json.dumps(error_response, ensure_ascii=False))
            return 1

        # Validate JSON-RPC 2.0 format
        if request.get("jsonrpc") != "2.0":
            error_response = self._build_error_response(
                -32600, "Invalid Request", None, request.get("id")
            )
            print(json.dumps(error_response, ensure_ascii=False))
            return 1

        method = request.get("method")
        params = request.get("params", {})
        request_id = request.get("id")

        # Handle different methods
        if method == "query":
            response = self.handle_query(params, request_id)
        elif method == "action":
            response = self.handle_action(params, request_id)
        else:
            # Method not found
            response = self._build_error_response(
                -32601,
                "Method not found",
                f"Method '{method}' not supported",
                request_id,
            )

        # Output response
        print(json.dumps(response, ensure_ascii=False))
        return 0


class MyPlugin(WoxPluginBase):
    def query(
        self, raw_query: str, trigger_keyword: str, command: str, search: str
    ) -> List[WoxPluginBase.QueryResult]:
        # Access plugin settings via environment variables
        # Settings are prefixed with WOX_SETTING_ and keys are uppercase
        api_key = os.getenv("WOX_SETTING_API_KEY", "")

        # Generate results based on query
        results: List[WoxPluginBase.QueryResult] = [
            {
                "title": f"Query: {search}",
                "icon": "emoji:🔍",
                "subtitle": "Click to copy the query to clipboard",
                "score": 100,
                "actions": [
                    {"name": "Copy", "id": "copy-to-clipboard", "text": search}
                ],
            },
            {
                "title": "Example: Multiple Actions",
                "icon": "emoji:⚙️",
                "subtitle": "Right-click to see multiple actions",
                "score": 90,
                "actions": [
                    {"name": "Copy", "id": "copy-to-clipboard", "text": "Copied text"},
                    {
                        "name": "Open Directory",
                        "id": "open-directory",
                        "path": os.environ.get("WOX_DIRECTORY_USER_SCRIPT_PLUGINS", ""),
                    },
                    {
                        "name": "Custom Action",
                        "id": "custom-action",
                        "data": {"key": "value"},
                    },
                ],
            },
            {
                "title": "Settings Example",
                "icon": "emoji:⚙️",
                "subtitle": f"API Key configured: {'Yes' if api_key else 'No'}",
                "score": 70,
                "actions": [
                    {
                        "name": "Copy API Key",
                        "id": "copy-to-clipboard",
                        "text": f"API Key: {api_key if api_key else 'Not configured'}",
                    }
                ],
            },
        ]

        return results

    def action(self, action_id: str, data: Any) -> None:
        # Handle custom actions
        if action_id == "custom-action":
            # Custom action handling logic can be added here
            pass


if __name__ == "__main__":
    sys.exit(MyPlugin().run())
