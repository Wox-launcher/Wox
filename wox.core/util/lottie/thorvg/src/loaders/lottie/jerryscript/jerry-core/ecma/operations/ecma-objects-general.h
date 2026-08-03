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

#ifndef ECMA_OBJECTS_GENERAL_H
#define ECMA_OBJECTS_GENERAL_H

#include "ecma-conversion.h"
#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaobjectsinternalops ECMA objects' operations
 * @{
 */

ecma_object_t *ecma_op_create_object_object_noarg (void);
ecma_object_t *ecma_op_create_object_object_noarg_and_set_prototype (ecma_object_t *object_prototype_p);

ecma_value_t ecma_op_general_object_delete (ecma_object_t *obj_p, ecma_string_t *property_name_p, bool is_throw);
ecma_value_t ecma_op_general_object_default_value (ecma_object_t *obj_p, ecma_preferred_type_hint_t hint);
ecma_value_t ecma_op_general_object_ordinary_value (ecma_object_t *obj_p, ecma_preferred_type_hint_t hint);
ecma_value_t ecma_op_general_object_define_own_property (ecma_object_t *object_p,
                                                         ecma_string_t *property_name_p,
                                                         const ecma_property_descriptor_t *property_desc_p);

void ecma_op_to_complete_property_descriptor (ecma_property_descriptor_t *desc_p);

bool ecma_op_is_compatible_property_descriptor (const ecma_property_descriptor_t *desc_p,
                                                const ecma_property_descriptor_t *current_p,
                                                bool is_extensible);

/**
 * @}
 * @}
 */

#endif /* !ECMA_OBJECTS_GENERAL_H */
