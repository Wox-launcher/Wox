// {
//   "Id": "{{.PluginID}}",
//   "Name": "{{.Name}}",
//   "Author": "{{.Author}}",
//   "Version": "1.0.0",
//   "MinWoxVersion": "{{.MinWoxVersion}}",
//   "Runtime": "{{.Runtime}}",
//   "Description": "{{.Description}}",
//   "Icon": "svg:<svg xmlns='http://www.w3.org/2000/svg' width='48' height='48' viewBox='0 0 48 48'><rect width='48' height='48' rx='12' fill='#2563eb'/><path d='M14 11h14l7 7v19H14z' fill='#eff6ff'/><path d='M28 11v7h7' fill='#bfdbfe'/><path d='M19 24h11M19 29h11M19 34h7' fill='none' stroke='#2563eb' stroke-width='2.5' stroke-linecap='round'/></svg>",
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
          ImageType: "svg",
          ImageData: "<svg xmlns='http://www.w3.org/2000/svg' width='48' height='48' viewBox='0 0 48 48'><rect width='48' height='48' rx='12' fill='#2563eb'/><path d='M14 11h14l7 7v19H14z' fill='#eff6ff'/><path d='M28 11v7h7' fill='#bfdbfe'/><path d='M19 24h11M19 29h11M19 34h7' fill='none' stroke='#2563eb' stroke-width='2.5' stroke-linecap='round'/></svg>"
        },
        Actions: []
      }]
    }
  }
}

module.exports.plugin = new MyPlugin()
