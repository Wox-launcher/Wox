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

#include "ecma-async-generator-object.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-promise-object.h"

#include "jcontext.h"
#include "opcodes.h"
#include "vm-defines.h"

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
  ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_START = 0,
  ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_NEXT,
  ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_THROW,
  ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_RETURN
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-async-generator-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  async_generator_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup asyncgeneratorprototype ECMA AsyncGenerator.prototype object built-in
 * @{
 */

/**
 * Convert routine type to operation type.
 */
#define ECMA_ASYNC_GENERATOR_ROUTINE_TO_OPERATION(type) \
  ((ecma_async_generator_operation_type_t) ((type) -ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_NEXT))

JERRY_STATIC_ASSERT ((ECMA_ASYNC_GENERATOR_ROUTINE_TO_OPERATION (ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_NEXT) == ECMA_ASYNC_GENERATOR_DO_NEXT),
                     convert_ecma_async_generator_routine_next_to_ecma_async_generator_do_next_failed);

JERRY_STATIC_ASSERT ((ECMA_ASYNC_GENERATOR_ROUTINE_TO_OPERATION (ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_THROW) == ECMA_ASYNC_GENERATOR_DO_THROW),
                     convert_ecma_async_generator_routine_throw_to_ecma_async_generator_do_throw_failed);

JERRY_STATIC_ASSERT ((ECMA_ASYNC_GENERATOR_ROUTINE_TO_OPERATION (ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_RETURN) == ECMA_ASYNC_GENERATOR_DO_RETURN),
                     convert_ecma_async_generator_routine_return_to_ecma_async_generator_do_return_failed);

/**
 * Dispatcher of the Generator built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_async_generator_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                                      *   identifier */
                                                         ecma_value_t this_arg, /**< 'this' argument value */
                                                         const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                                 *   passed to
                                                                                                 *   routine */
                                                         uint32_t arguments_number) /**< length of arguments'
                                                                                     *   list */
{
  JERRY_UNUSED (arguments_number);

  vm_executable_object_t *executable_object_p = NULL;

  if (ecma_is_value_object (this_arg))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (this_arg);

    if (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_ASYNC_GENERATOR))
    {
      executable_object_p = (vm_executable_object_t *) object_p;
    }
  }

  if (executable_object_p == NULL)
  {
    const char *msg_p = ecma_get_error_msg (ECMA_ERR_ARGUMENT_THIS_NOT_ASYNC_GENERATOR);
    lit_utf8_size_t msg_size = ecma_get_error_size (ECMA_ERR_ARGUMENT_THIS_NOT_ASYNC_GENERATOR);
    ecma_string_t *error_msg_p = ecma_new_ecma_string_from_ascii ((const lit_utf8_byte_t *) msg_p, msg_size);

    ecma_object_t *type_error_obj_p = ecma_new_standard_error (JERRY_ERROR_TYPE, error_msg_p);
    ecma_deref_ecma_string (error_msg_p);

    ecma_value_t promise = ecma_op_create_promise_object (ECMA_VALUE_EMPTY, ECMA_VALUE_UNDEFINED, NULL);
    ecma_reject_promise (promise, ecma_make_object_value (type_error_obj_p));
    ecma_deref_object (type_error_obj_p);

    return promise;
  }

  if (executable_object_p->extended_object.u.cls.u2.executable_obj_flags & ECMA_EXECUTABLE_OBJECT_COMPLETED)
  {
    ecma_value_t promise = ecma_make_object_value (ecma_builtin_get (ECMA_BUILTIN_ID_PROMISE));

    if (JERRY_UNLIKELY (builtin_routine_id == ECMA_ASYNC_GENERATOR_PROTOTYPE_ROUTINE_THROW))
    {
      return ecma_promise_reject_or_resolve (promise, arguments_list_p[0], false);
    }

    ecma_value_t iter_result = ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
    ecma_value_t result = ecma_promise_reject_or_resolve (promise, iter_result, true);
    ecma_free_value (iter_result);
    return result;
  }

  return ecma_async_generator_enqueue (executable_object_p,
                                       ECMA_ASYNC_GENERATOR_ROUTINE_TO_OPERATION (builtin_routine_id),
                                       arguments_list_p[0]);
} /* ecma_builtin_async_generator_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
