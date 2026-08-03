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
 * Garbage collector implementation
 */

#include "ecma-gc.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-arraybuffer-object.h"
#include "ecma-builtin-handlers.h"
#include "ecma-container-object.h"
#include "ecma-function-object.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-lcache.h"
#include "ecma-objects.h"
#include "ecma-property-hashmap.h"
#include "ecma-proxy-object.h"

#include "jcontext.h"
#include "jrt-bit-fields.h"
#include "jrt-libc-includes.h"
#include "jrt.h"
#include "re-compiler.h"
#include "vm-defines.h"
#include "vm-stack.h"

#if JERRY_BUILTIN_TYPEDARRAY
#include "ecma-typedarray-object.h"
#endif /* JERRY_BUILTIN_TYPEDARRAY */

#include "ecma-promise-object.h"

/* TODO: Extract GC to a separate component */

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmagc Garbage collector
 * @{
 */

/*
 * The garbage collector uses the reference counter
 * of object: it increases the counter by one when
 * the object is marked at the first time.
 */

/**
 * Get visited flag of the object.
 *
 * @return true  - if visited
 *         false - otherwise
 */
static inline bool
ecma_gc_is_object_visited (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p != NULL);

  return (object_p->type_flags_refs < ECMA_OBJECT_NON_VISITED);
} /* ecma_gc_is_object_visited */

/**
 * Mark objects as visited starting from specified object as root
 */
static void ecma_gc_mark (ecma_object_t *object_p);

/**
 * Set visited flag of the object.
 */
static void
ecma_gc_set_object_visited (ecma_object_t *object_p) /**< object */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (object_p->type_flags_refs >= ECMA_OBJECT_NON_VISITED)
  {
#if (JERRY_GC_MARK_LIMIT != 0)
    if (JERRY_CONTEXT (ecma_gc_mark_recursion_limit) != 0)
    {
      JERRY_CONTEXT (ecma_gc_mark_recursion_limit)--;
      /* Set the reference count of gray object to 0 */
      object_p->type_flags_refs &= (ecma_object_descriptor_t) (ECMA_OBJECT_REF_ONE - 1);
      ecma_gc_mark (object_p);
      JERRY_CONTEXT (ecma_gc_mark_recursion_limit)++;
    }
    else
    {
      /* Set the reference count of the non-marked gray object to 1 */
      object_p->type_flags_refs &= (ecma_object_descriptor_t) ((ECMA_OBJECT_REF_ONE << 1) - 1);
      JERRY_ASSERT (object_p->type_flags_refs >= ECMA_OBJECT_REF_ONE);
    }
#else /* (JERRY_GC_MARK_LIMIT == 0) */
    /* Set the reference count of gray object to 0 */
    object_p->type_flags_refs &= (ecma_object_descriptor_t) (ECMA_OBJECT_REF_ONE - 1);
#endif /* (JERRY_GC_MARK_LIMIT != 0) */
  }
} /* ecma_gc_set_object_visited */

/**
 * Initialize GC information for the object
 */
void
ecma_init_gc_info (ecma_object_t *object_p) /**< object */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_CONTEXT (ecma_gc_objects_number)++;
  JERRY_CONTEXT (ecma_gc_new_objects)++;

  JERRY_ASSERT (object_p->type_flags_refs < ECMA_OBJECT_REF_ONE);
  object_p->type_flags_refs |= ECMA_OBJECT_REF_ONE;

  object_p->gc_next_cp = JERRY_CONTEXT (ecma_gc_objects_cp);
  ECMA_SET_NON_NULL_POINTER (JERRY_CONTEXT (ecma_gc_objects_cp), object_p);
} /* ecma_init_gc_info */

/**
 * Increase reference counter of an object
 *
 * @return void
 */
void
ecma_ref_object_inline (ecma_object_t *object_p) /**< object */
{
  if (JERRY_LIKELY (object_p->type_flags_refs < ECMA_OBJECT_MAX_REF))
  {
    object_p->type_flags_refs = (ecma_object_descriptor_t) (object_p->type_flags_refs + ECMA_OBJECT_REF_ONE);
  }
  else
  {
    jerry_fatal (JERRY_FATAL_REF_COUNT_LIMIT);
  }
} /* ecma_ref_object_inline */

/**
 * Increase reference counter of an object
 */
void
ecma_ref_object (ecma_object_t *object_p) /**< object */
{
  ecma_ref_object_inline (object_p);
} /* ecma_ref_object */

/**
 * Decrease reference counter of an object
 *
 * @return void
 */
void
ecma_deref_object (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p->type_flags_refs >= ECMA_OBJECT_REF_ONE);
  object_p->type_flags_refs = (ecma_object_descriptor_t) (object_p->type_flags_refs - ECMA_OBJECT_REF_ONE);
} /* ecma_deref_object */

/**
 * Mark objects referenced by global object
 */
static void
ecma_gc_mark_global_object (ecma_global_object_t *global_object_p) /**< global object */
{
  JERRY_ASSERT (global_object_p->extended_object.u.built_in.routine_id == 0);

  ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, global_object_p->global_env_cp));

#if JERRY_BUILTIN_REALMS
  ecma_gc_set_object_visited (ecma_get_object_from_value (global_object_p->this_binding));
#endif /* JERRY_BUILTIN_REALMS */

  if (global_object_p->global_scope_cp != global_object_p->global_env_cp)
  {
    ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, global_object_p->global_scope_cp));
  }

  jmem_cpointer_t *builtin_objects_p = global_object_p->builtin_objects;

  for (int i = 0; i < ECMA_BUILTIN_OBJECTS_COUNT; i++)
  {
    if (builtin_objects_p[i] != JMEM_CP_NULL)
    {
      ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, builtin_objects_p[i]));
    }
  }
} /* ecma_gc_mark_global_object */

/**
 * Mark objects referenced by arguments object
 */
static void
ecma_gc_mark_arguments_object (ecma_extended_object_t *ext_object_p) /**< arguments object */
{
  JERRY_ASSERT (ecma_get_object_type ((ecma_object_t *) ext_object_p) == ECMA_OBJECT_TYPE_CLASS);

  ecma_unmapped_arguments_t *arguments_p = (ecma_unmapped_arguments_t *) ext_object_p;
  ecma_gc_set_object_visited (ecma_get_object_from_value (arguments_p->callee));

  ecma_value_t *argv_p = (ecma_value_t *) (arguments_p + 1);

  if (ext_object_p->u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED)
  {
    ecma_mapped_arguments_t *mapped_arguments_p = (ecma_mapped_arguments_t *) ext_object_p;
    argv_p = (ecma_value_t *) (mapped_arguments_p + 1);

    ecma_gc_set_object_visited (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, mapped_arguments_p->lex_env));
  }

  uint32_t arguments_number = arguments_p->header.u.cls.u3.arguments_number;

  for (uint32_t i = 0; i < arguments_number; i++)
  {
    if (ecma_is_value_object (argv_p[i]))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (argv_p[i]));
    }
  }
} /* ecma_gc_mark_arguments_object */

/**
 * Mark referenced object from property
 */
static void
ecma_gc_mark_properties (ecma_object_t *object_p, /**< object */
                         bool mark_references) /**< mark references */
{
  JERRY_UNUSED (mark_references);

#if !JERRY_MODULE_SYSTEM
  JERRY_ASSERT (!mark_references);
#endif /* !JERRY_MODULE_SYSTEM */

  jmem_cpointer_t prop_iter_cp = object_p->u1.property_list_cp;

#if JERRY_PROPERTY_HASHMAP
  if (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);
    if (prop_iter_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
    {
      prop_iter_cp = prop_iter_p->next_property_cp;
    }
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  while (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);
    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

    ecma_property_pair_t *property_pair_p = (ecma_property_pair_t *) prop_iter_p;

    for (uint32_t index = 0; index < ECMA_PROPERTY_PAIR_ITEM_COUNT; index++)
    {
      uint8_t property = property_pair_p->header.types[index];

      if (JERRY_LIKELY (ECMA_PROPERTY_IS_RAW (property)))
      {
        if (property & ECMA_PROPERTY_FLAG_DATA)
        {
          ecma_value_t value = property_pair_p->values[index].value;

          if (ecma_is_value_object (value))
          {
            ecma_gc_set_object_visited (ecma_get_object_from_value (value));
          }
          continue;
        }

#if JERRY_MODULE_SYSTEM
        if (mark_references)
        {
          continue;
        }
#endif /* JERRY_MODULE_SYSTEM */

        ecma_property_value_t *accessor_objs_p = property_pair_p->values + index;

        ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (accessor_objs_p);

        if (get_set_pair_p->getter_cp != JMEM_CP_NULL)
        {
          ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->getter_cp));
        }

        if (get_set_pair_p->setter_cp != JMEM_CP_NULL)
        {
          ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->setter_cp));
        }

        continue;
      }

      if (!ECMA_PROPERTY_IS_INTERNAL (property))
      {
        JERRY_ASSERT (property == ECMA_PROPERTY_TYPE_DELETED || property == ECMA_PROPERTY_TYPE_HASHMAP);
        continue;
      }

      JERRY_ASSERT (property_pair_p->names_cp[index] >= LIT_INTERNAL_MAGIC_STRING_FIRST_DATA
                    && property_pair_p->names_cp[index] < LIT_MAGIC_STRING__COUNT);

      switch (property_pair_p->names_cp[index])
      {
        case LIT_INTERNAL_MAGIC_STRING_ENVIRONMENT_RECORD:
        {
          ecma_environment_record_t *environment_record_p;
          environment_record_p =
            ECMA_GET_INTERNAL_VALUE_POINTER (ecma_environment_record_t, property_pair_p->values[index].value);

          if (environment_record_p->this_binding != ECMA_VALUE_UNINITIALIZED)
          {
            JERRY_ASSERT (ecma_is_value_object (environment_record_p->this_binding));
            ecma_gc_set_object_visited (ecma_get_object_from_value (environment_record_p->this_binding));
          }

          JERRY_ASSERT (ecma_is_value_object (environment_record_p->function_object));
          ecma_gc_set_object_visited (ecma_get_object_from_value (environment_record_p->function_object));
          break;
        }
        case LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS:
        {
          ecma_value_t *compact_collection_p;
          compact_collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, property_pair_p->values[index].value);

          ecma_value_t *end_p = ecma_compact_collection_end (compact_collection_p);
          ecma_value_t *current_p = compact_collection_p + 1;

          while (end_p - current_p >= ECMA_PRIVATE_ELEMENT_LIST_SIZE)
          {
            current_p++; /* skip the type */
            current_p++; /* skip the name */
            ecma_value_t value = *current_p++;

            if (!ecma_is_value_undefined (value))
            {
              ecma_gc_set_object_visited (ecma_get_object_from_value (value));
            }
          }
          break;
        }
