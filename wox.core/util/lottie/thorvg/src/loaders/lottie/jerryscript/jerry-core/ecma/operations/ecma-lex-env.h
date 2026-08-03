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

#ifndef ECMA_LEX_ENV_H
#define ECMA_LEX_ENV_H

#include "ecma-globals.h"
#include "ecma-reference.h"

#include "jrt.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup lexicalenvironment Lexical environment
 * @{
 *
 * \addtogroup globallexicalenvironment Global lexical environment
 * @{
 */

void ecma_init_global_environment (void);
void ecma_finalize_global_environment (void);
ecma_object_t *ecma_get_global_environment (ecma_object_t *global_object_p);
ecma_object_t *ecma_get_global_scope (ecma_object_t *global_object_p);
void ecma_create_global_lexical_block (ecma_object_t *global_object_p);
ecma_value_t ecma_op_raise_set_binding_error (ecma_property_t *property_p, bool is_strict);

#if JERRY_MODULE_SYSTEM
void ecma_module_add_lex_env (ecma_object_t *lex_env_p);
void ecma_module_finalize_lex_envs (void);
#endif /* JERRY_MODULE_SYSTEM */

/**
 * @}
 */

/* ECMA-262 v5, 8.7.1 and 8.7.2 */
ecma_value_t
ecma_op_get_value_lex_env_base (ecma_object_t *lex_env_p, ecma_object_t **ref_base_lex_env_p, ecma_string_t *name_p);
ecma_value_t ecma_op_get_value_object_base (ecma_value_t base_value, ecma_string_t *property_name_p);
ecma_value_t ecma_op_put_value_lex_env_base (ecma_object_t *lex_env_p,
                                             ecma_string_t *var_name_string_p,
                                             bool is_strict,
                                             ecma_value_t value);

/* ECMA-262 v5, Table 17. Abstract methods of Environment Records */
ecma_value_t ecma_op_has_binding (ecma_object_t *lex_env_p, ecma_string_t *name_p);
ecma_property_t *ecma_op_create_mutable_binding (ecma_object_t *lex_env_p, ecma_string_t *name_p, bool is_deletable);
ecma_value_t
ecma_op_set_mutable_binding (ecma_object_t *lex_env_p, ecma_string_t *name_p, ecma_value_t value, bool is_strict);
ecma_value_t ecma_op_get_binding_value (ecma_object_t *lex_env_p, ecma_string_t *name_p, bool is_strict);
ecma_value_t ecma_op_delete_binding (ecma_object_t *lex_env_p, ecma_string_t *name_p);
ecma_value_t ecma_op_implicit_this_value (ecma_object_t *lex_env_p);

/* ECMA-262 v5, Table 18. Additional methods of Declarative Environment Records */
void ecma_op_create_immutable_binding (ecma_object_t *lex_env_p, ecma_string_t *name_p, ecma_value_t value);

void ecma_op_initialize_binding (ecma_object_t *lex_env_p, ecma_string_t *name_p, ecma_value_t value);

void ecma_op_create_environment_record (ecma_object_t *lex_env_p, ecma_value_t this_binding, ecma_object_t *func_obj_p);
ecma_environment_record_t *ecma_op_get_environment_record (ecma_object_t *lex_env_p);

bool ecma_op_this_binding_is_initialized (ecma_environment_record_t *environment_record_p);
void ecma_op_bind_this_value (ecma_environment_record_t *environment_record_p, ecma_value_t this_binding);
ecma_value_t ecma_op_get_this_binding (ecma_object_t *lex_env_p);

/**
 * @}
 * @}
 */

#endif /* !ECMA_LEX_ENV_H */
