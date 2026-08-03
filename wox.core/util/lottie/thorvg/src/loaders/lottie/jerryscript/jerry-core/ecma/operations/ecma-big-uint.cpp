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

#include "ecma-big-uint.h"

#include "ecma-helpers.h"

#include "jmem.h"
#include "lit-char-helpers.h"

#if JERRY_BUILTIN_BIGINT

JERRY_STATIC_ASSERT ((sizeof (ecma_bigint_two_digits_t) == 2 * sizeof (ecma_bigint_digit_t)),
                     ecma_big_int_two_digits_must_be_twice_as_long_as_ecma_big_int_digit);

JERRY_STATIC_ASSERT (((1 << (int)ECMA_BIGINT_DIGIT_SHIFT) == (8 * sizeof (ecma_bigint_digit_t))),
                     ecma_bigint_digit_shift_is_incorrect);

JERRY_STATIC_ASSERT ((((int)ECMA_BIG_UINT_BITWISE_DECREASE_LEFT << 1) == (int)ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT),
                     ecma_big_uint_bitwise_left_and_right_sub_option_bits_must_follow_each_other);

/**
 * Create a new BigInt value
 *
 * @return new BigInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_bigint_create (uint32_t size) /**< size of the new BigInt value */
{
  JERRY_ASSERT (size > 0);
  JERRY_ASSERT ((size % sizeof (ecma_bigint_digit_t)) == 0);

  if (JERRY_UNLIKELY (size > ECMA_BIGINT_MAX_SIZE))
  {
    return NULL;
  }

  ecma_extended_primitive_t *value_p;

  size_t mem_size = ECMA_BIGINT_GET_BYTE_SIZE (size) + sizeof (ecma_extended_primitive_t);
  value_p = (ecma_extended_primitive_t *) jmem_heap_alloc_block_null_on_error (mem_size);

  if (JERRY_UNLIKELY (value_p == NULL))
  {
    return NULL;
  }

  value_p->refs_and_type = ECMA_EXTENDED_PRIMITIVE_REF_ONE | ECMA_TYPE_BIGINT;
  value_p->u.bigint_sign_and_size = size;
  return value_p;
} /* ecma_bigint_create */

/**
 * Extend a BigUInt value with a new data prefix value
 *
 * @return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_extend (ecma_extended_primitive_t *value_p, /**< BigUInt value */
                      ecma_bigint_digit_t digit) /**< new digit */
{
  uint32_t old_size = ECMA_BIGINT_GET_SIZE (value_p);

  if (ECMA_BIGINT_SIZE_IS_ODD (old_size))
  {
    value_p->u.bigint_sign_and_size += (uint32_t) sizeof (ecma_bigint_digit_t);
    *ECMA_BIGINT_GET_DIGITS (value_p, old_size) = digit;
    return value_p;
  }

  ecma_extended_primitive_t *result_p = ecma_bigint_create (old_size + (uint32_t) sizeof (ecma_bigint_digit_t));

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    ecma_deref_bigint (value_p);
    return NULL;
  }

  memcpy (result_p + 1, value_p + 1, old_size);
  ecma_deref_bigint (value_p);

  *ECMA_BIGINT_GET_DIGITS (result_p, old_size) = digit;
  return result_p;
} /* ecma_big_uint_extend */

/**
 * Count the number of leading zero bits of a digit
 *
 * return number of leading zero bits
 */
ecma_bigint_digit_t
ecma_big_uint_count_leading_zero (ecma_bigint_digit_t digit) /**< digit value */
{
  ecma_bigint_digit_t shift = 4 * sizeof (ecma_bigint_digit_t);
  ecma_bigint_digit_t result = 8 * sizeof (ecma_bigint_digit_t);

  do
  {
    ecma_bigint_digit_t value = digit >> shift;
    if (value > 0)
    {
      digit = value;
      result -= shift;
    }
    shift >>= 1;
  } while (shift > 0);

  return result - digit;
} /* ecma_big_uint_count_leading_zero */

/**
 * Helper function which discards the leading zero digits of a BigUInt value
 *
 * @return new BigUInt value, NULL on error
 */
static ecma_extended_primitive_t *
ecma_big_uint_normalize_result (ecma_extended_primitive_t *value_p, /**< BigUInt value */
                                ecma_bigint_digit_t *last_digit_p) /**< points to the end of BigUInt */
{
  JERRY_ASSERT (last_digit_p[-1] == 0);

  ecma_bigint_digit_t *first_digit_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);

  /* The following code is tricky. The value stored in first_digit_p[-1] is the size
   * of the BigUInt value, and it cannot be zero. Hence the loop below will terminate. */
  JERRY_ASSERT (first_digit_p[-1] != 0);

  do
  {
    --last_digit_p;
  } while (last_digit_p[-1] == 0);

  JERRY_ASSERT (last_digit_p >= first_digit_p);

  if (first_digit_p == last_digit_p)
  {
    ecma_deref_bigint (value_p);
    return ECMA_BIGINT_POINTER_TO_ZERO;
  }

  uint32_t new_size = (uint32_t) ((uint8_t *) last_digit_p - (uint8_t *) first_digit_p);

  if (ECMA_BIGINT_SIZE_IS_ODD (new_size)
      && ((new_size + sizeof (ecma_bigint_digit_t)) == ECMA_BIGINT_GET_SIZE (value_p)))
  {
    value_p->u.bigint_sign_and_size -= (uint32_t) sizeof (ecma_bigint_digit_t);
    return value_p;
  }

  ecma_extended_primitive_t *result_p = ecma_bigint_create (new_size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    ecma_deref_bigint (value_p);
    return NULL;
  }

  memcpy (ECMA_BIGINT_GET_DIGITS (result_p, 0), ECMA_BIGINT_GET_DIGITS (value_p, 0), new_size);
  ecma_deref_bigint (value_p);

  return result_p;
} /* ecma_big_uint_normalize_result */

/**
 * Helper function which increases the result by 1 and extends or shrinks the BigUInt when necessary
 *
 * @return new BigUInt value, NULL on error
 */
static ecma_extended_primitive_t *
ecma_big_uint_increase_result (ecma_extended_primitive_t *value_p) /**< BigUInt value */
{
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  JERRY_ASSERT (size > 0);

  ecma_bigint_digit_t *first_digit_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);
  ecma_bigint_digit_t *last_digit_p = ECMA_BIGINT_GET_DIGITS (value_p, size);

  while (*first_digit_p == ~((ecma_bigint_digit_t) 0))
  {
    *first_digit_p++ = 0;

    if (first_digit_p == last_digit_p)
    {
      return ecma_big_uint_extend (value_p, 1);
    }
  }

  (*first_digit_p)++;

  if (last_digit_p[-1] != 0)
  {
    return value_p;
  }

  return ecma_big_uint_normalize_result (value_p, last_digit_p);
} /* ecma_big_uint_increase_result */