#if JERRY_BUILTIN_CONTAINER
        case LIT_INTERNAL_MAGIC_STRING_WEAK_REFS:
        {
          ecma_value_t key_arg = ecma_make_object_value (object_p);
          ecma_collection_t *refs_p;

          refs_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, property_pair_p->values[index].value);

          for (uint32_t j = 0; j < refs_p->item_count; j++)
          {
            const ecma_value_t reference_value = refs_p->buffer_p[j];

            if (ecma_is_value_empty (reference_value))
            {
              continue;
            }

            ecma_object_t *reference_object_p = ecma_get_object_from_value (reference_value);

            JERRY_ASSERT (ecma_get_object_type (reference_object_p) == ECMA_OBJECT_TYPE_CLASS);

            ecma_extended_object_t *map_object_p = (ecma_extended_object_t *) reference_object_p;

            if (map_object_p->u.cls.type != ECMA_OBJECT_CLASS_CONTAINER
                || map_object_p->u.cls.u2.container_id != LIT_MAGIC_STRING_WEAKMAP_UL
                || !ecma_gc_is_object_visited (reference_object_p))
            {
              continue;
            }

            ecma_value_t value = ecma_op_container_find_weak_value (reference_object_p, key_arg);

            if (ecma_is_value_object (value))
            {
              ecma_gc_set_object_visited (ecma_get_object_from_value (value));
            }
          }
          break;
        }
#endif /* JERRY_BUILTIN_CONTAINER */
        case LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER_WITH_REFERENCES:
        {
          jerry_value_t value = property_pair_p->values[index].value;

          if (value == JMEM_CP_NULL)
          {
            JERRY_ASSERT (!(property & ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL));
            break;
          }

          ecma_native_pointer_t *item_p;
          item_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_t, value);

          do
          {
            jerry_object_native_info_t *native_info_p = item_p->native_info_p;

            JERRY_ASSERT (native_info_p != NULL && native_info_p->number_of_references > 0);

            uint8_t *start_p = ((uint8_t *) item_p->native_p) + native_info_p->offset_of_references;
            ecma_value_t *value_p = (ecma_value_t *) start_p;
            ecma_value_t *end_p = value_p + native_info_p->number_of_references;

            do
            {
              if (ecma_is_value_object (*value_p))
              {
                ecma_gc_set_object_visited (ecma_get_object_from_value (*value_p));
              }
            } while (++value_p < end_p);

            if (property & ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL)
            {
              break;
            }

            item_p = &(((ecma_native_pointer_chain_t *) item_p)->next_p->data);
          } while (item_p != NULL);

          break;
        }
      }
    }

    prop_iter_cp = prop_iter_p->next_property_cp;
  }
} /* ecma_gc_mark_properties */

/**
 * Mark compiled code.
 */
static void
ecma_gc_mark_compiled_code (ecma_value_t script_value) /**< script value */
{
  cbc_script_t *script_p = ECMA_GET_INTERNAL_VALUE_POINTER (cbc_script_t, script_value);

  if (script_p->refs_and_type & CBC_SCRIPT_USER_VALUE_IS_OBJECT)
  {
    ecma_value_t user_value = CBC_SCRIPT_GET_USER_VALUE (script_p);

    JERRY_ASSERT (ecma_is_value_object (user_value));
    ecma_gc_set_object_visited (ecma_get_object_from_value (user_value));
  }

#if JERRY_MODULE_SYSTEM
  if (script_p->refs_and_type & CBC_SCRIPT_HAS_IMPORT_META)
  {
    ecma_value_t import_meta = CBC_SCRIPT_GET_IMPORT_META (script_p, script_p->refs_and_type);

    JERRY_ASSERT (ecma_is_value_object (import_meta));
    ecma_gc_set_object_visited (ecma_get_object_from_value (import_meta));
  }
#endif /* JERRY_MODULE_SYSTEM */

#if JERRY_BUILTIN_REALMS
  ecma_gc_set_object_visited (script_p->realm_p);
#endif /* JERRY_BUILTIN_REALMS */
} /* ecma_gc_mark_compiled_code */

/**
 * Mark objects referenced by bound function object.
 *
 * @return void
 */
static void JERRY_ATTR_NOINLINE
ecma_gc_mark_bound_function_object (ecma_object_t *object_p) /**< bound function object */
{
  JERRY_ASSERT (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_BOUND_FUNCTION);

  ecma_bound_function_t *bound_func_p = (ecma_bound_function_t *) object_p;

  ecma_object_t *target_func_p;
  target_func_p =
    ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, bound_func_p->header.u.bound_function.target_function);

  ecma_gc_set_object_visited (target_func_p);

  ecma_value_t args_len_or_this = bound_func_p->header.u.bound_function.args_len_or_this;

  if (!ecma_is_value_integer_number (args_len_or_this))
  {
    if (ecma_is_value_object (args_len_or_this))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (args_len_or_this));
    }

    return;
  }

  ecma_integer_value_t args_length = ecma_get_integer_from_value (args_len_or_this);
  ecma_value_t *args_p = (ecma_value_t *) (bound_func_p + 1);

  JERRY_ASSERT (args_length > 0);

  for (ecma_integer_value_t i = 0; i < args_length; i++)
  {
    if (ecma_is_value_object (args_p[i]))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (args_p[i]));
    }
  }
} /* ecma_gc_mark_bound_function_object */

/**
 * Mark objects referenced by Promise built-in.
 */
static void
ecma_gc_mark_promise_object (ecma_extended_object_t *ext_object_p) /**< extended object */
{
  /* Mark promise result. */
  ecma_value_t result = ext_object_p->u.cls.u3.value;

  if (ecma_is_value_object (result))
  {
    ecma_gc_set_object_visited (ecma_get_object_from_value (result));
  }

  /* Mark all reactions. */
  ecma_promise_object_t *promise_object_p = (ecma_promise_object_t *) ext_object_p;

  ecma_collection_t *collection_p = promise_object_p->reactions;

  if (collection_p != NULL)
  {
    ecma_value_t *buffer_p = collection_p->buffer_p;
    ecma_value_t *buffer_end_p = buffer_p + collection_p->item_count;

    while (buffer_p < buffer_end_p)
    {
      ecma_value_t value = *buffer_p++;

      ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, value));

      if (JMEM_CP_GET_FIRST_BIT_FROM_POINTER_TAG (value))
      {
        ecma_gc_set_object_visited (ecma_get_object_from_value (*buffer_p++));
      }

      if (JMEM_CP_GET_SECOND_BIT_FROM_POINTER_TAG (value))
      {
        ecma_gc_set_object_visited (ecma_get_object_from_value (*buffer_p++));
      }
    }
  }
} /* ecma_gc_mark_promise_object */

#if JERRY_BUILTIN_CONTAINER
/**
 * Mark objects referenced by Map built-in.
 */
static void
ecma_gc_mark_map_object (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p != NULL);

  ecma_extended_object_t *map_object_p = (ecma_extended_object_t *) object_p;
  ecma_collection_t *container_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, map_object_p->u.cls.u3.value);
  ecma_value_t *start_p = ECMA_CONTAINER_START (container_p);
  uint32_t entry_count = ECMA_CONTAINER_ENTRY_COUNT (container_p);

  for (uint32_t i = 0; i < entry_count; i += ECMA_CONTAINER_PAIR_SIZE)
  {
    ecma_container_pair_t *entry_p = (ecma_container_pair_t *) (start_p + i);

    if (ecma_is_value_empty (entry_p->key))
    {
      continue;
    }

    if (ecma_is_value_object (entry_p->key))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (entry_p->key));
    }

    if (ecma_is_value_object (entry_p->value))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (entry_p->value));
    }
  }
} /* ecma_gc_mark_map_object */

/**
 * Mark objects referenced by WeakMap built-in.
 */
static void
ecma_gc_mark_weakmap_object (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p != NULL);

  ecma_extended_object_t *map_object_p = (ecma_extended_object_t *) object_p;
  ecma_collection_t *container_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, map_object_p->u.cls.u3.value);
  ecma_value_t *start_p = ECMA_CONTAINER_START (container_p);
  uint32_t entry_count = ECMA_CONTAINER_ENTRY_COUNT (container_p);

  for (uint32_t i = 0; i < entry_count; i += ECMA_CONTAINER_PAIR_SIZE)
  {
    ecma_container_pair_t *entry_p = (ecma_container_pair_t *) (start_p + i);

    if (ecma_is_value_empty (entry_p->key))
    {
      continue;
    }

    JERRY_ASSERT (ecma_is_value_object (entry_p->key));

    if (ecma_is_value_object (entry_p->value) && ecma_gc_is_object_visited (ecma_get_object_from_value (entry_p->key)))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (entry_p->value));
    }
  }
} /* ecma_gc_mark_weakmap_object */

/**
 * Mark objects referenced by Set built-in.
 */
