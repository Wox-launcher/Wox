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
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#include "jrt-libc-includes.h"
#include "jrt.h"

#if defined(_WIN32)
#include <intrin.h>
#endif /* defined(_WIN32) */

#if JERRY_BUILTIN_MATH

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
  ECMA_MATH_OBJECT_ROUTINE_START = 0,

  ECMA_MATH_OBJECT_ABS, /* ECMA-262 v5, 15.8.2.1 */
  ECMA_MATH_OBJECT_ACOS, /* ECMA-262 v5, 15.8.2.2 */
  ECMA_MATH_OBJECT_ASIN, /* ECMA-262 v5, 15.8.2.3 */
  ECMA_MATH_OBJECT_ATAN, /* ECMA-262 v5, 15.8.2.4 */
  ECMA_MATH_OBJECT_CEIL, /* ECMA-262 v5, 15.8.2.6 */
  ECMA_MATH_OBJECT_COS, /* ECMA-262 v5, 15.8.2.7 */
  ECMA_MATH_OBJECT_EXP, /* ECMA-262 v5, 15.8.2.8 */
  ECMA_MATH_OBJECT_FLOOR, /* ECMA-262 v5, 15.8.2.9 */
  ECMA_MATH_OBJECT_LOG, /* ECMA-262 v5, 15.8.2.10 */
  ECMA_MATH_OBJECT_ROUND, /* ECMA-262 v5, 15.8.2.15 */
  ECMA_MATH_OBJECT_SIN, /* ECMA-262 v5, 15.8.2.16 */
  ECMA_MATH_OBJECT_SQRT, /* ECMA-262 v5, 15.8.2.17 */
  ECMA_MATH_OBJECT_TAN, /* ECMA-262 v5, 15.8.2.18 */
  ECMA_MATH_OBJECT_ACOSH, /* ECMA-262 v6, 20.2.2.3  */
  ECMA_MATH_OBJECT_ASINH, /* ECMA-262 v6, 20.2.2.5  */
  ECMA_MATH_OBJECT_ATANH, /* ECMA-262 v6, 20.2.2.7  */
  ECMA_MATH_OBJECT_CBRT, /* ECMA-262 v6, 20.2.2.9  */
  ECMA_MATH_OBJECT_CLZ32, /* ECMA-262 v6, 20.2.2.11  */
  ECMA_MATH_OBJECT_COSH, /* ECMA-262 v6, 20.2.2.13  */
  ECMA_MATH_OBJECT_EXPM1, /* ECMA-262 v6, 20.2.2.15  */
  ECMA_MATH_OBJECT_FROUND, /* ECMA-262 v6, 20.2.2.17  */
  ECMA_MATH_OBJECT_LOG1P, /* ECMA-262 v6, 20.2.2.21  */
  ECMA_MATH_OBJECT_LOG10, /* ECMA-262 v6, 20.2.2.22  */
  ECMA_MATH_OBJECT_LOG2, /* ECMA-262 v6, 20.2.2.23  */
  ECMA_MATH_OBJECT_SIGN, /* ECMA-262 v6, 20.2.2.29 */
  ECMA_MATH_OBJECT_SINH, /* ECMA-262 v6, 20.2.2.31  */
  ECMA_MATH_OBJECT_TANH, /* ECMA-262 v6, 20.2.2.34  */
  ECMA_MATH_OBJECT_TRUNC, /* ECMA-262 v6, 20.2.2.35  */
  ECMA_MATH_OBJECT_ATAN2,
  /* ECMA-262 v5, 15.8.2.5 */ /* first routine with 2 arguments */
  ECMA_MATH_OBJECT_IMUL, /* ECMA-262 v6, 20.2.2.19  */
  ECMA_MATH_OBJECT_POW,
  /* ECMA-262 v5, 15.8.2.13 */ /* last routine with 1 or 2 arguments*/
  ECMA_MATH_OBJECT_MAX, /* ECMA-262 v5, 15.8.2.11 */
  ECMA_MATH_OBJECT_MIN, /* ECMA-262 v5, 15.8.2.12 */
  ECMA_MATH_OBJECT_HYPOT, /* ECMA-262 v6, 20.2.2.18  */
  ECMA_MATH_OBJECT_RANDOM, /* ECMA-262 v5, 15.8.2.14 */
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-math.inc.h"
#define BUILTIN_UNDERSCORED_ID  math
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup object ECMA Object object built-in
 * @{
 */

/**
 * The Math object's 'max' 'min' routines.
 *
 * See also:
 *          ECMA-262 v5, 15.8.2.11
 *          ECMA-262 v5, 15.8.2.12
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_math_object_max_min (bool is_max, /**< 'max' or 'min' operation */
                                  const ecma_value_t *arg, /**< arguments list */
                                  uint32_t args_number) /**< number of arguments */
{
  ecma_number_t result_num = ecma_number_make_infinity (is_max);

  while (args_number > 0)
  {
    ecma_number_t arg_num;
    ecma_value_t value = ecma_op_to_number (*arg, &arg_num);

    if (ECMA_IS_VALUE_ERROR (value))
    {
      return value;
    }

    arg++;
    args_number--;

    if (ecma_number_is_nan (arg_num))
    {
      result_num = arg_num;
    }

    if (ecma_number_is_zero (arg_num) && ecma_number_is_zero (result_num))
    {
      bool is_negative = ecma_number_is_negative (arg_num);

      if (is_max ? !is_negative : is_negative)
      {
        result_num = arg_num;
      }
    }
    else
    {
      if (is_max ? (arg_num > result_num) : (arg_num < result_num))
      {
        result_num = arg_num;
      }
    }
  }

  return ecma_make_number_value (result_num);
} /* ecma_builtin_math_object_max_min */

