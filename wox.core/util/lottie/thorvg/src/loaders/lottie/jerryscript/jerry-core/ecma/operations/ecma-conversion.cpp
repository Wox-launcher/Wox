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

/**
 * Implementation of ECMA-defined conversion routines
 */

#include "ecma-conversion.h"

#include <math.h>

#include "ecma-alloc.h"
#include "ecma-bigint-object.h"
#include "ecma-bigint.h"
#include "ecma-boolean-object.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers-number.h"
#include "ecma-helpers.h"
#include "ecma-number-object.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"
#include "ecma-string-object.h"
#include "ecma-symbol-object.h"

#include "jrt-libc-includes.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaconversion ECMA conversion routines
 * @{
 */

/**
 * RequireObjectCoercible operation.
 *
 * See also:
 *          ECMA-262 v11, 7.2.1
 *
 * @return true - if the value can be coerced to object without any exceptions
 *         false - otherwise
 */
bool
ecma_op_require_object_coercible (ecma_value_t value) /**< ecma value */
{
  ecma_check_value_type_is_spec_defined (value);

  if (ecma_is_value_undefined (value) || ecma_is_value_null (value))
  {
    ecma_raise_type_error (ECMA_ERR_ARGUMENT_CANNOT_CONVERT_TO_OBJECT);
    return false;
  }

  return true;
} /* ecma_op_require_object_coercible */

/**
 * SameValue operation.
 *
 * See also:
 *          ECMA-262 v5, 9.12
 *
 * @return true - if the value are same according to ECMA-defined SameValue algorithm,
 *         false - otherwise
 */
bool
ecma_op_same_value (ecma_value_t x, /**< ecma value */
                    ecma_value_t y) /**< ecma value */
{
  if (x == y)
  {
    return true;
  }

  ecma_type_t type_of_x = ecma_get_value_type_field (x);

  if (type_of_x != ecma_get_value_type_field (y) || type_of_x == ECMA_TYPE_DIRECT)
  {
    return false;
  }

  if (ecma_is_value_number (x))
  {
    ecma_number_t x_num = ecma_get_number_from_value (x);
    ecma_number_t y_num = ecma_get_number_from_value (y);

    bool is_x_nan = ecma_number_is_nan (x_num);
    bool is_y_nan = ecma_number_is_nan (y_num);

    if (is_x_nan || is_y_nan)
    {
      return is_x_nan && is_y_nan;
    }

    if (ecma_number_is_zero (x_num) && ecma_number_is_zero (y_num)
        && ecma_number_is_negative (x_num) != ecma_number_is_negative (y_num))
    {
      return false;
    }

    return (x_num == y_num);
  }

  if (ecma_is_value_string (x))
  {
    ecma_string_t *x_str_p = ecma_get_string_from_value (x);
    ecma_string_t *y_str_p = ecma_get_string_from_value (y);

    return ecma_compare_ecma_strings (x_str_p, y_str_p);
  }

#if JERRY_BUILTIN_BIGINT
  if (ecma_is_value_bigint (x))
  {
    return (ecma_is_value_bigint (y) && ecma_bigint_compare_to_bigint (x, y) == 0);
  }
#endif /* JERRY_BUILTIN_BIGINT */

  JERRY_ASSERT (ecma_is_value_object (x) || ecma_is_value_symbol (x));

  return false;
} /* ecma_op_same_value */

#if JERRY_BUILTIN_CONTAINER
/**
 * SameValueZero operation.
 *
 * See also:
 *          ECMA-262 v6, 7.2.10
 *
 * @return true - if the value are same according to ECMA-defined SameValueZero algorithm,
 *         false - otherwise
 */
bool
ecma_op_same_value_zero (ecma_value_t x, /**< ecma value */
                         ecma_value_t y, /**< ecma value */
                         bool strict_equality) /**< strict equality */
{
  if (ecma_is_value_number (x) && ecma_is_value_number (y))
  {
    ecma_number_t x_num = ecma_get_number_from_value (x);
    ecma_number_t y_num = ecma_get_number_from_value (y);

    bool is_x_nan = ecma_number_is_nan (x_num);
    bool is_y_nan = ecma_number_is_nan (y_num);

    if (strict_equality && is_x_nan && is_y_nan)
    {
      return false;
    }

    if (is_x_nan || is_y_nan)
    {
      return (is_x_nan && is_y_nan);
    }

    if (ecma_number_is_zero (x_num) && ecma_number_is_zero (y_num)
        && ecma_number_is_negative (x_num) != ecma_number_is_negative (y_num))
    {
      return true;
    }

    return (x_num == y_num);
  }

  return ecma_op_same_value (x, y);
} /* ecma_op_same_value_zero */
#endif /* JERRY_BUILTIN_CONTAINER */

/**
 * ToPrimitive operation.
 *
 * See also:
 *          ECMA-262 v5, 9.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_to_primitive (ecma_value_t value, /**< ecma value */
                      ecma_preferred_type_hint_t preferred_type) /**< preferred type hint */
{
  ecma_check_value_type_is_spec_defined (value);

  if (ecma_is_value_object (value))
  {
    ecma_object_t *obj_p = ecma_get_object_from_value (value);

    return ecma_op_object_default_value (obj_p, preferred_type);
  }
  else
  {
    return ecma_copy_value (value);
  }
} /* ecma_op_to_primitive */

