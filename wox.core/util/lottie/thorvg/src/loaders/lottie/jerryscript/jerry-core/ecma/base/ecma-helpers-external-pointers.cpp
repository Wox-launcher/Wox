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

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmahelpers Helpers for operations with ECMA data types
 * @{
 */

/**
 * Create a native pointer property to store the native pointer and its type info.
 *
 * @return true - if property was just created with specified value,
 *         false - otherwise, if property existed before the call, it's value was updated
 */
bool
ecma_create_native_pointer_property (ecma_object_t *obj_p, /**< object to create property in */
                                     void *native_p, /**< native pointer */
                                     const jerry_object_native_info_t *native_info_p) /**< native type info */
{
  ecma_string_t *name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER);

  if (native_info_p != NULL && native_info_p->number_of_references > 0)
  {
    name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER_WITH_REFERENCES);
  }

  if (ecma_op_object_is_fast_array (obj_p))
  {
    ecma_fast_array_convert_to_normal (obj_p);
  }

  ecma_property_t *property_p = ecma_find_named_property (obj_p, name_p);

  bool is_new = (property_p == NULL);

  ecma_native_pointer_t *native_pointer_p;

  if (property_p == NULL)
  {
    native_pointer_p = (ecma_native_pointer_t *) jmem_heap_alloc_block (sizeof (ecma_native_pointer_t));

    ecma_property_value_t *value_p;
    ECMA_CREATE_INTERNAL_PROPERTY (obj_p, name_p, property_p, value_p);

    ECMA_SET_INTERNAL_VALUE_POINTER (value_p->value, native_pointer_p);
    *property_p |= ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL;
  }
  else if (*property_p & ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL)
  {
    ecma_property_value_t *value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

    native_pointer_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_t, value_p->value);

    if (native_pointer_p->native_info_p == native_info_p)
    {
      native_pointer_p->native_p = native_p;
      return false;
    }

    value_p->value = JMEM_CP_NULL;
    (void) value_p->value; /* Make cppcheck happy. */
    *property_p &= (ecma_property_t) ~ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL;

    ecma_native_pointer_chain_t *item_p;
    item_p = (ecma_native_pointer_chain_t *) jmem_heap_alloc_block (sizeof (ecma_native_pointer_chain_t));
    item_p->data = *native_pointer_p;

    jmem_heap_free_block (native_pointer_p, sizeof (ecma_native_pointer_t));

    item_p->next_p = (ecma_native_pointer_chain_t *) jmem_heap_alloc_block (sizeof (ecma_native_pointer_chain_t));
    item_p->next_p->next_p = NULL;

    native_pointer_p = &item_p->next_p->data;
    ECMA_SET_INTERNAL_VALUE_POINTER (value_p->value, item_p);
  }
  else
  {
    ecma_property_value_t *value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

    if (value_p->value == JMEM_CP_NULL)
    {
      native_pointer_p = (ecma_native_pointer_t *) jmem_heap_alloc_block (sizeof (ecma_native_pointer_t));
      ECMA_SET_INTERNAL_VALUE_POINTER (value_p->value, native_pointer_p);

      *property_p |= ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL;
    }
    else
    {
      ecma_native_pointer_chain_t *item_p;
      item_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_chain_t, value_p->value);

      /* There should be at least 2 native pointers in the chain */
      JERRY_ASSERT (item_p != NULL && item_p->next_p != NULL);

      while (true)
      {
        if (item_p->data.native_info_p == native_info_p)
        {
          /* The native info already exists -> update the corresponding data */
          item_p->data.native_p = native_p;
          return false;
        }

        if (item_p->next_p == NULL)
        {
          /* The native info does not exist -> append a new element to the chain */
          break;
        }

        item_p = item_p->next_p;
      }

      ecma_native_pointer_chain_t *new_item_p;

      new_item_p = (ecma_native_pointer_chain_t *) jmem_heap_alloc_block (sizeof (ecma_native_pointer_chain_t));
      item_p->next_p = new_item_p;
      new_item_p->next_p = NULL;

      native_pointer_p = &new_item_p->data;
    }
  }

  native_pointer_p->native_p = native_p;
  native_pointer_p->native_info_p = (jerry_object_native_info_t *) native_info_p;

  return is_new;
} /* ecma_create_native_pointer_property */

/**
 * Get value of native package stored in the object's property with specified identifier
 *
 * Note:
 *      property identifier should be one of the following:
 *        - LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER
 *
 * @return native pointer data if property exists
 *         NULL otherwise
 */
