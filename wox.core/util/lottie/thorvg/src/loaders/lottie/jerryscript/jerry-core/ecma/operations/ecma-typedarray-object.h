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

#ifndef ECMA_TYPEDARRAY_OBJECT_H
#define ECMA_TYPEDARRAY_OBJECT_H

#include "ecma-builtins.h"
#include "ecma-globals.h"

#if JERRY_BUILTIN_TYPEDARRAY

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmatypedarrayobject ECMA TypedArray object related routines
 * @{
 */

uint8_t ecma_typedarray_helper_get_shift_size (ecma_typedarray_type_t typedarray_id);
lit_magic_string_id_t ecma_get_typedarray_magic_string_id (ecma_typedarray_type_t typedarray_id);
ecma_typedarray_getter_fn_t ecma_get_typedarray_getter_fn (ecma_typedarray_type_t typedarray_id);
ecma_typedarray_setter_fn_t ecma_get_typedarray_setter_fn (ecma_typedarray_type_t typedarray_id);
ecma_value_t ecma_get_typedarray_element (ecma_typedarray_info_t *info_p, uint32_t index);
ecma_value_t ecma_set_typedarray_element (ecma_typedarray_info_t *info_p, ecma_value_t value, uint32_t index);
bool ecma_typedarray_helper_is_typedarray (ecma_builtin_id_t builtin_id);
ecma_typedarray_type_t ecma_get_typedarray_id (ecma_object_t *obj_p);
ecma_builtin_id_t ecma_typedarray_helper_get_prototype_id (ecma_typedarray_type_t typedarray_id);
ecma_builtin_id_t ecma_typedarray_helper_get_constructor_id (ecma_typedarray_type_t typedarray_id);
ecma_typedarray_type_t ecma_typedarray_helper_builtin_to_typedarray_id (ecma_builtin_id_t builtin_id);

ecma_value_t
ecma_op_typedarray_from (ecma_value_t this_val, ecma_value_t source_val, ecma_value_t mapfn_val, ecma_value_t this_arg);
uint32_t ecma_typedarray_get_length (ecma_object_t *typedarray_p);
uint32_t ecma_typedarray_get_offset (ecma_object_t *typedarray_p);
uint8_t *ecma_typedarray_get_buffer (ecma_typedarray_info_t *info_p);
uint8_t ecma_typedarray_get_element_size_shift (ecma_object_t *typedarray_p);
ecma_object_t *ecma_typedarray_get_arraybuffer (ecma_object_t *typedarray_p);
ecma_value_t ecma_op_create_typedarray (const ecma_value_t *arguments_list_p,
                                        uint32_t arguments_list_len,
                                        ecma_object_t *proto_p,
                                        uint8_t element_size_shift,
                                        ecma_typedarray_type_t typedarray_id);
ecma_value_t ecma_typedarray_iterators_helper (ecma_value_t this_arg, ecma_iterator_kind_t kind);

bool ecma_object_is_typedarray (ecma_object_t *obj_p);
bool ecma_is_typedarray (ecma_value_t target);
bool ecma_typedarray_is_element_index (ecma_string_t *property_name_p);
void ecma_op_typedarray_list_lazy_property_names (ecma_object_t *obj_p,
                                                  ecma_collection_t *prop_names_p,
                                                  ecma_property_counter_t *prop_counter_p,
                                                  jerry_property_filter_t filter);
ecma_value_t ecma_op_typedarray_define_own_property (ecma_object_t *obj_p,
                                                     ecma_string_t *property_name_p,
                                                     const ecma_property_descriptor_t *property_desc_p);
ecma_value_t ecma_op_create_typedarray_with_type_and_length (ecma_typedarray_type_t typedarray_id,
                                                             uint32_t array_length);
ecma_typedarray_info_t ecma_typedarray_get_info (ecma_object_t *typedarray_p);
ecma_value_t ecma_typedarray_create_object_with_length (uint32_t array_length,
                                                        ecma_object_t *src_arraybuffer_p,
                                                        ecma_object_t *proto_p,
                                                        uint8_t element_size_shift,
                                                        ecma_typedarray_type_t typedarray_id);
ecma_value_t ecma_typedarray_create_object_with_object (ecma_value_t items_val,
                                                        ecma_object_t *proto_p,
                                                        uint8_t element_size_shift,
                                                        ecma_typedarray_type_t typedarray_id);
ecma_value_t
ecma_typedarray_create (ecma_object_t *constructor_p, ecma_value_t *arguments_list_p, uint32_t arguments_list_len);
ecma_value_t ecma_typedarray_species_create (ecma_value_t this_arg, ecma_value_t *length, uint32_t arguments_list_len);

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_TYPEDARRAY */
#endif /* !ECMA_TYPEDARRAY_OBJECT_H */