static void
ecma_gc_mark_set_object (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p != NULL);

  ecma_extended_object_t *map_object_p = (ecma_extended_object_t *) object_p;
  ecma_collection_t *container_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, map_object_p->u.cls.u3.value);
  ecma_value_t *start_p = ECMA_CONTAINER_START (container_p);
  uint32_t entry_count = ECMA_CONTAINER_ENTRY_COUNT (container_p);

  for (uint32_t i = 0; i < entry_count; i += ECMA_CONTAINER_VALUE_SIZE)
  {
    ecma_value_t *entry_p = start_p + i;

    if (ecma_is_value_empty (*entry_p))
    {
      continue;
    }

    if (ecma_is_value_object (*entry_p))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (*entry_p));
    }
  }
} /* ecma_gc_mark_set_object */
#endif /* JERRY_BUILTIN_CONTAINER */

/**
 * Mark objects referenced by inactive generator functions, async functions, etc.
 */
static void
ecma_gc_mark_executable_object (ecma_object_t *object_p) /**< object */
{
  vm_executable_object_t *executable_object_p = (vm_executable_object_t *) object_p;

  if (executable_object_p->extended_object.u.cls.u2.executable_obj_flags & ECMA_ASYNC_GENERATOR_CALLED)
  {
    ecma_value_t task = executable_object_p->extended_object.u.cls.u3.head;

    while (!ECMA_IS_INTERNAL_VALUE_NULL (task))
    {
      ecma_async_generator_task_t *task_p;
      task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, task);

      JERRY_ASSERT (ecma_is_value_object (task_p->promise));
      ecma_gc_set_object_visited (ecma_get_object_from_value (task_p->promise));

      if (ecma_is_value_object (task_p->operation_value))
      {
        ecma_gc_set_object_visited (ecma_get_object_from_value (task_p->operation_value));
      }

      task = task_p->next;
    }
  }

  ecma_gc_set_object_visited (executable_object_p->frame_ctx.lex_env_p);
  ecma_gc_set_object_visited (executable_object_p->shared.function_object_p);

  if (!ECMA_EXECUTABLE_OBJECT_IS_SUSPENDED (executable_object_p)
      && !(executable_object_p->extended_object.u.cls.u2.executable_obj_flags
           & ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD))
  {
    /* All objects referenced by running executable objects are strong roots,
     * and a finished executable object cannot refer to other values. */
    return;
  }

  if (ecma_is_value_object (executable_object_p->frame_ctx.this_binding))
  {
    ecma_gc_set_object_visited (ecma_get_object_from_value (executable_object_p->frame_ctx.this_binding));
  }

  const ecma_compiled_code_t *bytecode_header_p = executable_object_p->shared.bytecode_header_p;
  size_t register_end;

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_header_p;
    register_end = args_p->register_end;
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_header_p;
    register_end = args_p->register_end;
  }

  ecma_value_t *register_p = VM_GET_REGISTERS (&executable_object_p->frame_ctx);
  ecma_value_t *register_end_p = register_p + register_end;

  while (register_p < register_end_p)
  {
    if (ecma_is_value_object (*register_p))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (*register_p));
    }

    register_p++;
  }

  if (executable_object_p->frame_ctx.context_depth > 0)
  {
    ecma_value_t *context_end_p = register_p;

    register_p += executable_object_p->frame_ctx.context_depth;

    ecma_value_t *context_top_p = register_p;

    do
    {
      if (VM_CONTEXT_IS_VARIABLE_LENGTH (VM_GET_CONTEXT_TYPE (context_top_p[-1])))
      {
        ecma_value_t *last_item_p = context_top_p - VM_GET_CONTEXT_END (context_top_p[-1]);
        JERRY_ASSERT (last_item_p >= context_end_p);
        context_top_p--;

        do
        {
          --context_top_p;
          if (ecma_is_value_object (*context_top_p))
          {
            ecma_gc_set_object_visited (ecma_get_object_from_value (*context_top_p));
          }
        } while (context_top_p > last_item_p);

        continue;
      }

      uint32_t offsets = vm_get_context_value_offsets (context_top_p);

      while (VM_CONTEXT_HAS_NEXT_OFFSET (offsets))
      {
        int32_t offset = VM_CONTEXT_GET_NEXT_OFFSET (offsets);

        if (ecma_is_value_object (context_top_p[offset]))
        {
          ecma_gc_set_object_visited (ecma_get_object_from_value (context_top_p[offset]));
        }

        offsets >>= VM_CONTEXT_OFFSET_SHIFT;
      }

      JERRY_ASSERT (context_top_p >= context_end_p + offsets);
      context_top_p -= offsets;
    } while (context_top_p > context_end_p);
  }

  register_end_p = executable_object_p->frame_ctx.stack_top_p;

  while (register_p < register_end_p)
  {
    if (ecma_is_value_object (*register_p))
    {
      ecma_gc_set_object_visited (ecma_get_object_from_value (*register_p));
    }

    register_p++;
  }

  if (ecma_is_value_object (executable_object_p->iterator))
  {
    ecma_gc_set_object_visited (ecma_get_object_from_value (executable_object_p->iterator));
  }
} /* ecma_gc_mark_executable_object */

#if JERRY_BUILTIN_PROXY
/**
 * Mark the objects referenced by a proxy object
 */
static void
ecma_gc_mark_proxy_object (ecma_object_t *object_p) /**< proxy object */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (object_p));

  ecma_proxy_object_t *proxy_p = (ecma_proxy_object_t *) object_p;

  if (!ecma_is_value_null (proxy_p->target))
  {
    ecma_gc_set_object_visited (ecma_get_object_from_value (proxy_p->target));
  }

  if (!ecma_is_value_null (proxy_p->handler))
  {
    ecma_gc_set_object_visited (ecma_get_object_from_value (proxy_p->handler));
  }
} /* ecma_gc_mark_proxy_object */
#endif /* JERRY_BUILTIN_PROXY */

/**
 * Mark objects as visited starting from specified object as root
 */
static void
ecma_gc_mark (ecma_object_t *object_p) /**< object to mark from */
{
  JERRY_ASSERT (object_p != NULL);
  JERRY_ASSERT (ecma_gc_is_object_visited (object_p));

  if (ecma_is_lexical_environment (object_p))
  {
    jmem_cpointer_t outer_lex_env_cp = object_p->u2.outer_reference_cp;

    if (outer_lex_env_cp != JMEM_CP_NULL)
    {
      ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, outer_lex_env_cp));
    }

    switch (ecma_get_lex_env_type (object_p))
    {
      case ECMA_LEXICAL_ENVIRONMENT_CLASS:
      {
#if JERRY_MODULE_SYSTEM
        if (object_p->type_flags_refs & ECMA_OBJECT_FLAG_LEXICAL_ENV_HAS_DATA)
        {
          if (ECMA_LEX_ENV_CLASS_IS_MODULE (object_p))
          {
            ecma_gc_mark_properties (object_p, true);
          }

          ecma_gc_set_object_visited (((ecma_lexical_environment_class_t *) object_p)->object_p);
          return;
        }
#endif /* JERRY_MODULE_SYSTEM */
        /* FALLTHRU */
      }
      case ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND:
      {
        ecma_object_t *binding_object_p = ecma_get_lex_env_binding_object (object_p);
        ecma_gc_set_object_visited (binding_object_p);
        return;
      }
      default:
      {
        break;
      }
    }
  }
  else
  {
    /**
     * Have the object's prototype here so the object could set it to JMEM_CP_NULL
     * if the prototype should be ignored (like in case of PROXY).
     */
    jmem_cpointer_t proto_cp = object_p->u2.prototype_cp;

    switch (ecma_get_object_type (object_p))
    {
      case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
      {
        ecma_extended_object_t *extended_object_p = (ecma_extended_object_t *) object_p;

        if (extended_object_p->u.built_in.id == ECMA_BUILTIN_ID_GLOBAL)
        {
          ecma_gc_mark_global_object ((ecma_global_object_t *) object_p);
        }

#if JERRY_BUILTIN_REALMS
        ecma_value_t realm_value = extended_object_p->u.built_in.realm_value;
        ecma_gc_set_object_visited (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, realm_value));
#endif /* JERRY_BUILTIN_REALMS */
        break;
      }
      case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
      {
#if JERRY_BUILTIN_REALMS
        ecma_value_t realm_value = ((ecma_extended_built_in_object_t *) object_p)->built_in.realm_value;
        ecma_gc_set_object_visited (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, realm_value));
#endif /* JERRY_BUILTIN_REALMS */
        /* FALLTHRU */
      }
      case ECMA_OBJECT_TYPE_CLASS:
      {
        ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

        switch (ext_object_p->u.cls.type)
        {
          case ECMA_OBJECT_CLASS_ARGUMENTS:
          {
            ecma_gc_mark_arguments_object (ext_object_p);
            break;
          }
#if JERRY_PARSER
          case ECMA_OBJECT_CLASS_SCRIPT:
          {
            const ecma_compiled_code_t *compiled_code_p;
            compiled_code_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, ext_object_p->u.cls.u3.value);

            JERRY_ASSERT (!(compiled_code_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION));
            ecma_gc_mark_compiled_code (((cbc_uint8_arguments_t *) compiled_code_p)->script_value);
            break;
          }
#endif /* JERRY_PARSER */
#if JERRY_BUILTIN_TYPEDARRAY
          case ECMA_OBJECT_CLASS_TYPEDARRAY:
          {
            ecma_gc_set_object_visited (ecma_typedarray_get_arraybuffer (object_p));
            break;
          }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_MODULE_SYSTEM
          case ECMA_OBJECT_CLASS_MODULE_NAMESPACE:
          {
            JERRY_ASSERT (proto_cp == JMEM_CP_NULL);
            ecma_gc_set_object_visited (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, ext_object_p->u.cls.u3.value));
            ecma_gc_mark_properties (object_p, true);
            return;
          }