ecma_native_pointer_t *
ecma_get_native_pointer_value (ecma_object_t *obj_p, /**< object to get property value from */
                               const jerry_object_native_info_t *native_info_p) /**< native type info */
{
  if (ecma_op_object_is_fast_array (obj_p))
  {
    /* Fast access mode array cannot have native pointer properties */
    return NULL;
  }

  ecma_string_t *name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER);

  if (native_info_p != NULL && native_info_p->number_of_references > 0)
  {
    name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER_WITH_REFERENCES);
  }

  ecma_property_t *property_p = ecma_find_named_property (obj_p, name_p);

  if (property_p == NULL)
  {
    return NULL;
  }

  ecma_property_value_t *value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

  if (JERRY_LIKELY (*property_p & ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL))
  {
    ecma_native_pointer_t *native_pointer_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_t, value_p->value);

    if (native_pointer_p->native_info_p == native_info_p)
    {
      return native_pointer_p;
    }

    return NULL;
  }

  if (value_p->value == JMEM_CP_NULL)
  {
    return NULL;
  }

  ecma_native_pointer_chain_t *item_p;
  item_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_chain_t, value_p->value);

  /* There should be at least 2 native pointers in the chain */
  JERRY_ASSERT (item_p != NULL && item_p->next_p != NULL);

  do
  {
    if (item_p->data.native_info_p == native_info_p)
    {
      return &item_p->data;
    }

    item_p = item_p->next_p;
  } while (item_p != NULL);

  return NULL;
} /* ecma_get_native_pointer_value */

/**
 * Delete the previously set native pointer by the native type info from the specified object.
 *
 * Note:
 *      If the specified object has no matching native pointer for the given native type info
 *      the function has no effect.
 *
 * @return true - if the native pointer has been deleted successfully
 *         false - otherwise
 */
bool
ecma_delete_native_pointer_property (ecma_object_t *obj_p, /**< object to delete property from */
                                     const jerry_object_native_info_t *native_info_p) /**< native type info */
{
  if (ecma_op_object_is_fast_array (obj_p))
  {
    /* Fast access mode array cannot have native pointer properties */
    return false;
  }

  ecma_string_t *name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER);

  if (native_info_p != NULL && native_info_p->number_of_references > 0)
  {
    name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER_WITH_REFERENCES);
  }

  ecma_property_t *property_p = ecma_find_named_property (obj_p, name_p);

  if (property_p == NULL)
  {
    return false;
  }

  ecma_property_value_t *value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

  if (JERRY_LIKELY (*property_p & ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL))
  {
    ecma_native_pointer_t *native_pointer_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_t, value_p->value);

    if (native_pointer_p->native_info_p != native_info_p)
    {
      return false;
    }

    value_p->value = JMEM_CP_NULL;
    *property_p &= (ecma_property_t) ~ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL;
    jmem_heap_free_block (native_pointer_p, sizeof (ecma_native_pointer_t));
    return true;
  }

  if (value_p->value == JMEM_CP_NULL)
  {
    return false;
  }

  ecma_native_pointer_chain_t *first_p;
  first_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_native_pointer_chain_t, value_p->value);

  /* There should be at least 2 native pointers in the chain */
  JERRY_ASSERT (first_p != NULL && first_p->next_p != NULL);

  ecma_native_pointer_chain_t *item_p = first_p;
  ecma_native_pointer_chain_t *prev_p = NULL;

  do
  {
    if (item_p->data.native_info_p == native_info_p)
    {
      if (prev_p == NULL)
      {
        /* The first element is deleted from the chain: change the property value. */
        first_p = item_p->next_p;
        ECMA_SET_INTERNAL_VALUE_POINTER (value_p->value, first_p);
      }
      else
      {
        /* A non-first element is deleted from the chain: update the previous pointer. */
        prev_p->next_p = item_p->next_p;
      }

      jmem_heap_free_block (item_p, sizeof (ecma_native_pointer_chain_t));

      if (first_p->next_p != NULL)
      {
        return true;
      }

      /* Only one item remained. The ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL flag is
       * set early to avoid using the chain if the allocation below triggers a GC. */
      *property_p |= ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL;

      ecma_native_pointer_t *native_pointer_p;
      native_pointer_p = (ecma_native_pointer_t *) jmem_heap_alloc_block (sizeof (ecma_native_pointer_t));
      *native_pointer_p = first_p->data;

      ECMA_SET_INTERNAL_VALUE_POINTER (value_p->value, native_pointer_p);

      jmem_heap_free_block (first_p, sizeof (ecma_native_pointer_chain_t));
      return true;
    }

    prev_p = item_p;
    item_p = item_p->next_p;
  } while (item_p != NULL);

  return false;
} /* ecma_delete_native_pointer_property */

/**
 * @}
 * @}
 */
