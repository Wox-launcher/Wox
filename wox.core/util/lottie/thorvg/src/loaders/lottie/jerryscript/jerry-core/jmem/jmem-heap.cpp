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
 * Heap implementation
 */

#include "ecma-gc.h"

#include "jcontext.h"
#include "jmem.h"
#include "jrt-bit-fields.h"
#include "jrt-libc-includes.h"

#define JMEM_ALLOCATOR_INTERNAL
#include "jmem-allocator-internal.h"

/** \addtogroup mem Memory allocation
 * @{
 *
 * \addtogroup heap Heap
 * @{
 */

#if !JERRY_SYSTEM_ALLOCATOR
/**
 * End of list marker.
 */
#define JMEM_HEAP_END_OF_LIST ((uint32_t) 0xffffffff)

/**
 * @{
 */
#ifdef ECMA_VALUE_CAN_STORE_UINTPTR_VALUE_DIRECTLY
/* In this case we simply store the pointer, since it fits anyway. */
#define JMEM_HEAP_GET_OFFSET_FROM_ADDR(p) ((uint32_t) (p))
#define JMEM_HEAP_GET_ADDR_FROM_OFFSET(u) ((jmem_heap_free_t *) (u))
#else /* !ECMA_VALUE_CAN_STORE_UINTPTR_VALUE_DIRECTLY */
/**
 * Get heap offset from address
 */
#define JMEM_HEAP_GET_OFFSET_FROM_ADDR(p) ((uint32_t) ((uint8_t *) (p) -JERRY_HEAP_CONTEXT (area)))

/**
 * Get heap address from offset
 */
#define JMEM_HEAP_GET_ADDR_FROM_OFFSET(u) ((jmem_heap_free_t *) (JERRY_HEAP_CONTEXT (area) + (u)))
#endif /* ECMA_VALUE_CAN_STORE_UINTPTR_VALUE_DIRECTLY */
/**
 * @}
 */

/**
 * Get end of region
 *
 * @return pointer to the end of the region
 */
static inline jmem_heap_free_t * JERRY_ATTR_PURE
jmem_heap_get_region_end (jmem_heap_free_t *curr_p) /**< current region */
{
  return (jmem_heap_free_t *) ((uint8_t *) curr_p + curr_p->size);
} /* jmem_heap_get_region_end */
#endif /* !JERRY_SYSTEM_ALLOCATOR */

/**
 * Startup initialization of heap
 */
void
jmem_heap_init (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if !JERRY_SYSTEM_ALLOCATOR
  JERRY_ASSERT ((uintptr_t) JERRY_HEAP_CONTEXT (area) % JMEM_ALIGNMENT == 0);

  JERRY_CONTEXT (jmem_heap_limit) = CONFIG_GC_LIMIT;

  jmem_heap_free_t *const region_p = (jmem_heap_free_t *) JERRY_HEAP_CONTEXT (area);

  region_p->size = JMEM_HEAP_AREA_SIZE;
  region_p->next_offset = JMEM_HEAP_END_OF_LIST;

  JERRY_HEAP_CONTEXT (first).size = 0;
  JERRY_HEAP_CONTEXT (first).next_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (region_p);

  JERRY_CONTEXT (jmem_heap_list_skip_p) = &JERRY_HEAP_CONTEXT (first);

  JMEM_VALGRIND_NOACCESS_SPACE (&JERRY_HEAP_CONTEXT (first), sizeof (jmem_heap_free_t));
  JMEM_VALGRIND_NOACCESS_SPACE (JERRY_HEAP_CONTEXT (area), JMEM_HEAP_AREA_SIZE);

#endif /* !JERRY_SYSTEM_ALLOCATOR */
  JMEM_HEAP_STAT_INIT ();
} /* jmem_heap_init */

/**
 * Finalize heap
 */
void
jmem_heap_finalize (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (JERRY_CONTEXT (jmem_heap_allocated_size) == 0);
#if !JERRY_SYSTEM_ALLOCATOR
  JMEM_VALGRIND_NOACCESS_SPACE (&JERRY_HEAP_CONTEXT (first), JMEM_HEAP_SIZE);
#endif /* !JERRY_SYSTEM_ALLOCATOR */
} /* jmem_heap_finalize */

/**
 * Allocation of memory region.
 *
 * See also:
 *          jmem_heap_alloc_block
 *
 * @return pointer to allocated memory block - if allocation is successful,
 *         NULL - if there is not enough memory.
 */
