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

#include "jrt.h"

static jerry_log_level_t jerry_log_level = JERRY_LOG_LEVEL_ERROR;

/**
 * Get current log level
 *
 * @return log level
 */
jerry_log_level_t
jerry_jrt_get_log_level (void)
{
  return jerry_log_level;
} /* jerry_jrt_get_log_level */

/**
 * Set log level
 *
 * @param level: new log level
 */
void
jerry_jrt_set_log_level (jerry_log_level_t level)
{
  jerry_log_level = level;
} /* jerry_jrt_set_log_level */
