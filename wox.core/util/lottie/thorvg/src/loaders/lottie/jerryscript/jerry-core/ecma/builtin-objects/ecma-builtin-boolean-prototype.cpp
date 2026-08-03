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
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-string-object.h"

#include "jrt.h"

#if JERRY_BUILTIN_BOOLEAN

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
  ECMA_BOOLEAN_PROTOTYPE_ROUTINE_START = 0,
  ECMA_BOOLEAN_PROTOTYPE_ROUTINE_TO_STRING,
  ECMA_BOOLEAN_PROTOTYPE_ROUTINE_VALUE_OF
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-boolean-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  boolean_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup booleanprototype ECMA Boolean.prototype object built-in
 * @{
 */

/**
 * The Boolean.prototype object's 'valueOf' routine
 *
 * See also:
 *          ECMA-262 v5, 15.6.4.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_boolean_prototype_object_value_of (ecma_value_t this_arg) /**< this argument */
{
  if (ecma_is_value_boolean (this_arg))
  {
    return this_arg;
  }
  else if (ecma_is_value_object (this_arg))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (this_arg);

    if (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_BOOLEAN))
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      JERRY_ASSERT (ecma_is_value_boolean (ext_object_p->u.cls.u3.value));

      return ext_object_p->u.cls.u3.value;
    }
  }

  return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_BOOLEAN_OBJECT);
} /* ecma_builtin_boolean_prototype_object_value_of */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_boolean_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                              *   identifier */
                                                 ecma_value_t this_arg, /**< 'this' argument value */
                                                 const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                         *   passed to routine */
                                                 uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (arguments_number, arguments_list_p);

  ecma_value_t value_of_ret = ecma_builtin_boolean_prototype_object_value_of (this_arg);
  if (builtin_routine_id == ECMA_BOOLEAN_PROTOTYPE_ROUTINE_VALUE_OF)
  {
    return value_of_ret;
  }

  JERRY_ASSERT (builtin_routine_id == ECMA_BOOLEAN_PROTOTYPE_ROUTINE_TO_STRING);

  if (ECMA_IS_VALUE_ERROR (value_of_ret))
  {
    return value_of_ret;
  }

  if (ecma_is_value_true (value_of_ret))
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING_TRUE);
  }

  JERRY_ASSERT (ecma_is_value_false (value_of_ret));

  return ecma_make_magic_string_value (LIT_MAGIC_STRING_FALSE);
} /* ecma_builtin_boolean_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_BOOLEAN */