/**
 * Compare two BigUInt numbers
 *
 * return -1, if left value < right value, 0 if they are equal, and 1 otherwise
 */
int
ecma_big_uint_compare (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                       ecma_extended_primitive_t *right_value_p) /**< right BigUInt value */
{
  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  uint32_t right_size = ECMA_BIGINT_GET_SIZE (right_value_p);

  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);
  JERRY_ASSERT (right_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (right_value_p, right_size) != 0);

  if (left_size > right_size)
  {
    return 1;
  }

  if (left_size < right_size)
  {
    return -1;
  }

  ecma_bigint_digit_t *start_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, left_size);
  ecma_bigint_digit_t *right_p = ECMA_BIGINT_GET_DIGITS (right_value_p, left_size);

  do
  {
    ecma_bigint_digit_t left_value = *(--left_p);
    ecma_bigint_digit_t right_value = *(--right_p);

    if (left_value < right_value)
    {
      return -1;
    }

    if (left_value > right_value)
    {
      return 1;
    }
  } while (left_p > start_p);

  return 0;
} /* ecma_big_uint_compare */

/**
 * In-place multiply and addition operation with digit
 *
 * return updated value on success, NULL if no memory is available
 */
ecma_extended_primitive_t *
ecma_big_uint_mul_digit (ecma_extended_primitive_t *value_p, /**< BigUInt value */
                         ecma_bigint_digit_t mul, /**< multiply value */
                         ecma_bigint_digit_t add) /**< addition value */
{
  JERRY_ASSERT (mul > 1);
  JERRY_ASSERT (add < mul);

  if (JERRY_UNLIKELY (value_p == NULL))
  {
    JERRY_ASSERT (add > 0);

    value_p = ecma_bigint_create (sizeof (ecma_bigint_digit_t));

    if (JERRY_UNLIKELY (value_p == NULL))
    {
      return NULL;
    }

    *ECMA_BIGINT_GET_DIGITS (value_p, 0) = add;
    return value_p;
  }

  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  JERRY_ASSERT (size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (value_p, size) != 0);

  ecma_bigint_digit_t *current_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);
  ecma_bigint_digit_t *end_p = ECMA_BIGINT_GET_DIGITS (value_p, size);
  ecma_bigint_digit_t carry = add;

  do
  {
    ecma_bigint_two_digits_t multiply_result = ((ecma_bigint_two_digits_t) *current_p) * mul;
    ecma_bigint_digit_t multiply_result_low, new_carry;

    multiply_result_low = (ecma_bigint_digit_t) multiply_result;
    new_carry = (ecma_bigint_digit_t) (multiply_result >> (8 * sizeof (ecma_bigint_digit_t)));

    multiply_result_low += carry;
    if (multiply_result_low < carry)
    {
      new_carry++;
    }

    *current_p++ = multiply_result_low;
    carry = new_carry;
  } while (current_p < end_p);

  if (carry == 0)
  {
    return value_p;
  }

  return ecma_big_uint_extend (value_p, carry);
} /* ecma_big_uint_mul_digit */

/**
 * Convert a BigUInt to a human readable number
 *
 * return char sequence on success, NULL otherwise
 */
lit_utf8_byte_t *
ecma_big_uint_to_string (ecma_extended_primitive_t *value_p, /**< BigUInt value */
                         uint32_t radix, /**< radix number between 2 and 36 */
                         uint32_t *char_start_p, /**< [out] start offset of numbers */
                         uint32_t *char_size_p) /**< [out] size of the output buffer */
{
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  JERRY_ASSERT (radix >= 2 && radix <= 36);
  JERRY_ASSERT (size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (value_p, size) != 0);

  uint32_t max_size = size * 8;

  if (radix < 16)
  {
    if (radix >= 8)
    {
      /* Most frequent case. */
      max_size = (max_size + 2) / 3;
    }
    else if (radix >= 4)
    {
      max_size = (max_size + 1) >> 1;
    }
  }
  else if (radix < 32)
  {
    max_size = (max_size + 3) >> 2;
  }
  else
  {
    max_size = (max_size + 4) / 5;
  }

  /* This space can be used to store a sign. */
  max_size += (uint32_t) (2 * sizeof (ecma_bigint_digit_t) - 1);
  max_size &= ~(uint32_t) (sizeof (ecma_bigint_digit_t) - 1);
  *char_size_p = max_size;

  lit_utf8_byte_t *result_p = (lit_utf8_byte_t *) jmem_heap_alloc_block_null_on_error (max_size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return NULL;
  }

  memcpy (result_p, value_p + 1, size);

  ecma_bigint_digit_t *start_p = (ecma_bigint_digit_t *) (result_p + size);
  ecma_bigint_digit_t *end_p = (ecma_bigint_digit_t *) result_p;
  lit_utf8_byte_t *string_p = result_p + max_size;

  do
  {
    ecma_bigint_digit_t *current_p = (ecma_bigint_digit_t *) start_p;
    ecma_bigint_digit_t remainder = 0;

    if (sizeof (uintptr_t) == sizeof (ecma_bigint_two_digits_t))
    {
      do
      {
        ecma_bigint_two_digits_t result = *(--current_p) | ECMA_BIGINT_HIGH_DIGIT (remainder);

        *current_p = (ecma_bigint_digit_t) (result / radix);
        remainder = (ecma_bigint_digit_t) (result % radix);
      } while (current_p > end_p);
    }
    else
    {
      if (ECMA_BIGINT_SIZE_IS_ODD ((uintptr_t) current_p - (uintptr_t) end_p))
      {
        ecma_bigint_digit_t result = *(--current_p);
        *current_p = result / radix;
        remainder = result % radix;
      }

      while (current_p > end_p)
      {
        /* The following algorithm splits the 64 bit input into three numbers, extend
         * them with remainder, divide them by radix, and updates the three bit ranges
         * corresponding to the three numbers. */

        const uint32_t extract_bits_low = 10;
        const uint32_t extract_bits_low_mask = (uint32_t) ((1 << extract_bits_low) - 1);
        const uint32_t extract_bits_high = (uint32_t) ((sizeof (ecma_bigint_digit_t) * 8) - extract_bits_low);
        const uint32_t extract_bits_high_mask = (uint32_t) ((1 << extract_bits_high) - 1);

        ecma_bigint_digit_t result_high = current_p[-1];
        ecma_bigint_digit_t result_mid = (result_high & extract_bits_low_mask) << extract_bits_low;

        result_high = (result_high >> extract_bits_low) | (remainder << extract_bits_high);
        result_mid |= (result_high % radix) << (extract_bits_low * 2);
        result_high = (result_high / radix) << extract_bits_low;

        ecma_bigint_digit_t result_low = current_p[-2];
        result_mid |= result_low >> extract_bits_high;
        result_low = (result_low & extract_bits_high_mask) | ((result_mid % radix) << extract_bits_high);

        result_mid = result_mid / radix;

        current_p[-1] = result_high | (result_mid >> extract_bits_low);
        current_p[-2] = (result_low / radix) | (result_mid << extract_bits_high);

        remainder = result_low % radix;
        current_p -= 2;
      }
    }

    *(--string_p) =
      (lit_utf8_byte_t) ((remainder < 10) ? (remainder + LIT_CHAR_0) : (remainder + (LIT_CHAR_LOWERCASE_A - 10)));
    JERRY_ASSERT (string_p >= (lit_utf8_byte_t *) start_p);

    if (start_p[-1] == 0)
    {
      start_p--;
    }
  } while (start_p > end_p);

  *char_start_p = (uint32_t) (string_p - result_p);
  return result_p;
} /* ecma_big_uint_to_string */

