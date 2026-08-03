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

#include "opcodes.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup vm_opcodes Opcodes
 * @{
 */

/**
 * Perform ECMA number logic operation.
 *
 * The algorithm of the operation is following:
 *   leftNum = ToNumber (leftValue);
 *   rightNum = ToNumber (rightValue);
 *   result = leftNum BitwiseLogicOp rightNum;
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
do_number_bitwise_logic (number_bitwise_logic_op op, /**< number bitwise logic operation */
                         ecma_value_t left_value, /**< left value */
                         ecma_value_t right_value) /**< right value */
{
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (left_value) && !ECMA_IS_VALUE_ERROR (right_value));

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
    uint32_t right_uint32 = ecma_number_to_uint32 (right_number);

    switch (op)
    {
      case NUMBER_BITWISE_LOGIC_AND:
      {
        uint32_t left_uint32 = ecma_number_to_uint32 (left_number);
        result = (ecma_number_t) ((int32_t) (left_uint32 & right_uint32));
        break;
      }
      case NUMBER_BITWISE_LOGIC_OR:
      {
        uint32_t left_uint32 = ecma_number_to_uint32 (left_number);
        result = (ecma_number_t) ((int32_t) (left_uint32 | right_uint32));
        break;
      }
      case NUMBER_BITWISE_LOGIC_XOR:
      {
        uint32_t left_uint32 = ecma_number_to_uint32 (left_number);
        result = (ecma_number_t) ((int32_t) (left_uint32 ^ right_uint32));
        break;
      }
      case NUMBER_BITWISE_SHIFT_LEFT:
      {
        result = (ecma_number_t) ((int32_t) ((uint32_t) ecma_number_to_int32 (left_number) << (right_uint32 & 0x1F)));
        break;
      }
      case NUMBER_BITWISE_SHIFT_RIGHT:
      {
        result = (ecma_number_t) (ecma_number_to_int32 (left_number) >> (right_uint32 & 0x1F));
        break;
      }
      default:
      {
        JERRY_ASSERT (op == NUMBER_BITWISE_SHIFT_URIGHT);

        uint32_t left_uint32 = ecma_number_to_uint32 (left_number);
        result = (ecma_number_t) (left_uint32 >> (right_uint32 & 0x1F));
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
      case NUMBER_BITWISE_LOGIC_AND:
      {
        ret_value = ecma_bigint_and (left_value, right_value);
        break;
      }
      case NUMBER_BITWISE_LOGIC_OR:
      {
        ret_value = ecma_bigint_or (left_value, right_value);
        break;
      }
      case NUMBER_BITWISE_LOGIC_XOR:
      {
        ret_value = ecma_bigint_xor (left_value, right_value);
        break;
      }
      case NUMBER_BITWISE_SHIFT_LEFT:
      {
        ret_value = ecma_bigint_shift (left_value, right_value, true);
        break;
      }
      case NUMBER_BITWISE_SHIFT_RIGHT:
      {
        ret_value = ecma_bigint_shift (left_value, right_value, false);
        break;
      }
      default:
      {
        JERRY_ASSERT (op == NUMBER_BITWISE_SHIFT_URIGHT);

        ret_value = ecma_raise_type_error (ECMA_ERR_UNSIGNED_RIGHT_SHIFT_IS_NOT_ALLOWED_FOR_BIGINTS);
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
} /* do_number_bitwise_logic */

/**
 * Perform ECMA number bitwise not operation.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
do_number_bitwise_not (ecma_value_t value) /**< value */
{
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (value));

  ecma_number_t number;
  value = ecma_op_to_numeric (value, &number, ECMA_TO_NUMERIC_ALLOW_BIGINT);

  if (ECMA_IS_VALUE_ERROR (value))
  {
    return value;
  }

#if JERRY_BUILTIN_BIGINT
  if (JERRY_LIKELY (!ecma_is_value_bigint (value)))
  {
#endif /* JERRY_BUILTIN_BIGINT */
    return ecma_make_number_value ((ecma_number_t) ((int32_t) ~ecma_number_to_uint32 (number)));
#if JERRY_BUILTIN_BIGINT
  }

  ecma_value_t ret_value = ecma_bigint_unary (value, ECMA_BIGINT_UNARY_BITWISE_NOT);
  ecma_free_value (value);
  return ret_value;
#endif /* JERRY_BUILTIN_BIGINT */
} /* do_number_bitwise_not */

/**
 * @}
 * @}
 */
