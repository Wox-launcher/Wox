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

#ifndef ECMA_GC_H
#define ECMA_GC_H

#include "ecma-globals.h"

#include "jmem.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmagc Garbage collector
 * @{
 */

/**
 * Free option flags
 */
typedef enum
{
  ECMA_GC_FREE_NO_OPTIONS = 0, /**< no options */
  ECMA_GC_FREE_SECOND_PROPERTY = (1 << 0), /**< free second property of a property pair */
  ECMA_GC_FREE_REFERENCES = (1 << 1), /**< free references */
} ecma_gc_free_options_t;

void ecma_init_gc_info (ecma_object_t *object_p);
void ecma_ref_object (ecma_object_t *object_p);
void ecma_ref_object_inline (ecma_object_t *object_p);
void ecma_deref_object (ecma_object_t *object_p);
void ecma_gc_free_property (ecma_object_t *object_p, ecma_property_pair_t *prop_pair_p, uint32_t options);
void ecma_gc_free_properties (ecma_object_t *object_p, uint32_t options);
void ecma_gc_run (void);
void ecma_free_unused_memory (jmem_pressure_t pressure);

/**
 * @}
 * @}
 */

#endif /* !ECMA_GC_H */