/**
 * Increase the value of a BigUInt value by 1
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_increase (ecma_extended_primitive_t *value_p) /**< BigUInt value */
{
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  JERRY_ASSERT (size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (value_p, size) != 0);

  ecma_bigint_digit_t *digits_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);
  ecma_bigint_digit_t *digits_end_p = ECMA_BIGINT_GET_DIGITS (value_p, size);

  if (JERRY_UNLIKELY (digits_p[0] == ~((ecma_bigint_digit_t) 0) && digits_end_p[-1] == ~((ecma_bigint_digit_t) 0)))
  {
    do
    {
      digits_p++;
    } while (digits_p < digits_end_p && digits_p[0] == ~((ecma_bigint_digit_t) 0));

    if (digits_p == digits_end_p)
    {
      ecma_extended_primitive_t *result_value_p;
      result_value_p = ecma_bigint_create ((uint32_t) (size + sizeof (ecma_bigint_digit_t)));

      if (JERRY_UNLIKELY (result_value_p == NULL))
      {
        return NULL;
      }

      memset (ECMA_BIGINT_GET_DIGITS (result_value_p, 0), 0, size);
      *ECMA_BIGINT_GET_DIGITS (result_value_p, size) = 1;
      return result_value_p;
    }

    digits_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);
  }

  ecma_extended_primitive_t *result_value_p = ecma_bigint_create (size);

  if (JERRY_UNLIKELY (result_value_p == NULL))
  {
    return NULL;
  }

  ecma_bigint_digit_t *result_p = ECMA_BIGINT_GET_DIGITS (result_value_p, 0);

  while (digits_p[0] == ~((ecma_bigint_digit_t) 0))
  {
    digits_p++;
    *result_p++ = 0;
  }

  *result_p++ = (*digits_p++) + 1;

  if (digits_p < digits_end_p)
  {
    memcpy (result_p, digits_p, (size_t) ((uint8_t *) digits_end_p - (uint8_t *) digits_p));
  }
  return result_value_p;
} /* ecma_big_uint_increase */

/**
 * Decrease the value of a BigUInt value by 1
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_decrease (ecma_extended_primitive_t *value_p) /**< BigUInt value */
{
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  JERRY_ASSERT (size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (value_p, size) != 0);

  ecma_bigint_digit_t *digits_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);
  ecma_bigint_digit_t *digits_end_p = ECMA_BIGINT_GET_DIGITS (value_p, size);

  JERRY_ASSERT (size > sizeof (ecma_bigint_digit_t) || *digits_p > 1);

  if (JERRY_UNLIKELY (digits_p[0] == 0 && digits_end_p[-1] == 1))
  {
    do
    {
      digits_p++;
      JERRY_ASSERT (digits_p < digits_end_p);
    } while (digits_p[0] == 0);

    if (digits_p + 1 == digits_end_p)
    {
      size -= (uint32_t) sizeof (ecma_bigint_digit_t);
      ecma_extended_primitive_t *result_value_p = ecma_bigint_create (size);

      if (JERRY_UNLIKELY (result_value_p == NULL))
      {
        return NULL;
      }

      memset (ECMA_BIGINT_GET_DIGITS (result_value_p, 0), 0xff, size);
      return result_value_p;
    }

    digits_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);
  }

  ecma_extended_primitive_t *result_value_p = ecma_bigint_create (size);

  if (JERRY_UNLIKELY (result_value_p == NULL))
  {
    return NULL;
  }

  ecma_bigint_digit_t *result_p = ECMA_BIGINT_GET_DIGITS (result_value_p, 0);

  while (digits_p[0] == 0)
  {
    digits_p++;
    *result_p++ = ~((ecma_bigint_digit_t) 0);
  }

  *result_p++ = (*digits_p++) - 1;

  if (digits_p < digits_end_p)
  {
    memcpy (result_p, digits_p, (size_t) ((uint8_t *) digits_end_p - (uint8_t *) digits_p));
  }
  return result_value_p;
} /* ecma_big_uint_decrease */

/**
 * Add right BigUInt value to the left BigUInt value
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_add (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                   ecma_extended_primitive_t *right_value_p) /**< right BigUInt value */
{
  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  uint32_t right_size = ECMA_BIGINT_GET_SIZE (right_value_p);

  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);
  JERRY_ASSERT (right_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (right_value_p, right_size) != 0);

  if (left_size < right_size)
  {
    /* Swap values. */
    ecma_extended_primitive_t *tmp_value_p = left_value_p;
    left_value_p = right_value_p;
    right_value_p = tmp_value_p;

    uint32_t tmp_size = left_size;
    left_size = right_size;
    right_size = tmp_size;
  }

  ecma_extended_primitive_t *result_p = ecma_bigint_create (left_size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return NULL;
  }

  ecma_bigint_digit_t *current_p = ECMA_BIGINT_GET_DIGITS (result_p, 0);
  ecma_bigint_digit_t *end_p = ECMA_BIGINT_GET_DIGITS (result_p, right_size);
  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
  ecma_bigint_digit_t *right_p = ECMA_BIGINT_GET_DIGITS (right_value_p, 0);
  ecma_bigint_digit_t carry = 0;

  left_size -= right_size;

  do
  {
    ecma_bigint_digit_t left = *left_p++;

    if (carry == 0 || left != ~(ecma_bigint_digit_t) 0)
    {
      left += carry;
      carry = 0;
    }
    else
    {
      left = 0;
      carry = 1;
    }

    ecma_bigint_digit_t right = *right_p++;
    left += right;

    if (left < right)
    {
      JERRY_ASSERT (carry == 0);
      carry = 1;
    }

    *current_p++ = left;
  } while (current_p < end_p);

  end_p = (ecma_bigint_digit_t *) (((uint8_t *) end_p) + left_size);

  if (carry != 0)
  {
    while (true)
    {
      if (JERRY_UNLIKELY (current_p == end_p))
      {
        return ecma_big_uint_extend (result_p, 1);
      }

      ecma_bigint_digit_t value = *left_p++;

      if (value != ~(ecma_bigint_digit_t) 0)
      {
        *current_p++ = value + 1;
        break;
      }

      *current_p++ = 0;
    }
  }

  if (current_p < end_p)
  {
    memcpy (current_p, left_p, (size_t) ((uint8_t *) end_p - (uint8_t *) current_p));
  }

  return result_p;
} /* ecma_big_uint_add */

