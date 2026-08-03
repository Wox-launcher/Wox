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
#include "ecma-symbol-object.h"

#include "jrt.h"

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
  ECMA_SYMBOL_PROTOTYPE_ROUTINE_START = 0,
  ECMA_SYMBOL_PROTOTYPE_VALUE_OF, /**< ECMA-262 v11, 19.4.3.4 */
  ECMA_SYMBOL_PROTOTYPE_TO_PRIMITIVE, /**< ECMA-262 v11, 19.4.3.5 */
  ECMA_SYMBOL_PROTOTYPE_TO_STRING, /**< ECMA-262 v11, 19.4.3.3 */
  ECMA_SYMBOL_PROTOTYPE_DESCRIPTION, /**< ECMA-262 v11, 19.4.3.2 */
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-symbol-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  symbol_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup symbolprototype ECMA Symbol prototype object built-in
 * @{
 */

/**
 * Dispatcher of the Symbol built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_symbol_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                                ecma_value_t this_arg, /**< 'this' argument value */
                                                const ecma_value_t arguments_list[], /**< list of arguments
                                                                                      *   passed to routine */
                                                uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (arguments_list, arguments_number);

  ecma_value_t sym = ecma_symbol_this_value (this_arg);

  if (ECMA_IS_VALUE_ERROR (sym))
  {
    return sym;
  }

  if (builtin_routine_id < ECMA_SYMBOL_PROTOTYPE_TO_STRING)
  {
    return ecma_copy_value (sym);
  }

  if (builtin_routine_id == ECMA_SYMBOL_PROTOTYPE_TO_STRING)
  {
    return ecma_get_symbol_descriptive_string (sym);
  }

  JERRY_ASSERT (builtin_routine_id == ECMA_SYMBOL_PROTOTYPE_DESCRIPTION);
  ecma_string_t *symbol_p = ecma_get_symbol_from_value (sym);
  ecma_value_t desc = ecma_get_symbol_description (symbol_p);
  if (ecma_is_value_undefined (desc))
  {
    return desc;
  }

  ecma_string_t *desc_p = ecma_get_string_from_value (desc);
  ecma_ref_ecma_string (desc_p);
  return desc;
} /* ecma_builtin_symbol_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