static void *JERRY_ATTR_HOT
jmem_heap_alloc (const size_t size) /**< size of requested block */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if !JERRY_SYSTEM_ALLOCATOR
  /* Align size. */
  const size_t required_size = ((size + JMEM_ALIGNMENT - 1) / JMEM_ALIGNMENT) * JMEM_ALIGNMENT;
  jmem_heap_free_t *data_space_p = NULL;

  JMEM_VALGRIND_DEFINED_SPACE (&JERRY_HEAP_CONTEXT (first), sizeof (jmem_heap_free_t));

  /* Fast path for 8 byte chunks, first region is guaranteed to be sufficient. */
  if (required_size == JMEM_ALIGNMENT && JERRY_LIKELY (JERRY_HEAP_CONTEXT (first).next_offset != JMEM_HEAP_END_OF_LIST))
  {
    data_space_p = JMEM_HEAP_GET_ADDR_FROM_OFFSET (JERRY_HEAP_CONTEXT (first).next_offset);
    JERRY_ASSERT (jmem_is_heap_pointer (data_space_p));

    JMEM_VALGRIND_DEFINED_SPACE (data_space_p, sizeof (jmem_heap_free_t));
    JERRY_CONTEXT (jmem_heap_allocated_size) += JMEM_ALIGNMENT;

    if (JERRY_CONTEXT (jmem_heap_allocated_size) >= JERRY_CONTEXT (jmem_heap_limit))
    {
      JERRY_CONTEXT (jmem_heap_limit) += CONFIG_GC_LIMIT;
    }

    if (data_space_p->size == JMEM_ALIGNMENT)
    {
      JERRY_HEAP_CONTEXT (first).next_offset = data_space_p->next_offset;
    }
    else
    {
      JERRY_ASSERT (data_space_p->size > JMEM_ALIGNMENT);

      jmem_heap_free_t *remaining_p;
      remaining_p = JMEM_HEAP_GET_ADDR_FROM_OFFSET (JERRY_HEAP_CONTEXT (first).next_offset) + 1;

      JMEM_VALGRIND_DEFINED_SPACE (remaining_p, sizeof (jmem_heap_free_t));
      remaining_p->size = data_space_p->size - JMEM_ALIGNMENT;
      remaining_p->next_offset = data_space_p->next_offset;
      JMEM_VALGRIND_NOACCESS_SPACE (remaining_p, sizeof (jmem_heap_free_t));

      JERRY_HEAP_CONTEXT (first).next_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (remaining_p);
    }

    JMEM_VALGRIND_NOACCESS_SPACE (data_space_p, sizeof (jmem_heap_free_t));

    if (JERRY_UNLIKELY (data_space_p == JERRY_CONTEXT (jmem_heap_list_skip_p)))
    {
      JERRY_CONTEXT (jmem_heap_list_skip_p) = JMEM_HEAP_GET_ADDR_FROM_OFFSET (JERRY_HEAP_CONTEXT (first).next_offset);
    }
  }
  /* Slow path for larger regions. */
  else
  {
    uint32_t current_offset = JERRY_HEAP_CONTEXT (first).next_offset;
    jmem_heap_free_t *prev_p = &JERRY_HEAP_CONTEXT (first);

    while (JERRY_LIKELY (current_offset != JMEM_HEAP_END_OF_LIST))
    {
      jmem_heap_free_t *current_p = JMEM_HEAP_GET_ADDR_FROM_OFFSET (current_offset);
      JERRY_ASSERT (jmem_is_heap_pointer (current_p));
      JMEM_VALGRIND_DEFINED_SPACE (current_p, sizeof (jmem_heap_free_t));

      const uint32_t next_offset = current_p->next_offset;
      JERRY_ASSERT (next_offset == JMEM_HEAP_END_OF_LIST
                    || jmem_is_heap_pointer (JMEM_HEAP_GET_ADDR_FROM_OFFSET (next_offset)));

      if (current_p->size >= required_size)
      {
        /* Region is sufficiently big, store address. */
        data_space_p = current_p;

        /* Region was larger than necessary. */
        if (current_p->size > required_size)
        {
          /* Get address of remaining space. */
          jmem_heap_free_t *const remaining_p = (jmem_heap_free_t *) ((uint8_t *) current_p + required_size);

          /* Update metadata. */
          JMEM_VALGRIND_DEFINED_SPACE (remaining_p, sizeof (jmem_heap_free_t));
          remaining_p->size = current_p->size - (uint32_t) required_size;
          remaining_p->next_offset = next_offset;
          JMEM_VALGRIND_NOACCESS_SPACE (remaining_p, sizeof (jmem_heap_free_t));

          /* Update list. */
          JMEM_VALGRIND_DEFINED_SPACE (prev_p, sizeof (jmem_heap_free_t));
          prev_p->next_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (remaining_p);
          JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
        }
        /* Block is an exact fit. */
        else
        {
          /* Remove the region from the list. */
          JMEM_VALGRIND_DEFINED_SPACE (prev_p, sizeof (jmem_heap_free_t));
          prev_p->next_offset = next_offset;
          JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
        }

        JERRY_CONTEXT (jmem_heap_list_skip_p) = prev_p;

        /* Found enough space. */
        JERRY_CONTEXT (jmem_heap_allocated_size) += required_size;

        while (JERRY_CONTEXT (jmem_heap_allocated_size) >= JERRY_CONTEXT (jmem_heap_limit))
        {
          JERRY_CONTEXT (jmem_heap_limit) += CONFIG_GC_LIMIT;
        }

        break;
      }

      JMEM_VALGRIND_NOACCESS_SPACE (current_p, sizeof (jmem_heap_free_t));
      /* Next in list. */
      prev_p = current_p;
      current_offset = next_offset;
    }
  }

  JMEM_VALGRIND_NOACCESS_SPACE (&JERRY_HEAP_CONTEXT (first), sizeof (jmem_heap_free_t));

  JERRY_ASSERT ((uintptr_t) data_space_p % JMEM_ALIGNMENT == 0);
  JMEM_VALGRIND_MALLOCLIKE_SPACE (data_space_p, size);

  return (void *) data_space_p;
#else /* JERRY_SYSTEM_ALLOCATOR */
  JERRY_CONTEXT (jmem_heap_allocated_size) += size;

  while (JERRY_CONTEXT (jmem_heap_allocated_size) >= JERRY_CONTEXT (jmem_heap_limit))
  {
    JERRY_CONTEXT (jmem_heap_limit) += CONFIG_GC_LIMIT;
  }

  return malloc (size);
#endif /* !JERRY_SYSTEM_ALLOCATOR */
} /* jmem_heap_alloc */

/**
 * Allocation of memory block, reclaiming memory if the request cannot be fulfilled.
 *
 * Note:
 *    Each failed allocation attempt tries to reclaim memory with an increasing pressure,
 *    up to 'max_pressure', or until a sufficient memory block is found. When JMEM_PRESSURE_FULL
 *    is reached, the engine is terminated with JERRY_FATAL_OUT_OF_MEMORY. The `max_pressure` argument
 *    can be used to limit the maximum pressure, and prevent the engine from terminating.
 *
 * @return NULL, if the required memory size is 0 or not enough memory
 *         pointer to the allocated memory block, if allocation is successful
 */
static void *
jmem_heap_gc_and_alloc_block (const size_t size, /**< required memory size */
                              jmem_pressure_t max_pressure) /**< pressure limit */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (JERRY_UNLIKELY (size == 0))
  {
    return NULL;
  }

  jmem_pressure_t pressure = JMEM_PRESSURE_NONE;

  if (JERRY_CONTEXT (jmem_heap_allocated_size) + size >= JERRY_CONTEXT (jmem_heap_limit))
  {
    pressure = JMEM_PRESSURE_LOW;
    ecma_free_unused_memory (pressure);
  }

  void *data_space_p = jmem_heap_alloc (size);

  while (JERRY_UNLIKELY (data_space_p == NULL) && JERRY_LIKELY (pressure < max_pressure))
  {
    auto v = (int)(pressure);
    pressure = (jmem_pressure_t)(++v);
    ecma_free_unused_memory (pressure);
    data_space_p = jmem_heap_alloc (size);
  }

  return data_space_p;
} /* jmem_heap_gc_and_alloc_block */

/**
 * Internal method for allocating a memory block.
 *
 * @return NULL, if the required memory size is 0 or not enough memory, or
 *         pointer to the allocated memory block, if allocation is successful
 */
void *JERRY_ATTR_HOT
jmem_heap_alloc_block_internal (const size_t size) /**< required memory size */
{
  return jmem_heap_gc_and_alloc_block (size, JMEM_PRESSURE_FULL);
} /* jmem_heap_alloc_block_internal */

/**
 * Allocation of memory block, reclaiming unused memory if there is not enough.
 *
 * Note:
 *      If a sufficiently sized block can't be found, the engine will be terminated with JERRY_FATAL_OUT_OF_MEMORY.
 *
 * @return NULL, if the required memory is 0
 *         pointer to allocated memory block, otherwise
 */
void *JERRY_ATTR_HOT
jmem_heap_alloc_block (const size_t size) /**< required memory size */
{
  void *block_p = jmem_heap_gc_and_alloc_block (size, JMEM_PRESSURE_FULL);
  JMEM_HEAP_STAT_ALLOC (size);
  return block_p;
} /* jmem_heap_alloc_block */

/**
 * Allocation of memory block, reclaiming unused memory if there is not enough.
 *
 * Note:
 *      If a sufficiently sized block can't be found, NULL will be returned.
 *
 * @return NULL, if the required memory size is 0
 *         also NULL, if the allocation has failed
 *         pointer to the allocated memory block, otherwise
 */
void *JERRY_ATTR_HOT
jmem_heap_alloc_block_null_on_error (const size_t size) /**< required memory size */
{
  void *block_p = jmem_heap_gc_and_alloc_block (size, JMEM_PRESSURE_HIGH);

  return block_p;
} /* jmem_heap_alloc_block_null_on_error */

#if !JERRY_SYSTEM_ALLOCATOR
/**
 * Finds the block in the free block list which precedes the argument block
 *
 * @return pointer to the preceding block
 */
static jmem_heap_free_t *
jmem_heap_find_prev (const jmem_heap_free_t *const block_p) /**< which memory block's predecessor we're looking for */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  const jmem_heap_free_t *prev_p;

  if (block_p > JERRY_CONTEXT (jmem_heap_list_skip_p))
  {
    prev_p = JERRY_CONTEXT (jmem_heap_list_skip_p);
  }
  else
  {
    prev_p = &JERRY_HEAP_CONTEXT (first);
  }

  JERRY_ASSERT (jmem_is_heap_pointer (block_p));
  const uint32_t block_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (block_p);

  JMEM_VALGRIND_DEFINED_SPACE (prev_p, sizeof (jmem_heap_free_t));
  /* Find position of region in the list. */
  while (prev_p->next_offset < block_offset)
  {
    const jmem_heap_free_t *const next_p = JMEM_HEAP_GET_ADDR_FROM_OFFSET (prev_p->next_offset);
    JERRY_ASSERT (jmem_is_heap_pointer (next_p));

    JMEM_VALGRIND_DEFINED_SPACE (next_p, sizeof (jmem_heap_free_t));
    JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
    prev_p = next_p;
  }

  JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
  return (jmem_heap_free_t *) prev_p;
} /* jmem_heap_find_prev */

/**
 * Inserts the block into the free chain after a specified block.
 *
 * Note:
 *     'jmem_heap_find_prev' can and should be used to find the previous free block
 */
static void
jmem_heap_insert_block (jmem_heap_free_t *block_p, /**< block to insert */
                        jmem_heap_free_t *prev_p, /**< the free block after which to insert 'block_p' */
                        const size_t size) /**< size of the inserted block */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT ((uintptr_t) block_p % JMEM_ALIGNMENT == 0);
  JERRY_ASSERT (size % JMEM_ALIGNMENT == 0);

  JMEM_VALGRIND_NOACCESS_SPACE (block_p, size);

  JMEM_VALGRIND_DEFINED_SPACE (prev_p, sizeof (jmem_heap_free_t));
  jmem_heap_free_t *next_p = JMEM_HEAP_GET_ADDR_FROM_OFFSET (prev_p->next_offset);
  JMEM_VALGRIND_DEFINED_SPACE (block_p, sizeof (jmem_heap_free_t));
  JMEM_VALGRIND_DEFINED_SPACE (next_p, sizeof (jmem_heap_free_t));

  const uint32_t block_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (block_p);

  /* Update prev. */
  if (jmem_heap_get_region_end (prev_p) == block_p)
  {
    /* Can be merged. */
    prev_p->size += (uint32_t) size;
    JMEM_VALGRIND_NOACCESS_SPACE (block_p, sizeof (jmem_heap_free_t));
    block_p = prev_p;
  }
  else
  {
    block_p->size = (uint32_t) size;
    prev_p->next_offset = block_offset;
  }

  /* Update next. */
  if (jmem_heap_get_region_end (block_p) == next_p)
  {
    /* Can be merged. */
    block_p->size += next_p->size;
    block_p->next_offset = next_p->next_offset;
  }
  else
  {
    block_p->next_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (next_p);
  }

  JERRY_CONTEXT (jmem_heap_list_skip_p) = prev_p;

  JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
  JMEM_VALGRIND_NOACCESS_SPACE (block_p, sizeof (jmem_heap_free_t));
  JMEM_VALGRIND_NOACCESS_SPACE (next_p, sizeof (jmem_heap_free_t));
} /* jmem_heap_insert_block */
#endif /* !JERRY_SYSTEM_ALLOCATOR */

/**
 * Internal method for freeing a memory block.
 *
 * @return void
 */
void JERRY_ATTR_HOT
jmem_heap_free_block_internal (void *ptr, /**< pointer to beginning of data space of the block */
                               const size_t size /**< size of allocated region */)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (size > 0);
  JERRY_ASSERT (JERRY_CONTEXT (jmem_heap_limit) >= JERRY_CONTEXT (jmem_heap_allocated_size));
  JERRY_ASSERT (JERRY_CONTEXT (jmem_heap_allocated_size) > 0);

#if !JERRY_SYSTEM_ALLOCATOR
  /* checking that ptr points to the heap */
  JERRY_ASSERT (jmem_is_heap_pointer (ptr));
  JERRY_ASSERT ((uintptr_t) ptr % JMEM_ALIGNMENT == 0);

  const size_t aligned_size = (size + JMEM_ALIGNMENT - 1) / JMEM_ALIGNMENT * JMEM_ALIGNMENT;

  jmem_heap_free_t *const block_p = (jmem_heap_free_t *) ptr;
  jmem_heap_free_t *const prev_p = jmem_heap_find_prev (block_p);
  jmem_heap_insert_block (block_p, prev_p, aligned_size);

  JERRY_CONTEXT (jmem_heap_allocated_size) -= aligned_size;

  JMEM_VALGRIND_FREELIKE_SPACE (ptr);
#else /* JERRY_SYSTEM_ALLOCATOR */
  JERRY_CONTEXT (jmem_heap_allocated_size) -= size;
  free (ptr);
#endif /* !JERRY_SYSTEM_ALLOCATOR */
  while (JERRY_CONTEXT (jmem_heap_allocated_size) + CONFIG_GC_LIMIT <= JERRY_CONTEXT (jmem_heap_limit))
  {
    JERRY_CONTEXT (jmem_heap_limit) -= CONFIG_GC_LIMIT;
  }

  JERRY_ASSERT (JERRY_CONTEXT (jmem_heap_limit) >= JERRY_CONTEXT (jmem_heap_allocated_size));
} /* jmem_heap_free_block_internal */

/**
 * Reallocates the memory region pointed to by 'ptr', changing the size of the allocated region.
 *
 * @return pointer to the reallocated region
 */
void *JERRY_ATTR_HOT
jmem_heap_realloc_block (void *ptr, /**< memory region to reallocate */
                         const size_t old_size, /**< current size of the region */
                         const size_t new_size) /**< desired new size */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if !JERRY_SYSTEM_ALLOCATOR
  JERRY_ASSERT (jmem_is_heap_pointer (ptr));
  JERRY_ASSERT ((uintptr_t) ptr % JMEM_ALIGNMENT == 0);
  JERRY_ASSERT (old_size != 0);
  JERRY_ASSERT (new_size != 0);

  jmem_heap_free_t *const block_p = (jmem_heap_free_t *) ptr;
  const size_t aligned_new_size = (new_size + JMEM_ALIGNMENT - 1) / JMEM_ALIGNMENT * JMEM_ALIGNMENT;
  const size_t aligned_old_size = (old_size + JMEM_ALIGNMENT - 1) / JMEM_ALIGNMENT * JMEM_ALIGNMENT;

  if (aligned_old_size == aligned_new_size)
  {
    JMEM_VALGRIND_RESIZE_SPACE (block_p, old_size, new_size);
    JMEM_HEAP_STAT_FREE (old_size);
    JMEM_HEAP_STAT_ALLOC (new_size);
    return block_p;
  }

  if (aligned_new_size < aligned_old_size)
  {
    JMEM_VALGRIND_RESIZE_SPACE (block_p, old_size, new_size);
    JMEM_HEAP_STAT_FREE (old_size);
    JMEM_HEAP_STAT_ALLOC (new_size);
    jmem_heap_insert_block ((jmem_heap_free_t *) ((uint8_t *) block_p + aligned_new_size),
                            jmem_heap_find_prev (block_p),
                            aligned_old_size - aligned_new_size);

    JERRY_CONTEXT (jmem_heap_allocated_size) -= (aligned_old_size - aligned_new_size);
    while (JERRY_CONTEXT (jmem_heap_allocated_size) + CONFIG_GC_LIMIT <= JERRY_CONTEXT (jmem_heap_limit))
    {
      JERRY_CONTEXT (jmem_heap_limit) -= CONFIG_GC_LIMIT;
    }

    return block_p;
  }

  void *ret_block_p = NULL;
  const size_t required_size = aligned_new_size - aligned_old_size;

  if (JERRY_CONTEXT (jmem_heap_allocated_size) + required_size >= JERRY_CONTEXT (jmem_heap_limit))
  {
    ecma_free_unused_memory (JMEM_PRESSURE_LOW);
  }

  jmem_heap_free_t *prev_p = jmem_heap_find_prev (block_p);
  JMEM_VALGRIND_DEFINED_SPACE (prev_p, sizeof (jmem_heap_free_t));
  jmem_heap_free_t *const next_p = JMEM_HEAP_GET_ADDR_FROM_OFFSET (prev_p->next_offset);

  /* Check if block can be extended at the end */
  if (((jmem_heap_free_t *) ((uint8_t *) block_p + aligned_old_size)) == next_p)
  {
    JMEM_VALGRIND_DEFINED_SPACE (next_p, sizeof (jmem_heap_free_t));

    if (required_size <= next_p->size)
    {
      /* Block can be extended, update the list. */
      if (required_size == next_p->size)
      {
        prev_p->next_offset = next_p->next_offset;
      }
      else
      {
        jmem_heap_free_t *const new_next_p = (jmem_heap_free_t *) ((uint8_t *) next_p + required_size);
        JMEM_VALGRIND_DEFINED_SPACE (new_next_p, sizeof (jmem_heap_free_t));
        new_next_p->next_offset = next_p->next_offset;
        new_next_p->size = (uint32_t) (next_p->size - required_size);
        JMEM_VALGRIND_NOACCESS_SPACE (new_next_p, sizeof (jmem_heap_free_t));
        prev_p->next_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (new_next_p);
      }

      /* next_p will be marked as undefined space. */
      JMEM_VALGRIND_RESIZE_SPACE (block_p, old_size, new_size);
      ret_block_p = block_p;
    }
    else
    {
      JMEM_VALGRIND_NOACCESS_SPACE (next_p, sizeof (jmem_heap_free_t));
    }

    JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
  }
  /*
   * Check if block can be extended at the front.
   * This is less optimal because we need to copy the data, but still better than allocating a new block.
   */
  else if (jmem_heap_get_region_end (prev_p) == block_p)
  {
    if (required_size <= prev_p->size)
    {
      if (required_size == prev_p->size)
      {
        JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
        prev_p = jmem_heap_find_prev (prev_p);
        JMEM_VALGRIND_DEFINED_SPACE (prev_p, sizeof (jmem_heap_free_t));
        prev_p->next_offset = JMEM_HEAP_GET_OFFSET_FROM_ADDR (next_p);
      }
      else
      {
        prev_p->size = (uint32_t) (prev_p->size - required_size);
      }

      JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));

      ret_block_p = (uint8_t *) block_p - required_size;

      /* Mark the new block as undefined so that we are able to write to it. */
      JMEM_VALGRIND_UNDEFINED_SPACE (ret_block_p, old_size);
      /* The blocks are likely to overlap, so mark the old block as defined memory again. */
      JMEM_VALGRIND_DEFINED_SPACE (block_p, old_size);
      memmove (ret_block_p, block_p, old_size);

      JMEM_VALGRIND_FREELIKE_SPACE (block_p);
      JMEM_VALGRIND_MALLOCLIKE_SPACE (ret_block_p, new_size);
      JMEM_VALGRIND_DEFINED_SPACE (ret_block_p, old_size);
    }
    else
    {
      JMEM_VALGRIND_NOACCESS_SPACE (prev_p, sizeof (jmem_heap_free_t));
    }
  }

  if (ret_block_p != NULL)
  {
    /* Managed to extend the block. Update memory usage and the skip pointer. */
    JERRY_CONTEXT (jmem_heap_list_skip_p) = prev_p;
    JERRY_CONTEXT (jmem_heap_allocated_size) += required_size;

    while (JERRY_CONTEXT (jmem_heap_allocated_size) >= JERRY_CONTEXT (jmem_heap_limit))
    {
      JERRY_CONTEXT (jmem_heap_limit) += CONFIG_GC_LIMIT;
    }
  }
  else
  {
    /* Could not extend block. Allocate new region and copy the data. */
    /* jmem_heap_alloc_block_internal will adjust the allocated_size, but insert_block will not,
       so we reduce it here first, so that the limit calculation remains consistent. */
    JERRY_CONTEXT (jmem_heap_allocated_size) -= aligned_old_size;
    ret_block_p = jmem_heap_alloc_block_internal (new_size);

    /* jmem_heap_alloc_block_internal may trigger garbage collection, which can create new free blocks
     * in the heap structure, so we need to look up the previous block again. */
    prev_p = jmem_heap_find_prev (block_p);

    memcpy (ret_block_p, block_p, old_size);
    jmem_heap_insert_block (block_p, prev_p, aligned_old_size);
    /* jmem_heap_alloc_block_internal will call JMEM_VALGRIND_MALLOCLIKE_SPACE */
    JMEM_VALGRIND_FREELIKE_SPACE (block_p);
  }

  JMEM_HEAP_STAT_FREE (old_size);
  JMEM_HEAP_STAT_ALLOC (new_size);
  return ret_block_p;
#else /* JERRY_SYSTEM_ALLOCATOR */
  const size_t required_size = new_size - old_size;

  if (JERRY_CONTEXT (jmem_heap_allocated_size) + required_size >= JERRY_CONTEXT (jmem_heap_limit))
  {
    ecma_free_unused_memory (JMEM_PRESSURE_LOW);
  }

  JERRY_CONTEXT (jmem_heap_allocated_size) += required_size;

  while (JERRY_CONTEXT (jmem_heap_allocated_size) >= JERRY_CONTEXT (jmem_heap_limit))
  {
    JERRY_CONTEXT (jmem_heap_limit) += CONFIG_GC_LIMIT;
  }

  while (JERRY_CONTEXT (jmem_heap_allocated_size) + CONFIG_GC_LIMIT <= JERRY_CONTEXT (jmem_heap_limit))
  {
    JERRY_CONTEXT (jmem_heap_limit) -= CONFIG_GC_LIMIT;
  }

  JMEM_HEAP_STAT_FREE (old_size);
  JMEM_HEAP_STAT_ALLOC (new_size);
  return realloc (ptr, new_size);
#endif /* !JERRY_SYSTEM_ALLOCATOR */
} /* jmem_heap_realloc_block */

/**
 * Free memory block
 *
 * @return void
 */
void JERRY_ATTR_HOT
jmem_heap_free_block (void *ptr, /**< pointer to beginning of data space of the block */
                      const size_t size) /**< size of allocated region */
{
  jmem_heap_free_block_internal (ptr, size);
  JMEM_HEAP_STAT_FREE (size);
  return;
} /* jmem_heap_free_block */

#ifndef JERRY_NDEBUG
/**
 * Check whether the pointer points to the heap
 *
 * Note:
 *      the routine should be used only for assertion checks
 *
 * @return true - if pointer points to the heap,
 *         false - otherwise
 */
bool
jmem_is_heap_pointer (const void *pointer) /**< pointer */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if !JERRY_SYSTEM_ALLOCATOR
  return ((uint8_t *) pointer >= JERRY_HEAP_CONTEXT (area)
          && (uint8_t *) pointer <= (JERRY_HEAP_CONTEXT (area) + JMEM_HEAP_AREA_SIZE));
#else /* JERRY_SYSTEM_ALLOCATOR */
  JERRY_UNUSED (pointer);
  return true;
#endif /* !JERRY_SYSTEM_ALLOCATOR */
} /* jmem_is_heap_pointer */
#endif /* !JERRY_NDEBUG */


/**
 * @}
 * @}
 */