/**
 * Subtract right BigUInt value from the left BigUInt value
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_sub (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                   ecma_extended_primitive_t *right_value_p) /**< right BigUInt value */
{
  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  uint32_t right_size = ECMA_BIGINT_GET_SIZE (right_value_p);

  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);
  JERRY_ASSERT (right_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (right_value_p, right_size) != 0);
  JERRY_ASSERT (left_size >= right_size);

  ecma_extended_primitive_t *result_p = ecma_bigint_create (left_size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return NULL;
  }

  ecma_bigint_digit_t *current_p = ECMA_BIGINT_GET_DIGITS (result_p, 0);
  ecma_bigint_digit_t *end_p = ECMA_BIGINT_GET_DIGITS (result_p, right_size);
  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
  ecma_bigint_digit_t *right_p = ECMA_BIGINT_GET_DIGITS (right_value_p, 0);
  ecma_bigint_digit_t carry = 0;

  left_size -= right_size;

  do
  {
    ecma_bigint_digit_t left = *left_p++;
    ecma_bigint_digit_t right = *right_p++;

    if (carry == 0 || left != 0)
    {
      left -= carry;
      carry = left < right;
    }
    else
    {
      left = ~(ecma_bigint_digit_t) 0;
      carry = 1;
    }

    *current_p++ = left - right;
  } while (current_p < end_p);

  end_p = (ecma_bigint_digit_t *) (((uint8_t *) end_p) + left_size);

  if (carry != 0)
  {
    while (true)
    {
      JERRY_ASSERT (current_p < end_p);

      ecma_bigint_digit_t value = *left_p++;

      if (value != 0)
      {
        *current_p++ = value - 1;
        break;
      }

      *current_p++ = ~(ecma_bigint_digit_t) 0;
    }
  }

  if (current_p < end_p)
  {
    memcpy (current_p, left_p, (size_t) ((uint8_t *) end_p - (uint8_t *) current_p));
    return result_p;
  }

  if (current_p[-1] != 0)
  {
    return result_p;
  }

  return ecma_big_uint_normalize_result (result_p, current_p);
} /* ecma_big_uint_sub */

/**
 * Multiply two BigUInt values
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_mul (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                   ecma_extended_primitive_t *right_value_p) /**< right BigUInt value */
{
  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  uint32_t right_size = ECMA_BIGINT_GET_SIZE (right_value_p);

  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);
  JERRY_ASSERT (right_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (right_value_p, right_size) != 0);

  if (left_size < right_size)
  {
    /* Swap values. */
    ecma_extended_primitive_t *tmp_value_p = left_value_p;
    left_value_p = right_value_p;
    right_value_p = tmp_value_p;

    uint32_t tmp_size = left_size;
    left_size = right_size;
    right_size = tmp_size;
  }

  uint32_t result_size = left_size + right_size - (uint32_t) sizeof (ecma_bigint_digit_t);

  ecma_extended_primitive_t *result_p = ecma_bigint_create (result_size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return NULL;
  }

  memset (ECMA_BIGINT_GET_DIGITS (result_p, 0), 0, result_size);

  /* Lower amount of space is allocated by default. This value provides extra space if needed. */
  ecma_bigint_digit_t extra_space[1] = { 0 };

  ecma_bigint_digit_t *right_p = ECMA_BIGINT_GET_DIGITS (right_value_p, 0);
  ecma_bigint_digit_t *right_end_p = ECMA_BIGINT_GET_DIGITS (right_value_p, right_size);
  ecma_bigint_digit_t *left_start_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
  ecma_bigint_digit_t *left_end_p = ECMA_BIGINT_GET_DIGITS (left_value_p, left_size);

  ecma_bigint_digit_t *result_start_p = ECMA_BIGINT_GET_DIGITS (result_p, 0);
  ecma_bigint_digit_t *result_end_p = ECMA_BIGINT_GET_DIGITS (result_p, result_size);

  do
  {
    ecma_bigint_two_digits_t right = *right_p++;

    if (right == 0)
    {
      result_start_p++;
      continue;
    }

    ecma_bigint_digit_t *left_p = left_start_p;
    ecma_bigint_digit_t *destination_p = result_start_p;
    ecma_bigint_digit_t carry = 0;

    do
    {
      JERRY_ASSERT (destination_p != (ecma_bigint_digit_t *) (extra_space + 1));

      ecma_bigint_two_digits_t multiply_result;
      ecma_bigint_digit_t multiply_result_low, new_carry;
      ecma_bigint_digit_t value = *destination_p;

      multiply_result = ((ecma_bigint_two_digits_t) (*left_p++)) * ((ecma_bigint_two_digits_t) right);
      multiply_result_low = (ecma_bigint_digit_t) multiply_result;
      value += multiply_result_low;
      new_carry = (ecma_bigint_digit_t) (multiply_result >> (8 * sizeof (ecma_bigint_digit_t)));

      /* The new_carry can never overflow because:
       *   a) If left or right is less than 0xff..ff, new_carry will be less than or equal to
       *      0xff...fd, and increasing it by maximum of two (carries) cannot overflow.
       *   b) If left and right are both equal to 0xff..ff, multiply_result_low will be 1,
       *      and computing value + carry + 1 can only increase new_carry at most once. */

      if (value < multiply_result_low)
      {
        JERRY_ASSERT (new_carry < ~(ecma_bigint_digit_t) 0);
        new_carry++;
      }

      value += carry;

      if (value < carry)
      {
        JERRY_ASSERT (new_carry < ~(ecma_bigint_digit_t) 0);
        new_carry++;
      }

      carry = new_carry;
      *destination_p++ = value;

      if (destination_p == result_end_p)
      {
        destination_p = (ecma_bigint_digit_t *) extra_space;
      }
    } while (left_p < left_end_p);

    while (carry > 0)
    {
      JERRY_ASSERT (destination_p != (ecma_bigint_digit_t *) (extra_space + 1));

      ecma_bigint_digit_t value = *destination_p;

      value += carry;
      carry = (value < carry);

      *destination_p++ = value;

      if (destination_p == result_end_p)
      {
        destination_p = (ecma_bigint_digit_t *) extra_space;
      }
    }

    result_start_p++;
  } while (right_p < right_end_p);

  if (extra_space[0] == 0)
  {
    return result_p;
  }

  return ecma_big_uint_extend (result_p, extra_space[0]);
} /* ecma_big_uint_mul */

