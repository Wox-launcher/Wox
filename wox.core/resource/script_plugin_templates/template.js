#!/usr/bin/env node
// Required parameters:
// @wox.id script-plugin-template
// @wox.name Script Plugin Template
// @wox.keywords spt

// Optional parameters:
// @wox.icon 📝
// @wox.version 1.0.0
// @wox.author Wox Team
// @wox.description A template for Wox script plugins
// @wox.minWoxVersion 2.0.0

/**
 * Wox Script Plugin Template
 *
 * This is a template for creating Wox script plugins.
 * Script plugins are single-file plugins that are executed once per query.
 *
 * Communication with Wox is done via JSON-RPC over stdin/stdout.
 *
 * Available methods:
 * - query: Process user queries and return results
 * - action: Handle user selection of a result
 *
 * Available environment variables:
 * - WOX_DIRECTORY_USER_SCRIPT_PLUGINS: Directory where script plugins are stored
 * - WOX_DIRECTORY_USER_DATA: User data directory
 * - WOX_DIRECTORY_WOX_DATA: Wox application data directory
 * - WOX_DIRECTORY_PLUGINS: Plugin directory
 * - WOX_DIRECTORY_THEMES: Theme directory
 */

// Parse input from command line or stdin
let request;
try {
  if (process.argv.length > 2) {
    // From command line arguments
    request = JSON.parse(process.argv[2]);
  } else {
    // From stdin
    const data = require('fs').readFileSync(0, 'utf-8');
    request = JSON.parse(data);
  }
} catch (e) {
  console.log(JSON.stringify({
    jsonrpc: "2.0",
    error: {
      code: -32700,
      message: "Parse error",
      data: e.message
    },
    id: null
  }));
  process.exit(1);
}

// Validate JSON-RPC request
if (request.jsonrpc !== "2.0") {
  console.log(JSON.stringify({
    jsonrpc: "2.0",
    error: {
      code: -32600,
      message: "Invalid Request",
      data: "Expected JSON-RPC 2.0"
    },
    id: request.id || null
  }));
  process.exit(1);
}

// Handle different methods
switch (request.method) {
  case "query":
    handleQuery(request);
    break;
  case "action":
    handleAction(request);
    break;
  default:
    // Method not found
    console.log(JSON.stringify({
      jsonrpc: "2.0",
      error: {
        code: -32601,
        message: "Method not found",
        data: `Method '${request.method}' not supported`
      },
      id: request.id
    }));
    break;
}

/**
 * Handle query requests
 * @param {Object} request - The JSON-RPC request
 */
function handleQuery(request) {
  // Generate results
  const results = [
    {
      title: "Open Plugin Directory",
      subtitle: "Open the script plugins directory in file manager",
      score: 100,
      action: {
        id: "open-plugin-directory",
        data: ""
      }
    }
  ];

  // Return results
  console.log(JSON.stringify({
    jsonrpc: "2.0",
    result: {
      items: results
    },
    id: request.id
  }));
}

/**
 * Handle action requests
 * @param {Object} request - The JSON-RPC request
 */
function handleAction(request) {
  const actionId = request.params.id;

  // Handle different action types
  switch (actionId) {
    case "open-plugin-directory":
      // Open plugin directory action
      console.log(JSON.stringify({
        jsonrpc: "2.0",
        result: {
          action: "open-directory",
          path: process.env.WOX_DIRECTORY_USER_SCRIPT_PLUGINS
        },
        id: request.id
      }));
      break;
    default:
      // Unknown action
      console.log(JSON.stringify({
        jsonrpc: "2.0",
        error: {
          code: -32000,
          message: "Unknown action",
          data: `Action '${actionId}' not supported`
        },
        id: request.id
      }));
      break;
  }
}
