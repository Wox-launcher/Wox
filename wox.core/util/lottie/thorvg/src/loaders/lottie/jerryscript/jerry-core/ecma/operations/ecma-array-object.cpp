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

#include "ecma-array-object.h"

#include "ecma-alloc.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"
#include "ecma-property-hashmap.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaarrayobject ECMA Array object related routines
 * @{
 */

/**
 * Property name type flag for array indices.
 */
#define ECMA_FAST_ARRAY_UINT_DIRECT_STRING_PROP_TYPE (ECMA_DIRECT_STRING_UINT << ECMA_PROPERTY_NAME_TYPE_SHIFT)

/**
 * Allocate a new array object with the given length
 *
 * @return pointer to the constructed array object
 */
static ecma_object_t *
ecma_op_alloc_array_object (uint32_t length) /**< length of the new array */
{
#if JERRY_BUILTIN_ARRAY
  ecma_object_t *array_prototype_object_p = ecma_builtin_get (ECMA_BUILTIN_ID_ARRAY_PROTOTYPE);
#else /* !JERRY_BUILTIN_ARRAY */
  ecma_object_t *array_prototype_object_p = ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE);
#endif /* JERRY_BUILTIN_ARRAY */

  ecma_object_t *object_p =
    ecma_create_object (array_prototype_object_p, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_ARRAY);

  /*
   * [[Class]] property is not stored explicitly for objects of ECMA_OBJECT_TYPE_ARRAY type.
   *
   * See also: ecma_object_get_class_name
   */

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  ext_obj_p->u.array.length = length;
  ext_obj_p->u.array.length_prop_and_hole_count = ECMA_PROPERTY_FLAG_WRITABLE | ECMA_PROPERTY_VIRTUAL;

  return object_p;
} /* ecma_op_alloc_array_object */

/**
 * Check whether the given object is fast-access mode array
 *
 * @return true - if the object is fast-access mode array
 *         false, otherwise
 */
bool 
ecma_op_object_is_fast_array (ecma_object_t *object_p) /**< ecma-object */
{
  return (ecma_get_object_base_type (object_p) == ECMA_OBJECT_BASE_TYPE_ARRAY
          && ecma_op_array_is_fast_array ((ecma_extended_object_t *) object_p));
} /* ecma_op_object_is_fast_array */

/**
 * Check whether the given array object is fast-access mode array
 *
 * @return true - if the array object is fast-access mode array
 *         false, otherwise
 */
bool 
ecma_op_array_is_fast_array (ecma_extended_object_t *array_p) /**< ecma-array-object */
{
  JERRY_ASSERT (ecma_get_object_base_type ((ecma_object_t *) array_p) == ECMA_OBJECT_BASE_TYPE_ARRAY);

  return array_p->u.array.length_prop_and_hole_count & ECMA_FAST_ARRAY_FLAG;
} /* ecma_op_array_is_fast_array */

/**
 * Allocate a new array object with the given length
 *
 * Note: The returned array can be normal of fast access mode
 *
 * @return pointer to the constructed array object
 */
ecma_object_t *
ecma_op_new_array_object (uint32_t length) /**< length of the new array */
{
  ecma_object_t *object_p = ecma_op_alloc_array_object (length);

  const uint32_t aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (length);
  ecma_value_t *values_p = NULL;

  if (length > 0)
  {
    if (length >= ECMA_FAST_ARRAY_MAX_HOLE_COUNT)
    {
      return object_p;
    }

    values_p = (ecma_value_t *) jmem_heap_alloc_block_null_on_error (aligned_length * sizeof (ecma_value_t));

    if (JERRY_UNLIKELY (values_p == NULL))
    {
      return object_p;
    }
  }

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  ext_obj_p->u.array.length_prop_and_hole_count |= ECMA_FAST_ARRAY_FLAG;
  ext_obj_p->u.array.length_prop_and_hole_count += length * ECMA_FAST_ARRAY_HOLE_ONE;

  for (uint32_t i = 0; i < aligned_length; i++)
  {
    values_p[i] = ECMA_VALUE_ARRAY_HOLE;
  }

  JERRY_ASSERT (object_p->u1.property_list_cp == JMEM_CP_NULL);
  ECMA_SET_POINTER (object_p->u1.property_list_cp, values_p);
  return object_p;
} /* ecma_op_new_array_object */

/**
 * Allocate a new array object from the given length
 *
 * Note: The returned array can be normal of fast access mode
 *
 * @return NULL - if the given length is invalid
 *         pointer to the constructed array object - otherwise
 */
ecma_object_t *
ecma_op_new_array_object_from_length (ecma_length_t length) /**< length of the new array */
{
  if (length > UINT32_MAX)
  {
    ecma_raise_range_error (ECMA_ERR_INVALID_ARRAY_LENGTH);
    return NULL;
  }

  return ecma_op_new_array_object ((uint32_t) length);
} /* ecma_op_new_array_object_from_length */

/**
 * Allocate a new array object from the given buffer
 *
 * Note: The returned array can be normal of fast access mode
 *
 * @return ecma_value - constructed array object
 */
