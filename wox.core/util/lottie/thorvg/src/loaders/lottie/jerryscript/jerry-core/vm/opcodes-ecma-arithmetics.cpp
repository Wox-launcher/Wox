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
#include "ecma-bigint.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"

#include "jrt-libc-includes.h"
#include "opcodes.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup vm_opcodes Opcodes
 * @{
 */

/**
 * Perform ECMA number arithmetic operation.
 *
 * The algorithm of the operation is following:
 *   leftNum = ToNumber (leftValue);
 *   rightNum = ToNumber (rightValue);
 *   result = leftNum ArithmeticOp rightNum;
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
do_number_arithmetic (number_arithmetic_op op, /**< number arithmetic operation */
                      ecma_value_t left_value, /**< left value */
                      ecma_value_t right_value) /**< right value */
{
  ecma_number_t left_number;
  left_value = ecma_op_to_numeric (left_value, &left_number, ECMA_TO_NUMERIC_ALLOW_BIGINT);

  if (ECMA_IS_VALUE_ERROR (left_value))
  {
    return left_value;
  }

  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

#if JERRY_BUILTIN_BIGINT
  if (JERRY_LIKELY (!ecma_is_value_bigint (left_value)))
  {
#endif /* JERRY_BUILTIN_BIGINT */

    ecma_number_t right_number;
    if (ECMA_IS_VALUE_ERROR (ecma_op_to_number (right_value, &right_number)))
    {
      return ECMA_VALUE_ERROR;
    }

    ecma_number_t result = ECMA_NUMBER_ZERO;

    switch (op)
    {
      case NUMBER_ARITHMETIC_SUBTRACTION:
      {
        result = left_number - right_number;
        break;
      }
      case NUMBER_ARITHMETIC_MULTIPLICATION:
      {
        result = left_number * right_number;
        break;
      }
      case NUMBER_ARITHMETIC_DIVISION:
      {
        result = left_number / right_number;
        break;
      }
      case NUMBER_ARITHMETIC_REMAINDER:
      {
        if (ecma_number_is_nan (left_number) || ecma_number_is_nan (right_number)
            || ecma_number_is_infinity (left_number) || ecma_number_is_zero (right_number))
        {
          result = ecma_number_make_nan ();
          break;
        }

        if (ecma_number_is_infinity (right_number)
            || (ecma_number_is_zero (left_number) && !ecma_number_is_zero (right_number)))
        {
          result = left_number;
          break;
        }

        result = ecma_number_remainder (left_number, right_number);
        break;
      }
      case NUMBER_ARITHMETIC_EXPONENTIATION:
      {
        result = ecma_number_pow (left_number, right_number);
        break;
      }
    }

    ret_value = ecma_make_number_value (result);
#if JERRY_BUILTIN_BIGINT
  }
  else
  {
    bool free_right_value;
    right_value = ecma_bigint_get_bigint (right_value, &free_right_value);

    if (ECMA_IS_VALUE_ERROR (right_value))
    {
      ecma_free_value (left_value);
      return right_value;
    }

    switch (op)
    {
      case NUMBER_ARITHMETIC_SUBTRACTION:
      {
        ret_value = ecma_bigint_add_sub (left_value, right_value, false);
        break;
      }
      case NUMBER_ARITHMETIC_MULTIPLICATION:
      {
        ret_value = ecma_bigint_mul (left_value, right_value);
        break;
      }
      case NUMBER_ARITHMETIC_DIVISION:
      {
        ret_value = ecma_bigint_div_mod (left_value, right_value, false);
        break;
      }
      case NUMBER_ARITHMETIC_REMAINDER:
      {
        ret_value = ecma_bigint_div_mod (left_value, right_value, true);
        break;
      }
      case NUMBER_ARITHMETIC_EXPONENTIATION:
      {
        ret_value = ecma_bigint_pow (left_value, right_value);
        break;
      }
    }

    ecma_free_value (left_value);
    if (free_right_value)
    {
      ecma_free_value (right_value);
    }
  }
#endif /* JERRY_BUILTIN_BIGINT */
  return ret_value;
} /* do_number_arithmetic */

