// {
//   "Id": "{{.PluginID}}",
//   "Name": "{{.Name}}",
//   "Author": "{{.Author}}",
//   "Version": "1.0.0",
//   "MinWoxVersion": "{{.MinWoxVersion}}",
//   "Runtime": "{{.Runtime}}",
//   "Description": "{{.Description}}",
//   "Icon": "emoji:🟨",
//   "TriggerKeywords": {{.TriggerKeywordsJSON}},
//   "SupportedOS": ["Windows", "Linux", "Macos"]
// }

/**
 * Wox Node.js Single-file SDK Plugin Template
 *
 * This file is loaded by the shared Node.js runtime host as CommonJS.
 * Query and action calls reuse that host process; they do not start a
 * new Node process.
 *
 * Do not import @wox-launcher/wox-plugin. Use params.API for the full
 * Public API, and object literals for images and results.
 *
 * First version does not support ESM, TypeScript, npm dependencies, or
 * relative image paths. Use a packaged .wox SDK plugin for those.
 *
 * Register OnUnload if you create timers, watchers, or sockets so reload
 * can clean them up.
 */

class MyPlugin {
  async init(ctx, params) {
    this.api = params.API
  }

  async query(ctx, query) {
    return {
      Results: [{
        Title: "{{.Name}}",
        SubTitle: query.Search || "Single-file Node.js SDK plugin",
        Icon: {
          ImageType: "emoji",
          ImageData: "🟨"
        },
        Actions: []
      }]
    }
  }
}

module.exports.plugin = new MyPlugin()