ecma_value_t
ecma_op_new_array_object_from_buffer (const ecma_value_t *args_p, /**< array element list */
                                      uint32_t length) /**< number of array elements */
{
  if (length == 0)
  {
    return ecma_make_object_value (ecma_op_new_array_object (0));
  }

  ecma_object_t *object_p = ecma_op_alloc_array_object (length);
  const uint32_t aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (length);
  ecma_value_t *values_p;
  values_p = (ecma_value_t *) jmem_heap_alloc_block_null_on_error (aligned_length * sizeof (ecma_value_t));

  if (values_p != NULL)
  {
    ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
    ext_obj_p->u.array.length_prop_and_hole_count |= ECMA_FAST_ARRAY_FLAG;
    JERRY_ASSERT (object_p->u1.property_list_cp == JMEM_CP_NULL);

    for (uint32_t i = 0; i < length; i++)
    {
      JERRY_ASSERT (!ecma_is_value_array_hole (args_p[i]));
      values_p[i] = ecma_copy_value_if_not_object (args_p[i]);
    }

    for (uint32_t i = length; i < aligned_length; i++)
    {
      values_p[i] = ECMA_VALUE_ARRAY_HOLE;
    }

    ECMA_SET_POINTER (object_p->u1.property_list_cp, values_p);
  }
  else
  {
    for (uint32_t i = 0; i < length; i++)
    {
      JERRY_ASSERT (!ecma_is_value_array_hole (args_p[i]));

      ecma_string_t *prop_name_p = ecma_new_ecma_string_from_uint32 (i);
      ecma_property_value_t *prop_value_p;
      prop_value_p =
        ecma_create_named_data_property (object_p, prop_name_p, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE, NULL);
      ecma_deref_ecma_string (prop_name_p);
      prop_value_p->value = ecma_copy_value_if_not_object (args_p[i]);
    }
  }

  return ecma_make_object_value (object_p);
} /* ecma_op_new_array_object_from_buffer */

/**
 * Allocate a new fast access mode array object from the given collection
 *
 * Note: The given collection will be unavailable after and it's underlying buffer is reused
 *
 * @return ecma_value - constructed fast access mode array object
 */
ecma_value_t
ecma_op_new_array_object_from_collection (ecma_collection_t *collection_p, /**< collection to create array from */
                                          bool unref_objects) /**< true - if the collection potentially
                                                                          contains objects
                                                                   false - otherwise */
{
  const uint32_t item_count = collection_p->item_count;

  if (item_count == 0)
  {
    ecma_collection_destroy (collection_p);
    return ecma_make_object_value (ecma_op_new_array_object (0));
  }

  ecma_object_t *object_p;
  ecma_value_t *buffer_p = collection_p->buffer_p;
  const uint32_t old_size = ECMA_COLLECTION_ALLOCATED_SIZE (collection_p->capacity);
  const uint32_t aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (collection_p->item_count);

  jmem_heap_free_block (collection_p, sizeof (ecma_collection_t));
  buffer_p = (ecma_value_t *) jmem_heap_realloc_block (buffer_p, old_size, aligned_length * sizeof (ecma_value_t));
  object_p = (ecma_object_t *) ecma_op_alloc_array_object (item_count);

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  ext_obj_p->u.array.length_prop_and_hole_count |= ECMA_FAST_ARRAY_FLAG;
  JERRY_ASSERT (object_p->u1.property_list_cp == JMEM_CP_NULL);
  JERRY_ASSERT (ext_obj_p->u.array.length_prop_and_hole_count < ECMA_FAST_ARRAY_HOLE_ONE);
  ECMA_SET_POINTER (object_p->u1.property_list_cp, buffer_p);

  if (JERRY_UNLIKELY (unref_objects))
  {
    for (uint32_t i = 0; i < item_count; i++)
    {
      /* Strong references from the collection are no longer needed
         since GC will mark these object as a fast access mode array properties */
      ecma_deref_if_object (buffer_p[i]);
    }
  }

  for (uint32_t i = item_count; i < aligned_length; i++)
  {
    buffer_p[i] = ECMA_VALUE_ARRAY_HOLE;
  }

  return ecma_make_object_value (object_p);
} /* ecma_op_new_array_object_from_collection */

/**
 * Converts a fast access mode array back to a normal property list based array
 */