/**
 * Divide left BigUInt value with right digit value
 *
 * return new BigUInt value, NULL on error
 */
static ecma_extended_primitive_t *
ecma_big_uint_div_digit (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                         ecma_bigint_digit_t divisor_digit, /**< divisor value */
                         bool is_mod) /**< true if return with remainder */
{
  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);

  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);
  JERRY_ASSERT (divisor_digit > 0);

  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, left_size - sizeof (ecma_bigint_digit_t));
  ecma_bigint_digit_t *end_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);

  ecma_bigint_digit_t last_digit = *left_p;
  ecma_bigint_digit_t remainder = last_digit % divisor_digit;

  last_digit = last_digit / divisor_digit;

  ecma_extended_primitive_t *result_p = NULL;
  ecma_bigint_digit_t *current_p = NULL;

  if (!is_mod)
  {
    ecma_bigint_digit_t result_size = left_size;

    if (last_digit == 0)
    {
      result_size -= (uint32_t) sizeof (ecma_bigint_digit_t);
    }

    result_p = ecma_bigint_create (result_size);

    if (JERRY_UNLIKELY (result_p == NULL))
    {
      return NULL;
    }

    current_p = ECMA_BIGINT_GET_DIGITS (result_p, result_size);

    if (last_digit != 0)
    {
      *(--current_p) = last_digit;
    }
  }

  while (left_p > end_p)
  {
    const uint32_t shift = 1 << ECMA_BIGINT_DIGIT_SHIFT;

    ecma_bigint_two_digits_t result = *(--left_p) | (((ecma_bigint_two_digits_t) remainder) << shift);

    if (!is_mod)
    {
      *(--current_p) = (ecma_bigint_digit_t) (result / divisor_digit);
    }

    remainder = (ecma_bigint_digit_t) (result % divisor_digit);
  }

  if (!is_mod)
  {
    JERRY_ASSERT (current_p == ECMA_BIGINT_GET_DIGITS (result_p, 0));
    return result_p;
  }

  if (remainder == 0)
  {
    return ECMA_BIGINT_POINTER_TO_ZERO;
  }

  result_p = ecma_bigint_create (sizeof (ecma_bigint_digit_t));

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return NULL;
  }

  *ECMA_BIGINT_GET_DIGITS (result_p, 0) = remainder;
  return result_p;
} /* ecma_big_uint_div_digit */

/**
 * Shift left a BigUInt value by a digit value
 *
 * return newly allocated buffer, NULL on error
 */
static ecma_bigint_digit_t *
ecma_big_uint_div_shift_left (ecma_extended_primitive_t *value_p, /**< BigUInt value */
                              ecma_bigint_digit_t shift_left, /**< left shift */
                              bool extend) /**< extend the result with an extra digit */
{
  uint32_t size = ECMA_BIGINT_GET_SIZE (value_p);

  JERRY_ASSERT (size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (value_p, size) != 0);

  ecma_bigint_digit_t *source_p = ECMA_BIGINT_GET_DIGITS (value_p, 0);
  ecma_bigint_digit_t *end_p = ECMA_BIGINT_GET_DIGITS (value_p, size);

  if (extend)
  {
    size += (uint32_t) sizeof (ecma_bigint_digit_t);
  }

  ecma_bigint_digit_t *result_p = (ecma_bigint_digit_t *) jmem_heap_alloc_block_null_on_error (size);

  if (JERRY_UNLIKELY (result_p == NULL))
  {
    return result_p;
  }

  if (shift_left == 0)
  {
    JERRY_ASSERT (extend);

    size -= (uint32_t) sizeof (ecma_bigint_digit_t);
    *(ecma_bigint_digit_t *) (((uint8_t *) result_p) + size) = 0;

    memcpy (result_p, source_p, size);
    return result_p;
  }

  ecma_bigint_digit_t *destination_p = result_p;
  ecma_bigint_digit_t carry = 0;
  uint32_t shift_right = (1 << ECMA_BIGINT_DIGIT_SHIFT) - shift_left;

  do
  {
    ecma_bigint_digit_t value = *source_p++;

    *destination_p++ = (value << shift_left) | carry;
    carry = value >> shift_right;
  } while (source_p < end_p);

  if (extend)
  {
    *destination_p++ = carry;
  }

  return result_p;
} /* ecma_big_uint_div_shift_left */

