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
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-typedarray-object.h"

#include "jrt.h"

#if JERRY_BUILTIN_TYPEDARRAY

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-typedarray.inc.h"
#define BUILTIN_UNDERSCORED_ID  typedarray
#include "ecma-builtin-internal-routines-template.inc.h"
#include "ecma-builtin-typedarray-helpers.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup typedarray ECMA %TypedArray% object built-in
 * @{
 */

/**
 * The %TypedArray%.from routine
 *
 * See also:
 *         ES2015 22.2.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_typedarray_from (ecma_value_t this_arg, /**< 'this' argument */
                              const ecma_value_t *arguments_list_p, /**< arguments list */
                              uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  if (!ecma_is_constructor (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_CONSTRUCTOR);
  }

  ecma_value_t source;
  ecma_value_t map_fn = ECMA_VALUE_UNDEFINED;
  ecma_value_t this_in_fn = ECMA_VALUE_UNDEFINED;

  if (arguments_list_len == 0)
  {
    return ecma_raise_type_error (ECMA_ERR_NO_SOURCE_ARGUMENT);
  }

  source = arguments_list_p[0];

  if (arguments_list_len > 1)
  {
    map_fn = arguments_list_p[1];

    if (!ecma_op_is_callable (map_fn))
    {
      return ecma_raise_type_error (ECMA_ERR_THE_MAPFN_ARGUMENT_IS_NOT_CALLABLE);
    }

    if (arguments_list_len > 2)
    {
      this_in_fn = arguments_list_p[2];
    }
  }

  return ecma_op_typedarray_from (this_arg, source, map_fn, this_in_fn);

} /* ecma_builtin_typedarray_from */

/**
 * The %TypedArray%.of routine
 *
 * See also:
 *         ES2015 22.2.2.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_typedarray_of (ecma_value_t this_arg, /**< 'this' argument */
                            const ecma_value_t *arguments_list_p, /**< arguments list */
                            uint32_t arguments_list_len) /**< number of arguments */
{
  if (!ecma_is_constructor (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_CONSTRUCTOR);
  }

  ecma_object_t *constructor_obj_p = ecma_get_object_from_value (this_arg);
  ecma_value_t len_val = ecma_make_uint32_value (arguments_list_len);
  ecma_value_t ret_val = ecma_typedarray_create (constructor_obj_p, &len_val, 1);
  ecma_free_value (len_val);

  if (ECMA_IS_VALUE_ERROR (ret_val))
  {
    return ret_val;
  }

  uint32_t k = 0;
  ecma_object_t *ret_obj_p = ecma_get_object_from_value (ret_val);
  ecma_typedarray_info_t info = ecma_typedarray_get_info (ret_obj_p);
  ecma_typedarray_setter_fn_t setter_cb = ecma_get_typedarray_setter_fn (info.id);

  if (ECMA_ARRAYBUFFER_LAZY_ALLOC (info.array_buffer_p))
  {
    ecma_deref_object (ret_obj_p);
    return ECMA_VALUE_ERROR;
  }

  if (ecma_arraybuffer_is_detached (info.array_buffer_p))
  {
    ecma_deref_object (ret_obj_p);
    return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
  }

  lit_utf8_byte_t *buffer_p = ecma_typedarray_get_buffer (&info);

  while (k < arguments_list_len)
  {
    ecma_value_t set_element = setter_cb (buffer_p, arguments_list_p[k]);

    if (ECMA_IS_VALUE_ERROR (set_element))
    {
      ecma_deref_object (ret_obj_p);
      return set_element;
    }

    k++;
    buffer_p += info.element_size;
  }

  return ret_val;
} /* ecma_builtin_typedarray_of */

/**
 * Handle calling [[Call]] of built-in %TypedArray% object
 *
 * ES2015 22.2.1 If %TypedArray% is directly called or
 * called as part of a new expression an exception is thrown
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_typedarray_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                       uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_raise_type_error (ECMA_ERR_TYPEDARRAY_INTRINSIC_DIRECTLY_CALLED);
} /* ecma_builtin_typedarray_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in %TypedArray% object
 *
 * ES2015 22.2.1 If %TypedArray% is directly called or
 * called as part of a new expression an exception is thrown
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_typedarray_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                            uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_raise_type_error (ECMA_ERR_TYPEDARRAY_INTRINSIC_CALLED_BY_NEW_EXPRESSION);
} /* ecma_builtin_typedarray_dispatch_construct */

/**
 * 22.2.2.4 get %TypedArray% [ @@species ] accessor
 *
 * @return ecma_value
 *         returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_builtin_typedarray_species_get (ecma_value_t this_value) /**< This Value */
{
  return ecma_copy_value (this_value);
} /* ecma_builtin_typedarray_species_get */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_TYPEDARRAY */
