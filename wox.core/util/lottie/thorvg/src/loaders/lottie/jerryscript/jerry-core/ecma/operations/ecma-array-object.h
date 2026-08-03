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

#ifndef ECMA_ARRAY_OBJECT_H
#define ECMA_ARRAY_OBJECT_H

#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaarrayobject ECMA Array object related routines
 * @{
 */

/**
 * Attributes of fast access mode arrays:
 *
 * - The internal property is replaced with a buffer which directly stores the values
 * - Whenever any operation would change the following attributes of the array it should be converted back to normal
 *  - All properties must be enumerable configurable writable data properties
 *  - The prototype must be Array.prototype
 *  - [[Extensible]] internal property must be true
 *  - 'length' property of the array must be writable
 *
 * - The conversion is also required when a property is set if:
 *  - The property name is not an array index
 *  - The new hole count of the array would reach ECMA_FAST_ARRAY_MAX_NEW_HOLES_COUNT
 */

/**
 * Maximum number of new array holes in a fast mode access array.
 * If the number of new holes exceeds this limit, the array is converted back
 * to normal property list based array.
 */
#define ECMA_FAST_ARRAY_MAX_NEW_HOLES_COUNT 32

/**
 * Bitshift index for fast array hole count representation
 */
#define ECMA_FAST_ARRAY_HOLE_SHIFT 8

/**
 * This number represents 1 array hole in underlying buffer of a fast access mode array
 */
#define ECMA_FAST_ARRAY_HOLE_ONE (1 << ECMA_FAST_ARRAY_HOLE_SHIFT)

/**
 * Maximum number of array holes in a fast access mode array
 */
#define ECMA_FAST_ARRAY_MAX_HOLE_COUNT (1 << 16)

ecma_object_t *ecma_op_new_array_object (uint32_t length);

ecma_object_t *ecma_op_new_array_object_from_length (ecma_length_t length);

ecma_value_t ecma_op_new_array_object_from_buffer (const ecma_value_t *args_p, uint32_t length);

ecma_value_t ecma_op_new_array_object_from_collection (ecma_collection_t *collection_p, bool unref_objects);

bool ecma_op_object_is_fast_array (ecma_object_t *object_p);

bool ecma_op_array_is_fast_array (ecma_extended_object_t *array_p);

uint32_t ecma_fast_array_get_hole_count (ecma_object_t *obj_p);

ecma_value_t *ecma_fast_array_extend (ecma_object_t *object_p, uint32_t new_length);

bool ecma_fast_array_set_property (ecma_object_t *object_p, uint32_t index, ecma_value_t value);

bool ecma_array_object_delete_property (ecma_object_t *object_p, ecma_string_t *property_name_p);

uint32_t ecma_delete_fast_array_properties (ecma_object_t *object_p, uint32_t new_length);

ecma_collection_t *ecma_fast_array_object_own_property_keys (ecma_object_t *object_p, jerry_property_filter_t filter);

void ecma_fast_array_convert_to_normal (ecma_object_t *object_p);

ecma_object_t *ecma_op_array_species_create (ecma_object_t *original_array_p, ecma_length_t length);

ecma_value_t ecma_op_create_array_iterator (ecma_object_t *obj_p, ecma_iterator_kind_t kind);

ecma_value_t ecma_op_array_object_set_length (ecma_object_t *object_p, ecma_value_t new_value, uint16_t flags);

ecma_value_t ecma_op_array_object_define_own_property (ecma_object_t *object_p,
                                                       ecma_string_t *property_name_p,
                                                       const ecma_property_descriptor_t *property_desc_p);

uint32_t ecma_array_get_length (ecma_object_t *array_p);

ecma_value_t ecma_array_object_to_string (ecma_value_t this_arg);

/**
 * @}
 * @}
 */

#endif /* !ECMA_ARRAY_OBJECT_H */