/**
 * ToBoolean operation. Cannot throw an exception.
 *
 * See also:
 *          ECMA-262 v5, 9.2
 *
 * @return true - if the logical value is true
 *         false - otherwise
 */
bool
ecma_op_to_boolean (ecma_value_t value) /**< ecma value */
{
  ecma_check_value_type_is_spec_defined (value);

  if (ecma_is_value_simple (value))
  {
    JERRY_ASSERT (ecma_is_value_boolean (value) || ecma_is_value_undefined (value) || ecma_is_value_null (value));

    return ecma_is_value_true (value);
  }

  if (ecma_is_value_integer_number (value))
  {
    return (value != ecma_make_integer_value (0));
  }

  if (ecma_is_value_float_number (value))
  {
    ecma_number_t num = ecma_get_float_from_value (value);

    return (!ecma_number_is_nan (num) && !ecma_number_is_zero (num));
  }

  if (ecma_is_value_string (value))
  {
    ecma_string_t *str_p = ecma_get_string_from_value (value);

    return !ecma_string_is_empty (str_p);
  }

#if JERRY_BUILTIN_BIGINT
  if (ecma_is_value_bigint (value))
  {
    return value != ECMA_BIGINT_ZERO;
  }
#endif /* JERRY_BUILTIN_BIGINT */

  JERRY_ASSERT (ecma_is_value_object (value) || ecma_is_value_symbol (value));

  return true;
} /* ecma_op_to_boolean */

/**
 * ToNumber operation.
 *
 * See also:
 *          ECMA-262 v5, 9.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_to_number (ecma_value_t value, /**< ecma value */
                   ecma_number_t *number_p) /**< [out] ecma number */
{
  return ecma_op_to_numeric (value, number_p, ECMA_TO_NUMERIC_NO_OPTS);
} /* ecma_op_to_number */

