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

#include <math.h>

#include "ecma-alloc.h"
#include "ecma-bigint.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers-number.h"
#include "ecma-helpers.h"
#include "ecma-number-object.h"
#include "ecma-objects.h"

#include "jrt.h"

#if JERRY_BUILTIN_NUMBER

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
  ECMA_NUMBER_OBJECT_ROUTINE_START = 0,
  ECMA_NUMBER_OBJECT_ROUTINE_IS_FINITE,
  ECMA_NUMBER_OBJECT_ROUTINE_IS_NAN,
  ECMA_NUMBER_OBJECT_ROUTINE_IS_INTEGER,
  ECMA_NUMBER_OBJECT_ROUTINE_IS_SAFE_INTEGER
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-number.inc.h"
#define BUILTIN_UNDERSCORED_ID  number
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup number ECMA Number object built-in
 * @{
 */

/**
 * Handle calling [[Call]] of built-in Number object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_number_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                   uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

  if (arguments_list_len == 0)
  {
    ret_value = ecma_make_integer_value (0);
  }
  else
  {
    ecma_number_t num;
    ret_value = ecma_op_to_numeric (arguments_list_p[0], &num, ECMA_TO_NUMERIC_ALLOW_BIGINT);

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      return ret_value;
    }

#if JERRY_BUILTIN_BIGINT
    if (ecma_is_value_bigint (ret_value))
    {
      ecma_value_t bigint = ret_value;
      ret_value = ecma_bigint_to_number (bigint);
      ecma_free_value (bigint);
    }
    else
#endif /* JERRY_BUILTIN_BIGINT */
    {
      ret_value = ecma_make_number_value (num);
    }
  }

  return ret_value;
} /* ecma_builtin_number_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Number object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_number_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                        uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  if (arguments_list_len == 0)
  {
    return ecma_op_create_number_object (ecma_make_integer_value (0));
  }

#if JERRY_BUILTIN_BIGINT
  ecma_number_t num;
  ecma_value_t value = ecma_op_to_numeric (arguments_list_p[0], &num, ECMA_TO_NUMERIC_ALLOW_BIGINT);

  if (ECMA_IS_VALUE_ERROR (value))
  {
    return value;
  }

  if (ecma_is_value_bigint (value))
  {
    ecma_value_t bigint = value;
    value = ecma_bigint_to_number (bigint);
    ecma_free_value (bigint);
  }
  else
  {
    value = ecma_make_number_value (num);
  }

  ecma_value_t result = ecma_op_create_number_object (value);
  ecma_free_value (value);
  return result;
#else /* !JERRY_BUILTIN_BIGINT */
  return ecma_op_create_number_object (arguments_list_p[0]);
#endif /* JERRY_BUILTIN_BIGINT */
} /* ecma_builtin_number_dispatch_construct */

/**
 * The Number object 'isInteger' and 'isSafeInteger' routine
 *
 * See also:
 *          ECMA-262 v6, 20.1.2.3
 *          ECMA-262 v6, 20.1.2.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_number_object_is_integer_helper (ecma_value_t arg, /**< routine's argument */
                                              ecma_number_t num, /**< this number */
                                              bool is_safe) /**< is the number safe */
{
  if (ecma_number_is_nan (num) || ecma_number_is_infinity (num))
  {
    return ECMA_VALUE_FALSE;
  }

  ecma_number_t int_num;

  if (is_safe)
  {
    int_num = ecma_number_trunc (num);

    if (fabs (int_num) > ECMA_NUMBER_MAX_SAFE_INTEGER)
    {
      return ECMA_VALUE_FALSE;
    }
  }
  else
  {
    ecma_op_to_integer (arg, &int_num);
  }

  return (int_num == num) ? ECMA_VALUE_TRUE : ECMA_VALUE_FALSE;
} /* ecma_builtin_number_object_is_integer_helper */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_number_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                      ecma_value_t this_arg, /**< 'this' argument value */
                                      const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                              *   passed to routine */
                                      uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (this_arg, arguments_number);

  if (!ecma_is_value_number (arguments_list_p[0]))
  {
    return ECMA_VALUE_FALSE;
  }

  ecma_number_t num = ecma_get_number_from_value (arguments_list_p[0]);

  switch (builtin_routine_id)
  {
    case ECMA_NUMBER_OBJECT_ROUTINE_IS_FINITE:
    {
      return ecma_make_boolean_value (!(ecma_number_is_nan (num) || ecma_number_is_infinity (num)));
    }
    case ECMA_NUMBER_OBJECT_ROUTINE_IS_NAN:
    {
      return ecma_make_boolean_value (ecma_number_is_nan (num));
    }
    case ECMA_NUMBER_OBJECT_ROUTINE_IS_INTEGER:
    case ECMA_NUMBER_OBJECT_ROUTINE_IS_SAFE_INTEGER:
    {
      bool is_safe = (builtin_routine_id == ECMA_NUMBER_OBJECT_ROUTINE_IS_SAFE_INTEGER);
      return ecma_builtin_number_object_is_integer_helper (arguments_list_p[0], num, is_safe);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_number_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_NUMBER */
