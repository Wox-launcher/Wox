import { WoxImage } from "../types"

export function NewBase64WoxImage(imageData: string): WoxImage {
  return {
    ImageType: "base64",
    ImageData: imageData
  }
}
