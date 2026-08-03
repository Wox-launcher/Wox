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

#include "ecma-builtins.h"
#include "ecma-container-object.h"
#include "ecma-exceptions.h"

#if JERRY_BUILTIN_CONTAINER

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-map.inc.h"
#define BUILTIN_UNDERSCORED_ID  map
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup map ECMA Map object built-in
 * @{
 */

/**
 * Handle calling [[Call]] of built-in Map object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_map_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_raise_type_error (ECMA_ERR_CONSTRUCTOR_MAP_REQUIRES_NEW);
} /* ecma_builtin_map_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Map object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_map_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                     uint32_t arguments_list_len) /**< number of arguments */
{
  return ecma_op_container_create (arguments_list_p,
                                   arguments_list_len,
                                   LIT_MAGIC_STRING_MAP_UL,
                                   ECMA_BUILTIN_ID_MAP_PROTOTYPE);
} /* ecma_builtin_map_dispatch_construct */

/**
 * 23.1.2.2 get Map [ @@species ] accessor
 *
 * @return ecma_value
 *         returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_builtin_map_species_get (ecma_value_t this_value) /**< This Value */
{
  return ecma_copy_value (this_value);
} /* ecma_builtin_map_species_get */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_CONTAINER */
