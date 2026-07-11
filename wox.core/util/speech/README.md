# Speech native loading

Dictation uses sherpa-onnx through its C API, loaded from `.wox/dictation` when dictation first starts.

Do not import `github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx` directly here. That package links `sherpa-onnx-c-api` and `onnxruntime` at build time, which makes Wox depend on those native libraries during process startup. Dictation is optional, so Wox should start without loading or resolving sherpa/onnxruntime.

The flow is:

1. `resource` embeds the common dictation files plus only the native libraries for the current `GOOS/GOARCH`.
2. Startup extracts them to `.wox/dictation`.
3. `speech` calls `LoadLibraryExW` on Windows or `dlopen` on macOS/Linux the first time a recognizer or VAD is created.
4. Models are still owned by the recognizer/VAD pools. Lazy dictation loading evicts idle recognizers; eager loading keeps the selected recognizer resident until model switch or plugin unload.

Native libraries are intentionally not unloaded. ONNX Runtime and sherpa keep process-wide state; unloading and reloading them is riskier than keeping the library images mapped after first use.
