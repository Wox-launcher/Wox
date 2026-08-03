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

#ifndef ECMA_PROPERTY_HASHMAP_H
#define ECMA_PROPERTY_HASHMAP_H

#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmapropertyhashmap Property hashmap
 * @{
 */

/**
 * Recommended minimum number of items in a property cache.
 */
#define ECMA_PROPERTY_HASHMAP_MINIMUM_SIZE 32

/**
 * Property hash.
 */
typedef struct
{
  ecma_property_header_t header; /**< header of the property */
  uint32_t max_property_count; /**< maximum property count (power of 2) */
  uint32_t null_count; /**< number of NULLs in the map */
  uint32_t unused_count; /**< number of unused entries in the map */

  /*
   * The hash is followed by max_property_count ecma_cpointer_t
   * compressed pointers and (max_property_count + 7) / 8 bytes
   * which stores a flag for each compressed pointer.
   *
   * If the compressed pointer is equal to ECMA_NULL_POINTER
   *   - flag is cleared if the entry is NULL
   *   - flag is set if the entry is deleted
   *
   * If the compressed pointer is not equal to ECMA_NULL_POINTER
   *   - flag is cleared if the first entry of a property pair is referenced
   *   - flag is set if the second entry of a property pair is referenced
   */
} ecma_property_hashmap_t;

#if JERRY_PROPERTY_HASHMAP

/**
 * Simple ecma values
 */
typedef enum
{
  ECMA_PROPERTY_HASHMAP_DELETE_NO_HASHMAP, /**< object has no hashmap */
  ECMA_PROPERTY_HASHMAP_DELETE_HAS_HASHMAP, /**< object has hashmap */
  ECMA_PROPERTY_HASHMAP_DELETE_RECREATE_HASHMAP, /**< hashmap should be recreated */
} ecma_property_hashmap_delete_status;

void ecma_property_hashmap_create (ecma_object_t *object_p);
void ecma_property_hashmap_free (ecma_object_t *object_p);
void ecma_property_hashmap_insert (ecma_object_t *object_p,
                                   ecma_string_t *name_p,
                                   ecma_property_pair_t *property_pair_p,
                                   int property_index);
ecma_property_hashmap_delete_status
ecma_property_hashmap_delete (ecma_object_t *object_p, jmem_cpointer_t name_cp, ecma_property_t *property_p);

ecma_property_t *ecma_property_hashmap_find (ecma_property_hashmap_t *hashmap_p,
                                             ecma_string_t *name_p,
                                             jmem_cpointer_t *property_real_name_cp);
#endif /* JERRY_PROPERTY_HASHMAP */

/**
 * @}
 * @}
 */

#endif /* !ECMA_PROPERTY_HASHMAP_H */
