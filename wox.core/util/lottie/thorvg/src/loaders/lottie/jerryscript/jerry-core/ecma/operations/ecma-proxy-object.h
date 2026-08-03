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

#ifndef ECMA_PROXY_OBJECT_H
#define ECMA_PROXY_OBJECT_H

#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaproxyobject ECMA Proxy object related routines
 * @{
 */

#if JERRY_BUILTIN_PROXY

ecma_object_t *ecma_proxy_create (ecma_value_t target, ecma_value_t handler, uint32_t options);

ecma_object_t *ecma_proxy_create_revocable (ecma_value_t target, ecma_value_t handler);

ecma_value_t
ecma_proxy_revoke_cb (ecma_object_t *function_obj_p, const ecma_value_t args_p[], const uint32_t args_count);

ecma_value_t ecma_proxy_object_find (ecma_object_t *obj_p, ecma_string_t *prop_name_p);

/* Internal operations */

ecma_value_t ecma_proxy_object_get_prototype_of (ecma_object_t *obj_p);

ecma_value_t ecma_proxy_object_set_prototype_of (ecma_object_t *obj_p, ecma_value_t proto);

ecma_value_t ecma_proxy_object_is_extensible (ecma_object_t *obj_p);

ecma_value_t ecma_proxy_object_prevent_extensions (ecma_object_t *obj_p);

ecma_value_t ecma_proxy_object_get_own_property_descriptor (ecma_object_t *obj_p,
                                                            ecma_string_t *prop_name_p,
                                                            ecma_property_descriptor_t *prop_desc_p);

ecma_value_t ecma_proxy_object_define_own_property (ecma_object_t *obj_p,
                                                    ecma_string_t *prop_name_p,
                                                    const ecma_property_descriptor_t *prop_desc_p);

ecma_value_t ecma_proxy_object_has (ecma_object_t *obj_p, ecma_string_t *prop_name_p);

ecma_value_t ecma_proxy_object_get (ecma_object_t *obj_p, ecma_string_t *prop_name_p, ecma_value_t receiver);

ecma_value_t ecma_proxy_object_set (ecma_object_t *obj_p,
                                    ecma_string_t *prop_name_p,
                                    ecma_value_t name,
                                    ecma_value_t receiver,
                                    bool is_strict);

ecma_value_t ecma_proxy_object_delete_property (ecma_object_t *obj_p, ecma_string_t *prop_name_p, bool is_strict);

ecma_collection_t *ecma_proxy_object_own_property_keys (ecma_object_t *obj_p);

ecma_value_t
ecma_proxy_object_call (ecma_object_t *obj_p, ecma_value_t this_argument, const ecma_value_t *args_p, uint32_t argc);

ecma_value_t ecma_proxy_object_construct (ecma_object_t *obj_p,
                                          ecma_object_t *new_target_p,
                                          const ecma_value_t *args_p,
                                          uint32_t argc);

#endif /* JERRY_BUILTIN_PROXY */

/**
 * @}
 * @}
 */

#endif /* !ECMA_PROXY_OBJECT_H */
