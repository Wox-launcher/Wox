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
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
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
 * Equality opcode handler.
 *
 * See also: ECMA-262 v5, 11.9.1, 11.9.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
opfunc_equality (ecma_value_t left_value, /**< left value */
                 ecma_value_t right_value) /**< right value */
{
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (left_value) && !ECMA_IS_VALUE_ERROR (right_value));

  ecma_value_t compare_result = ecma_op_abstract_equality_compare (left_value, right_value);

  JERRY_ASSERT (ecma_is_value_boolean (compare_result) || ECMA_IS_VALUE_ERROR (compare_result));

  return compare_result;
} /* opfunc_equality */

/**
 * Relation opcode handler.
 *
 * See also: ECMA-262 v5, 11.8.1, 11.8.2, 11.8.3, 11.8.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
opfunc_relation (ecma_value_t left_value, /**< left value */
                 ecma_value_t right_value, /**< right value */
                 bool left_first, /**< 'LeftFirst' flag */
                 bool is_invert) /**< is invert */
{
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (left_value) && !ECMA_IS_VALUE_ERROR (right_value));

  ecma_value_t ret_value = ecma_op_abstract_relational_compare (left_value, right_value, left_first);

  if (ECMA_IS_VALUE_ERROR (ret_value))
  {
    return ret_value;
  }

  if (ecma_is_value_undefined (ret_value))
  {
    ret_value = ECMA_VALUE_FALSE;
  }
  else
  {
    JERRY_ASSERT (ecma_is_value_boolean (ret_value));

    if (is_invert)
    {
      ret_value = ecma_invert_boolean_value (ret_value);
    }
  }

  return ret_value;
} /* opfunc_relation */

/**
 * 'instanceof' opcode handler.
 *
 * See also: ECMA-262 v5, 11.8.6
 *
 * @return ecma value
 *         returned value must be freed with ecma_free_value.
 */
ecma_value_t
opfunc_instanceof (ecma_value_t left_value, /**< left value */
                   ecma_value_t right_value) /**< right value */
{
  if (!ecma_is_value_object (right_value))
  {
    return ecma_raise_type_error (ECMA_ERR_RIGHT_VALUE_OF_INSTANCEOF_MUST_BE_AN_OBJECT);
  }

  ecma_value_t has_instance_method = ecma_op_get_method_by_symbol_id (right_value, LIT_GLOBAL_SYMBOL_HAS_INSTANCE);
  if (ECMA_IS_VALUE_ERROR (has_instance_method))
  {
    return has_instance_method;
  }

  if (JERRY_UNLIKELY (!ecma_is_value_undefined (has_instance_method)))
  {
    ecma_object_t *method_obj_p = ecma_get_object_from_value (has_instance_method);
    ecma_value_t has_instance_result = ecma_op_function_call (method_obj_p, right_value, &left_value, 1);

    ecma_free_value (has_instance_method);

    if (ECMA_IS_VALUE_ERROR (has_instance_result))
    {
      return has_instance_result;
    }

    bool has_instance = ecma_op_to_boolean (has_instance_result);
    ecma_free_value (has_instance_result);

    return ecma_make_boolean_value (has_instance);
  }

  ecma_object_t *right_value_obj_p = ecma_get_object_from_value (right_value);
  return ecma_op_object_has_instance (right_value_obj_p, left_value);
} /* opfunc_instanceof */

/**
 * 'in' opcode handler.
 *
 * See also:
 *  * ECMA-262 v5, 11.8.7
 *  * ECAM-262 v6, 12.9.3
 *
 * @return ecma value
 *         returned value must be freed with ecma_free_value.
 */
ecma_value_t
opfunc_in (ecma_value_t left_value, /**< left value */
           ecma_value_t right_value) /**< right value */
{
  if (!ecma_is_value_object (right_value))
  {
    return ecma_raise_type_error (ECMA_ERR_RIGHT_VALUE_OF_IN_MUST_BE_AN_OBJECT);
  }

  ecma_string_t *property_name_p = ecma_op_to_property_key (left_value);

  if (JERRY_UNLIKELY (property_name_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_object_t *right_value_obj_p = ecma_get_object_from_value (right_value);
  ecma_value_t result = ecma_op_object_has_property (right_value_obj_p, property_name_p);
  ecma_deref_ecma_string (property_name_p);
  return result;
} /* opfunc_in */

/**
 * @}
 * @}
 */