/**
 * The Math object's 'hypot' routine
 *
 * See also:
 *          ECMA-262 v6, 20.2.2.18
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_math_object_hypot (const ecma_value_t *arg, /**< arguments list */
                                uint32_t args_number) /**< number of arguments */
{
  if (args_number == 0)
  {
    return ecma_make_number_value (0.0);
  }

  ecma_number_t result_num = 0;
  bool inf = false;

  while (args_number > 0)
  {
    ecma_number_t arg_num;
    ecma_value_t value = ecma_op_to_number (*arg, &arg_num);
    if (ECMA_IS_VALUE_ERROR (value))
    {
      return value;
    }

    arg++;
    args_number--;

    if (ecma_number_is_nan (arg_num) && !inf)
    {
      result_num = arg_num;
      continue;
    }

    if (ecma_number_is_infinity (arg_num))
    {
      inf = true;
      result_num = ecma_number_make_infinity (false);
      continue;
    }

    result_num += arg_num * arg_num;
  }

  return ecma_make_number_value (sqrt (result_num));
} /* ecma_builtin_math_object_hypot */

/**
 * The Math object's 'trunc' routine
 *
 * See also:
 *          ECMA-262 v6, 20.2.2.35
 *
 * @return ecma number
 */
static ecma_number_t
ecma_builtin_math_object_trunc (ecma_number_t arg)
{
  if (ecma_number_is_nan (arg) || ecma_number_is_infinity (arg) || ecma_number_is_zero (arg))
  {
    return arg;
  }

  if ((arg > 0) && (arg < 1))
  {
    return (ecma_number_t) 0.0;
  }

  if ((arg < 0) && (arg > -1))
  {
    return (ecma_number_t) -0.0;
  }

  return arg - (ecma_number_t) fmodf (arg, 1.0f);
} /* ecma_builtin_math_object_trunc */

/**
 * The Math object's 'sign' routine
 *
 * See also:
 *          ECMA-262 v6, 20.2.2.29
 *
 * @return ecma number
 */
static ecma_number_t
ecma_builtin_math_object_sign (ecma_number_t arg)
{
  if (ecma_number_is_nan (arg) || ecma_number_is_zero (arg))
  {
    return arg;
  }

  if (ecma_number_is_negative (arg))
  {
    return (ecma_number_t) -1.0;
  }

  return (ecma_number_t) 1.0;
} /* ecma_builtin_math_object_sign */

/**
 * The Math object's 'random' routine.
 *
 * See also:
 *          ECMA-262 v5, 15.8.2.14
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_math_object_random (void)
{
  const ecma_number_t rand_max = (ecma_number_t) RAND_MAX;
  const ecma_number_t rand_max_min_1 = (ecma_number_t) (RAND_MAX - 1);

  return ecma_make_number_value (((ecma_number_t) rand ()) / rand_max * rand_max_min_1 / rand_max);
} /* ecma_builtin_math_object_random */

