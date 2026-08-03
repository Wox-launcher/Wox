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

#ifndef ECMA_SYMBOL_H
#define ECMA_SYMBOL_H

#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmasymbolobject ECMA Symbol object related routines
 * @{
 */

ecma_value_t ecma_op_create_symbol (const ecma_value_t *arguments_list_p, uint32_t arguments_list_len);

ecma_value_t ecma_op_create_symbol_object (const ecma_value_t value);

bool ecma_prop_name_is_symbol (ecma_string_t *string_p);

ecma_value_t ecma_get_symbol_description (ecma_string_t *symbol_p);

ecma_value_t ecma_symbol_this_value (ecma_value_t this_arg);

ecma_value_t ecma_get_symbol_descriptive_string (ecma_value_t symbol_value);

/**
 * @}
 * @}
 */

#endif /* !ECMA_SYMBOL_H */