#endif /* JERRY_MODULE_SYSTEM */
#if JERRY_MODULE_SYSTEM
          case ECMA_OBJECT_CLASS_MODULE:
          {
            ecma_module_t *module_p = ((ecma_module_t *) ext_object_p);

            if (module_p->scope_p != NULL)
            {
              ecma_gc_set_object_visited (((ecma_module_t *) ext_object_p)->scope_p);
            }

            if (module_p->namespace_object_p != NULL)
            {
              ecma_gc_set_object_visited (((ecma_module_t *) ext_object_p)->namespace_object_p);
            }

            if (!(module_p->header.u.cls.u2.module_flags & ECMA_MODULE_IS_NATIVE)
                && module_p->u.compiled_code_p != NULL)
            {
              const ecma_compiled_code_t *compiled_code_p = module_p->u.compiled_code_p;

              JERRY_ASSERT (!(compiled_code_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION));
              ecma_gc_mark_compiled_code (((cbc_uint8_arguments_t *) compiled_code_p)->script_value);
            }

            ecma_module_node_t *node_p = module_p->imports_p;

            while (node_p != NULL)
            {
              if (ecma_is_value_object (node_p->u.path_or_module))
              {
                ecma_gc_set_object_visited (ecma_get_object_from_value (node_p->u.path_or_module));
              }

              node_p = node_p->next_p;
            }
            break;
          }
#endif /* JERRY_MODULE_SYSTEM */
#if JERRY_BUILTIN_DATAVIEW
          case ECMA_OBJECT_CLASS_DATAVIEW:
          {
            ecma_dataview_object_t *dataview_p = (ecma_dataview_object_t *) object_p;
            ecma_gc_set_object_visited (dataview_p->buffer_p);
            break;
          }
#endif /* JERRY_BUILTIN_DATAVIEW */
#if JERRY_BUILTIN_CONTAINER
          case ECMA_OBJECT_CLASS_CONTAINER:
          {
            if (ext_object_p->u.cls.u2.container_id == LIT_MAGIC_STRING_MAP_UL)
            {
              ecma_gc_mark_map_object (object_p);
              break;
            }
            if (ext_object_p->u.cls.u2.container_id == LIT_MAGIC_STRING_WEAKMAP_UL)
            {
              ecma_gc_mark_weakmap_object (object_p);
              break;
            }
            if (ext_object_p->u.cls.u2.container_id == LIT_MAGIC_STRING_SET_UL)
            {
              ecma_gc_mark_set_object (object_p);
              break;
            }
            JERRY_ASSERT (ext_object_p->u.cls.u2.container_id == LIT_MAGIC_STRING_WEAKSET_UL);
            break;
          }
#endif /* JERRY_BUILTIN_CONTAINER */
          case ECMA_OBJECT_CLASS_GENERATOR:
          case ECMA_OBJECT_CLASS_ASYNC_GENERATOR:
          {
            ecma_gc_mark_executable_object (object_p);
            break;
          }
          case ECMA_OBJECT_CLASS_PROMISE:
          {
            ecma_gc_mark_promise_object (ext_object_p);
            break;
          }
          case ECMA_OBJECT_CLASS_PROMISE_CAPABILITY:
          {
            ecma_promise_capability_t *capability_p = (ecma_promise_capability_t *) object_p;

            if (ecma_is_value_object (capability_p->header.u.cls.u3.promise))
            {
              ecma_gc_set_object_visited (ecma_get_object_from_value (capability_p->header.u.cls.u3.promise));
            }
            if (ecma_is_value_object (capability_p->resolve))
            {
              ecma_gc_set_object_visited (ecma_get_object_from_value (capability_p->resolve));
            }
            if (ecma_is_value_object (capability_p->reject))
            {
              ecma_gc_set_object_visited (ecma_get_object_from_value (capability_p->reject));
            }
            break;
          }
          case ECMA_OBJECT_CLASS_ASYNC_FROM_SYNC_ITERATOR:
          {
            ecma_async_from_sync_iterator_object_t *iter_p = (ecma_async_from_sync_iterator_object_t *) ext_object_p;

            ecma_gc_set_object_visited (ecma_get_object_from_value (iter_p->header.u.cls.u3.sync_iterator));

            if (!ecma_is_value_undefined (iter_p->sync_next_method))
            {
              ecma_gc_set_object_visited (ecma_get_object_from_value (iter_p->sync_next_method));
            }

            break;
          }
          case ECMA_OBJECT_CLASS_ARRAY_ITERATOR:
          case ECMA_OBJECT_CLASS_SET_ITERATOR:
          case ECMA_OBJECT_CLASS_MAP_ITERATOR:
          {
            ecma_value_t iterated_value = ext_object_p->u.cls.u3.iterated_value;
            if (!ecma_is_value_empty (iterated_value))
            {
              ecma_gc_set_object_visited (ecma_get_object_from_value (iterated_value));
            }
            break;
          }
#if JERRY_BUILTIN_REGEXP
          case ECMA_OBJECT_CLASS_REGEXP_STRING_ITERATOR:
          {
            ecma_regexp_string_iterator_t *regexp_string_iterator_obj = (ecma_regexp_string_iterator_t *) object_p;
            ecma_value_t regexp = regexp_string_iterator_obj->iterating_regexp;
            ecma_gc_set_object_visited (ecma_get_object_from_value (regexp));
            break;
          }
#endif /* JERRY_BUILTIN_REGEXP */
          default:
          {
            /* The ECMA_OBJECT_CLASS__MAX type represents an uninitialized class. */
            JERRY_ASSERT (ext_object_p->u.cls.type <= ECMA_OBJECT_CLASS__MAX);
            break;
          }
        }

        break;
      }
      case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
      {
#if JERRY_BUILTIN_REALMS
        ecma_value_t realm_value = ((ecma_extended_built_in_object_t *) object_p)->built_in.realm_value;
        ecma_gc_set_object_visited (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, realm_value));
#endif /* JERRY_BUILTIN_REALMS */
        /* FALLTHRU */
      }
      case ECMA_OBJECT_TYPE_ARRAY:
      {
        ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

        if (JERRY_UNLIKELY (ext_object_p->u.array.length_prop_and_hole_count & ECMA_ARRAY_TEMPLATE_LITERAL))
        {
          /* Template objects are never marked. */
          JERRY_ASSERT (object_p->type_flags_refs >= ECMA_OBJECT_REF_ONE);
          return;
        }

        if (ecma_op_array_is_fast_array (ext_object_p))
        {
          if (object_p->u1.property_list_cp != JMEM_CP_NULL)
          {
            ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

            for (uint32_t i = 0; i < ext_object_p->u.array.length; i++)
            {
              if (ecma_is_value_object (values_p[i]))
              {
                ecma_gc_set_object_visited (ecma_get_object_from_value (values_p[i]));
              }
            }
          }

          if (proto_cp != JMEM_CP_NULL)
          {
            ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp));
          }
          return;
        }
        break;
      }
#if JERRY_BUILTIN_PROXY
      case ECMA_OBJECT_TYPE_PROXY:
      {
        ecma_gc_mark_proxy_object (object_p);
        /* Prototype of proxy object is a bit set. */
        proto_cp = JMEM_CP_NULL;
        break;
      }
#endif /* JERRY_BUILTIN_PROXY */
      case ECMA_OBJECT_TYPE_FUNCTION:
      {
        ecma_extended_object_t *ext_func_p = (ecma_extended_object_t *) object_p;
        ecma_gc_set_object_visited (
          ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, ext_func_p->u.function.scope_cp));

        const ecma_compiled_code_t *compiled_code_p = ecma_op_function_get_compiled_code (ext_func_p);

        if (CBC_FUNCTION_IS_ARROW (compiled_code_p->status_flags))
        {
          ecma_arrow_function_t *arrow_func_p = (ecma_arrow_function_t *) object_p;

          if (ecma_is_value_object (arrow_func_p->this_binding))
          {
            ecma_gc_set_object_visited (ecma_get_object_from_value (arrow_func_p->this_binding));
          }

          if (ecma_is_value_object (arrow_func_p->new_target))
          {
            ecma_gc_set_object_visited (ecma_get_object_from_value (arrow_func_p->new_target));
          }
        }

#if JERRY_SNAPSHOT_EXEC
        if (JERRY_UNLIKELY (compiled_code_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION))
        {
          /* Static snapshot functions have a global realm */
          break;
        }
#endif /* JERRY_SNAPSHOT_EXEC */

        JERRY_ASSERT (!(compiled_code_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION));
        ecma_gc_mark_compiled_code (((cbc_uint8_arguments_t *) compiled_code_p)->script_value);
        break;
      }
      case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
      {
        ecma_gc_mark_bound_function_object (object_p);
        break;
      }
      case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
      {
        ecma_extended_object_t *ext_func_p = (ecma_extended_object_t *) object_p;

#if JERRY_BUILTIN_REALMS
        ecma_value_t realm_value = ext_func_p->u.built_in.realm_value;
        ecma_gc_set_object_visited (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, realm_value));
