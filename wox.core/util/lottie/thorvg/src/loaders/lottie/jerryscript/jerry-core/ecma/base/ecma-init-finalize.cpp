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

#include "ecma-init-finalize.h"

#include "ecma-builtins.h"
#include "ecma-gc.h"
#include "ecma-helpers.h"
#include "ecma-lex-env.h"
#include "ecma-literal-storage.h"

#include "jcontext.h"
#include "jmem.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmainitfinalize Initialization and finalization of ECMA components
 * @{
 */

/**
 * Maximum number of GC loops on cleanup.
 */
#define JERRY_GC_LOOP_LIMIT 100

/**
 * Initialize ECMA components
 */
void
ecma_init (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if (JERRY_GC_MARK_LIMIT != 0)
  JERRY_CONTEXT (ecma_gc_mark_recursion_limit) = JERRY_GC_MARK_LIMIT;
#endif /* (JERRY_GC_MARK_LIMIT != 0) */

  ecma_init_global_environment ();

#if JERRY_PROPERTY_HASHMAP
  JERRY_CONTEXT (ecma_prop_hashmap_alloc_state) = ECMA_PROP_HASHMAP_ALLOC_ON;
  JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_HIGH_PRESSURE_GC;
#endif /* JERRY_PROPERTY_HASHMAP */

#if (JERRY_STACK_LIMIT != 0)
  volatile int sp;
  JERRY_CONTEXT (stack_base) = (uintptr_t) &sp;
#endif /* (JERRY_STACK_LIMIT != 0) */

  ecma_job_queue_init ();

  JERRY_CONTEXT (current_new_target_p) = NULL;

#if JERRY_BUILTIN_TYPEDARRAY
  JERRY_CONTEXT (arraybuffer_compact_allocation_limit) = 256;
#endif /* JERRY_BUILTIN_TYPEDARRAY */
} /* ecma_init */

/**
 * Finalize ECMA components
 */
void
ecma_finalize (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (JERRY_CONTEXT (current_new_target_p) == NULL);

  ecma_finalize_global_environment ();
  uint8_t runs = 0;

  do
  {
    ecma_gc_run ();
    if (++runs >= JERRY_GC_LOOP_LIMIT)
    {
      jerry_fatal (JERRY_FATAL_UNTERMINATED_GC_LOOPS);
    }
  } while (JERRY_CONTEXT (ecma_gc_new_objects) != 0);

  jmem_cpointer_t *global_symbols_cp = JERRY_CONTEXT (global_symbols_cp);

  for (uint32_t i = 0; i < ECMA_BUILTIN_GLOBAL_SYMBOL_COUNT; i++)
  {
    if (global_symbols_cp[i] != JMEM_CP_NULL)
    {
      ecma_deref_ecma_string (ECMA_GET_NON_NULL_POINTER (ecma_string_t, global_symbols_cp[i]));
    }
  }

  ecma_finalize_lit_storage ();
} /* ecma_finalize */

/**
 * @}
 * @}
 */
