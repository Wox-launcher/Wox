# Speech native loading

Dictation uses sherpa-onnx through its C API, loaded on demand when dictation first starts. Native executables are published by `Wox-launcher/Wox.Dictation.Native.Dependecies`; Wox authenticates its Ed25519-signed release manifest, verifies archive and file SHA-256 digests, and verifies macOS code signatures before calling `dlopen`.

Do not import `github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx` directly here. That package links `sherpa-onnx-c-api` and `onnxruntime` at build time, which makes Wox depend on those native libraries during process startup. Dictation is optional, so Wox should start without loading or resolving sherpa/onnxruntime.

The flow is:

1. `speech` downloads the platform archive and signed manifest from the Wox dependency release on first use.
2. The manager verifies the manifest signature, archive digest, exact library allowlist, individual file digests, and platform signature requirements.
3. Verified files are atomically installed under `.wox/runtime/onnxruntime/<version>/<goos>-<goarch>` and `.wox/runtime/sherpa-onnx/<version>/<goos>-<goarch>`, with the signed manifest cached beside the sherpa-onnx files. The Silero VAD model is stored under `.wox/models/dictation/silero-vad`. Existing caches without valid metadata are replaced.
4. `speech` calls `LoadLibraryExW` on Windows or `dlopen` on macOS/Linux the first time a recognizer or VAD is created.
5. Models are still owned by the recognizer/VAD pools. Lazy dictation loading evicts idle recognizers; eager loading keeps the selected recognizer resident until model switch or plugin unload.

Native libraries are intentionally not unloaded. ONNX Runtime and sherpa keep process-wide state; unloading and reloading them is riskier than keeping the library images mapped after first use.
