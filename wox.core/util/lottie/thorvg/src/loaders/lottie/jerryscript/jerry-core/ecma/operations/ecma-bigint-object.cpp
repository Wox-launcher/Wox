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

#include "ecma-bigint-object.h"

#include "ecma-alloc.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#if JERRY_BUILTIN_BIGINT

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabigintobject ECMA BigInt object related routines
 * @{
 */

/**
 * BigInt object creation operation.
 *
 * See also: ECMA-262 v11, 7.1.18
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_create_bigint_object (ecma_value_t arg) /**< argument passed to the toObject operation */
{
  JERRY_ASSERT (ecma_is_value_bigint (arg));

  ecma_object_t *prototype_obj_p = ecma_builtin_get (ECMA_BUILTIN_ID_BIGINT_PROTOTYPE);

  ecma_object_t *object_p =
    ecma_create_object (prototype_obj_p, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_BIGINT;
  ext_object_p->u.cls.u3.value = ecma_copy_value (arg);

  return ecma_make_object_value (object_p);
} /* ecma_op_create_bigint_object */

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_BIGINT */
