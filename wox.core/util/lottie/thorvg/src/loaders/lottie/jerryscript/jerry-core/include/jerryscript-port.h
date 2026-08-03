/* Copyright JS Foundation and other contributors, http://js.foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#ifndef JERRYSCRIPT_PORT_H
#define JERRYSCRIPT_PORT_H

#include "jerryscript-types.h"

JERRY_C_API_BEGIN

/**
 * @defgroup jerry-port JerryScript Port API
 * @{
 */

/**
 * @defgroup jerry-port-process Process management API
 *
 * It is questionable whether a library should be able to terminate an
 * application. However, as of now, we only have the concept of completion
 * code around jerry_parse and jerry_run. Most of the other API functions
 * have no way of signaling an error. So, we keep the termination approach
 * with this port function.
 *
 * @{
 */

/**
 * Error codes that can be passed by the engine when calling jerry_port_fatal
 */
typedef enum
{
  JERRY_FATAL_OUT_OF_MEMORY = 10, /**< Out of memory */
  JERRY_FATAL_REF_COUNT_LIMIT = 12, /**< Reference count limit reached */
  JERRY_FATAL_DISABLED_BYTE_CODE = 13, /**< Executed disabled instruction */
  JERRY_FATAL_UNTERMINATED_GC_LOOPS = 14, /**< Garbage collection loop limit reached */
  JERRY_FATAL_FAILED_ASSERTION = 120 /**< Assertion failed */
} jerry_fatal_code_t;

/**
 * Signal the port that the process experienced a fatal failure from which it cannot
 * recover.
 *
 * A libc-based port may implement this with exit() or abort(), or both.
 *
 * @param code: the cause of the error.
 * @return This function is expected to not return.
 */
void JERRY_ATTR_NORETURN jerry_port_fatal (jerry_fatal_code_t code);

/**
 * Make the process sleep for a given time.
 *
 * This port function can be called by jerry-core when JERRY_DEBUGGER is enabled.
 * Otherwise this function is not used.
 *
 * @param sleep_time: milliseconds to sleep.
 */
void jerry_port_sleep (uint32_t sleep_time);

/**
 * jerry-port-process @}
 */

/**
 * @defgroup jerry-port-context External Context API
 * @{
 */

/**
 * Allocate a new context for the engine.
 *
 * This port function is called by jerry_init when JERRY_EXTERNAL_CONTEXT is enabled. Otherwise this function is not
 * used.
 *
 * The engine will pass the size required for the context structure. An implementation must make sure to
 * allocate at least this amount.
 *
 * Excess allocated space will be used as the engine heap when JerryScript is configured to use it's internal allocator,
 * this can be used to control the internal heap size.
 *
 * NOTE: The allocated memory must be pointer-aligned, otherwise the behavior is undefined.
 *
 * @param context_size: the size of the internal context structure
 *
 * @return total size of the allocated buffer
 */
size_t jerry_port_context_alloc (size_t context_size);

/**
 * Get the currently active context of the engine.
 *
 * This port function is called by jerry-core when JERRY_EXTERNAL_CONTEXT is enabled.
 * Otherwise this function is not used.
 *
 * @return the pointer to the currently used engine context.
 */
struct jerry_context_t *jerry_port_context_get (void);

/**
 * Free the currently used context.
 *
 * This port function is called by jerry_cleanup when JERRY_EXTERNAL_CONTEXT is enabled.
 * Otherwise this function is not used.
 */
void jerry_port_context_free (void);

/**
 * ThorVG-specific: Set the currently active context of the engine for cross-thread cleanup.
 * @param context_p: the pointer to the context to be set as active.

 */
void jerry_port_context_set(jerry_context_t* context_p);

/**
 * jerry-port-context @}
 */

/**
 * @defgroup jerry-port-io I/O API
 * @{
 */

/**
 * Display or log a debug/error message.
 *
 * The message is passed as a zero-terminated string. Messages may be logged in parts, which
 * will result in multiple calls to this functions. The implementation should consider
 * this before appending or prepending strings to the argument.
 *
 * This function is called with messages coming from the jerry engine as
 * the result of some abnormal operation or describing its internal operations
 * (e.g., data structure dumps or tracing info).
 *
 * The implementation can decide whether error and debug messages are logged to
 * the console, or saved to a database or to a file.
 */
void jerry_port_log (const char *message_p);

/**
 * Print a single character to standard output.
 *
 * This port function is never called from jerry-core directly, it is only used by jerry-ext components to print
 * information.
 *
 * @param byte: the byte to print.
 */
void jerry_port_print_byte (jerry_char_t byte);

/**
 * Print a buffer to standard output
 *
 * This port function is never called from jerry-core directly, it is only used by jerry-ext components to print
 * information.
 *
 * @param buffer_p: input buffer
 * @param buffer_size: data size
 */
