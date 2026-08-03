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

/**
 * Implementation of exit with specified status code.
 */

#include "jrt-libc-includes.h"
#include "jrt.h"

/*
 * Exit with specified status code.
 *
 * If !JERRY_NDEBUG and code != 0, print status code with description
 * and call assertion fail handler.
 */
void JERRY_ATTR_NORETURN
jerry_fatal (jerry_fatal_code_t code) /**< status code */
{
#ifndef JERRY_NDEBUG
  switch (code)
  {
    case JERRY_FATAL_OUT_OF_MEMORY:
    {
      JERRY_ERROR_MSG ("Error: JERRY_FATAL_OUT_OF_MEMORY\n");
      break;
    }
    case JERRY_FATAL_REF_COUNT_LIMIT:
    {
      JERRY_ERROR_MSG ("Error: JERRY_FATAL_REF_COUNT_LIMIT\n");
      break;
    }
    case JERRY_FATAL_UNTERMINATED_GC_LOOPS:
    {
      JERRY_ERROR_MSG ("Error: JERRY_FATAL_UNTERMINATED_GC_LOOPS\n");
      break;
    }
    case JERRY_FATAL_DISABLED_BYTE_CODE:
    {
      JERRY_ERROR_MSG ("Error: JERRY_FATAL_DISABLED_BYTE_CODE\n");
      break;
    }
    case JERRY_FATAL_FAILED_ASSERTION:
    {
      JERRY_ERROR_MSG ("Error: JERRY_FATAL_FAILED_ASSERTION\n");
      break;
    }
  }
#endif /* !JERRY_NDEBUG */

  /* to make compiler happy for some RTOS: 'control reaches end of non-void function' */
  while (true)
  {
  }
} /* jerry_fatal */

#ifndef JERRY_NDEBUG
/**
 * Handle failed assertion
 */
void JERRY_ATTR_NORETURN
jerry_assert_fail (const char *assertion, /**< assertion condition string */
                   const char *file, /**< file name */
                   const char *function, /**< function name */
                   const uint32_t line) /**< line */
{
  JERRY_ERROR_MSG ("ICE: Assertion '%s' failed at %s(%s):%u.\n", assertion, file, function, line);

  jerry_fatal (JERRY_FATAL_FAILED_ASSERTION);
} /* jerry_assert_fail */

/**
 * Handle execution of control path that should be unreachable
 */
void JERRY_ATTR_NORETURN
jerry_unreachable (const char *file, /**< file name */
                   const char *function, /**< function name */
                   const uint32_t line) /**< line */
{
  JERRY_ERROR_MSG ("ICE: Unreachable control path at %s(%s):%u was executed.\n", file, function, line);

  jerry_fatal (JERRY_FATAL_FAILED_ASSERTION);
} /* jerry_unreachable */
#endif /* !JERRY_NDEBUG */
