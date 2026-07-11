// sherpa-onnx/c-api/c-api.h
//
// Copyright (c)  2023  Xiaomi Corporation
/**
 * @file c-api.h
 * @brief Public C API for sherpa-onnx.
 *
 * This header exposes the main sherpa-onnx inference features through a stable
 * C interface. It is intended for native C/C++ applications and for language
 * bindings that need a C ABI.
 *
 * The file is organized by feature family. The major API groups are:
 *
 * - Utility helpers: version/build information, file checks, WAVE I/O, and a
 *   display helper for incremental text output
 * - Streaming ASR: online recognizers, online streams, endpointing, and
 *   per-stream runtime options
 * - Non-streaming ASR: offline recognizers, offline streams, batch decode, and
 *   result retrieval
 * - Keyword spotting: streaming keyword detection, custom keyword streams, and
 *   keyword result snapshots
 * - Voice activity detection: Silero/Ten VAD models, speech segment buffers,
 *   and detector state management
 * - Text-to-speech: offline TTS model families, generation configuration, and
 *   generated audio helpers
 * - Spoken language identification
 * - Speaker embedding extraction and speaker enrollment/search/verification
 * - Audio tagging
 * - Offline and online punctuation restoration
 * - Linear resampling
 * - Offline speaker diarization
 * - Offline and online speech enhancement / denoising
 * - HarmonyOS-specific constructor variants
 *
 * Common ownership rules:
 *
 * - Opaque handles created by `SherpaOnnxCreate*()` functions are generally
 *   destroyed with a matching `SherpaOnnxDestroy*()` function
 * - Snapshot/result objects returned by query functions usually need explicit
 *   destruction as documented on each API
 * - Strings or arrays returned by helper/query functions are either:
 *   - statically owned by the library and must not be freed, or
 *   - heap-allocated for the caller and must be released with the matching
 *     `Free`/`Destroy` API
 *
 * General usage pattern:
 *
 * 1. Zero-initialize a config struct with `memset(&config, 0, sizeof(config))`
 * 2. Fill in the required model paths and runtime options
 * 3. Create the corresponding engine with `SherpaOnnxCreate*()`
 * 4. Create a stream if the feature uses one
 * 5. Feed audio or text, run the compute/decode API, and retrieve results
 * 6. Release every returned object with the documented matching API
 *
 * The examples in `c-api-examples/` show complete end-to-end usage. Useful
 * starting points include:
 *
 * - `decode-file-c-api.c` for ASR
 * - `kws-c-api.c` for keyword spotting
 * - `vad-whisper-c-api.c` for VAD
 * - `offline-tts-c-api.c` and `kokoro-tts-en-c-api.c` for TTS
 * - `speaker-identification-c-api.c` for speaker embedding and verification
 * - `audio-tagging-c-api.c` for audio tagging
 * - `add-punctuation-c-api.c` and `add-punctuation-online-c-api.c` for
 *   punctuation
 * - `offline-sepaker-diarization-c-api.c` for diarization
 * - `speech-enhancement-gtcrn-c-api.c` and
 *   `online-speech-enhancement-gtcrn-c-api.c` for speech enhancement
 */

#ifndef SHERPA_ONNX_C_API_C_API_H_
#define SHERPA_ONNX_C_API_C_API_H_

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

// See https://github.com/pytorch/pytorch/blob/main/c10/macros/Export.h
// We will set SHERPA_ONNX_BUILD_SHARED_LIBS and SHERPA_ONNX_BUILD_MAIN_LIB in
// CMakeLists.txt

#if defined(__GNUC__)
#pragma GCC diagnostic push
#pragma GCC diagnostic ignored "-Wattributes"
#endif

#if defined(_WIN32)
#if defined(SHERPA_ONNX_BUILD_SHARED_LIBS)
#define SHERPA_ONNX_EXPORT __declspec(dllexport)
#define SHERPA_ONNX_IMPORT __declspec(dllimport)
#else
#define SHERPA_ONNX_EXPORT
#define SHERPA_ONNX_IMPORT
#endif
#else  // WIN32
#define SHERPA_ONNX_EXPORT __attribute__((visibility("default")))

#define SHERPA_ONNX_IMPORT SHERPA_ONNX_EXPORT
#endif  // WIN32

#if defined(SHERPA_ONNX_BUILD_MAIN_LIB)
#define SHERPA_ONNX_API SHERPA_ONNX_EXPORT
#else
#define SHERPA_ONNX_API SHERPA_ONNX_IMPORT
#endif

#ifndef SHERPA_ONNX_DEPRECATED
#if defined(_MSC_VER)
#define SHERPA_ONNX_DEPRECATED(msg) __declspec(deprecated(msg))
#elif defined(__GNUC__) || defined(__clang__)
#define SHERPA_ONNX_DEPRECATED(msg) __attribute__((deprecated(msg)))
#else
#define SHERPA_ONNX_DEPRECATED(msg)
#endif
#endif

/**
 * @brief Return the sherpa-onnx version string.
 *
 * The returned pointer refers to statically allocated memory owned by the
 * library. Do not free it and do not modify it.
 *
 * @return Version string, for example `"1.12.1"`.
 *
 * @code
 * printf("sherpa-onnx version: %s\n", SherpaOnnxGetVersionStr());
 * @endcode
 */
SHERPA_ONNX_API const char *SherpaOnnxGetVersionStr();

/**
 * @brief Return the Git SHA1 used to build the library.
 *
 * The returned pointer refers to statically allocated memory owned by the
 * library. Do not free it and do not modify it.
 *
 * @return Short Git SHA1 string, for example `"6982b86c"`.
 */
SHERPA_ONNX_API const char *SherpaOnnxGetGitSha1();

/**
 * @brief Return the Git build date used to build the library.
 *
 * The returned pointer refers to statically allocated memory owned by the
 * library. Do not free it and do not modify it.
 *
 * @return Build date string, for example `"Fri Jun 20 11:22:52 2025"`.
 */
SHERPA_ONNX_API const char *SherpaOnnxGetGitDate();

/**
 * @brief Check whether a file exists.
 *
 * @param filename File path to test.
 * @return 1 if the file exists; otherwise 0.
 *
 * @code
 * if (!SherpaOnnxFileExists("./Obama.wav")) {
 *   fprintf(stderr, "Please download Obama.wav\n");
 * }
 * @endcode
 */
SHERPA_ONNX_API int32_t SherpaOnnxFileExists(const char *filename);

/**
 * @brief Configuration for a streaming transducer model.
 *
 * Please refer to
 * https://k2-fsa.github.io/sherpa/onnx/pretrained_models/index.html
 * to download compatible pre-trained models.
 */
typedef struct SherpaOnnxOnlineTransducerModelConfig {
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model. */
  const char *decoder;
  /** Path to the joiner ONNX model. */
  const char *joiner;
} SherpaOnnxOnlineTransducerModelConfig;

/**
 * @brief Configuration for a streaming Paraformer model.
 *
 * Please visit
 * https://k2-fsa.github.io/sherpa/onnx/pretrained_models/online-paraformer/index.html
 * to download compatible models.
 */
typedef struct SherpaOnnxOnlineParaformerModelConfig {
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model. */
  const char *decoder;
} SherpaOnnxOnlineParaformerModelConfig;

/**
 * @brief Configuration for a streaming Zipformer2 CTC model.
 */
typedef struct SherpaOnnxOnlineZipformer2CtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOnlineZipformer2CtcModelConfig;

/** @brief Configuration for a streaming NeMo CTC model. */
typedef struct SherpaOnnxOnlineNemoCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOnlineNemoCtcModelConfig;

/** @brief Configuration for a streaming T-One CTC model. */
typedef struct SherpaOnnxOnlineToneCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOnlineToneCtcModelConfig;

/**
 * @brief Model configuration shared by streaming ASR recognizers.
 *
 * Zero-initialize this struct before use, then fill in the sub-config for the
 * model family you want to use together with the shared fields such as
 * @c tokens, @c provider, and @c num_threads.
 *
 * Exactly one model family should be configured for each recognizer. For
 * example, set only one of @c transducer, @c paraformer, @c zipformer2_ctc,
 * @c nemo_ctc, or @c t_one_ctc.
 *
 * If multiple model families are configured at the same time, the
 * implementation will choose one of them, and which one is used is
 * implementation-defined. Do not rely on any precedence rule.
 */
typedef struct SherpaOnnxOnlineModelConfig {
  /** Streaming transducer model files. */
  SherpaOnnxOnlineTransducerModelConfig transducer;
  /** Streaming Paraformer model files. */
  SherpaOnnxOnlineParaformerModelConfig paraformer;
  /** Streaming Zipformer2 CTC model files. */
  SherpaOnnxOnlineZipformer2CtcModelConfig zipformer2_ctc;
  /** Path to the tokens file. */
  const char *tokens;
  /** Number of threads used by the ONNX Runtime backend. */
  int32_t num_threads;
  /** Execution provider, for example "cpu", "cuda", or "coreml". */
  const char *provider;
  /** Non-zero to print model debug information. */
  int32_t debug;
  /** Optional explicit model type override. */
  const char *model_type;
  /**
   * Modeling unit used by the tokens.
   *
   * Valid values include:
   * - "cjkchar"
   * - "bpe"
   * - "cjkchar+bpe"
   */
  const char *modeling_unit;
  /** Path to the BPE vocabulary file when BPE is used. */
  const char *bpe_vocab;
  /** Optional in-memory tokens data. Used instead of @c tokens when non-NULL.
   */
  const char *tokens_buf;
  /** Size in bytes of @c tokens_buf, excluding the trailing '\0'. */
  int32_t tokens_buf_size;
  /** Streaming NeMo CTC model files. */
  SherpaOnnxOnlineNemoCtcModelConfig nemo_ctc;
  /** Streaming T-One CTC model files. */
  SherpaOnnxOnlineToneCtcModelConfig t_one_ctc;
} SherpaOnnxOnlineModelConfig;

/**
 * @brief Feature extraction settings for ASR.
 *
 * The bundled ASR models typically expect 16 kHz mono audio and 80-bin
 * features.
 */
typedef struct SherpaOnnxFeatureConfig {
  /** Sample rate expected by the model, for example 16000. */
  int32_t sample_rate;

  /** Feature dimension expected by the model, for example 80. */
  int32_t feature_dim;
} SherpaOnnxFeatureConfig;

/** @brief Configuration for HLG/FST-based online CTC decoding. */
typedef struct SherpaOnnxOnlineCtcFstDecoderConfig {
  /** Path to the decoding graph. */
  const char *graph;
  /** Decoder max-active setting. */
  int32_t max_active;
} SherpaOnnxOnlineCtcFstDecoderConfig;

/** @brief Configuration for homophone replacement. */
typedef struct SherpaOnnxHomophoneReplacerConfig {
  /** Unused legacy field kept for ABI compatibility. */
  const char *dict_dir;
  /** Path to the lexicon used by the homophone replacer. */
  const char *lexicon;
  /** Path to the replacement rule FST file. */
  const char *rule_fsts;
} SherpaOnnxHomophoneReplacerConfig;

/**
 * @brief Configuration for a streaming ASR recognizer.
 *
 * Zero-initialize this struct before use. Then fill in @c feat_config,
 * @c model_config, and any optional decoding, endpoint, or hotword settings.
 *
 * Example model package:
 * `sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20`
 *
 * @code
 * SherpaOnnxOnlineRecognizerConfig config;
 * memset(&config, 0, sizeof(config));
 *
 * config.feat_config.sample_rate = 16000;
 * config.feat_config.feature_dim = 80;
 *
 * config.model_config.transducer.encoder =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "encoder-epoch-99-avg-1.int8.onnx";
 * config.model_config.transducer.decoder =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "decoder-epoch-99-avg-1.onnx";
 * config.model_config.transducer.joiner =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "joiner-epoch-99-avg-1.int8.onnx";
 * config.model_config.tokens =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "tokens.txt";
 * config.model_config.provider = "cpu";
 * config.model_config.num_threads = 1;
 *
 * config.decoding_method = "greedy_search";
 * @endcode
 * @see SherpaOnnxCreateOnlineRecognizer
 */
typedef struct SherpaOnnxOnlineRecognizerConfig {
  /** Feature extraction settings. */
  SherpaOnnxFeatureConfig feat_config;
  /** Streaming model configuration. */
  SherpaOnnxOnlineModelConfig model_config;

  /** Decoding method, for example "greedy_search" or "modified_beam_search". */
  const char *decoding_method;

  /** Number of active paths for modified beam search. */
  int32_t max_active_paths;

  /** Set to non-zero to enable endpoint detection. */
  int32_t enable_endpoint;

  /** Endpoint rule 1 trailing silence threshold in seconds. */
  float rule1_min_trailing_silence;

  /** Endpoint rule 2 trailing silence threshold in seconds. */
  float rule2_min_trailing_silence;

  /** Endpoint rule 3 utterance-length threshold in seconds. */
  float rule3_min_utterance_length;

  /** Path to a hotwords file. */
  const char *hotwords_file;

  /** Bonus score added to each hotword token during decoding. */
  float hotwords_score;

  /** Optional HLG/FST online CTC decoder configuration. */
  SherpaOnnxOnlineCtcFstDecoderConfig ctc_fst_decoder_config;
  /** Path to punctuation or text-processing rule FSTs. */
  const char *rule_fsts;
  /** Path to FAR archives used by text-processing rules. */
  const char *rule_fars;
  /** Optional blank penalty applied during decoding. */
  float blank_penalty;

  /** Optional in-memory hotwords text used instead of @c hotwords_file. */
  const char *hotwords_buf;
  /** Size in bytes of @c hotwords_buf, excluding the trailing '\0'. */
  int32_t hotwords_buf_size;
  /** Optional homophone replacement configuration. */
  SherpaOnnxHomophoneReplacerConfig hr;
} SherpaOnnxOnlineRecognizerConfig;

/**
 * @brief Incremental recognition result for a streaming ASR stream.
 *
 * All pointers in this struct are owned by the result object returned from
 * SherpaOnnxGetOnlineStreamResult() and become invalid after
 * SherpaOnnxDestroyOnlineRecognizerResult() is called.
 * @see SherpaOnnxGetOnlineStreamResult
 */
typedef struct SherpaOnnxOnlineRecognizerResult {
  /** Recognized text accumulated so far. */
  const char *text;

  /**
   * Contiguous memory block containing token strings separated by '\0'.
   *
   * Use @c tokens_arr for convenient indexed access.
   */
  const char *tokens;

  /** Array of @c count pointers into @c tokens. */
  const char *const *tokens_arr;

  /**
   * Optional token timestamps in seconds.
   *
   * This field may be NULL when the model does not provide timestamps.
   * When non-NULL, it contains @c count entries and is parallel to
   * @c tokens_arr.
   */
  float *timestamps;

  /** Number of entries in @c tokens_arr and, when available, @c timestamps. */
  int32_t count;

  /** JSON serialization of the result. */
  const char *json;
} SherpaOnnxOnlineRecognizerResult;

/** @brief Streaming recognizer handle. */
typedef struct SherpaOnnxOnlineRecognizer SherpaOnnxOnlineRecognizer;
/** @brief Streaming decoding state for one utterance or stream. */
typedef struct SherpaOnnxOnlineStream SherpaOnnxOnlineStream;

/**
 * @brief Create a streaming ASR recognizer.
 *
 * The returned recognizer runs locally and does not require Internet access.
 *
 * @param config Recognizer configuration.
 * @return A recognizer handle on success, or NULL if the configuration is
 *         invalid. The caller owns the returned object and must free it with
 *         SherpaOnnxDestroyOnlineRecognizer().
 *
 * @code
 * SherpaOnnxOnlineRecognizerConfig config;
 * memset(&config, 0, sizeof(config));
 * config.feat_config.sample_rate = 16000;
 * config.feat_config.feature_dim = 80;
 * config.model_config.transducer.encoder =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "encoder-epoch-99-avg-1.int8.onnx";
 * config.model_config.transducer.decoder =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "decoder-epoch-99-avg-1.onnx";
 * config.model_config.transducer.joiner =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "joiner-epoch-99-avg-1.int8.onnx";
 * config.model_config.tokens =
 *     "./sherpa-onnx-streaming-zipformer-bilingual-zh-en-2023-02-20/"
 *     "tokens.txt";
 * config.model_config.provider = "cpu";
 * config.model_config.num_threads = 1;
 * config.decoding_method = "greedy_search";
 *
 * const SherpaOnnxOnlineRecognizer *recognizer =
 *     SherpaOnnxCreateOnlineRecognizer(&config);
 * @endcode
 * @see SherpaOnnxOnlineRecognizerConfig, SherpaOnnxDestroyOnlineRecognizer
 */
SHERPA_ONNX_API const SherpaOnnxOnlineRecognizer *
SherpaOnnxCreateOnlineRecognizer(
    const SherpaOnnxOnlineRecognizerConfig *config);

/**
 * @brief Destroy a streaming recognizer.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 *
 * @code
 * SherpaOnnxDestroyOnlineRecognizer(recognizer);
 * recognizer = NULL;
 * @endcode
 * @see SherpaOnnxCreateOnlineRecognizer
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOnlineRecognizer(
    const SherpaOnnxOnlineRecognizer *recognizer);

/**
 * @brief Create a streaming ASR state object.
 *
 * One stream corresponds to one decoding state. Reuse the same recognizer to
 * create multiple streams.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @return A newly created stream. The caller owns the returned object and must
 *         free it with SherpaOnnxDestroyOnlineStream().
 *
 * @code
 * const SherpaOnnxWave *wave = SherpaOnnxReadWave(
 *     "./sherpa-onnx-streaming-paraformer-bilingual-zh-en/test_wavs/0.wav");
 * const SherpaOnnxOnlineStream *stream =
 *     SherpaOnnxCreateOnlineStream(recognizer);
 * @endcode
 * @see SherpaOnnxDestroyOnlineStream
 */
SHERPA_ONNX_API const SherpaOnnxOnlineStream *SherpaOnnxCreateOnlineStream(
    const SherpaOnnxOnlineRecognizer *recognizer);

/**
 * @brief Create a streaming ASR state object with per-stream hotwords.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param hotwords Hotwords text to associate with the stream.
 * @return A newly created stream. The caller owns the returned object and must
 *         free it with SherpaOnnxDestroyOnlineStream().
 *
 * @code
 * const SherpaOnnxOnlineStream *stream =
 *     SherpaOnnxCreateOnlineStreamWithHotwords(recognizer,
 *                                              "▁HELLO ▁WORLD");
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxOnlineStream *
SherpaOnnxCreateOnlineStreamWithHotwords(
    const SherpaOnnxOnlineRecognizer *recognizer, const char *hotwords);

/**
 * @brief Destroy a streaming ASR state object.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream() or
 *               SherpaOnnxCreateOnlineStreamWithHotwords().
 *
 * @code
 * SherpaOnnxDestroyOnlineStream(stream);
 * stream = NULL;
 * @endcode
 * @see SherpaOnnxCreateOnlineStream
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOnlineStream(
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Append audio samples to a streaming ASR stream.
 *
 * The input is mono floating-point PCM normalized to the range [-1, 1].
 * If @p sample_rate differs from the recognizer feature sample rate,
 * sherpa-onnx resamples internally.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @param sample_rate Sample rate of @p samples.
 * @param samples Pointer to @p n samples in the range [-1, 1].
 * @param n Number of samples.
 *
 * @code
 * int32_t start = 0;
 * int32_t chunk_size = 3200;  // 0.2 seconds at 16 kHz
 * SherpaOnnxOnlineStreamAcceptWaveform(stream, wave->sample_rate,
 *                                      wave->samples + start, chunk_size);
 * @endcode
 * @see SherpaOnnxCreateOnlineStream, SherpaOnnxDecodeOnlineStream
 */
SHERPA_ONNX_API void SherpaOnnxOnlineStreamAcceptWaveform(
    const SherpaOnnxOnlineStream *stream, int32_t sample_rate,
    const float *samples, int32_t n);

/**
 * @brief Check whether a streaming ASR stream is ready to decode.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @return 1 if enough frames are available for decoding; otherwise 0.
 *
 * @code
 * if (SherpaOnnxIsOnlineStreamReady(recognizer, stream)) {
 *   SherpaOnnxDecodeOnlineStream(recognizer, stream);
 * }
 * @endcode
 */
SHERPA_ONNX_API int32_t
SherpaOnnxIsOnlineStreamReady(const SherpaOnnxOnlineRecognizer *recognizer,
                              const SherpaOnnxOnlineStream *stream);

/**
 * @brief Decode one step of a streaming ASR stream.
 *
 * Call this only when SherpaOnnxIsOnlineStreamReady() returns 1.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 *
 * @code
 * SherpaOnnxOnlineStreamAcceptWaveform(stream, sample_rate, samples, n);
 * while (SherpaOnnxIsOnlineStreamReady(recognizer, stream)) {
 *   SherpaOnnxDecodeOnlineStream(recognizer, stream);
 * }
 * @endcode
 * @see SherpaOnnxIsOnlineStreamReady, SherpaOnnxGetOnlineStreamResult
 */
