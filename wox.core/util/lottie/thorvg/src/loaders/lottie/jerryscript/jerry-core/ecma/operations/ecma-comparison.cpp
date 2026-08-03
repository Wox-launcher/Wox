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

#include "ecma-comparison.h"

#include "ecma-bigint.h"
#include "ecma-conversion.h"
#include "ecma-globals.h"
#include "ecma-objects.h"

#include "jrt.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmacomparison ECMA comparison
 * @{
 */

/**
 * ECMA abstract equality comparison routine.
 *
 * See also: ECMA-262 v5, 11.9.3
 *
 * Note:
 *      This function might raise an exception, so the
 *      returned value must be freed with ecma_free_value.
 *
 * @return true - if values are equal,
 *         false - otherwise
 *         error - in case of any problems
 */
ecma_value_t
ecma_op_abstract_equality_compare (ecma_value_t x, /**< first operand */
                                   ecma_value_t y) /**< second operand */
{
  if (x == y)
  {
    return ECMA_VALUE_TRUE;
  }

  if (ecma_are_values_integer_numbers (x, y))
  {
    /* Note: the (x == y) comparison captures the true case. */
    return ECMA_VALUE_FALSE;
  }

  if (ecma_is_value_number (x))
  {
    if (ecma_is_value_number (y))
    {
      /* 1.c */
      ecma_number_t x_num = ecma_get_number_from_value (x);
      ecma_number_t y_num = ecma_get_number_from_value (y);

      bool is_x_equal_to_y = (x_num == y_num);

#ifndef JERRY_NDEBUG
      bool is_x_equal_to_y_check;

      if (ecma_number_is_nan (x_num) || ecma_number_is_nan (y_num))
      {
        is_x_equal_to_y_check = false;
      }
      else if (x_num == y_num || (ecma_number_is_zero (x_num) && ecma_number_is_zero (y_num)))
      {
        is_x_equal_to_y_check = true;
      }
      else
      {
        is_x_equal_to_y_check = false;
      }

      JERRY_ASSERT (is_x_equal_to_y == is_x_equal_to_y_check);
#endif /* !JERRY_NDEBUG */

      return ecma_make_boolean_value (is_x_equal_to_y);
    }

    /* Swap values. */
    x ^= y;
    y ^= x;
    x ^= y;
  }

  if (ecma_is_value_string (x))
  {
    if (ecma_is_value_string (y))
    {
      /* 1., d. */
      ecma_string_t *x_str_p = ecma_get_string_from_value (x);
      ecma_string_t *y_str_p = ecma_get_string_from_value (y);

      bool is_equal = ecma_compare_ecma_strings (x_str_p, y_str_p);

      return ecma_make_boolean_value (is_equal);
    }

    if (ecma_is_value_number (y))
    {
      /* 4. */
      ecma_number_t num;
      ecma_value_t x_num_value = ecma_op_to_number (x, &num);

      if (ECMA_IS_VALUE_ERROR (x_num_value))
      {
        return x_num_value;
      }
      ecma_value_t num_value = ecma_make_number_value (num);
      ecma_value_t compare_result = ecma_op_abstract_equality_compare (num_value, y);

      ecma_free_value (num_value);
      return compare_result;
    }

    /* Swap values. */
    x ^= y;
    y ^= x;
    x ^= y;
  }

  if (ecma_is_value_boolean (y))
  {
    if (ecma_is_value_boolean (x))
    {
      /* 1., e. */
      /* Note: the (x == y) comparison captures the true case. */
      return ECMA_VALUE_FALSE;
    }

    /* 7. */
    return ecma_op_abstract_equality_compare (x, ecma_make_integer_value (ecma_is_value_true (y) ? 1 : 0));
  }

  if (ecma_is_value_boolean (x))
  {
    /* 6. */
    return ecma_op_abstract_equality_compare (ecma_make_integer_value (ecma_is_value_true (x) ? 1 : 0), y);
  }

#if JERRY_BUILTIN_BIGINT
  if (JERRY_UNLIKELY (ecma_is_value_bigint (x)))
  {
    if (ecma_is_value_bigint (y))
    {
      return ecma_make_boolean_value (ecma_bigint_is_equal_to_bigint (x, y));
    }

    if (ecma_is_value_string (y))
    {
      ecma_value_t bigint = ecma_bigint_parse_string_value (y, ECMA_BIGINT_PARSE_DISALLOW_SYNTAX_ERROR);

      if (ECMA_IS_VALUE_ERROR (bigint) || bigint == ECMA_VALUE_FALSE)
      {
        return bigint;
      }

      JERRY_ASSERT (ecma_is_value_bigint (bigint));

      ecma_value_t result = ecma_make_boolean_value (ecma_bigint_is_equal_to_bigint (x, bigint));

      ecma_free_value (bigint);
      return result;
    }

    if (ecma_is_value_number (y))
    {
      return ecma_make_boolean_value (ecma_bigint_is_equal_to_number (x, ecma_get_number_from_value (y)));
    }

    /* Swap values. */
    x ^= y;
    y ^= x;
    x ^= y;
  }
#endif /* JERRY_BUILTIN_BIGINT */

  if (ecma_is_value_undefined (x) || ecma_is_value_null (x))
  {
    /* 1. a., b. */
    /* 2., 3. */
    bool is_equal = ecma_is_value_undefined (y) || ecma_is_value_null (y);

    return ecma_make_boolean_value (is_equal);
  }

  if (JERRY_UNLIKELY (ecma_is_value_symbol (x)))
  {
    if (!ecma_is_value_object (y))
    {
      return ECMA_VALUE_FALSE;
    }

    /* Swap values. */
    x ^= y;
    y ^= x;
    x ^= y;
  }

  JERRY_ASSERT (ecma_is_value_object (x));

  if (ecma_is_value_string (y) || ecma_is_value_symbol (y)
#if JERRY_BUILTIN_BIGINT
      || ecma_is_value_bigint (y)
#endif /* JERRY_BUILTIN_BIGINT */
      || ecma_is_value_number (y))
  {
    /* 9. */
    ecma_object_t *obj_p = ecma_get_object_from_value (x);

    ecma_value_t def_value = ecma_op_object_default_value (obj_p, ECMA_PREFERRED_TYPE_NO);

    if (ECMA_IS_VALUE_ERROR (def_value))
    {
      return def_value;
    }

    ecma_value_t compare_result = ecma_op_abstract_equality_compare (def_value, y);

    ecma_free_value (def_value);

    return compare_result;
  }

  return ECMA_VALUE_FALSE;
} /* ecma_op_abstract_equality_compare */

/**
 * ECMA strict equality comparison routine.
 *
 * See also: ECMA-262 v5, 11.9.6
 *
 * @return true - if values are strict equal,
 *         false - otherwise
 */
bool
ecma_op_strict_equality_compare (ecma_value_t x, /**< first operand */
                                 ecma_value_t y) /**< second operand */
{
  if (ecma_is_value_direct (x) || ecma_is_value_direct (y) || ecma_is_value_symbol (x) || ecma_is_value_symbol (y)
      || ecma_is_value_object (x) || ecma_is_value_object (y))
  {
    JERRY_ASSERT (!ecma_is_value_direct (x) || ecma_is_value_undefined (x) || ecma_is_value_null (x)
                  || ecma_is_value_boolean (x) || ecma_is_value_integer_number (x));

    JERRY_ASSERT (!ecma_is_value_direct (y) || ecma_is_value_undefined (y) || ecma_is_value_null (y)
                  || ecma_is_value_boolean (y) || ecma_is_value_integer_number (y));

    if ((x != ecma_make_integer_value (0) || !ecma_is_value_float_number (y))
        && (y != ecma_make_integer_value (0) || !ecma_is_value_float_number (x)))
    {
      return (x == y);
    }

    /* The +0 === -0 case handled below. */
  }

  JERRY_ASSERT (ecma_is_value_number (x) || ecma_is_value_string (x) || ecma_is_value_bigint (x));
  JERRY_ASSERT (ecma_is_value_number (y) || ecma_is_value_string (y) || ecma_is_value_bigint (y));

  if (ecma_is_value_string (x))
  {
    if (!ecma_is_value_string (y))
    {
      return false;
    }

    ecma_string_t *x_str_p = ecma_get_string_from_value (x);
    ecma_string_t *y_str_p = ecma_get_string_from_value (y);

    return ecma_compare_ecma_strings (x_str_p, y_str_p);
  }

#if JERRY_BUILTIN_BIGINT
  if (JERRY_UNLIKELY (ecma_is_value_bigint (x)))
  {
    if (!ecma_is_value_bigint (y))
    {
      return false;
    }

    return ecma_bigint_is_equal_to_bigint (x, y);
  }
#endif /* JERRY_BUILTIN_BIGINT */

  if (!ecma_is_value_number (y))
  {
    return false;
  }

  ecma_number_t x_num = ecma_get_number_from_value (x);
  ecma_number_t y_num = ecma_get_number_from_value (y);

  bool is_x_equal_to_y = (x_num == y_num);

#ifndef JERRY_NDEBUG
  bool is_x_equal_to_y_check;

  if (ecma_number_is_nan (x_num) || ecma_number_is_nan (y_num))
  {
    is_x_equal_to_y_check = false;
  }
  else if (x_num == y_num || (ecma_number_is_zero (x_num) && ecma_number_is_zero (y_num)))
  {
    is_x_equal_to_y_check = true;
  }
  else
  {
    is_x_equal_to_y_check = false;
  }

  JERRY_ASSERT (is_x_equal_to_y == is_x_equal_to_y_check);
#endif /* !JERRY_NDEBUG */

  return is_x_equal_to_y;
} /* ecma_op_strict_equality_compare */

/**
 * ECMA abstract relational comparison routine.
 *
 * See also: ECMA-262 v5, 11.8.5
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_abstract_relational_compare (ecma_value_t x, /**< first operand */
                                     ecma_value_t y, /**< second operand */
                                     bool left_first) /**< 'LeftFirst' flag */
{
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

  /* 1., 2. */
  ecma_value_t prim_first_converted_value = ecma_op_to_primitive (x, ECMA_PREFERRED_TYPE_NUMBER);
  if (ECMA_IS_VALUE_ERROR (prim_first_converted_value))
  {
    return prim_first_converted_value;
  }

  ecma_value_t prim_second_converted_value = ecma_op_to_primitive (y, ECMA_PREFERRED_TYPE_NUMBER);
  if (ECMA_IS_VALUE_ERROR (prim_second_converted_value))
  {
    ecma_free_value (prim_first_converted_value);
    return prim_second_converted_value;
  }

  ecma_value_t px = left_first ? prim_first_converted_value : prim_second_converted_value;
  ecma_value_t py = left_first ? prim_second_converted_value : prim_first_converted_value;

  const bool is_px_string = ecma_is_value_string (px);
  const bool is_py_string = ecma_is_value_string (py);

  if (!(is_px_string && is_py_string))
  {
#if JERRY_BUILTIN_BIGINT
    if (JERRY_LIKELY (!ecma_is_value_bigint (px)) && JERRY_LIKELY (!ecma_is_value_bigint (py)))
    {
#endif /* JERRY_BUILTIN_BIGINT */
      /* 3. */

      /* a. */

      ecma_number_t nx;
      ecma_number_t ny;

      if (ECMA_IS_VALUE_ERROR (ecma_op_to_number (px, &nx)) || ECMA_IS_VALUE_ERROR (ecma_op_to_number (py, &ny)))
      {
        ret_value = ECMA_VALUE_ERROR;
        goto end;
      }

      /* b. */
      if (ecma_number_is_nan (nx) || ecma_number_is_nan (ny))
      {
        /* c., d. */
        ret_value = ECMA_VALUE_UNDEFINED;
      }
      else
      {
        bool is_x_less_than_y = (nx < ny);

#ifndef JERRY_NDEBUG
        bool is_x_less_than_y_check;

        if (nx == ny || (ecma_number_is_zero (nx) && ecma_number_is_zero (ny)))
        {
          /* e., f., g. */
          is_x_less_than_y_check = false;
        }
        else if (ecma_number_is_infinity (nx) && !ecma_number_is_negative (nx))
        {
          /* h. */
          is_x_less_than_y_check = false;
        }
        else if (ecma_number_is_infinity (ny) && !ecma_number_is_negative (ny))
        {
          /* i. */
          is_x_less_than_y_check = true;
        }
        else if (ecma_number_is_infinity (ny) && ecma_number_is_negative (ny))
        {
          /* j. */
          is_x_less_than_y_check = false;
        }
        else if (ecma_number_is_infinity (nx) && ecma_number_is_negative (nx))
        {
          /* k. */
          is_x_less_than_y_check = true;
        }
        else
        {
          /* l. */
          JERRY_ASSERT (!ecma_number_is_nan (nx) && !ecma_number_is_infinity (nx));
          JERRY_ASSERT (!ecma_number_is_nan (ny) && !ecma_number_is_infinity (ny));
          JERRY_ASSERT (!(ecma_number_is_zero (nx) && ecma_number_is_zero (ny)));

          if (nx < ny)
          {
            is_x_less_than_y_check = true;
          }
          else
          {
            is_x_less_than_y_check = false;
          }
        }

        JERRY_ASSERT (is_x_less_than_y_check == is_x_less_than_y);
#endif /* !JERRY_NDEBUG */

        ret_value = ecma_make_boolean_value (is_x_less_than_y);
      }
#if JERRY_BUILTIN_BIGINT
    }
    else
    {
      bool invert_result = false;
      int compare_result = 0;

      if (!ecma_is_value_bigint (px))
      {
        ecma_value_t tmp = px;
        px = py;
        py = tmp;
        invert_result = true;
      }

      JERRY_ASSERT (ecma_is_value_bigint (px));

      if (ecma_is_value_bigint (py))
      {
        compare_result = ecma_bigint_compare_to_bigint (px, py);
      }
      else if (ecma_is_value_string (py))
      {
        ret_value = ecma_bigint_parse_string_value (py, ECMA_BIGINT_PARSE_DISALLOW_SYNTAX_ERROR);

        if (!ECMA_IS_VALUE_ERROR (ret_value))
        {
          if (ret_value == ECMA_VALUE_FALSE)
          {
            ret_value = ECMA_VALUE_UNDEFINED;
          }
          else
          {
            compare_result = ecma_bigint_compare_to_bigint (px, ret_value);
            ecma_free_value (ret_value);
            ret_value = ECMA_VALUE_EMPTY;
          }
        }
      }
      else
      {
        ecma_number_t ny;
        if (ECMA_IS_VALUE_ERROR (ecma_op_to_number (py, &ny)))
        {
          ret_value = ECMA_VALUE_ERROR;
          goto end;
        }

        if (ecma_number_is_nan (ny))
        {
          ret_value = ECMA_VALUE_UNDEFINED;
        }
        else
        {
          compare_result = ecma_bigint_compare_to_number (px, ny);
        }
      }

      if (ret_value == ECMA_VALUE_EMPTY)
      {
        if (invert_result)
        {
          compare_result = -compare_result;
        }

        ret_value = ecma_make_boolean_value (compare_result < 0);
      }
    }
#endif /* JERRY_BUILTIN_BIGINT */
  }
  else
  { /* 4. */
    JERRY_ASSERT (is_px_string && is_py_string);

    ecma_string_t *str_x_p = ecma_get_string_from_value (px);
    ecma_string_t *str_y_p = ecma_get_string_from_value (py);

    bool is_px_less = ecma_compare_ecma_strings_relational (str_x_p, str_y_p);

    ret_value = ecma_make_boolean_value (is_px_less);
  }

end:
  ecma_free_value (prim_second_converted_value);
  ecma_free_value (prim_first_converted_value);

  return ret_value;
} /* ecma_op_abstract_relational_compare */

/**
 * @}
 * @}
 */
