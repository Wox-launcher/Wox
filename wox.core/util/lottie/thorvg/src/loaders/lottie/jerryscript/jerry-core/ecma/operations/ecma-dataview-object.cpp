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

#include "ecma-dataview-object.h"

#include "ecma-arraybuffer-object.h"
#include "ecma-bigint.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-shared-arraybuffer-object.h"
#include "ecma-typedarray-object.h"

#include "jcontext.h"

#if JERRY_BUILTIN_DATAVIEW

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmadataviewobject ECMA builtin DataView helper functions
 * @{
 */

/**
 * Handle calling [[Construct]] of built-in DataView like objects
 *
 * See also:
 *          ECMA-262 v11, 24.3.2.1
 *
 * @return created DataView object as an ecma-value - if success
 *         raised error - otherwise
 */
ecma_value_t
ecma_op_dataview_create (const ecma_value_t *arguments_list_p, /**< arguments list */
                         uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);
  JERRY_ASSERT (JERRY_CONTEXT (current_new_target_p));

  ecma_value_t buffer = arguments_list_len > 0 ? arguments_list_p[0] : ECMA_VALUE_UNDEFINED;

  /* 2. */
  if (!ecma_is_value_object (buffer))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_BUFFER_NOT_OBJECT);
  }

  ecma_object_t *buffer_p = ecma_get_object_from_value (buffer);

  if (!(ecma_object_class_is (buffer_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER)
        || ecma_object_is_shared_arraybuffer (buffer_p)))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_BUFFER_NOT_ARRAY_OR_SHARED_BUFFER);
  }

  /* 3. */
  ecma_number_t offset = 0;

  if (arguments_list_len > 1)
  {
    ecma_value_t offset_value = ecma_op_to_index (arguments_list_p[1], &offset);
    if (ECMA_IS_VALUE_ERROR (offset_value))
    {
      return offset_value;
    }
  }

  /* 4. */
  if (ecma_arraybuffer_is_detached (buffer_p))
  {
    return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
  }

  /* 5. */
  ecma_number_t buffer_byte_length = ecma_arraybuffer_get_length (buffer_p);

  /* 6. */
  if (offset > buffer_byte_length)
  {
    return ecma_raise_range_error (ECMA_ERR_START_OFFSET_IS_OUTSIDE_THE_BOUNDS_OF_THE_BUFFER);
  }

  /* 7. */
  uint32_t view_byte_length;
  if (arguments_list_len > 2 && !ecma_is_value_undefined (arguments_list_p[2]))
  {
    /* 8.a */
    ecma_number_t byte_length_to_index;
    ecma_value_t byte_length_value = ecma_op_to_index (arguments_list_p[2], &byte_length_to_index);

    if (ECMA_IS_VALUE_ERROR (byte_length_value))
    {
      return byte_length_value;
    }

    /* 8.b */
    if (offset + byte_length_to_index > buffer_byte_length)
    {
      return ecma_raise_range_error (ECMA_ERR_START_OFFSET_IS_OUTSIDE_THE_BOUNDS_OF_THE_BUFFER);
    }

    JERRY_ASSERT (byte_length_to_index <= UINT32_MAX);
    view_byte_length = (uint32_t) byte_length_to_index;
  }
  else
  {
    /* 7.a */
    view_byte_length = (uint32_t) (buffer_byte_length - offset);
  }

  /* 9. */
  ecma_object_t *prototype_obj_p =
    ecma_op_get_prototype_from_constructor (JERRY_CONTEXT (current_new_target_p), ECMA_BUILTIN_ID_DATAVIEW_PROTOTYPE);
  if (JERRY_UNLIKELY (prototype_obj_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  /* 10. */
  if (ecma_arraybuffer_is_detached (buffer_p))
  {
    ecma_deref_object (prototype_obj_p);
    return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
  }

  /* 9. */
  /* It must happen after 10., because uninitialized object can't be destroyed properly. */
  ecma_object_t *object_p =
    ecma_create_object (prototype_obj_p, sizeof (ecma_dataview_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_deref_object (prototype_obj_p);

  /* 11 - 14. */
  ecma_dataview_object_t *dataview_obj_p = (ecma_dataview_object_t *) object_p;
  dataview_obj_p->header.u.cls.type = ECMA_OBJECT_CLASS_DATAVIEW;
  dataview_obj_p->header.u.cls.u3.length = view_byte_length;
  dataview_obj_p->buffer_p = buffer_p;
  dataview_obj_p->byte_offset = (uint32_t) offset;

  return ecma_make_object_value (object_p);
} /* ecma_op_dataview_create */

/**
 * Get the DataView object pointer
 *
 * Note:
 *   If the function returns with NULL, the error object has
 *   already set, and the caller must return with ECMA_VALUE_ERROR
 *
 * @return pointer to the dataView if this_arg is a valid dataView object
 *         NULL otherwise
 */
ecma_dataview_object_t *
ecma_op_dataview_get_object (ecma_value_t this_arg) /**< this argument */
{
  if (ecma_is_value_object (this_arg))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (this_arg);

    if (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_DATAVIEW))
    {
      return (ecma_dataview_object_t *) object_p;
    }
  }

  ecma_raise_type_error (ECMA_ERR_EXPECTED_A_DATAVIEW_OBJECT);
  return NULL;
} /* ecma_op_dataview_get_object */

/**
 * Helper union to specify the system's endianness
 */
typedef union
{
  uint32_t number; /**< for write numeric data */
  char data[sizeof (uint32_t)]; /**< for read numeric data */
} ecma_dataview_endianness_check_t;

/**
 * Helper function to check the current system endianness
 *
 * @return true - if the current system has little endian byteorder
 *         false - otherwise
 */
static bool
ecma_dataview_check_little_endian (void)
{
  ecma_dataview_endianness_check_t checker;
  checker.number = 0x01;

  return checker.data[0] == 0x01;
} /* ecma_dataview_check_little_endian */

/**
 * Helper function for swap bytes if the system's endianness
 * does not match with the requested endianness.
 */
static void
ecma_dataview_swap_order (bool system_is_little_endian, /**< true - if the system has little endian byteorder
                                                         *   false - otherwise */
                          bool is_little_endian, /**< true - if little endian byteorder is requested
                                                  *   false - otherwise */
                          uint32_t element_size, /**< element size byte according to the Table 49.*/
                          lit_utf8_byte_t *block_p) /**< data block */
{
  if (system_is_little_endian ^ is_little_endian)
  {
    for (uint32_t i = 0; i < element_size / 2; i++)
    {
      lit_utf8_byte_t tmp = block_p[i];
      block_p[i] = block_p[element_size - i - 1];
      block_p[element_size - i - 1] = tmp;
    }
  }
} /* ecma_dataview_swap_order */

/**
 * GetViewValue and SetViewValue abstract operation
 *
 * See also:
 *          ECMA-262 v11, 24.3.1.1
 *          ECMA-262 v11, 24.3.1.2
 *
 * @return ecma value
 */
ecma_value_t
ecma_op_dataview_get_set_view_value (ecma_value_t view, /**< the operation's 'view' argument */
                                     ecma_value_t request_index, /**< the operation's 'requestIndex' argument */
                                     ecma_value_t is_little_endian_value, /**< the operation's
                                                                           *   'isLittleEndian' argument */
                                     ecma_value_t value_to_set, /**< the operation's 'value' argument */
                                     ecma_typedarray_type_t id) /**< the operation's 'type' argument */
{
  /* 1 - 2. */
  ecma_dataview_object_t *view_p = ecma_op_dataview_get_object (view);

  if (JERRY_UNLIKELY (view_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_object_t *buffer_p = view_p->buffer_p;
  JERRY_ASSERT (ecma_object_class_is (buffer_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER)
                || ecma_object_is_shared_arraybuffer (buffer_p));

  /* 3. */
  ecma_number_t get_index;
  ecma_value_t number_index_value = ecma_op_to_index (request_index, &get_index);

  if (ECMA_IS_VALUE_ERROR (number_index_value))
  {
    return number_index_value;
  }

  /* SetViewValue 4 - 5. */
  if (!ecma_is_value_empty (value_to_set))
  {
#if JERRY_BUILTIN_BIGINT
    if (ECMA_TYPEDARRAY_IS_BIGINT_TYPE (id))
    {
      value_to_set = ecma_bigint_to_bigint (value_to_set, true);

      if (ECMA_IS_VALUE_ERROR (value_to_set))
      {
        return value_to_set;
      }
    }
    else
#endif /* JERRY_BUILTIN_BIGINT */
    {
      ecma_number_t value_to_set_number;
      ecma_value_t value = ecma_op_to_number (value_to_set, &value_to_set_number);

      if (ECMA_IS_VALUE_ERROR (value))
      {
        return value;
      }

      value_to_set = ecma_make_number_value (value_to_set_number);
    }
  }

  /* GetViewValue 4., SetViewValue 6. */
  bool is_little_endian = ecma_op_to_boolean (is_little_endian_value);

  if (ECMA_ARRAYBUFFER_LAZY_ALLOC (buffer_p))
  {
    ecma_free_value (value_to_set);
    return ECMA_VALUE_ERROR;
  }

  if (ecma_arraybuffer_is_detached (buffer_p))
  {
    ecma_free_value (value_to_set);
    return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
  }

  /* GetViewValue 7., SetViewValue 9. */
  uint32_t view_offset = view_p->byte_offset;

  /* GetViewValue 8., SetViewValue 10. */
  uint32_t view_size = view_p->header.u.cls.u3.length;

  /* GetViewValue 9., SetViewValue 11. */
  uint8_t element_size = (uint8_t) (1 << (ecma_typedarray_helper_get_shift_size (id)));

  /* GetViewValue 10., SetViewValue 12. */
  if (get_index + element_size > (ecma_number_t) view_size)
  {
    ecma_free_value (value_to_set);
    return ecma_raise_range_error (ECMA_ERR_START_OFFSET_IS_OUTSIDE_THE_BOUNDS_OF_THE_BUFFER);
  }

  if (ECMA_ARRAYBUFFER_LAZY_ALLOC (buffer_p))
  {
    ecma_free_value (value_to_set);
    return ECMA_VALUE_ERROR;
  }

  if (ecma_arraybuffer_is_detached (buffer_p))
  {
    ecma_free_value (value_to_set);
    return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
  }

  /* GetViewValue 11., SetViewValue 13. */
  bool system_is_little_endian = ecma_dataview_check_little_endian ();

  ecma_typedarray_info_t info;
  info.id = id;
  info.length = view_size;
  info.shift = ecma_typedarray_helper_get_shift_size (id);
  info.element_size = element_size;
  info.offset = view_p->byte_offset;
  info.array_buffer_p = buffer_p;

  /* GetViewValue 12. */
  uint8_t *block_p = ecma_arraybuffer_get_buffer (buffer_p) + (uint32_t) get_index + view_offset;

  if (ecma_is_value_empty (value_to_set))
  {
    JERRY_VLA (lit_utf8_byte_t, swap_block_p, element_size);
    memcpy (swap_block_p, block_p, element_size * sizeof (lit_utf8_byte_t));
    ecma_dataview_swap_order (system_is_little_endian, is_little_endian, element_size, swap_block_p);

    ecma_typedarray_getter_fn_t typedarray_getter_cb = ecma_get_typedarray_getter_fn (info.id);
    return typedarray_getter_cb (swap_block_p);
  }

  if (!ecma_number_is_nan (get_index) && get_index <= 0)
  {
    get_index = 0;
  }

  /* SetViewValue 14. */
  ecma_typedarray_setter_fn_t typedarray_setter_cb = ecma_get_typedarray_setter_fn (info.id);
  ecma_value_t set_element = typedarray_setter_cb (block_p, value_to_set);

  ecma_free_value (value_to_set);

  if (ECMA_IS_VALUE_ERROR (set_element))
  {
    return set_element;
  }

  ecma_dataview_swap_order (system_is_little_endian, is_little_endian, element_size, block_p);

  return ECMA_VALUE_UNDEFINED;
} /* ecma_op_dataview_get_set_view_value */

/**
 * Check if the value is dataview
 *
 * @return true - if value is a DataView object
 *         false - otherwise
 */
bool
ecma_is_dataview (ecma_value_t value) /**< the target need to be checked */
{
  if (!ecma_is_value_object (value))
  {
    return false;
  }

  return ecma_object_class_is (ecma_get_object_from_value (value), ECMA_OBJECT_CLASS_DATAVIEW);
} /* ecma_is_dataview */

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_DATAVIEW */
