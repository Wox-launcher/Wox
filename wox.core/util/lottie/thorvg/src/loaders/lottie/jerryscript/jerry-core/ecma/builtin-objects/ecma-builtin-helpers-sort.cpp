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

#include "ecma-builtin-helpers.h"
#include "ecma-globals.h"
#include "ecma-helpers-number.h"

/**
 * Function used to merge two arrays for merge sort.
 * Arrays are stored as below:
 * First  -> source_array_p [left_idx : right_idx - 1]
 * Second -> source_array_p [right_idx : end_idx - 1]
 * Output -> output_array_p
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_helper_array_merge_sort_bottom_up (ecma_value_t *source_array_p, /**< arrays to merge */
                                                uint32_t left_idx, /**< first array begin */
                                                uint32_t right_idx, /**< first array end */
                                                uint32_t end_idx, /**< second array end */
                                                ecma_value_t *output_array_p, /**< output array */
                                                ecma_value_t compare_func, /**< compare function */
                                                const ecma_builtin_helper_sort_compare_fn_t sort_cb, /**< sorting cb */
                                                ecma_object_t *array_buffer_p) /* array_buffer_p */
{
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;
  uint32_t i = left_idx, j = right_idx;

  for (uint32_t k = left_idx; k < end_idx; k++)
  {
    ecma_value_t compare_value = ecma_make_number_value (ECMA_NUMBER_ZERO);

    if (i < right_idx && j < end_idx)
    {
      compare_value = sort_cb (source_array_p[i], source_array_p[j], compare_func, array_buffer_p);
      if (ECMA_IS_VALUE_ERROR (compare_value))
      {
        ret_value = ECMA_VALUE_ERROR;
        break;
      }
    }

    if (i < right_idx && ecma_get_number_from_value (compare_value) <= ECMA_NUMBER_ZERO)
    {
      output_array_p[k] = source_array_p[i];
      i++;
    }
    else
    {
      output_array_p[k] = source_array_p[j];
      j++;
    }
    ecma_free_value (compare_value);
  }

  return ret_value;
} /* ecma_builtin_helper_array_merge_sort_bottom_up */

/**
 * Mergesort function
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_helper_array_merge_sort_helper (ecma_value_t *array_p, /**< array to sort */
                                             uint32_t length, /**< length */
                                             ecma_value_t compare_func, /**< compare function */
                                             const ecma_builtin_helper_sort_compare_fn_t sort_cb, /**< sorting cb */
                                             ecma_object_t *array_buffer_p) /**< arrayBuffer */
{
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;
  JMEM_DEFINE_LOCAL_ARRAY (dest_array_p, length, ecma_value_t);

  ecma_value_t *temp_p;
  ecma_value_t *base_array_p = array_p;
  uint32_t r, e;

  for (uint32_t w = 1; w < length; w = 2 * w)
  {
    for (uint32_t i = 0; i < length; i = i + 2 * w)
    {
      // End of first array
      r = i + w;
      if (r > length)
      {
        r = length;
      }

      // End of second array
      e = i + 2 * w;
      if (e > length)
      {
        e = length;
      }

      // Merge two arrays
      ret_value = ecma_builtin_helper_array_merge_sort_bottom_up (array_p,
                                                                  i,
                                                                  r,
                                                                  e,
                                                                  dest_array_p,
                                                                  compare_func,
                                                                  sort_cb,
                                                                  array_buffer_p);
      if (ECMA_IS_VALUE_ERROR (ret_value))
      {
        break;
      }
    }

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      break;
    }

    // Swap arrays
    temp_p = dest_array_p;
    dest_array_p = array_p;
    array_p = temp_p;
  }

  // Sorted array is in dest_array_p - there was uneven number of arrays swaps
  if (dest_array_p == base_array_p)
  {
    uint32_t index = 0;
    temp_p = dest_array_p;
    dest_array_p = array_p;
    array_p = temp_p;

    while (index < length)
    {
      array_p[index] = dest_array_p[index];
      index++;
    }
    JERRY_ASSERT (index == length);
  }

  JMEM_FINALIZE_LOCAL_ARRAY (dest_array_p);

  return ret_value;
} /* ecma_builtin_helper_array_merge_sort_helper */