#endif /* JERRY_BUILTIN_REALMS */

        if (ext_func_p->u.built_in.id == ECMA_BUILTIN_ID_HANDLER)
        {
          switch (ext_func_p->u.built_in.routine_id)
          {
            case ECMA_NATIVE_HANDLER_PROMISE_RESOLVE:
            case ECMA_NATIVE_HANDLER_PROMISE_REJECT:
            {
              ecma_promise_resolver_t *resolver_obj_p = (ecma_promise_resolver_t *) object_p;
              ecma_gc_set_object_visited (ecma_get_object_from_value (resolver_obj_p->promise));
              break;
            }
            case ECMA_NATIVE_HANDLER_PROMISE_THEN_FINALLY:
            case ECMA_NATIVE_HANDLER_PROMISE_CATCH_FINALLY:
            {
              ecma_promise_finally_function_t *finally_obj_p = (ecma_promise_finally_function_t *) object_p;
              ecma_gc_set_object_visited (ecma_get_object_from_value (finally_obj_p->constructor));
              ecma_gc_set_object_visited (ecma_get_object_from_value (finally_obj_p->on_finally));
              break;
            }
            case ECMA_NATIVE_HANDLER_PROMISE_CAPABILITY_EXECUTOR:
            {
              ecma_promise_capability_executor_t *executor_p = (ecma_promise_capability_executor_t *) object_p;
              ecma_gc_set_object_visited (ecma_get_object_from_value (executor_p->capability));
              break;
            }
            case ECMA_NATIVE_HANDLER_PROMISE_ALL_HELPER:
            {
              ecma_promise_all_executor_t *executor_p = (ecma_promise_all_executor_t *) object_p;
              ecma_gc_set_object_visited (ecma_get_object_from_value (executor_p->capability));
              ecma_gc_set_object_visited (ecma_get_object_from_value (executor_p->values));
              ecma_gc_set_object_visited (ecma_get_object_from_value (executor_p->remaining_elements));
              break;
            }
#if JERRY_BUILTIN_PROXY
            case ECMA_NATIVE_HANDLER_PROXY_REVOKE:
            {
              ecma_revocable_proxy_object_t *rev_proxy_p = (ecma_revocable_proxy_object_t *) object_p;

              if (!ecma_is_value_null (rev_proxy_p->proxy))
              {
                ecma_gc_set_object_visited (ecma_get_object_from_value (rev_proxy_p->proxy));
              }

              break;
            }
#endif /* JERRY_BUILTIN_PROXY */
            case ECMA_NATIVE_HANDLER_VALUE_THUNK:
            case ECMA_NATIVE_HANDLER_VALUE_THROWER:
            {
              ecma_promise_value_thunk_t *thunk_obj_p = (ecma_promise_value_thunk_t *) object_p;

              if (ecma_is_value_object (thunk_obj_p->value))
              {
                ecma_gc_set_object_visited (ecma_get_object_from_value (thunk_obj_p->value));
              }
              break;
            }
            case ECMA_NATIVE_HANDLER_ASYNC_FROM_SYNC_ITERATOR_UNWRAP:
            {
              break;
            }
            default:
            {
              JERRY_UNREACHABLE ();
            }
          }
        }
        break;
      }
      case ECMA_OBJECT_TYPE_CONSTRUCTOR_FUNCTION:
      {
        ecma_gc_mark_compiled_code (((ecma_extended_object_t *) object_p)->u.constructor_function.script_value);
        break;
      }
#if JERRY_BUILTIN_REALMS
      case ECMA_OBJECT_TYPE_NATIVE_FUNCTION:
      {
        ecma_native_function_t *native_function_p = (ecma_native_function_t *) object_p;
        ecma_gc_set_object_visited (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, native_function_p->realm_value));
        break;
      }
#endif /* JERRY_BUILTIN_REALMS */
      default:
      {
        break;
      }
    }

    if (proto_cp != JMEM_CP_NULL)
    {
      ecma_gc_set_object_visited (ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp));
    }
  }

  ecma_gc_mark_properties (object_p, false);
} /* ecma_gc_mark */

/**
 * Free the native handle/pointer by calling its free callback.
 */
static void
ecma_gc_free_native_pointer (ecma_property_t property, /**< property descriptor */
                             ecma_value_t value) /**< property value */
{
  if (JERRY_LIKELY (property & ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL))
  {
    JERRY_ASSERT (value != JMEM_CP_NULL);

    ecma_native_pointer_t *native_pointer_p;
    native_pointer_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_t, value);

    if (native_pointer_p->native_info_p != NULL)
    {
      jerry_object_native_free_cb_t free_cb = native_pointer_p->native_info_p->free_cb;

      if (free_cb != NULL)
      {
        free_cb (native_pointer_p->native_p, native_pointer_p->native_info_p);
      }
    }

    jmem_heap_free_block (native_pointer_p, sizeof (ecma_native_pointer_t));
    return;
  }

  if (value == JMEM_CP_NULL)
  {
    return;
  }

  ecma_native_pointer_chain_t *item_p;
  item_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_chain_t, value);

  do
  {
    if (item_p->data.native_info_p != NULL)
    {
      jerry_object_native_free_cb_t free_cb = item_p->data.native_info_p->free_cb;

      if (free_cb != NULL)
      {
        free_cb (item_p->data.native_p, item_p->data.native_info_p);
      }
    }

    ecma_native_pointer_chain_t *next_p = item_p->next_p;

    jmem_heap_free_block (item_p, sizeof (ecma_native_pointer_chain_t));

    item_p = next_p;
  } while (item_p != NULL);
} /* ecma_gc_free_native_pointer */

/**
 * Free specified arguments object.
 *
 * @return allocated object's size
 */
static size_t
ecma_free_arguments_object (ecma_extended_object_t *ext_object_p) /**< arguments object */
{
  JERRY_ASSERT (ecma_get_object_type ((ecma_object_t *) ext_object_p) == ECMA_OBJECT_TYPE_CLASS);

  size_t object_size = sizeof (ecma_unmapped_arguments_t);

  if (ext_object_p->u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED)
  {
    ecma_mapped_arguments_t *mapped_arguments_p = (ecma_mapped_arguments_t *) ext_object_p;
    object_size = sizeof (ecma_mapped_arguments_t);

#if JERRY_SNAPSHOT_EXEC
    if (!(mapped_arguments_p->unmapped.header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_STATIC_BYTECODE))
#endif /* JERRY_SNAPSHOT_EXEC */
    {
      ecma_compiled_code_t *byte_code_p =
        ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, mapped_arguments_p->u.byte_code);

      ecma_bytecode_deref (byte_code_p);
    }
  }

  ecma_value_t *argv_p = (ecma_value_t *) (((uint8_t *) ext_object_p) + object_size);
  ecma_unmapped_arguments_t *arguments_p = (ecma_unmapped_arguments_t *) ext_object_p;
  uint32_t arguments_number = arguments_p->header.u.cls.u3.arguments_number;

  for (uint32_t i = 0; i < arguments_number; i++)
  {
    ecma_free_value_if_not_object (argv_p[i]);
  }

  uint32_t saved_argument_count = JERRY_MAX (arguments_number, arguments_p->header.u.cls.u2.formal_params_number);

  return object_size + (saved_argument_count * sizeof (ecma_value_t));
} /* ecma_free_arguments_object */

/**
 * Free specified fast access mode array object.
 */
static void
ecma_free_fast_access_array (ecma_object_t *object_p) /**< fast access mode array object to free */
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (object_p));

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  const uint32_t aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (ext_object_p->u.array.length);

  if (object_p->u1.property_list_cp != JMEM_CP_NULL)
  {
    ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

    for (uint32_t i = 0; i < aligned_length; i++)
    {
      ecma_free_value_if_not_object (values_p[i]);
    }

    jmem_heap_free_block (values_p, aligned_length * sizeof (ecma_value_t));
  }

  ecma_dealloc_extended_object (object_p, sizeof (ecma_extended_object_t));
} /* ecma_free_fast_access_array */

/**
 * Free non-objects referenced by inactive generator functions, async functions, etc.
 *
 * @return total object size
 */
static size_t
ecma_gc_free_executable_object (ecma_object_t *object_p) /**< object */
{
  vm_executable_object_t *executable_object_p = (vm_executable_object_t *) object_p;

  const ecma_compiled_code_t *bytecode_header_p = executable_object_p->shared.bytecode_header_p;
  size_t size, register_end;

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_header_p;

    register_end = args_p->register_end;
    size = (register_end + (size_t) args_p->stack_limit) * sizeof (ecma_value_t);
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_header_p;

    register_end = args_p->register_end;
    size = (register_end + (size_t) args_p->stack_limit) * sizeof (ecma_value_t);
  }

  size = JERRY_ALIGNUP (sizeof (vm_executable_object_t) + size, sizeof (uintptr_t));
  ecma_bytecode_deref ((ecma_compiled_code_t *) bytecode_header_p);

  uint16_t executable_obj_flags = executable_object_p->extended_object.u.cls.u2.executable_obj_flags;

  JERRY_ASSERT (!(executable_obj_flags & ECMA_EXECUTABLE_OBJECT_RUNNING));

  if (executable_obj_flags & ECMA_ASYNC_GENERATOR_CALLED)
  {
    ecma_value_t task = executable_object_p->extended_object.u.cls.u3.head;

    while (!ECMA_IS_INTERNAL_VALUE_NULL (task))
    {
      ecma_async_generator_task_t *task_p;
      task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, task);

      JERRY_ASSERT (ecma_is_value_object (task_p->promise));
      ecma_free_value_if_not_object (task_p->operation_value);

      task = task_p->next;
      jmem_heap_free_block (task_p, sizeof (ecma_async_generator_task_t));
    }
  }

  if (executable_obj_flags & ECMA_EXECUTABLE_OBJECT_COMPLETED)
  {
    return size;
  }

  ecma_free_value_if_not_object (executable_object_p->frame_ctx.this_binding);

  ecma_value_t *register_p = VM_GET_REGISTERS (&executable_object_p->frame_ctx);
  ecma_value_t *register_end_p = register_p + register_end;

  while (register_p < register_end_p)
  {
    ecma_free_value_if_not_object (*register_p++);
  }

  if (executable_object_p->frame_ctx.context_depth > 0)
  {
    ecma_value_t *context_end_p = register_p;

    register_p += executable_object_p->frame_ctx.context_depth;

    ecma_value_t *context_top_p = register_p;

    do
    {
      context_top_p[-1] &= (uint32_t) ~VM_CONTEXT_HAS_LEX_ENV;

      if (VM_CONTEXT_IS_VARIABLE_LENGTH (VM_GET_CONTEXT_TYPE (context_top_p[-1])))
      {
        ecma_value_t *last_item_p = context_top_p - VM_GET_CONTEXT_END (context_top_p[-1]);
        JERRY_ASSERT (last_item_p >= context_end_p);
        context_top_p--;

        do
        {
          ecma_free_value_if_not_object (*(--context_top_p));
        } while (context_top_p > last_item_p);

        continue;
      }

      uint32_t offsets = vm_get_context_value_offsets (context_top_p);

      while (VM_CONTEXT_HAS_NEXT_OFFSET (offsets))
      {
        int32_t offset = VM_CONTEXT_GET_NEXT_OFFSET (offsets);

        if (ecma_is_value_object (context_top_p[offset]))
        {
          context_top_p[offset] = ECMA_VALUE_UNDEFINED;
        }

        offsets >>= VM_CONTEXT_OFFSET_SHIFT;
      }

      context_top_p = vm_stack_context_abort (&executable_object_p->frame_ctx, context_top_p);
    } while (context_top_p > context_end_p);
  }

  register_end_p = executable_object_p->frame_ctx.stack_top_p;

  while (register_p < register_end_p)
  {
    ecma_free_value_if_not_object (*register_p++);
  }

  return size;
} /* ecma_gc_free_executable_object */

