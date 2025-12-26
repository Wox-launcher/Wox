/**
 * Wox Plugin SDK for TypeScript/JavaScript
 *
 * This package provides utility functions for developing Wox plugins in TypeScript/JavaScript.
 *
 * @module
 *
 * @example
 * ```typescript
 * import { NewContext, NewBase64WoxImage } from "wox-plugin"
 *
 * // Create a new context
 * const ctx = NewContext()
 *
 * // Create a base64 image
 * const icon = NewBase64WoxImage("data:image/png;base64,...")
 * ```
 */

// Re-export all context-related functions
export * from "./context.js"

// Re-export all image-related functions
export * from "./image.js"
