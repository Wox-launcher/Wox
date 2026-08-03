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

#ifndef ECMA_ARGUMENTS_OBJECT_H
#define ECMA_ARGUMENTS_OBJECT_H

#include "ecma-globals.h"
#include "ecma-helpers.h"

#include "vm-defines.h"

ecma_value_t ecma_op_create_arguments_object (vm_frame_ctx_shared_args_t *shared_p, ecma_object_t *lex_env_p);

ecma_value_t ecma_op_arguments_object_define_own_property (ecma_object_t *object_p,
                                                           ecma_string_t *property_name_p,
                                                           const ecma_property_descriptor_t *property_desc_p);

ecma_property_t *ecma_op_arguments_object_try_to_lazy_instantiate_property (ecma_object_t *object_p,
                                                                            ecma_string_t *property_name_p);

void ecma_op_arguments_delete_built_in_property (ecma_object_t *object_p, ecma_string_t *property_name_p);

void ecma_op_arguments_object_list_lazy_property_names (ecma_object_t *obj_p,
                                                        ecma_collection_t *prop_names_p,
                                                        ecma_property_counter_t *prop_counter_p,
                                                        jerry_property_filter_t filter);

ecma_string_t *ecma_op_arguments_object_get_formal_parameter (ecma_mapped_arguments_t *mapped_arguments_p,
                                                              uint32_t index);
#endif /* !ECMA_ARGUMENTS_OBJECT_H */
