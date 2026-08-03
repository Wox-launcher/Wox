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

/*
 * Engine context for JerryScript
 */
#ifndef JCONTEXT_H
#define JCONTEXT_H

#include "ecma-builtins.h"
#include "ecma-helpers.h"
#include "ecma-jobqueue.h"

#include "jmem.h"
#include "js-parser-internal.h"
#include "re-bytecode.h"
#include "vm-defines.h"

/** \addtogroup context Context
 * @{
 */

/**
 * Advanced allocator configurations.
 */
/**
 * Maximum global heap size in bytes
 */
#define CONFIG_MEM_HEAP_SIZE (JERRY_GLOBAL_HEAP_SIZE * 1024)

/**
 * Maximum stack usage size in bytes
 */
#define CONFIG_MEM_STACK_LIMIT (JERRY_STACK_LIMIT * 1024)

/**
 * Max heap usage limit
 */
#define CONFIG_MAX_GC_LIMIT 8192

/**
 * Allowed heap usage limit until next garbage collection
 *
 * Whenever the total allocated memory size reaches the current heap limit, garbage collection will be triggered
 * to try and reduce clutter from unreachable objects. If the allocated memory can't be reduced below the limit,
 * then the current limit will be incremented by CONFIG_MEM_HEAP_LIMIT.
 */
#if defined(JERRY_GC_LIMIT) && (JERRY_GC_LIMIT != 0)
#define CONFIG_GC_LIMIT JERRY_GC_LIMIT
#else /* !(defined(JERRY_GC_LIMIT) && (JERRY_GC_LIMIT != 0)) */
#define CONFIG_GC_LIMIT (JERRY_MIN (CONFIG_MEM_HEAP_SIZE / 32, CONFIG_MAX_GC_LIMIT))
#endif /* defined(JERRY_GC_LIMIT) && (JERRY_GC_LIMIT != 0) */

/**
 * Amount of newly allocated objects since the last GC run, represented as a fraction of all allocated objects,
 * which when reached will trigger garbage collection to run with a low pressure setting.
 *
 * The fraction is calculated as:
 *                1.0 / CONFIG_ECMA_GC_NEW_OBJECTS_FRACTION
 */
#define CONFIG_ECMA_GC_NEW_OBJECTS_FRACTION (16)

#if !JERRY_SYSTEM_ALLOCATOR
/**
 * Heap structure
 *
 * Memory blocks returned by the allocator must not start from the
 * beginning of the heap area because offset 0 is reserved for
 * JMEM_CP_NULL. This special constant is used in several places,
 * e.g. it marks the end of the property chain list, so it cannot
 * be eliminated from the project. Although the allocator cannot
 * use the first 8 bytes of the heap, nothing prevents to use it
 * for other purposes. Currently the free region start is stored
 * there.
 */
typedef struct jmem_heap_t jmem_heap_t;
#endif /* !JERRY_SYSTEM_ALLOCATOR */

/**
 * User context item
 */
typedef struct jerry_context_data_header
{
  struct jerry_context_data_header *next_p; /**< pointer to next context item */
  const jerry_context_data_manager_t *manager_p; /**< manager responsible for deleting this item */
} jerry_context_data_header_t;

#define JERRY_CONTEXT_DATA_HEADER_USER_DATA(item_p) ((uint8_t *) (item_p + 1))

/**
 * JerryScript context
 *
 * The purpose of this header is storing
 * all global variables for Jerry
 */
struct jerry_context_t
{
  /* The value of external context members must be preserved across initializations and cleanups. */
#if JERRY_EXTERNAL_CONTEXT
#if !JERRY_SYSTEM_ALLOCATOR
  jmem_heap_t *heap_p; /**< point to the heap aligned to JMEM_ALIGNMENT. */
  uint32_t heap_size; /**< size of the heap */
#endif /* !JERRY_SYSTEM_ALLOCATOR */
#endif /* JERRY_EXTERNAL_CONTEXT */