/**
 * Divide left BigUInt value with right BigUInt value
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_div_mod (ecma_extended_primitive_t *dividend_value_p, /**< divider BigUInt value */
                       ecma_extended_primitive_t *divisor_value_p, /**< divisor BigUInt value */
                       bool is_mod) /**< true if return with remainder instead of quotient */
{
  /* This algorithm is based on Donald Knuth’s "Algorithm D" */
  uint32_t divisor_size = ECMA_BIGINT_GET_SIZE (divisor_value_p);

  JERRY_ASSERT (divisor_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (divisor_value_p, divisor_size) != 0);

  /* The divisor must have at least two digits, so the single digit case is handled separately. */
  if (divisor_size == sizeof (ecma_bigint_digit_t))
  {
    return ecma_big_uint_div_digit (dividend_value_p, *ECMA_BIGINT_GET_DIGITS (divisor_value_p, 0), is_mod);
  }

  /* D1. [Normalize] */
  ecma_bigint_digit_t divisor_high = ECMA_BIGINT_GET_LAST_DIGIT (divisor_value_p, divisor_size);
  ecma_bigint_digit_t shift_left = ecma_big_uint_count_leading_zero (divisor_high);
  ecma_bigint_digit_t *buffer_p = ecma_big_uint_div_shift_left (dividend_value_p, shift_left, true);
  ecma_bigint_digit_t *dividend_end_p;
  ecma_bigint_digit_t *dividend_p;
  ecma_bigint_digit_t *divisor_end_p;
  ecma_bigint_digit_t divisor_low;

  if (JERRY_UNLIKELY (buffer_p == NULL))
  {
    return NULL;
  }

  uint32_t dividend_size = ECMA_BIGINT_GET_SIZE (dividend_value_p);
  ecma_extended_primitive_t *result_p = NULL;
  ecma_bigint_digit_t *divisor_p;

  JERRY_ASSERT (dividend_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (dividend_value_p, dividend_size) != 0);
  JERRY_ASSERT (dividend_size >= divisor_size);

  if (shift_left > 0)
  {
    divisor_p = ecma_big_uint_div_shift_left (divisor_value_p, shift_left, false);

    if (JERRY_UNLIKELY (divisor_p == NULL))
    {
      goto error;
    }
  }
  else
  {
    divisor_p = ECMA_BIGINT_GET_DIGITS (divisor_value_p, 0);
  }

  dividend_end_p = (ecma_bigint_digit_t *) (((uint8_t *) buffer_p) + dividend_size);
  dividend_p = (ecma_bigint_digit_t *) (((uint8_t *) dividend_end_p) - divisor_size);
  divisor_end_p = (ecma_bigint_digit_t *) (((uint8_t *) divisor_p) + divisor_size);
  divisor_low = divisor_end_p[-2];

  divisor_high = divisor_end_p[-1];
  JERRY_ASSERT ((divisor_high & (((ecma_bigint_digit_t) 1) << (8 * sizeof (ecma_bigint_digit_t) - 1))) != 0);

  do
  {
    /* D3. [Calculate Q′] */
    ecma_bigint_digit_t result_div;

    /* This do-while(false) statement allows local declarations and early exit. */
    do
    {
      ecma_bigint_digit_t result_mod;

      if (dividend_end_p[0] < divisor_high)
      {
        ecma_bigint_two_digits_t dividend = dividend_end_p[-1] | ECMA_BIGINT_HIGH_DIGIT (dividend_end_p[0]);
        result_div = (ecma_bigint_digit_t) (dividend / divisor_high);
        result_mod = (ecma_bigint_digit_t) (dividend % divisor_high);
      }
      else
      {
        JERRY_ASSERT (dividend_end_p[0] == divisor_high && dividend_end_p[-1] < divisor_high);

        result_div = ~((ecma_bigint_digit_t) 0);
        result_mod = dividend_end_p[-1] + divisor_high;

        if (result_mod < divisor_high)
        {
          break;
        }
      }

      ecma_bigint_two_digits_t low_digits = ((ecma_bigint_two_digits_t) result_div) * divisor_low;

      while (low_digits > (ECMA_BIGINT_HIGH_DIGIT (result_mod) | divisor_low))
      {
        result_div--;
        result_mod += divisor_high;

        /* If result_mod becomes a two digit long number, the condition of the loop must be true,
         * so the loop can be aborted. This loop stops after maximum of two iterations, since
         * the highest bit of divisor_high is set. */
        if (result_mod < divisor_high)
        {
          break;
        }

        /* Subtraction is faster than recomputing result_div * divisor_low. */
        low_digits -= divisor_low;
      }
    } while (false);

    /* D4. [Multiply and subtract] */
    ecma_bigint_digit_t *destination_p = dividend_p;
    ecma_bigint_digit_t *source_p = divisor_p;
    ecma_bigint_digit_t carry = 0;

    do
    {
      ecma_bigint_two_digits_t multiply_result = ((ecma_bigint_two_digits_t) (*source_p++)) * result_div;
      ecma_bigint_digit_t multiply_result_low, new_carry;
      ecma_bigint_digit_t value = *destination_p;

      /* The new carry never overflows. See the comment in ecma_big_uint_mul. */
      new_carry = (ecma_bigint_digit_t) (multiply_result >> (8 * sizeof (ecma_bigint_digit_t)));
      multiply_result_low = (ecma_bigint_digit_t) multiply_result;

      if (value < multiply_result_low)
      {
        new_carry++;
      }

      value -= multiply_result_low;

      if (value < carry)
      {
        new_carry++;
      }

      *destination_p++ = value - carry;
      carry = new_carry;
    } while (source_p < divisor_end_p);

    bool negative_result = *destination_p < carry;
    *destination_p -= carry;

    if (negative_result)
    {
      /* D6. [Add back] */
      result_div--;

      destination_p = dividend_p;
      source_p = divisor_p;
      carry = 0;

      do
      {
        ecma_bigint_digit_t left = *destination_p;

        if (carry == 0 || left != ~(ecma_bigint_digit_t) 0)
        {
          left += carry;
          carry = 0;
        }
        else
        {
          left = 0;
          carry = 1;
        }

        ecma_bigint_digit_t right = *source_p++;
        left += right;

        if (left < right)
        {
          JERRY_ASSERT (carry == 0);
          carry = 1;
        }

        *destination_p++ = left;
      } while (source_p < divisor_end_p);
    }

    *dividend_end_p = result_div;

    dividend_p--;
    dividend_end_p--;
  } while (dividend_p >= buffer_p);

  ecma_bigint_digit_t *source_p;
  ecma_bigint_digit_t *source_end_p;

  if (is_mod)
  {
    source_p = buffer_p;
    source_end_p = dividend_end_p;

    while (source_end_p > source_p && *source_end_p == 0)
    {
      source_end_p--;
    }

    if ((*source_end_p >> shift_left) != 0)
    {
      source_end_p++;
      /* This is required to reset carry below. */
      *source_end_p = 0;
    }
  }
  else
  {
    source_p = dividend_end_p + 1;
    source_end_p = (ecma_bigint_digit_t *) (((uint8_t *) buffer_p) + dividend_size);

    if (*source_end_p != 0)
    {
      source_end_p++;
    }
  }

  result_p = ECMA_BIGINT_POINTER_TO_ZERO;

  if (source_p < source_end_p)
  {
    result_p = ecma_bigint_create ((uint32_t) ((uint8_t *) source_end_p - (uint8_t *) source_p));

    if (result_p != NULL)
    {
      ecma_bigint_digit_t *destination_p = ECMA_BIGINT_GET_DIGITS (result_p, 0);

      if (is_mod && shift_left > 0)
      {
        ecma_bigint_digit_t shift_right = shift_left;

        shift_left = (ecma_bigint_digit_t) (8 * (sizeof (ecma_bigint_digit_t)) - shift_left);
        destination_p += source_end_p - source_p;

        ecma_bigint_digit_t carry = *source_end_p << shift_left;

        do
        {
          ecma_bigint_digit_t value = *(--source_end_p);

          *(--destination_p) = (value >> shift_right) | carry;
          carry = value << shift_left;
        } while (source_end_p > source_p);
      }
      else
      {
        memcpy (destination_p, source_p, (size_t) ((uint8_t *) source_end_p - (uint8_t *) source_p));
      }
    }
  }

error:
  jmem_heap_free_block (buffer_p, dividend_size + sizeof (ecma_bigint_digit_t));

  if (shift_left > 0 && divisor_p != NULL)
  {
    jmem_heap_free_block (divisor_p, divisor_size);
  }

  return result_p;
} /* ecma_big_uint_div_mod */

