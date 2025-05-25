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
  const query = request.params.search || "";
  
  // Generate results based on the query
  const results = [
    {
      title: `You searched for: ${query}`,
      subtitle: "This is a template result",
      score: 100,
      action: {
        id: "example-action",
        data: query
      }
    },
    {
      title: "Another result",
      subtitle: "With a different action",
      score: 90,
      action: {
        id: "open-url",
        data: "https://github.com/Wox-launcher/Wox"
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
  const actionData = request.params.data;
  
  // Handle different action types
  switch (actionId) {
    case "example-action":
      // Example action that returns a message
      console.log(JSON.stringify({
        jsonrpc: "2.0",
        result: {
          action: "notify",
          message: `You selected: ${actionData}`
        },
        id: request.id
      }));
      break;
    case "open-url":
      // Open URL action
      console.log(JSON.stringify({
        jsonrpc: "2.0",
        result: {
          action: "open-url",
          url: actionData
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