void
ecma_fast_array_convert_to_normal (ecma_object_t *object_p) /**< fast access mode array object */
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (object_p));

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;

  if (object_p->u1.property_list_cp == JMEM_CP_NULL)
  {
    ext_obj_p->u.array.length_prop_and_hole_count &= (uint32_t) ~ECMA_FAST_ARRAY_FLAG;
    return;
  }

  uint32_t length = ext_obj_p->u.array.length;
  const uint32_t aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (length);
  ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

  ecma_ref_object (object_p);

  ecma_property_pair_t *property_pair_p = NULL;
  jmem_cpointer_t next_property_pair_cp = JMEM_CP_NULL;

  uint32_t prop_index = 1;
  int32_t index = (int32_t) (length - 1);

  while (index >= 0)
  {
    if (ecma_is_value_array_hole (values_p[index]))
    {
      index--;
      continue;
    }

    if (prop_index == 1)
    {
      property_pair_p = ecma_alloc_property_pair ();
      property_pair_p->header.next_property_cp = next_property_pair_cp;
      property_pair_p->names_cp[0] = LIT_INTERNAL_MAGIC_STRING_DELETED;
      property_pair_p->header.types[0] = ECMA_PROPERTY_TYPE_DELETED;
      ECMA_SET_NON_NULL_POINTER (next_property_pair_cp, property_pair_p);
    }

    JERRY_ASSERT (index <= ECMA_DIRECT_STRING_MAX_IMM);

    property_pair_p->names_cp[prop_index] = (jmem_cpointer_t) index;
    property_pair_p->header.types[prop_index] =
      (ecma_property_t) (ECMA_PROPERTY_FLAG_DATA | ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE
                         | ECMA_FAST_ARRAY_UINT_DIRECT_STRING_PROP_TYPE);

    property_pair_p->values[prop_index].value = values_p[index];

    index--;
    prop_index = !prop_index;
  }

  ext_obj_p->u.array.length_prop_and_hole_count &= (uint32_t) ~ECMA_FAST_ARRAY_FLAG;
  jmem_heap_free_block (values_p, aligned_length * sizeof (ecma_value_t));
  ECMA_SET_POINTER (object_p->u1.property_list_cp, property_pair_p);

  ecma_deref_object (object_p);
} /* ecma_fast_array_convert_to_normal */

/**
 * [[Put]] operation for a fast access mode array
 *
 * @return false - If the property name is not array index, or the requested index to be set
 *                 would result too much array hole in the underlying buffer. The these cases
 *                 the array is converted back to normal property list based array.
 *         true - If the indexed property can be set with/without resizing the underlying buffer.
 */
bool
ecma_fast_array_set_property (ecma_object_t *object_p, /**< fast access mode array object */
                              uint32_t index, /**< property name index */
                              ecma_value_t value) /**< value to be set */
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (object_p));
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  uint32_t old_length = ext_obj_p->u.array.length;

  ecma_value_t *values_p;

  if (JERRY_LIKELY (index < old_length))
  {
    JERRY_ASSERT (object_p->u1.property_list_cp != JMEM_CP_NULL);

    values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

    if (ecma_is_value_array_hole (values_p[index]))
    {
      ext_obj_p->u.array.length_prop_and_hole_count -= ECMA_FAST_ARRAY_HOLE_ONE;
    }
    else
    {
      ecma_free_value_if_not_object (values_p[index]);
    }

    values_p[index] = ecma_copy_value_if_not_object (value);

    return true;
  }

  uint32_t old_holes = ext_obj_p->u.array.length_prop_and_hole_count;
  uint32_t new_holes = index - old_length;

  if (JERRY_UNLIKELY (new_holes > (ECMA_FAST_ARRAY_MAX_HOLE_COUNT - (old_holes >> ECMA_FAST_ARRAY_HOLE_SHIFT))))
  {
    ecma_fast_array_convert_to_normal (object_p);

    return false;
  }

  uint32_t new_length = index + 1;

  JERRY_ASSERT (new_length < UINT32_MAX);

  const uint32_t aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (old_length);

  if (JERRY_LIKELY (index < aligned_length))
  {
    JERRY_ASSERT (object_p->u1.property_list_cp != JMEM_CP_NULL);

    values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);
    /* This area is filled with ECMA_VALUE_ARRAY_HOLE, but not counted in u.array.length_prop_and_hole_count */
    JERRY_ASSERT (ecma_is_value_array_hole (values_p[index]));
    ext_obj_p->u.array.length_prop_and_hole_count += new_holes * ECMA_FAST_ARRAY_HOLE_ONE;
    ext_obj_p->u.array.length = new_length;
  }
  else
  {
    values_p = ecma_fast_array_extend (object_p, new_length);
    ext_obj_p->u.array.length_prop_and_hole_count -= ECMA_FAST_ARRAY_HOLE_ONE;
  }

  values_p[index] = ecma_copy_value_if_not_object (value);

  return true;
} /* ecma_fast_array_set_property */

/**
 * Get the number of array holes in a fast access array object
 *
 * @return number of array holes in a fast access array object
 */
uint32_t 
ecma_fast_array_get_hole_count (ecma_object_t *obj_p) /**< fast access mode array object */
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (obj_p));

  return ((ecma_extended_object_t *) obj_p)->u.array.length_prop_and_hole_count >> ECMA_FAST_ARRAY_HOLE_SHIFT;
} /* ecma_fast_array_get_hole_count */

/**
 * Extend the underlying buffer of a fast mode access array for the given new length
 *
 * @return pointer to the extended underlying buffer
 */
