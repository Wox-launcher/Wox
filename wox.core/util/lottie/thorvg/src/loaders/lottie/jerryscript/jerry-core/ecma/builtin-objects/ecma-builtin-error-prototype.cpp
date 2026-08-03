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
#include "lit-char-helpers.h"
#include "lit-magic-strings.h"

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
  ECMA_ERROR_PROTOTYPE_ROUTINE_START = 0,
  ECMA_ERROR_PROTOTYPE_ROUTINE_TO_STRING,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-error-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  error_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup errorprototype ECMA Error.prototype object built-in
 * @{
 */

/**
 * Helper method to get a property value from an error object
 *
 * @return ecma_string_t
 */
static ecma_string_t *
ecma_builtin_error_prototype_object_to_string_helper (ecma_object_t *obj_p, /**< error object */
                                                      lit_magic_string_id_t property_id, /**< property id */
                                                      lit_magic_string_id_t default_value) /**< default prop value */
{
  ecma_value_t prop_value = ecma_op_object_get_by_magic_id (obj_p, property_id);

  if (ECMA_IS_VALUE_ERROR (prop_value))
  {
    return NULL;
  }

  if (ecma_is_value_undefined (prop_value))
  {
    return ecma_get_magic_string (default_value);
  }

  ecma_string_t *ret_str_p = ecma_op_to_string (prop_value);
  ecma_free_value (prop_value);

  return ret_str_p;
} /* ecma_builtin_error_prototype_object_to_string_helper */

/**
 * The Error.prototype object's 'toString' routine
 *
 * See also:
 *          ECMA-262 v5, 15.11.4.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_error_prototype_object_to_string (ecma_value_t this_arg) /**< this argument */
{
  /* 2. */
  if (!ecma_is_value_object (this_arg))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (this_arg);

  ecma_string_t *name_string_p =
    ecma_builtin_error_prototype_object_to_string_helper (obj_p, LIT_MAGIC_STRING_NAME, LIT_MAGIC_STRING_ERROR_UL);

  if (JERRY_UNLIKELY (name_string_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_string_t *msg_string_p =
    ecma_builtin_error_prototype_object_to_string_helper (obj_p, LIT_MAGIC_STRING_MESSAGE, LIT_MAGIC_STRING__EMPTY);

  if (JERRY_UNLIKELY (msg_string_p == NULL))
  {
    ecma_deref_ecma_string (name_string_p);
    return ECMA_VALUE_ERROR;
  }

  if (ecma_string_is_empty (name_string_p))
  {
    return ecma_make_string_value (msg_string_p);
  }

  if (ecma_string_is_empty (msg_string_p))
  {
    return ecma_make_string_value (name_string_p);
  }

  ecma_stringbuilder_t builder = ecma_stringbuilder_create_from (name_string_p);

  ecma_stringbuilder_append_raw (&builder, (const lit_utf8_byte_t *) ": ", 2);
  ecma_stringbuilder_append (&builder, msg_string_p);

  ecma_deref_ecma_string (name_string_p);
  ecma_deref_ecma_string (msg_string_p);

  return ecma_make_string_value (ecma_stringbuilder_finalize (&builder));
} /* ecma_builtin_error_prototype_object_to_string */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_error_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                            *   identifier */
                                               ecma_value_t this_arg, /**< 'this' argument value */
                                               const ecma_value_t arguments_list_p[], /**< list of arguments passed to
                                                                                       *  routine */
                                               uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (arguments_number, arguments_list_p);

  switch (builtin_routine_id)
  {
    case ECMA_ERROR_PROTOTYPE_ROUTINE_TO_STRING:
    {
      return ecma_builtin_error_prototype_object_to_string (this_arg);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_error_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