/**
 * Helper to get the numeric value of an ecma value
 *
 * See also:
 *          ECMA-262 v11, 7.1.3
 *
 * @return ECMA_VALUE_EMPTY if converted to number, BigInt if
 *         converted to BigInt, and conversion error otherwise
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_to_numeric (ecma_value_t value, /**< ecma value */
                    ecma_number_t *number_p, /**< [out] ecma number */
                    ecma_to_numeric_options_t options) /**< option bits */
{
  JERRY_UNUSED (options);

  if (ecma_is_value_integer_number (value))
  {
    *number_p = (ecma_number_t) ecma_get_integer_from_value (value);
    return ECMA_VALUE_EMPTY;
  }

  if (ecma_is_value_float_number (value))
  {
    *number_p = ecma_get_float_from_value (value);
    return ECMA_VALUE_EMPTY;
  }

  if (ecma_is_value_string (value))
  {
    ecma_string_t *str_p = ecma_get_string_from_value (value);
    *number_p = ecma_string_to_number (str_p);
    return ECMA_VALUE_EMPTY;
  }

  if (ecma_is_value_undefined (value))
  {
    *number_p = ecma_number_make_nan ();
    return ECMA_VALUE_EMPTY;
  }

  if (ecma_is_value_null (value))
  {
    *number_p = 0;
    return ECMA_VALUE_EMPTY;
  }

  if (ecma_is_value_true (value))
  {
    *number_p = 1;
    return ECMA_VALUE_EMPTY;
  }

  if (ecma_is_value_false (value))
  {
    *number_p = 0;
    return ECMA_VALUE_EMPTY;
  }

  if (ecma_is_value_symbol (value))
  {
    return ecma_raise_type_error (ECMA_ERR_CONVERT_SYMBOL_TO_NUMBER);
  }

#if JERRY_BUILTIN_BIGINT
  if (ecma_is_value_bigint (value))
  {
    if (options & ECMA_TO_NUMERIC_ALLOW_BIGINT)
    {
      return ecma_copy_value (value);
    }
    return ecma_raise_type_error (ECMA_ERR_CONVERT_BIGINT_TO_NUMBER);
  }
#endif /* JERRY_BUILTIN_BIGINT */

  JERRY_ASSERT (ecma_is_value_object (value));

  ecma_object_t *object_p = ecma_get_object_from_value (value);

  ecma_value_t def_value = ecma_op_object_default_value (object_p, ECMA_PREFERRED_TYPE_NUMBER);

  if (ECMA_IS_VALUE_ERROR (def_value))
  {
    return def_value;
  }

  ecma_value_t ret_value = ecma_op_to_numeric (def_value, number_p, options);

  ecma_fast_free_value (def_value);

  return ret_value;
} /* ecma_op_to_numeric */

/**
 * ToString operation.
 *
 * See also:
 *          ECMA-262 v5, 9.8
 *
 * @return NULL - if the conversion fails
 *         pointer to the string descriptor - otherwise
 */
ecma_string_t *
ecma_op_to_string (ecma_value_t value) /**< ecma value */
{
  ecma_check_value_type_is_spec_defined (value);

  if (ecma_is_value_string (value))
  {
    ecma_string_t *res_p = ecma_get_string_from_value (value);
    ecma_ref_ecma_string (res_p);
    return res_p;
  }

  if (ecma_is_value_integer_number (value))
  {
    ecma_integer_value_t num = ecma_get_integer_from_value (value);

    if (num < 0)
    {
      return ecma_new_ecma_string_from_number ((ecma_number_t) num);
    }
    else
    {
      return ecma_new_ecma_string_from_uint32 ((uint32_t) num);
    }
  }

  if (ecma_is_value_float_number (value))
  {
    ecma_number_t num = ecma_get_float_from_value (value);
    return ecma_new_ecma_string_from_number (num);
  }

  if (ecma_is_value_undefined (value))
  {
    return ecma_get_magic_string (LIT_MAGIC_STRING_UNDEFINED);
  }

  if (ecma_is_value_null (value))
  {
    return ecma_get_magic_string (LIT_MAGIC_STRING_NULL);
  }

  if (ecma_is_value_true (value))
  {
    return ecma_get_magic_string (LIT_MAGIC_STRING_TRUE);
  }

  if (ecma_is_value_false (value))
  {
    return ecma_get_magic_string (LIT_MAGIC_STRING_FALSE);
  }

  if (ecma_is_value_symbol (value))
  {
    ecma_raise_type_error (ECMA_ERR_CONVERT_SYMBOL_TO_STRING);
    return NULL;
  }

#if JERRY_BUILTIN_BIGINT
  if (ecma_is_value_bigint (value))
  {
    return ecma_bigint_to_string (value, 10);
  }
#endif /* JERRY_BUILTIN_BIGINT */

  JERRY_ASSERT (ecma_is_value_object (value));

  ecma_object_t *obj_p = ecma_get_object_from_value (value);

  ecma_value_t def_value = ecma_op_object_default_value (obj_p, ECMA_PREFERRED_TYPE_STRING);

  if (ECMA_IS_VALUE_ERROR (def_value))
  {
    return NULL;
  }

  ecma_string_t *ret_string_p = ecma_op_to_string (def_value);

  ecma_free_value (def_value);

  return ret_string_p;
} /* ecma_op_to_string */