ecma_value_t *
ecma_fast_array_extend (ecma_object_t *object_p, /**< fast access mode array object */
                        uint32_t new_length) /**< new length of the fast access mode array */
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (object_p));
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  uint32_t old_length = ext_obj_p->u.array.length;

  JERRY_ASSERT (old_length < new_length);

  ecma_ref_object (object_p);

  ecma_value_t *new_values_p;
  const uint32_t old_length_aligned = ECMA_FAST_ARRAY_ALIGN_LENGTH (old_length);
  const uint32_t new_length_aligned = ECMA_FAST_ARRAY_ALIGN_LENGTH (new_length);

  if (object_p->u1.property_list_cp == JMEM_CP_NULL)
  {
    new_values_p = (ecma_value_t *) jmem_heap_alloc_block (new_length_aligned * sizeof (ecma_value_t));
  }
  else
  {
    ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);
    new_values_p = (ecma_value_t *) jmem_heap_realloc_block (values_p,
                                                             old_length_aligned * sizeof (ecma_value_t),
                                                             new_length_aligned * sizeof (ecma_value_t));
  }

  for (uint32_t i = old_length; i < new_length_aligned; i++)
  {
    new_values_p[i] = ECMA_VALUE_ARRAY_HOLE;
  }

  ext_obj_p->u.array.length_prop_and_hole_count += (new_length - old_length) * ECMA_FAST_ARRAY_HOLE_ONE;
  ext_obj_p->u.array.length = new_length;

  ECMA_SET_NON_NULL_POINTER (object_p->u1.property_list_cp, new_values_p);

  ecma_deref_object (object_p);
  return new_values_p;
} /* ecma_fast_array_extend */

/**
 * Delete the array object's property referenced by its value pointer.
 *
 * Note: specified property must be owned by specified object.
 *
 * @return true, if the property is deleted
 *         false, otherwise
 */
bool
ecma_array_object_delete_property (ecma_object_t *object_p, /**< object */
                                   ecma_string_t *property_name_p) /**< property name */
{
  JERRY_ASSERT (ecma_get_object_base_type (object_p) == ECMA_OBJECT_BASE_TYPE_ARRAY);
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;

  if (!ecma_op_object_is_fast_array (object_p))
  {
    return false;
  }

  JERRY_ASSERT (object_p->u1.property_list_cp != JMEM_CP_NULL);

  uint32_t index = ecma_string_get_array_index (property_name_p);

  JERRY_ASSERT (index != ECMA_STRING_NOT_ARRAY_INDEX);
  JERRY_ASSERT (index < ext_obj_p->u.array.length);

  ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

  if (ecma_is_value_array_hole (values_p[index]))
  {
    return true;
  }

  ecma_free_value_if_not_object (values_p[index]);

  values_p[index] = ECMA_VALUE_ARRAY_HOLE;
  ext_obj_p->u.array.length_prop_and_hole_count += ECMA_FAST_ARRAY_HOLE_ONE;
  return true;
} /* ecma_array_object_delete_property */

/**
 * Low level delete of fast access mode array items
 *
 * @return the updated value of new_length
 */
uint32_t
ecma_delete_fast_array_properties (ecma_object_t *object_p, /**< fast access mode array */
                                   uint32_t new_length) /**< new length of the fast access mode array */
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (object_p));

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;

  ecma_ref_object (object_p);
  ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

  uint32_t old_length = ext_obj_p->u.array.length;
  const uint32_t old_aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (old_length);
  JERRY_ASSERT (new_length < old_length);

  for (uint32_t i = new_length; i < old_length; i++)
  {
    if (ecma_is_value_array_hole (values_p[i]))
    {
      ext_obj_p->u.array.length_prop_and_hole_count -= ECMA_FAST_ARRAY_HOLE_ONE;
    }
    else
    {
      ecma_free_value_if_not_object (values_p[i]);
    }
  }

  jmem_cpointer_t new_property_list_cp;

  if (new_length == 0)
  {
    jmem_heap_free_block (values_p, old_aligned_length * sizeof (ecma_value_t));
    new_property_list_cp = JMEM_CP_NULL;
  }
  else
  {
    const uint32_t new_aligned_length = ECMA_FAST_ARRAY_ALIGN_LENGTH (new_length);

    ecma_value_t *new_values_p;
    new_values_p = (ecma_value_t *) jmem_heap_realloc_block (values_p,
                                                             old_aligned_length * sizeof (ecma_value_t),
                                                             new_aligned_length * sizeof (ecma_value_t));

    for (uint32_t i = new_length; i < new_aligned_length; i++)
    {
      new_values_p[i] = ECMA_VALUE_ARRAY_HOLE;
    }

    ECMA_SET_NON_NULL_POINTER (new_property_list_cp, new_values_p);
  }

  ext_obj_p->u.array.length = new_length;
  object_p->u1.property_list_cp = new_property_list_cp;

  ecma_deref_object (object_p);

  return new_length;
} /* ecma_delete_fast_array_properties */

/**
 * Update the length of a fast access mode array to a new length
 *
 * Note: if the new length would result too much array hole in the underlying arraybuffer
 *       the array is converted back to normal property list based array
 */