SHERPA_ONNX_API void SherpaOnnxDecodeOnlineStream(
    const SherpaOnnxOnlineRecognizer *recognizer,
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Decode multiple streaming ASR streams in parallel.
 *
 * The caller must ensure every stream in @p streams is ready before calling
 * this function.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param streams Array of @p n stream pointers.
 * @param n Number of streams in @p streams.
 *
 * @code
 * const SherpaOnnxOnlineStream *streams[2] = {stream1, stream2};
 * SherpaOnnxDecodeMultipleOnlineStreams(recognizer, streams, 2);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxDecodeMultipleOnlineStreams(
    const SherpaOnnxOnlineRecognizer *recognizer,
    const SherpaOnnxOnlineStream **streams, int32_t n);

/**
 * @brief Get the current streaming ASR result for a stream.
 *
 * The returned snapshot is independent from the stream state. The caller owns
 * it and must free it with SherpaOnnxDestroyOnlineRecognizerResult().
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @return A newly allocated result snapshot.
 *
 * @code
 * const SherpaOnnxOnlineRecognizerResult *r =
 *     SherpaOnnxGetOnlineStreamResult(recognizer, stream);
 * printf("%s\n", r->text);
 * // r->tokens_arr[i] and r->timestamps[i] are parallel when timestamps
 * // are available.
 * SherpaOnnxDestroyOnlineRecognizerResult(r);
 * @endcode
 * @see SherpaOnnxDestroyOnlineRecognizerResult
 */
SHERPA_ONNX_API const SherpaOnnxOnlineRecognizerResult *
SherpaOnnxGetOnlineStreamResult(const SherpaOnnxOnlineRecognizer *recognizer,
                                const SherpaOnnxOnlineStream *stream);

/**
 * @brief Destroy a result returned by SherpaOnnxGetOnlineStreamResult().
 *
 * @param r A pointer returned by SherpaOnnxGetOnlineStreamResult().
 *
 * @code
 * SherpaOnnxDestroyOnlineRecognizerResult(r);
 * r = NULL;
 * @endcode
 * @see SherpaOnnxGetOnlineStreamResult
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOnlineRecognizerResult(
    const SherpaOnnxOnlineRecognizerResult *r);

/**
 * @brief Get the current streaming ASR result as JSON.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @return A newly allocated JSON string. Free it with
 *         SherpaOnnxDestroyOnlineStreamResultJson().
 *
 * @code
 * const char *json =
 *     SherpaOnnxGetOnlineStreamResultAsJson(recognizer, stream);
 * puts(json);
 * SherpaOnnxDestroyOnlineStreamResultJson(json);
 * @endcode
 */
SHERPA_ONNX_API const char *SherpaOnnxGetOnlineStreamResultAsJson(
    const SherpaOnnxOnlineRecognizer *recognizer,
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Free a JSON string returned by
 * SherpaOnnxGetOnlineStreamResultAsJson().
 *
 * @param s A pointer returned by SherpaOnnxGetOnlineStreamResultAsJson().
 *
 * @code
 * SherpaOnnxDestroyOnlineStreamResultJson(json);
 * json = NULL;
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOnlineStreamResultJson(const char *s);

/**
 * @brief Reset a streaming ASR stream after an endpoint or utterance boundary.
 *
 * This clears the decoder state for the stream so that it can be reused for a
 * new utterance.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 *
 * @code
 * if (SherpaOnnxOnlineStreamIsEndpoint(recognizer, stream)) {
 *   SherpaOnnxOnlineStreamReset(recognizer, stream);
 * }
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxOnlineStreamReset(
    const SherpaOnnxOnlineRecognizer *recognizer,
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Signal end-of-input for a streaming ASR stream.
 *
 * After calling this function, do not append more samples to the stream.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 *
 * @code
 * SherpaOnnxOnlineStreamInputFinished(stream);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxOnlineStreamInputFinished(
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Set a per-stream runtime option.
 *
 * This is a generic extension point for model-specific or runtime-specific
 * options such as "is_final" for streaming Paraformer.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @param key Option name.
 * @param value Option value represented as text.
 *
 * @code
 * SherpaOnnxOnlineStreamSetOption(stream, "is_final", "1");
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxOnlineStreamSetOption(
    const SherpaOnnxOnlineStream *stream, const char *key, const char *value);

/**
 * @brief Get a per-stream runtime option.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @param key Option name.
 * @return The option value. The returned pointer is owned by the stream, must
 *         not be freed by the caller, and may be invalidated if the option is
 *         overwritten or the stream is destroyed.
 *
 * @code
 * const char *value = SherpaOnnxOnlineStreamGetOption(stream, "is_final");
 * @endcode
 */
SHERPA_ONNX_API const char *SherpaOnnxOnlineStreamGetOption(
    const SherpaOnnxOnlineStream *stream, const char *key);

/**
 * @brief Check whether a per-stream runtime option exists.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @param key Option name.
 * @return 1 if the option exists; otherwise 0.
 *
 * @code
 * int32_t has_option = SherpaOnnxOnlineStreamHasOption(stream, "is_final");
 * @endcode
 */
SHERPA_ONNX_API int32_t SherpaOnnxOnlineStreamHasOption(
    const SherpaOnnxOnlineStream *stream, const char *key);

/**
 * @brief Check whether endpoint detection has triggered for a stream.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOnlineRecognizer().
 * @param stream A pointer returned by SherpaOnnxCreateOnlineStream().
 * @return 1 if an endpoint is detected; otherwise 0.
 *
 * @code
 * if (SherpaOnnxOnlineStreamIsEndpoint(recognizer, stream)) {
 *   SherpaOnnxOnlineStreamReset(recognizer, stream);
 * }
 * @endcode
 */
SHERPA_ONNX_API int32_t
SherpaOnnxOnlineStreamIsEndpoint(const SherpaOnnxOnlineRecognizer *recognizer,
                                 const SherpaOnnxOnlineStream *stream);

/**
 * @brief Helper for pretty-printing incremental recognition results.
 *
 * This utility is mainly used by example programs on Linux and macOS.
 */
typedef struct SherpaOnnxDisplay SherpaOnnxDisplay;

/**
 * @brief Create a display helper.
 *
 * @param max_word_per_line Maximum number of words to show per line.
 * @return A newly allocated display helper. Free it with
 *         SherpaOnnxDestroyDisplay().
 *
 * @code
 * const SherpaOnnxDisplay *display = SherpaOnnxCreateDisplay(50);
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxDisplay *SherpaOnnxCreateDisplay(
    int32_t max_word_per_line);

/**
 * @brief Destroy a display helper.
 *
 * @param display A pointer returned by SherpaOnnxCreateDisplay().
 */
SHERPA_ONNX_API void SherpaOnnxDestroyDisplay(const SherpaOnnxDisplay *display);

/**
 * @brief Print one line of text using the display helper.
 *
 * @param display A pointer returned by SherpaOnnxCreateDisplay().
 * @param idx Segment or utterance index to print.
 * @param s Text to print.
 *
 * @code
 * SherpaOnnxPrint(display, segment_id, r->text);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxPrint(const SherpaOnnxDisplay *display,
                                     int32_t idx, const char *s);
// ============================================================
// For offline ASR (i.e., non-streaming ASR)
// ============================================================

/**
 * @brief Configuration for a non-streaming transducer model.
 */
typedef struct SherpaOnnxOfflineTransducerModelConfig {
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model. */
  const char *decoder;
  /** Path to the joiner ONNX model. */
  const char *joiner;
} SherpaOnnxOfflineTransducerModelConfig;

/** @brief Configuration for a non-streaming Paraformer model. */
typedef struct SherpaOnnxOfflineParaformerModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineParaformerModelConfig;

/** @brief Configuration for a non-streaming NeMo CTC model. */
typedef struct SherpaOnnxOfflineNemoEncDecCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineNemoEncDecCtcModelConfig;

/**
 * @brief Configuration for a non-streaming Whisper model.
 */
typedef struct SherpaOnnxOfflineWhisperModelConfig {
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model. */
  const char *decoder;
  /** Optional language hint, for example "en" or "zh". */
  const char *language;
  /** Optional Whisper task such as "transcribe" or "translate". */
  const char *task;
  /** Number of tail padding frames appended internally. */
  int32_t tail_paddings;

  /** Non-zero to enable token-level timestamps when supported by the model. */
  int32_t enable_token_timestamps;

  /** Non-zero to enable Whisper segment-level timestamps. */
  int32_t enable_segment_timestamps;
} SherpaOnnxOfflineWhisperModelConfig;

/** @brief Configuration for a Canary model. */
typedef struct SherpaOnnxOfflineCanaryModelConfig {
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model. */
  const char *decoder;
  /** Source language hint. */
  const char *src_lang;
  /** Target language hint. */
  const char *tgt_lang;
  /** Non-zero to enable punctuation and capitalization when supported. */
  int32_t use_pnc;
} SherpaOnnxOfflineCanaryModelConfig;

/** @brief Configuration for a Cohere Transcribe model. */
typedef struct SherpaOnnxOfflineCohereTranscribeModelConfig {
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model. */
  const char *decoder;
  /** Optional language hint, for example "en" or "zh". */
  const char *language;
  /** Non-zero to enable punctuation. */
  int32_t use_punct;
  /** Non-zero to enable inverse text normalization. */
  int32_t use_itn;
} SherpaOnnxOfflineCohereTranscribeModelConfig;

/** @brief Configuration for a FireRedAsr encoder/decoder model. */
typedef struct SherpaOnnxOfflineFireRedAsrModelConfig {
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model. */
  const char *decoder;
} SherpaOnnxOfflineFireRedAsrModelConfig;

/** @brief Configuration for a FireRedAsr CTC model. */
typedef struct SherpaOnnxOfflineFireRedAsrCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineFireRedAsrCtcModelConfig;

/** @brief Configuration for a Moonshine model. */
typedef struct SherpaOnnxOfflineMoonshineModelConfig {
  /** Path to the preprocessor ONNX model. */
  const char *preprocessor;
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the uncached decoder ONNX model. */
  const char *uncached_decoder;
  /** Path to the cached decoder ONNX model. */
  const char *cached_decoder;
  /** Path to the merged decoder ONNX model. */
  const char *merged_decoder;
} SherpaOnnxOfflineMoonshineModelConfig;

/** @brief Configuration for a TDNN model. */
typedef struct SherpaOnnxOfflineTdnnModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineTdnnModelConfig;

/** @brief Configuration for an offline language model. */
typedef struct SherpaOnnxOfflineLMConfig {
  /** Path to the language model. */
  const char *model;
  /** Interpolation scale for the language model. */
  float scale;
} SherpaOnnxOfflineLMConfig;

/** @brief Configuration for a SenseVoice model. */
typedef struct SherpaOnnxOfflineSenseVoiceModelConfig {
  /** Path to the ONNX model. */
  const char *model;
  /** Optional language hint. */
  const char *language;
  /** Non-zero to enable inverse text normalization. */
  int32_t use_itn;
} SherpaOnnxOfflineSenseVoiceModelConfig;

/** @brief Configuration for a Dolphin model. */
typedef struct SherpaOnnxOfflineDolphinModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineDolphinModelConfig;

/** @brief Configuration for an offline Zipformer CTC model. */
typedef struct SherpaOnnxOfflineZipformerCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineZipformerCtcModelConfig;

/** @brief Configuration for an offline WeNet CTC model. */
typedef struct SherpaOnnxOfflineWenetCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineWenetCtcModelConfig;

/** @brief Configuration for an omnilingual offline CTC model. */
typedef struct SherpaOnnxOfflineOmnilingualAsrCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineOmnilingualAsrCtcModelConfig;

/** @brief Configuration for an offline FunASR Nano model. */
typedef struct SherpaOnnxOfflineFunASRNanoModelConfig {
  /** Path to the encoder adaptor. */
  const char *encoder_adaptor;
  /** Path to the LLM ONNX model. */
  const char *llm;
  /** Path to the embedding model. */
  const char *embedding;
  /** Path to the tokenizer file. */
  const char *tokenizer;
  /** System prompt. */
  const char *system_prompt;
  /** User prompt. */
  const char *user_prompt;
  /** Maximum number of generated tokens. */
  int32_t max_new_tokens;
  /** Sampling temperature. */
  float temperature;
  /** Top-p sampling threshold. */
  float top_p;
  /** Random seed. */
  int32_t seed;
  /** Optional language hint. */
  const char *language;
  /** Non-zero to enable inverse text normalization. */
  int32_t itn;
  /** Optional hotwords text. */
  const char *hotwords;
} SherpaOnnxOfflineFunASRNanoModelConfig;

/** @brief Configuration for an offline Qwen3-ASR model. */
typedef struct SherpaOnnxOfflineQwen3ASRModelConfig {
  /** Path to the conv-frontend ONNX model. */
  const char *conv_frontend;
  /** Path to the encoder ONNX model. */
  const char *encoder;
  /** Path to the decoder ONNX model (with KV cache). */
  const char *decoder;
  /** Path to the tokenizer directory (e.g. containing `vocab.json`). */
  const char *tokenizer;
  /** Maximum total sequence length supported by the model. */
  int32_t max_total_len;
  /** Maximum number of new tokens to generate. */
  int32_t max_new_tokens;
  /** Sampling temperature. */
  float temperature;
  /** Top-p (nucleus) sampling threshold. */
  float top_p;
  /** Random seed for reproducible sampling. */
  int32_t seed;
  /** Optional comma-separated hotwords (UTF-8, ASCII ','), e.g. @c
   * "foo,bar,baz". */
  const char *hotwords;
} SherpaOnnxOfflineQwen3ASRModelConfig;

/** @brief Configuration for a MedASR CTC model. */
typedef struct SherpaOnnxOfflineMedAsrCtcModelConfig {
  /** Path to the ONNX model. */
  const char *model;
} SherpaOnnxOfflineMedAsrCtcModelConfig;

/**
 * @brief Model configuration shared by offline ASR recognizers.
 *
 * Zero-initialize this struct before use, then fill in exactly the sub-config
 * needed by the model family you want to run.
 *
 * Exactly one model family should be configured for each recognizer. For
 * example, set only one of @c transducer, @c paraformer, @c nemo_ctc,
 * @c whisper, @c tdnn, @c sense_voice, @c moonshine, @c fire_red_asr,
 * @c dolphin, @c zipformer_ctc, @c canary, @c cohere_transcribe,
 * @c wenet_ctc, @c omnilingual, @c medasr, @c funasr_nano,
 * @c fire_red_asr_ctc, or @c qwen3_asr.
 *
 * If multiple model families are configured at the same time, the
 * implementation will choose one of them, and which one is used is
 * implementation-defined. Do not rely on any precedence rule.
 */
typedef struct SherpaOnnxOfflineModelConfig {
  /** Non-streaming transducer model files. */
  SherpaOnnxOfflineTransducerModelConfig transducer;
  /** Non-streaming Paraformer model files. */
  SherpaOnnxOfflineParaformerModelConfig paraformer;
  /** Non-streaming NeMo CTC model files. */
  SherpaOnnxOfflineNemoEncDecCtcModelConfig nemo_ctc;
  /** Whisper model files and options. */
  SherpaOnnxOfflineWhisperModelConfig whisper;
  /** TDNN model files. */
  SherpaOnnxOfflineTdnnModelConfig tdnn;

  /** Path to the tokens file. */
  const char *tokens;
  /** Number of backend threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider, for example "cpu" or "cuda". */
  const char *provider;
  /** Optional explicit model type override. */
  const char *model_type;
  /** Modeling unit, such as "cjkchar", "bpe", or "cjkchar+bpe". */
  const char *modeling_unit;
  /** Path to the BPE vocabulary file when BPE is used. */
  const char *bpe_vocab;
  /** Path to the TeleSpeech CTC model. */
  const char *telespeech_ctc;
  /** SenseVoice configuration. */
  SherpaOnnxOfflineSenseVoiceModelConfig sense_voice;
  /** Moonshine configuration. */
  SherpaOnnxOfflineMoonshineModelConfig moonshine;
  /** FireRedAsr configuration. */
  SherpaOnnxOfflineFireRedAsrModelConfig fire_red_asr;
  /** Dolphin configuration. */
  SherpaOnnxOfflineDolphinModelConfig dolphin;
  /** Zipformer CTC configuration. */
  SherpaOnnxOfflineZipformerCtcModelConfig zipformer_ctc;
  /** Canary configuration. */
  SherpaOnnxOfflineCanaryModelConfig canary;
  /** WeNet CTC configuration. */
  SherpaOnnxOfflineWenetCtcModelConfig wenet_ctc;
  /** Omnilingual CTC configuration. */
  SherpaOnnxOfflineOmnilingualAsrCtcModelConfig omnilingual;
  /** MedASR configuration. */
  SherpaOnnxOfflineMedAsrCtcModelConfig medasr;
  /** FunASR Nano configuration. */
  SherpaOnnxOfflineFunASRNanoModelConfig funasr_nano;
  /** FireRedAsr CTC configuration. */
  SherpaOnnxOfflineFireRedAsrCtcModelConfig fire_red_asr_ctc;
  /** Qwen3-ASR configuration. */
  SherpaOnnxOfflineQwen3ASRModelConfig qwen3_asr;
  /** Cohere Transcribe configuration. */
  SherpaOnnxOfflineCohereTranscribeModelConfig cohere_transcribe;
} SherpaOnnxOfflineModelConfig;

/**
 * @brief Configuration for a non-streaming ASR recognizer.
 *
 * Zero-initialize this struct before use.
 *
 * Example using Whisper:
 *
 * @code
 * SherpaOnnxOfflineRecognizerConfig config;
 * memset(&config, 0, sizeof(config));
 *
 * config.feat_config.sample_rate = 16000;
 * config.feat_config.feature_dim = 80;
 *
 * config.model_config.whisper.encoder =
 *     "./sherpa-onnx-whisper-tiny/tiny-encoder.onnx";
 * config.model_config.whisper.decoder =
 *     "./sherpa-onnx-whisper-tiny/tiny-decoder.onnx";
 * config.model_config.whisper.language = "en";
 * config.model_config.whisper.task = "transcribe";
 * config.model_config.tokens =
 *     "./sherpa-onnx-whisper-tiny/tiny-tokens.txt";
 * config.model_config.provider = "cpu";
 * config.model_config.num_threads = 1;
 *
 * config.decoding_method = "greedy_search";
 * @endcode
 *
 * Example using SenseVoice:
 *
 * @code
 * config.model_config.sense_voice.model =
 *     "./sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17-int8/model.int8.onnx";
 * config.model_config.sense_voice.language = "auto";
 * config.model_config.sense_voice.use_itn = 1;
 * config.model_config.tokens =
 *     "./sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17-int8/tokens.txt";
 * @endcode
 *
 * Example using Parakeet TDT:
 *
 * @code
 * config.model_config.transducer.encoder =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/encoder.int8.onnx";
 * config.model_config.transducer.decoder =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/decoder.int8.onnx";
 * config.model_config.transducer.joiner =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/joiner.int8.onnx";
 * config.model_config.tokens =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/tokens.txt";
 * config.model_config.model_type = "nemo_transducer";
 * @endcode
 * @see SherpaOnnxCreateOfflineRecognizer
 */
typedef struct SherpaOnnxOfflineRecognizerConfig {
  /** Feature extraction settings. */
  SherpaOnnxFeatureConfig feat_config;
  /** Offline model configuration. */
  SherpaOnnxOfflineModelConfig model_config;
  /** Optional language model configuration. */
  SherpaOnnxOfflineLMConfig lm_config;

  /** Decoding method, for example "greedy_search" or "modified_beam_search". */
  const char *decoding_method;
  /** Number of active paths for modified beam search. */
  int32_t max_active_paths;

  /** Path to a hotwords file. */
  const char *hotwords_file;

  /** Bonus score added to each hotword token. */
  float hotwords_score;
  /** Path to punctuation or text-processing rule FSTs. */
  const char *rule_fsts;
  /** Path to FAR archives used by text-processing rules. */
  const char *rule_fars;
  /** Optional blank penalty applied during decoding. */
  float blank_penalty;

  /** Optional homophone replacement configuration. */
  SherpaOnnxHomophoneReplacerConfig hr;
} SherpaOnnxOfflineRecognizerConfig;

/** @brief Non-streaming recognizer handle. */
typedef struct SherpaOnnxOfflineRecognizer SherpaOnnxOfflineRecognizer;

/** @brief Non-streaming decoding state for one utterance. */
typedef struct SherpaOnnxOfflineStream SherpaOnnxOfflineStream;

/**
 * @brief Create a non-streaming ASR recognizer.
 *
 * @param config Recognizer configuration.
 * @return A recognizer handle on success, or NULL if the configuration is
 *         invalid. The caller owns the returned object and must free it with
 *         SherpaOnnxDestroyOfflineRecognizer().
 *
 * Whisper example:
 *
 * @code
 * SherpaOnnxOfflineRecognizerConfig config;
 * memset(&config, 0, sizeof(config));
 * config.feat_config.sample_rate = 16000;
 * config.feat_config.feature_dim = 80;
 * config.model_config.whisper.encoder =
 *     "./sherpa-onnx-whisper-tiny/tiny-encoder.onnx";
 * config.model_config.whisper.decoder =
 *     "./sherpa-onnx-whisper-tiny/tiny-decoder.onnx";
 * config.model_config.whisper.language = "en";
 * config.model_config.whisper.task = "transcribe";
 * config.model_config.tokens =
 *     "./sherpa-onnx-whisper-tiny/tiny-tokens.txt";
 * config.model_config.provider = "cpu";
 * config.model_config.num_threads = 1;
 * config.decoding_method = "greedy_search";
 *
 * const SherpaOnnxOfflineRecognizer *recognizer =
 *     SherpaOnnxCreateOfflineRecognizer(&config);
 * @endcode
 *
 * SenseVoice example:
 *
 * @code
 * config.model_config.sense_voice.model =
 *     "./sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17-int8/model.int8.onnx";
 * config.model_config.sense_voice.language = "auto";
 * config.model_config.sense_voice.use_itn = 1;
 * config.model_config.tokens =
 *     "./sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17-int8/tokens.txt";
 * @endcode
 *
 * Parakeet TDT example:
 *
 * @code
 * config.model_config.transducer.encoder =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/encoder.int8.onnx";
 * config.model_config.transducer.decoder =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/decoder.int8.onnx";
 * config.model_config.transducer.joiner =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/joiner.int8.onnx";
 * config.model_config.tokens =
 *     "./sherpa-onnx-nemo-parakeet-tdt-0.6b-v3-int8/tokens.txt";
 * config.model_config.model_type = "nemo_transducer";
 * @endcode
 * @see SherpaOnnxOfflineRecognizerConfig, SherpaOnnxDestroyOfflineRecognizer
 */
SHERPA_ONNX_API const SherpaOnnxOfflineRecognizer *
SherpaOnnxCreateOfflineRecognizer(
    const SherpaOnnxOfflineRecognizerConfig *config);

/**
 * @brief Update the configuration of an existing offline recognizer.
 *
 * @param recognizer Recognizer handle.
 * @param config New recognizer configuration.
 *
 * @code
 * SherpaOnnxOfflineRecognizerSetConfig(recognizer, &config);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxOfflineRecognizerSetConfig(
    const SherpaOnnxOfflineRecognizer *recognizer,
    const SherpaOnnxOfflineRecognizerConfig *config);

/**
 * @brief Destroy a non-streaming recognizer.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOfflineRecognizer().
 *
 * @code
 * SherpaOnnxDestroyOfflineRecognizer(recognizer);
 * recognizer = NULL;
 * @endcode
 * @see SherpaOnnxCreateOfflineRecognizer
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineRecognizer(
    const SherpaOnnxOfflineRecognizer *recognizer);

/**
 * @brief Create a non-streaming ASR input stream.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOfflineRecognizer().
 * @return A newly created stream. The caller owns the returned object and must
 *         free it with SherpaOnnxDestroyOfflineStream().
 *
 * @code
 * const SherpaOnnxWave *wave =
 *     SherpaOnnxReadWave("./sherpa-onnx-whisper-tiny.en/test_wavs/0.wav");
 * const SherpaOnnxOfflineStream *stream =
 *     SherpaOnnxCreateOfflineStream(recognizer);
 * @endcode
 * @see SherpaOnnxDestroyOfflineStream, SherpaOnnxAcceptWaveformOffline
 */
SHERPA_ONNX_API const SherpaOnnxOfflineStream *SherpaOnnxCreateOfflineStream(
    const SherpaOnnxOfflineRecognizer *recognizer);

/**
 * @brief Create a non-streaming ASR input stream with per-stream hotwords.
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOfflineRecognizer().
 * @param hotwords Hotwords text to associate with the stream.
 * @return A newly created stream. The caller owns the returned object and must
 *         free it with SherpaOnnxDestroyOfflineStream().
 *
 * @code
 * const SherpaOnnxOfflineStream *stream =
 *     SherpaOnnxCreateOfflineStreamWithHotwords(recognizer,
 *                                               "▁HELLO ▁WORLD");
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxOfflineStream *
SherpaOnnxCreateOfflineStreamWithHotwords(
    const SherpaOnnxOfflineRecognizer *recognizer, const char *hotwords);

/**
 * @brief Destroy a non-streaming ASR stream.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream() or
 *               SherpaOnnxCreateOfflineStreamWithHotwords().
 *
 * @code
 * SherpaOnnxDestroyOfflineStream(stream);
 * stream = NULL;
 * @endcode
 * @see SherpaOnnxCreateOfflineStream
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineStream(
    const SherpaOnnxOfflineStream *stream);

/**
 * @brief Provide the full utterance to an offline ASR stream.
 *
 * The input is mono floating-point PCM normalized to the range [-1, 1].
 * If @p sample_rate differs from the recognizer feature sample rate,
 * sherpa-onnx resamples internally.
 *
 * @warning Call this function at most once for each offline stream. Offline
 * recognition expects the entire utterance in a single call.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream().
 * @param sample_rate Sample rate of @p samples.
 * @param samples Pointer to @p n samples in the range [-1, 1].
 * @param n Number of samples.
 *
 * @code
 * const SherpaOnnxWave *wave =
 *     SherpaOnnxReadWave("./sherpa-onnx-whisper-tiny.en/test_wavs/0.wav");
 * const SherpaOnnxOfflineStream *stream =
 *     SherpaOnnxCreateOfflineStream(recognizer);
 * SherpaOnnxAcceptWaveformOffline(stream, wave->sample_rate,
 *                                 wave->samples, wave->num_samples);
 * SherpaOnnxDecodeOfflineStream(recognizer, stream);
 * @endcode
 * @see SherpaOnnxCreateOfflineStream, SherpaOnnxDecodeOfflineStream
 */
SHERPA_ONNX_API void SherpaOnnxAcceptWaveformOffline(
    const SherpaOnnxOfflineStream *stream, int32_t sample_rate,
    const float *samples, int32_t n);

/**
 * @brief Set a per-stream runtime option for offline ASR.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream().
 * @param key Option name.
 * @param value Option value represented as text.
 *
 * @code
 * SherpaOnnxOfflineStreamSetOption(stream, "language", "en");
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxOfflineStreamSetOption(
    const SherpaOnnxOfflineStream *stream, const char *key, const char *value);

/**
 * @brief Get a per-stream runtime option for offline ASR.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream().
 * @param key Option name.
 * @return The option value. The returned pointer is owned by the stream, must
 *         not be freed by the caller, and may be invalidated if the option is
 *         overwritten or the stream is destroyed.
 *
 * @code
 * const char *value = SherpaOnnxOfflineStreamGetOption(stream, "language");
 * @endcode
 */
SHERPA_ONNX_API const char *SherpaOnnxOfflineStreamGetOption(
    const SherpaOnnxOfflineStream *stream, const char *key);

/**
 * @brief Check whether a per-stream runtime option exists.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream().
 * @param key Option name.
 * @return 1 if the option exists; otherwise 0.
 *
 * @code
 * int32_t has_language =
 *     SherpaOnnxOfflineStreamHasOption(stream, "language");
 * @endcode
 */
SHERPA_ONNX_API int32_t SherpaOnnxOfflineStreamHasOption(
    const SherpaOnnxOfflineStream *stream, const char *key);

/**
 * @brief Run offline ASR on one stream.
 *
 * Call this after SherpaOnnxAcceptWaveformOffline().
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOfflineRecognizer().
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream().
 *
 * @code
 * SherpaOnnxDecodeOfflineStream(recognizer, stream);
 * @endcode
 * @see SherpaOnnxAcceptWaveformOffline, SherpaOnnxGetOfflineStreamResult
 */
SHERPA_ONNX_API void SherpaOnnxDecodeOfflineStream(
    const SherpaOnnxOfflineRecognizer *recognizer,
    const SherpaOnnxOfflineStream *stream);

/**
 * @brief Run offline ASR on multiple streams in parallel.
 *
 * The caller must have already provided one utterance to each stream via
 * SherpaOnnxAcceptWaveformOffline().
 *
 * @param recognizer A pointer returned by SherpaOnnxCreateOfflineRecognizer().
 * @param streams Array of @p n offline stream pointers.
 * @param n Number of streams in @p streams.
 *
 * @code
 * const SherpaOnnxOfflineStream *streams[2] = {stream1, stream2};
 * SherpaOnnxDecodeMultipleOfflineStreams(recognizer, streams, 2);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxDecodeMultipleOfflineStreams(
    const SherpaOnnxOfflineRecognizer *recognizer,
    const SherpaOnnxOfflineStream **streams, int32_t n);

/**
 * @brief Recognition result for a non-streaming ASR stream.
 *
 * All pointers in this struct are owned by the result object returned from
 * SherpaOnnxGetOfflineStreamResult() and become invalid after
 * SherpaOnnxDestroyOfflineRecognizerResult() is called.
 * @see SherpaOnnxGetOfflineStreamResult, SherpaOnnxDestroyOfflineRecognizerResult
 */
typedef struct SherpaOnnxOfflineRecognizerResult {
  /** Recognized text. */
  const char *text;

  /**
   * Optional token timestamps in seconds.
   *
   * This field may be NULL when the model does not provide token timestamps.
   * When non-NULL, it contains @c count entries and is parallel to
   * @c tokens_arr.
   */
  float *timestamps;

  /** Number of token entries in @c tokens_arr and related per-token arrays. */
  int32_t count;

  /**
   * Contiguous memory block containing token strings separated by '\0'.
   *
   * Use @c tokens_arr for convenient indexed access.
   */
  const char *tokens;

  /** Array of @c count pointers into @c tokens. */
  const char *const *tokens_arr;

  /** JSON serialization of the result. */
  const char *json;

  /** Optional recognized language label. */
  const char *lang;

  /** Optional recognized emotion label. */
  const char *emotion;

  /** Optional recognized event label. */
  const char *event;

  /** Optional token durations in seconds, parallel to @c tokens_arr. */
  float *durations;

  /** Optional token log probabilities, parallel to @c tokens_arr. */
  float *ys_log_probs;

  /** Optional segment start times in seconds, parallel to @c segment_texts_arr.
   */
  const float *segment_timestamps;

  /** Optional segment durations in seconds, parallel to @c segment_texts_arr.
   */
  const float *segment_durations;

  /** Contiguous memory block containing segment texts separated by '\0'. */
  const char *segment_texts;

  /** Array of @c segment_count pointers into @c segment_texts. */
  const char *const *segment_texts_arr;

  /** Number of segment entries in the segment-level arrays. */
  int32_t segment_count;
} SherpaOnnxOfflineRecognizerResult;

/**
 * @brief Get the recognition result for an offline ASR stream.
 *
 * Call this after SherpaOnnxDecodeOfflineStream() or
 * SherpaOnnxDecodeMultipleOfflineStreams().
 *
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream().
 * @return A newly allocated result snapshot. Free it with
 *         SherpaOnnxDestroyOfflineRecognizerResult().
 *
 * @code
 * const SherpaOnnxOfflineRecognizerResult *r =
 *     SherpaOnnxGetOfflineStreamResult(stream);
 * printf("%s\n", r->text);
 * if (r->timestamps) {
 *   printf("First token starts at %.3f seconds\n", r->timestamps[0]);
 * }
 * SherpaOnnxDestroyOfflineRecognizerResult(r);
 * @endcode
 * @see SherpaOnnxDestroyOfflineRecognizerResult, SherpaOnnxDecodeOfflineStream
 */
SHERPA_ONNX_API const SherpaOnnxOfflineRecognizerResult *
SherpaOnnxGetOfflineStreamResult(const SherpaOnnxOfflineStream *stream);

/**
 * @brief Destroy a result returned by SherpaOnnxGetOfflineStreamResult().
 *
 * @param r A pointer returned by SherpaOnnxGetOfflineStreamResult().
 *
 * @code
 * SherpaOnnxDestroyOfflineRecognizerResult(r);
 * r = NULL;
 * @endcode
 * @see SherpaOnnxGetOfflineStreamResult
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineRecognizerResult(
    const SherpaOnnxOfflineRecognizerResult *r);

/**
 * @brief Get the offline ASR result as JSON.
 *
 * @param stream A pointer returned by SherpaOnnxCreateOfflineStream().
 * @return A newly allocated JSON string. Free it with
 *         SherpaOnnxDestroyOfflineStreamResultJson().
 *
 * @code
 * const char *json = SherpaOnnxGetOfflineStreamResultAsJson(stream);
 * puts(json);
 * SherpaOnnxDestroyOfflineStreamResultJson(json);
 * @endcode
 */
SHERPA_ONNX_API const char *SherpaOnnxGetOfflineStreamResultAsJson(
    const SherpaOnnxOfflineStream *stream);

/**
 * @brief Free a JSON string returned by
 * SherpaOnnxGetOfflineStreamResultAsJson().
 *
 * @param s A pointer returned by SherpaOnnxGetOfflineStreamResultAsJson().
 *
 * @code
 * SherpaOnnxDestroyOfflineStreamResultJson(json);
 * json = NULL;
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineStreamResultJson(const char *s);

// ============================================================
// For keyword spotting
// ============================================================
/**
 * @brief Snapshot of the current keyword spotting result.
 *
 * Free this object with SherpaOnnxDestroyKeywordResult().
 * @see SherpaOnnxGetKeywordResult
 */
typedef struct SherpaOnnxKeywordResult {
  /**
   * Triggered keyword text.
   *
   * For English models this is usually space-separated words. For Chinese
   * models it is typically the surface form without spaces.
   */
  const char *keyword;

  /**
   * Token sequence as a single string.
   *
   * For BPE-based models this contains the decoded BPE tokens.
   */
  const char *tokens;

  /**
   * Token sequence as an array.
   *
   * The array length is @c count. Each string is owned by this result object.
   */
  const char *const *tokens_arr;

  /** Number of decoded tokens in @c tokens_arr and @c timestamps. */
  int32_t count;

  /**
   * Per-token timestamps in seconds.
   *
   * This array has @c count elements. Element @c i corresponds to
   * `tokens_arr[i]`.
   */
  float *timestamps;

  /** Start time of the current segment in seconds. */
  float start_time;

  /**
   * JSON representation of the result.
   *
   * The JSON includes `keyword`, `tokens`, `timestamps`, and `start_time`.
   */
  const char *json;
} SherpaOnnxKeywordResult;

/**
 * @brief Configuration for keyword spotting.
 *
 * The acoustic model is configured through @c model_config. In practice this is
 * usually a streaming transducer model.
 *
 * Keyword definitions can be provided either through @c keywords_file or
 * through @c keywords_buf/@c keywords_buf_size. If both are set, the buffer is
 * used.
 *
 * Example using
 * `sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01-mobile`:
 *
 * @code
 * SherpaOnnxKeywordSpotterConfig config;
 * memset(&config, 0, sizeof(config));
 *
 * config.model_config.transducer.encoder =
 *     "./sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01-mobile/"
 *     "encoder-epoch-12-avg-2-chunk-16-left-64.int8.onnx";
 * config.model_config.transducer.decoder =
 *     "./sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01-mobile/"
 *     "decoder-epoch-12-avg-2-chunk-16-left-64.onnx";
 * config.model_config.transducer.joiner =
 *     "./sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01-mobile/"
 *     "joiner-epoch-12-avg-2-chunk-16-left-64.int8.onnx";
 * config.model_config.tokens =
 *     "./sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01-mobile/"
 *     "tokens.txt";
 * config.model_config.provider = "cpu";
 * config.model_config.num_threads = 1;
 *
 * config.keywords_file =
 *     "./sherpa-onnx-kws-zipformer-wenetspeech-3.3M-2024-01-01-mobile/"
 *     "test_wavs/test_keywords.txt";
 * config.max_active_paths = 4;
 * config.keywords_score = 3.0f;
 * config.keywords_threshold = 0.1f;
 * @endcode
 * @see SherpaOnnxCreateKeywordSpotter
 */
typedef struct SherpaOnnxKeywordSpotterConfig {
  /** Feature extraction parameters. */
  SherpaOnnxFeatureConfig feat_config;
  /** Streaming acoustic model configuration. */
  SherpaOnnxOnlineModelConfig model_config;
  /** Maximum number of active decoding paths. */
  int32_t max_active_paths;
  /** Number of trailing blank symbols required before trigger finalization. */
  int32_t num_trailing_blanks;
  /** Bonus score applied to keywords during search. */
  float keywords_score;
  /** Detection threshold. Larger values are more conservative. */
  float keywords_threshold;
  /** Optional keyword file. */
  const char *keywords_file;
  /** Optional in-memory keyword data. If non-null, it overrides @c
   * keywords_file. */
  const char *keywords_buf;
  /** Size in bytes of @c keywords_buf, excluding any trailing `'\0'`. */
  int32_t keywords_buf_size;
} SherpaOnnxKeywordSpotterConfig;

/** @brief Opaque keyword spotter handle. */
typedef struct SherpaOnnxKeywordSpotter SherpaOnnxKeywordSpotter;

/**
 * @brief Create a keyword spotter.
 *
 * @param config Keyword spotter configuration.
 * @return A newly allocated keyword spotter on success, or NULL on error. Free
 *         it with SherpaOnnxDestroyKeywordSpotter().
 * @see SherpaOnnxKeywordSpotterConfig, SherpaOnnxDestroyKeywordSpotter
 */
SHERPA_ONNX_API const SherpaOnnxKeywordSpotter *SherpaOnnxCreateKeywordSpotter(
    const SherpaOnnxKeywordSpotterConfig *config);

/**
 * @brief Destroy a keyword spotter.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @see SherpaOnnxCreateKeywordSpotter
 */
SHERPA_ONNX_API void SherpaOnnxDestroyKeywordSpotter(
    const SherpaOnnxKeywordSpotter *spotter);

/**
 * @brief Create a keyword spotting stream using the spotter's built-in keyword
 * list.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @return A newly allocated stream. Free it with
 * SherpaOnnxDestroyOnlineStream().
 */
SHERPA_ONNX_API const SherpaOnnxOnlineStream *SherpaOnnxCreateKeywordStream(
    const SherpaOnnxKeywordSpotter *spotter);

/**
 * @brief Create a keyword spotting stream with extra or replacement keywords.
 *
 * The @p keywords string uses the same textual format as the keyword files used
 * by the examples. For instance:
 *
 * @code
 * const SherpaOnnxOnlineStream *stream =
 *     SherpaOnnxCreateKeywordStreamWithKeywords(
 *         kws, "y ǎn y uán @演员/zh ī m íng @知名");
 * @endcode
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @param keywords Inline keyword definition string.
 * @return A newly allocated stream. Free it with
 * SherpaOnnxDestroyOnlineStream().
 */
SHERPA_ONNX_API const SherpaOnnxOnlineStream *
SherpaOnnxCreateKeywordStreamWithKeywords(
    const SherpaOnnxKeywordSpotter *spotter, const char *keywords);

/**
 * @brief Check whether a keyword stream has enough audio for decoding.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @param stream A pointer returned by SherpaOnnxCreateKeywordStream() or
 *               SherpaOnnxCreateKeywordStreamWithKeywords().
 * @return 1 if the stream is ready to decode; otherwise 0.
 */
SHERPA_ONNX_API int32_t
SherpaOnnxIsKeywordStreamReady(const SherpaOnnxKeywordSpotter *spotter,
                               const SherpaOnnxOnlineStream *stream);

/**
 * @brief Decode one ready keyword stream.
 *
 * Call this only when SherpaOnnxIsKeywordStreamReady() returns 1.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @param stream A pointer returned by SherpaOnnxCreateKeywordStream() or
 *               SherpaOnnxCreateKeywordStreamWithKeywords().
 */
SHERPA_ONNX_API void SherpaOnnxDecodeKeywordStream(
    const SherpaOnnxKeywordSpotter *spotter,
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Reset a keyword stream after a keyword is detected.
 *
 * The examples call this immediately after a successful trigger so the next
 * keyword can be detected independently.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @param stream A pointer returned by SherpaOnnxCreateKeywordStream() or
 *               SherpaOnnxCreateKeywordStreamWithKeywords().
 */
SHERPA_ONNX_API void SherpaOnnxResetKeywordStream(
    const SherpaOnnxKeywordSpotter *spotter,
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Decode multiple ready keyword streams in parallel.
 *
 * The caller must ensure every stream in @p streams is ready before calling
 * this function.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @param streams Array of ready streams.
 * @param n Number of elements in @p streams.
 */
SHERPA_ONNX_API void SherpaOnnxDecodeMultipleKeywordStreams(
    const SherpaOnnxKeywordSpotter *spotter,
    const SherpaOnnxOnlineStream **streams, int32_t n);

/**
 * @brief Get the current keyword spotting result for a stream.
 *
 * The returned snapshot may represent either "no trigger yet" or a detected
 * keyword. A common pattern is to check whether `strlen(r->keyword) != 0`.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @param stream A pointer returned by SherpaOnnxCreateKeywordStream() or
 *               SherpaOnnxCreateKeywordStreamWithKeywords().
 * @return A newly allocated result snapshot. Free it with
 *         SherpaOnnxDestroyKeywordResult().
 *
 * @code
 * const SherpaOnnxKeywordResult *r = SherpaOnnxGetKeywordResult(kws, stream);
 * if (r && r->json && strlen(r->keyword)) {
 *   fprintf(stderr, "Detected keyword: %s\n", r->json);
 *   SherpaOnnxResetKeywordStream(kws, stream);
 * }
 * SherpaOnnxDestroyKeywordResult(r);
 * @endcode
 * @see SherpaOnnxDestroyKeywordResult
 */
SHERPA_ONNX_API const SherpaOnnxKeywordResult *SherpaOnnxGetKeywordResult(
    const SherpaOnnxKeywordSpotter *spotter,
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Destroy a keyword result snapshot.
 *
 * @param r A pointer returned by SherpaOnnxGetKeywordResult().
 * @see SherpaOnnxGetKeywordResult
 */
SHERPA_ONNX_API void SherpaOnnxDestroyKeywordResult(
    const SherpaOnnxKeywordResult *r);

/**
 * @brief Get the current keyword spotting result as JSON.
 *
 * @param spotter A pointer returned by SherpaOnnxCreateKeywordSpotter().
 * @param stream A pointer returned by SherpaOnnxCreateKeywordStream() or
 *               SherpaOnnxCreateKeywordStreamWithKeywords().
 * @return A newly allocated JSON string. Free it with
 *         SherpaOnnxFreeKeywordResultJson().
 */
SHERPA_ONNX_API const char *SherpaOnnxGetKeywordResultAsJson(
    const SherpaOnnxKeywordSpotter *spotter,
    const SherpaOnnxOnlineStream *stream);

/**
 * @brief Free a JSON string returned by SherpaOnnxGetKeywordResultAsJson().
 *
 * @param s A pointer returned by SherpaOnnxGetKeywordResultAsJson().
 */
SHERPA_ONNX_API void SherpaOnnxFreeKeywordResultJson(const char *s);

// ============================================================
// For VAD
// ============================================================

/** @brief Configuration for a Silero VAD model. */
typedef struct SherpaOnnxSileroVadModelConfig {
  /** Path to `silero_vad.onnx`. */
  const char *model;
  /** Speech probability threshold. Frames above this value are speech. */
  float threshold;
  /** Minimum silence duration in seconds used to close a speech segment. */
  float min_silence_duration;
  /** Minimum speech duration in seconds to keep a detected segment. */
  float min_speech_duration;
  /** Input window size in samples. A common value is 512. */
  int32_t window_size;
  /**
   * Maximum speech duration in seconds.
   *
   * When a segment exceeds this value, the detector temporarily uses a higher
   * threshold to encourage a split.
   */
  float max_speech_duration;
} SherpaOnnxSileroVadModelConfig;

/** @brief Configuration for a Ten VAD model. */
typedef struct SherpaOnnxTenVadModelConfig {
  /** Path to `ten-vad.onnx`. */
  const char *model;
  /** Speech probability threshold. Frames above this value are speech. */
  float threshold;
  /** Minimum silence duration in seconds used to close a speech segment. */
  float min_silence_duration;
  /** Minimum speech duration in seconds to keep a detected segment. */
  float min_speech_duration;
  /** Input window size in samples. A common value is 256. */
  int32_t window_size;
  /**
   * Maximum speech duration in seconds.
   *
   * When a segment exceeds this value, the detector temporarily uses a higher
   * threshold to encourage a split.
   */
  float max_speech_duration;
} SherpaOnnxTenVadModelConfig;

/**
 * @brief Configuration shared by voice activity detectors.
 *
 * Exactly one VAD model family should be configured. Set either
 * @c silero_vad.model or @c ten_vad.model.
 *
 * If both are configured, the implementation will choose one of them, and
 * which one is used is implementation-defined. Do not rely on any precedence
 * rule.
 *
 * Example model files:
 * - `./silero_vad.onnx`
 * - `./ten-vad.onnx`
 *
 * @code
 * SherpaOnnxVadModelConfig config;
 * memset(&config, 0, sizeof(config));
 *
 * config.silero_vad.model = "./silero_vad.onnx";
 * config.silero_vad.threshold = 0.25f;
 * config.silero_vad.min_silence_duration = 0.5f;
 * config.silero_vad.min_speech_duration = 0.5f;
 * config.silero_vad.max_speech_duration = 10.0f;
 * config.silero_vad.window_size = 512;
 *
 * config.sample_rate = 16000;
 * config.num_threads = 1;
 * config.provider = "cpu";
 * config.debug = 0;
 * @endcode
 * @see SherpaOnnxCreateVoiceActivityDetector
 */
typedef struct SherpaOnnxVadModelConfig {
  /** Silero VAD configuration. */
  SherpaOnnxSileroVadModelConfig silero_vad;
  /** Input sample rate expected by the detector, usually 16000. */
  int32_t sample_rate;
  /** Number of backend threads. */
  int32_t num_threads;
  /** Execution provider, for example "cpu" or "cuda". */
  const char *provider;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Ten VAD configuration. */
  SherpaOnnxTenVadModelConfig ten_vad;
} SherpaOnnxVadModelConfig;

/** @brief Opaque circular-buffer handle used by helper APIs. */
typedef struct SherpaOnnxCircularBuffer SherpaOnnxCircularBuffer;

/**
 * @brief Create a floating-point circular buffer.
 *
 * @param capacity Maximum number of samples the buffer can keep.
 * @return A newly allocated buffer. Free it with
 *         SherpaOnnxDestroyCircularBuffer().
 *
 * @code
 * const SherpaOnnxCircularBuffer *buffer =
 *     SherpaOnnxCreateCircularBuffer(16000 * 30);
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxCircularBuffer *SherpaOnnxCreateCircularBuffer(
    int32_t capacity);

/**
 * @brief Destroy a circular buffer.
 *
 * @param buffer A pointer returned by SherpaOnnxCreateCircularBuffer().
 *
 * @code
 * SherpaOnnxDestroyCircularBuffer(buffer);
 * buffer = NULL;
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxDestroyCircularBuffer(
    const SherpaOnnxCircularBuffer *buffer);

/**
 * @brief Append samples to a circular buffer.
 *
 * @param buffer A pointer returned by SherpaOnnxCreateCircularBuffer().
 * @param p Pointer to @p n samples.
 * @param n Number of samples.
 *
 * @code
 * SherpaOnnxCircularBufferPush(buffer, wave->samples, wave->num_samples);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxCircularBufferPush(
    const SherpaOnnxCircularBuffer *buffer, const float *p, int32_t n);

/**
 * @brief Copy out a slice of samples from a circular buffer.
 *
 * @param buffer A pointer returned by SherpaOnnxCreateCircularBuffer().
 * @param start_index Absolute start index in the buffer timeline.
 * @param n Number of samples to copy.
 * @return A newly allocated array containing @p n samples. Free it with
 *         SherpaOnnxCircularBufferFree().
 *
 * @code
 * const float *samples = SherpaOnnxCircularBufferGet(buffer, start, 3200);
 * SherpaOnnxCircularBufferFree(samples);
 * @endcode
 */
SHERPA_ONNX_API const float *SherpaOnnxCircularBufferGet(
    const SherpaOnnxCircularBuffer *buffer, int32_t start_index, int32_t n);

/** @brief Free an array returned by SherpaOnnxCircularBufferGet(). */
SHERPA_ONNX_API void SherpaOnnxCircularBufferFree(const float *p);

/**
 * @brief Drop samples from the front of a circular buffer.
 *
 * @param buffer A pointer returned by SherpaOnnxCreateCircularBuffer().
 * @param n Number of samples to remove.
 */
SHERPA_ONNX_API void SherpaOnnxCircularBufferPop(
    const SherpaOnnxCircularBuffer *buffer, int32_t n);

/**
 * @brief Return the number of currently stored samples.
 *
 * @param buffer A pointer returned by SherpaOnnxCreateCircularBuffer().
 * @return Number of samples currently in the buffer.
 */
SHERPA_ONNX_API int32_t
SherpaOnnxCircularBufferSize(const SherpaOnnxCircularBuffer *buffer);

/**
 * @brief Return the current head index of the buffer timeline.
 *
 * The value is monotonically non-decreasing until
 * SherpaOnnxCircularBufferReset() is called.
 *
 * @param buffer A pointer returned by SherpaOnnxCreateCircularBuffer().
 * @return The current head index.
 */
SHERPA_ONNX_API int32_t
SherpaOnnxCircularBufferHead(const SherpaOnnxCircularBuffer *buffer);

/**
 * @brief Clear a circular buffer and reset its head index.
 *
 * @param buffer A pointer returned by SherpaOnnxCreateCircularBuffer().
 */
SHERPA_ONNX_API void SherpaOnnxCircularBufferReset(
    const SherpaOnnxCircularBuffer *buffer);

/**
 * @brief One detected speech segment returned by the VAD.
 *
 * The segment owns @c samples. Free the whole object with
 * SherpaOnnxDestroySpeechSegment().
 * @see SherpaOnnxVoiceActivityDetectorFront, SherpaOnnxDestroySpeechSegment
 */
typedef struct SherpaOnnxSpeechSegment {
  /** Start index, in input samples, of this segment. */
  int32_t start;
  /** Newly allocated mono samples for this segment. */
  float *samples;
  /** Number of samples in @c samples. */
  int32_t n;
} SherpaOnnxSpeechSegment;

/** @brief Opaque voice activity detector handle. */
typedef struct SherpaOnnxVoiceActivityDetector SherpaOnnxVoiceActivityDetector;

/**
 * @brief Create a voice activity detector.
 *
 * Example model files are shown in `c-api-examples/vad-whisper-c-api.c`.
 *
 * @param config VAD configuration.
 * @param buffer_size_in_seconds Internal buffering capacity in seconds.
 * @return A newly allocated detector on success, or NULL on configuration
 *         error. Free it with SherpaOnnxDestroyVoiceActivityDetector().
 *
 * @code
 * SherpaOnnxVadModelConfig config;
 * memset(&config, 0, sizeof(config));
 * config.silero_vad.model = "./silero_vad.onnx";
 * config.silero_vad.threshold = 0.25f;
 * config.silero_vad.min_silence_duration = 0.5f;
 * config.silero_vad.min_speech_duration = 0.5f;
 * config.silero_vad.max_speech_duration = 10.0f;
 * config.silero_vad.window_size = 512;
 * config.sample_rate = 16000;
 * config.num_threads = 1;
 *
 * const SherpaOnnxVoiceActivityDetector *vad =
 *     SherpaOnnxCreateVoiceActivityDetector(&config, 30.0f);
 * @endcode
 * @see SherpaOnnxVadModelConfig, SherpaOnnxDestroyVoiceActivityDetector
 */
SHERPA_ONNX_API const SherpaOnnxVoiceActivityDetector *
SherpaOnnxCreateVoiceActivityDetector(const SherpaOnnxVadModelConfig *config,
                                      float buffer_size_in_seconds);

/**
 * @brief Destroy a voice activity detector.
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 * @see SherpaOnnxCreateVoiceActivityDetector
 */
SHERPA_ONNX_API void SherpaOnnxDestroyVoiceActivityDetector(
    const SherpaOnnxVoiceActivityDetector *p);

/**
 * @brief Feed audio samples to the VAD.
 *
 * Input samples are mono floating-point PCM in the range [-1, 1].
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 * @param samples Pointer to @p n samples.
 * @param n Number of samples.
 *
 * @code
 * SherpaOnnxVoiceActivityDetectorAcceptWaveform(vad,
 *                                               wave->samples + i,
 *                                               window_size);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxVoiceActivityDetectorAcceptWaveform(
    const SherpaOnnxVoiceActivityDetector *p, const float *samples, int32_t n);

/**
 * @brief Check whether the detector currently has any completed speech segment.
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 * @return 1 if no completed speech segment is available; otherwise 0.
 */
SHERPA_ONNX_API int32_t
SherpaOnnxVoiceActivityDetectorEmpty(const SherpaOnnxVoiceActivityDetector *p);

/**
 * @brief Check whether the detector is currently inside speech.
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 * @return 1 if speech is currently detected; otherwise 0.
 */
SHERPA_ONNX_API int32_t SherpaOnnxVoiceActivityDetectorDetected(
    const SherpaOnnxVoiceActivityDetector *p);

/**
 * @brief Remove the front speech segment from the detector queue.
 *
 * Call this after consuming the segment returned by
 * SherpaOnnxVoiceActivityDetectorFront().
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 *
 * @code
 * const SherpaOnnxSpeechSegment *segment =
 *     SherpaOnnxVoiceActivityDetectorFront(vad);
 * // ... use segment ...
 * SherpaOnnxDestroySpeechSegment(segment);
 * SherpaOnnxVoiceActivityDetectorPop(vad);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxVoiceActivityDetectorPop(
    const SherpaOnnxVoiceActivityDetector *p);

/**
 * @brief Remove all queued speech segments.
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 */
SHERPA_ONNX_API void SherpaOnnxVoiceActivityDetectorClear(
    const SherpaOnnxVoiceActivityDetector *p);

/**
 * @brief Get the first queued speech segment.
 *
 * The returned segment is a copy owned by the caller. Free it with
 * SherpaOnnxDestroySpeechSegment().
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 * @return The first queued speech segment, or NULL if none is available.
 *
 * @code
 * while (!SherpaOnnxVoiceActivityDetectorEmpty(vad)) {
 *   const SherpaOnnxSpeechSegment *segment =
 *       SherpaOnnxVoiceActivityDetectorFront(vad);
 *   printf("start=%d, samples=%d\n", segment->start, segment->n);
 *   SherpaOnnxDestroySpeechSegment(segment);
 *   SherpaOnnxVoiceActivityDetectorPop(vad);
 * }
 * @endcode
 * @see SherpaOnnxSpeechSegment, SherpaOnnxDestroySpeechSegment
 */
SHERPA_ONNX_API const SherpaOnnxSpeechSegment *
SherpaOnnxVoiceActivityDetectorFront(const SherpaOnnxVoiceActivityDetector *p);

/**
 * @brief Destroy a speech segment returned by
 * SherpaOnnxVoiceActivityDetectorFront().
 *
 * @param p A pointer returned by SherpaOnnxVoiceActivityDetectorFront().
 * @see SherpaOnnxVoiceActivityDetectorFront
 */
SHERPA_ONNX_API void SherpaOnnxDestroySpeechSegment(
    const SherpaOnnxSpeechSegment *p);

/**
 * @brief Reset a voice activity detector so it can process a new stream.
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 */
SHERPA_ONNX_API void SherpaOnnxVoiceActivityDetectorReset(
    const SherpaOnnxVoiceActivityDetector *p);

/**
 * @brief Flush buffered tail samples and force final segmentation.
 *
 * Call this after the last chunk of input has been fed.
 *
 * @param p A pointer returned by SherpaOnnxCreateVoiceActivityDetector().
 *
 * @code
 * SherpaOnnxVoiceActivityDetectorFlush(vad);
 * @endcode
 */
SHERPA_ONNX_API void SherpaOnnxVoiceActivityDetectorFlush(
    const SherpaOnnxVoiceActivityDetector *p);

// ============================================================
// For offline Text-to-Speech (i.e., non-streaming TTS)
// ============================================================

/** @brief Configuration for a VITS TTS model. */
typedef struct SherpaOnnxOfflineTtsVitsModelConfig {
  /** Path to the VITS ONNX model, for example `./vits-ljs.onnx`. */
  const char *model;
  /** Path to the lexicon file. Ignored if @c data_dir is provided. */
  const char *lexicon;
  /** Path to the tokens file. */
  const char *tokens;
  /** Optional path to espeak-ng-data. */
  const char *data_dir;
  /** VITS noise scale. */
  float noise_scale;
  /** VITS duration noise scale. */
  float noise_scale_w;
  /** Speech rate scale. Values < 1 are slower; values > 1 are faster. */
  float length_scale;
  /** Unused legacy field kept for ABI compatibility. */
  const char *dict_dir;
} SherpaOnnxOfflineTtsVitsModelConfig;

/** @brief Configuration for a Matcha TTS model. */
typedef struct SherpaOnnxOfflineTtsMatchaModelConfig {
  /** Path to the Matcha acoustic model. */
  const char *acoustic_model;
  /** Path to the vocoder model, for example `./vocos-22khz-univ.onnx`. */
  const char *vocoder;
  /** Path to the lexicon file. */
  const char *lexicon;
  /** Path to the tokens file. */
  const char *tokens;
  /** Optional path to espeak-ng-data. */
  const char *data_dir;
  /** Matcha noise scale. */
  float noise_scale;
  /** Speech rate scale. Values < 1 are slower; values > 1 are faster. */
  float length_scale;
  /** Unused legacy field kept for ABI compatibility. */
  const char *dict_dir;
} SherpaOnnxOfflineTtsMatchaModelConfig;

/** @brief Configuration for a Kokoro TTS model. */
typedef struct SherpaOnnxOfflineTtsKokoroModelConfig {
  /** Path to the Kokoro model, for example `./kokoro-en-v0_19/model.onnx`. */
  const char *model;
  /** Path to the Kokoro voices file. */
  const char *voices;
  /** Path to the tokens file. */
  const char *tokens;
  /** Optional path to espeak-ng-data. */
  const char *data_dir;
  /** Speech rate scale. Values < 1 are slower; values > 1 are faster. */
  float length_scale;
  /** Unused legacy field kept for ABI compatibility. */
  const char *dict_dir;
  /** Optional lexicon file. */
  const char *lexicon;
  /** Optional language hint. */
  const char *lang;
} SherpaOnnxOfflineTtsKokoroModelConfig;

/** @brief Configuration for a Kitten TTS model. */
typedef struct SherpaOnnxOfflineTtsKittenModelConfig {
  /** Path to the Kitten model. */
  const char *model;
  /** Path to the Kitten voices file. */
  const char *voices;
  /** Path to the tokens file. */
  const char *tokens;
  /** Optional path to espeak-ng-data. */
  const char *data_dir;
  /** Speech rate scale. Values < 1 are slower; values > 1 are faster. */
  float length_scale;
} SherpaOnnxOfflineTtsKittenModelConfig;

/** @brief Configuration for a ZipVoice TTS model. */
typedef struct SherpaOnnxOfflineTtsZipvoiceModelConfig {
  /** Path to the tokens file. */
  const char *tokens;
  /** Path to the ZipVoice encoder model. */
  const char *encoder;
  /** Path to the ZipVoice decoder model. */
  const char *decoder;
  /** Path to the vocoder model. */
  const char *vocoder;
  /** Optional path to espeak-ng-data. */
  const char *data_dir;
  /** Path to the lexicon file. */
  const char *lexicon;
  /** Feature scaling factor. */
  float feat_scale;
  /** Time shift parameter. */
  float t_shift;
  /** Target RMS parameter. */
  float target_rms;
  /** Guidance scale parameter. */
  float guidance_scale;
} SherpaOnnxOfflineTtsZipvoiceModelConfig;

/** @brief Configuration for a Pocket TTS model. */
typedef struct SherpaOnnxOfflineTtsPocketModelConfig {
  /** Path to `lm_flow*.onnx`. */
  const char *lm_flow;
  /** Path to `lm_main*.onnx`. */
  const char *lm_main;
  /** Path to the Pocket encoder model. */
  const char *encoder;
  /** Path to the Pocket decoder model. */
  const char *decoder;
  /** Path to the text conditioner model. */
  const char *text_conditioner;
  /** Path to `vocab.json`. */
  const char *vocab_json;
  /** Path to `token_scores.json`. */
  const char *token_scores_json;
  /** Voice embedding cache capacity. */
  int32_t voice_embedding_cache_capacity;
} SherpaOnnxOfflineTtsPocketModelConfig;

/** @brief Configuration for a Supertonic TTS model. */
typedef struct SherpaOnnxOfflineTtsSupertonicModelConfig {
  /** Path to the duration predictor model. */
  const char *duration_predictor;
  /** Path to the text encoder model. */
  const char *text_encoder;
  /** Path to the vector estimator model. */
  const char *vector_estimator;
  /** Path to the vocoder model. */
  const char *vocoder;
  /** Path to `tts.json`. */
  const char *tts_json;
  /** Path to the unicode indexer file. */
  const char *unicode_indexer;
  /** Path to the voice style file. */
  const char *voice_style;
} SherpaOnnxOfflineTtsSupertonicModelConfig;

/**
 * @brief Configuration shared by offline TTS models.
 *
 * Exactly one TTS model family should be configured. For example, set only one
 * of @c vits, @c matcha, @c kokoro, @c kitten, @c zipvoice, @c pocket, or
 * @c supertonic.
 *
 * If multiple model families are configured at the same time, the
 * implementation will choose one of them, and which one is used is
 * implementation-defined. Do not rely on any precedence rule.
 *
 * Concrete example model packages in this repository include:
 * - `kokoro-en-v0_19`
 * - `sherpa-onnx-pocket-tts-int8-2026-01-26`
 * - `matcha-icefall-en_US-ljspeech`
 * - `sherpa-onnx-zipvoice-distill-int8-zh-en-emilia`
 */
typedef struct SherpaOnnxOfflineTtsModelConfig {
  /** VITS configuration. */
  SherpaOnnxOfflineTtsVitsModelConfig vits;
  /** Number of backend threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider, for example "cpu" or "cuda". */
  const char *provider;
  /** Matcha configuration. */
  SherpaOnnxOfflineTtsMatchaModelConfig matcha;
  /** Kokoro configuration. */
  SherpaOnnxOfflineTtsKokoroModelConfig kokoro;
  /** Kitten configuration. */
  SherpaOnnxOfflineTtsKittenModelConfig kitten;
  /** ZipVoice configuration. */
  SherpaOnnxOfflineTtsZipvoiceModelConfig zipvoice;
  /** Pocket configuration. */
  SherpaOnnxOfflineTtsPocketModelConfig pocket;
  /** Supertonic configuration. */
  SherpaOnnxOfflineTtsSupertonicModelConfig supertonic;
} SherpaOnnxOfflineTtsModelConfig;

/**
 * @brief Configuration for offline text-to-speech.
 *
 * @code
 * SherpaOnnxOfflineTtsConfig config;
 * memset(&config, 0, sizeof(config));
 *
 * config.model.kokoro.model = "./kokoro-en-v0_19/model.onnx";
 * config.model.kokoro.voices = "./kokoro-en-v0_19/voices.bin";
 * config.model.kokoro.tokens = "./kokoro-en-v0_19/tokens.txt";
 * config.model.kokoro.data_dir = "./kokoro-en-v0_19/espeak-ng-data";
 * config.model.num_threads = 2;
 * config.model.provider = "cpu";
 * config.model.debug = 0;
 * config.max_num_sentences = 2;
 * @endcode
 * @see SherpaOnnxCreateOfflineTts
 */
typedef struct SherpaOnnxOfflineTtsConfig {
  /** TTS model configuration. */
  SherpaOnnxOfflineTtsModelConfig model;
  /** Optional comma-separated rule FST list. */
  const char *rule_fsts;
  /** Maximum number of sentences processed per chunk. */
  int32_t max_num_sentences;
  /** Optional FAR archives used by text normalization rules. */
  const char *rule_fars;
  /** Default silence scale between sentences. */
  float silence_scale;
} SherpaOnnxOfflineTtsConfig;

/**
 * @brief Generated waveform returned by TTS APIs.
 *
 * The returned structure owns @c samples. Free the whole object with
 * SherpaOnnxDestroyOfflineTtsGeneratedAudio().
 * @see SherpaOnnxOfflineTtsGenerateWithConfig, SherpaOnnxDestroyOfflineTtsGeneratedAudio
 */
typedef struct SherpaOnnxGeneratedAudio {
  /** Generated mono samples in the range [-1, 1]. */
  const float *samples;
  /** Number of samples in @c samples. */
  int32_t n;
  /** Output sample rate. */
  int32_t sample_rate;
} SherpaOnnxGeneratedAudio;

/**
 * @brief Callback invoked during incremental generation.
 *
 * Return 1 to continue generation. Return 0 to stop early.
 *
 * The @p samples pointer is only valid during the callback. Copy the samples if
 * you need to keep them after the callback returns.
 */
typedef int32_t (*SherpaOnnxGeneratedAudioCallback)(const float *samples,
                                                    int32_t n);

/**
 * @brief Same as SherpaOnnxGeneratedAudioCallback but with an extra user
 * pointer.
 */
typedef int32_t (*SherpaOnnxGeneratedAudioCallbackWithArg)(const float *samples,
                                                           int32_t n,
                                                           void *arg);

/**
 * @brief Progress callback invoked during incremental generation.
 *
 * @param samples Newly generated samples valid only during the callback.
 * @param n Number of samples in @p samples.
 * @param p Progress in the range [0, 1].
 * @return Return 1 to continue generation. Return 0 to stop early.
 */
typedef int32_t (*SherpaOnnxGeneratedAudioProgressCallback)(
    const float *samples, int32_t n, float p);

/**
 * @brief Same as SherpaOnnxGeneratedAudioProgressCallback but with an extra
 * user pointer.
 */
typedef int32_t (*SherpaOnnxGeneratedAudioProgressCallbackWithArg)(
    const float *samples, int32_t n, float p, void *arg);

/** @brief Opaque offline TTS handle. */
typedef struct SherpaOnnxOfflineTts SherpaOnnxOfflineTts;

/**
 * @brief Create an offline TTS engine.
 *
 * @param config TTS configuration.
 * @return A newly allocated TTS engine on success, or NULL on configuration
 *         error. Free it with SherpaOnnxDestroyOfflineTts().
 *
 * @code
 * SherpaOnnxOfflineTtsConfig config;
 * memset(&config, 0, sizeof(config));
 * config.model.kokoro.model = "./kokoro-en-v0_19/model.onnx";
 * config.model.kokoro.voices = "./kokoro-en-v0_19/voices.bin";
 * config.model.kokoro.tokens = "./kokoro-en-v0_19/tokens.txt";
 * config.model.kokoro.data_dir = "./kokoro-en-v0_19/espeak-ng-data";
 * config.model.num_threads = 2;
 *
 * const SherpaOnnxOfflineTts *tts = SherpaOnnxCreateOfflineTts(&config);
 * @endcode
 * @see SherpaOnnxOfflineTtsConfig, SherpaOnnxDestroyOfflineTts
 */
SHERPA_ONNX_API const SherpaOnnxOfflineTts *SherpaOnnxCreateOfflineTts(
    const SherpaOnnxOfflineTtsConfig *config);

/**
 * @brief Destroy an offline TTS engine.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @see SherpaOnnxCreateOfflineTts
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineTts(
    const SherpaOnnxOfflineTts *tts);

/**
 * @brief Return the output sample rate of a TTS engine.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @return Output sample rate in Hz.
 */
SHERPA_ONNX_API int32_t
SherpaOnnxOfflineTtsSampleRate(const SherpaOnnxOfflineTts *tts);

/**
 * @brief Return the number of available speaker IDs.
 *
 * Single-speaker models often return 1.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @return Number of speakers supported by the model.
 */
SHERPA_ONNX_API int32_t
SherpaOnnxOfflineTtsNumSpeakers(const SherpaOnnxOfflineTts *tts);

/**
 * @brief Generate speech from text using the simple sid/speed interface.
 *
 * @deprecated Use SherpaOnnxOfflineTtsGenerateWithConfig() instead.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @param text Input text.
 * @param sid Speaker ID for multi-speaker models.
 * @param speed Speech rate. Values > 1 are faster.
 * @return Generated audio, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineTtsGeneratedAudio().
 *
 * @code
 * const SherpaOnnxGeneratedAudio *audio =
 *     SherpaOnnxOfflineTtsGenerate(tts, "Hello from sherpa-onnx!", 0, 1.0f);
 * SherpaOnnxWriteWave(audio->samples, audio->n, audio->sample_rate,
 *                     "./generated.wav");
 * SherpaOnnxDestroyOfflineTtsGeneratedAudio(audio);
 * @endcode
 * @see SherpaOnnxDestroyOfflineTtsGeneratedAudio, SherpaOnnxGenerationConfig
 */
SHERPA_ONNX_API SHERPA_ONNX_DEPRECATED(
    "Use SherpaOnnxOfflineTtsGenerateWithConfig() instead") const
    SherpaOnnxGeneratedAudio *SherpaOnnxOfflineTtsGenerate(
        const SherpaOnnxOfflineTts *tts, const char *text, int32_t sid,
        float speed);

/**
 * @brief Generate speech and receive incremental audio chunks through a
 * callback.
 *
 * @deprecated Use SherpaOnnxOfflineTtsGenerateWithConfig() instead.
 *
 * The callback receives newly generated samples. The sample pointer is valid
 * only for the duration of the callback.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @param text Input text.
 * @param sid Speaker ID for multi-speaker models.
 * @param speed Speech rate. Values > 1 are faster.
 * @param callback Incremental callback. Return 0 to stop generation early.
 * @return Final generated audio, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineTtsGeneratedAudio().
 */
SHERPA_ONNX_API SHERPA_ONNX_DEPRECATED(
    "Use SherpaOnnxOfflineTtsGenerateWithConfig() instead") const
    SherpaOnnxGeneratedAudio *SherpaOnnxOfflineTtsGenerateWithCallback(
        const SherpaOnnxOfflineTts *tts, const char *text, int32_t sid,
        float speed, SherpaOnnxGeneratedAudioCallback callback);

/**
 * @brief Generate speech with a progress callback.
 *
 * @deprecated Use SherpaOnnxOfflineTtsGenerateWithConfig() instead.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @param text Input text.
 * @param sid Speaker ID for multi-speaker models.
 * @param speed Speech rate. Values > 1 are faster.
 * @param callback Progress callback. Return 0 to stop generation early.
 * @return Final generated audio, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineTtsGeneratedAudio().
 *
 * @code
 * int32_t Progress(const float *samples, int32_t n, float p) {
 *   fprintf(stderr, "Progress: %.2f%%\n", p * 100);
 *   return 1;
 * }
 *
 * const SherpaOnnxGeneratedAudio *audio =
 *     SherpaOnnxOfflineTtsGenerateWithProgressCallback(tts, text, 0, 1.0f,
 *                                                      Progress);
 * @endcode
 */
SHERPA_ONNX_API SHERPA_ONNX_DEPRECATED(
    "Use SherpaOnnxOfflineTtsGenerateWithConfig() instead") const
    SherpaOnnxGeneratedAudio *SherpaOnnxOfflineTtsGenerateWithProgressCallback(
        const SherpaOnnxOfflineTts *tts, const char *text, int32_t sid,
        float speed, SherpaOnnxGeneratedAudioProgressCallback callback);

/**
 * @brief Generate speech with a progress callback that receives a user pointer.
 *
 * @deprecated Use SherpaOnnxOfflineTtsGenerateWithConfig() instead.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @param text Input text.
 * @param sid Speaker ID for multi-speaker models.
 * @param speed Speech rate. Values > 1 are faster.
 * @param callback Progress callback with user pointer. Return 0 to stop early.
 * @param arg User pointer forwarded to @p callback.
 * @return Final generated audio, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineTtsGeneratedAudio().
 */
SHERPA_ONNX_API SHERPA_ONNX_DEPRECATED(
    "Use SherpaOnnxOfflineTtsGenerateWithConfig() instead") const
    SherpaOnnxGeneratedAudio
        *SherpaOnnxOfflineTtsGenerateWithProgressCallbackWithArg(
            const SherpaOnnxOfflineTts *tts, const char *text, int32_t sid,
            float speed,
            SherpaOnnxGeneratedAudioProgressCallbackWithArg callback,
            void *arg);

/**
 * @brief Same as SherpaOnnxOfflineTtsGenerateWithCallback() but with a user
 * pointer.
 *
 * @deprecated Use SherpaOnnxOfflineTtsGenerateWithConfig() instead.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @param text Input text.
 * @param sid Speaker ID for multi-speaker models.
 * @param speed Speech rate. Values > 1 are faster.
 * @param callback Incremental callback with user pointer.
 * @param arg User pointer forwarded to @p callback.
 * @return Final generated audio, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineTtsGeneratedAudio().
 */
SHERPA_ONNX_API SHERPA_ONNX_DEPRECATED(
    "Use SherpaOnnxOfflineTtsGenerateWithConfig() instead") const
    SherpaOnnxGeneratedAudio *SherpaOnnxOfflineTtsGenerateWithCallbackWithArg(
        const SherpaOnnxOfflineTts *tts, const char *text, int32_t sid,
        float speed, SherpaOnnxGeneratedAudioCallbackWithArg callback,
        void *arg);

/**
 * @brief Deprecated ZipVoice-specific generation API.
 *
 * Use SherpaOnnxOfflineTtsGenerateWithConfig() instead.
 */
SHERPA_ONNX_API SHERPA_ONNX_DEPRECATED(
    "Use SherpaOnnxOfflineTtsGenerateWithConfig() instead") const
    SherpaOnnxGeneratedAudio *SherpaOnnxOfflineTtsGenerateWithZipvoice(
        const SherpaOnnxOfflineTts *tts, const char *text,
        const char *prompt_text, const float *prompt_samples, int32_t n_prompt,
        int32_t prompt_sr, float speed, int32_t num_steps);

/**
 * @brief Generation-time parameters shared by advanced TTS APIs.
 *
 * This struct supports both simple multi-speaker synthesis and more advanced
 * zero-shot or reference-conditioned models.
 *
 * Example for Pocket TTS:
 *
 * @code
 * SherpaOnnxGenerationConfig cfg;
 * memset(&cfg, 0, sizeof(cfg));
 * cfg.speed = 1.0f;
 * cfg.reference_audio = wave->samples;
 * cfg.reference_audio_len = wave->num_samples;
 * cfg.reference_sample_rate = wave->sample_rate;
 * cfg.extra = "{\"max_reference_audio_len\": 10.0, \"seed\": 42}";
 * @endcode
 * @see SherpaOnnxOfflineTtsGenerateWithConfig
 */
typedef struct SherpaOnnxGenerationConfig {
  /** Silence scale between sentences. */
  float silence_scale;
  /** Speech rate. Used only by models that support it. */
  float speed;
  /** Speaker ID for multi-speaker models. */
  int32_t sid;
  /** Optional reference audio for zero-shot or voice-cloning models. */
  const float *reference_audio;
  /** Length of @c reference_audio in samples. */
  int32_t reference_audio_len;
  /** Sample rate of @c reference_audio. */
  int32_t reference_sample_rate;
  /** Optional reference text associated with @c reference_audio. */
  const char *reference_text;
  /** Optional number of flow-matching steps. */
  int32_t num_steps;
  /** Optional model-specific JSON string with extra key/value pairs. */
  const char *extra;
} SherpaOnnxGenerationConfig;

/**
 * @brief Generate speech using the advanced configuration interface.
 *
 * This is the preferred API for new integrations. It supports callback-based
 * progress reporting and model-specific options such as reference audio.
 *
 * @param tts A pointer returned by SherpaOnnxCreateOfflineTts().
 * @param text Input text.
 * @param config Generation-time configuration.
 * @param callback Optional progress callback with user pointer. Return 0 to
 *                 stop early.
 * @param arg User pointer forwarded to @p callback.
 * @return Generated audio, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineTtsGeneratedAudio().
 *
 * @code
 * SherpaOnnxGenerationConfig cfg;
 * memset(&cfg, 0, sizeof(cfg));
 * cfg.sid = 0;
 * cfg.speed = 1.0f;
 * cfg.silence_scale = 0.2f;
 *
 * const SherpaOnnxGeneratedAudio *audio =
 *     SherpaOnnxOfflineTtsGenerateWithConfig(tts,
 *         "Today as always, men fall into two groups.",
 *         &cfg, NULL, NULL);
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxGeneratedAudio *
SherpaOnnxOfflineTtsGenerateWithConfig(
    const SherpaOnnxOfflineTts *tts, const char *text,
    const SherpaOnnxGenerationConfig *config,
    SherpaOnnxGeneratedAudioProgressCallbackWithArg callback, void *arg);

/**
 * @brief Destroy audio returned by a TTS generation API.
 *
 * @param p A pointer returned by one of the SherpaOnnxOfflineTtsGenerate*
 *          functions.
 * @see SherpaOnnxOfflineTtsGenerateWithConfig
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineTtsGeneratedAudio(
    const SherpaOnnxGeneratedAudio *p);

/**
 * @brief Write floating-point PCM to a mono 16-bit WAVE file.
 *
 * @param samples Pointer to @p n samples in the range [-1, 1].
 * @param n Number of samples.
 * @param sample_rate Sample rate in Hz.
 * @param filename Output filename.
 * @return 1 on success; 0 on failure.
 *
 * @code
 * SherpaOnnxWriteWave(audio->samples, audio->n, audio->sample_rate,
 *                     "./generated-kokoro-en.wav");
 * @endcode
 * @see SherpaOnnxReadWave
 */
SHERPA_ONNX_API int32_t SherpaOnnxWriteWave(const float *samples, int32_t n,
                                            int32_t sample_rate,
                                            const char *filename);

/**
 * @brief Return the number of bytes needed for a mono 16-bit WAVE file.
 *
 * @param n_samples Number of PCM samples.
 * @return Required buffer size in bytes.
 */
SHERPA_ONNX_API int64_t SherpaOnnxWaveFileSize(int32_t n_samples);

/**
 * @brief Write a mono 16-bit WAVE file to a caller-provided buffer.
 *
 * Allocate at least SherpaOnnxWaveFileSize(@p n) bytes before calling.
 *
 * @param samples Pointer to @p n samples in the range [-1, 1].
 * @param n Number of samples.
 * @param sample_rate Sample rate in Hz.
 * @param buffer Output buffer.
 */
SHERPA_ONNX_API void SherpaOnnxWriteWaveToBuffer(const float *samples,
                                                 int32_t n, int32_t sample_rate,
                                                 char *buffer);

/**
 * @brief Write multi-channel audio to a WAVE file (16-bit PCM).
 *
 * @param samples       samples[c] is a pointer to channel c samples in [-1, 1].
 * @param n             Number of samples per channel.
 * @param sample_rate   Sample rate in Hz.
 * @param num_channels  Number of channels.
 * @param filename      Output filename.
 * @return 1 on success; 0 on failure.
 */
SHERPA_ONNX_API int32_t SherpaOnnxWriteWaveMultiChannel(
    const float *const *samples, int32_t n, int32_t sample_rate,
    int32_t num_channels, const char *filename);

/**
 * @brief Decoded mono WAVE file content.
 *
 * Free this object with SherpaOnnxFreeWave().
 * @see SherpaOnnxReadWave, SherpaOnnxFreeWave
 */
typedef struct SherpaOnnxWave {
  /** Samples normalized to the range [-1, 1]. */
  const float *samples;
  /** Sample rate in Hz. */
  int32_t sample_rate;
  /** Number of samples. */
  int32_t num_samples;
} SherpaOnnxWave;

/**
 * @brief Read a mono 16-bit PCM WAVE file.
 *
 * @param filename Input WAVE filename.
 * @return A newly allocated wave object, or NULL on error. Free it with
 *         SherpaOnnxFreeWave().
 *
 * @code
 * const SherpaOnnxWave *wave = SherpaOnnxReadWave("./Obama.wav");
 * if (wave) {
 *   printf("sample_rate=%d, num_samples=%d\n",
 *          wave->sample_rate, wave->num_samples);
 *   SherpaOnnxFreeWave(wave);
 * }
 * @endcode
 * @see SherpaOnnxFreeWave, SherpaOnnxWave
 */
SHERPA_ONNX_API const SherpaOnnxWave *SherpaOnnxReadWave(const char *filename);

/**
 * @brief Read a mono 16-bit PCM WAVE file from binary memory.
 *
 * @param data Pointer to the WAVE file bytes.
 * @param n Size of @p data in bytes.
 * @return A newly allocated wave object, or NULL on error. Free it with
 *         SherpaOnnxFreeWave().
 */
SHERPA_ONNX_API const SherpaOnnxWave *SherpaOnnxReadWaveFromBinaryData(
    const char *data, int32_t n);

/**
 * @brief Destroy a wave object returned by SherpaOnnxReadWave() or
 * SherpaOnnxReadWaveFromBinaryData().
 * @see SherpaOnnxReadWave
 */
SHERPA_ONNX_API void SherpaOnnxFreeWave(const SherpaOnnxWave *wave);

/**
 * @brief Decoded multi-channel WAVE file content.
 *
 * Free this object with SherpaOnnxFreeMultiChannelWave().
 */
typedef struct SherpaOnnxMultiChannelWave {
  /** samples[c] points to channel c samples normalized to [-1, 1].
   * Note: The sample data for all channels are stored in a single contiguous
   * memory block, one channel after another.
   * */
  const float *const *samples;
  /** Number of channels. */
  int32_t num_channels;
  /** Number of samples per channel. */
  int32_t num_samples;
  /** Sample rate in Hz. */
  int32_t sample_rate;
} SherpaOnnxMultiChannelWave;

/**
 * @brief Read a multi-channel 16-bit PCM WAVE file.
 *
 * @param filename Input WAVE filename.
 * @return A newly allocated multi-channel wave object, or NULL on error.
 *         Free it with SherpaOnnxFreeMultiChannelWave().
 */
SHERPA_ONNX_API const SherpaOnnxMultiChannelWave *
SherpaOnnxReadWaveMultiChannel(const char *filename);

/**
 * @brief Destroy a multi-channel wave object.
 *
 * @param wave A pointer returned by SherpaOnnxReadWaveMultiChannel().
 */
SHERPA_ONNX_API void SherpaOnnxFreeMultiChannelWave(
    const SherpaOnnxMultiChannelWave *wave);

// ============================================================
// For spoken language identification
// ============================================================

/**
 * @brief Whisper-based model files for spoken language identification.
 *
 * Example:
 *
 * @code
 * SherpaOnnxSpokenLanguageIdentificationWhisperConfig whisper;
 * memset(&whisper, 0, sizeof(whisper));
 * whisper.encoder = "./sherpa-onnx-whisper-tiny/tiny-encoder.int8.onnx";
 * whisper.decoder = "./sherpa-onnx-whisper-tiny/tiny-decoder.int8.onnx";
 * @endcode
 */
typedef struct SherpaOnnxSpokenLanguageIdentificationWhisperConfig {
  /** Whisper encoder model. */
  const char *encoder;
  /** Whisper decoder model. */
  const char *decoder;
  /** Optional tail padding in samples appended internally before inference. */
  int32_t tail_paddings;
} SherpaOnnxSpokenLanguageIdentificationWhisperConfig;

/**
 * @brief Configuration for spoken language identification.
 *
 * The current implementation uses Whisper-based models.
 *
 * Example using `sherpa-onnx-whisper-tiny`:
 *
 * @code
 * SherpaOnnxSpokenLanguageIdentificationConfig config;
 * memset(&config, 0, sizeof(config));
 * config.whisper.encoder = "./sherpa-onnx-whisper-tiny/tiny-encoder.int8.onnx";
 * config.whisper.decoder = "./sherpa-onnx-whisper-tiny/tiny-decoder.int8.onnx";
 * config.num_threads = 1;
 * config.provider = "cpu";
 * @endcode
 */
typedef struct SherpaOnnxSpokenLanguageIdentificationConfig {
  /** Whisper model configuration. */
  SherpaOnnxSpokenLanguageIdentificationWhisperConfig whisper;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
} SherpaOnnxSpokenLanguageIdentificationConfig;

/** @brief Opaque spoken-language identification handle. */
typedef struct SherpaOnnxSpokenLanguageIdentification
    SherpaOnnxSpokenLanguageIdentification;

/**
 * @brief Create a spoken-language identifier.
 *
 * @param config Spoken-language identification configuration.
 * @return A newly allocated identifier on success, or NULL on error. Free it
 *         with SherpaOnnxDestroySpokenLanguageIdentification().
 * @see SherpaOnnxDestroySpokenLanguageIdentification
 */
SHERPA_ONNX_API const SherpaOnnxSpokenLanguageIdentification *
SherpaOnnxCreateSpokenLanguageIdentification(
    const SherpaOnnxSpokenLanguageIdentificationConfig *config);

/**
 * @brief Destroy a spoken-language identifier.
 *
 * @param slid A pointer returned by
 * SherpaOnnxCreateSpokenLanguageIdentification().
 * @see SherpaOnnxCreateSpokenLanguageIdentification
 */
SHERPA_ONNX_API void SherpaOnnxDestroySpokenLanguageIdentification(
    const SherpaOnnxSpokenLanguageIdentification *slid);

/**
 * @brief Create an offline stream for spoken-language identification.
 *
 * Feed audio to the returned stream with SherpaOnnxAcceptWaveformOffline(), and
 * then call SherpaOnnxSpokenLanguageIdentificationCompute().
 *
 * @param slid A pointer returned by
 * SherpaOnnxCreateSpokenLanguageIdentification().
 * @return A newly allocated offline stream. Free it with
 *         SherpaOnnxDestroyOfflineStream().
 */
SHERPA_ONNX_API SherpaOnnxOfflineStream *
SherpaOnnxSpokenLanguageIdentificationCreateOfflineStream(
    const SherpaOnnxSpokenLanguageIdentification *slid);

/**
 * @brief Result of spoken-language identification.
 *
 * Free this object with SherpaOnnxDestroySpokenLanguageIdentificationResult().
 */
typedef struct SherpaOnnxSpokenLanguageIdentificationResult {
  /**
   * Predicted language code such as `"en"`, `"de"`, `"zh"`, or `"es"`.
   */
  const char *lang;
} SherpaOnnxSpokenLanguageIdentificationResult;

/**
 * @brief Run spoken-language identification on an offline stream.
 *
 * Example:
 *
 * @code
 * SherpaOnnxOfflineStream *stream =
 *     SherpaOnnxSpokenLanguageIdentificationCreateOfflineStream(slid);
 * SherpaOnnxAcceptWaveformOffline(stream, wave->sample_rate, wave->samples,
 *                                 wave->num_samples);
 * const SherpaOnnxSpokenLanguageIdentificationResult *result =
 *     SherpaOnnxSpokenLanguageIdentificationCompute(slid, stream);
 * printf("lang=%s\n", result->lang);
 * SherpaOnnxDestroySpokenLanguageIdentificationResult(result);
 * SherpaOnnxDestroyOfflineStream(stream);
 * @endcode
 *
 * @param slid A pointer returned by
 * SherpaOnnxCreateSpokenLanguageIdentification().
 * @param s A pointer returned by
 *          SherpaOnnxSpokenLanguageIdentificationCreateOfflineStream().
 * @return A newly allocated result object. Free it with
 *         SherpaOnnxDestroySpokenLanguageIdentificationResult().
 */
SHERPA_ONNX_API const SherpaOnnxSpokenLanguageIdentificationResult *
SherpaOnnxSpokenLanguageIdentificationCompute(
    const SherpaOnnxSpokenLanguageIdentification *slid,
    const SherpaOnnxOfflineStream *s);

/**
 * @brief Destroy a spoken-language identification result.
 *
 * @param r A pointer returned by
 * SherpaOnnxSpokenLanguageIdentificationCompute().
 */
SHERPA_ONNX_API void SherpaOnnxDestroySpokenLanguageIdentificationResult(
    const SherpaOnnxSpokenLanguageIdentificationResult *r);

// ============================================================
// For speaker embedding extraction
// ============================================================
/**
 * @brief Configuration for speaker embedding extraction.
 *
 * Example using
 * `3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx`:
 *
 * @code
 * SherpaOnnxSpeakerEmbeddingExtractorConfig config;
 * memset(&config, 0, sizeof(config));
 * config.model = "./3dspeaker_speech_campplus_sv_zh-cn_16k-common.onnx";
 * config.num_threads = 1;
 * config.provider = "cpu";
 * @endcode
 * @see SherpaOnnxCreateSpeakerEmbeddingExtractor
 */
typedef struct SherpaOnnxSpeakerEmbeddingExtractorConfig {
  /** Speaker embedding model file. */
  const char *model;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
} SherpaOnnxSpeakerEmbeddingExtractorConfig;

/** @brief Opaque speaker embedding extractor handle. */
typedef struct SherpaOnnxSpeakerEmbeddingExtractor
    SherpaOnnxSpeakerEmbeddingExtractor;

/**
 * @brief Create a speaker embedding extractor.
 *
 * @param config Speaker embedding extractor configuration.
 * @return A newly allocated extractor on success, or NULL on error. Free it
 *         with SherpaOnnxDestroySpeakerEmbeddingExtractor().
 * @see SherpaOnnxSpeakerEmbeddingExtractorConfig, SherpaOnnxDestroySpeakerEmbeddingExtractor
 */
SHERPA_ONNX_API const SherpaOnnxSpeakerEmbeddingExtractor *
SherpaOnnxCreateSpeakerEmbeddingExtractor(
    const SherpaOnnxSpeakerEmbeddingExtractorConfig *config);

/**
 * @brief Destroy a speaker embedding extractor.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingExtractor().
 * @see SherpaOnnxCreateSpeakerEmbeddingExtractor
 */
SHERPA_ONNX_API void SherpaOnnxDestroySpeakerEmbeddingExtractor(
    const SherpaOnnxSpeakerEmbeddingExtractor *p);

/**
 * @brief Return the embedding dimension produced by the extractor.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingExtractor().
 * @return Embedding dimension.
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingExtractorDim(
    const SherpaOnnxSpeakerEmbeddingExtractor *p);

/**
 * @brief Create a streaming feature buffer for embedding extraction.
 *
 * Feed samples with SherpaOnnxOnlineStreamAcceptWaveform(), then call
 * SherpaOnnxSpeakerEmbeddingExtractorIsReady() and
 * SherpaOnnxSpeakerEmbeddingExtractorComputeEmbedding().
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingExtractor().
 * @return A newly allocated online stream. Free it with
 *         SherpaOnnxDestroyOnlineStream().
 */
SHERPA_ONNX_API const SherpaOnnxOnlineStream *
SherpaOnnxSpeakerEmbeddingExtractorCreateStream(
    const SherpaOnnxSpeakerEmbeddingExtractor *p);

/**
 * @brief Check whether enough audio has been provided to compute an embedding.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingExtractor().
 * @param s A pointer returned by
 * SherpaOnnxSpeakerEmbeddingExtractorCreateStream().
 * @return 1 if the stream is ready; otherwise 0.
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingExtractorIsReady(
    const SherpaOnnxSpeakerEmbeddingExtractor *p,
    const SherpaOnnxOnlineStream *s);

/**
 * @brief Compute the embedding for a stream.
 *
 * The returned vector has `SherpaOnnxSpeakerEmbeddingExtractorDim(p)` elements.
 * Free it with SherpaOnnxSpeakerEmbeddingExtractorDestroyEmbedding().
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingExtractor().
 * @param s A pointer returned by
 * SherpaOnnxSpeakerEmbeddingExtractorCreateStream().
 * @return A newly allocated embedding vector.
 *
 * @code
 * const SherpaOnnxOnlineStream *stream =
 *     SherpaOnnxSpeakerEmbeddingExtractorCreateStream(ex);
 * SherpaOnnxOnlineStreamAcceptWaveform(stream, wave->sample_rate,
 * wave->samples, wave->num_samples);
 * SherpaOnnxOnlineStreamInputFinished(stream);
 * if (SherpaOnnxSpeakerEmbeddingExtractorIsReady(ex, stream)) {
 *   const float *v =
 *       SherpaOnnxSpeakerEmbeddingExtractorComputeEmbedding(ex, stream);
 *   SherpaOnnxSpeakerEmbeddingExtractorDestroyEmbedding(v);
 * }
 * SherpaOnnxDestroyOnlineStream(stream);
 * @endcode
 */
SHERPA_ONNX_API const float *
SherpaOnnxSpeakerEmbeddingExtractorComputeEmbedding(
    const SherpaOnnxSpeakerEmbeddingExtractor *p,
    const SherpaOnnxOnlineStream *s);

/**
 * @brief Destroy an embedding vector returned by
 * SherpaOnnxSpeakerEmbeddingExtractorComputeEmbedding().
 *
 * @param v A pointer returned by
 *          SherpaOnnxSpeakerEmbeddingExtractorComputeEmbedding().
 */
SHERPA_ONNX_API void SherpaOnnxSpeakerEmbeddingExtractorDestroyEmbedding(
    const float *v);

/**
 * @brief Opaque speaker embedding manager handle.
 *
 * @see SherpaOnnxCreateSpeakerEmbeddingManager
 */
typedef struct SherpaOnnxSpeakerEmbeddingManager
    SherpaOnnxSpeakerEmbeddingManager;

/**
 * @brief Create a speaker embedding manager.
 *
 * The manager stores enrolled speaker embeddings and supports speaker search
 * and verification.
 *
 * @param dim Embedding dimension. This should match
 *            SherpaOnnxSpeakerEmbeddingExtractorDim().
 * @return A newly allocated manager. Free it with
 *         SherpaOnnxDestroySpeakerEmbeddingManager().
 * @see SherpaOnnxDestroySpeakerEmbeddingManager
 */
SHERPA_ONNX_API const SherpaOnnxSpeakerEmbeddingManager *
SherpaOnnxCreateSpeakerEmbeddingManager(int32_t dim);

/**
 * @brief Destroy a speaker embedding manager.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @see SherpaOnnxCreateSpeakerEmbeddingManager
 */
SHERPA_ONNX_API void SherpaOnnxDestroySpeakerEmbeddingManager(
    const SherpaOnnxSpeakerEmbeddingManager *p);

/**
 * @brief Add one enrollment embedding for a speaker.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param name Speaker name.
 * @param v Embedding vector with exactly `dim` elements.
 * @return 1 on success; 0 on error.
 */
SHERPA_ONNX_API int32_t
SherpaOnnxSpeakerEmbeddingManagerAdd(const SherpaOnnxSpeakerEmbeddingManager *p,
                                     const char *name, const float *v);

/**
 * @brief Add multiple enrollment embeddings for one speaker.
 *
 * @p v is a NULL-terminated array of embedding pointers:
 * `v[0]`, `v[1]`, ..., `v[n - 1]`, followed by `v[n] == NULL`.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param name Speaker name.
 * @param v NULL-terminated array of embedding pointers.
 * @return 1 on success; 0 on error.
 *
 * @code
 * const float *spk1_vec[4] = {e1, e2, e3, NULL};
 * SherpaOnnxSpeakerEmbeddingManagerAddList(manager, "fangjun", spk1_vec);
 * @endcode
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingManagerAddList(
    const SherpaOnnxSpeakerEmbeddingManager *p, const char *name,
    const float **v);

/**
 * @brief Add multiple enrollment embeddings packed in one flat array.
 *
 * The input contains @p n embeddings laid out consecutively, so the total
 * array length must be `n * dim`.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param name Speaker name.
 * @param v Flattened embedding array.
 * @param n Number of embeddings in @p v.
 * @return 1 on success; 0 on error.
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingManagerAddListFlattened(
    const SherpaOnnxSpeakerEmbeddingManager *p, const char *name,
    const float *v, int32_t n);

/**
 * @brief Remove a speaker from the manager.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param name Speaker name to remove.
 * @return 1 if removed; otherwise 0. Returns 0 if the speaker does not exist.
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingManagerRemove(
    const SherpaOnnxSpeakerEmbeddingManager *p, const char *name);

/**
 * @brief Search for the best matching enrolled speaker.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param v Query embedding vector.
 * @param threshold Minimum similarity threshold in the range [0, 1].
 * @return A newly allocated speaker name on match, or NULL if no speaker
 *         passes the threshold. Free the returned name with
 *         SherpaOnnxSpeakerEmbeddingManagerFreeSearch().
 */
SHERPA_ONNX_API const char *SherpaOnnxSpeakerEmbeddingManagerSearch(
    const SherpaOnnxSpeakerEmbeddingManager *p, const float *v,
    float threshold);

/**
 * @brief Free a string returned by SherpaOnnxSpeakerEmbeddingManagerSearch().
 *
 * @param name A pointer returned by
 *             SherpaOnnxSpeakerEmbeddingManagerSearch().
 */
SHERPA_ONNX_API void SherpaOnnxSpeakerEmbeddingManagerFreeSearch(
    const char *name);

/**
 * @brief One speaker match returned by the best-matches API.
 */
typedef struct SherpaOnnxSpeakerEmbeddingManagerSpeakerMatch {
  /** Similarity score. Larger means more similar. */
  float score;
  /** Speaker name. */
  const char *name;
} SherpaOnnxSpeakerEmbeddingManagerSpeakerMatch;

/**
 * @brief Collection of best speaker matches.
 *
 * Free this object with SherpaOnnxSpeakerEmbeddingManagerFreeBestMatches().
 */
typedef struct SherpaOnnxSpeakerEmbeddingManagerBestMatchesResult {
  /** Pointer to an array of @c count matches. */
  const SherpaOnnxSpeakerEmbeddingManagerSpeakerMatch *matches;
  /** Number of valid entries in @c matches. */
  int32_t count;
} SherpaOnnxSpeakerEmbeddingManagerBestMatchesResult;

/**
 * @brief Return up to @p n best matches above a similarity threshold.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param v Query embedding vector.
 * @param threshold Minimum similarity threshold in the range [0, 1].
 * @param n Maximum number of matches to return.
 * @return A newly allocated result object, or NULL if no matches are found.
 *         Free it with SherpaOnnxSpeakerEmbeddingManagerFreeBestMatches().
 */
SHERPA_ONNX_API const SherpaOnnxSpeakerEmbeddingManagerBestMatchesResult *
SherpaOnnxSpeakerEmbeddingManagerGetBestMatches(
    const SherpaOnnxSpeakerEmbeddingManager *p, const float *v, float threshold,
    int32_t n);

/**
 * @brief Destroy a best-matches result.
 *
 * @param r A pointer returned by
 * SherpaOnnxSpeakerEmbeddingManagerGetBestMatches().
 */
SHERPA_ONNX_API void SherpaOnnxSpeakerEmbeddingManagerFreeBestMatches(
    const SherpaOnnxSpeakerEmbeddingManagerBestMatchesResult *r);

/**
 * @brief Verify whether a query embedding matches a named speaker.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param name Speaker name to compare against.
 * @param v Query embedding vector.
 * @param threshold Minimum similarity threshold in the range [0, 1].
 * @return 1 if the speaker matches; otherwise 0.
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingManagerVerify(
    const SherpaOnnxSpeakerEmbeddingManager *p, const char *name,
    const float *v, float threshold);

/**
 * @brief Check whether a speaker is enrolled.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @param name Speaker name.
 * @return 1 if the speaker exists; otherwise 0.
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingManagerContains(
    const SherpaOnnxSpeakerEmbeddingManager *p, const char *name);

/**
 * @brief Return the number of enrolled speakers.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @return Number of enrolled speakers.
 */
SHERPA_ONNX_API int32_t SherpaOnnxSpeakerEmbeddingManagerNumSpeakers(
    const SherpaOnnxSpeakerEmbeddingManager *p);

/**
 * @brief Return all enrolled speaker names.
 *
 * The returned array is NULL-terminated. If no speakers are enrolled, the
 * returned array still exists and its first element is NULL.
 *
 * @param p A pointer returned by SherpaOnnxCreateSpeakerEmbeddingManager().
 * @return A newly allocated NULL-terminated array of speaker names. Free it
 *         with SherpaOnnxSpeakerEmbeddingManagerFreeAllSpeakers().
 */
SHERPA_ONNX_API const char *const *
SherpaOnnxSpeakerEmbeddingManagerGetAllSpeakers(
    const SherpaOnnxSpeakerEmbeddingManager *p);

/**
 * @brief Free an array returned by
 * SherpaOnnxSpeakerEmbeddingManagerGetAllSpeakers().
 *
 * @param names A pointer returned by
 * SherpaOnnxSpeakerEmbeddingManagerGetAllSpeakers().
 */
SHERPA_ONNX_API void SherpaOnnxSpeakerEmbeddingManagerFreeAllSpeakers(
    const char *const *names);

// ============================================================
// For audio tagging
// ============================================================
/** @brief Zipformer audio-tagging model configuration. */
typedef struct SherpaOnnxOfflineZipformerAudioTaggingModelConfig {
  /** Model filename. */
  const char *model;
} SherpaOnnxOfflineZipformerAudioTaggingModelConfig;

/**
 * @brief Audio-tagging model configuration.
 *
 * Configure exactly one model family. If multiple model families are provided,
 * one of them will be used and the choice is implementation-defined.
 *
 * Example using
 * `sherpa-onnx-zipformer-audio-tagging-2024-04-09`:
 *
 * @code
 * SherpaOnnxAudioTaggingModelConfig model;
 * memset(&model, 0, sizeof(model));
 * model.zipformer.model =
 *     "./sherpa-onnx-zipformer-audio-tagging-2024-04-09/model.int8.onnx";
 * model.num_threads = 1;
 * model.provider = "cpu";
 * @endcode
 */
typedef struct SherpaOnnxAudioTaggingModelConfig {
  /** Zipformer model configuration. */
  SherpaOnnxOfflineZipformerAudioTaggingModelConfig zipformer;
  /** Alternative CED model file. */
  const char *ced;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
} SherpaOnnxAudioTaggingModelConfig;

/**
 * @brief Configuration for audio tagging.
 *
 * @code
 * SherpaOnnxAudioTaggingConfig config;
 * memset(&config, 0, sizeof(config));
 * config.model.zipformer.model =
 *     "./sherpa-onnx-zipformer-audio-tagging-2024-04-09/model.int8.onnx";
 * config.model.num_threads = 1;
 * config.model.provider = "cpu";
 * config.labels =
 *     "./sherpa-onnx-zipformer-audio-tagging-2024-04-09/class_labels_indices.csv";
 * config.top_k = 5;
 * @endcode
 * @see SherpaOnnxCreateAudioTagging
 */
typedef struct SherpaOnnxAudioTaggingConfig {
  /** Acoustic model configuration. */
  SherpaOnnxAudioTaggingModelConfig model;
  /** CSV file containing class labels. */
  const char *labels;
  /** Default number of results to return when `top_k == -1` at inference time.
   */
  int32_t top_k;
} SherpaOnnxAudioTaggingConfig;

/**
 * @brief One audio-tagging prediction.
 */
typedef struct SherpaOnnxAudioEvent {
  /** Event label. */
  const char *name;
  /** Integer label index. */
  int32_t index;
  /** Probability or confidence score. */
  float prob;
} SherpaOnnxAudioEvent;

/** @brief Opaque audio tagger handle. */
typedef struct SherpaOnnxAudioTagging SherpaOnnxAudioTagging;

/**
 * @brief Create an audio tagger.
 *
 * @param config Audio-tagging configuration.
 * @return A newly allocated audio tagger on success, or NULL on error. Free it
 *         with SherpaOnnxDestroyAudioTagging().
 * @see SherpaOnnxAudioTaggingConfig, SherpaOnnxDestroyAudioTagging
 */
SHERPA_ONNX_API const SherpaOnnxAudioTagging *SherpaOnnxCreateAudioTagging(
    const SherpaOnnxAudioTaggingConfig *config);

/**
 * @brief Destroy an audio tagger.
 *
 * @param tagger A pointer returned by SherpaOnnxCreateAudioTagging().
 * @see SherpaOnnxCreateAudioTagging
 */
SHERPA_ONNX_API void SherpaOnnxDestroyAudioTagging(
    const SherpaOnnxAudioTagging *tagger);

/**
 * @brief Create an offline stream for audio tagging.
 *
 * @param tagger A pointer returned by SherpaOnnxCreateAudioTagging().
 * @return A newly allocated offline stream. Free it with
 *         SherpaOnnxDestroyOfflineStream().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineStream *
SherpaOnnxAudioTaggingCreateOfflineStream(const SherpaOnnxAudioTagging *tagger);

/**
 * @brief Run audio tagging on an offline stream.
 *
 * The returned array is NULL-terminated. If @p top_k is -1, the value stored in
 * `config.top_k` is used instead.
 *
 * @param tagger A pointer returned by SherpaOnnxCreateAudioTagging().
 * @param s A pointer returned by SherpaOnnxAudioTaggingCreateOfflineStream().
 * @param top_k Number of top results to return, or -1 to use the configured
 *              default.
 * @return A newly allocated NULL-terminated array of result pointers ordered by
 *         descending probability. Free it with
 *         SherpaOnnxAudioTaggingFreeResults().
 *
 * @code
 * const SherpaOnnxAudioEvent *const *results =
 *     SherpaOnnxAudioTaggingCompute(tagger, stream, 5);
 * for (int32_t i = 0; results[i] != NULL; ++i) {
 *   printf("%d %.3f %s\n", results[i]->index, results[i]->prob,
 *          results[i]->name);
 * }
 * SherpaOnnxAudioTaggingFreeResults(results);
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxAudioEvent *const *
SherpaOnnxAudioTaggingCompute(const SherpaOnnxAudioTagging *tagger,
                              const SherpaOnnxOfflineStream *s, int32_t top_k);

/**
 * @brief Destroy results returned by SherpaOnnxAudioTaggingCompute().
 *
 * @param p A pointer returned by SherpaOnnxAudioTaggingCompute().
 */
SHERPA_ONNX_API void SherpaOnnxAudioTaggingFreeResults(
    const SherpaOnnxAudioEvent *const *p);

// ============================================================
// For punctuation
// ============================================================

/**
 * @brief Offline punctuation model configuration.
 *
 * Example:
 *
 * @code
 * SherpaOnnxOfflinePunctuationModelConfig model;
 * memset(&model, 0, sizeof(model));
 * model.ct_transformer =
 *     "./sherpa-onnx-punct-ct-transformer-zh-en-vocab272727-2024-04-12/model.onnx";
 * model.num_threads = 1;
 * model.provider = "cpu";
 * @endcode
 */
typedef struct SherpaOnnxOfflinePunctuationModelConfig {
  /** Offline punctuation model file. */
  const char *ct_transformer;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
} SherpaOnnxOfflinePunctuationModelConfig;

/** @brief Configuration for offline punctuation. */
typedef struct SherpaOnnxOfflinePunctuationConfig {
  /** Model configuration. */
  SherpaOnnxOfflinePunctuationModelConfig model;
} SherpaOnnxOfflinePunctuationConfig;

/** @brief Opaque offline punctuation handle. */
typedef struct SherpaOnnxOfflinePunctuation SherpaOnnxOfflinePunctuation;

/**
 * @brief Create an offline punctuation processor.
 *
 * @param config Offline punctuation configuration.
 * @return A newly allocated punctuation processor on success, or NULL on
 *         error. Free it with SherpaOnnxDestroyOfflinePunctuation().
 * @see SherpaOnnxDestroyOfflinePunctuation, SherpaOfflinePunctuationAddPunct
 */
SHERPA_ONNX_API const SherpaOnnxOfflinePunctuation *
SherpaOnnxCreateOfflinePunctuation(
    const SherpaOnnxOfflinePunctuationConfig *config);

/**
 * @brief Destroy an offline punctuation processor.
 *
 * @param punct A pointer returned by SherpaOnnxCreateOfflinePunctuation().
 * @see SherpaOnnxCreateOfflinePunctuation
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflinePunctuation(
    const SherpaOnnxOfflinePunctuation *punct);

/**
 * @brief Add punctuation to a complete input text.
 *
 * @param punct A pointer returned by SherpaOnnxCreateOfflinePunctuation().
 * @param text Input text without punctuation.
 * @return A newly allocated punctuated string. Free it with
 *         SherpaOfflinePunctuationFreeText().
 * @see SherpaOfflinePunctuationFreeText
 */
SHERPA_ONNX_API const char *SherpaOfflinePunctuationAddPunct(
    const SherpaOnnxOfflinePunctuation *punct, const char *text);

/**
 * @brief Free a string returned by SherpaOfflinePunctuationAddPunct().
 *
 * @param text A pointer returned by SherpaOfflinePunctuationAddPunct().
 * @see SherpaOfflinePunctuationAddPunct
 */
SHERPA_ONNX_API void SherpaOfflinePunctuationFreeText(const char *text);

/**
 * @brief Online punctuation model configuration.
 *
 * Example using `sherpa-onnx-online-punct-en-2024-08-06`:
 *
 * @code
 * SherpaOnnxOnlinePunctuationModelConfig model;
 * memset(&model, 0, sizeof(model));
 * model.cnn_bilstm =
 * "./sherpa-onnx-online-punct-en-2024-08-06/model.int8.onnx"; model.bpe_vocab =
 * "./sherpa-onnx-online-punct-en-2024-08-06/bpe.vocab"; model.num_threads = 1;
 * model.provider = "cpu";
 * @endcode
 */
typedef struct SherpaOnnxOnlinePunctuationModelConfig {
  /** Online punctuation model file. */
  const char *cnn_bilstm;
  /** BPE vocabulary used by the model. */
  const char *bpe_vocab;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
} SherpaOnnxOnlinePunctuationModelConfig;

/** @brief Configuration for online punctuation. */
typedef struct SherpaOnnxOnlinePunctuationConfig {
  /** Model configuration. */
  SherpaOnnxOnlinePunctuationModelConfig model;
} SherpaOnnxOnlinePunctuationConfig;

/** @brief Opaque online punctuation handle. */
typedef struct SherpaOnnxOnlinePunctuation SherpaOnnxOnlinePunctuation;

/**
 * @brief Create an online punctuation processor.
 *
 * @param config Online punctuation configuration.
 * @return A newly allocated punctuation processor on success, or NULL on
 *         error. Free it with SherpaOnnxDestroyOnlinePunctuation().
 */
SHERPA_ONNX_API const SherpaOnnxOnlinePunctuation *
SherpaOnnxCreateOnlinePunctuation(
    const SherpaOnnxOnlinePunctuationConfig *config);

/**
 * @brief Destroy an online punctuation processor.
 *
 * @param punctuation A pointer returned by SherpaOnnxCreateOnlinePunctuation().
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOnlinePunctuation(
    const SherpaOnnxOnlinePunctuation *punctuation);

/**
 * @brief Add punctuation to one text chunk using the online punctuation model.
 *
 * @param punctuation A pointer returned by SherpaOnnxCreateOnlinePunctuation().
 * @param text Input text chunk.
 * @return A newly allocated punctuated string. Free it with
 *         SherpaOnnxOnlinePunctuationFreeText().
 *
 * @code
 * const char *out =
 *     SherpaOnnxOnlinePunctuationAddPunct(punct,
 *         "how are you i am fine thank you");
 * printf("%s\n", out);
 * SherpaOnnxOnlinePunctuationFreeText(out);
 * @endcode
 */
SHERPA_ONNX_API const char *SherpaOnnxOnlinePunctuationAddPunct(
    const SherpaOnnxOnlinePunctuation *punctuation, const char *text);

/**
 * @brief Free a string returned by SherpaOnnxOnlinePunctuationAddPunct().
 *
 * @param text A pointer returned by SherpaOnnxOnlinePunctuationAddPunct().
 */
SHERPA_ONNX_API void SherpaOnnxOnlinePunctuationFreeText(const char *text);

// For resampling
/** @brief Opaque linear resampler handle. */
typedef struct SherpaOnnxLinearResampler SherpaOnnxLinearResampler;

/**
 * @brief Create a linear resampler.
 *
 * If @p filter_cutoff_hz or @p num_zeros is 0, the following defaults
 * are used (same convention as alsa-play.cc):
 *
 * @code
 * float min_freq = samp_rate_in_hz < samp_rate_out_hz ? samp_rate_in_hz
 *                                                 : samp_rate_out_hz;
 * float filter_cutoff_hz = 0.99f * 0.5f * min_freq;
 * int32_t num_zeros = 6;
 * @endcode
 *
 * @param samp_rate_in_hz Input sample rate in Hz. Must be > 0.
 * @param samp_rate_out_hz Output sample rate in Hz. Must be > 0.
 * @param filter_cutoff_hz Low-pass cutoff frequency in Hz. Pass 0 to use
 *                         the default formula above. Must be >= 0.
 * @param num_zeros Low-pass filter width control parameter. Pass 0 to use
 *                  the default value of 6. Must be >= 0.
 * @return A newly allocated resampler, or nullptr on invalid input. Free it
 *         with SherpaOnnxDestroyLinearResampler().
 */
SHERPA_ONNX_API const SherpaOnnxLinearResampler *
SherpaOnnxCreateLinearResampler(int32_t samp_rate_in_hz,
                                int32_t samp_rate_out_hz,
                                float filter_cutoff_hz, int32_t num_zeros);

/**
 * @brief Destroy a linear resampler.
 *
 * @param p A pointer returned by SherpaOnnxCreateLinearResampler().
 */
SHERPA_ONNX_API void SherpaOnnxDestroyLinearResampler(
    const SherpaOnnxLinearResampler *p);

/**
 * @brief Reset a linear resampler to its initial state.
 *
 * @param p A pointer returned by SherpaOnnxCreateLinearResampler().
 */
SHERPA_ONNX_API void SherpaOnnxLinearResamplerReset(
    const SherpaOnnxLinearResampler *p);

/**
 * @brief Output chunk returned by SherpaOnnxLinearResamplerResample().
 *
 * Free this object with SherpaOnnxLinearResamplerResampleFree().
 */
typedef struct SherpaOnnxResampleOut {
  /** Output samples. */
  const float *samples;
  /** Number of output samples. */
  int32_t n;
} SherpaOnnxResampleOut;

/**
 * @brief Resample one chunk of input audio.
 *
 * Set @p flush to 1 for the final chunk so buffered samples are emitted.
 *
 * @param p A pointer returned by SherpaOnnxCreateLinearResampler().
 * @param input Input sample array.
 * @param input_dim Number of input samples.
 * @param flush 1 if this is the final chunk; otherwise 0.
 * @return A newly allocated output chunk. Free it with
 *         SherpaOnnxLinearResamplerResampleFree().
 */
SHERPA_ONNX_API const SherpaOnnxResampleOut *SherpaOnnxLinearResamplerResample(
    const SherpaOnnxLinearResampler *p, const float *input, int32_t input_dim,
    int32_t flush);

/**
 * @brief Destroy a resampler output chunk.
 *
 * @param p A pointer returned by SherpaOnnxLinearResamplerResample().
 */
SHERPA_ONNX_API void SherpaOnnxLinearResamplerResampleFree(
    const SherpaOnnxResampleOut *p);

/**
 * @brief Return the resampler input sample rate.
 *
 * @param p A pointer returned by SherpaOnnxCreateLinearResampler().
 * @return Input sample rate in Hz.
 */
SHERPA_ONNX_API int32_t SherpaOnnxLinearResamplerResampleGetInputSampleRate(
    const SherpaOnnxLinearResampler *p);

/**
 * @brief Return the resampler output sample rate.
 *
 * @param p A pointer returned by SherpaOnnxCreateLinearResampler().
 * @return Output sample rate in Hz.
 */
SHERPA_ONNX_API int32_t SherpaOnnxLinearResamplerResampleGetOutputSampleRate(
    const SherpaOnnxLinearResampler *p);

// =========================================================================
// For offline speaker diarization (i.e., non-streaming speaker diarization)
// =========================================================================
/** @brief Pyannote speaker-segmentation model configuration. */
typedef struct SherpaOnnxOfflineSpeakerSegmentationPyannoteModelConfig {
  /** Segmentation model filename. */
  const char *model;
} SherpaOnnxOfflineSpeakerSegmentationPyannoteModelConfig;

/**
 * @brief Segmentation model configuration for offline speaker diarization.
 *
 * Configure exactly one model family. If multiple model families are provided,
 * one is chosen and the choice is implementation-defined.
 */
typedef struct SherpaOnnxOfflineSpeakerSegmentationModelConfig {
  /** Pyannote segmentation model configuration. */
  SherpaOnnxOfflineSpeakerSegmentationPyannoteModelConfig pyannote;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
} SherpaOnnxOfflineSpeakerSegmentationModelConfig;

/**
 * @brief Fast clustering configuration.
 *
 * If @c num_clusters is greater than 0, @c threshold is ignored. When the
 * number of speakers is known in advance, setting @c num_clusters is strongly
 * recommended.
 */
typedef struct SherpaOnnxFastClusteringConfig {
  /** Known number of speakers. If > 0, threshold-based clustering is bypassed.
   */
  int32_t num_clusters;
  /** Distance threshold used when the number of speakers is unknown. */
  float threshold;
} SherpaOnnxFastClusteringConfig;

/**
 * @brief Configuration for offline speaker diarization.
 *
 * Example based on `offline-sepaker-diarization-c-api.c`:
 *
 * @code
 * SherpaOnnxOfflineSpeakerDiarizationConfig config;
 * memset(&config, 0, sizeof(config));
 * config.segmentation.pyannote.model =
 *     "./sherpa-onnx-pyannote-segmentation-3-0/model.onnx";
 * config.embedding.model =
 *     "./3dspeaker_speech_eres2net_base_sv_zh-cn_3dspeaker_16k.onnx";
 * config.clustering.num_clusters = 4;
 * @endcode
 */
typedef struct SherpaOnnxOfflineSpeakerDiarizationConfig {
  /** Speaker segmentation model configuration. */
  SherpaOnnxOfflineSpeakerSegmentationModelConfig segmentation;
  /** Speaker embedding extractor configuration. */
  SherpaOnnxSpeakerEmbeddingExtractorConfig embedding;
  /** Clustering configuration. */
  SherpaOnnxFastClusteringConfig clustering;
  /** Segments shorter than this duration in seconds are discarded. */
  float min_duration_on;
  /** Small gaps shorter than this duration in seconds may be merged. */
  float min_duration_off;
} SherpaOnnxOfflineSpeakerDiarizationConfig;

/** @brief Opaque offline speaker diarization handle. */
typedef struct SherpaOnnxOfflineSpeakerDiarization
    SherpaOnnxOfflineSpeakerDiarization;

/**
 * @brief Create an offline speaker diarization pipeline.
 *
 * @param config Offline speaker diarization configuration.
 * @return A newly allocated diarizer on success, or NULL on error. Free it
 *         with SherpaOnnxDestroyOfflineSpeakerDiarization().
 * @see SherpaOnnxDestroyOfflineSpeakerDiarization
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeakerDiarization *
SherpaOnnxCreateOfflineSpeakerDiarization(
    const SherpaOnnxOfflineSpeakerDiarizationConfig *config);

/**
 * @brief Destroy an offline speaker diarizer.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeakerDiarization().
 * @see SherpaOnnxCreateOfflineSpeakerDiarization
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineSpeakerDiarization(
    const SherpaOnnxOfflineSpeakerDiarization *sd);

/**
 * @brief Return the expected input sample rate.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeakerDiarization().
 * @return Required input sample rate in Hz.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOfflineSpeakerDiarizationGetSampleRate(
    const SherpaOnnxOfflineSpeakerDiarization *sd);

/**
 * @brief Update clustering-related settings of an existing diarizer.
 *
 * Only `config->clustering` is used. Other fields are ignored.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeakerDiarization().
 * @param config Configuration whose `clustering` field will be applied.
 */
SHERPA_ONNX_API void SherpaOnnxOfflineSpeakerDiarizationSetConfig(
    const SherpaOnnxOfflineSpeakerDiarization *sd,
    const SherpaOnnxOfflineSpeakerDiarizationConfig *config);

/** @brief Opaque offline speaker diarization result. */
typedef struct SherpaOnnxOfflineSpeakerDiarizationResult
    SherpaOnnxOfflineSpeakerDiarizationResult;

/**
 * @brief One diarization segment.
 */
typedef struct SherpaOnnxOfflineSpeakerDiarizationSegment {
  /** Segment start time in seconds. */
  float start;
  /** Segment end time in seconds. */
  float end;
  /** Speaker label, typically an integer cluster ID. */
  int32_t speaker;
} SherpaOnnxOfflineSpeakerDiarizationSegment;

/**
 * @brief Return the number of speakers in a diarization result.
 *
 * @param r A pointer returned by one of the
 *          SherpaOnnxOfflineSpeakerDiarizationProcess*() functions.
 * @return Number of speaker clusters.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOfflineSpeakerDiarizationResultGetNumSpeakers(
    const SherpaOnnxOfflineSpeakerDiarizationResult *r);

/**
 * @brief Return the number of diarization segments.
 *
 * @param r A pointer returned by one of the
 *          SherpaOnnxOfflineSpeakerDiarizationProcess*() functions.
 * @return Number of segments.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOfflineSpeakerDiarizationResultGetNumSegments(
    const SherpaOnnxOfflineSpeakerDiarizationResult *r);

/**
 * @brief Return segments sorted by start time.
 *
 * The returned array contains exactly
 * SherpaOnnxOfflineSpeakerDiarizationResultGetNumSegments() entries.
 *
 * @param r A pointer returned by one of the
 *          SherpaOnnxOfflineSpeakerDiarizationProcess*() functions.
 * @return A newly allocated segment array. Free it with
 *         SherpaOnnxOfflineSpeakerDiarizationDestroySegment().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeakerDiarizationSegment *
SherpaOnnxOfflineSpeakerDiarizationResultSortByStartTime(
    const SherpaOnnxOfflineSpeakerDiarizationResult *r);

/**
 * @brief Destroy a segment array returned by
 * SherpaOnnxOfflineSpeakerDiarizationResultSortByStartTime().
 *
 * @param s A pointer returned by
 *          SherpaOnnxOfflineSpeakerDiarizationResultSortByStartTime().
 */
SHERPA_ONNX_API void SherpaOnnxOfflineSpeakerDiarizationDestroySegment(
    const SherpaOnnxOfflineSpeakerDiarizationSegment *s);

/**
 * @brief Progress callback for offline speaker diarization.
 *
 * The current implementation reports progress but ignores the callback's
 * return value.
 */
typedef int32_t (*SherpaOnnxOfflineSpeakerDiarizationProgressCallback)(
    int32_t num_processed_chunks, int32_t num_total_chunks, void *arg);

/**
 * @brief Same as SherpaOnnxOfflineSpeakerDiarizationProgressCallback but
 * without a user pointer.
 */
typedef int32_t (*SherpaOnnxOfflineSpeakerDiarizationProgressCallbackNoArg)(
    int32_t num_processed_chunks, int32_t num_total_chunks);

/**
 * @brief Run offline speaker diarization.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeakerDiarization().
 * @param samples Input mono PCM samples normalized to [-1, 1].
 * @param n Number of input samples.
 * @return A newly allocated diarization result. Free it with
 *         SherpaOnnxOfflineSpeakerDiarizationDestroyResult().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeakerDiarizationResult *
SherpaOnnxOfflineSpeakerDiarizationProcess(
    const SherpaOnnxOfflineSpeakerDiarization *sd, const float *samples,
    int32_t n);

/**
 * @brief Run offline speaker diarization with a progress callback.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeakerDiarization().
 * @param samples Input mono PCM samples normalized to [-1, 1].
 * @param n Number of input samples.
 * @param callback Progress callback.
 * @param arg User pointer forwarded to @p callback.
 * @return A newly allocated diarization result. Free it with
 *         SherpaOnnxOfflineSpeakerDiarizationDestroyResult().
 *
 * @code
 * static int32_t ProgressCallback(int32_t done, int32_t total, void *arg) {
 *   fprintf(stderr, "progress %.2f%%\n", 100.0f * done / total);
 *   return 0;
 * }
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeakerDiarizationResult *
SherpaOnnxOfflineSpeakerDiarizationProcessWithCallback(
    const SherpaOnnxOfflineSpeakerDiarization *sd, const float *samples,
    int32_t n, SherpaOnnxOfflineSpeakerDiarizationProgressCallback callback,
    void *arg);

/**
 * @brief Run offline speaker diarization with a progress callback that has no
 * user pointer.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeakerDiarization().
 * @param samples Input mono PCM samples normalized to [-1, 1].
 * @param n Number of input samples.
 * @param callback Progress callback.
 * @return A newly allocated diarization result. Free it with
 *         SherpaOnnxOfflineSpeakerDiarizationDestroyResult().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeakerDiarizationResult *
SherpaOnnxOfflineSpeakerDiarizationProcessWithCallbackNoArg(
    const SherpaOnnxOfflineSpeakerDiarization *sd, const float *samples,
    int32_t n,
    SherpaOnnxOfflineSpeakerDiarizationProgressCallbackNoArg callback);

/**
 * @brief Destroy a diarization result.
 *
 * @param r A pointer returned by one of the
 *          SherpaOnnxOfflineSpeakerDiarizationProcess*() functions.
 */
SHERPA_ONNX_API void SherpaOnnxOfflineSpeakerDiarizationDestroyResult(
    const SherpaOnnxOfflineSpeakerDiarizationResult *r);

// =========================================================================
// For offline speech enhancement
// =========================================================================
/** @brief GTCRN offline denoiser model configuration. */
typedef struct SherpaOnnxOfflineSpeechDenoiserGtcrnModelConfig {
  /** Model filename. */
  const char *model;
} SherpaOnnxOfflineSpeechDenoiserGtcrnModelConfig;

/** @brief DPDFNet offline denoiser model configuration. */
typedef struct SherpaOnnxOfflineSpeechDenoiserDpdfNetModelConfig {
  /** Model filename. */
  const char *model;
} SherpaOnnxOfflineSpeechDenoiserDpdfNetModelConfig;

/**
 * @brief Speech denoiser model configuration shared by offline and online APIs.
 *
 * Configure exactly one model family. If multiple model families are provided,
 * one is chosen and the choice is implementation-defined.
 */
typedef struct SherpaOnnxOfflineSpeechDenoiserModelConfig {
  /** GTCRN model configuration. */
  SherpaOnnxOfflineSpeechDenoiserGtcrnModelConfig gtcrn;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
  /** DPDFNet model configuration. */
  SherpaOnnxOfflineSpeechDenoiserDpdfNetModelConfig dpdfnet;
} SherpaOnnxOfflineSpeechDenoiserModelConfig;

/** @brief Configuration for offline speech denoising. */
typedef struct SherpaOnnxOfflineSpeechDenoiserConfig {
  /** Model configuration. */
  SherpaOnnxOfflineSpeechDenoiserModelConfig model;
} SherpaOnnxOfflineSpeechDenoiserConfig;

/** @brief Opaque offline speech denoiser handle. */
typedef struct SherpaOnnxOfflineSpeechDenoiser SherpaOnnxOfflineSpeechDenoiser;

/**
 * @brief Create an offline speech denoiser.
 *
 * Example using `gtcrn_simple.onnx`:
 *
 * @code
 * SherpaOnnxOfflineSpeechDenoiserConfig config;
 * memset(&config, 0, sizeof(config));
 * config.model.gtcrn.model = "./gtcrn_simple.onnx";
 * @endcode
 *
 * @param config Offline denoiser configuration.
 * @return A newly allocated denoiser on success, or NULL on error. Free it
 *         with SherpaOnnxDestroyOfflineSpeechDenoiser().
 * @see SherpaOnnxDestroyOfflineSpeechDenoiser
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeechDenoiser *
SherpaOnnxCreateOfflineSpeechDenoiser(
    const SherpaOnnxOfflineSpeechDenoiserConfig *config);

/**
 * @brief Destroy an offline speech denoiser.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeechDenoiser().
 * @see SherpaOnnxCreateOfflineSpeechDenoiser
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineSpeechDenoiser(
    const SherpaOnnxOfflineSpeechDenoiser *sd);

/**
 * @brief Return the expected sample rate for the denoiser.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeechDenoiser().
 * @return Required input sample rate in Hz.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOfflineSpeechDenoiserGetSampleRate(
    const SherpaOnnxOfflineSpeechDenoiser *sd);

/**
 * @brief Denoised audio returned by offline or online speech enhancement APIs.
 *
 * Free this object with SherpaOnnxDestroyDenoisedAudio().
 */
typedef struct SherpaOnnxDenoisedAudio {
  /** Output samples in the range [-1, 1]. */
  const float *samples;
  /** Number of output samples. */
  int32_t n;
  /** Output sample rate in Hz. */
  int32_t sample_rate;
} SherpaOnnxDenoisedAudio;

/**
 * @brief Run offline speech denoising on a complete waveform.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOfflineSpeechDenoiser().
 * @param samples Input mono PCM samples normalized to [-1, 1].
 * @param n Number of input samples.
 * @param sample_rate Input sample rate in Hz.
 * @return A newly allocated denoised waveform. Free it with
 *         SherpaOnnxDestroyDenoisedAudio().
 *
 * @code
 * const SherpaOnnxDenoisedAudio *denoised =
 *     SherpaOnnxOfflineSpeechDenoiserRun(sd, wave->samples, wave->num_samples,
 *                                        wave->sample_rate);
 * SherpaOnnxWriteWave(denoised->samples, denoised->n, denoised->sample_rate,
 *                     "./enhanced.wav");
 * SherpaOnnxDestroyDenoisedAudio(denoised);
 * @endcode
 */
SHERPA_ONNX_API const SherpaOnnxDenoisedAudio *
SherpaOnnxOfflineSpeechDenoiserRun(const SherpaOnnxOfflineSpeechDenoiser *sd,
                                   const float *samples, int32_t n,
                                   int32_t sample_rate);

/**
 * @brief Destroy denoised audio returned by a speech enhancement API.
 *
 * @param p A pointer returned by SherpaOnnxOfflineSpeechDenoiserRun(),
 *          SherpaOnnxOnlineSpeechDenoiserRun(), or
 *          SherpaOnnxOnlineSpeechDenoiserFlush().
 */
SHERPA_ONNX_API void SherpaOnnxDestroyDenoisedAudio(
    const SherpaOnnxDenoisedAudio *p);

// =========================================================================
// For streaming speech enhancement
// =========================================================================
/** @brief Configuration for streaming speech denoising. */
typedef struct SherpaOnnxOnlineSpeechDenoiserConfig {
  /** Model configuration. */
  SherpaOnnxOfflineSpeechDenoiserModelConfig model;
} SherpaOnnxOnlineSpeechDenoiserConfig;

/** @brief Opaque online speech denoiser handle. */
typedef struct SherpaOnnxOnlineSpeechDenoiser SherpaOnnxOnlineSpeechDenoiser;

/**
 * @brief Create an online speech denoiser.
 *
 * @param config Online denoiser configuration.
 * @return A newly allocated denoiser on success, or NULL on error. Free it
 *         with SherpaOnnxDestroyOnlineSpeechDenoiser().
 * @see SherpaOnnxDestroyOnlineSpeechDenoiser
 */
SHERPA_ONNX_API const SherpaOnnxOnlineSpeechDenoiser *
SherpaOnnxCreateOnlineSpeechDenoiser(
    const SherpaOnnxOnlineSpeechDenoiserConfig *config);

/**
 * @brief Destroy an online speech denoiser.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOnlineSpeechDenoiser().
 * @see SherpaOnnxCreateOnlineSpeechDenoiser
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOnlineSpeechDenoiser(
    const SherpaOnnxOnlineSpeechDenoiser *sd);

/**
 * @brief Return the expected input sample rate for the online denoiser.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOnlineSpeechDenoiser().
 * @return Required input sample rate in Hz.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOnlineSpeechDenoiserGetSampleRate(
    const SherpaOnnxOnlineSpeechDenoiser *sd);

/**
 * @brief Return the recommended chunk size in samples for streaming input.
 *
 * Example programs feed audio to the online denoiser in this chunk size.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOnlineSpeechDenoiser().
 * @return Frame shift in samples.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOnlineSpeechDenoiserGetFrameShiftInSamples(
    const SherpaOnnxOnlineSpeechDenoiser *sd);

/**
 * @brief Process one chunk of streaming audio.
 *
 * This function is not thread-safe. It may return NULL when not enough input
 * has been accumulated to produce denoised output yet.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOnlineSpeechDenoiser().
 * @param samples Input chunk normalized to [-1, 1].
 * @param n Number of input samples.
 * @param sample_rate Input sample rate in Hz.
 * @return A newly allocated denoised chunk, or NULL if no output is available
 *         yet. Free non-NULL results with SherpaOnnxDestroyDenoisedAudio().
 */
SHERPA_ONNX_API const SherpaOnnxDenoisedAudio *
SherpaOnnxOnlineSpeechDenoiserRun(const SherpaOnnxOnlineSpeechDenoiser *sd,
                                  const float *samples, int32_t n,
                                  int32_t sample_rate);

/**
 * @brief Flush buffered samples and reset the online denoiser.
 *
 * This also resets the denoiser so it can be reused for a new utterance.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOnlineSpeechDenoiser().
 * @return A newly allocated denoised chunk, or NULL if no buffered output
 *         remains. Free non-NULL results with SherpaOnnxDestroyDenoisedAudio().
 */
SHERPA_ONNX_API const SherpaOnnxDenoisedAudio *
SherpaOnnxOnlineSpeechDenoiserFlush(const SherpaOnnxOnlineSpeechDenoiser *sd);

/**
 * @brief Reset an online denoiser so it can process a new stream.
 *
 * @param sd A pointer returned by SherpaOnnxCreateOnlineSpeechDenoiser().
 */
SHERPA_ONNX_API void SherpaOnnxOnlineSpeechDenoiserReset(
    const SherpaOnnxOnlineSpeechDenoiser *sd);

// =========================================================================
// Source separation
// =========================================================================

/** @brief Spleeter source-separation model configuration. */
typedef struct SherpaOnnxOfflineSourceSeparationSpleeterModelConfig {
  /** Path to the vocals ONNX model. */
  const char *vocals;
  /** Path to the accompaniment ONNX model. */
  const char *accompaniment;
} SherpaOnnxOfflineSourceSeparationSpleeterModelConfig;

/** @brief UVR (MDX-Net) source-separation model configuration. */
typedef struct SherpaOnnxOfflineSourceSeparationUvrModelConfig {
  /** Path to the UVR ONNX model. */
  const char *model;
} SherpaOnnxOfflineSourceSeparationUvrModelConfig;

/** @brief Source-separation model configuration. */
typedef struct SherpaOnnxOfflineSourceSeparationModelConfig {
  SherpaOnnxOfflineSourceSeparationSpleeterModelConfig spleeter;
  SherpaOnnxOfflineSourceSeparationUvrModelConfig uvr;
  int32_t num_threads;
  int32_t debug;
  const char *provider;
} SherpaOnnxOfflineSourceSeparationModelConfig;

/** @brief Top-level source-separation configuration. */
typedef struct SherpaOnnxOfflineSourceSeparationConfig {
  SherpaOnnxOfflineSourceSeparationModelConfig model;
} SherpaOnnxOfflineSourceSeparationConfig;

/** @brief Opaque source-separation engine handle. */
typedef struct SherpaOnnxOfflineSourceSeparation
    SherpaOnnxOfflineSourceSeparation;

/**
 * @brief Create a source-separation engine.
 *
 * @param config Source-separation configuration.
 * @return A newly allocated engine on success, or NULL on error. Free it
 *         with SherpaOnnxDestroyOfflineSourceSeparation().
 * @see SherpaOnnxDestroyOfflineSourceSeparation
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSourceSeparation *
SherpaOnnxCreateOfflineSourceSeparation(
    const SherpaOnnxOfflineSourceSeparationConfig *config);

/**
 * @brief Destroy a source-separation engine.
 *
 * @param ss A pointer returned by SherpaOnnxCreateOfflineSourceSeparation().
 * @see SherpaOnnxCreateOfflineSourceSeparation
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineSourceSeparation(
    const SherpaOnnxOfflineSourceSeparation *ss);

/**
 * @brief Return the output sample rate of the source-separation engine.
 *
 * @param ss A pointer returned by SherpaOnnxCreateOfflineSourceSeparation().
 * @return Output sample rate in Hz.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOfflineSourceSeparationGetOutputSampleRate(
    const SherpaOnnxOfflineSourceSeparation *ss);

/**
 * @brief Return the number of stems produced by the engine.
 *
 * For Spleeter 2-stems this returns 2 (vocals + accompaniment).
 *
 * @param ss A pointer returned by SherpaOnnxCreateOfflineSourceSeparation().
 * @return Number of output stems.
 */
SHERPA_ONNX_API int32_t SherpaOnnxOfflineSourceSeparationGetNumberOfStems(
    const SherpaOnnxOfflineSourceSeparation *ss);

/** @brief A single stem (one output track) with one or more channels. */
typedef struct SherpaOnnxSourceSeparationStem {
  /** samples[c] points to the heap-allocated sample array for channel c. */
  float **samples;
  /** Number of channels in this stem. */
  int32_t num_channels;
  /** Number of samples per channel. */
  int32_t n;
} SherpaOnnxSourceSeparationStem;

/** @brief Output of a source-separation run. */
typedef struct SherpaOnnxSourceSeparationOutput {
  /** Heap-allocated array of stems (length num_stems). */
  const SherpaOnnxSourceSeparationStem *stems;
  /** Number of stems. */
  int32_t num_stems;
  /** Sample rate of every stem in Hz. */
  int32_t sample_rate;
} SherpaOnnxSourceSeparationOutput;

/**
 * @brief Run source separation on multi-channel audio.
 *
 * All input channels must have the same number of samples.
 *
 * @param ss            A pointer returned by
 *                      SherpaOnnxCreateOfflineSourceSeparation().
 * @param samples       samples[c] is a float array for channel c, values in
 *                      [-1, 1].
 * @param num_channels  Number of input channels.
 * @param num_samples   Number of samples per channel (all channels must have
 *                      the same length).
 * @param sample_rate   Input sample rate in Hz.
 * @return A newly allocated output on success, or NULL on error. Free it
 *         with SherpaOnnxDestroySourceSeparationOutput().
 */
SHERPA_ONNX_API const SherpaOnnxSourceSeparationOutput *
SherpaOnnxOfflineSourceSeparationProcess(
    const SherpaOnnxOfflineSourceSeparation *ss, const float *const *samples,
    int32_t num_channels, int32_t num_samples, int32_t sample_rate);

/**
 * @brief Destroy the output of a source-separation run.
 *
 * @param p A pointer returned by SherpaOnnxOfflineSourceSeparationProcess().
 */
SHERPA_ONNX_API void SherpaOnnxDestroySourceSeparationOutput(
    const SherpaOnnxSourceSeparationOutput *p);

#ifdef __OHOS__

/**
 * @brief HarmonyOS native resource manager type.
 *
 * Pass the resource manager provided by the HarmonyOS application runtime when
 * using the `*OHOS()` constructors below.
 */
typedef struct NativeResourceManager NativeResourceManager;

/**
 * @brief Create an offline speech denoiser on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateOfflineSpeechDenoiser().
 *
 * @param config Offline denoiser configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated denoiser, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineSpeechDenoiser().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeechDenoiser *
SherpaOnnxCreateOfflineSpeechDenoiserOHOS(
    const SherpaOnnxOfflineSpeechDenoiserConfig *config,
    NativeResourceManager *mgr);

/**
 * @brief Create an online speech denoiser on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateOnlineSpeechDenoiser().
 *
 * @param config Online denoiser configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated denoiser, or NULL on error. Free it with
 *         SherpaOnnxDestroyOnlineSpeechDenoiser().
 */
SHERPA_ONNX_API const SherpaOnnxOnlineSpeechDenoiser *
SherpaOnnxCreateOnlineSpeechDenoiserOHOS(
    const SherpaOnnxOnlineSpeechDenoiserConfig *config,
    NativeResourceManager *mgr);

/**
 * @brief Create an online recognizer on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateOnlineRecognizer().
 *
 * @param config Recognizer configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated recognizer, or NULL on error. Free it with
 *         SherpaOnnxDestroyOnlineRecognizer().
 */
SHERPA_ONNX_API const SherpaOnnxOnlineRecognizer *
SherpaOnnxCreateOnlineRecognizerOHOS(
    const SherpaOnnxOnlineRecognizerConfig *config, NativeResourceManager *mgr);

/**
 * @brief Create an offline recognizer on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateOfflineRecognizer().
 *
 * @param config Recognizer configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated recognizer, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineRecognizer().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineRecognizer *
SherpaOnnxCreateOfflineRecognizerOHOS(
    const SherpaOnnxOfflineRecognizerConfig *config,
    NativeResourceManager *mgr);

/**
 * @brief Create a voice activity detector on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateVoiceActivityDetector().
 *
 * @param config VAD model configuration.
 * @param buffer_size_in_seconds Internal buffer duration in seconds.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated VAD instance, or NULL on error. Free it with
 *         SherpaOnnxDestroyVoiceActivityDetector().
 */
SHERPA_ONNX_API const SherpaOnnxVoiceActivityDetector *
SherpaOnnxCreateVoiceActivityDetectorOHOS(
    const SherpaOnnxVadModelConfig *config, float buffer_size_in_seconds,
    NativeResourceManager *mgr);

/**
 * @brief Create an offline TTS engine on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateOfflineTts().
 *
 * @param config Offline TTS configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated TTS engine, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineTts().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineTts *SherpaOnnxCreateOfflineTtsOHOS(
    const SherpaOnnxOfflineTtsConfig *config, NativeResourceManager *mgr);

/**
 * @brief Create an offline punctuation processor on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateOfflinePunctuation().
 *
 * @param config Offline punctuation configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated punctuation processor, or NULL on error. Free it
 *         with SherpaOnnxDestroyOfflinePunctuation().
 */
SHERPA_ONNX_API const SherpaOnnxOfflinePunctuation *
SherpaOnnxCreateOfflinePunctuationOHOS(
    const SherpaOnnxOfflinePunctuationConfig *config,
    NativeResourceManager *mgr);

/**
 * @brief Create an online punctuation processor on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateOnlinePunctuation().
 *
 * @param config Online punctuation configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated punctuation processor, or NULL on error. Free it
 *         with SherpaOnnxDestroyOnlinePunctuation().
 */
SHERPA_ONNX_API const SherpaOnnxOnlinePunctuation *
SherpaOnnxCreateOnlinePunctuationOHOS(
    const SherpaOnnxOnlinePunctuationConfig *config,
    NativeResourceManager *mgr);

/**
 * @brief Create a speaker embedding extractor on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of
 * SherpaOnnxCreateSpeakerEmbeddingExtractor().
 *
 * @param config Speaker embedding extractor configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated extractor, or NULL on error. Free it with
 *         SherpaOnnxDestroySpeakerEmbeddingExtractor().
 */
SHERPA_ONNX_API const SherpaOnnxSpeakerEmbeddingExtractor *
SherpaOnnxCreateSpeakerEmbeddingExtractorOHOS(
    const SherpaOnnxSpeakerEmbeddingExtractorConfig *config,
    NativeResourceManager *mgr);

/**
 * @brief Create a keyword spotter on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of SherpaOnnxCreateKeywordSpotter().
 *
 * @param config Keyword spotter configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated keyword spotter, or NULL on error. Free it with
 *         SherpaOnnxDestroyKeywordSpotter().
 */
SHERPA_ONNX_API const SherpaOnnxKeywordSpotter *
SherpaOnnxCreateKeywordSpotterOHOS(const SherpaOnnxKeywordSpotterConfig *config,
                                   NativeResourceManager *mgr);

/**
 * @brief Create an offline speaker diarizer on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of
 * SherpaOnnxCreateOfflineSpeakerDiarization().
 *
 * @param config Offline speaker diarization configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated diarizer, or NULL on error. Free it with
 *         SherpaOnnxDestroyOfflineSpeakerDiarization().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSpeakerDiarization *
SherpaOnnxCreateOfflineSpeakerDiarizationOHOS(
    const SherpaOnnxOfflineSpeakerDiarizationConfig *config,
    NativeResourceManager *mgr);

/**
 * @brief Create a source separation engine on HarmonyOS.
 *
 * This is the HarmonyOS counterpart of
 * SherpaOnnxCreateOfflineSourceSeparation().
 *
 * @param config Source separation configuration.
 * @param mgr HarmonyOS resource manager used to resolve bundled assets.
 * @return A newly allocated source separation engine, or NULL on error. Free it
 *         with SherpaOnnxDestroyOfflineSourceSeparation().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineSourceSeparation *
SherpaOnnxCreateOfflineSourceSeparationOHOS(
    const SherpaOnnxOfflineSourceSeparationConfig *config,
    NativeResourceManager *mgr);
#endif

// ============================================================
// For diacritization
// ============================================================

/**
 * @brief Offline diacritization model configuration.
 */
typedef struct SherpaOnnxOfflineDiacritizationModelConfig {
  /** Offline diacritization encoder model file. */
  const char *catt_encoder;
  /** Offline diacritization decoder model file. */
  const char *catt_decoder;
  /** Number of inference threads. */
  int32_t num_threads;
  /** Non-zero to print debug information. */
  int32_t debug;
  /** Execution provider such as `"cpu"`. */
  const char *provider;
} SherpaOnnxOfflineDiacritizationModelConfig;

/** @brief Configuration for offline diacritization. */
typedef struct SherpaOnnxOfflineDiacritizationConfig {
  /** Model configuration. */
  SherpaOnnxOfflineDiacritizationModelConfig model;
} SherpaOnnxOfflineDiacritizationConfig;

/** @brief Opaque offline diacritization handle. */
typedef struct SherpaOnnxOfflineDiacritization SherpaOnnxOfflineDiacritization;

/**
 * @brief Create an offline diacritization processor.
 *
 * @param config Offline diacritization configuration.
 * @return A newly allocated diacritization processor on success, or NULL on
 *         error. Free it with SherpaOnnxDestroyOfflineDiacritization().
 */
SHERPA_ONNX_API const SherpaOnnxOfflineDiacritization *
SherpaOnnxCreateOfflineDiacritization(
    const SherpaOnnxOfflineDiacritizationConfig *config);

/**
 * @brief Destroy an offline diacritization processor.
 *
 * @param diacrt A pointer returned by SherpaOnnxCreateOfflineDiacritization().
 */
SHERPA_ONNX_API void SherpaOnnxDestroyOfflineDiacritization(
    const SherpaOnnxOfflineDiacritization *diacrt);

/**
 * @brief Add diacritics to a complete input text.
 *
 * @param diacrt A pointer returned by SherpaOnnxCreateOfflineDiacritization().
 * @param text Input text without diacritics.
 * @return A newly allocated diacritized string. Free it with
 *         SherpaOfflineDiacritizationFreeText().
 */
SHERPA_ONNX_API const char *SherpaOfflineDiacritizationAddDiacritics(
    const SherpaOnnxOfflineDiacritization *diacrt, const char *text);

/**
 * @brief Free a string returned by SherpaOfflineDiacritizationAddDiacritics().
 *
 * @param text A pointer returned by SherpaOfflineDiacritizationAddDiacritics().
 */
SHERPA_ONNX_API void SherpaOfflineDiacritizationFreeText(const char *text);

#if defined(__GNUC__)
#pragma GCC diagnostic pop
#endif

#ifdef __cplusplus
} /* extern "C" */
#endif

#endif  // SHERPA_ONNX_C_API_C_API_H_