  ecma_global_object_t *global_object_p; /**< current global object */
  jmem_heap_free_t *jmem_heap_list_skip_p; /**< improves deallocation performance */
  jmem_pools_chunk_t *jmem_free_8_byte_chunk_p; /**< list of free eight byte pool chunks */
#if JERRY_BUILTIN_REGEXP
  re_compiled_code_t *re_cache[RE_CACHE_SIZE]; /**< regex cache */
#endif /* JERRY_BUILTIN_REGEXP */
  const lit_utf8_byte_t *const *lit_magic_string_ex_array; /**< array of external magic strings */
  const lit_utf8_size_t *lit_magic_string_ex_sizes; /**< external magic string lengths */
  jmem_cpointer_t ecma_gc_objects_cp; /**< List of currently alive objects. */
  jmem_cpointer_t string_list_first_cp; /**< first item of the literal string list */
  jmem_cpointer_t symbol_list_first_cp; /**< first item of the global symbol list */
  jmem_cpointer_t number_list_first_cp; /**< first item of the literal number list */
#if JERRY_BUILTIN_BIGINT
  jmem_cpointer_t bigint_list_first_cp; /**< first item of the literal bigint list */
#endif /* JERRY_BUILTIN_BIGINT */
  jmem_cpointer_t global_symbols_cp[ECMA_BUILTIN_GLOBAL_SYMBOL_COUNT]; /**< global symbols */

#if JERRY_MODULE_SYSTEM
  ecma_module_t *module_current_p; /**< current module context */
  jerry_module_state_changed_cb_t module_state_changed_callback_p; /**< callback which is called after the
                                                                    *   state of a module is changed */
  void *module_state_changed_callback_user_p; /**< user pointer for module_state_changed_callback_p */
  jerry_module_import_meta_cb_t module_import_meta_callback_p; /**< callback which is called when an
                                                                *   import.meta expression of a module
                                                                *   is evaluated the first time */
  void *module_import_meta_callback_user_p; /**< user pointer for module_import_meta_callback_p */
  jerry_module_import_cb_t module_import_callback_p; /**< callback for dynamic module import */
  void *module_import_callback_user_p; /**< user pointer for module_import_callback_p */
#endif /* JERRY_MODULE_SYSTEM */

  vm_frame_ctx_t *vm_top_context_p; /**< top (current) interpreter context */
  jerry_context_data_header_t *context_data_p; /**< linked list of user-provided context-specific pointers */
  jerry_external_string_free_cb_t external_string_free_callback_p; /**< free callback for external strings */
  void *error_object_created_callback_user_p; /**< user pointer for error_object_update_callback_p */
  jerry_error_object_created_cb_t error_object_created_callback_p; /**< decorator callback for Error objects */
  size_t ecma_gc_objects_number; /**< number of currently allocated objects */
  size_t ecma_gc_new_objects; /**< number of newly allocated objects since last GC session */
  size_t jmem_heap_allocated_size; /**< size of allocated regions */
  size_t jmem_heap_limit; /**< current limit of heap usage, that is upon being reached,
                           *   causes call of "try give memory back" callbacks */
  ecma_value_t error_value; /**< currently thrown error value */
  uint32_t lit_magic_string_ex_count; /**< external magic strings count */
  uint32_t jerry_init_flags; /**< run-time configuration flags */
  uint32_t status_flags; /**< run-time flags (the top 8 bits are used for passing class parsing options) */

#if (JERRY_GC_MARK_LIMIT != 0)
  uint32_t ecma_gc_mark_recursion_limit; /**< GC mark recursion limit */
#endif /* (JERRY_GC_MARK_LIMIT != 0) */

#if JERRY_PROPERTY_HASHMAP
  uint8_t ecma_prop_hashmap_alloc_state; /**< property hashmap allocation state: 0-4,
                                          *   if !0 property hashmap allocation is disabled */
#endif /* JERRY_PROPERTY_HASHMAP */

#if JERRY_BUILTIN_REGEXP
  uint8_t re_cache_idx; /**< evicted item index when regex cache is full (round-robin) */
#endif /* JERRY_BUILTIN_REGEXP */
  ecma_job_queue_item_t *job_queue_head_p; /**< points to the head item of the job queue */
  ecma_job_queue_item_t *job_queue_tail_p; /**< points to the tail item of the job queue */

#if JERRY_BUILTIN_TYPEDARRAY
  uint32_t arraybuffer_compact_allocation_limit; /**< maximum size of compact allocation */
  jerry_arraybuffer_allocate_cb_t arraybuffer_allocate_callback; /**< callback for allocating
                                                                  *   arraybuffer memory */
  jerry_arraybuffer_free_cb_t arraybuffer_free_callback; /**< callback for freeing arraybuffer memory */
  void *arraybuffer_allocate_callback_user_p; /**< user pointer passed to arraybuffer_allocate_callback
                                               *   and arraybuffer_free_callback functions */
#endif /* JERRY_BUILTIN_TYPEDARRAY */

#if JERRY_VM_HALT
  uint32_t vm_exec_stop_frequency; /**< reset value for vm_exec_stop_counter */
  uint32_t vm_exec_stop_counter; /**< down counter for reducing the calls of vm_exec_stop_cb */
  void *vm_exec_stop_user_p; /**< user pointer for vm_exec_stop_cb */
  ecma_vm_exec_stop_callback_t vm_exec_stop_cb; /**< user function which returns whether the
                                                 *   ECMAScript execution should be stopped */
#endif /* JERRY_VM_HALT */

#if JERRY_VM_THROW
  void *vm_throw_callback_user_p; /**< user pointer for vm_throw_callback_p */
  jerry_throw_cb_t vm_throw_callback_p; /**< callback for capturing throws */
#endif /* JERRY_VM_THROW */

#if (JERRY_STACK_LIMIT != 0)
  uintptr_t stack_base; /**< stack base marker */
#endif /* (JERRY_STACK_LIMIT != 0) */