/**
 * ToPropertyKey operation.
 *
 * See also:
 *   ECMA 262 v6, 7.1.14
 *   ECMA 262 v10, 7.1.14
 *   ECMA 262 v11, 7.1.19
 *
 * @return NULL - if the conversion fails
 *         ecma-string - otherwise
 */
ecma_string_t *
ecma_op_to_property_key (ecma_value_t value) /**< ecma value */
{
  /* Fast path for strings and symbols */
  if (JERRY_LIKELY (ecma_is_value_prop_name (value)))
  {
    ecma_string_t *key_p = ecma_get_prop_name_from_value (value);
    ecma_ref_ecma_string (key_p);
    return key_p;
  }

  ecma_value_t key = ecma_op_to_primitive (value, ECMA_PREFERRED_TYPE_STRING);

  if (ECMA_IS_VALUE_ERROR (key))
  {
    return NULL;
  }

  if (ecma_is_value_symbol (key))
  {
    ecma_string_t *symbol_p = ecma_get_symbol_from_value (key);
    return symbol_p;
  }

  ecma_string_t *result = ecma_op_to_string (key);
  ecma_free_value (key);

  return result;
} /* ecma_op_to_property_key */

/**
 * ToObject operation.
 *
 * See also:
 *          ECMA-262 v5, 9.9
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_to_object (ecma_value_t value) /**< ecma value */
{
  ecma_check_value_type_is_spec_defined (value);
  ecma_builtin_id_t proto_id = ECMA_BUILTIN_ID_OBJECT_PROTOTYPE;
  uint8_t class_type;

  if (ecma_is_value_number (value))
  {
#if JERRY_BUILTIN_NUMBER
    proto_id = ECMA_BUILTIN_ID_NUMBER_PROTOTYPE;
#endif /* JERRY_BUILTIN_NUMBER */
    class_type = ECMA_OBJECT_CLASS_NUMBER;
  }
  else if (ecma_is_value_string (value))
  {
#if JERRY_BUILTIN_STRING
    proto_id = ECMA_BUILTIN_ID_STRING_PROTOTYPE;
#endif /* JERRY_BUILTIN_STRING */
    class_type = ECMA_OBJECT_CLASS_STRING;
  }
  else if (ecma_is_value_object (value))
  {
    return ecma_copy_value (value);
  }
  else if (ecma_is_value_symbol (value))
  {
    proto_id = ECMA_BUILTIN_ID_SYMBOL_PROTOTYPE;
    class_type = ECMA_OBJECT_CLASS_SYMBOL;
  }
#if JERRY_BUILTIN_BIGINT
  else if (ecma_is_value_bigint (value))
  {
    return ecma_op_create_bigint_object (value);
  }
#endif /* JERRY_BUILTIN_BIGINT */
  else
  {
    if (ecma_is_value_undefined (value) || ecma_is_value_null (value))
    {
      return ecma_raise_type_error (ECMA_ERR_ARGUMENT_CANNOT_CONVERT_TO_OBJECT);
    }
    else
    {
      JERRY_ASSERT (ecma_is_value_boolean (value));
#if JERRY_BUILTIN_BOOLEAN
      proto_id = ECMA_BUILTIN_ID_BOOLEAN_PROTOTYPE;
#endif /* JERRY_BUILTIN_BOOLEAN */
      class_type = ECMA_OBJECT_CLASS_BOOLEAN;
    }
  }

  ecma_object_t *object_p =
    ecma_create_object (ecma_builtin_get (proto_id), sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  ext_object_p->u.cls.type = class_type;
  ext_object_p->u.cls.u3.value = ecma_copy_value_if_not_object (value);

  return ecma_make_object_value (object_p);
} /* ecma_op_to_object */

/**
 * FromPropertyDescriptor operation.
 *
 * See also:
 *          ECMA-262 v5, 8.10.4
 *
 * @return constructed object
 */
ecma_object_t *
ecma_op_from_property_descriptor (const ecma_property_descriptor_t *src_prop_desc_p) /**< property descriptor */
{
  /* 2. */
  ecma_object_t *obj_p = ecma_op_create_object_object_noarg ();
  ecma_property_descriptor_t prop_desc = ecma_make_empty_property_descriptor ();
  {
    prop_desc.flags = (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE_DEFINED | JERRY_PROP_IS_WRITABLE
                       | JERRY_PROP_IS_ENUMERABLE_DEFINED | JERRY_PROP_IS_ENUMERABLE
                       | JERRY_PROP_IS_CONFIGURABLE_DEFINED | JERRY_PROP_IS_CONFIGURABLE);
  }

  /* 3. */
  if (src_prop_desc_p->flags & (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE_DEFINED))
  {
    /* a. */
    prop_desc.value = src_prop_desc_p->value;
    ecma_op_object_define_own_property (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_VALUE), &prop_desc);

    /* b. */
    prop_desc.value = ecma_make_boolean_value (src_prop_desc_p->flags & JERRY_PROP_IS_WRITABLE);
    ecma_op_object_define_own_property (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_WRITABLE), &prop_desc);
  }
  else if (src_prop_desc_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED))
  {
    /* a. */
    if (src_prop_desc_p->get_p == NULL)
    {
      prop_desc.value = ECMA_VALUE_UNDEFINED;
    }
    else
    {
      prop_desc.value = ecma_make_object_value (src_prop_desc_p->get_p);
    }

    ecma_op_object_define_own_property (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_GET), &prop_desc);

    /* b. */
    if (src_prop_desc_p->set_p == NULL)
    {
      prop_desc.value = ECMA_VALUE_UNDEFINED;
    }
    else
    {
      prop_desc.value = ecma_make_object_value (src_prop_desc_p->set_p);
    }
    ecma_op_object_define_own_property (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_SET), &prop_desc);
  }

  prop_desc.value = ecma_make_boolean_value (src_prop_desc_p->flags & JERRY_PROP_IS_ENUMERABLE);
  ecma_op_object_define_own_property (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_ENUMERABLE), &prop_desc);

  prop_desc.value = ecma_make_boolean_value (src_prop_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE);
  ecma_op_object_define_own_property (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_CONFIGURABLE), &prop_desc);

  return obj_p;
} /* ecma_op_from_property_descriptor */

