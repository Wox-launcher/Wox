import { WoxImage } from "../types/index.js"

/**
 * Create a base64 WoxImage from image data.
 *
 * This helper function creates a WoxImage with type "base64".
 * The image data must include the data URI prefix.
 *
 * @param imageData - Base64 image data with data URI prefix (e.g., "data:image/png;base64,...")
 * @returns A WoxImage with type "base64"
 *
 * @example
 * ```typescript
 * const pngData = "data:image/png;base64,iVBORw0KGgoAAAA..."
 * const icon = NewBase64WoxImage(pngData)
 *
 * // Use in a result
 * const result = {
 *   Title: "My Result",
 *   Icon: icon
 * }
 * ```
 */
export function NewBase64WoxImage(imageData: string): WoxImage {
  return {
    ImageType: "base64",
    ImageData: imageData
  }
}
