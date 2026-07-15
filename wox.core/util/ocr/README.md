# OCR engines

Wox uses the operating system OCR implementation by default. It does not download an OCR model until an OCR consumer selects and downloads `PaddleOCR v6 Small`.

Each OCR consumer keeps its own `ocr_model` setting. The setting stores `system` or `paddle_ppocrv6_small`; model files themselves are shared.

The local layout is:

```text
~/.wox/runtime/onnxruntime/1.27.0/<goos>-<goarch>/
~/.wox/runtime/sherpa-onnx/v1.13.4-wox.1/<goos>-<goarch>/
~/.wox/models/ocr/paddle_ppocrv6_small/
```

The PaddleOCR model downloader verifies the size and SHA-256 of every upstream ONNX/config file before atomically replacing the installed model. The PP-OCRv6 recognition character dictionary is generated from the verified `inference.yml` file.

ONNX Runtime is shared with dictation. The current signed Wox native release delivers ONNX Runtime together with sherpa-onnx; the runtime manager separates the installed files by runtime ownership. Native libraries intentionally remain loaded after first use because both ONNX Runtime and sherpa-onnx keep process-wide state.
