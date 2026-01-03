#!/usr/bin/env node
// {
//   "Id": "{{.PluginID}}",
//   "Name": "{{.Name}}",
//   "Author": "{{.Author}}",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.0.0",
//   "Description": "{{.Description}}",
//   "Icon": "emoji:📝",
//   "TriggerKeywords": {{.TriggerKeywordsJSON}},
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
 * IMPORTANT:
 * - Do not modify the base implementation in this file.
 * - Only edit the MyPlugin class section below.
 *
 * Communication with Wox is done via JSON-RPC over stdin/stdout.
 *
 * To create your plugin:
 * 1. Create a class that inherits from WoxPluginBase
 * 2. Implement the `query` method to handle user queries
 * 3. Optionally implement the `action` method to handle custom actions
 * 4. Create an instance of your class at the bottom of the file
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

/**
 * Wox plugin base class for script plugins. Do not modify this class.
 */
class WoxPluginBase {
  constructor() {
    this.logFilePath = `${__filename}.log`;
  }

  /**
   * Process user queries and return results.
   * @param {Object} input
   * @param {string} input.rawQuery
   * @param {string} input.triggerKeyword
   * @param {string} input.command
   * @param {string} input.search
   * @returns {Array<Object>}
   */
  query(input) {
    return [
      {
        title: "Hello Wox!",
        subtitle: "This is a default result. Override the query method in your plugin class.",
        score: 100,
        actions: [{ name: "Copy", id: "copy-to-clipboard", text: "Hello Wox!" }],
      },
    ];
  }

  /**
   * Handle action requests (OPTIONAL - only needed for custom actions)
   * @param {Object} input
   * @param {string} input.id
   * @param {any} input.data
   */
  action(input) {
    if (input.id === "custom-action") {
      return {
        action: "notify",
        message: `Custom action triggered with data: ${input.data || ""}`,
      };
    }
  }

  isInvokeFromWox() {
    return Boolean(process.env.WOX_PLUGIN_ID);
  }

  log(message) {
    const ts = new Date();
    const pad = (value) => String(value).padStart(2, "0");
    const formatted = `${ts.getFullYear()}-${pad(ts.getMonth() + 1)}-${pad(ts.getDate())} ${pad(ts.getHours())}:${pad(ts.getMinutes())}:${pad(ts.getSeconds())}`;
    const line = `[${formatted}] ${message}`;
    if (this.isInvokeFromWox()) {
      require("fs").appendFileSync(this.logFilePath, `${line}\n`, "utf-8");
    } else {
      console.log(`LOG: ${line}`);
    }
  }

  buildResponse(result, requestId) {
    return { jsonrpc: "2.0", result, id: requestId };
  }

  buildErrorResponse(code, message, data, requestId) {
    const error = { code, message };
    if (data !== undefined && data !== null) {
      error.data = data;
    }
    return { jsonrpc: "2.0", error, id: requestId };
  }

  parseRequest() {
    if (process.argv.length > 2) {
      return JSON.parse(process.argv[2]);
    }
    const data = require("fs").readFileSync(0, "utf-8");
    return JSON.parse(data);
  }

  handleQuery(params, requestId) {
    const result = this.query({
      rawQuery: params.raw_query || "",
      triggerKeyword: params.trigger_keyword || "",
      command: params.command || "",
      search: params.search || "",
    });
    return this.buildResponse({ items: result }, requestId);
  }

  handleAction(params, requestId) {
    const result = this.action({ id: params.id || "", data: params.data || "" });
    return this.buildResponse(result || {}, requestId);
  }

  readManualQuery() {
    return new Promise((resolve, reject) => {
      const readline = require("readline");
      const rl = readline.createInterface({
        input: process.stdin,
        output: process.stdout,
      });

      rl.question("", (answer) => {
        rl.close();
        resolve(answer);
      });

      rl.on("error", (err) => {
        rl.close();
        reject(err);
      });
    });
  }

  async run() {
    let request;
    if (this.isInvokeFromWox()) {
      try {
        request = this.parseRequest();
      } catch (e) {
        console.log(JSON.stringify(this.buildErrorResponse(-32700, "Parse error", e.message, null)));
        return 1;
      }
    } else {
      console.log("Manual mode - please enter query:");
      let queryInput = "";
      try {
        queryInput = await this.readManualQuery();
      } catch (_e) {
        return 1;
      }
      request = {
        jsonrpc: "2.0",
        method: "query",
        params: { query: queryInput },
        id: 1,
      };
    }

    if (request.jsonrpc !== "2.0") {
      console.log(JSON.stringify(this.buildErrorResponse(-32600, "Invalid Request", "Expected JSON-RPC 2.0", request.id || null)));
      return 1;
    }

    const params = request.params || {};
    const requestId = request.id;
    let response;

    switch (request.method) {
      case "query":
        response = this.handleQuery(params, requestId);
        break;
      case "action":
        response = this.handleAction(params, requestId);
        break;
      default:
        response = this.buildErrorResponse(-32601, "Method not found", `Method '${request.method}' not supported`, requestId);
        break;
    }

    console.log(JSON.stringify(response));
    return 0;
  }
}

class MyPlugin extends WoxPluginBase {
  query(input) {
    // Access plugin settings via environment variables
    // Settings are prefixed with WOX_SETTING_ and keys are uppercase
    const apiKey = process.env.WOX_SETTING_API_KEY || "";

    return [
      {
        title: `Query: ${input.search}`,
        subtitle: "Click to copy the query to clipboard",
        icon: "emoji:📋",
        score: 100,
        actions: [
          {
            name: "Copy",
            icon: "emoji:📋",
            id: "copy-to-clipboard",
            text: input.search,
          },
        ],
      },
      {
        title: "Example: Multiple Actions",
        subtitle: "Alt/Cmd + J to see multiple actions",
        icon: "emoji:⚙️",
        score: 90,
        actions: [
          {
            name: "Copy",
            icon: "emoji:📋",
            id: "copy-to-clipboard",
            text: "Copied text",
          },
          {
            name: "Open Directory",
            icon: "emoji:📁",
            id: "open-directory",
            path: process.env.WOX_DIRECTORY_USER_SCRIPT_PLUGINS,
          },
          {
            name: "Custom Action",
            icon: "emoji:🚀",
            id: "custom-action",
            data: "custom data",
          },
        ],
      },
      {
        title: "Settings Example",
        subtitle: `API Key configured: ${apiKey ? "Yes" : "No"}`,
        icon: "emoji:🔑",
        score: 70,
        actions: [
          {
            name: "Copy API Key",
            icon: "emoji:📋",
            id: "copy-to-clipboard",
            text: `API Key: ${apiKey || "Not configured"}`,
          },
        ],
      },
    ];
  }

  action(input) {
    if (input.id === "custom-action") {
      return {
        action: "notify",
        message: `Custom action triggered with data: ${input.data || ""}`,
      };
    }
  }
}

if (require.main === module) {
  new MyPlugin()
    .run()
    .then((code) => process.exit(code))
    .catch(() => process.exit(1));
}