  /* This must be at the end of the context for performance reasons */
#if JERRY_LCACHE
  /** hash table for caching the last access of properties */
  ecma_lcache_hash_entry_t lcache[ECMA_LCACHE_HASH_ROWS_COUNT][ECMA_LCACHE_HASH_ROW_LENGTH];
#endif /* JERRY_LCACHE */

  /**
   * Allowed values and it's meaning:
   * * NULL (0x0): the current "new.target" is undefined, that is the execution is inside a normal method.
   * * Any other valid function object pointer: the current "new.target" is valid and it is constructor call.
   */
  ecma_object_t *current_new_target_p;
};

#if JERRY_EXTERNAL_CONTEXT

/*
 * This part is for JerryScript which uses external context.
 */
#if defined(__MINGW32__) || defined(__MINGW64__)

#include <windows.h>

extern DWORD tls_context_index;

#define JERRY_DEFINE_CURRENT_CONTEXT() \
  jerry_context_t *jerry_current_context_p = (jerry_context_t*)TlsGetValue(tls_context_index); \
  JERRY_UNUSED (jerry_current_context_p)

#elif defined(_MSC_VER)

extern __declspec(thread) jerry_context_t* tls_context_p;

#define JERRY_DEFINE_CURRENT_CONTEXT() \
  jerry_context_t *jerry_current_context_p = tls_context_p; \
  JERRY_UNUSED (jerry_current_context_p)

#else

extern __thread jerry_context_t* tls_context_p;

#define JERRY_DEFINE_CURRENT_CONTEXT() \
  jerry_context_t *jerry_current_context_p = tls_context_p; \
  JERRY_UNUSED (jerry_current_context_p)

#endif

/**
 * Provides a reference to the current context structure.
 */
#define JERRY_CONTEXT_STRUCT (*jerry_current_context_p)

/**
 * Provides a reference to a field in the current context.
 */
#define JERRY_CONTEXT(field) (jerry_current_context_p->field)

#if !JERRY_SYSTEM_ALLOCATOR

#define JMEM_HEAP_SIZE (JERRY_CONTEXT (heap_size))

#define JMEM_HEAP_AREA_SIZE (JMEM_HEAP_SIZE - JMEM_ALIGNMENT)

#if defined(_MSC_VER)
  #pragma warning(push)
  #pragma warning(disable:4200)
#endif
struct jmem_heap_t
{
  jmem_heap_free_t first; /**< first node in free region list */
  uint8_t area[]; /**< heap area */
};
#if defined(_MSC_VER)
  #pragma warning(pop)
#endif

#define JERRY_HEAP_CONTEXT(field) (JERRY_CONTEXT (heap_p)->field)

#endif /* !JERRY_SYSTEM_ALLOCATOR */

#else /* !JERRY_EXTERNAL_CONTEXT */

/*
 * This part is for JerryScript which uses default context.
 */

/**
 * Global context.
 */
extern jerry_context_t jerry_global_context;

/**
 * If EXTERNAL_CONTEXT is not enabled then JERRY_DEFINE_CURRENT_CONTEXT is an empty macro to avoid errors.
 */
#define JERRY_DEFINE_CURRENT_CONTEXT()

/**
 * Config-independent name for context.
 */
#define JERRY_CONTEXT_STRUCT (jerry_global_context)

/**
 * Provides a reference to a field in the current context.
 */
#define JERRY_CONTEXT(field) (jerry_global_context.field)

#if !JERRY_SYSTEM_ALLOCATOR

/**
 * Size of heap
 */
#define JMEM_HEAP_SIZE            ((size_t) (CONFIG_MEM_HEAP_SIZE))

/**
 * Calculate heap area size, leaving space for a pointer to the free list
 */
#define JMEM_HEAP_AREA_SIZE       (JMEM_HEAP_SIZE - JMEM_ALIGNMENT)

struct jmem_heap_t
{
  jmem_heap_free_t first; /**< first node in free region list */
  uint8_t area[JMEM_HEAP_AREA_SIZE]; /**< heap area */
};

/**
 * Global heap.
 */
extern jmem_heap_t jerry_global_heap;

/**
 * Provides a reference to a field of the heap.
 */
#define JERRY_HEAP_CONTEXT(field) (jerry_global_heap.field)

#endif /* !JERRY_SYSTEM_ALLOCATOR */

#endif /* JERRY_EXTERNAL_CONTEXT */

void jcontext_set_exception_flag (bool is_exception);

void jcontext_set_abort_flag (bool is_abort);

bool jcontext_has_pending_exception (void);

bool jcontext_has_pending_abort (void);

void jcontext_raise_exception (ecma_value_t error);

void jcontext_release_exception (void);

ecma_value_t jcontext_take_exception (void);

/**
 * @}
 */

#endif /* !JCONTEXT_H */
