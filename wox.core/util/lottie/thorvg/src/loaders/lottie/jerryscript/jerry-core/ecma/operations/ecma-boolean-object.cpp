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

#include "ecma-boolean-object.h"

#include "ecma-alloc.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#include "jcontext.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabooleanobject ECMA Boolean object related routines
 * @{
 */

/**
 * Boolean object creation operation.
 *
 * See also: ECMA-262 v5, 15.6.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_create_boolean_object (ecma_value_t arg) /**< argument passed to the Boolean constructor */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  bool boolean_value = ecma_op_to_boolean (arg);
  ecma_builtin_id_t proto_id;

#if JERRY_BUILTIN_BOOLEAN
  proto_id = ECMA_BUILTIN_ID_BOOLEAN_PROTOTYPE;
#else /* JERRY_BUILTIN_BOOLEAN */
  proto_id = ECMA_BUILTIN_ID_OBJECT_PROTOTYPE;
#endif /* !(JERRY_BUILTIN_BOOLEAN */

  ecma_object_t *prototype_obj_p = ecma_builtin_get (proto_id);

  ecma_object_t *new_target = JERRY_CONTEXT (current_new_target_p);
  if (new_target)
  {
    prototype_obj_p = ecma_op_get_prototype_from_constructor (new_target, proto_id);

    if (JERRY_UNLIKELY (prototype_obj_p == NULL))
    {
      return ECMA_VALUE_ERROR;
    }
  }

  ecma_object_t *object_p =
    ecma_create_object (prototype_obj_p, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_BOOLEAN;
  ext_object_p->u.cls.u3.value = ecma_make_boolean_value (boolean_value);

  if (new_target)
  {
    ecma_deref_object (prototype_obj_p);
  }

  return ecma_make_object_value (object_p);
} /* ecma_op_create_boolean_object */

/**
 * @}
 * @}
 */
