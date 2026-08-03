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
 * Memory pool manager implementation
 */

#include "jcontext.h"
#include "jmem.h"
#include "jrt-libc-includes.h"

#define JMEM_ALLOCATOR_INTERNAL
#include "jmem-allocator-internal.h"

/** \addtogroup mem Memory allocation
 * @{
 *
 * \addtogroup poolman Memory pool manager
 * @{
 */

/**
 * Finalize pool manager
 */
void
jmem_pools_finalize (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  jmem_pools_collect_empty ();

  JERRY_ASSERT (JERRY_CONTEXT (jmem_free_8_byte_chunk_p) == NULL);
} /* jmem_pools_finalize */

/**
 * Allocate a chunk of specified size
 *
 * @return pointer to allocated chunk, if allocation was successful,
 *         or NULL - if not enough memory.
 */
void *JERRY_ATTR_HOT
jmem_pools_alloc (size_t size) /**< size of the chunk */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (size <= 8);

  if (JERRY_CONTEXT (jmem_free_8_byte_chunk_p) != NULL)
    {
      const jmem_pools_chunk_t *const chunk_p = JERRY_CONTEXT (jmem_free_8_byte_chunk_p);

      JMEM_VALGRIND_DEFINED_SPACE (chunk_p, sizeof (jmem_pools_chunk_t));
      JERRY_CONTEXT (jmem_free_8_byte_chunk_p) = chunk_p->next_p;
      JMEM_VALGRIND_UNDEFINED_SPACE (chunk_p, sizeof (jmem_pools_chunk_t));

      JMEM_HEAP_STAT_ALLOC (8);
      return (void *) chunk_p;
    }
    else
    {
      void *chunk_p = jmem_heap_alloc_block_internal (8);
      JMEM_HEAP_STAT_ALLOC (8);
      return chunk_p;
    }
} /* jmem_pools_alloc */

/**
 * Free the chunk
 *
 * @return void
 */
void JERRY_ATTR_HOT
jmem_pools_free (void *chunk_p, /**< pointer to the chunk */
                 size_t size) /**< size of the chunk */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (chunk_p != NULL);
  JMEM_HEAP_STAT_FREE (size);

  jmem_pools_chunk_t *const chunk_to_free_p = (jmem_pools_chunk_t *) chunk_p;

  JMEM_VALGRIND_DEFINED_SPACE (chunk_to_free_p, size);

  JERRY_ASSERT (size <= 8);

  chunk_to_free_p->next_p = JERRY_CONTEXT (jmem_free_8_byte_chunk_p);
  JERRY_CONTEXT (jmem_free_8_byte_chunk_p) = chunk_to_free_p;

  JMEM_VALGRIND_NOACCESS_SPACE (chunk_to_free_p, size);
} /* jmem_pools_free */

/**
 *  Collect empty pool chunks
 */
void
jmem_pools_collect_empty (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  jmem_pools_chunk_t *chunk_p = JERRY_CONTEXT (jmem_free_8_byte_chunk_p);
  JERRY_CONTEXT (jmem_free_8_byte_chunk_p) = NULL;

  while (chunk_p)
  {
    JMEM_VALGRIND_DEFINED_SPACE (chunk_p, sizeof (jmem_pools_chunk_t));
    jmem_pools_chunk_t *const next_p = chunk_p->next_p;
    JMEM_VALGRIND_NOACCESS_SPACE (chunk_p, sizeof (jmem_pools_chunk_t));

    jmem_heap_free_block_internal (chunk_p, 8);
    chunk_p = next_p;
  }
} /* jmem_pools_collect_empty */

/**
 * @}
 * @}
 */
