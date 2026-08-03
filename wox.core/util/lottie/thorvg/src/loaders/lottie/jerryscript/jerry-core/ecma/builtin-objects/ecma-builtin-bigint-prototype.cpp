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

#include "ecma-bigint.h"
#include "ecma-exceptions.h"

#if JERRY_BUILTIN_BIGINT

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
  ECMA_BIGINT_PROTOTYPE_ROUTINE_START = 0,
  ECMA_BIGINT_PROTOTYPE_VALUE_OF,
  ECMA_BIGINT_PROTOTYPE_TO_STRING,
  ECMA_BIGINT_PROTOTYPE_TO_LOCALE_STRING,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-bigint-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  bigint_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup bigint ECMA BigInt object built-in
 * @{
 */

/**
 * The BigInt.prototype object's 'valueOf' routine
 *
 * See also:
 *          ECMA-262 v11, 20.2.3.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_bigint_prototype_object_value_of (ecma_value_t this_arg) /**< this argument */
{
  if (ecma_is_value_bigint (this_arg))
  {
    return ecma_copy_value (this_arg);
  }

  if (ecma_is_value_object (this_arg))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (this_arg);

    if (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_BIGINT))
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      JERRY_ASSERT (ecma_is_value_bigint (ext_object_p->u.cls.u3.value));

      return ecma_copy_value (ext_object_p->u.cls.u3.value);
    }
  }

  return ecma_raise_type_error (ECMA_ERR_BIGINT_VALUE_EXPECTED);
} /* ecma_builtin_bigint_prototype_object_value_of */

/**
 * The BigInt.prototype object's 'toString' routine
 *
 * See also:
 *          ECMA-262 v11, 20.2.3.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_bigint_prototype_object_to_string (ecma_value_t this_arg, /**< this argument */
                                                const ecma_value_t *arguments_list_p, /**< arguments list */
                                                uint32_t arguments_list_len) /**< number of arguments */
{
  uint32_t radix = 10;

  if (arguments_list_len > 0 && !ecma_is_value_undefined (arguments_list_p[0]))
  {
    ecma_number_t arg_num;

    if (ECMA_IS_VALUE_ERROR (ecma_op_to_integer (arguments_list_p[0], &arg_num)))
    {
      return ECMA_VALUE_ERROR;
    }

    if (arg_num < 2 || arg_num > 36)
    {
      return ecma_raise_range_error (ECMA_ERR_RADIX_IS_OUT_OF_RANGE);
    }

    radix = (uint32_t) arg_num;
  }

  ecma_string_t *string_p = ecma_bigint_to_string (this_arg, radix);

  if (string_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  return ecma_make_string_value (string_p);
} /* ecma_builtin_bigint_prototype_object_to_string */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_bigint_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                             *   identifier */
                                                ecma_value_t this_arg, /**< 'this' argument value */
                                                const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                        *   passed to routine */
                                                uint32_t arguments_number) /**< length of arguments' list */
{
  ecma_value_t this_value = ecma_builtin_bigint_prototype_object_value_of (this_arg);
  ecma_value_t ret_val;

  if (ECMA_IS_VALUE_ERROR (this_value))
  {
    return this_value;
  }

  switch (builtin_routine_id)
  {
    case ECMA_BIGINT_PROTOTYPE_VALUE_OF:
    {
      ret_val = this_value;
      break;
    }
    case ECMA_BIGINT_PROTOTYPE_TO_STRING:
    {
      ret_val = ecma_builtin_bigint_prototype_object_to_string (this_value, arguments_list_p, arguments_number);
      ecma_free_value (this_value);
      break;
    }
    case ECMA_BIGINT_PROTOTYPE_TO_LOCALE_STRING:
    {
      ret_val = ecma_builtin_bigint_prototype_object_to_string (this_value, 0, 0);
      ecma_free_value (this_value);
      break;
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
  return ret_val;
} /* ecma_builtin_bigint_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_BIGINT */
