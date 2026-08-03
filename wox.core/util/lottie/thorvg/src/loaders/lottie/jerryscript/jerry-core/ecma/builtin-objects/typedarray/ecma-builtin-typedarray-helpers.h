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

#ifndef ECMA_TYPEDARRAY_HELPERS_H
#define ECMA_TYPEDARRAY_HELPERS_H
#include "ecma-globals.h"

#if JERRY_BUILTIN_TYPEDARRAY

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltinhelpers ECMA builtin helper operations
 * @{
 */

ecma_value_t ecma_typedarray_helper_dispatch_construct (const ecma_value_t *arguments_list_p,
                                                        uint32_t arguments_list_len,
                                                        ecma_typedarray_type_t typedarray_id);

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_TYPEDARRAY */
#endif /* !ECMA_TYPEDARRAY_HELPERS_H */
