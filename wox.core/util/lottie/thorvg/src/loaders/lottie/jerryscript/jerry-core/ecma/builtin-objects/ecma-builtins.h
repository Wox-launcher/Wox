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

#ifndef ECMA_BUILTINS_H
#define ECMA_BUILTINS_H

#include "ecma-globals.h"

/**
 * A built-in object's identifier
 */
typedef enum
{
/** @cond doxygen_suppress */
#define BUILTIN(a, b, c, d, e)
#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) builtin_id,
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
#define BUILTIN_ROUTINE(a, b, c, d, e)
#define BUILTIN(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) builtin_id,
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
  /** @endcond */
  ECMA_BUILTIN_ID__COUNT /**< number of built-in objects */
} ecma_builtin_id_t;

/**
 * Special id for handlers (handlers are not regular built-ins, but
 * they use the same ecma_built_in_props_t structure as other built-ins)
 */
#define ECMA_BUILTIN_ID_HANDLER ECMA_BUILTIN_ID__COUNT

/**
 * Number of global symbols
 */
#define ECMA_BUILTIN_GLOBAL_SYMBOL_COUNT (LIT_GLOBAL_SYMBOL__LAST - LIT_GLOBAL_SYMBOL__FIRST + 1)

/**
 * Construct a routine value
 */
#define ECMA_ROUTINE_VALUE(id, length) (((id) << 4) | length)

/**
 * Get routine length
 */
#define ECMA_GET_ROUTINE_LENGTH(value) ((uint8_t) ((value) &0xf))

/**
 * Get routine ID
 */
#define ECMA_GET_ROUTINE_ID(value) ((uint8_t) ((value) >> 4))

/**
 * Construct a fully accessor value
 */
#define ECMA_ACCESSOR_READ_WRITE(getter, setter) (((getter) << 8) | (setter))

/**
 * Get accessor setter ID
 */
#define ECMA_ACCESSOR_READ_WRITE_GET_SETTER_ID(value) ((uint8_t) ((value) &0xff))

/**
 * Get accessor getter ID
 */
#define ECMA_ACCESSOR_READ_WRITE_GET_GETTER_ID(value) ((uint8_t) ((value) >> 8))

/**
 * Number ob built-in objects excluding global object
 */
#define ECMA_BUILTIN_OBJECTS_COUNT (ECMA_BUILTIN_ID__COUNT - 1)

/**
 * Description of built-in global ECMA-object.
 */
typedef struct
{
  ecma_extended_object_t extended_object; /**< extended object part */
  uint32_t extra_instantiated_bitset[1]; /**< extra bit set for instantiated properties */
#if JERRY_BUILTIN_REALMS
  uint32_t extra_realms_bitset; /**< extra bit set for instantiated properties when realms is enabled */
  ecma_value_t this_binding; /**< 'this' binding of this global object */
#endif /* JERRY_BUILTIN_REALMS */
  jmem_cpointer_t global_env_cp; /**< global lexical environment */
  jmem_cpointer_t global_scope_cp; /**< global lexical scope */
  jmem_cpointer_t builtin_objects[ECMA_BUILTIN_OBJECTS_COUNT]; /**< pointer to instances of built-in objects */
} ecma_global_object_t;

/* ecma-builtins.c */

ecma_global_object_t *ecma_builtin_create_global_object (void);

ecma_value_t ecma_builtin_dispatch_call (ecma_object_t *obj_p,
                                         ecma_value_t this_arg_value,
                                         const ecma_value_t *arguments_list_p,
                                         uint32_t arguments_list_len);
ecma_value_t ecma_builtin_dispatch_construct (ecma_object_t *obj_p,
                                              const ecma_value_t *arguments_list_p,
                                              uint32_t arguments_list_len);
ecma_property_t *ecma_builtin_routine_try_to_instantiate_property (ecma_object_t *object_p,
                                                                   ecma_string_t *property_name_p);
ecma_property_t *ecma_builtin_try_to_instantiate_property (ecma_object_t *object_p, ecma_string_t *property_name_p);
void ecma_builtin_routine_delete_built_in_property (ecma_object_t *object_p, ecma_string_t *property_name_p);
void ecma_builtin_delete_built_in_property (ecma_object_t *object_p, ecma_string_t *property_name_p);
void ecma_builtin_routine_list_lazy_property_names (ecma_object_t *object_p,
                                                    ecma_collection_t *prop_names_p,
                                                    ecma_property_counter_t *prop_counter_p,
                                                    jerry_property_filter_t filter);
void ecma_builtin_list_lazy_property_names (ecma_object_t *object_p,
                                            ecma_collection_t *prop_names_p,
                                            ecma_property_counter_t *prop_counter_p,
                                            jerry_property_filter_t filter);
bool ecma_builtin_is_global (ecma_object_t *object_p);
ecma_object_t *ecma_builtin_get (ecma_builtin_id_t builtin_id);
ecma_object_t *ecma_builtin_get_global (void);
bool ecma_builtin_function_is_routine (ecma_object_t *func_obj_p);

#if JERRY_BUILTIN_REALMS
ecma_object_t *ecma_builtin_get_from_realm (ecma_global_object_t *global_object_p, ecma_builtin_id_t builtin_id);
#endif /* JERRY_BUILTIN_REALMS */

#endif /* !ECMA_BUILTINS_H */