JERRY_STATIC_ASSERT (!ECMA_PROPERTY_IS_RAW (ECMA_PROPERTY_TYPE_DELETED),
                     ecma_property_type_deleted_must_not_be_raw_property);
JERRY_STATIC_ASSERT ((((int)ECMA_PROPERTY_TYPE_DELETED & (int)ECMA_PROPERTY_FLAG_LCACHED) == 0),
                     ecma_property_type_deleted_must_not_have_lcached_flag);
JERRY_STATIC_ASSERT ((((int)ECMA_GC_FREE_SECOND_PROPERTY) == 1), ecma_gc_free_second_must_be_one);

/**
 * Free property of an object
 */
void
ecma_gc_free_property (ecma_object_t *object_p, /**< object */
                       ecma_property_pair_t *prop_pair_p, /**< property pair */
                       uint32_t options) /**< option bits including property index */
{
  /* Both cannot be deleted. */
  JERRY_ASSERT (prop_pair_p->header.types[0] != ECMA_PROPERTY_TYPE_DELETED
                || prop_pair_p->header.types[1] != ECMA_PROPERTY_TYPE_DELETED);
  JERRY_ASSERT (prop_pair_p->header.types[0] != ECMA_PROPERTY_TYPE_HASHMAP);

  uint32_t index = (options & ECMA_GC_FREE_SECOND_PROPERTY);
  jmem_cpointer_t name_cp = prop_pair_p->names_cp[index];
  ecma_property_t *property_p = prop_pair_p->header.types + index;
  ecma_property_t property = *property_p;

#if JERRY_LCACHE
  if ((property & ECMA_PROPERTY_FLAG_LCACHED) != 0)
  {
    ecma_lcache_invalidate (object_p, name_cp, property_p);
  }
#endif /* JERRY_LCACHE */

  if (ECMA_PROPERTY_IS_RAW (property))
  {
    if (ECMA_PROPERTY_GET_NAME_TYPE (property) == ECMA_DIRECT_STRING_PTR)
    {
      ecma_string_t *prop_name_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, name_cp);
      ecma_deref_ecma_string (prop_name_p);
    }

    if (property & ECMA_PROPERTY_FLAG_DATA)
    {
      ecma_free_value_if_not_object (prop_pair_p->values[index].value);
      return;
    }

    if (JERRY_UNLIKELY (options & ECMA_GC_FREE_REFERENCES))
    {
      return;
    }
    return;
  }

  if (property == ECMA_PROPERTY_TYPE_DELETED)
  {
    return;
  }

  ecma_value_t value = prop_pair_p->values[index].value;

  switch (name_cp)
  {
    case LIT_INTERNAL_MAGIC_STRING_ENVIRONMENT_RECORD:
    {
      ecma_environment_record_t *environment_record_p;
      environment_record_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_environment_record_t, value);
      jmem_heap_free_block (environment_record_p, sizeof (ecma_environment_record_t));
      break;
    }
    case LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_COMPUTED:
    {
      ecma_value_t *compact_collection_p;
      compact_collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, value);
      ecma_compact_collection_free (compact_collection_p);
      break;
    }
    case LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS:
    {
      ecma_value_t *compact_collection_p;
      compact_collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, value);

      ecma_value_t *end_p = ecma_compact_collection_end (compact_collection_p);
      ecma_value_t *current_p = compact_collection_p + 1;

      JERRY_ASSERT ((end_p - current_p) % ECMA_PRIVATE_ELEMENT_LIST_SIZE == 0);

      while (current_p < end_p)
      {
        current_p++; /* skip the type */
        ecma_deref_ecma_string (ecma_get_prop_name_from_value (*current_p++));
        current_p++; /* skip the value */
      }

      ecma_compact_collection_destroy (compact_collection_p);
      break;
    }
    default:
    {
      JERRY_ASSERT (name_cp == LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER
                    || name_cp == LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER_WITH_REFERENCES);
      ecma_gc_free_native_pointer (property, value);
      break;
    }
  }
} /* ecma_gc_free_property */

/**
 * Free properties of an object
 */
void
ecma_gc_free_properties (ecma_object_t *object_p, /**< object */
                         uint32_t options) /**< option bits */
{
  jmem_cpointer_t prop_iter_cp = object_p->u1.property_list_cp;

#if JERRY_PROPERTY_HASHMAP
  if (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);
    if (prop_iter_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
    {
      ecma_property_hashmap_free (object_p);
      prop_iter_cp = object_p->u1.property_list_cp;
    }
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  while (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);
    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

    ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

    for (uint32_t i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      ecma_gc_free_property (object_p, prop_pair_p, i | options);
    }

    prop_iter_cp = prop_iter_p->next_property_cp;

    ecma_dealloc_property_pair (prop_pair_p);
  }
} /* ecma_gc_free_properties */

/**
 * Free specified object.
 */
