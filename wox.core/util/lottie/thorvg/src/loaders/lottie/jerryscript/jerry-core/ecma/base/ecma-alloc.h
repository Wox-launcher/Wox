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

#ifndef ECMA_ALLOC_H
#define ECMA_ALLOC_H

#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaalloc Routines for allocation/freeing memory for ECMA data types
 * @{
 */

/**
 * Allocate memory for ecma-object
 *
 * @return pointer to allocated memory
 */
ecma_object_t *ecma_alloc_object (void);

/**
 * Dealloc memory from an ecma-object
 */
void ecma_dealloc_object (ecma_object_t *object_p);

/**
 * Allocate memory for extended object
 *
 * @return pointer to allocated memory
 */
ecma_extended_object_t *ecma_alloc_extended_object (size_t size);

/**
 * Dealloc memory of an extended object
 */
void ecma_dealloc_extended_object (ecma_object_t *object_p, size_t size);

/**
 * Allocate memory for ecma-number
 *
 * @return pointer to allocated memory
 */
ecma_number_t *ecma_alloc_number (void);

/**
 * Dealloc memory from an ecma-number
 */
void ecma_dealloc_number (ecma_number_t *number_p);

/**
 * Allocate memory for ecma-string descriptor
 *
 * @return pointer to allocated memory
 */
ecma_string_t *ecma_alloc_string (void);

/**
 * Dealloc memory from ecma-string descriptor
 */
void ecma_dealloc_string (ecma_string_t *string_p);

/**
 * Allocate memory for extended ecma-string descriptor
 *
 * @return pointer to allocated memory
 */
ecma_extended_string_t *ecma_alloc_extended_string (void);

/**
 * Dealloc memory from extended ecma-string descriptor
 */
void ecma_dealloc_extended_string (ecma_extended_string_t *string_p);

/**
 * Allocate memory for external ecma-string descriptor
 *
 * @return pointer to allocated memory
 */
ecma_external_string_t *ecma_alloc_external_string (void);

/**
 * Dealloc memory from external ecma-string descriptor
 */
void ecma_dealloc_external_string (ecma_external_string_t *string_p);

/**
 * Allocate memory for string with character data
 *
 * @return pointer to allocated memory
 */
ecma_string_t *ecma_alloc_string_buffer (size_t size);

/**
 * Dealloc memory of a string with character data
 */
void ecma_dealloc_string_buffer (ecma_string_t *string_p, size_t size);

/**
 * Allocate memory for ecma-property pair
 *
 * @return pointer to allocated memory
 */
ecma_property_pair_t *ecma_alloc_property_pair (void);

/**
 * Dealloc memory from an ecma-property pair
 */
void ecma_dealloc_property_pair (ecma_property_pair_t *property_pair_p);

/**
 * @}
 * @}
 */

#endif /* !ECMA_ALLOC_H */
