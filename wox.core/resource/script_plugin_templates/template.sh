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
# - action: Handle user selection of a result
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
        "title": "Open Plugin Directory",
        "subtitle": "Open the script plugins directory in file manager",
        "score": 100,
        "action": {
          "id": "open-plugin-directory",
          "data": ""
        }
      },
      {
        "title": "Settings Example",
        "subtitle": "API Key configured: $API_KEY_STATUS",
        "score": 90,
        "action": {
          "id": "show-settings",
          "data": ""
        }
      }
    ]
  },
  "id": $ID
}
EOF
    ;;
  "action")
    # Handle action request
    case "$ACTION_ID" in
      "open-plugin-directory")
        # Open plugin directory action
        cat << EOF
{
  "jsonrpc": "2.0",
  "result": {
    "action": "open-directory",
    "path": "$WOX_DIRECTORY_USER_SCRIPT_PLUGINS"
  },
  "id": $ID
}
EOF
        ;;
      *)
        # Unknown action
        cat << EOF
{
  "jsonrpc": "2.0",
  "error": {
    "code": -32000,
    "message": "Unknown action",
    "data": "Action '$ACTION_ID' not supported"
  },
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