static void
ecma_gc_free_object (ecma_object_t *object_p) /**< object to free */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (object_p != NULL && !ecma_gc_is_object_visited (object_p));

  JERRY_ASSERT (JERRY_CONTEXT (ecma_gc_objects_number) > 0);
  JERRY_CONTEXT (ecma_gc_objects_number)--;

  if (ecma_is_lexical_environment (object_p))
  {
#if JERRY_MODULE_SYSTEM
    if (ecma_get_lex_env_type (object_p) == ECMA_LEXICAL_ENVIRONMENT_CLASS
        && (object_p->type_flags_refs & ECMA_OBJECT_FLAG_LEXICAL_ENV_HAS_DATA))
    {
      if (ECMA_LEX_ENV_CLASS_IS_MODULE (object_p))
      {
        ecma_gc_free_properties (object_p, ECMA_GC_FREE_REFERENCES);
      }

      ecma_dealloc_extended_object (object_p, sizeof (ecma_lexical_environment_class_t));
      return;
    }
#endif /* JERRY_MODULE_SYSTEM */

    if (ecma_get_lex_env_type (object_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
    {
      ecma_gc_free_properties (object_p, ECMA_GC_FREE_NO_OPTIONS);
    }

    ecma_dealloc_object (object_p);
    return;
  }

  ecma_object_type_t object_type = ecma_get_object_type (object_p);

  size_t ext_object_size = sizeof (ecma_extended_object_t);

  switch (object_type)
  {
    case ECMA_OBJECT_TYPE_GENERAL:
    {
      ecma_gc_free_properties (object_p, ECMA_GC_FREE_NO_OPTIONS);
      ecma_dealloc_object (object_p);
      return;
    }
    case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
    {
      if (((ecma_extended_object_t *) object_p)->u.built_in.id == ECMA_BUILTIN_ID_GLOBAL)
      {
        ext_object_size = sizeof (ecma_global_object_t);
        break;
      }

      uint8_t bitset_size = ((ecma_extended_object_t *) object_p)->u.built_in.u.length_and_bitset_size;
      ext_object_size += sizeof (uint64_t) * (bitset_size >> ECMA_BUILT_IN_BITSET_SHIFT);
      break;
    }
    case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
    {
      ext_object_size = sizeof (ecma_extended_built_in_object_t);
      uint8_t bitset_size = ((ecma_extended_built_in_object_t *) object_p)->built_in.u.length_and_bitset_size;
      ext_object_size += sizeof (uint64_t) * (bitset_size >> ECMA_BUILT_IN_BITSET_SHIFT);
      /* FALLTHRU */
    }
    case ECMA_OBJECT_TYPE_CLASS:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      switch (ext_object_p->u.cls.type)
      {
        case ECMA_OBJECT_CLASS_STRING:
        case ECMA_OBJECT_CLASS_NUMBER:
        case ECMA_OBJECT_CLASS_SYMBOL:
#if JERRY_BUILTIN_BIGINT
        case ECMA_OBJECT_CLASS_BIGINT:
#endif /* JERRY_BUILTIN_BIGINT */
        {
          ecma_free_value (ext_object_p->u.cls.u3.value);
          break;
        }
        case ECMA_OBJECT_CLASS_ARGUMENTS:
        {
          ext_object_size = ecma_free_arguments_object (ext_object_p);
          break;
        }
#if JERRY_BUILTIN_TYPEDARRAY
        case ECMA_OBJECT_CLASS_TYPEDARRAY:
        {
          if (ext_object_p->u.cls.u2.typedarray_flags & ECMA_TYPEDARRAY_IS_EXTENDED)
          {
            ext_object_size = sizeof (ecma_extended_typedarray_object_t);
          }
          break;
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_MODULE_SYSTEM
        case ECMA_OBJECT_CLASS_MODULE_NAMESPACE:
        {
          ecma_gc_free_properties (object_p, ECMA_GC_FREE_REFERENCES);
          ecma_dealloc_extended_object (object_p, sizeof (ecma_extended_object_t));
          return;
        }
#endif /* JERRY_MODULE_SYSTEM */
#if JERRY_PARSER
        case ECMA_OBJECT_CLASS_SCRIPT:
        {
          ecma_compiled_code_t *compiled_code_p;
          compiled_code_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, ext_object_p->u.cls.u3.value);

          ecma_bytecode_deref (compiled_code_p);
          break;
        }
#endif /* JERRY_PARSER */
#if JERRY_BUILTIN_DATE
        case ECMA_OBJECT_CLASS_DATE:
        {
          ext_object_size = sizeof (ecma_date_object_t);
          break;
        }
#endif /* JERRY_BUILTIN_DATE */
#if JERRY_BUILTIN_REGEXP
        case ECMA_OBJECT_CLASS_REGEXP:
        {
          ecma_compiled_code_t *bytecode_p =
            ECMA_GET_INTERNAL_VALUE_ANY_POINTER (ecma_compiled_code_t, ext_object_p->u.cls.u3.value);

          ecma_bytecode_deref (bytecode_p);

          break;
        }
#endif /* JERRY_BUILTIN_REGEXP */
        case ECMA_OBJECT_CLASS_STRING_ITERATOR:
        {
          ecma_value_t iterated_value = ext_object_p->u.cls.u3.iterated_value;

          if (!ecma_is_value_empty (iterated_value))
          {
            ecma_deref_ecma_string (ecma_get_string_from_value (iterated_value));
          }

          break;
        }
#if JERRY_BUILTIN_REGEXP
        case ECMA_OBJECT_CLASS_REGEXP_STRING_ITERATOR:
        {
          ecma_regexp_string_iterator_t *regexp_string_iterator_obj = (ecma_regexp_string_iterator_t *) object_p;
          ecma_value_t iterated_string = regexp_string_iterator_obj->iterated_string;

          if (!ecma_is_value_empty (iterated_string))
          {
            ecma_deref_ecma_string (ecma_get_string_from_value (iterated_string));
          }

          ext_object_size = sizeof (ecma_regexp_string_iterator_t);
          break;
        }
#endif /* JERRY_BUILTIN_REGEXP */
#if JERRY_BUILTIN_TYPEDARRAY
        case ECMA_OBJECT_CLASS_ARRAY_BUFFER:
#if JERRY_BUILTIN_SHAREDARRAYBUFFER
        case ECMA_OBJECT_CLASS_SHARED_ARRAY_BUFFER:
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */
        {
          if (!(ECMA_ARRAYBUFFER_GET_FLAGS (ext_object_p) & ECMA_ARRAYBUFFER_HAS_POINTER))
          {
            ext_object_size += ext_object_p->u.cls.u3.length;
            break;
          }

          ext_object_size = sizeof (ecma_arraybuffer_pointer_t);

          if (ECMA_ARRAYBUFFER_GET_FLAGS (ext_object_p) & ECMA_ARRAYBUFFER_ALLOCATED)
          {
            ecma_arraybuffer_release_buffer (object_p);
          }
          break;
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_BUILTIN_WEAKREF
        case ECMA_OBJECT_CLASS_WEAKREF:
        {
          ecma_value_t target = ext_object_p->u.cls.u3.target;

          if (!ecma_is_value_undefined (target))
          {
            ecma_op_object_unref_weak (ecma_get_object_from_value (target), ecma_make_object_value (object_p));
          }
          break;
        }
#endif /* JERRY_BUILTIN_WEAKREF */
#if JERRY_BUILTIN_CONTAINER
        case ECMA_OBJECT_CLASS_CONTAINER:
        {
          ecma_extended_object_t *map_object_p = (ecma_extended_object_t *) object_p;
          ecma_collection_t *container_p =
            ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, map_object_p->u.cls.u3.value);
          ecma_op_container_free_entries (object_p);
          ecma_collection_destroy (container_p);
          break;
        }
#endif /* JERRY_BUILTIN_CONTAINER */
#if JERRY_BUILTIN_DATAVIEW
        case ECMA_OBJECT_CLASS_DATAVIEW:
        {
          ext_object_size = sizeof (ecma_dataview_object_t);
          break;
        }
#endif /* JERRY_BUILTIN_DATAVIEW */
        case ECMA_OBJECT_CLASS_GENERATOR:
        case ECMA_OBJECT_CLASS_ASYNC_GENERATOR:
        {
          ext_object_size = ecma_gc_free_executable_object (object_p);
          break;
        }
        case ECMA_OBJECT_CLASS_PROMISE:
        {
          ecma_free_value_if_not_object (ext_object_p->u.cls.u3.value);

          /* Reactions only contains objects. */
          ecma_collection_destroy (((ecma_promise_object_t *) object_p)->reactions);

          ext_object_size = sizeof (ecma_promise_object_t);
          break;
        }
        case ECMA_OBJECT_CLASS_PROMISE_CAPABILITY:
        {
          ext_object_size = sizeof (ecma_promise_capability_t);
          break;
        }
        case ECMA_OBJECT_CLASS_ASYNC_FROM_SYNC_ITERATOR:
        {
          ext_object_size = sizeof (ecma_async_from_sync_iterator_object_t);
          break;
        }
#if JERRY_MODULE_SYSTEM
        case ECMA_OBJECT_CLASS_MODULE:
        {
          ecma_module_release_module ((ecma_module_t *) ext_object_p);
          ext_object_size = sizeof (ecma_module_t);
          break;
        }
#endif /* JERRY_MODULE_SYSTEM */
        default:
        {
          /* The ECMA_OBJECT_CLASS__MAX type represents an uninitialized class. */
          JERRY_ASSERT (ext_object_p->u.cls.type <= ECMA_OBJECT_CLASS__MAX);
          break;
        }
      }

      break;
    }
    case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
    {
      ext_object_size = sizeof (ecma_extended_built_in_object_t);
      uint8_t bitset_size = ((ecma_extended_built_in_object_t *) object_p)->built_in.u.length_and_bitset_size;
      ext_object_size += sizeof (uint64_t) * (bitset_size >> ECMA_BUILT_IN_BITSET_SHIFT);
      /* FALLTHRU */
    }
    case ECMA_OBJECT_TYPE_ARRAY:
    {
      if (ecma_op_array_is_fast_array ((ecma_extended_object_t *) object_p))
      {
        ecma_free_fast_access_array (object_p);
        return;
      }
      break;
    }
#if JERRY_BUILTIN_PROXY
    case ECMA_OBJECT_TYPE_PROXY:
    {
      ext_object_size = sizeof (ecma_proxy_object_t);
      break;
    }
#endif /* JERRY_BUILTIN_PROXY */
    case ECMA_OBJECT_TYPE_FUNCTION:
    {
      /* Function with byte-code (not a built-in function). */
      ecma_extended_object_t *ext_func_p = (ecma_extended_object_t *) object_p;

#if JERRY_SNAPSHOT_EXEC
      if (ext_func_p->u.function.bytecode_cp != ECMA_NULL_POINTER)
      {
#endif /* JERRY_SNAPSHOT_EXEC */
        ecma_compiled_code_t *byte_code_p =
          (ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, ext_func_p->u.function.bytecode_cp));

        if (CBC_FUNCTION_IS_ARROW (byte_code_p->status_flags))
        {
          ecma_free_value_if_not_object (((ecma_arrow_function_t *) object_p)->this_binding);
          ecma_free_value_if_not_object (((ecma_arrow_function_t *) object_p)->new_target);
          ext_object_size = sizeof (ecma_arrow_function_t);
        }

        ecma_bytecode_deref (byte_code_p);
#if JERRY_SNAPSHOT_EXEC
      }
      else
      {
        ext_object_size = sizeof (ecma_static_function_t);
      }
#endif /* JERRY_SNAPSHOT_EXEC */
      break;
    }
    case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
    {
      ecma_extended_object_t *extended_func_p = (ecma_extended_object_t *) object_p;

      if (!ecma_builtin_function_is_routine (object_p))
      {
        uint8_t bitset_size = extended_func_p->u.built_in.u.length_and_bitset_size;
        ext_object_size += sizeof (uint64_t) * (bitset_size >> ECMA_BUILT_IN_BITSET_SHIFT);
        break;
      }

      if (extended_func_p->u.built_in.id != ECMA_BUILTIN_ID_HANDLER)
      {
        break;
      }

      switch (extended_func_p->u.built_in.routine_id)
      {
        case ECMA_NATIVE_HANDLER_PROMISE_RESOLVE:
        case ECMA_NATIVE_HANDLER_PROMISE_REJECT:
        {
          ext_object_size = sizeof (ecma_promise_resolver_t);
          break;
        }
        case ECMA_NATIVE_HANDLER_PROMISE_THEN_FINALLY:
        case ECMA_NATIVE_HANDLER_PROMISE_CATCH_FINALLY:
        {
          ext_object_size = sizeof (ecma_promise_finally_function_t);
          break;
        }
        case ECMA_NATIVE_HANDLER_PROMISE_CAPABILITY_EXECUTOR:
        {
          ext_object_size = sizeof (ecma_promise_capability_executor_t);
          break;
        }
        case ECMA_NATIVE_HANDLER_PROMISE_ALL_HELPER:
        {
          ext_object_size = sizeof (ecma_promise_all_executor_t);
          break;
        }
#if JERRY_BUILTIN_PROXY
        case ECMA_NATIVE_HANDLER_PROXY_REVOKE:
        {
          ext_object_size = sizeof (ecma_revocable_proxy_object_t);
          break;
        }
#endif /* JERRY_BUILTIN_PROXY */
        case ECMA_NATIVE_HANDLER_VALUE_THUNK:
        case ECMA_NATIVE_HANDLER_VALUE_THROWER:
        {
          ecma_free_value_if_not_object (((ecma_promise_value_thunk_t *) object_p)->value);
          ext_object_size = sizeof (ecma_promise_value_thunk_t);
          break;
        }
        case ECMA_NATIVE_HANDLER_ASYNC_FROM_SYNC_ITERATOR_UNWRAP:
        {
          break;
        }
        default:
        {
          JERRY_UNREACHABLE ();
        }
      }
      break;
    }
    case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
    {
      ext_object_size = sizeof (ecma_bound_function_t);
      ecma_bound_function_t *bound_func_p = (ecma_bound_function_t *) object_p;

      ecma_value_t args_len_or_this = bound_func_p->header.u.bound_function.args_len_or_this;

      ecma_free_value (bound_func_p->target_length);

      if (!ecma_is_value_integer_number (args_len_or_this))
      {
        ecma_free_value_if_not_object (args_len_or_this);
        break;
      }

      ecma_integer_value_t args_length = ecma_get_integer_from_value (args_len_or_this);
      ecma_value_t *args_p = (ecma_value_t *) (bound_func_p + 1);

      for (ecma_integer_value_t i = 0; i < args_length; i++)
      {
        ecma_free_value_if_not_object (args_p[i]);
      }

      size_t args_size = ((size_t) args_length) * sizeof (ecma_value_t);
      ext_object_size += args_size;
      break;
    }
    case ECMA_OBJECT_TYPE_CONSTRUCTOR_FUNCTION:
    {
      ecma_script_deref (((ecma_extended_object_t *) object_p)->u.constructor_function.script_value);
      break;
    }
    case ECMA_OBJECT_TYPE_NATIVE_FUNCTION:
    {
      ext_object_size = sizeof (ecma_native_function_t);
      break;
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }

  ecma_gc_free_properties (object_p, ECMA_GC_FREE_NO_OPTIONS);
  ecma_dealloc_extended_object (object_p, ext_object_size);
} /* ecma_gc_free_object */

