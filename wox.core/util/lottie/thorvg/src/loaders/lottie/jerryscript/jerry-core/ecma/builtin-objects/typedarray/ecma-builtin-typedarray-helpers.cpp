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

#include "ecma-builtin-typedarray-helpers.h"

#if JERRY_BUILTIN_TYPEDARRAY

#include "ecma-builtins.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-objects.h"
#include "ecma-typedarray-object.h"

#include "jcontext.h"

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

/**
 * Common implementation of the [[Construct]] call of TypedArrays.
 *
 * @return ecma value of the new TypedArray object
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_typedarray_helper_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                           uint32_t arguments_list_len, /**< number of arguments */
                                           ecma_typedarray_type_t typedarray_id) /**< id of the typedarray */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);
  ecma_builtin_id_t proto_id = ecma_typedarray_helper_get_prototype_id (typedarray_id);
  ecma_object_t *prototype_obj_p = NULL;
  ecma_object_t *current_new_target_p = JERRY_CONTEXT (current_new_target_p);

  if (current_new_target_p != NULL)
  {
    prototype_obj_p = ecma_op_get_prototype_from_constructor (current_new_target_p, proto_id);
    if (prototype_obj_p == NULL)
    {
      return ECMA_VALUE_ERROR;
    }
  }
  else
  {
    prototype_obj_p = ecma_builtin_get (proto_id);
  }

  ecma_value_t val = ecma_op_create_typedarray (arguments_list_p,
                                                arguments_list_len,
                                                prototype_obj_p,
                                                ecma_typedarray_helper_get_shift_size (typedarray_id),
                                                typedarray_id);

  if (current_new_target_p != NULL)
  {
    ecma_deref_object (prototype_obj_p);
  }

  return val;
} /* ecma_typedarray_helper_dispatch_construct */

#endif /* JERRY_BUILTIN_TYPEDARRAY */