/**
 * Shift left BigUInt values by an uint32 value
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_shift_left (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                          uint32_t right_value) /**< shift value */
{
  JERRY_ASSERT (right_value > 0);

  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);

  uint32_t zero_size = (right_value >> ECMA_BIGINT_DIGIT_SHIFT) * (uint32_t) sizeof (ecma_bigint_digit_t);
  uint32_t result_size = left_size + zero_size;

  uint32_t shift_left = right_value & ((1 << ECMA_BIGINT_DIGIT_SHIFT) - 1);
  uint32_t shift_right = (1 << ECMA_BIGINT_DIGIT_SHIFT) - shift_left;

  if (shift_left > 0 && (ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) >> shift_right) != 0)
  {
    result_size += (uint32_t) sizeof (ecma_bigint_digit_t);
  }

  if (result_size > ECMA_BIGINT_MAX_SIZE)
  {
    return NULL;
  }

  ecma_extended_primitive_t *result_value_p = ecma_bigint_create (result_size);

  if (JERRY_UNLIKELY (result_value_p == NULL))
  {
    return NULL;
  }

  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
  ecma_bigint_digit_t *result_p = ECMA_BIGINT_GET_DIGITS (result_value_p, 0);

  if (zero_size > 0)
  {
    memset (result_p, 0, zero_size);
    result_p = (ecma_bigint_digit_t *) (((uint8_t *) result_p) + zero_size);
  }

  if (shift_left == 0)
  {
    /* Shift by full digits. */
    memcpy (result_p, left_p, left_size);
    return result_value_p;
  }

  ecma_bigint_digit_t *left_end_p = ECMA_BIGINT_GET_DIGITS (left_value_p, left_size);
  ecma_bigint_digit_t carry = 0;

  do
  {
    ecma_bigint_digit_t value = *left_p++;

    *result_p++ = (value << shift_left) | carry;
    carry = value >> shift_right;
  } while (left_p < left_end_p);

  if (carry > 0)
  {
    *result_p = carry;
  }

  return result_value_p;
} /* ecma_big_uint_shift_left */

/**
 * Shift right BigUInt values by an uint32 value
 *
 * @return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_shift_right (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                           uint32_t right_value, /**< shift value */
                           bool increase_result) /**< increase result */
{
  JERRY_ASSERT (right_value > 0);

  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);

  uint32_t crop_size = (right_value >> ECMA_BIGINT_DIGIT_SHIFT) * (uint32_t) sizeof (ecma_bigint_digit_t);

  uint32_t shift_right = right_value & ((1 << ECMA_BIGINT_DIGIT_SHIFT) - 1);
  uint32_t shift_left = (1 << ECMA_BIGINT_DIGIT_SHIFT) - shift_right;
  ecma_bigint_digit_t carry = 0;

  if (shift_right > 0 && (ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) >> shift_right) == 0)
  {
    carry = ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) << shift_left;
    left_size -= (uint32_t) sizeof (ecma_bigint_digit_t);
  }

  if (left_size <= crop_size)
  {
    if (JERRY_LIKELY (!increase_result))
    {
      return ECMA_BIGINT_POINTER_TO_ZERO;
    }

    ecma_extended_primitive_t *result_value_p = ecma_bigint_create (sizeof (ecma_bigint_digit_t));
    if (result_value_p != NULL)
    {
      *ECMA_BIGINT_GET_DIGITS (result_value_p, 0) = 1;
    }
    return result_value_p;
  }

  if (JERRY_UNLIKELY (increase_result)
      && (shift_right == 0 || (*ECMA_BIGINT_GET_DIGITS (left_value_p, crop_size) << shift_left) == 0))
  {
    ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
    ecma_bigint_digit_t *left_end_p = ECMA_BIGINT_GET_DIGITS (left_value_p, crop_size);

    while (left_p < left_end_p)
    {
      if (*left_p != 0)
      {
        break;
      }
      left_p++;
    }

    if (left_p == left_end_p)
    {
      increase_result = false;
    }
  }

  uint32_t size = left_size - crop_size;
  ecma_extended_primitive_t *result_value_p = ecma_bigint_create (size);

  if (JERRY_UNLIKELY (result_value_p == NULL))
  {
    return NULL;
  }

  if (shift_right == 0)
  {
    memcpy (ECMA_BIGINT_GET_DIGITS (result_value_p, 0), ECMA_BIGINT_GET_DIGITS (left_value_p, crop_size), size);

    if (JERRY_LIKELY (!increase_result))
    {
      return result_value_p;
    }
    return ecma_big_uint_increase_result (result_value_p);
  }

  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, left_size);
  ecma_bigint_digit_t *result_p = ECMA_BIGINT_GET_DIGITS (result_value_p, size);
  ecma_bigint_digit_t *end_p = ECMA_BIGINT_GET_DIGITS (result_value_p, 0);

  do
  {
    ecma_bigint_digit_t value = *(--left_p);

    *(--result_p) = (value >> shift_right) | carry;
    carry = value << shift_left;
  } while (result_p > end_p);

  if (JERRY_LIKELY (!increase_result))
  {
    return result_value_p;
  }
  return ecma_big_uint_increase_result (result_value_p);
} /* ecma_big_uint_shift_right */

/**
 * Compute the left value raised to the power of right value
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_pow (ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                   uint32_t right_value) /**< power value */
{
  ecma_extended_primitive_t *result_p = ECMA_BIGINT_NUMBER_IS_ODD (right_value) ? left_value_p : NULL;
  ecma_extended_primitive_t *square_p = left_value_p;

  JERRY_ASSERT (right_value >= 2);

  while (true)
  {
    ecma_extended_primitive_t *new_square_p = ecma_big_uint_mul (square_p, square_p);

    if (JERRY_UNLIKELY (new_square_p == NULL))
    {
      if (result_p != NULL && result_p != left_value_p)
      {
        ecma_deref_bigint (result_p);
      }
      result_p = NULL;
      break;
    }

    if (square_p != left_value_p)
    {
      ecma_deref_bigint (square_p);
    }

    square_p = new_square_p;
    right_value >>= 1;

    if (ECMA_BIGINT_NUMBER_IS_ODD (right_value))
    {
      if (result_p != NULL)
      {
        ecma_extended_primitive_t *new_result_p = ecma_big_uint_mul (square_p, result_p);

        if (result_p != left_value_p)
        {
          ecma_deref_bigint (result_p);
        }

        result_p = new_result_p;
      }
      else
      {
        ecma_ref_extended_primitive (square_p);
        result_p = square_p;
      }

      if (JERRY_UNLIKELY (result_p == NULL) || right_value == 1)
      {
        break;
      }
    }
  }

  if (square_p != left_value_p)
  {
    ecma_deref_bigint (square_p);
  }

  return result_p;
} /* ecma_big_uint_pow */

/**
 * Perform bitwise operations on two BigUInt numbers
 *
 * return new BigUInt value, NULL on error
 */