/**
 * Dispatcher for the built-in's routines.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_math_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                    ecma_value_t this_arg, /**< 'this' argument value */
                                    const ecma_value_t arguments_list[], /**< list of arguments
                                                                          *   passed to routine */
                                    uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (this_arg);

  if (builtin_routine_id <= ECMA_MATH_OBJECT_POW)
  {
    ecma_number_t x = ecma_number_make_nan ();
    ecma_number_t y = ecma_number_make_nan ();

    if (arguments_number >= 1)
    {
      ecma_value_t value = ecma_op_to_number (arguments_list[0], &x);

      if (ECMA_IS_VALUE_ERROR (value))
      {
        return value;
      }
    }

    if (builtin_routine_id >= ECMA_MATH_OBJECT_ATAN2 && arguments_number >= 2)
    {
      if (ecma_is_value_number (arguments_list[1]))
      {
        y = ecma_get_number_from_value (arguments_list[1]);
      }
      else
      {
        ecma_value_t value = ecma_op_to_number (arguments_list[1], &y);

        if (ECMA_IS_VALUE_ERROR (value))
        {
          return value;
        }
      }
    }

    switch (builtin_routine_id)
    {
      case ECMA_MATH_OBJECT_ABS:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (fabs (x));
        break;
      }
      case ECMA_MATH_OBJECT_ACOS:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (acos (x));
        break;
      }
      case ECMA_MATH_OBJECT_ASIN:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (asin (x));
        break;
      }
      case ECMA_MATH_OBJECT_ATAN:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (atan (x));
        break;
      }
      case ECMA_MATH_OBJECT_CEIL:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (ceil (x));
        break;
      }
      case ECMA_MATH_OBJECT_COS:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (cos (x));
        break;
      }
      case ECMA_MATH_OBJECT_EXP:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (exp (x));
        break;
      }
      case ECMA_MATH_OBJECT_FLOOR:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (floor (x));
        break;
      }
      case ECMA_MATH_OBJECT_LOG:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (log (x));
        break;
      }
      case ECMA_MATH_OBJECT_TRUNC:
      {
        x = ecma_builtin_math_object_trunc (x);
        break;
      }
      case ECMA_MATH_OBJECT_SIGN:
      {
        x = ecma_builtin_math_object_sign (x);
        break;
      }
      case ECMA_MATH_OBJECT_ROUND:
      {
        if (ecma_number_is_nan (x) || ecma_number_is_zero (x) || ecma_number_is_infinity (x))
        {
          break;
        }

        ecma_number_t fraction = fmod (x, ECMA_NUMBER_ONE);

        if (ecma_number_is_zero (fraction))
        {
          break;
        }

        if (ecma_number_is_negative (x))
        {
          if (x >= -ECMA_NUMBER_HALF)
          {
            x = -ECMA_NUMBER_ZERO;
            break;
          }

          if (fraction < -ECMA_NUMBER_HALF)
          {
            x -= ECMA_NUMBER_HALF;
          }
        }
        else if (fraction >= ECMA_NUMBER_HALF)
        {
          x += ECMA_NUMBER_HALF;
        }

        x = ecma_number_trunc (x);
        break;
      }
      case ECMA_MATH_OBJECT_SIN:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (sin (x));
        break;
      }
      case ECMA_MATH_OBJECT_SQRT:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (sqrt (x));
        break;
      }
      case ECMA_MATH_OBJECT_TAN:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (tan (x));
        break;
      }
      case ECMA_MATH_OBJECT_ATAN2:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (atan2 (x, y));
        break;
      }
      case ECMA_MATH_OBJECT_POW:
      {
        x = ecma_number_pow (x, y);
        break;
      }
      case ECMA_MATH_OBJECT_ACOSH:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (acosh (x));
        break;
      }
      case ECMA_MATH_OBJECT_ASINH:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (asinh (x));
        break;
      }
      case ECMA_MATH_OBJECT_ATANH:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (atanh (x));
        break;
      }
      case ECMA_MATH_OBJECT_CBRT:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (cbrt (x));
        break;
      }
      case ECMA_MATH_OBJECT_COSH:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (cosh (x));
        break;
      }
      case ECMA_MATH_OBJECT_EXPM1:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (expm1 (x));
        break;
      }
      case ECMA_MATH_OBJECT_LOG1P:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (log1p (x));
        break;
      }
      case ECMA_MATH_OBJECT_LOG10:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (log10 (x));
        break;
      }
      case ECMA_MATH_OBJECT_LOG2:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (log2 (x));
        break;
      }
      case ECMA_MATH_OBJECT_SINH:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (sinh (x));
        break;
      }
      case ECMA_MATH_OBJECT_TANH:
      {
        x = DOUBLE_TO_ECMA_NUMBER_T (tanh (x));
        break;
      }
      case ECMA_MATH_OBJECT_CLZ32:
      {
        uint32_t n = ecma_number_to_uint32 (x);
#if defined(__GNUC__) || defined(__clang__)
        x = n ? __builtin_clz (n) : 32;
#elif defined(_WIN32)
        unsigned long ret;
        x = (ecma_number_t)(_BitScanReverse (&ret, n) ? 31 - ret : 32);
#else /* !(defined(__GNUC__) || defined(__clang__) || defined(_WIN32)) */
        x = 32;
        for (int i = 31; i >= 0; i--)
        {
          if (n >> i)
          {
            x = 31 - i;
            break;
          }
        }
#endif /* defined (__GNUC__) || defined (__clang__) */
        break;
      }
      case ECMA_MATH_OBJECT_FROUND:
      {
        break;
      }
      case ECMA_MATH_OBJECT_IMUL:
      {
        x = (ecma_number_t) (ecma_number_to_uint32 (x) * ecma_number_to_uint32 (y));
        break;
      }
    }
    return ecma_make_number_value (x);
  } /* if (builtin_routine_id <= ECMA_MATH_OBJECT_POW) */

  if (builtin_routine_id <= ECMA_MATH_OBJECT_MIN)
  {
    return ecma_builtin_math_object_max_min (builtin_routine_id == ECMA_MATH_OBJECT_MAX,
                                             arguments_list,
                                             arguments_number);
  }

  if (builtin_routine_id == ECMA_MATH_OBJECT_HYPOT)
  {
    return ecma_builtin_math_object_hypot (arguments_list, arguments_number);
  }

  JERRY_ASSERT (builtin_routine_id == ECMA_MATH_OBJECT_RANDOM);

  return ecma_builtin_math_object_random ();
} /* ecma_builtin_math_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_MATH */