/**
 * ToPropertyDescriptor operation.
 *
 * See also:
 *          ECMA-262 v5, 8.10.5
 *
 * @return ECMA_VALUE_EMPTY if successful, ECMA_VALUE_ERROR otherwise
 */
ecma_value_t
ecma_op_to_property_descriptor (ecma_value_t obj_value, /**< object value */
                                ecma_property_descriptor_t *out_prop_desc_p) /**< [out] filled property descriptor
                                                                              *   if the operation is successful,
                                                                              *   unmodified otherwise */
{
  /* 1. */
  if (!ecma_is_value_object (obj_value))
  {
    return ecma_raise_type_error (ECMA_ERR_EXPECTED_AN_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (obj_value);
  ecma_value_t ret_value = ECMA_VALUE_ERROR;
  ecma_value_t set_prop_value;
  ecma_value_t get_prop_value;
  ecma_value_t writable_prop_value;
  ecma_value_t value_prop_value;
  ecma_value_t configurable_prop_value;
  ecma_property_descriptor_t prop_desc;
  ecma_value_t enumerable_prop_value;

  /* 3. */
  enumerable_prop_value = ecma_op_object_find (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_ENUMERABLE));

  if (ECMA_IS_VALUE_ERROR (enumerable_prop_value))
  {
    return enumerable_prop_value;
  }

  /* 2. */
  prop_desc = ecma_make_empty_property_descriptor ();

  if (ecma_is_value_found (enumerable_prop_value))
  {
    uint32_t is_enumerable =
      (ecma_op_to_boolean (enumerable_prop_value) ? JERRY_PROP_IS_ENUMERABLE : JERRY_PROP_NO_OPTS);

    prop_desc.flags |= (uint16_t) (JERRY_PROP_IS_ENUMERABLE_DEFINED | is_enumerable);

    ecma_free_value (enumerable_prop_value);
  }

  /* 4. */
  configurable_prop_value = ecma_op_object_find (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_CONFIGURABLE));

  if (ECMA_IS_VALUE_ERROR (configurable_prop_value))
  {
    goto free_desc;
  }

  if (ecma_is_value_found (configurable_prop_value))
  {
    uint32_t is_configurable =
      (ecma_op_to_boolean (configurable_prop_value) ? JERRY_PROP_IS_CONFIGURABLE : JERRY_PROP_NO_OPTS);

    prop_desc.flags |= (uint16_t) (JERRY_PROP_IS_CONFIGURABLE_DEFINED | is_configurable);

    ecma_free_value (configurable_prop_value);
  }

  /* 5. */
  value_prop_value = ecma_op_object_find (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_VALUE));

  if (ECMA_IS_VALUE_ERROR (value_prop_value))
  {
    goto free_desc;
  }

  if (ecma_is_value_found (value_prop_value))
  {
    prop_desc.flags |= JERRY_PROP_IS_VALUE_DEFINED;
    prop_desc.value = ecma_copy_value (value_prop_value);
    ecma_free_value (value_prop_value);
  }

  /* 6. */
  writable_prop_value = ecma_op_object_find (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_WRITABLE));

  if (ECMA_IS_VALUE_ERROR (writable_prop_value))
  {
    goto free_desc;
  }

  if (ecma_is_value_found (writable_prop_value))
  {
    uint32_t is_writable = (ecma_op_to_boolean (writable_prop_value) ? JERRY_PROP_IS_WRITABLE : JERRY_PROP_NO_OPTS);

    prop_desc.flags |= (uint16_t) (JERRY_PROP_IS_WRITABLE_DEFINED | is_writable);

    ecma_free_value (writable_prop_value);
  }

  /* 7. */
  get_prop_value = ecma_op_object_find (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_GET));

  if (ECMA_IS_VALUE_ERROR (get_prop_value))
  {
    goto free_desc;
  }

  if (ecma_is_value_found (get_prop_value))
  {
    if (!ecma_op_is_callable (get_prop_value) && !ecma_is_value_undefined (get_prop_value))
    {
      ecma_free_value (get_prop_value);
      ret_value = ecma_raise_type_error (ECMA_ERR_EXPECTED_A_FUNCTION);
      goto free_desc;
    }

    prop_desc.flags |= JERRY_PROP_IS_GET_DEFINED;

    if (ecma_is_value_undefined (get_prop_value))
    {
      prop_desc.get_p = NULL;
    }
    else
    {
      JERRY_ASSERT (ecma_is_value_object (get_prop_value));

      ecma_object_t *get_p = ecma_get_object_from_value (get_prop_value);
      ecma_ref_object (get_p);

      prop_desc.get_p = get_p;
    }

    ecma_free_value (get_prop_value);
  }

  /* 8. */
  set_prop_value = ecma_op_object_find (obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_SET));

  if (ECMA_IS_VALUE_ERROR (set_prop_value))
  {
    goto free_desc;
  }

  if (ecma_is_value_found (set_prop_value))
  {
    if (!ecma_op_is_callable (set_prop_value) && !ecma_is_value_undefined (set_prop_value))
    {
      ecma_free_value (set_prop_value);
      ret_value = ecma_raise_type_error (ECMA_ERR_EXPECTED_A_FUNCTION);
      goto free_desc;
    }

    prop_desc.flags |= JERRY_PROP_IS_SET_DEFINED;

    if (ecma_is_value_undefined (set_prop_value))
    {
      prop_desc.set_p = NULL;
    }
    else
    {
      JERRY_ASSERT (ecma_is_value_object (set_prop_value));

      ecma_object_t *set_p = ecma_get_object_from_value (set_prop_value);
      ecma_ref_object (set_p);

      prop_desc.set_p = set_p;
    }

    ecma_free_value (set_prop_value);
  }

  /* 9. */
  if ((prop_desc.flags & (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE_DEFINED))
      && (prop_desc.flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED)))
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_ACCESSOR_WRITABLE);
  }
  else
  {
    *out_prop_desc_p = prop_desc;
    ret_value = ECMA_VALUE_EMPTY;
  }