static void
ecma_fast_array_set_length (ecma_object_t *object_p, /**< fast access mode array object */
                            uint32_t new_length) /**< new length of the fast access mode array object*/
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (object_p));

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  uint32_t old_length = ext_obj_p->u.array.length;

  JERRY_ASSERT (new_length >= old_length);

  if (new_length == old_length)
  {
    return;
  }

  uint32_t old_holes = ext_obj_p->u.array.length_prop_and_hole_count;
  uint32_t new_holes = new_length - old_length;

  if (JERRY_UNLIKELY (new_holes > (ECMA_FAST_ARRAY_MAX_HOLE_COUNT - (old_holes >> ECMA_FAST_ARRAY_HOLE_SHIFT))))
  {
    ecma_fast_array_convert_to_normal (object_p);
  }
  else
  {
    ecma_fast_array_extend (object_p, new_length);
  }
} /* ecma_fast_array_set_length */

/**
 * Get collection of property names of a fast access mode array object
 *
 * Note: Since the fast array object only contains indexed, enumerable, writable, configurable properties
 *       we can return a collection of non-array hole array indices
 *
 * @return collection of strings - property names
 */
ecma_collection_t *
ecma_fast_array_object_own_property_keys (ecma_object_t *object_p, /**< fast access mode array object */
                                          jerry_property_filter_t filter) /**< property name filter options */
{
  JERRY_ASSERT (ecma_op_object_is_fast_array (object_p));

  ecma_collection_t *ret_p = ecma_new_collection ();

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_INTEGER_INDICES))
  {
    ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
    uint32_t length = ext_obj_p->u.array.length;

    if (length != 0)
    {
      ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

      for (uint32_t i = 0; i < length; i++)
      {
        if (ecma_is_value_array_hole (values_p[i]))
        {
          continue;
        }

        ecma_string_t *index_str_p = ecma_new_ecma_string_from_uint32 (i);

        ecma_collection_push_back (ret_p, ecma_make_string_value (index_str_p));
      }
    }
  }

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS))
  {
    ecma_collection_push_back (ret_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_LENGTH));
  }

  return ret_p;
} /* ecma_fast_array_object_own_property_keys */

/**
 * Array object creation with custom prototype.
 *
 * See also: ECMA-262 v6, 9.4.2.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_object_t *
ecma_op_array_species_create (ecma_object_t *original_array_p, /**< The object from whom the new array object
                                                                *   is being created */
                              ecma_length_t length) /**< length of the array */
{
  ecma_value_t constructor = ECMA_VALUE_UNDEFINED;
  ecma_value_t original_array = ecma_make_object_value (original_array_p);

  ecma_value_t is_array = ecma_is_value_array (original_array);

  if (ECMA_IS_VALUE_ERROR (is_array))
  {
    return NULL;
  }

  if (ecma_is_value_true (is_array))
  {
    constructor = ecma_op_object_get_by_magic_id (original_array_p, LIT_MAGIC_STRING_CONSTRUCTOR);
    if (ECMA_IS_VALUE_ERROR (constructor))
    {
      return NULL;
    }

#if JERRY_BUILTIN_REALMS
    if (ecma_is_constructor (constructor))
    {
      ecma_object_t *constructor_p = ecma_get_object_from_value (constructor);
      ecma_global_object_t *global_object_p = ecma_op_function_get_function_realm (constructor_p);

      if ((ecma_object_t *) global_object_p != ecma_builtin_get_global ()
          && constructor_p == ecma_builtin_get_from_realm (global_object_p, ECMA_BUILTIN_ID_ARRAY))
      {
        ecma_deref_object (constructor_p);
        constructor = ECMA_VALUE_UNDEFINED;
      }
    }
#endif /* JERRY_BUILTIN_REALMS */

    if (ecma_is_value_object (constructor))
    {
      ecma_object_t *ctor_object_p = ecma_get_object_from_value (constructor);
      constructor = ecma_op_object_get_by_symbol_id (ctor_object_p, LIT_GLOBAL_SYMBOL_SPECIES);
      ecma_deref_object (ctor_object_p);

      if (ECMA_IS_VALUE_ERROR (constructor))
      {
        return NULL;
      }

      if (ecma_is_value_null (constructor))
      {
        constructor = ECMA_VALUE_UNDEFINED;
      }
    }
  }

  if (ecma_is_value_undefined (constructor))
  {
    return ecma_op_new_array_object_from_length (length);
  }

  if (!ecma_is_constructor (constructor))
  {
    ecma_free_value (constructor);
    ecma_raise_type_error (ECMA_ERR_INVALID_SPECIES_CONSTRUCTOR);
    return NULL;
  }

  ecma_value_t len_val = ecma_make_length_value (length);
  ecma_object_t *ctor_object_p = ecma_get_object_from_value (constructor);
  ecma_value_t ret_val = ecma_op_function_construct (ctor_object_p, ctor_object_p, &len_val, 1);

  ecma_deref_object (ctor_object_p);
  ecma_free_value (len_val);

  if (ECMA_IS_VALUE_ERROR (ret_val))
  {
    return NULL;
  }

  return ecma_get_object_from_value (ret_val);
} /* ecma_op_array_species_create */

