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

#ifndef ECMA_REFERENCE_H
#define ECMA_REFERENCE_H

#include "ecma-globals.h"

#include "jrt.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup references ECMA-Reference
 * @{
 */

ecma_object_t *ecma_op_resolve_reference_base (ecma_object_t *lex_env_p, ecma_string_t *name_p);
ecma_value_t ecma_op_resolve_reference_value (ecma_object_t *lex_env_p, ecma_string_t *name_p);
ecma_value_t ecma_op_object_bound_environment_resolve_reference_value (ecma_object_t *lex_env_p, ecma_string_t *name_p);
ecma_value_t ecma_op_resolve_super_base (ecma_object_t *lex_env_p);

/**
 * @}
 * @}
 */

#endif /* !ECMA_REFERENCE_H */
