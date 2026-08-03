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
#include "ecma-dataview-object.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-shared-arraybuffer-object.h"
#include "ecma-typedarray-object.h"

#include "jrt.h"

#if JERRY_BUILTIN_SHAREDARRAYBUFFER

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-shared-arraybuffer.inc.h"
#define BUILTIN_UNDERSCORED_ID  shared_arraybuffer
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup sharedarraybuffer ECMA SharedArrayBuffer object built-in
 * @{
 */

/**
 * Handle calling [[Call]] of built-in SharedArrayBuffer object
 *
 * ES11 24.2.2 SharedArrayBuffer is not intended to be called as
 * a function and will throw an exception when called in
 * that manner.
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_shared_arraybuffer_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                               uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_raise_type_error (ECMA_ERR_CONSTRUCTOR_SHAREDARRAYBUFFER_REQUIRES_NEW);
} /* ecma_builtin_shared_arraybuffer_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in SharedArrayBuffer object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_shared_arraybuffer_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                                    uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_op_create_shared_arraybuffer_object (arguments_list_p, arguments_list_len);
} /* ecma_builtin_shared_arraybuffer_dispatch_construct */

/**
 * 24.2.3.2 get SharedArrayBuffer [ @@species ] accessor
 *
 * @return ecma_value
 *         returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_builtin_shared_arraybuffer_species_get (ecma_value_t this_value) /**< This Value */
{
  return ecma_copy_value (this_value);
} /* ecma_builtin_shared_arraybuffer_species_get */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */
