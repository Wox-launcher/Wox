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

#ifndef JMEM_H
#define JMEM_H

#include "jrt.h"

/** \addtogroup mem Memory allocation
 * @{
 *
 * \addtogroup heap Heap
 * @{
 */

/**
 * Logarithm of required alignment for allocated units/blocks
 */
#define JMEM_ALIGNMENT_LOG 3

/**
 * Representation of NULL value for compressed pointers
 */
#define JMEM_CP_NULL ((jmem_cpointer_t) 0)

/**
 * Required alignment for allocated units/blocks
 */
#define JMEM_ALIGNMENT (1u << JMEM_ALIGNMENT_LOG)

/**
 * Pointer value can be directly stored without compression
 */
#if UINTPTR_MAX <= UINT32_MAX
#define JMEM_CAN_STORE_POINTER_VALUE_DIRECTLY
#endif /* UINTPTR_MAX <= UINT32_MAX */

/**
 * Mask for tag part in jmem_cpointer_tag_t
 */
#define JMEM_TAG_MASK 0x7u

/**
 * Shift for tag part in jmem_cpointer_tag_t
 */
#define JMEM_TAG_SHIFT 3

/**
 * Bit mask for tag part in jmem_cpointer_tag_t
 */
enum
{
  JMEM_FIRST_TAG_BIT_MASK = (1u << 0), /**< first tag bit mask **/
  JMEM_SECOND_TAG_BIT_MASK = (1u << 1), /**< second tag bit mask **/
  JMEM_THIRD_TAG_BIT_MASK = (1u << 2), /**< third tag bit mask **/
};

/**
 * Compressed pointer representations
 *
 * 16 bit representation:
 *   The jmem_cpointer_t is defined as uint16_t
 *   and it can contain any sixteen bit value.
 *
 * 32 bit representation:
 *   The jmem_cpointer_t is defined as uint32_t.
 *   The lower JMEM_ALIGNMENT_LOG bits must be zero.
 *   The other bits can have any value.
 *
 * The 16 bit representation always encodes an offset from
 * a heap base. The 32 bit representation currently encodes
 * raw 32 bit JMEM_ALIGNMENT aligned pointers on 32 bit systems.
 * This can be extended to encode a 32 bit offset from a heap
 * base on 64 bit systems in the future. There are no plans
 * to support more than 4G address space for JerryScript.
 */

/**
 * Compressed pointer
 */
typedef uint16_t jmem_cpointer_t;

/**
 * Compressed pointer with tag value
 */
typedef uint32_t jmem_cpointer_tag_t;

/**
 * Memory usage pressure for reclaiming unused memory.
 *
 * Each failed allocation will try to reclaim memory with increasing pressure,
 * until enough memory is freed to fulfill the allocation request.
 *
 * If not enough memory is freed and JMEM_PRESSURE_FULL is reached,
 * then the engine is shut down with JERRY_FATAL_OUT_OF_MEMORY.
 */
typedef enum
{
  JMEM_PRESSURE_NONE, /**< no memory pressure */
  JMEM_PRESSURE_LOW, /**< low memory pressure */
  JMEM_PRESSURE_HIGH, /**< high memory pressure */
  JMEM_PRESSURE_FULL, /**< memory full */
} jmem_pressure_t;

/**
 * Node for free chunk list
 */
typedef struct jmem_pools_chunk_t
{
  struct jmem_pools_chunk_t *next_p; /**< pointer to next pool chunk */
} jmem_pools_chunk_t;

/**
 *  Free region node
 */
typedef struct
{
  uint32_t next_offset; /**< Offset of next region in list */
  uint32_t size; /**< Size of region */
} jmem_heap_free_t;

void jmem_init (void);
void jmem_finalize (void);

void *jmem_heap_alloc_block (const size_t size);
void *jmem_heap_alloc_block_null_on_error (const size_t size);
void *jmem_heap_realloc_block (void *ptr, const size_t old_size, const size_t new_size);
void jmem_heap_free_block (void *ptr, const size_t size);


jmem_cpointer_t JERRY_ATTR_PURE jmem_compress_pointer (const void *pointer_p);
void *JERRY_ATTR_PURE jmem_decompress_pointer (uintptr_t compressed_pointer);

/**
 * Define a local array variable and allocate memory for the array on the heap.
 *
 * If requested number of elements is zero, assign NULL to the variable.
 *
 * Warning:
 *         if there is not enough memory on the heap, shutdown engine with JERRY_FATAL_OUT_OF_MEMORY.
 */
#define JMEM_DEFINE_LOCAL_ARRAY(var_name, number, type)           \
  {                                                               \
    size_t var_name##___size = (size_t) (number) * sizeof (type); \
    type *var_name = (type *) (jmem_heap_alloc_block (var_name##___size));

/**
 * Free the previously defined local array variable, freeing corresponding block on the heap,
 * if it was allocated (i.e. if the array's size was non-zero).
 */
#define JMEM_FINALIZE_LOCAL_ARRAY(var_name)             \
  if (var_name != NULL)                                 \
  {                                                     \
    JERRY_ASSERT (var_name##___size != 0);              \
                                                        \
    jmem_heap_free_block (var_name, var_name##___size); \
  }                                                     \
  else                                                  \
  {                                                     \
    JERRY_ASSERT (var_name##___size == 0);              \
  }                                                     \
  }

/**
 * Get value of pointer from specified non-null compressed pointer value
 */
#define JMEM_CP_GET_NON_NULL_POINTER(type, cp_value) ((type *) (jmem_decompress_pointer (cp_value)))

/**
 * Get value of pointer from specified compressed pointer value
 */
#define JMEM_CP_GET_POINTER(type, cp_value) \
  (((JERRY_UNLIKELY ((cp_value) == JMEM_CP_NULL)) ? NULL : JMEM_CP_GET_NON_NULL_POINTER (type, cp_value)))

/**
 * Set value of non-null compressed pointer so that it will correspond
 * to specified non_compressed_pointer
 */
#define JMEM_CP_SET_NON_NULL_POINTER(cp_value, non_compressed_pointer) \
  (cp_value) = jmem_compress_pointer (non_compressed_pointer)

/**
 * Set value of compressed pointer so that it will correspond
 * to specified non_compressed_pointer
 */
#define JMEM_CP_SET_POINTER(cp_value, non_compressed_pointer) \
  do                                                          \
  {                                                           \
    void *ptr_value = (void *) non_compressed_pointer;        \
                                                              \
    if (JERRY_UNLIKELY ((ptr_value) == NULL))                 \
    {                                                         \
      (cp_value) = JMEM_CP_NULL;                              \
    }                                                         \
    else                                                      \
    {                                                         \
      JMEM_CP_SET_NON_NULL_POINTER (cp_value, ptr_value);     \
    }                                                         \
  } while (false);

/**
 * Set value of pointer-tag value so that it will correspond
 * to specified non_compressed_pointer along with tag
 */
#define JMEM_CP_SET_NON_NULL_POINTER_TAG(cp_value, pointer, tag)                   \
  do                                                                               \
  {                                                                                \
    JERRY_ASSERT ((uintptr_t) tag < (uintptr_t) (JMEM_ALIGNMENT));                 \
    jmem_cpointer_tag_t compressed_ptr = jmem_compress_pointer (pointer);          \
    (cp_value) = (jmem_cpointer_tag_t) ((compressed_ptr << JMEM_TAG_SHIFT) | tag); \
  } while (false);

/**
 * Extract value of pointer from specified pointer-tag value
 */
#define JMEM_CP_GET_NON_NULL_POINTER_FROM_POINTER_TAG(type, cp_value) \
  ((type *) (jmem_decompress_pointer ((cp_value & ~JMEM_TAG_MASK) >> JMEM_TAG_SHIFT)))

/**
 * Extract tag bits from pointer-tag value
 */
#define JMEM_CP_GET_POINTER_TAG_BITS(cp_value) \
  (cp_value & (JMEM_FIRST_TAG_BIT_MASK | JMEM_SECOND_TAG_BIT_MASK | JMEM_THIRD_TAG_BIT_MASK))

/**
 * Get value of each tag from specified pointer-tag value
 */
#define JMEM_CP_GET_FIRST_BIT_FROM_POINTER_TAG(cp_value)      \
  (cp_value & JMEM_FIRST_TAG_BIT_MASK) /**< get first tag bit \
                                        **/
#define JMEM_CP_GET_SECOND_BIT_FROM_POINTER_TAG(cp_value) \
  (cp_value & JMEM_SECOND_TAG_BIT_MASK) /**< get second tag bit **/
#define JMEM_CP_GET_THIRD_BIT_FROM_POINTER_TAG(cp_value)      \
  (cp_value & JMEM_THIRD_TAG_BIT_MASK) /**< get third tag bit \
                                        **/

/**
 * Set value of each tag to specified pointer-tag value
 */
#define JMEM_CP_SET_FIRST_BIT_TO_POINTER_TAG(cp_value) \
  (cp_value) = (cp_value | JMEM_FIRST_TAG_BIT_MASK) /**< set first tag bit **/
#define JMEM_CP_SET_SECOND_BIT_TO_POINTER_TAG(cp_value) \
  (cp_value) = (cp_value | JMEM_SECOND_TAG_BIT_MASK) /**< set second tag bit **/
#define JMEM_CP_SET_THIRD_BIT_TO_POINTER_TAG(cp_value) \
  (cp_value) = (cp_value | JMEM_THIRD_TAG_BIT_MASK) /**< set third tag bit **/

/**
 * @}
 * \addtogroup poolman Memory pool manager
 * @{
 */

void *jmem_pools_alloc (size_t size);
void jmem_pools_free (void *chunk_p, size_t size);
void jmem_pools_collect_empty (void);

/**
 * @}
 * @}
 */

#endif /* !JMEM_H */
