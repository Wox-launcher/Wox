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

#include "ecma-arraybuffer-object.h"
#include "ecma-dataview-object.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"

#if JERRY_BUILTIN_DATAVIEW

#if !JERRY_BUILTIN_TYPEDARRAY
#error "DataView builtin requires ES2015 TypedArray builtin"
#endif /* !JERRY_BUILTIN_TYPEDARRAY */

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

/**
 * This object has a custom dispatch function.
 */
#define BUILTIN_CUSTOM_DISPATCH

/**
 * List of built-in routine identifiers.
 */
enum
{
  ECMA_DATAVIEW_PROTOTYPE_ROUTINE_START = 0,
  ECMA_DATAVIEW_PROTOTYPE_BUFFER_GETTER,
  ECMA_DATAVIEW_PROTOTYPE_BYTE_LENGTH_GETTER,
  ECMA_DATAVIEW_PROTOTYPE_BYTE_OFFSET_GETTER,
  ECMA_DATAVIEW_PROTOTYPE_GET_INT8,
  ECMA_DATAVIEW_PROTOTYPE_GET_UINT8,
  ECMA_DATAVIEW_PROTOTYPE_GET_UINT8_CLAMPED, /* unused value */
  ECMA_DATAVIEW_PROTOTYPE_GET_INT16,
  ECMA_DATAVIEW_PROTOTYPE_GET_UINT16,
  ECMA_DATAVIEW_PROTOTYPE_GET_INT32,
  ECMA_DATAVIEW_PROTOTYPE_GET_UINT32,
  ECMA_DATAVIEW_PROTOTYPE_GET_FLOAT32,
#if JERRY_BUILTIN_BIGINT
  ECMA_DATAVIEW_PROTOTYPE_GET_BIGINT64,
  ECMA_DATAVIEW_PROTOTYPE_GET_BIGUINT64,
#endif /* JERRY_BUILTIN_BIGINT */
  ECMA_DATAVIEW_PROTOTYPE_SET_INT8,
  ECMA_DATAVIEW_PROTOTYPE_SET_UINT8,
  ECMA_DATAVIEW_PROTOTYPE_SET_UINT8_CLAMPED, /* unused value */
  ECMA_DATAVIEW_PROTOTYPE_SET_INT16,
  ECMA_DATAVIEW_PROTOTYPE_SET_UINT16,
  ECMA_DATAVIEW_PROTOTYPE_SET_INT32,
  ECMA_DATAVIEW_PROTOTYPE_SET_UINT32,
  ECMA_DATAVIEW_PROTOTYPE_SET_FLOAT32,
#if JERRY_BUILTIN_BIGINT
  ECMA_DATAVIEW_PROTOTYPE_SET_BIGINT64,
  ECMA_DATAVIEW_PROTOTYPE_SET_BIGUINT64,
#endif /* JERRY_BUILTIN_BIGINT */
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-dataview-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  dataview_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup dataviewprototype ECMA DataView.prototype object built-in
 * @{
 */

/**
 * The DataView.prototype object's {buffer, byteOffset, byteLength} getters
 *
 * See also:
 *          ECMA-262 v6, 24.2.4.1
 *          ECMA-262 v6, 24.2.4.2
 *          ECMA-262 v6, 24.2.4.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_dataview_prototype_object_getters (ecma_value_t this_arg, /**< this argument */
                                                uint16_t builtin_routine_id) /**< built-in wide routine identifier */
{
  ecma_dataview_object_t *obj_p = ecma_op_dataview_get_object (this_arg);

  if (JERRY_UNLIKELY (obj_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  switch (builtin_routine_id)
  {
    case ECMA_DATAVIEW_PROTOTYPE_BUFFER_GETTER:
    {
      ecma_object_t *buffer_p = obj_p->buffer_p;
      ecma_ref_object (buffer_p);

      return ecma_make_object_value (buffer_p);
    }
    case ECMA_DATAVIEW_PROTOTYPE_BYTE_LENGTH_GETTER:
    {
      if (ecma_arraybuffer_is_detached (obj_p->buffer_p))
      {
        return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
      }
      return ecma_make_uint32_value (obj_p->header.u.cls.u3.length);
    }
    default:
    {
      JERRY_ASSERT (builtin_routine_id == ECMA_DATAVIEW_PROTOTYPE_BYTE_OFFSET_GETTER);

      if (ecma_arraybuffer_is_detached (obj_p->buffer_p))
      {
        return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
      }
      return ecma_make_uint32_value (obj_p->byte_offset);
    }
  }
} /* ecma_builtin_dataview_prototype_object_getters */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_dataview_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                                  ecma_value_t this_arg, /**< 'this' argument value */
                                                  const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                          *   passed to routine */
                                                  uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (arguments_number);
  switch (builtin_routine_id)
  {
    case ECMA_DATAVIEW_PROTOTYPE_BUFFER_GETTER:
    case ECMA_DATAVIEW_PROTOTYPE_BYTE_LENGTH_GETTER:
    case ECMA_DATAVIEW_PROTOTYPE_BYTE_OFFSET_GETTER:
    {
      return ecma_builtin_dataview_prototype_object_getters (this_arg, builtin_routine_id);
    }
    case ECMA_DATAVIEW_PROTOTYPE_GET_FLOAT32:
    case ECMA_DATAVIEW_PROTOTYPE_GET_INT16:
    case ECMA_DATAVIEW_PROTOTYPE_GET_INT32:
    case ECMA_DATAVIEW_PROTOTYPE_GET_UINT16:
    case ECMA_DATAVIEW_PROTOTYPE_GET_UINT32:
#if JERRY_BUILTIN_BIGINT
    case ECMA_DATAVIEW_PROTOTYPE_GET_BIGINT64:
    case ECMA_DATAVIEW_PROTOTYPE_GET_BIGUINT64:
#endif /* JERRY_BUILTIN_BIGINT */
    {
      ecma_typedarray_type_t id = (ecma_typedarray_type_t) (builtin_routine_id - ECMA_DATAVIEW_PROTOTYPE_GET_INT8);

      return ecma_op_dataview_get_set_view_value (this_arg,
                                                  arguments_list_p[0],
                                                  arguments_list_p[1],
                                                  ECMA_VALUE_EMPTY,
                                                  id);
    }
    case ECMA_DATAVIEW_PROTOTYPE_SET_FLOAT32:
    case ECMA_DATAVIEW_PROTOTYPE_SET_INT16:
    case ECMA_DATAVIEW_PROTOTYPE_SET_INT32:
    case ECMA_DATAVIEW_PROTOTYPE_SET_UINT16:
    case ECMA_DATAVIEW_PROTOTYPE_SET_UINT32:
#if JERRY_BUILTIN_BIGINT
    case ECMA_DATAVIEW_PROTOTYPE_SET_BIGINT64:
    case ECMA_DATAVIEW_PROTOTYPE_SET_BIGUINT64:
#endif /* JERRY_BUILTIN_BIGINT */
    {
      ecma_typedarray_type_t id = (ecma_typedarray_type_t) (builtin_routine_id - ECMA_DATAVIEW_PROTOTYPE_SET_INT8);

      return ecma_op_dataview_get_set_view_value (this_arg,
                                                  arguments_list_p[0],
                                                  arguments_list_p[2],
                                                  arguments_list_p[1],
                                                  id);
    }
    case ECMA_DATAVIEW_PROTOTYPE_GET_INT8:
    case ECMA_DATAVIEW_PROTOTYPE_GET_UINT8:
    {
      ecma_typedarray_type_t id = (ecma_typedarray_type_t) (builtin_routine_id - ECMA_DATAVIEW_PROTOTYPE_GET_INT8);

      return ecma_op_dataview_get_set_view_value (this_arg,
                                                  arguments_list_p[0],
                                                  ECMA_VALUE_FALSE,
                                                  ECMA_VALUE_EMPTY,
                                                  id);
    }
    default:
    {
      JERRY_ASSERT (builtin_routine_id == ECMA_DATAVIEW_PROTOTYPE_SET_INT8
                    || builtin_routine_id == ECMA_DATAVIEW_PROTOTYPE_SET_UINT8);
      ecma_typedarray_type_t id = (ecma_typedarray_type_t) (builtin_routine_id - ECMA_DATAVIEW_PROTOTYPE_SET_INT8);

      return ecma_op_dataview_get_set_view_value (this_arg,
                                                  arguments_list_p[0],
                                                  ECMA_VALUE_FALSE,
                                                  arguments_list_p[1],
                                                  id);
    }
  }
} /* ecma_builtin_dataview_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_DATAVIEW */
