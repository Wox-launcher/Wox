package speech

/*
#cgo windows CFLAGS: -DWIN32_LEAN_AND_MEAN
#cgo windows LDFLAGS: -lkernel32
#cgo linux LDFLAGS: -ldl
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#if defined(_WIN32)
  #include <windows.h>
#else
  #include <dlfcn.h>
#endif
#include "sherpa_onnx_c_api.h"

static void *wox_sherpa_modules[8];
static int wox_sherpa_module_count;
static void *wox_sherpa_c_api_module;

typedef const SherpaOnnxOnlineRecognizer *(*SherpaOnnxCreateOnlineRecognizerFn)(const SherpaOnnxOnlineRecognizerConfig *);
typedef void (*SherpaOnnxDestroyOnlineRecognizerFn)(const SherpaOnnxOnlineRecognizer *);
typedef const SherpaOnnxOnlineStream *(*SherpaOnnxCreateOnlineStreamFn)(const SherpaOnnxOnlineRecognizer *);
typedef void (*SherpaOnnxDestroyOnlineStreamFn)(const SherpaOnnxOnlineStream *);
typedef void (*SherpaOnnxOnlineStreamAcceptWaveformFn)(const SherpaOnnxOnlineStream *, int32_t, const float *, int32_t);
typedef int32_t (*SherpaOnnxIsOnlineStreamReadyFn)(const SherpaOnnxOnlineRecognizer *, const SherpaOnnxOnlineStream *);
typedef void (*SherpaOnnxDecodeOnlineStreamFn)(const SherpaOnnxOnlineRecognizer *, const SherpaOnnxOnlineStream *);
typedef const SherpaOnnxOnlineRecognizerResult *(*SherpaOnnxGetOnlineStreamResultFn)(const SherpaOnnxOnlineRecognizer *, const SherpaOnnxOnlineStream *);
typedef void (*SherpaOnnxDestroyOnlineRecognizerResultFn)(const SherpaOnnxOnlineRecognizerResult *);
typedef void (*SherpaOnnxOnlineStreamResetFn)(const SherpaOnnxOnlineRecognizer *, const SherpaOnnxOnlineStream *);
typedef int32_t (*SherpaOnnxOnlineStreamIsEndpointFn)(const SherpaOnnxOnlineRecognizer *, const SherpaOnnxOnlineStream *);

typedef const SherpaOnnxOfflineRecognizer *(*SherpaOnnxCreateOfflineRecognizerFn)(const SherpaOnnxOfflineRecognizerConfig *);
typedef void (*SherpaOnnxDestroyOfflineRecognizerFn)(const SherpaOnnxOfflineRecognizer *);
typedef const SherpaOnnxOfflineStream *(*SherpaOnnxCreateOfflineStreamFn)(const SherpaOnnxOfflineRecognizer *);
typedef void (*SherpaOnnxDestroyOfflineStreamFn)(const SherpaOnnxOfflineStream *);
typedef void (*SherpaOnnxAcceptWaveformOfflineFn)(const SherpaOnnxOfflineStream *, int32_t, const float *, int32_t);
typedef void (*SherpaOnnxDecodeOfflineStreamFn)(const SherpaOnnxOfflineRecognizer *, const SherpaOnnxOfflineStream *);
typedef const SherpaOnnxOfflineRecognizerResult *(*SherpaOnnxGetOfflineStreamResultFn)(const SherpaOnnxOfflineStream *);
typedef void (*SherpaOnnxDestroyOfflineRecognizerResultFn)(const SherpaOnnxOfflineRecognizerResult *);

typedef const SherpaOnnxVoiceActivityDetector *(*SherpaOnnxCreateVoiceActivityDetectorFn)(const SherpaOnnxVadModelConfig *, float);
typedef void (*SherpaOnnxDestroyVoiceActivityDetectorFn)(const SherpaOnnxVoiceActivityDetector *);
typedef void (*SherpaOnnxVoiceActivityDetectorAcceptWaveformFn)(const SherpaOnnxVoiceActivityDetector *, const float *, int32_t);
typedef int32_t (*SherpaOnnxVoiceActivityDetectorEmptyFn)(const SherpaOnnxVoiceActivityDetector *);
typedef int32_t (*SherpaOnnxVoiceActivityDetectorDetectedFn)(const SherpaOnnxVoiceActivityDetector *);
typedef void (*SherpaOnnxVoiceActivityDetectorPopFn)(const SherpaOnnxVoiceActivityDetector *);
typedef void (*SherpaOnnxVoiceActivityDetectorClearFn)(const SherpaOnnxVoiceActivityDetector *);
typedef const SherpaOnnxSpeechSegment *(*SherpaOnnxVoiceActivityDetectorFrontFn)(const SherpaOnnxVoiceActivityDetector *);
typedef void (*SherpaOnnxDestroySpeechSegmentFn)(const SherpaOnnxSpeechSegment *);
typedef void (*SherpaOnnxVoiceActivityDetectorFlushFn)(const SherpaOnnxVoiceActivityDetector *);

static SherpaOnnxCreateOnlineRecognizerFn p_SherpaOnnxCreateOnlineRecognizer;
static SherpaOnnxDestroyOnlineRecognizerFn p_SherpaOnnxDestroyOnlineRecognizer;
static SherpaOnnxCreateOnlineStreamFn p_SherpaOnnxCreateOnlineStream;
static SherpaOnnxDestroyOnlineStreamFn p_SherpaOnnxDestroyOnlineStream;
static SherpaOnnxOnlineStreamAcceptWaveformFn p_SherpaOnnxOnlineStreamAcceptWaveform;
static SherpaOnnxIsOnlineStreamReadyFn p_SherpaOnnxIsOnlineStreamReady;
static SherpaOnnxDecodeOnlineStreamFn p_SherpaOnnxDecodeOnlineStream;
static SherpaOnnxGetOnlineStreamResultFn p_SherpaOnnxGetOnlineStreamResult;
static SherpaOnnxDestroyOnlineRecognizerResultFn p_SherpaOnnxDestroyOnlineRecognizerResult;
static SherpaOnnxOnlineStreamResetFn p_SherpaOnnxOnlineStreamReset;
static SherpaOnnxOnlineStreamIsEndpointFn p_SherpaOnnxOnlineStreamIsEndpoint;

static SherpaOnnxCreateOfflineRecognizerFn p_SherpaOnnxCreateOfflineRecognizer;
static SherpaOnnxDestroyOfflineRecognizerFn p_SherpaOnnxDestroyOfflineRecognizer;
static SherpaOnnxCreateOfflineStreamFn p_SherpaOnnxCreateOfflineStream;
static SherpaOnnxDestroyOfflineStreamFn p_SherpaOnnxDestroyOfflineStream;
static SherpaOnnxAcceptWaveformOfflineFn p_SherpaOnnxAcceptWaveformOffline;
static SherpaOnnxDecodeOfflineStreamFn p_SherpaOnnxDecodeOfflineStream;
static SherpaOnnxGetOfflineStreamResultFn p_SherpaOnnxGetOfflineStreamResult;
static SherpaOnnxDestroyOfflineRecognizerResultFn p_SherpaOnnxDestroyOfflineRecognizerResult;

static SherpaOnnxCreateVoiceActivityDetectorFn p_SherpaOnnxCreateVoiceActivityDetector;
static SherpaOnnxDestroyVoiceActivityDetectorFn p_SherpaOnnxDestroyVoiceActivityDetector;
static SherpaOnnxVoiceActivityDetectorAcceptWaveformFn p_SherpaOnnxVoiceActivityDetectorAcceptWaveform;
static SherpaOnnxVoiceActivityDetectorEmptyFn p_SherpaOnnxVoiceActivityDetectorEmpty;
static SherpaOnnxVoiceActivityDetectorDetectedFn p_SherpaOnnxVoiceActivityDetectorDetected;
static SherpaOnnxVoiceActivityDetectorPopFn p_SherpaOnnxVoiceActivityDetectorPop;
static SherpaOnnxVoiceActivityDetectorClearFn p_SherpaOnnxVoiceActivityDetectorClear;
static SherpaOnnxVoiceActivityDetectorFrontFn p_SherpaOnnxVoiceActivityDetectorFront;
static SherpaOnnxDestroySpeechSegmentFn p_SherpaOnnxDestroySpeechSegment;
static SherpaOnnxVoiceActivityDetectorFlushFn p_SherpaOnnxVoiceActivityDetectorFlush;

static void wox_sherpa_set_error(char *err, int err_len, const char *message) {
  if (err == NULL || err_len <= 0) {
    return;
  }
  snprintf(err, (size_t)err_len, "%s", message);
}

#if defined(_WIN32)
static void *wox_sherpa_load_native_library(const char *path, char *err, int err_len) {
  wchar_t wide_path[4096];
  int written = MultiByteToWideChar(CP_UTF8, 0, path, -1, wide_path, (int)(sizeof(wide_path) / sizeof(wide_path[0])));
  if (written <= 0) {
    wox_sherpa_set_error(err, err_len, "MultiByteToWideChar failed");
    return NULL;
  }

  HMODULE module = LoadLibraryExW(wide_path, NULL, LOAD_WITH_ALTERED_SEARCH_PATH);
  if (module == NULL) {
    char message[256];
    snprintf(message, sizeof(message), "LoadLibraryExW failed for %s (win32=%lu)", path, (unsigned long)GetLastError());
    wox_sherpa_set_error(err, err_len, message);
  }
  return (void *)module;
}

static void *wox_sherpa_load_symbol(const char *name, char *err, int err_len) {
  FARPROC proc = GetProcAddress((HMODULE)wox_sherpa_c_api_module, name);
  if (proc == NULL) {
    char message[256];
    snprintf(message, sizeof(message), "%s (win32=%lu)", name, (unsigned long)GetLastError());
    wox_sherpa_set_error(err, err_len, message);
  }
  return (void *)proc;
}
#else
static void *wox_sherpa_load_native_library(const char *path, char *err, int err_len) {
  dlerror();
  void *module = dlopen(path, RTLD_NOW | RTLD_GLOBAL);
  if (module == NULL) {
    const char *dl_error = dlerror();
    char message[1024];
    snprintf(message, sizeof(message), "dlopen failed for %s: %s", path, dl_error == NULL ? "unknown" : dl_error);
    wox_sherpa_set_error(err, err_len, message);
  }
  return module;
}

static void *wox_sherpa_load_symbol(const char *name, char *err, int err_len) {
  dlerror();
  void *proc = dlsym(wox_sherpa_c_api_module, name);
  const char *dl_error = dlerror();
  if (dl_error != NULL) {
    char message[1024];
    snprintf(message, sizeof(message), "%s: %s", name, dl_error);
    wox_sherpa_set_error(err, err_len, message);
    return NULL;
  }
  return proc;
}
#endif

#define WOX_LOAD_SHERPA_SYMBOL(name) \
  p_##name = (name##Fn)wox_sherpa_load_symbol(#name, err, err_len); \
  if (p_##name == NULL) { \
    return 0; \
  }

static int wox_sherpa_load_symbols(char *err, int err_len) {
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxCreateOnlineRecognizer)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroyOnlineRecognizer)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxCreateOnlineStream)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroyOnlineStream)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxOnlineStreamAcceptWaveform)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxIsOnlineStreamReady)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDecodeOnlineStream)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxGetOnlineStreamResult)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroyOnlineRecognizerResult)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxOnlineStreamReset)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxOnlineStreamIsEndpoint)

  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxCreateOfflineRecognizer)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroyOfflineRecognizer)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxCreateOfflineStream)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroyOfflineStream)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxAcceptWaveformOffline)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDecodeOfflineStream)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxGetOfflineStreamResult)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroyOfflineRecognizerResult)

  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxCreateVoiceActivityDetector)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroyVoiceActivityDetector)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxVoiceActivityDetectorAcceptWaveform)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxVoiceActivityDetectorEmpty)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxVoiceActivityDetectorDetected)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxVoiceActivityDetectorPop)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxVoiceActivityDetectorClear)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxVoiceActivityDetectorFront)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxDestroySpeechSegment)
  WOX_LOAD_SHERPA_SYMBOL(SherpaOnnxVoiceActivityDetectorFlush)

  return 1;
}

static int wox_sherpa_load_library(const char *path, int is_c_api, char *err, int err_len) {
  if (is_c_api && wox_sherpa_c_api_module != NULL) {
    return 1;
  }
  if (wox_sherpa_module_count >= (int)(sizeof(wox_sherpa_modules) / sizeof(wox_sherpa_modules[0]))) {
    wox_sherpa_set_error(err, err_len, "too many sherpa native libraries");
    return 0;
  }

  void *module = wox_sherpa_load_native_library(path, err, err_len);
  if (module == NULL) {
    return 0;
  }
  wox_sherpa_modules[wox_sherpa_module_count++] = module;

  if (!is_c_api) {
    return 1;
  }
  wox_sherpa_c_api_module = module;
  return wox_sherpa_load_symbols(err, err_len);
}

static const SherpaOnnxOnlineRecognizer *wox_sherpa_create_online_recognizer(const SherpaOnnxOnlineRecognizerConfig *config) {
  return p_SherpaOnnxCreateOnlineRecognizer(config);
}

static void wox_sherpa_destroy_online_recognizer(const SherpaOnnxOnlineRecognizer *recognizer) {
  p_SherpaOnnxDestroyOnlineRecognizer(recognizer);
}

static const SherpaOnnxOnlineStream *wox_sherpa_create_online_stream(const SherpaOnnxOnlineRecognizer *recognizer) {
  return p_SherpaOnnxCreateOnlineStream(recognizer);
}

static void wox_sherpa_destroy_online_stream(const SherpaOnnxOnlineStream *stream) {
  p_SherpaOnnxDestroyOnlineStream(stream);
}

static void wox_sherpa_online_stream_accept_waveform(const SherpaOnnxOnlineStream *stream, int32_t sample_rate, const float *samples, int32_t n) {
  p_SherpaOnnxOnlineStreamAcceptWaveform(stream, sample_rate, samples, n);
}

static int32_t wox_sherpa_is_online_stream_ready(const SherpaOnnxOnlineRecognizer *recognizer, const SherpaOnnxOnlineStream *stream) {
  return p_SherpaOnnxIsOnlineStreamReady(recognizer, stream);
}

static void wox_sherpa_decode_online_stream(const SherpaOnnxOnlineRecognizer *recognizer, const SherpaOnnxOnlineStream *stream) {
  p_SherpaOnnxDecodeOnlineStream(recognizer, stream);
}

static const SherpaOnnxOnlineRecognizerResult *wox_sherpa_get_online_stream_result(const SherpaOnnxOnlineRecognizer *recognizer, const SherpaOnnxOnlineStream *stream) {
  return p_SherpaOnnxGetOnlineStreamResult(recognizer, stream);
}

static void wox_sherpa_destroy_online_recognizer_result(const SherpaOnnxOnlineRecognizerResult *result) {
  p_SherpaOnnxDestroyOnlineRecognizerResult(result);
}

static void wox_sherpa_online_stream_reset(const SherpaOnnxOnlineRecognizer *recognizer, const SherpaOnnxOnlineStream *stream) {
  p_SherpaOnnxOnlineStreamReset(recognizer, stream);
}

static int32_t wox_sherpa_online_stream_is_endpoint(const SherpaOnnxOnlineRecognizer *recognizer, const SherpaOnnxOnlineStream *stream) {
  return p_SherpaOnnxOnlineStreamIsEndpoint(recognizer, stream);
}

static const SherpaOnnxOfflineRecognizer *wox_sherpa_create_offline_recognizer(const SherpaOnnxOfflineRecognizerConfig *config) {
  return p_SherpaOnnxCreateOfflineRecognizer(config);
}

static void wox_sherpa_destroy_offline_recognizer(const SherpaOnnxOfflineRecognizer *recognizer) {
  p_SherpaOnnxDestroyOfflineRecognizer(recognizer);
}

static const SherpaOnnxOfflineStream *wox_sherpa_create_offline_stream(const SherpaOnnxOfflineRecognizer *recognizer) {
  return p_SherpaOnnxCreateOfflineStream(recognizer);
}

static void wox_sherpa_destroy_offline_stream(const SherpaOnnxOfflineStream *stream) {
  p_SherpaOnnxDestroyOfflineStream(stream);
}

static void wox_sherpa_accept_waveform_offline(const SherpaOnnxOfflineStream *stream, int32_t sample_rate, const float *samples, int32_t n) {
  p_SherpaOnnxAcceptWaveformOffline(stream, sample_rate, samples, n);
}

static void wox_sherpa_decode_offline_stream(const SherpaOnnxOfflineRecognizer *recognizer, const SherpaOnnxOfflineStream *stream) {
  p_SherpaOnnxDecodeOfflineStream(recognizer, stream);
}

static const SherpaOnnxOfflineRecognizerResult *wox_sherpa_get_offline_stream_result(const SherpaOnnxOfflineStream *stream) {
  return p_SherpaOnnxGetOfflineStreamResult(stream);
}

static void wox_sherpa_destroy_offline_recognizer_result(const SherpaOnnxOfflineRecognizerResult *result) {
  p_SherpaOnnxDestroyOfflineRecognizerResult(result);
}

static const SherpaOnnxVoiceActivityDetector *wox_sherpa_create_vad(const SherpaOnnxVadModelConfig *config, float buffer_size_in_seconds) {
  return p_SherpaOnnxCreateVoiceActivityDetector(config, buffer_size_in_seconds);
}

static void wox_sherpa_destroy_vad(const SherpaOnnxVoiceActivityDetector *vad) {
  p_SherpaOnnxDestroyVoiceActivityDetector(vad);
}

static void wox_sherpa_vad_accept_waveform(const SherpaOnnxVoiceActivityDetector *vad, const float *samples, int32_t n) {
  p_SherpaOnnxVoiceActivityDetectorAcceptWaveform(vad, samples, n);
}

static int32_t wox_sherpa_vad_empty(const SherpaOnnxVoiceActivityDetector *vad) {
  return p_SherpaOnnxVoiceActivityDetectorEmpty(vad);
}

static int32_t wox_sherpa_vad_detected(const SherpaOnnxVoiceActivityDetector *vad) {
  return p_SherpaOnnxVoiceActivityDetectorDetected(vad);
}

static void wox_sherpa_vad_pop(const SherpaOnnxVoiceActivityDetector *vad) {
  p_SherpaOnnxVoiceActivityDetectorPop(vad);
}

static void wox_sherpa_vad_clear(const SherpaOnnxVoiceActivityDetector *vad) {
  p_SherpaOnnxVoiceActivityDetectorClear(vad);
}

static const SherpaOnnxSpeechSegment *wox_sherpa_vad_front(const SherpaOnnxVoiceActivityDetector *vad) {
  return p_SherpaOnnxVoiceActivityDetectorFront(vad);
}

static void wox_sherpa_destroy_speech_segment(const SherpaOnnxSpeechSegment *segment) {
  p_SherpaOnnxDestroySpeechSegment(segment);
}

static void wox_sherpa_vad_flush(const SherpaOnnxVoiceActivityDetector *vad) {
  p_SherpaOnnxVoiceActivityDetectorFlush(vad);
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unsafe"
	"wox/resource"
	"wox/util"
)

var (
	sherpaLoadOnce sync.Once
	sherpaLoadErr  error
)

type cStringScope struct {
	values []*C.char
}

func (s *cStringScope) add(value string) *C.char {
	if value == "" {
		return nil
	}
	cValue := C.CString(value)
	s.values = append(s.values, cValue)
	return cValue
}

func (s *cStringScope) free() {
	for _, value := range s.values {
		C.free(unsafe.Pointer(value))
	}
	s.values = nil
}

func ensureSherpaLoaded() error {
	sherpaLoadOnce.Do(func() {
		sherpaLoadErr = loadSherpaLibraries()
	})
	return sherpaLoadErr
}

func loadSherpaLibraries() error {
	libraryDir, err := ensureSherpaResourceLibraries()
	if err != nil {
		return err
	}

	for _, name := range sherpaLibraryNames() {
		libraryPath := filepath.Join(libraryDir, name)
		cPath := C.CString(libraryPath)
		errBuf := make([]C.char, 1024)
		isCAPI := C.int(0)
		if name == sherpaCAPILibraryName() {
			isCAPI = 1
		}
		ok := C.wox_sherpa_load_library(cPath, isCAPI, &errBuf[0], C.int(len(errBuf)))
		C.free(unsafe.Pointer(cPath))
		if ok == 0 {
			return fmt.Errorf("load sherpa native library %s: %s", libraryPath, C.GoString(&errBuf[0]))
		}
	}
	return nil
}

func ensureSherpaResourceLibraries() (string, error) {
	resourceDir := filepath.Join(runtime.GOOS, runtime.GOARCH)
	libraryDir := resource.GetDictationResourcePath(filepath.ToSlash(resourceDir))

	for _, name := range sherpaLibraryNames() {
		targetPath := filepath.Join(libraryDir, name)
		if util.IsFileExists(targetPath) {
			continue
		}

		data, err := resource.GetDictationNativeFile(filepath.ToSlash(filepath.Join(resourceDir, name)))
		if err != nil {
			return "", fmt.Errorf("read embedded sherpa native library %s: %w", name, err)
		}
		if err := util.GetLocation().EnsureDirectoryExist(filepath.Dir(targetPath)); err != nil {
			return "", err
		}
		if err := os.WriteFile(targetPath, data, 0644); err != nil {
			return "", fmt.Errorf("write sherpa native library %s: %w", targetPath, err)
		}
	}

	return libraryDir, nil
}

func sherpaLibraryNames() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"onnxruntime.dll", "sherpa-onnx-cxx-api.dll", "sherpa-onnx-c-api.dll"}
	case "darwin":
		return []string{"libonnxruntime.1.24.4.dylib", "libsherpa-onnx-cxx-api.dylib", "libsherpa-onnx-c-api.dylib"}
	default:
		return []string{"libonnxruntime.so", "libsherpa-onnx-cxx-api.so", "libsherpa-onnx-c-api.so"}
	}
}

func sherpaCAPILibraryName() string {
	switch runtime.GOOS {
	case "windows":
		return "sherpa-onnx-c-api.dll"
	case "darwin":
		return "libsherpa-onnx-c-api.dylib"
	default:
		return "libsherpa-onnx-c-api.so"
	}
}

// ---------------------------------------------------------------------------
// Streaming (online) recognizer — zipformer2 / paraformer
// ---------------------------------------------------------------------------

type sherpaOnlineRecognizer struct {
	config     RecognizerConfig
	recognizer *C.struct_SherpaOnnxOnlineRecognizer
	stream     *C.struct_SherpaOnnxOnlineStream
}

func newOnlineRecognizer(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	if err := ensureSherpaLoaded(); err != nil {
		return nil, err
	}

	t0 := time.Now()
	logger := util.GetLogger()

	cStrings := &cStringScope{}
	defer cStrings.free()

	c := C.struct_SherpaOnnxOnlineRecognizerConfig{}
	c.feat_config.sample_rate = C.int(16000)
	c.feat_config.feature_dim = C.int(80)
	c.model_config.num_threads = C.int(config.NumThreads)
	c.model_config.debug = C.int(0)
	c.model_config.provider = cStrings.add("cpu")
	c.model_config.model_type = cStrings.add(config.ModelType)
	c.decoding_method = cStrings.add("greedy_search")
	c.max_active_paths = C.int(4)
	c.enable_endpoint = C.int(1)
	c.rule1_min_trailing_silence = C.float(2.4)
	c.rule2_min_trailing_silence = C.float(1.2)
	c.rule3_min_utterance_length = C.float(20)

	switch config.ModelType {
	case "zipformer2":
		encoder, err := findModelFile(config.ModelPath, "encoder*.onnx")
		if err != nil {
			return nil, fmt.Errorf("encoder model not found in %s: %w", config.ModelPath, err)
		}
		decoder, err := findModelFile(config.ModelPath, "decoder*.onnx")
		if err != nil {
			return nil, fmt.Errorf("decoder model not found in %s: %w", config.ModelPath, err)
		}
		joiner, err := findModelFile(config.ModelPath, "joiner*.onnx")
		if err != nil {
			return nil, fmt.Errorf("joiner model not found in %s: %w", config.ModelPath, err)
		}
		c.model_config.transducer.encoder = cStrings.add(encoder)
		c.model_config.transducer.decoder = cStrings.add(decoder)
		c.model_config.transducer.joiner = cStrings.add(joiner)
	case "paraformer":
		c.model_config.paraformer.encoder = cStrings.add(filepath.Join(config.ModelPath, "encoder.int8.onnx"))
		c.model_config.paraformer.decoder = cStrings.add(filepath.Join(config.ModelPath, "decoder.int8.onnx"))
	default:
		return nil, fmt.Errorf("unsupported streaming model type: %s", config.ModelType)
	}
	c.model_config.tokens = cStrings.add(filepath.Join(config.ModelPath, "tokens.txt"))

	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.findModelFile cost=%dms", time.Since(t0).Milliseconds()))

	recognizer := C.wox_sherpa_create_online_recognizer(&c)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create sherpa recognizer (model path: %s)", config.ModelPath)
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.NewOnlineRecognizer cost=%dms", time.Since(t0).Milliseconds()))

	stream := C.wox_sherpa_create_online_stream(recognizer)
	if stream == nil {
		C.wox_sherpa_destroy_online_recognizer(recognizer)
		return nil, fmt.Errorf("failed to create sherpa stream")
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.total cost=%dms", time.Since(t0).Milliseconds()))

	return &sherpaOnlineRecognizer{
		config:     config,
		recognizer: recognizer,
		stream:     stream,
	}, nil
}

func (r *sherpaOnlineRecognizer) IsStreaming() bool { return true }

func (r *sherpaOnlineRecognizer) AcceptWaveform(sampleRate int, samples []float32) {
	if len(samples) == 0 {
		return
	}
	C.wox_sherpa_online_stream_accept_waveform(r.stream, C.int(sampleRate), (*C.float)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
}

func (r *sherpaOnlineRecognizer) GetResult() PartialResult {
	result := C.wox_sherpa_get_online_stream_result(r.recognizer, r.stream)
	if result == nil {
		return PartialResult{}
	}
	defer C.wox_sherpa_destroy_online_recognizer_result(result)
	return PartialResult{Text: C.GoString(result.text)}
}

func (r *sherpaOnlineRecognizer) IsReady() bool {
	return C.wox_sherpa_is_online_stream_ready(r.recognizer, r.stream) == 1
}

func (r *sherpaOnlineRecognizer) Decode() {
	C.wox_sherpa_decode_online_stream(r.recognizer, r.stream)
}

func (r *sherpaOnlineRecognizer) IsEndpoint() bool {
	return C.wox_sherpa_online_stream_is_endpoint(r.recognizer, r.stream) == 1
}

func (r *sherpaOnlineRecognizer) Reset() {
	C.wox_sherpa_online_stream_reset(r.recognizer, r.stream)
}

func (r *sherpaOnlineRecognizer) DecodeSamples(samples []float32) string {
	return ""
}

func (r *sherpaOnlineRecognizer) Close() {
	if r.stream != nil {
		C.wox_sherpa_destroy_online_stream(r.stream)
		r.stream = nil
	}
	if r.recognizer != nil {
		C.wox_sherpa_destroy_online_recognizer(r.recognizer)
		r.recognizer = nil
	}
}

// ---------------------------------------------------------------------------
// Offline recognizer — Qwen3-ASR / SenseVoice
// ---------------------------------------------------------------------------

type sherpaOfflineRecognizer struct {
	config     RecognizerConfig
	recognizer *C.struct_SherpaOnnxOfflineRecognizer
}

func newOfflineRecognizer(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	if err := ensureSherpaLoaded(); err != nil {
		return nil, err
	}

	t0 := time.Now()
	logger := util.GetLogger()

	cStrings := &cStringScope{}
	defer cStrings.free()

	c := C.struct_SherpaOnnxOfflineRecognizerConfig{}
	c.feat_config.sample_rate = C.int(16000)
	c.feat_config.feature_dim = C.int(80)
	c.model_config.num_threads = C.int(config.NumThreads)
	c.model_config.debug = C.int(0)
	c.model_config.provider = cStrings.add("cpu")
	c.model_config.model_type = cStrings.add(config.ModelType)
	c.decoding_method = cStrings.add("greedy_search")
	c.max_active_paths = C.int(4)

	switch config.ModelType {
	case "qwen3_asr":
		c.model_config.qwen3_asr.conv_frontend = cStrings.add(filepath.Join(config.ModelPath, "conv_frontend.onnx"))
		c.model_config.qwen3_asr.encoder = cStrings.add(filepath.Join(config.ModelPath, "encoder.int8.onnx"))
		c.model_config.qwen3_asr.decoder = cStrings.add(filepath.Join(config.ModelPath, "decoder.int8.onnx"))
		c.model_config.qwen3_asr.tokenizer = cStrings.add(filepath.Join(config.ModelPath, "tokenizer"))
		c.model_config.qwen3_asr.seed = C.int(42)
		c.model_config.tokens = nil
	case "sense_voice":
		model, err := findModelFile(config.ModelPath, "model*.onnx")
		if err != nil {
			return nil, fmt.Errorf("SenseVoice model not found in %s: %w", config.ModelPath, err)
		}
		language := config.Language
		if language == "" {
			language = "auto"
		}
		c.model_config.sense_voice.model = cStrings.add(model)
		c.model_config.sense_voice.language = cStrings.add(language)
		c.model_config.sense_voice.use_itn = C.int(1)
		c.model_config.tokens = cStrings.add(filepath.Join(config.ModelPath, "tokens.txt"))
	default:
		return nil, fmt.Errorf("unsupported offline model type: %s", config.ModelType)
	}

	recognizer := C.wox_sherpa_create_offline_recognizer(&c)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create offline recognizer (model path: %s)", config.ModelPath)
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.NewOfflineRecognizer cost=%dms", time.Since(t0).Milliseconds()))

	rec := &sherpaOfflineRecognizer{
		config:     config,
		recognizer: recognizer,
	}

	// Warm up ONNX Runtime before the first real segment reaches Stop(), where
	// short dictations otherwise pay the first inference cost after key release.
	warmupT0 := time.Now()
	_ = rec.DecodeSamples(make([]float32, 8000)) // 0.5s silence at 16kHz.
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.warmup cost=%dms", time.Since(warmupT0).Milliseconds()))
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.total cost=%dms", time.Since(t0).Milliseconds()))

	return rec, nil
}

func (r *sherpaOfflineRecognizer) IsStreaming() bool { return false }

func (r *sherpaOfflineRecognizer) AcceptWaveform(sampleRate int, samples []float32) {}

func (r *sherpaOfflineRecognizer) GetResult() PartialResult { return PartialResult{} }

func (r *sherpaOfflineRecognizer) IsReady() bool { return false }

func (r *sherpaOfflineRecognizer) Decode() {}

func (r *sherpaOfflineRecognizer) IsEndpoint() bool { return false }

func (r *sherpaOfflineRecognizer) Reset() {}

func (r *sherpaOfflineRecognizer) DecodeSamples(samples []float32) string {
	if len(samples) == 0 {
		return ""
	}

	stream := C.wox_sherpa_create_offline_stream(r.recognizer)
	if stream == nil {
		return ""
	}
	defer C.wox_sherpa_destroy_offline_stream(stream)

	C.wox_sherpa_accept_waveform_offline(stream, C.int(16000), (*C.float)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
	C.wox_sherpa_decode_offline_stream(r.recognizer, stream)

	result := C.wox_sherpa_get_offline_stream_result(stream)
	if result == nil {
		return ""
	}
	defer C.wox_sherpa_destroy_offline_recognizer_result(result)
	return C.GoString(result.text)
}

func (r *sherpaOfflineRecognizer) Close() {
	if r.recognizer != nil {
		C.wox_sherpa_destroy_offline_recognizer(r.recognizer)
		r.recognizer = nil
	}
}

// ---------------------------------------------------------------------------
// VAD
// ---------------------------------------------------------------------------

// VoiceActivityDetector wraps sherpa-onnx VAD for real-time speech
// segmentation. Audio samples are fed in continuously; the detector splits
// them into complete speech segments separated by silence.
type VoiceActivityDetector struct {
	config VadConfig
	vad    *C.struct_SherpaOnnxVoiceActivityDetector
}

// NewVoiceActivityDetector creates a VAD from the given config.
func NewVoiceActivityDetector(ctx context.Context, config VadConfig) (*VoiceActivityDetector, error) {
	if err := ensureSherpaLoaded(); err != nil {
		return nil, err
	}

	t0 := time.Now()
	logger := util.GetLogger()

	cStrings := &cStringScope{}
	defer cStrings.free()

	c := C.struct_SherpaOnnxVadModelConfig{}
	c.silero_vad.model = cStrings.add(config.ModelPath)
	c.silero_vad.threshold = C.float(config.Threshold)
	c.silero_vad.min_silence_duration = C.float(config.MinSilenceDuration)
	c.silero_vad.min_speech_duration = C.float(config.MinSpeechDuration)
	c.silero_vad.window_size = C.int(config.WindowSize)
	c.silero_vad.max_speech_duration = C.float(config.MaxSpeechDuration)
	c.sample_rate = C.int(16000)
	c.num_threads = C.int(config.NumThreads)
	c.provider = cStrings.add("cpu")
	c.debug = C.int(0)

	vad := C.wox_sherpa_create_vad(&c, C.float(20.0))
	if vad == nil {
		return nil, fmt.Errorf("failed to create VAD (model: %s)", config.ModelPath)
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: vad.NewVoiceActivityDetector cost=%dms", time.Since(t0).Milliseconds()))

	return &VoiceActivityDetector{
		config: config,
		vad:    vad,
	}, nil
}

// AcceptWaveform feeds 16kHz mono float32 PCM samples into the VAD.
func (v *VoiceActivityDetector) AcceptWaveform(samples []float32) {
	if len(samples) == 0 {
		return
	}
	C.wox_sherpa_vad_accept_waveform(v.vad, (*C.float)(unsafe.Pointer(&samples[0])), C.int(len(samples)))
}

// IsSpeech reports whether speech is currently being detected.
func (v *VoiceActivityDetector) IsSpeech() bool {
	return C.wox_sherpa_vad_detected(v.vad) == 1
}

// IsEmpty reports whether there are no completed speech segments available.
func (v *VoiceActivityDetector) IsEmpty() bool {
	return C.wox_sherpa_vad_empty(v.vad) == 1
}

// Front returns the first completed speech segment without removing it.
func (v *VoiceActivityDetector) Front() *SpeechSegment {
	front := C.wox_sherpa_vad_front(v.vad)
	if front == nil {
		return nil
	}
	defer C.wox_sherpa_destroy_speech_segment(front)

	segment := &SpeechSegment{
		Start: int(front.start),
	}
	n := int(front.n)
	if n <= 0 || front.samples == nil {
		return segment
	}

	samples := unsafe.Slice(front.samples, n)
	segment.Samples = make([]float32, n)
	for i := range samples {
		segment.Samples[i] = float32(samples[i])
	}
	return segment
}

// Pop removes the first completed speech segment.
func (v *VoiceActivityDetector) Pop() {
	C.wox_sherpa_vad_pop(v.vad)
}

// Flush forces any buffered audio at the end of a session into a final segment.
func (v *VoiceActivityDetector) Flush() {
	C.wox_sherpa_vad_flush(v.vad)
}

// Clear resets the VAD internal buffer and state.
func (v *VoiceActivityDetector) Clear() {
	C.wox_sherpa_vad_clear(v.vad)
}

// Close releases the VAD resources. Must be called exactly once.
func (v *VoiceActivityDetector) Close() {
	if v.vad != nil {
		C.wox_sherpa_destroy_vad(v.vad)
		v.vad = nil
	}
}
