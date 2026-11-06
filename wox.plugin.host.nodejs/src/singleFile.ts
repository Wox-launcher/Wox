import path from "path"
import Module from "module"
import * as woxPlugin from "@wox-launcher/wox-plugin"

export function assertPathWithinDirectory(filePath: string, directory: string): string {
  const resolvedFile = path.resolve(filePath)
  const resolvedDir = path.resolve(directory)
  const relative = path.relative(resolvedDir, resolvedFile)
  if (relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative)) {
    throw new Error(`plugin entry escapes plugin directory: ${filePath}`)
  }
  return resolvedFile
}

export function getPluginExport(loaded: { plugin?: unknown; default?: { plugin?: unknown } } | null | undefined): unknown {
  const pluginExport = loaded?.plugin ?? loaded?.default?.plugin
  if (pluginExport === undefined || pluginExport === null) {
    throw new Error("plugin doesn't export plugin object")
  }
  return pluginExport
}

export function loadCommonJSModule(modulePath: string): unknown {
  const nativeRequire = Module.createRequire(modulePath)
  const resolved = nativeRequire.resolve(modulePath)
  const cached = nativeRequire.cache[resolved]
  if (cached) return cached.exports
  const plugin = new Module(resolved) as Module & { load: (filename: string) => void }
  const pluginRequire = plugin.require.bind(plugin)
  // The SDK is bundled in the host, so it is not in the plugin's module search path.
  plugin.require = ((name: string) => (name === "@wox-launcher/wox-plugin" ? woxPlugin : pluginRequire(name))) as NodeJS.Require
  nativeRequire.cache[resolved] = plugin
  try {
    plugin.load(resolved)
    return plugin.exports
  } catch (error) {
    delete nativeRequire.cache[resolved]
    throw error
  }
}

export function evictCommonJSModule(modulePath: string): void {
  try {
    const nativeRequire = Module.createRequire(modulePath)
    delete nativeRequire.cache[nativeRequire.resolve(modulePath)]
  } catch {
    // The module may never have been loaded.
  }
}