free_desc:
  if (ECMA_IS_VALUE_ERROR (ret_value))
  {
    ecma_free_property_descriptor (&prop_desc);
  }

  return ret_value;
} /* ecma_op_to_property_descriptor */

/**
 * IsInteger operation.
 *
 * See also:
 *          ECMA-262 v5, 9.4
 *          ECMA-262 v6, 7.1.4
 *
 * @return true - if the argument is integer
 *              false - otherwise
 */
bool
ecma_op_is_integer (ecma_number_t num) /**< ecma number */
{
  if (ecma_number_is_nan (num) || ecma_number_is_infinity (num))
  {
    return false;
  }

  ecma_number_t floor_fabs = (ecma_number_t) floor (fabs (num));
  ecma_number_t fabs_value = (ecma_number_t) fabs (num);

  return (floor_fabs == fabs_value);
} /* ecma_op_is_integer */

/**
 * ToInteger operation.
 *
 * See also:
 *          ECMA-262 v5, 9.4
 *          ECMA-262 v6, 7.1.4
 *
 * @return ECMA_VALUE_EMPTY if successful
 *         conversion error otherwise
 */
ecma_value_t
ecma_op_to_integer (ecma_value_t value, /**< ecma value */
                    ecma_number_t *number_p) /**< [out] ecma number */
{
  if (ECMA_IS_VALUE_ERROR (value))
  {
    return value;
  }

  /* 1 */
  ecma_value_t to_number = ecma_op_to_number (value, number_p);

  /* 2 */
  if (ECMA_IS_VALUE_ERROR (to_number))
  {
    return to_number;
  }

  ecma_number_t number = *number_p;

  /* 3 */
  if (ecma_number_is_nan (number))
  {
    *number_p = ECMA_NUMBER_ZERO;
    return ECMA_VALUE_EMPTY;
  }

  /* 4 */
  if (ecma_number_is_zero (number) || ecma_number_is_infinity (number))
  {
    return ECMA_VALUE_EMPTY;
  }

  ecma_number_t floor_fabs = (ecma_number_t) floor (fabs (number));
  /* 5 */
  *number_p = ecma_number_is_negative (number) ? -floor_fabs : floor_fabs;
  return ECMA_VALUE_EMPTY;
} /* ecma_op_to_integer */

