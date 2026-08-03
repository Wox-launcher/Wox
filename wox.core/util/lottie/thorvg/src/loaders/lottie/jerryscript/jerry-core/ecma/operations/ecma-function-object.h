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

#ifndef ECMA_FUNCTION_OBJECT_H
#define ECMA_FUNCTION_OBJECT_H

#include "ecma-builtin-handlers.h"
#include "ecma-builtins.h"
#include "ecma-globals.h"

#include "vm.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmafunctionobject ECMA Function object related routines
 * @{
 */

ecma_value_t ecma_op_function_form_name (ecma_string_t *prop_name_p, char *prefix_p, lit_utf8_size_t prefix_size);

bool ecma_op_is_callable (ecma_value_t value);
#if JERRY_BUILTIN_PROXY
bool ecma_op_proxy_object_is_callable (ecma_object_t *obj_p);
#endif /* JERRY_BUILTIN_PROXY */
bool ecma_op_object_is_callable (ecma_object_t *obj_p);
bool ecma_is_constructor (ecma_value_t value);
bool ecma_object_is_constructor (ecma_object_t *obj_p);

/**
 * Special constant indicating that the value is a valid constructor
 *
 * Use after the ecma_*_check_constructor calls.
 */

ecma_error_msg_t ecma_object_check_constructor (ecma_object_t *obj_p);
ecma_error_msg_t ecma_check_constructor (ecma_value_t value);

ecma_object_t *ecma_op_create_simple_function_object (ecma_object_t *scope_p,
                                                      const ecma_compiled_code_t *bytecode_data_p);

ecma_object_t *ecma_op_create_external_function_object (ecma_native_handler_t handler_cb);

const ecma_compiled_code_t *ecma_op_function_get_compiled_code (ecma_extended_object_t *function_p);

#if JERRY_BUILTIN_REALMS
ecma_global_object_t *ecma_op_function_get_realm (const ecma_compiled_code_t *bytecode_header_p);

ecma_global_object_t *ecma_op_function_get_function_realm (ecma_object_t *func_obj_p);
#endif /* JERRY_BUILTIN_REALMS */

ecma_value_t ecma_op_create_dynamic_function (const ecma_value_t *arguments_list_p,
                                              uint32_t arguments_list_len,
                                              ecma_parse_opts_t opts);

ecma_value_t ecma_op_function_get_super_constructor (ecma_object_t *func_obj_p);

ecma_object_t *ecma_op_create_any_function_object (ecma_object_t *scope_p, const ecma_compiled_code_t *bytecode_data_p);

ecma_object_t *ecma_op_create_arrow_function_object (ecma_object_t *scope_p,
                                                     const ecma_compiled_code_t *bytecode_data_p,
                                                     ecma_value_t this_binding);

ecma_object_t *ecma_op_create_native_handler (ecma_native_handler_id_t id, size_t object_size);

ecma_object_t *ecma_op_get_prototype_from_constructor (ecma_object_t *ctor_obj_p, ecma_builtin_id_t default_proto_id);

ecma_value_t ecma_op_function_has_instance (ecma_object_t *func_obj_p, ecma_value_t value);

ecma_value_t ecma_op_function_validated_call (ecma_value_t callee,
                                              ecma_value_t this_arg_value,
                                              const ecma_value_t *arguments_list_p,
                                              uint32_t arguments_list_len);

ecma_value_t ecma_op_function_call (ecma_object_t *func_obj_p,
                                    ecma_value_t this_arg_value,
                                    const ecma_value_t *arguments_list_p,
                                    uint32_t arguments_list_len);

ecma_value_t ecma_op_function_construct (ecma_object_t *func_obj_p,
                                         ecma_object_t *new_target_p,
                                         const ecma_value_t *arguments_list_p,
                                         uint32_t arguments_list_len);

ecma_property_t *ecma_op_function_try_to_lazy_instantiate_property (ecma_object_t *object_p,
                                                                    ecma_string_t *property_name_p);

ecma_property_t *ecma_op_external_function_try_to_lazy_instantiate_property (ecma_object_t *object_p,
                                                                             ecma_string_t *property_name_p);

ecma_property_t *ecma_op_bound_function_try_to_lazy_instantiate_property (ecma_object_t *object_p,
                                                                          ecma_string_t *property_name_p);

void ecma_op_function_delete_built_in_property (ecma_object_t *object_p, ecma_string_t *property_name_p);

void ecma_op_bound_function_delete_built_in_property (ecma_object_t *object_p, ecma_string_t *property_name_p);

void ecma_op_function_list_lazy_property_names (ecma_object_t *object_p,
                                                ecma_collection_t *prop_names_p,
                                                ecma_property_counter_t *prop_counter_p,
                                                jerry_property_filter_t filter);

void ecma_op_external_function_list_lazy_property_names (ecma_object_t *object_p,
                                                         ecma_collection_t *prop_names_p,
                                                         ecma_property_counter_t *prop_counter_p,
                                                         jerry_property_filter_t filter);

void ecma_op_bound_function_list_lazy_property_names (ecma_object_t *object_p,
                                                      ecma_collection_t *prop_names_p,
                                                      ecma_property_counter_t *prop_counter_p,
                                                      jerry_property_filter_t filter);

/**
 * @}
 * @}
 */

#endif /* !ECMA_FUNCTION_OBJECT_H */
