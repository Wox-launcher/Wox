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

#ifndef ECMA_CONVERSION_H
#define ECMA_CONVERSION_H

#include "ecma-builtins.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaconversion ECMA conversion routines
 * @{
 */

/**
 * Second argument of 'ToPrimitive' operation that is a hint,
 * specifying the preferred type of conversion result.
 */
typedef enum
{
  ECMA_PREFERRED_TYPE_NO = 0, /**< no preferred type is specified */
  ECMA_PREFERRED_TYPE_NUMBER, /**< Number */
  ECMA_PREFERRED_TYPE_STRING /**< String */
} ecma_preferred_type_hint_t;

/**
 * Option bits for ecma_op_to_numeric.
 */
typedef enum
{
  ECMA_TO_NUMERIC_NO_OPTS = 0, /**< no options (same as toNumber operation) */
  ECMA_TO_NUMERIC_ALLOW_BIGINT = (1 << 0), /**< allow BigInt values (ignored if BigInts are disabled) */
} ecma_to_numeric_options_t;

bool ecma_op_require_object_coercible (ecma_value_t value);
bool ecma_op_same_value (ecma_value_t x, ecma_value_t y);
#if JERRY_BUILTIN_CONTAINER
bool ecma_op_same_value_zero (ecma_value_t x, ecma_value_t y, bool strict_equality);
#endif /* JERRY_BUILTIN_CONTAINER */
ecma_value_t ecma_op_to_primitive (ecma_value_t value, ecma_preferred_type_hint_t preferred_type);
bool ecma_op_to_boolean (ecma_value_t value);
ecma_value_t ecma_op_to_number (ecma_value_t value, ecma_number_t *number_p);
ecma_value_t ecma_op_to_numeric (ecma_value_t value, ecma_number_t *number_p, ecma_to_numeric_options_t options);
ecma_string_t *ecma_op_to_string (ecma_value_t value);
ecma_string_t *ecma_op_to_property_key (ecma_value_t value);
ecma_value_t ecma_op_to_object (ecma_value_t value);
bool ecma_op_is_integer (ecma_number_t value);
ecma_value_t ecma_op_to_integer (ecma_value_t value, ecma_number_t *number_p);
ecma_value_t ecma_op_to_length (ecma_value_t value, ecma_length_t *length);
ecma_value_t ecma_op_to_index (ecma_value_t value, ecma_number_t *index);
ecma_collection_t *ecma_op_create_list_from_array_like (ecma_value_t arr, bool prop_names_only);

ecma_object_t *ecma_op_from_property_descriptor (const ecma_property_descriptor_t *src_prop_desc_p);
ecma_value_t ecma_op_to_property_descriptor (ecma_value_t obj_value, ecma_property_descriptor_t *out_prop_desc_p);

/**
 * @}
 * @}
 */

#endif /* !ECMA_CONVERSION_H */