/**
 * ToLength operation.
 *
 * See also:
 *          ECMA-262 v6, 7.1.15
 *
 * @return ECMA_VALUE_EMPTY if successful
 *         conversion error otherwise
 */
ecma_value_t
ecma_op_to_length (ecma_value_t value, /**< ecma value */
                   ecma_length_t *length) /**< [out] ecma number */
{
  /* 1 */
  if (ECMA_IS_VALUE_ERROR (value))
  {
    return value;
  }

  /* 2 */
  ecma_number_t num;
  ecma_value_t length_num = ecma_op_to_integer (value, &num);

  /* 3 */
  if (ECMA_IS_VALUE_ERROR (length_num))
  {
    return length_num;
  }

  /* 4 */
  if (num <= 0)
  {
    *length = 0;
    return ECMA_VALUE_EMPTY;
  }

  /* 5 */
  if (num >= ECMA_NUMBER_MAX_SAFE_INTEGER)
  {
    *length = (ecma_length_t) ECMA_NUMBER_MAX_SAFE_INTEGER;
    return ECMA_VALUE_EMPTY;
  }

  /* 6 */
  *length = (ecma_length_t) num;
  return ECMA_VALUE_EMPTY;
} /* ecma_op_to_length */

/**
 * ToIndex operation.
 *
 * See also:
 *          ECMA-262 v11, 7.1.22
 *
 * @return ECMA_VALUE_EMPTY if successful
 *         conversion error otherwise
 */