/**
 * Run garbage collection, freeing objects that are no longer referenced.
 */
void
ecma_gc_run (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if (JERRY_GC_MARK_LIMIT != 0)
  JERRY_ASSERT (JERRY_CONTEXT (ecma_gc_mark_recursion_limit) == JERRY_GC_MARK_LIMIT);
#endif /* (JERRY_GC_MARK_LIMIT != 0) */

  JERRY_CONTEXT (ecma_gc_new_objects) = 0;

  ecma_object_t black_list_head;
  black_list_head.gc_next_cp = JMEM_CP_NULL;
  ecma_object_t *black_end_p = &black_list_head;

  ecma_object_t white_gray_list_head;
  white_gray_list_head.gc_next_cp = JERRY_CONTEXT (ecma_gc_objects_cp);

  ecma_object_t *obj_prev_p = &white_gray_list_head;
  jmem_cpointer_t obj_iter_cp = obj_prev_p->gc_next_cp;
  ecma_object_t *obj_iter_p;

  /* Move root objects (i.e. they have global or stack references) to the black list. */
  while (obj_iter_cp != JMEM_CP_NULL)
  {
    obj_iter_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_object_t, obj_iter_cp);
    const jmem_cpointer_t obj_next_cp = obj_iter_p->gc_next_cp;

    JERRY_ASSERT (obj_prev_p == NULL
                  || ECMA_GET_NON_NULL_POINTER (ecma_object_t, obj_prev_p->gc_next_cp) == obj_iter_p);

    if (obj_iter_p->type_flags_refs >= ECMA_OBJECT_REF_ONE)
    {
      /* Moving the object to list of marked objects. */
      obj_prev_p->gc_next_cp = obj_next_cp;

      black_end_p->gc_next_cp = obj_iter_cp;
      black_end_p = obj_iter_p;
    }
    else
    {
      obj_iter_p->type_flags_refs |= ECMA_OBJECT_NON_VISITED;
      obj_prev_p = obj_iter_p;
    }

    obj_iter_cp = obj_next_cp;
  }

  black_end_p->gc_next_cp = JMEM_CP_NULL;

  /* Mark root objects. */
  obj_iter_cp = black_list_head.gc_next_cp;
  while (obj_iter_cp != JMEM_CP_NULL)
  {
    obj_iter_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_object_t, obj_iter_cp);
    ecma_gc_mark (obj_iter_p);
    obj_iter_cp = obj_iter_p->gc_next_cp;
  }

  /* Mark non-root objects. */
  bool marked_anything_during_current_iteration;

  do
  {
#if (JERRY_GC_MARK_LIMIT != 0)
    JERRY_ASSERT (JERRY_CONTEXT (ecma_gc_mark_recursion_limit) == JERRY_GC_MARK_LIMIT);
#endif /* (JERRY_GC_MARK_LIMIT != 0) */

    marked_anything_during_current_iteration = false;

    obj_prev_p = &white_gray_list_head;
    obj_iter_cp = obj_prev_p->gc_next_cp;

    while (obj_iter_cp != JMEM_CP_NULL)
    {
      obj_iter_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_object_t, obj_iter_cp);
      const jmem_cpointer_t obj_next_cp = obj_iter_p->gc_next_cp;

      JERRY_ASSERT (obj_prev_p == NULL
                    || ECMA_GET_NON_NULL_POINTER (ecma_object_t, obj_prev_p->gc_next_cp) == obj_iter_p);

      if (ecma_gc_is_object_visited (obj_iter_p))
      {
        /* Moving the object to list of marked objects */
        obj_prev_p->gc_next_cp = obj_next_cp;

        black_end_p->gc_next_cp = obj_iter_cp;
        black_end_p = obj_iter_p;

#if (JERRY_GC_MARK_LIMIT != 0)
        if (obj_iter_p->type_flags_refs >= ECMA_OBJECT_REF_ONE)
        {
          /* Set the reference count of non-marked gray object to 0 */
          obj_iter_p->type_flags_refs &= (ecma_object_descriptor_t) (ECMA_OBJECT_REF_ONE - 1);
          ecma_gc_mark (obj_iter_p);
          marked_anything_during_current_iteration = true;
        }
#else /* (JERRY_GC_MARK_LIMIT == 0) */
        marked_anything_during_current_iteration = true;
#endif /* (JERRY_GC_MARK_LIMIT != 0) */
      }
      else
      {
        obj_prev_p = obj_iter_p;
      }

      obj_iter_cp = obj_next_cp;
    }
  } while (marked_anything_during_current_iteration);

  black_end_p->gc_next_cp = JMEM_CP_NULL;
  JERRY_CONTEXT (ecma_gc_objects_cp) = black_list_head.gc_next_cp;

  /* Sweep objects that are currently unmarked. */
  obj_iter_cp = white_gray_list_head.gc_next_cp;

  while (obj_iter_cp != JMEM_CP_NULL)
  {
    obj_iter_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_object_t, obj_iter_cp);
    const jmem_cpointer_t obj_next_cp = obj_iter_p->gc_next_cp;

    JERRY_ASSERT (!ecma_gc_is_object_visited (obj_iter_p));

    ecma_gc_free_object (obj_iter_p);
    obj_iter_cp = obj_next_cp;
  }

#if JERRY_BUILTIN_REGEXP
  /* Free RegExp bytecodes stored in cache */
  re_cache_gc ();
#endif /* JERRY_BUILTIN_REGEXP */
} /* ecma_gc_run */

/**
 * Try to free some memory (depending on memory pressure).
 *
 * When called with JMEM_PRESSURE_FULL, the engine will be terminated with JERRY_FATAL_OUT_OF_MEMORY.
 */
void
ecma_free_unused_memory (jmem_pressure_t pressure) /**< current pressure */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (JERRY_LIKELY (pressure == JMEM_PRESSURE_LOW))
  {
#if JERRY_PROPERTY_HASHMAP
    if (JERRY_CONTEXT (ecma_prop_hashmap_alloc_state) > ECMA_PROP_HASHMAP_ALLOC_ON)
    {
      --JERRY_CONTEXT (ecma_prop_hashmap_alloc_state);
    }
    JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_HIGH_PRESSURE_GC;
#endif /* JERRY_PROPERTY_HASHMAP */
    /*
     * If there is enough newly allocated objects since last GC, probably it is worthwhile to start GC now.
     * Otherwise, probability to free sufficient space is considered to be low.
     */
    size_t new_objects_fraction = CONFIG_ECMA_GC_NEW_OBJECTS_FRACTION;

    if (JERRY_CONTEXT (ecma_gc_new_objects) * new_objects_fraction > JERRY_CONTEXT (ecma_gc_objects_number))
    {
      ecma_gc_run ();
    }

    return;
  }
  else if (pressure == JMEM_PRESSURE_HIGH)
  {
    /* Freeing as much memory as we currently can */
#if JERRY_PROPERTY_HASHMAP
    if (JERRY_CONTEXT (status_flags) & ECMA_STATUS_HIGH_PRESSURE_GC)
    {
      JERRY_CONTEXT (ecma_prop_hashmap_alloc_state) = ECMA_PROP_HASHMAP_ALLOC_MAX;
    }
    else if (JERRY_CONTEXT (ecma_prop_hashmap_alloc_state) < ECMA_PROP_HASHMAP_ALLOC_MAX)
    {
      ++JERRY_CONTEXT (ecma_prop_hashmap_alloc_state);
      JERRY_CONTEXT (status_flags) |= ECMA_STATUS_HIGH_PRESSURE_GC;
    }
#endif /* JERRY_PROPERTY_HASHMAP */

    ecma_gc_run ();

#if JERRY_PROPERTY_HASHMAP
    /* Free hashmaps of remaining objects. */
    jmem_cpointer_t obj_iter_cp = JERRY_CONTEXT (ecma_gc_objects_cp);

    while (obj_iter_cp != JMEM_CP_NULL)
    {
      ecma_object_t *obj_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, obj_iter_cp);

      if (!ecma_is_lexical_environment (obj_iter_p)
          || ecma_get_lex_env_type (obj_iter_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
      {
        if (!ecma_is_lexical_environment (obj_iter_p) && ecma_op_object_is_fast_array (obj_iter_p))
        {
          obj_iter_cp = obj_iter_p->gc_next_cp;
          continue;
        }

        jmem_cpointer_t prop_iter_cp = obj_iter_p->u1.property_list_cp;

        if (prop_iter_cp != JMEM_CP_NULL)
        {
          ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

          if (prop_iter_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
          {
            ecma_property_hashmap_free (obj_iter_p);
          }
        }
      }

      obj_iter_cp = obj_iter_p->gc_next_cp;
    }
#endif /* JERRY_PROPERTY_HASHMAP */

    jmem_pools_collect_empty ();
    return;
  }
  else if (JERRY_UNLIKELY (pressure == JMEM_PRESSURE_FULL))
  {
    jerry_fatal (JERRY_FATAL_OUT_OF_MEMORY);
  }
  else
  {
    JERRY_ASSERT (pressure == JMEM_PRESSURE_NONE);
    JERRY_UNREACHABLE ();
  }
} /* ecma_free_unused_memory */

/**
 * @}
 * @}
 */
