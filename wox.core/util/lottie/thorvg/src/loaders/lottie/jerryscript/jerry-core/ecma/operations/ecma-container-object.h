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

#ifndef ECMA_CONTAINER_OBJECT_H
#define ECMA_CONTAINER_OBJECT_H

#include "ecma-builtins.h"
#include "ecma-globals.h"

#if JERRY_BUILTIN_CONTAINER

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmamaphelpers ECMA builtin map/set helper functions
 * @{
 */

/**
 * List of built-in routine identifiers.
 */
enum
{
  ECMA_CONTAINER_ROUTINE_START = 0,
  ECMA_CONTAINER_ROUTINE_ADD,
  ECMA_CONTAINER_ROUTINE_CLEAR,
  ECMA_CONTAINER_ROUTINE_DELETE_WEAK,
  ECMA_CONTAINER_ROUTINE_DELETE,
  ECMA_CONTAINER_ROUTINE_FOREACH,
  ECMA_CONTAINER_ROUTINE_HAS,
  ECMA_CONTAINER_ROUTINE_SIZE_GETTER,
  ECMA_CONTAINER_ROUTINE_GET,
  ECMA_CONTAINER_ROUTINE_SET,
  /* Note: These 3 routines MUST be in this order */
  ECMA_CONTAINER_ROUTINE_KEYS,
  ECMA_CONTAINER_ROUTINE_VALUES,
  ECMA_CONTAINER_ROUTINE_ENTRIES
};

uint8_t ecma_op_container_entry_size (lit_magic_string_id_t lit_id);
ecma_extended_object_t *ecma_op_container_get_object (ecma_value_t this_arg, lit_magic_string_id_t lit_id);
ecma_value_t ecma_op_container_create (const ecma_value_t *arguments_list_p,
                                       uint32_t arguments_list_len,
                                       lit_magic_string_id_t lit_id,
                                       ecma_builtin_id_t proto_id);
ecma_value_t ecma_op_container_size (ecma_extended_object_t *map_object_p);
ecma_value_t
ecma_op_container_get (ecma_extended_object_t *map_object_p, ecma_value_t key_arg, lit_magic_string_id_t lit_id);
ecma_value_t ecma_op_container_foreach (ecma_extended_object_t *map_object_p,
                                        ecma_value_t predicate,
                                        ecma_value_t predicate_this_arg,
                                        lit_magic_string_id_t lit_id);
ecma_value_t
ecma_op_container_has (ecma_extended_object_t *map_object_p, ecma_value_t key_arg, lit_magic_string_id_t lit_id);
ecma_value_t ecma_op_container_set (ecma_extended_object_t *map_object_p,
                                    ecma_value_t key_arg,
                                    ecma_value_t value_arg,
                                    lit_magic_string_id_t lit_id);
ecma_value_t ecma_op_container_clear (ecma_extended_object_t *map_object_p);
ecma_value_t
ecma_op_container_delete (ecma_extended_object_t *map_object_p, ecma_value_t key_arg, lit_magic_string_id_t lit_id);
ecma_value_t ecma_op_container_delete_weak (ecma_extended_object_t *map_object_p,
                                            ecma_value_t key_arg,
                                            lit_magic_string_id_t lit_id);
ecma_value_t ecma_op_container_find_weak_value (ecma_object_t *object_p, ecma_value_t key_arg);
void ecma_op_container_remove_weak_entry (ecma_object_t *object_p, ecma_value_t key_arg);
void ecma_op_container_free_entries (ecma_object_t *object_p);
ecma_value_t ecma_op_container_create_iterator (ecma_value_t this_arg,
                                                ecma_builtin_id_t proto_id,
                                                ecma_object_class_type_t iterator_type,
                                                ecma_iterator_kind_t kind);
ecma_value_t ecma_op_container_iterator_next (ecma_value_t this_val, ecma_object_class_type_t iterator_type);
ecma_value_t ecma_builtin_container_dispatch_routine (uint16_t builtin_routine_id,
                                                      ecma_value_t this_arg,
                                                      const ecma_value_t arguments_list_p[],
                                                      lit_magic_string_id_t lit_id);

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_CONTAINER */

#endif /* !ECMA_CONTAINER_OBJECT_H */