/**
 * CreateArrayIterator Abstract Operation
 *
 * See also:
 *          ECMA-262 v6, 22.1.5.1
 *
 * Referenced by:
 *          ECMA-262 v6, 22.1.3.4
 *          ECMA-262 v6, 22.1.3.13
 *          ECMA-262 v6, 22.1.3.29
 *          ECMA-262 v6, 22.1.3.30
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return array iterator object
 */
ecma_value_t
ecma_op_create_array_iterator (ecma_object_t *obj_p, /**< array object */
                               ecma_iterator_kind_t kind) /**< array iterator kind */
{
  ecma_object_t *prototype_obj_p = ecma_builtin_get (ECMA_BUILTIN_ID_ARRAY_ITERATOR_PROTOTYPE);

  return ecma_op_create_iterator_object (ecma_make_object_value (obj_p),
                                         prototype_obj_p,
                                         ECMA_OBJECT_CLASS_ARRAY_ITERATOR,
                                         kind);
} /* ecma_op_create_array_iterator */

/**
 * Low level delete of array items from new_length to old_length
 *
 * Note: new_length must be less than old_length
 *
 * @return the updated value of new_length
 */
static uint32_t
ecma_delete_array_properties (ecma_object_t *object_p, /**< object */
                              uint32_t new_length, /**< new length */
                              uint32_t old_length) /**< old length */
{
  JERRY_ASSERT (new_length < old_length);
  JERRY_ASSERT (ecma_get_object_base_type (object_p) == ECMA_OBJECT_BASE_TYPE_ARRAY);

  if (ecma_op_object_is_fast_array (object_p))
  {
    return ecma_delete_fast_array_properties (object_p, new_length);
  }

  /* First the minimum value of new_length is updated. */
  jmem_cpointer_t current_prop_cp = object_p->u1.property_list_cp;

  if (current_prop_cp == JMEM_CP_NULL)
  {
    return new_length;
  }

  ecma_property_header_t *current_prop_p;

#if JERRY_PROPERTY_HASHMAP
  current_prop_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, current_prop_cp);

  if (current_prop_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
  {
    current_prop_cp = current_prop_p->next_property_cp;
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  while (current_prop_cp != JMEM_CP_NULL)
  {
    current_prop_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, current_prop_cp);
    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (current_prop_p));

    ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) current_prop_p;

    for (int i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      if (current_prop_p->types[i] != ECMA_PROPERTY_TYPE_DELETED
          && !ecma_is_property_configurable (current_prop_p->types[i]))
      {
        uint32_t index = ecma_string_get_property_index (current_prop_p->types[i], prop_pair_p->names_cp[i]);

        if (index < old_length && index >= new_length)
        {
          JERRY_ASSERT (index != ECMA_STRING_NOT_ARRAY_INDEX);

          new_length = index + 1;

          if (new_length == old_length)
          {
            /* Early return. */
            return new_length;
          }
        }
      }
    }

    current_prop_cp = current_prop_p->next_property_cp;
  }

  /* Second all properties between new_length and old_length are deleted. */
  current_prop_cp = object_p->u1.property_list_cp;
  ecma_property_header_t *prev_prop_p = NULL;

#if JERRY_PROPERTY_HASHMAP
  JERRY_ASSERT (current_prop_cp != JMEM_CP_NULL);

  ecma_property_hashmap_delete_status hashmap_status = ECMA_PROPERTY_HASHMAP_DELETE_NO_HASHMAP;
  current_prop_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, current_prop_cp);

  if (current_prop_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
  {
    prev_prop_p = current_prop_p;
    current_prop_cp = current_prop_p->next_property_cp;
    hashmap_status = ECMA_PROPERTY_HASHMAP_DELETE_HAS_HASHMAP;
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  while (current_prop_cp != JMEM_CP_NULL)
  {
    current_prop_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, current_prop_cp);

    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (current_prop_p));
    ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) current_prop_p;

    for (uint32_t i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      if (current_prop_p->types[i] != ECMA_PROPERTY_TYPE_DELETED
          && ecma_is_property_configurable (current_prop_p->types[i]))
      {
        uint32_t index = ecma_string_get_property_index (current_prop_p->types[i], prop_pair_p->names_cp[i]);

        if (index < old_length && index >= new_length)
        {
          JERRY_ASSERT (index != ECMA_STRING_NOT_ARRAY_INDEX);

#if JERRY_PROPERTY_HASHMAP
          if (hashmap_status == ECMA_PROPERTY_HASHMAP_DELETE_HAS_HASHMAP)
          {
            hashmap_status =
              ecma_property_hashmap_delete (object_p, prop_pair_p->names_cp[i], current_prop_p->types + i);
          }
#endif /* JERRY_PROPERTY_HASHMAP */

          ecma_gc_free_property (object_p, prop_pair_p, i);
          current_prop_p->types[i] = ECMA_PROPERTY_TYPE_DELETED;
          prop_pair_p->names_cp[i] = LIT_INTERNAL_MAGIC_STRING_DELETED;
        }
      }
    }

    if (current_prop_p->types[0] == ECMA_PROPERTY_TYPE_DELETED
        && current_prop_p->types[1] == ECMA_PROPERTY_TYPE_DELETED)
    {
      if (prev_prop_p == NULL)
      {
        object_p->u1.property_list_cp = current_prop_p->next_property_cp;
      }
      else
      {
        prev_prop_p->next_property_cp = current_prop_p->next_property_cp;
      }

      jmem_cpointer_t next_prop_cp = current_prop_p->next_property_cp;
      ecma_dealloc_property_pair ((ecma_property_pair_t *) current_prop_p);
      current_prop_cp = next_prop_cp;
    }
    else
    {
      prev_prop_p = current_prop_p;
      current_prop_cp = current_prop_p->next_property_cp;
    }
  }

