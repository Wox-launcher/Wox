import { WoxImage } from "../types/index.js"

export function NewBase64WoxImage(imageData: string): WoxImage {
  return {
    ImageType: "base64",
    ImageData: imageData
  }
}
