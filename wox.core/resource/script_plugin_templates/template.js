#!/usr/bin/env node
// {
//   "Id": "script-plugin-template",
//   "Name": "Script Plugin Template",
//   "Author": "Wox Team",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.0.0",
//   "Description": "A template for Wox script plugins",
//   "Icon": "emoji:📝",
//   "TriggerKeywords": ["spt"],
//   "SettingDefinitions": [
//     {
//       "Type": "textbox",
//       "Value": {
//         "Key": "api_key",
//         "Label": "API Key",
//         "Tooltip": "Enter your API key here (optional example)",
//         "DefaultValue": "",
//         "Style": {
//           "Width": 400
//         }
//       }
//     }
//   ]
// }

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
 * - action: Handle user selection of a result (optional, only needed for custom actions)
 *
 * Actions:
 * - Use "actions" field with an array of action objects (even for single action)
 * - Each action can have a "name" field (displayed in UI, defaults to "Execute")
 *
 * Built-in Actions (handled automatically by Wox, no need to implement in handleAction):
 * - copy-to-clipboard: Copy text to clipboard
 *   Usage: {id: "copy-to-clipboard", text: "text to copy"}
 * - open-url: Open URL in browser
 *   Usage: {id: "open-url", url: "https://example.com"}
 * - open-directory: Open directory in file manager
 *   Usage: {id: "open-directory", path: "/path/to/directory"}
 * - notify: Show notification
 *   Usage: {id: "notify", message: "notification message"}
 *
 * Custom Actions:
 * - Define your own action IDs and handle them in handleAction()
 * - Wox will call handleAction() for both built-in and custom actions as a hook
 * - For built-in actions, you can optionally handle them in handleAction() for additional logic
 *
 * Available environment variables:
 * - WOX_PLUGIN_ID: Plugin ID
 * - WOX_PLUGIN_NAME: Plugin name
 * - WOX_DIRECTORY_USER_SCRIPT_PLUGINS: Directory where script plugins are stored
 * - WOX_DIRECTORY_USER_DATA: User data directory
 * - WOX_DIRECTORY_WOX_DATA: Wox application data directory
 * - WOX_DIRECTORY_PLUGINS: Plugin directory
 * - WOX_DIRECTORY_THEMES: Theme directory
 * - WOX_SETTING_<KEY>: Plugin settings (e.g., WOX_SETTING_API_KEY for setting key "api_key")
 */

// Parse input from command line or stdin
let request;
try {
  if (process.argv.length > 2) {
    // From command line arguments
    request = JSON.parse(process.argv[2]);
  } else {
    // From stdin
    const data = require("fs").readFileSync(0, "utf-8");
    request = JSON.parse(data);
  }
} catch (e) {
  console.log(
    JSON.stringify({
      jsonrpc: "2.0",
      error: {
        code: -32700,
        message: "Parse error",
        data: e.message,
      },
      id: null,
    })
  );
  process.exit(1);
}

// Validate JSON-RPC request
if (request.jsonrpc !== "2.0") {
  console.log(
    JSON.stringify({
      jsonrpc: "2.0",
      error: {
        code: -32600,
        message: "Invalid Request",
        data: "Expected JSON-RPC 2.0",
      },
      id: request.id || null,
    })
  );
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
    console.log(
      JSON.stringify({
        jsonrpc: "2.0",
        error: {
          code: -32601,
          message: "Method not found",
          data: `Method '${request.method}' not supported`,
        },
        id: request.id,
      })
    );
    break;
}

/**
 * Handle query requests
 * @param {Object} request - The JSON-RPC request
 */
function handleQuery(request) {
  // Access plugin settings via environment variables
  // Settings are prefixed with WOX_SETTING_ and keys are uppercase
  const apiKey = process.env.WOX_SETTING_API_KEY || "";

  // Generate results
  const results = [
    {
      title: "Example: Single Action",
      subtitle: "Click to copy 'Hello Wox!' to clipboard",
      score: 100,
      actions: [
        {
          name: "Copy",
          id: "copy-to-clipboard",
          text: "Hello Wox!",
        },
      ],
    },
    {
      title: "Example: Multiple Actions",
      subtitle: "Right-click to see multiple actions",
      score: 90,
      actions: [
        {
          name: "Copy",
          id: "copy-to-clipboard",
          text: "Copied text",
        },
        {
          name: "Open Directory",
          id: "open-directory",
          path: process.env.WOX_DIRECTORY_USER_SCRIPT_PLUGINS,
        },
        {
          name: "Custom Action",
          id: "custom-action",
          data: "custom data",
        },
      ],
    },
    {
      title: "Settings Example",
      subtitle: `API Key configured: ${apiKey ? "Yes" : "No"}`,
      score: 70,
      actions: [
        {
          name: "Copy API Key",
          id: "copy-to-clipboard",
          text: `API Key: ${apiKey || "Not configured"}`,
        },
      ],
    },
  ];

  // Return results
  console.log(
    JSON.stringify({
      jsonrpc: "2.0",
      result: {
        items: results,
      },
      id: request.id,
    })
  );
}

/**
 * Handle action requests (OPTIONAL - only needed for custom actions)
 *
 * Built-in actions (copy-to-clipboard, open-url, open-directory, notify) are handled
 * automatically by Wox. You only need to implement this function if you have custom actions.
 *
 * Note: This function is called as a hook for ALL actions (built-in and custom), so you can
 * optionally add additional logic for built-in actions if needed.
 *
 * @param {Object} request - The JSON-RPC request
 */
function handleAction(request) {
  const actionId = request.params.id;
  const actionData = request.params.data || "";

  // Handle custom actions
  switch (actionId) {
    case "custom-action":
      // Example: Custom action that shows a notification
      console.log(
        JSON.stringify({
          jsonrpc: "2.0",
          result: {
            action: "notify",
            message: `Custom action triggered with data: ${actionData}`,
          },
          id: request.id,
        })
      );
      break;

    // For built-in actions, you can optionally add logic here
    // For example, logging when clipboard action is triggered:
    case "copy-to-clipboard":
      // Built-in action is already handled by Wox, this is just a hook
      // You can add additional logic here if needed
      console.log(
        JSON.stringify({
          jsonrpc: "2.0",
          result: {},
          id: request.id,
        })
      );
      break;

    default:
      // Return empty result for built-in actions or unknown actions
      console.log(
        JSON.stringify({
          jsonrpc: "2.0",
          result: {},
          id: request.id,
        })
      );
      break;
  }
}