#if JERRY_PROPERTY_HASHMAP
  if (hashmap_status == ECMA_PROPERTY_HASHMAP_DELETE_RECREATE_HASHMAP)
  {
    ecma_property_hashmap_free (object_p);
    ecma_property_hashmap_create (object_p);
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  return new_length;
} /* ecma_delete_array_properties */

/**
 * Update the length of an array to a new length
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_array_object_set_length (ecma_object_t *object_p, /**< the array object */
                                 ecma_value_t new_value, /**< new length value */
                                 uint16_t flags) /**< property descriptor flags */
{
  ecma_number_t new_len_num;
  ecma_value_t completion = ecma_op_to_number (new_value, &new_len_num);

  if (ECMA_IS_VALUE_ERROR (completion))
  {
    return completion;
  }

  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (completion));

  if (ecma_is_value_object (new_value))
  {
    ecma_value_t compared_num_val = ecma_op_to_number (new_value, &new_len_num);

    if (ECMA_IS_VALUE_ERROR (compared_num_val))
    {
      return compared_num_val;
    }
  }

  uint32_t new_len_uint32 = ecma_number_to_uint32 (new_len_num);

  if (((ecma_number_t) new_len_uint32) != new_len_num)
  {
    return ecma_raise_range_error (ECMA_ERR_INVALID_ARRAY_LENGTH);
  }

  /* Only the writable and data properties can be modified. */
  if (flags
      & (JERRY_PROP_IS_CONFIGURABLE | JERRY_PROP_IS_ENUMERABLE | JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED))
  {
    return ecma_raise_property_redefinition (ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH), flags);
  }

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

  uint32_t old_len_uint32 = ext_object_p->u.array.length;

  if (new_len_num == old_len_uint32)
  {
    /* Only the writable flag must be updated. */
    if (flags & JERRY_PROP_IS_WRITABLE_DEFINED)
    {
      if (!(flags & JERRY_PROP_IS_WRITABLE))
      {
        if (ecma_op_array_is_fast_array (ext_object_p))
        {
          ecma_fast_array_convert_to_normal (object_p);
        }

        ext_object_p->u.array.length_prop_and_hole_count &= (uint32_t) ~ECMA_PROPERTY_FLAG_WRITABLE;
      }
      else if (!ecma_is_property_writable ((ecma_property_t) ext_object_p->u.array.length_prop_and_hole_count))
      {
        return ecma_raise_property_redefinition (ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH), flags);
      }
    }
    return ECMA_VALUE_TRUE;
  }
  else if (!ecma_is_property_writable ((ecma_property_t) ext_object_p->u.array.length_prop_and_hole_count))
  {
    return ecma_raise_property_redefinition (ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH), flags);
  }

  uint32_t current_len_uint32 = new_len_uint32;

  if (new_len_uint32 < old_len_uint32)
  {
    current_len_uint32 = ecma_delete_array_properties (object_p, new_len_uint32, old_len_uint32);
  }
  else if (ecma_op_object_is_fast_array (object_p))
  {
    ecma_fast_array_set_length (object_p, new_len_uint32);
  }

  ext_object_p->u.array.length = current_len_uint32;

  if ((flags & JERRY_PROP_IS_WRITABLE_DEFINED) && !(flags & JERRY_PROP_IS_WRITABLE))
  {
    if (ecma_op_array_is_fast_array (ext_object_p))
    {
      ecma_fast_array_convert_to_normal (object_p);
    }

    ext_object_p->u.array.length_prop_and_hole_count &= (uint32_t) ~ECMA_PROPERTY_FLAG_WRITABLE;
  }

  if (current_len_uint32 == new_len_uint32)
  {
    return ECMA_VALUE_TRUE;
  }

  return ecma_raise_property_redefinition (ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH), flags);
} /* ecma_op_array_object_set_length */

/**
 * Property descriptor bitset for fast array data properties.
 * If the property descriptor fields contains all the flags below
 * attempt to stay fast access array during [[DefineOwnProperty]] operation.
 */
#define ECMA_FAST_ARRAY_DATA_PROP_FLAGS                                                               \
  (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_ENUMERABLE_DEFINED | JERRY_PROP_IS_ENUMERABLE          \
   | JERRY_PROP_IS_CONFIGURABLE_DEFINED | JERRY_PROP_IS_CONFIGURABLE | JERRY_PROP_IS_WRITABLE_DEFINED \
   | JERRY_PROP_IS_WRITABLE)

