import path from "path"

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
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  return require(modulePath)
}

export function evictCommonJSModule(modulePath: string): void {
  try {
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    delete require.cache[require.resolve(modulePath)]
  } catch {
    // The module may never have been loaded.
  }
}
