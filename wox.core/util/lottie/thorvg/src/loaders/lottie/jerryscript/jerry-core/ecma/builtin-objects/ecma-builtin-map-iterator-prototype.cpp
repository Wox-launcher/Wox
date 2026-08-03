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

#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-container-object.h"

#if JERRY_BUILTIN_CONTAINER

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
  ECMA_BUILTIN_MAP_ITERATOR_PROTOTYPE_ROUTINE_START = 0,
  ECMA_BUILTIN_MAP_ITERATOR_PROTOTYPE_ROUTINE_OBJECT_NEXT,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-map-iterator-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  map_iterator_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup %mapiteratorprototype% ECMA %MapIteratorPrototype% object built-in
 * @{
 */

/**
 * The %MapIteratorPrototype% object's 'next' routine
 *
 * See also:
 *          ECMA-262 v6, 23.1.5.2.1
 *
 * Note:
 *     Returned value must be freed with ecma_free_value.
 *
 * @return iterator result object, if success
 *         error - otherwise
 */
static ecma_value_t
ecma_builtin_map_iterator_prototype_object_next (ecma_value_t this_val) /**< this argument */
{
  return ecma_op_container_iterator_next (this_val, ECMA_OBJECT_CLASS_MAP_ITERATOR);
} /* ecma_builtin_map_iterator_prototype_object_next */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_map_iterator_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide
                                                                                   *   routine identifier */
                                                      ecma_value_t this_arg, /**< 'this' argument value */
                                                      const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                              *   passed to routine */
                                                      uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (arguments_list_p, arguments_number);

  switch (builtin_routine_id)
  {
    case ECMA_BUILTIN_MAP_ITERATOR_PROTOTYPE_ROUTINE_OBJECT_NEXT:
    {
      return ecma_builtin_map_iterator_prototype_object_next (this_arg);
    }
    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* ecma_builtin_map_iterator_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_CONTAINER */
