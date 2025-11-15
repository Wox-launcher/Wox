#!/usr/bin/env python3
# {
#   "Id": "python-script-template",
#   "Name": "Python Script Template",
#   "Author": "Wox Team",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.0.0",
#   "Description": "A Python template for Wox script plugins",
#   "Icon": "emoji:🐍",
#   "TriggerKeywords": ["pst"],
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
#   ]
# }

"""
Wox Python Script Plugin Template

This is a template for creating Wox script plugins in Python.
Script plugins are single-file plugins that are executed once per query.

Communication with Wox is done via JSON-RPC over stdin/stdout.

Available methods:
- query: Process user queries and return results
- action: Handle user selection of a result (optional, only needed for custom actions)

Actions:
- Use "actions" field with an array of action objects (even for single action)
- Each action can have a "name" field (displayed in UI, defaults to "Execute")

Built-in Actions (handled automatically by Wox, no need to implement in handle_action):
- copy-to-clipboard: Copy text to clipboard
  Usage: {"id": "copy-to-clipboard", "text": "text to copy"}
- open-url: Open URL in browser
  Usage: {"id": "open-url", "url": "https://example.com"}
- open-directory: Open directory in file manager
  Usage: {"id": "open-directory", "path": "/path/to/directory"}
- notify: Show notification
  Usage: {"id": "notify", "message": "notification message"}

Custom Actions:
- Define your own action IDs and handle them in handle_action()
- Wox will call handle_action() for both built-in and custom actions as a hook
- For built-in actions, you can optionally handle them in handle_action() for additional logic

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

import sys
import json
import os


def handle_query(params, request_id):
    """
    Handle query requests

    Args:
        params: The parameters from the JSON-RPC request (unused in this template)
        request_id: The ID of the JSON-RPC request

    Returns:
        A JSON-RPC response with the query results
    """
    # Access plugin settings via environment variables
    # Settings are prefixed with WOX_SETTING_ and keys are uppercase
    api_key = os.getenv('WOX_SETTING_API_KEY', '')

    # Generate results
    results = [
        {
            "title": "Example: Single Action",
            "subtitle": "Click to copy 'Hello Wox!' to clipboard",
            "score": 100,
            "actions": [
                {
                    "name": "Copy",
                    "id": "copy-to-clipboard",
                    "text": "Hello Wox!"
                }
            ]
        },
        {
            "title": "Example: Multiple Actions",
            "subtitle": "Right-click to see multiple actions",
            "score": 90,
            "actions": [
                {
                    "name": "Copy",
                    "id": "copy-to-clipboard",
                    "text": "Copied text"
                },
                {
                    "name": "Open Directory",
                    "id": "open-directory",
                    "path": os.environ.get("WOX_DIRECTORY_USER_SCRIPT_PLUGINS", "")
                },
                {
                    "name": "Custom Action",
                    "id": "custom-action",
                    "data": "custom data"
                }
            ]
        },
        {
            "title": "Settings Example",
            "subtitle": f"API Key configured: {'Yes' if api_key else 'No'}",
            "score": 70,
            "actions": [
                {
                    "name": "Copy API Key",
                    "id": "copy-to-clipboard",
                    "text": f"API Key: {api_key if api_key else 'Not configured'}"
                }
            ]
        }
    ]

    # Return results
    return {
        "jsonrpc": "2.0",
        "result": {
            "items": results
        },
        "id": request_id
    }


def handle_action(params, request_id):
    """
    Handle action requests (OPTIONAL - only needed for custom actions)

    Built-in actions (copy-to-clipboard, open-url, open-directory, notify) are handled
    automatically by Wox. You only need to implement this function if you have custom actions.

    Note: This function is called as a hook for ALL actions (built-in and custom), so you can
    optionally add additional logic for built-in actions if needed.

    Args:
        params: The parameters from the JSON-RPC request
        request_id: The ID of the JSON-RPC request

    Returns:
        A JSON-RPC response (can be empty for built-in actions)
    """
    action_id = params.get("id", "")
    action_data = params.get("data", "")

    # Handle custom actions
    if action_id == "custom-action":
        # Example: Custom action that shows a notification
        return {
            "jsonrpc": "2.0",
            "result": {
                "action": "notify",
                "message": f"Custom action triggered with data: {action_data}"
            },
            "id": request_id
        }

    # For built-in actions, you can optionally add logic here
    # For example, logging when clipboard action is triggered:
    elif action_id == "copy-to-clipboard":
        # Built-in action is already handled by Wox, this is just a hook
        # You can add additional logic here if needed
        pass

    # Return empty result for built-in actions or unknown actions
    return {
        "jsonrpc": "2.0",
        "result": {},
        "id": request_id
    }


def main():
    """Main entry point for the script plugin"""
    # Parse input
    try:
        if len(sys.argv) > 1:
            # From command line arguments
            request = json.loads(sys.argv[1])
        else:
            # From stdin
            request = json.loads(sys.stdin.read())
    except json.JSONDecodeError as e:
        # Return parse error
        print(json.dumps({
            "jsonrpc": "2.0",
            "error": {
                "code": -32700,
                "message": "Parse error",
                "data": str(e)
            },
            "id": None
        }))
        return 1

    # Validate JSON-RPC request
    if request.get("jsonrpc") != "2.0":
        print(json.dumps({
            "jsonrpc": "2.0",
            "error": {
                "code": -32600,
                "message": "Invalid Request",
                "data": "Expected JSON-RPC 2.0"
            },
            "id": request.get("id")
        }))
        return 1

    # Handle different methods
    method = request.get("method")
    params = request.get("params", {})
    request_id = request.get("id")

    if method == "query":
        response = handle_query(params, request_id)
    elif method == "action":
        response = handle_action(params, request_id)
    else:
        # Method not found
        response = {
            "jsonrpc": "2.0",
            "error": {
                "code": -32601,
                "message": "Method not found",
                "data": f"Method '{method}' not supported"
            },
            "id": request_id
        }

    # Output response
    print(json.dumps(response))
    return 0


if __name__ == "__main__":
    sys.exit(main())
