#!/bin/bash
# {
#   "Id": "bash-script-template",
#   "Name": "Bash Script Template",
#   "Author": "Wox Team",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.0.0",
#   "Description": "A Bash template for Wox script plugins",
#   "Icon": "emoji:🐚",
#   "TriggerKeywords": ["bst"],
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

# Wox Bash Script Plugin Template
#
# This is a template for creating Wox script plugins in Bash.
# Script plugins are single-file plugins that are executed once per query.
#
# Communication with Wox is done via JSON-RPC over stdin/stdout.
#
# Available methods:
# - query: Process user queries and return results
# - action: Handle user selection of a result (optional, only needed for custom actions)
#
# Actions:
# - Use "actions" field with an array of action objects (even for single action)
# - Each action can have a "name" field (displayed in UI, defaults to "Execute")
#
# Built-in Actions (handled automatically by Wox, no need to implement in action method):
# - copy-to-clipboard: Copy text to clipboard
#   Usage: {"name": "Copy", "id": "copy-to-clipboard", "text": "text to copy"}
# - open-url: Open URL in browser
#   Usage: {"name": "Open", "id": "open-url", "url": "https://example.com"}
# - open-directory: Open directory in file manager
#   Usage: {"name": "Open Folder", "id": "open-directory", "path": "/path/to/directory"}
# - notify: Show notification
#   Usage: {"name": "Notify", "id": "notify", "message": "notification message"}
#
# Custom Actions:
# - Define your own action IDs and handle them in the action method
# - Wox will call the action method for both built-in and custom actions as a hook
# - For built-in actions, you can optionally handle them for additional logic
#
# Available environment variables:
# - WOX_PLUGIN_ID: Plugin ID
# - WOX_PLUGIN_NAME: Plugin name
# - WOX_DIRECTORY_USER_SCRIPT_PLUGINS: Directory where script plugins are stored
# - WOX_DIRECTORY_USER_DATA: User data directory
# - WOX_DIRECTORY_WOX_DATA: Wox application data directory
# - WOX_DIRECTORY_PLUGINS: Plugin directory
# - WOX_DIRECTORY_THEMES: Theme directory
# - WOX_SETTING_<KEY>: Plugin settings (e.g., WOX_SETTING_API_KEY for setting key "api_key")

# Read input from command line or stdin
if [ $# -gt 0 ]; then
  # From command line arguments
  REQUEST="$1"
else
  # From stdin
  REQUEST=$(cat)
fi

# Parse JSON-RPC request
# Note: This is a simple JSON parser for Bash
# For more complex JSON parsing, consider using jq if available
METHOD=$(echo "$REQUEST" | grep -o '"method"[^,}]*' | sed 's/"method"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
ID=$(echo "$REQUEST" | grep -o '"id"[^,}]*' | sed 's/"id"[[:space:]]*:[[:space:]]*\([^,}]*\).*/\1/')
JSONRPC=$(echo "$REQUEST" | grep -o '"jsonrpc"[^,}]*' | sed 's/"jsonrpc"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')

# Extract params based on method
if [ "$METHOD" = "query" ]; then
  SEARCH=$(echo "$REQUEST" | grep -o '"search"[^,}]*' | sed 's/"search"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
elif [ "$METHOD" = "action" ]; then
  ACTION_ID=$(echo "$REQUEST" | grep -o '"id"[^,}]*' | sed 's/"id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
  ACTION_DATA=$(echo "$REQUEST" | grep -o '"data"[^,}]*' | sed 's/"data"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
fi

# Validate JSON-RPC version
if [ "$JSONRPC" != "2.0" ]; then
  echo '{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request","data":"Expected JSON-RPC 2.0"},"id":null}'
  exit 1
fi

# Handle different methods
case "$METHOD" in
  "query")
    # Handle query request
    # Access plugin settings via environment variables
    # Settings are prefixed with WOX_SETTING_ and keys are uppercase
    API_KEY="${WOX_SETTING_API_KEY:-}"
    API_KEY_STATUS="No"
    if [ -n "$API_KEY" ]; then
      API_KEY_STATUS="Yes"
    fi

    # Generate results
    cat << EOF
{
  "jsonrpc": "2.0",
  "result": {
    "items": [
      {
        "title": "Example: Single Built-in Action",
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
            "path": "$WOX_DIRECTORY_USER_SCRIPT_PLUGINS"
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
        "subtitle": "API Key configured: $API_KEY_STATUS",
        "score": 70,
        "actions": [
          {
            "name": "Copy API Key",
            "id": "copy-to-clipboard",
            "text": "API Key: ${API_KEY:-Not configured}"
          }
        ]
      }
    ]
  },
  "id": $ID
}
EOF
    ;;
  "action")
    # Handle action request
    # For built-in actions, you can optionally handle them here for additional logic
    # For custom actions, you must handle them here
    case "$ACTION_ID" in
      "custom-action")
        # Handle custom action
        cat << EOF
{
  "jsonrpc": "2.0",
  "result": {},
  "id": $ID
}
EOF
        ;;
      *)
        # For built-in actions or unknown actions, return empty result
        # Wox will handle built-in actions automatically
        cat << EOF
{
  "jsonrpc": "2.0",
  "result": {},
  "id": $ID
}
EOF
        ;;
    esac
    ;;
  *)
    # Method not found
    cat << EOF
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32601,
    "message": "Method not found",
    "data": "Method '$METHOD' not supported"
  },
  "id": $ID
}
EOF
    ;;
esac