/**
 * [[DefineOwnProperty]] ecma array object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *          ECMA-262 v5, 15.4.5.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_array_object_define_own_property (ecma_object_t *object_p, /**< the array object */
                                          ecma_string_t *property_name_p, /**< property name */
                                          const ecma_property_descriptor_t *property_desc_p) /**< property descriptor */
{
  if (ecma_string_is_length (property_name_p))
  {
    JERRY_ASSERT ((property_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE_DEFINED)
                  || !(property_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE));
    JERRY_ASSERT ((property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE_DEFINED)
                  || !(property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE));
    JERRY_ASSERT ((property_desc_p->flags & JERRY_PROP_IS_WRITABLE_DEFINED)
                  || !(property_desc_p->flags & JERRY_PROP_IS_WRITABLE));

    if (property_desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
    {
      return ecma_op_array_object_set_length (object_p, property_desc_p->value, property_desc_p->flags);
    }

    ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
    ecma_value_t length_value = ecma_make_uint32_value (ext_object_p->u.array.length);

    ecma_value_t result = ecma_op_array_object_set_length (object_p, length_value, property_desc_p->flags);

    ecma_fast_free_value (length_value);
    return result;
  }

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

  if (ecma_op_object_is_fast_array (object_p))
  {
    if ((property_desc_p->flags & ECMA_FAST_ARRAY_DATA_PROP_FLAGS) == ECMA_FAST_ARRAY_DATA_PROP_FLAGS)
    {
      uint32_t index = ecma_string_get_array_index (property_name_p);

      if (JERRY_UNLIKELY (index == ECMA_STRING_NOT_ARRAY_INDEX))
      {
        ecma_fast_array_convert_to_normal (object_p);
      }
      else if (ecma_fast_array_set_property (object_p, index, property_desc_p->value))
      {
        return ECMA_VALUE_TRUE;
      }

      JERRY_ASSERT (!ecma_op_array_is_fast_array (ext_object_p));
    }
    else
    {
      ecma_fast_array_convert_to_normal (object_p);
    }
  }

  JERRY_ASSERT (!ecma_op_object_is_fast_array (object_p));
  uint32_t index = ecma_string_get_array_index (property_name_p);

  if (index == ECMA_STRING_NOT_ARRAY_INDEX)
  {
    return ecma_op_general_object_define_own_property (object_p, property_name_p, property_desc_p);
  }

  bool update_length = (index >= ext_object_p->u.array.length);

  if (update_length && !ecma_is_property_writable ((ecma_property_t) ext_object_p->u.array.length_prop_and_hole_count))
  {
    return ecma_raise_property_redefinition (property_name_p, property_desc_p->flags);
  }

  ecma_property_descriptor_t prop_desc;

  prop_desc = *property_desc_p;
  prop_desc.flags &= (uint16_t) ~JERRY_PROP_SHOULD_THROW;

  ecma_value_t completion = ecma_op_general_object_define_own_property (object_p, property_name_p, &prop_desc);
  JERRY_ASSERT (ecma_is_value_boolean (completion));

  if (ecma_is_value_false (completion))
  {
    return ecma_raise_property_redefinition (property_name_p, property_desc_p->flags);
  }

  if (update_length)
  {
    ext_object_p->u.array.length = index + 1;
  }

  return ECMA_VALUE_TRUE;
} /* ecma_op_array_object_define_own_property */

/**
 * Get the length of the an array object
 *
 * @return the array length
 */
uint32_t 
ecma_array_get_length (ecma_object_t *array_p) /**< array object */
{
  JERRY_ASSERT (ecma_get_object_base_type (array_p) == ECMA_OBJECT_BASE_TYPE_ARRAY);

  return ((ecma_extended_object_t *) array_p)->u.array.length;
} /* ecma_array_get_length */

/**
 * The Array.prototype and %TypedArray%.prototype objects' 'toString' routine.
 *
 * See also:
 *          ECMA-262 v5, 15.4.4.2
 *          ECMA-262 v6, 22.1.3.7
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_array_object_to_string (ecma_value_t this_arg) /**< this argument */
{
  JERRY_ASSERT (ecma_is_value_object (this_arg));

  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

  ecma_object_t *obj_p = ecma_get_object_from_value (this_arg);

  /* 2. */
  ecma_value_t join_value = ecma_op_object_get_by_magic_id (obj_p, LIT_MAGIC_STRING_JOIN);
  if (ECMA_IS_VALUE_ERROR (join_value))
  {
    return join_value;
  }

  if (!ecma_op_is_callable (join_value))
  {
    /* 3. */
    ret_value = ecma_builtin_helper_object_to_string (this_arg);
  }
  else
  {
    /* 4. */
    ecma_object_t *join_func_obj_p = ecma_get_object_from_value (join_value);

    ret_value = ecma_op_function_call (join_func_obj_p, this_arg, NULL, 0);
  }

  ecma_free_value (join_value);

  return ret_value;
} /* ecma_array_object_to_string */

/**
 * @}
 * @}
 */