ecma_value_t
ecma_op_to_index (ecma_value_t value, /**< ecma value */
                  ecma_number_t *index) /**< [out] ecma number */
{
  /* 1. */
  if (ecma_is_value_undefined (value))
  {
    *index = 0;
    return ECMA_VALUE_EMPTY;
  }

  /* 2.a */
  ecma_number_t integer_index;
  ecma_value_t index_value = ecma_op_to_integer (value, &integer_index);

  if (ECMA_IS_VALUE_ERROR (index_value))
  {
    return index_value;
  }

  /* 2.b - 2.d */
  if (integer_index < 0 || integer_index > ECMA_NUMBER_MAX_SAFE_INTEGER)
  {
    return ecma_raise_range_error (ECMA_ERR_INVALID_OR_OUT_OF_RANGE_INDEX);
  }

  /* 3. */
  *index = integer_index;
  return ECMA_VALUE_EMPTY;
} /* ecma_op_to_index */

/**
 * CreateListFromArrayLike operation.
 * Different types are not handled yet.
 *
 * See also:
 *          ECMA-262 v6, 7.3.17
 *
 * @return ecma_collection_t if successful
 *         NULL otherwise
 */
ecma_collection_t *
ecma_op_create_list_from_array_like (ecma_value_t arr, /**< array value */
                                     bool prop_names_only) /**< true - accept only property names
                                                                false - otherwise */
{
  /* 1. */
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (arr));

  /* 3. */
  if (!ecma_is_value_object (arr))
  {
    ecma_raise_type_error (ECMA_ERR_ARGUMENT_IS_NOT_AN_OBJECT);
    return NULL;
  }
  ecma_object_t *obj_p = ecma_get_object_from_value (arr);

  /* 4. 5. */
  ecma_length_t len;
  if (ECMA_IS_VALUE_ERROR (ecma_op_object_get_length (obj_p, &len)))
  {
    return NULL;
  }

  /* 6. */
  ecma_collection_t *list_ptr = ecma_new_collection ();

  /* 7. 8. */
  for (ecma_length_t idx = 0; idx < len; idx++)
  {
    ecma_value_t next = ecma_op_object_get_by_index (obj_p, idx);
    if (ECMA_IS_VALUE_ERROR (next))
    {
      ecma_collection_free (list_ptr);
      return NULL;
    }

    if (prop_names_only && !ecma_is_value_prop_name (next))
    {
      ecma_free_value (next);
      ecma_collection_free (list_ptr);
      ecma_raise_type_error (ECMA_ERR_PROPERTY_NAME_IS_NEITHER_SYMBOL_NOR_STRING);
      return NULL;
    }

    ecma_collection_push_back (list_ptr, next);
  }

  /* 9. */
  return list_ptr;
} /* ecma_op_create_list_from_array_like */

/**
 * @}
 * @}
 */
