#!/usr/bin/env python3
# Required parameters:
# @wox.id python-script-template
# @wox.name Python Script Template
# @wox.keywords pst

# Optional parameters:
# @wox.icon 🐍
# @wox.version 1.0.0
# @wox.author Wox Team
# @wox.description A Python template for Wox script plugins
# @wox.minWoxVersion 2.0.0

"""
Wox Python Script Plugin Template

This is a template for creating Wox script plugins in Python.
Script plugins are single-file plugins that are executed once per query.

Communication with Wox is done via JSON-RPC over stdin/stdout.

Available methods:
- query: Process user queries and return results
- action: Handle user selection of a result
"""

import sys
import json


def handle_query(params, request_id):
    """
    Handle query requests
    
    Args:
        params: The parameters from the JSON-RPC request
        request_id: The ID of the JSON-RPC request
    
    Returns:
        A JSON-RPC response with the query results
    """
    query = params.get("search", "")
    
    # Generate results based on the query
    results = [
        {
            "title": f"You searched for: {query}",
            "subtitle": "This is a template result",
            "score": 100,
            "action": {
                "id": "example-action",
                "data": query
            }
        },
        {
            "title": "Another result",
            "subtitle": "With a different action",
            "score": 90,
            "action": {
                "id": "open-url",
                "data": "https://github.com/Wox-launcher/Wox"
            }
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
    Handle action requests
    
    Args:
        params: The parameters from the JSON-RPC request
        request_id: The ID of the JSON-RPC request
    
    Returns:
        A JSON-RPC response with the action result
    """
    action_id = params.get("id", "")
    action_data = params.get("data", "")
    
    # Handle different action types
    if action_id == "example-action":
        # Example action that returns a message
        return {
            "jsonrpc": "2.0",
            "result": {
                "action": "notify",
                "message": f"You selected: {action_data}"
            },
            "id": request_id
        }
    elif action_id == "open-url":
        # Open URL action
        return {
            "jsonrpc": "2.0",
            "result": {
                "action": "open-url",
                "url": action_data
            },
            "id": request_id
        }
    else:
        # Unknown action
        return {
            "jsonrpc": "2.0",
            "error": {
                "code": -32000,
                "message": "Unknown action",
                "data": f"Action '{action_id}' not supported"
            },
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
