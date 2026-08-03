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

#include "jcontext.h"

/** \addtogroup context Context
 * @{
 */

/**
 * Check the existence of the ECMA_STATUS_EXCEPTION flag.
 *
 * @return true - if the flag is set
 *         false - otherwise
 */
bool
jcontext_has_pending_exception (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  return JERRY_CONTEXT (status_flags) & ECMA_STATUS_EXCEPTION;
} /* jcontext_has_pending_exception */

/**
 * Check the existence of the ECMA_STATUS_ABORT flag.
 *
 * @return true - if the flag is set
 *         false - otherwise
 */
bool
jcontext_has_pending_abort (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  return JERRY_CONTEXT (status_flags) & ECMA_STATUS_ABORT;
} /* jcontext_has_pending_abort */

/**
 * Set the abort flag for the context.
 *
 * @return void
 */
void
jcontext_set_abort_flag (bool is_abort) /**< true - if the abort flag should be set
                                         *   false - if the abort flag should be removed */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (jcontext_has_pending_exception ());

  if (is_abort)
  {
    JERRY_CONTEXT (status_flags) |= ECMA_STATUS_ABORT;
  }
  else
  {
    JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_ABORT;
  }
} /* jcontext_set_abort_flag */

/**
 * Set the exception flag for the context.
 *
 * @return void
 */
void
jcontext_set_exception_flag (bool is_exception) /**< true - if the exception flag should be set
                                                 *   false - if the exception flag should be removed */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (is_exception)
  {
    JERRY_CONTEXT (status_flags) |= ECMA_STATUS_EXCEPTION;
  }
  else
  {
    JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_EXCEPTION;
  }
} /* jcontext_set_exception_flag */

/**
 * Raise exception from the given error value.
 *
 * @return void
 */
void
jcontext_raise_exception (ecma_value_t error) /**< error to raise */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (!jcontext_has_pending_exception ());
  JERRY_ASSERT (!jcontext_has_pending_abort ());

  JERRY_CONTEXT (error_value) = error;
  jcontext_set_exception_flag (true);
} /* jcontext_raise_exception */

/**
 * Release the current exception/abort of the context.
 */
void
jcontext_release_exception (void)
{
  JERRY_ASSERT (jcontext_has_pending_exception ());

  ecma_free_value (jcontext_take_exception ());
} /* jcontext_release_exception */

/**
 * Take the current exception/abort of context.
 *
 * @return current exception as an ecma-value
 */
ecma_value_t
jcontext_take_exception (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (jcontext_has_pending_exception ());

  JERRY_CONTEXT (status_flags) &= (uint32_t) ~(ECMA_STATUS_EXCEPTION
#if JERRY_VM_THROW
                                               | ECMA_STATUS_ERROR_THROWN
#endif /* JERRY_VM_THROW */
                                               | ECMA_STATUS_ABORT);
  return JERRY_CONTEXT (error_value);
} /* jcontext_take_exception */

#if !JERRY_EXTERNAL_CONTEXT

/**
 * Global context.
 */
jerry_context_t jerry_global_context;

#if !JERRY_SYSTEM_ALLOCATOR

/**
 * Check size of heap is corresponding to configuration
 */
JERRY_STATIC_ASSERT ((sizeof (jmem_heap_t) <= JMEM_HEAP_SIZE),
                     size_of_mem_heap_must_be_less_than_or_equal_to_JMEM_HEAP_SIZE);

/**
 * Global heap.
 */
jmem_heap_t jerry_global_heap JERRY_ATTR_ALIGNED (JMEM_ALIGNMENT) JERRY_ATTR_GLOBAL_HEAP;

#endif /* !JERRY_SYSTEM_ALLOCATOR */

#endif /* !JERRY_EXTERNAL_CONTEXT */

/**
 * @}
 */