ecma_extended_primitive_t *
ecma_big_uint_bitwise_op (uint32_t operation_and_options, /**< bitwise operation type and options */
                          ecma_extended_primitive_t *left_value_p, /**< left BigUInt value */
                          ecma_extended_primitive_t *right_value_p) /**< right BigUInt value */
{
  uint32_t left_size = ECMA_BIGINT_GET_SIZE (left_value_p);
  uint32_t right_size = ECMA_BIGINT_GET_SIZE (right_value_p);

  JERRY_ASSERT (left_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (left_value_p, left_size) != 0);
  JERRY_ASSERT (right_size > 0 && ECMA_BIGINT_GET_LAST_DIGIT (right_value_p, right_size) != 0);

  uint32_t operation_type = ECMA_BIGINT_BITWISE_GET_OPERATION_TYPE (operation_and_options);

  switch (operation_type)
  {
    case ECMA_BIG_UINT_BITWISE_AND:
    {
      if (left_size > right_size)
      {
        left_size = right_size;
        break;
      }
      /* FALLTHRU */
    }
    case ECMA_BIG_UINT_BITWISE_AND_NOT:
    {
      if (right_size > left_size)
      {
        right_size = left_size;
      }
      break;
    }
    default:
    {
      JERRY_ASSERT (operation_type == ECMA_BIG_UINT_BITWISE_OR || operation_type == ECMA_BIG_UINT_BITWISE_XOR);

      if (right_size <= left_size)
      {
        break;
      }

      /* Swap values. */
      ecma_extended_primitive_t *tmp_value_p = left_value_p;
      left_value_p = right_value_p;
      right_value_p = tmp_value_p;

      uint32_t tmp_size = left_size;
      left_size = right_size;
      right_size = tmp_size;

      uint32_t decrease_opts = (operation_and_options & ECMA_BIG_UINT_BITWISE_DECREASE_BOTH);

      /* When exactly one bit is set, invert both bits. */
      if (decrease_opts >= ECMA_BIG_UINT_BITWISE_DECREASE_LEFT && decrease_opts <= ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT)
      {
        operation_and_options ^= ECMA_BIG_UINT_BITWISE_DECREASE_BOTH;
      }
      break;
    }
  }

  ecma_extended_primitive_t *result_value_p = ecma_bigint_create (left_size);

  if (JERRY_UNLIKELY (result_value_p == NULL))
  {
    return NULL;
  }

  ecma_bigint_digit_t *left_p = ECMA_BIGINT_GET_DIGITS (left_value_p, 0);
  ecma_bigint_digit_t *right_p = ECMA_BIGINT_GET_DIGITS (right_value_p, 0);
  ecma_bigint_digit_t *result_p = ECMA_BIGINT_GET_DIGITS (result_value_p, 0);
  ecma_bigint_digit_t *result_end_p = ECMA_BIGINT_GET_DIGITS (result_value_p, right_size);

  if (!(operation_and_options & ECMA_BIG_UINT_BITWISE_DECREASE_BOTH))
  {
    JERRY_ASSERT (!(operation_and_options & ECMA_BIG_UINT_BITWISE_INCREASE_RESULT));

    if (operation_type == ECMA_BIG_UINT_BITWISE_AND)
    {
      do
      {
        *result_p++ = *left_p++ & *right_p++;
      } while (result_p < result_end_p);

      if (result_p[-1] == 0)
      {
        return ecma_big_uint_normalize_result (result_value_p, result_p);
      }
      return result_value_p;
    }

    if (operation_type == ECMA_BIG_UINT_BITWISE_OR)
    {
      do
      {
        *result_p++ = *left_p++ | *right_p++;
      } while (result_p < result_end_p);

      if (left_size > right_size)
      {
        memcpy (result_p, left_p, left_size - right_size);
      }
      return result_value_p;
    }

    JERRY_ASSERT (operation_type == ECMA_BIG_UINT_BITWISE_XOR);

    do
    {
      *result_p++ = *left_p++ ^ *right_p++;
    } while (result_p < result_end_p);

    if (left_size > right_size)
    {
      memcpy (result_p, left_p, left_size - right_size);
      return result_value_p;
    }

    if (result_p[-1] == 0)
    {
      return ecma_big_uint_normalize_result (result_value_p, result_p);
    }
    return result_value_p;
  }

  uint32_t left_carry = 0, right_carry = 0;

  if (operation_and_options & ECMA_BIG_UINT_BITWISE_DECREASE_LEFT)
  {
    left_carry = 1;
  }

  if (operation_and_options & ECMA_BIG_UINT_BITWISE_DECREASE_RIGHT)
  {
    right_carry = 1;
  }

  do
  {
    ecma_bigint_digit_t left = (*left_p++) - left_carry;

    if (left != ~((ecma_bigint_digit_t) 0))
    {
      left_carry = 0;
    }

    ecma_bigint_digit_t right = (*right_p++) - right_carry;

    if (right != ~((ecma_bigint_digit_t) 0))
    {
      right_carry = 0;
    }

    switch (operation_type)
    {
      case ECMA_BIG_UINT_BITWISE_AND:
      {
        *result_p++ = left & right;
        break;
      }
      case ECMA_BIG_UINT_BITWISE_OR:
      {
        *result_p++ = left | right;
        break;
      }
      case ECMA_BIG_UINT_BITWISE_XOR:
      {
        *result_p++ = left ^ right;
        break;
      }
      default:
      {
        JERRY_ASSERT (operation_type == ECMA_BIG_UINT_BITWISE_AND_NOT);
        *result_p++ = left & ~right;
        break;
      }
    }
  } while (result_p < result_end_p);

  if (operation_type != ECMA_BIG_UINT_BITWISE_AND)
  {
    result_end_p = ECMA_BIGINT_GET_DIGITS (result_value_p, left_size);

    if (left_carry > 0)
    {
      while (*left_p == 0)
      {
        *result_p++ = ~((ecma_bigint_digit_t) 0);
        left_p++;

        JERRY_ASSERT (result_p < result_end_p);
      }

      *result_p++ = *left_p++ - 1;
    }

    if (result_p < result_end_p)
    {
      memcpy (result_p, left_p, (size_t) ((uint8_t *) result_end_p - (uint8_t *) result_p));

      if (operation_and_options & ECMA_BIG_UINT_BITWISE_INCREASE_RESULT)
      {
        return ecma_big_uint_increase_result (result_value_p);
      }
      return result_value_p;
    }
  }

  if (operation_and_options & ECMA_BIG_UINT_BITWISE_INCREASE_RESULT)
  {
    return ecma_big_uint_increase_result (result_value_p);
  }

  if (result_p[-1] == 0)
  {
    return ecma_big_uint_normalize_result (result_value_p, result_p);
  }
  return result_value_p;
} /* ecma_big_uint_bitwise_op */

#endif /* JERRY_BUILTIN_BIGINT */
