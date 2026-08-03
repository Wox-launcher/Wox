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

#include "re-compiler.h"

#include "ecma-exceptions.h"
#include "ecma-helpers.h"
#include "ecma-regexp-object.h"

#include "jcontext.h"
#include "jmem.h"
#include "jrt-libc-includes.h"
#include "lit-char-helpers.h"
#include "re-bytecode.h"
#include "re-compiler-context.h"
#include "re-parser.h"

#if JERRY_BUILTIN_REGEXP

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup regexparser Regular expression
 * @{
 *
 * \addtogroup regexparser_compiler Compiler
 * @{
 */

/**
 * Search for the given pattern in the RegExp cache.
 *
 * @return pointer to bytecode if found
 *         NULL - otherwise
 */
static re_compiled_code_t *
re_cache_lookup (ecma_string_t *pattern_str_p, /**< pattern string */
                 uint16_t flags) /**< flags */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  re_compiled_code_t **cache_p = JERRY_CONTEXT (re_cache);

  for (uint8_t idx = 0u; idx < RE_CACHE_SIZE; idx++)
  {
    re_compiled_code_t *cached_bytecode_p = cache_p[idx];

    if (cached_bytecode_p == NULL)
    {
      break;
    }

    ecma_string_t *cached_pattern_str_p = ecma_get_string_from_value (cached_bytecode_p->source);

    if (cached_bytecode_p->header.status_flags == flags
        && ecma_compare_ecma_strings (cached_pattern_str_p, pattern_str_p))
    {
      return cached_bytecode_p;
    }
  }

  return NULL;
} /* re_cache_lookup */

/**
 * Run garbage collection in RegExp cache.
 */
void
re_cache_gc (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  re_compiled_code_t **cache_p = JERRY_CONTEXT (re_cache);

  for (uint32_t i = 0u; i < RE_CACHE_SIZE; i++)
  {
    const re_compiled_code_t *cached_bytecode_p = cache_p[i];

    if (cached_bytecode_p == NULL)
    {
      break;
    }

    ecma_bytecode_deref ((ecma_compiled_code_t *) cached_bytecode_p);
    cache_p[i] = NULL;
  }

  JERRY_CONTEXT (re_cache_idx) = 0;
} /* re_cache_gc */

/**
 * Compilation of RegExp bytecode
 *
 * @return pointer to bytecode if compilation was successful
 *         NULL - otherwise
 */
re_compiled_code_t *
re_compile_bytecode (ecma_string_t *pattern_str_p, /**< pattern */
                     uint16_t flags) /**< flags */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  re_compiled_code_t *cached_bytecode_p = re_cache_lookup (pattern_str_p, flags);

  if (cached_bytecode_p != NULL)
  {
    ecma_bytecode_ref ((ecma_compiled_code_t *) cached_bytecode_p);
    return cached_bytecode_p;
  }

  re_compiler_ctx_t re_ctx;
  re_ctx.flags = flags;
  re_ctx.captures_count = 1;
  re_ctx.non_captures_count = 0;

  re_initialize_regexp_bytecode (&re_ctx);

  ECMA_STRING_TO_UTF8_STRING (pattern_str_p, pattern_start_p, pattern_start_size);

  re_ctx.input_start_p = pattern_start_p;
  re_ctx.input_curr_p = (lit_utf8_byte_t *) pattern_start_p;
  re_ctx.input_end_p = pattern_start_p + pattern_start_size;
  re_ctx.groups_count = -1;

  /* Parse RegExp pattern */
  ecma_value_t result = re_parse_alternative (&re_ctx, true);

  ECMA_FINALIZE_UTF8_STRING (pattern_start_p, pattern_start_size);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    /* Compilation failed, free bytecode. */
    jmem_heap_free_block (re_ctx.bytecode_start_p, re_ctx.bytecode_size);
    return NULL;
  }

  /* Align bytecode size to JMEM_ALIGNMENT so that it can be stored in the bytecode header. */
  const uint32_t final_size = JERRY_ALIGNUP (re_ctx.bytecode_size, JMEM_ALIGNMENT);
  re_compiled_code_t *re_compiled_code_p =
    (re_compiled_code_t *) jmem_heap_realloc_block (re_ctx.bytecode_start_p, re_ctx.bytecode_size, final_size);

  /* Bytecoded will be inserted into the cache and returned to the caller, so refcount is implicitly set to 2. */
  re_compiled_code_p->header.refs = 2;
  re_compiled_code_p->header.size = (uint16_t) (final_size >> JMEM_ALIGNMENT_LOG);
  re_compiled_code_p->header.status_flags = re_ctx.flags;

  ecma_ref_ecma_string (pattern_str_p);
  re_compiled_code_p->source = ecma_make_string_value (pattern_str_p);
  re_compiled_code_p->captures_count = re_ctx.captures_count;
  re_compiled_code_p->non_captures_count = re_ctx.non_captures_count;

#if JERRY_REGEXP_DUMP_BYTE_CODE
  if (JERRY_CONTEXT (jerry_init_flags) & JERRY_INIT_SHOW_REGEXP_OPCODES)
  {
    re_dump_bytecode (&re_ctx);
  }
#endif /* JERRY_REGEXP_DUMP_BYTE_CODE */

  uint8_t cache_idx = JERRY_CONTEXT (re_cache_idx);

  if (JERRY_CONTEXT (re_cache)[cache_idx] != NULL)
  {
    ecma_bytecode_deref ((ecma_compiled_code_t *) JERRY_CONTEXT (re_cache)[cache_idx]);
  }

  JERRY_CONTEXT (re_cache)[cache_idx] = re_compiled_code_p;
  JERRY_CONTEXT (re_cache_idx) = (uint8_t) (cache_idx + 1) % RE_CACHE_SIZE;

  return re_compiled_code_p;
} /* re_compile_bytecode */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
