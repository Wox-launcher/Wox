#!/bin/bash
# Required parameters:
# @wox.id bash-script-template
# @wox.name Bash Script Template
# @wox.keywords bst

# Optional parameters:
# @wox.icon 🐚
# @wox.version 1.0.0
# @wox.author Wox Team
# @wox.description A Bash template for Wox script plugins
# @wox.minWoxVersion 2.0.0

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
# - WOX_DIRECTORY_USER_SCRIPT_PLUGINS: Directory where script plugins are stored
# - WOX_DIRECTORY_USER_DATA: User data directory
# - WOX_DIRECTORY_WOX_DATA: Wox application data directory
# - WOX_DIRECTORY_PLUGINS: Plugin directory
# - WOX_DIRECTORY_THEMES: Theme directory

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