/**
 * 'Addition' opcode handler.
 *
 * See also: ECMA-262 v5, 11.6.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
opfunc_addition (ecma_value_t left_value, /**< left value */
                 ecma_value_t right_value) /**< right value */
{
  bool free_left_value = false;
  bool free_right_value = false;

  if (ecma_is_value_object (left_value))
  {
    ecma_object_t *obj_p = ecma_get_object_from_value (left_value);
    left_value = ecma_op_object_default_value (obj_p, ECMA_PREFERRED_TYPE_NO);
    free_left_value = true;

    if (ECMA_IS_VALUE_ERROR (left_value))
    {
      return left_value;
    }
  }

  if (ecma_is_value_object (right_value))
  {
    ecma_object_t *obj_p = ecma_get_object_from_value (right_value);
    right_value = ecma_op_object_default_value (obj_p, ECMA_PREFERRED_TYPE_NO);
    free_right_value = true;

    if (ECMA_IS_VALUE_ERROR (right_value))
    {
      if (free_left_value)
      {
        ecma_free_value (left_value);
      }
      return right_value;
    }
  }

  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

  if (ecma_is_value_string (left_value) || ecma_is_value_string (right_value))
  {
    ecma_string_t *string1_p = ecma_op_to_string (left_value);

    if (JERRY_UNLIKELY (string1_p == NULL))
    {
      if (free_left_value)
      {
        ecma_free_value (left_value);
      }
      if (free_right_value)
      {
        ecma_free_value (right_value);
      }
      return ECMA_VALUE_ERROR;
    }

    ecma_string_t *string2_p = ecma_op_to_string (right_value);

    if (JERRY_UNLIKELY (string2_p == NULL))
    {
      if (free_right_value)
      {
        ecma_free_value (right_value);
      }
      if (free_left_value)
      {
        ecma_free_value (left_value);
      }
      ecma_deref_ecma_string (string1_p);
      return ECMA_VALUE_ERROR;
    }

    string1_p = ecma_concat_ecma_strings (string1_p, string2_p);
    ret_value = ecma_make_string_value (string1_p);

    ecma_deref_ecma_string (string2_p);
  }
#if JERRY_BUILTIN_BIGINT
  else if (JERRY_UNLIKELY (ecma_is_value_bigint (left_value)) && JERRY_UNLIKELY (ecma_is_value_bigint (right_value)))
  {
    ret_value = ecma_bigint_add_sub (left_value, right_value, true);
  }
#endif /* JERRY_BUILTIN_BIGINT */
  else
  {
    ecma_number_t num_left;
    ecma_number_t num_right;
    if (!ECMA_IS_VALUE_ERROR (ecma_op_to_number (left_value, &num_left))
        && !ECMA_IS_VALUE_ERROR (ecma_op_to_number (right_value, &num_right)))
    {
      ret_value = ecma_make_number_value (num_left + num_right);
    }
    else
    {
      ret_value = ECMA_VALUE_ERROR;
    }
  }

  if (free_left_value)
  {
    ecma_free_value (left_value);
  }

  if (free_right_value)
  {
    ecma_free_value (right_value);
  }

  return ret_value;
} /* opfunc_addition */

/**
 * Unary operation opcode handler.
 *
 * See also: ECMA-262 v5, 11.4, 11.4.6, 11.4.7
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
opfunc_unary_operation (ecma_value_t left_value, /**< left value */
                        bool is_plus) /**< unary plus flag */
{
  ecma_number_t left_number;
  left_value = ecma_op_to_numeric (left_value, &left_number, ECMA_TO_NUMERIC_ALLOW_BIGINT);

  if (ECMA_IS_VALUE_ERROR (left_value))
  {
    return left_value;
  }

#if JERRY_BUILTIN_BIGINT
  if (JERRY_LIKELY (!ecma_is_value_bigint (left_value)))
  {
    return ecma_make_number_value (is_plus ? left_number : -left_number);
  }

  ecma_value_t ret_value;

  if (is_plus)
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_UNARY_PLUS_IS_NOT_ALLOWED_FOR_BIGINTS);
  }
  else
  {
    ret_value = left_value;

    if (left_value != ECMA_BIGINT_ZERO)
    {
      ret_value = ecma_bigint_negate (ecma_get_extended_primitive_from_value (left_value));
    }
  }

  ecma_free_value (left_value);
  return ret_value;
#else /* !JERRY_BUILTIN_BIGINT */
  return ecma_make_number_value (is_plus ? left_number : -left_number);
#endif /* JERRY_BUILTIN_BIGINT */
} /* opfunc_unary_operation */

/**
 * @}
 * @}
 */