void jerry_port_print_buffer (const jerry_char_t *buffer_p, jerry_size_t buffer_size);

/**
 * Read a line from standard input.
 *
 * The implementation should allocate storage necessary for the string. The result string should include the ending line
 * terminator character(s) and should be zero terminated.
 *
 * An implementation may return NULL to signal that the end of input is reached, or an error occurred.
 *
 * When a non-NULL value is returned, the caller will pass the returned value to `jerry_port_line_free` when the line is
 * no longer needed. This can be used to finalize dynamically allocated buffers if necessary.
 *
 * This port function is never called from jerry-core directly, it is only used by some jerry-ext components that
 * require user input.
 *
 * @param out_size_p: size of the input string in bytes, excluding terminating zero byte
 *
 * @return pointer to the buffer storing the string,
 *         or NULL if end of input
 */
jerry_char_t *jerry_port_line_read (jerry_size_t *out_size_p);

/**
 * Free a line buffer allocated by jerry_port_line_read
 *
 * @param buffer_p: buffer returned by jerry_port_line_read
 */
void jerry_port_line_free (jerry_char_t *buffer_p);

/**
 * jerry-port-io @}
 */

/**
 * @defgroup jerry-port-fd Filesystem API
 * @{
 */

/**
 * Canonicalize a file path.
 *
 * If possible, the implementation should resolve symbolic links and other directory references found in the input path,
 * and create a fully canonicalized file path as the result.
 *
 * The function may return with NULL in case an error is encountered, in which case the calling operation will not
 * proceed.
 *
 * The implementation should allocate storage for the result path as necessary. Non-NULL return values will be passed
 * to `jerry_port_path_free` when the result is no longer needed by the caller, which can be used to finalize
 * dynamically allocated buffers.
 *
 * NOTE: The implementation must not return directly with the input, as the input buffer is released after the call.
 *
 * @param path_p: zero-terminated string containing the input path
 * @param path_size: size of the input path string in bytes, excluding terminating zero
 *
 * @return buffer with the normalized path if the operation is successful,
 *         NULL otherwise
 */
jerry_char_t *jerry_port_path_normalize (const jerry_char_t *path_p, jerry_size_t path_size);

/**
 * Free a path buffer returned by jerry_port_path_normalize.
 *
 * @param path_p: the path buffer to free
 */
void jerry_port_path_free (jerry_char_t *path_p);

/**
 * Get the offset of the basename component in the input path.
 *
 * The implementation should return the offset of the first character after the last path separator found in the path.
 * This is used by the caller to split the path into a directory name and a file name.
 *
 * @param path_p: input zero-terminated path string
 *
 * @return offset of the basename component in the input path
 */
jerry_size_t jerry_port_path_base (const jerry_char_t *path_p);

/**
 * Open a source file and read the content into a buffer.
 *
 * When the source file is no longer needed by the caller, the returned pointer will be passed to
 * `jerry_port_source_free`, which can be used to finalize the buffer.
 *
 * @param file_name_p: Path that points to the source file in the filesystem.
 * @param out_size_p: The opened file's size in bytes.
 *
 * @return pointer to the buffer which contains the content of the file.
 */
jerry_char_t *jerry_port_source_read (const char *file_name_p, jerry_size_t *out_size_p);

/**
 * Free a source file buffer.
 *
 * @param buffer_p: buffer returned by jerry_port_source_read
 */
void jerry_port_source_free (jerry_char_t *buffer_p);

/**
 * jerry-port-fs @}
 */

/**
 * @defgroup jerry-port-date Date API
 * @{
 */

/**
 * Get local time zone adjustment in milliseconds for the given input time.
 *
 * The argument is a time value representing milliseconds since unix epoch.
 *
 * Ideally, this function should satisfy the stipulations applied to LocalTZA
 * in section 21.4.1.7 of the ECMAScript version 12.0, as if called with isUTC true.
 *
 * This port function can be called by jerry-core when JERRY_BUILTIN_DATE is enabled.
 * Otherwise this function is not used.
 *
 * @param unix_ms: time value in milliseconds since unix epoch
 *
 * @return local time offset in milliseconds applied to UTC for the given time value
 */
int32_t jerry_port_local_tza (double unix_ms);

/**
 * Get the current system time in UTC.
 *
 * This port function is called by jerry-core when JERRY_BUILTIN_DATE is enabled.
 * It can also be used in the implementing application to initialize the random number generator.
 *
 * @return milliseconds since Unix epoch
 */
double jerry_port_current_time (void);

/**
 * jerry-port-date @}
 */

/**
 * jerry-port @}
 */

JERRY_C_API_END

#endif /* !JERRYSCRIPT_PORT_H */

/* vim: set fdm=marker fmr=@{,@}: */
