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

#ifndef ECMA_BUILTINS_INTERNAL_H
#define ECMA_BUILTINS_INTERNAL_H

#ifndef ECMA_BUILTINS_INTERNAL
#error "!ECMA_BUILTINS_INTERNAL"
#endif /* !ECMA_BUILTINS_INTERNAL */

#include "ecma-builtins.h"
#include "ecma-globals.h"

/**
 * Type of built-in properties.
 */
typedef enum
{
  ECMA_BUILTIN_PROPERTY_SIMPLE, /**< simple value property */
  ECMA_BUILTIN_PROPERTY_NUMBER, /**< number value property */
  ECMA_BUILTIN_PROPERTY_STRING, /**< string value property */
  ECMA_BUILTIN_PROPERTY_SYMBOL, /**< symbol value property */
  ECMA_BUILTIN_PROPERTY_INTRINSIC_PROPERTY, /**< intrinsic routine property */
  ECMA_BUILTIN_PROPERTY_ACCESSOR_BUILTIN_FUNCTION, /**< full accessor property with builtin function object
                                                        getter/setter pair */
  ECMA_BUILTIN_PROPERTY_OBJECT, /**< builtin object property */
  ECMA_BUILTIN_PROPERTY_ROUTINE, /**< routine property */
  ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_WRITE, /**< full accessor property */
  ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_ONLY, /**< read-only accessor property */
  ECMA_BUILTIN_PROPERTY_END, /**< last property */
} ecma_builtin_property_type_t;

/**
 * Type of symbolic built-in number types (starting from 256).
 */
typedef enum
{
  ECMA_BUILTIN_NUMBER_MAX = 256, /**< value of ECMA_NUMBER_MAX_VALUE */
  ECMA_BUILTIN_NUMBER_MIN, /**< value of ECMA_NUMBER_MIN_VALUE */
  ECMA_BUILTIN_NUMBER_EPSILON, /**< value of ECMA_NUMBER_EPSILON */
  ECMA_BUILTIN_NUMBER_MAX_SAFE_INTEGER, /**< value of ECMA_NUMBER_MAX_SAFE_INTEGER */
  ECMA_BUILTIN_NUMBER_MIN_SAFE_INTEGER, /**< value of ECMA_NUMBER_MIN_SAFE_INTEGER */
  ECMA_BUILTIN_NUMBER_E, /**< value of ECMA_NUMBER_E */
  ECMA_BUILTIN_NUMBER_PI, /**< value of ECMA_NUMBER_PI */
  ECMA_BUILTIN_NUMBER_LN10, /**< value of ECMA_NUMBER_LN10 */
  ECMA_BUILTIN_NUMBER_LN2, /**< value of ECMA_NUMBER_LN2 */
  ECMA_BUILTIN_NUMBER_LOG2E, /**< value of ECMA_NUMBER_LOG2E */
  ECMA_BUILTIN_NUMBER_LOG10E, /**< value of ECMA_NUMBER_LOG10E */
  ECMA_BUILTIN_NUMBER_SQRT2, /**< value of ECMA_NUMBER_SQRT2 */
  ECMA_BUILTIN_NUMBER_SQRT_1_2, /**< value of ECMA_NUMBER_SQRT_1_2 */
  ECMA_BUILTIN_NUMBER_NAN, /**< result of ecma_number_make_nan () */
  ECMA_BUILTIN_NUMBER_POSITIVE_INFINITY, /**< result of ecma_number_make_infinity (false) */
  ECMA_BUILTIN_NUMBER_NEGATIVE_INFINITY, /**< result of ecma_number_make_infinity (true) */
} ecma_builtin_number_type_t;

/**
 * Description of built-in properties.
 */
typedef struct
{
  uint16_t magic_string_id; /**< name of the property */
  uint8_t type; /**< type of the property */
  uint8_t attributes; /**< attributes of the property */
  uint16_t value; /**< value of the property */
} ecma_builtin_property_descriptor_t;

#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name)  \
  extern const ecma_builtin_property_descriptor_t ecma_builtin_##lowercase_name##_property_descriptor_list[]; \
  ecma_value_t ecma_builtin_##lowercase_name##_dispatch_call (const ecma_value_t *, uint32_t);                \
  ecma_value_t ecma_builtin_##lowercase_name##_dispatch_construct (const ecma_value_t *, uint32_t);           \
  ecma_value_t ecma_builtin_##lowercase_name##_dispatch_routine (uint8_t builtin_routine_id,                  \
                                                                 ecma_value_t this_arg_value,                 \
                                                                 const ecma_value_t[],                        \
                                                                 uint32_t);
#define BUILTIN(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name)          \
  extern const ecma_builtin_property_descriptor_t ecma_builtin_##lowercase_name##_property_descriptor_list[]; \
  ecma_value_t ecma_builtin_##lowercase_name##_dispatch_routine (uint8_t builtin_routine_id,                  \
                                                                 ecma_value_t this_arg_value,                 \
                                                                 const ecma_value_t[],                        \
                                                                 uint32_t);
#include "ecma-builtins.inc.h"

#undef BUILTIN_ROUTINE
#undef BUILTIN

#endif /* !ECMA_BUILTINS_INTERNAL_H */
