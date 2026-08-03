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
#ifndef ECMA_BUILTIN_OBJECT_H
#define ECMA_BUILTIN_OBJECT_H

#include "ecma-globals.h"

ecma_value_t ecma_builtin_object_object_get_prototype_of (ecma_object_t *obj_p);

ecma_value_t ecma_builtin_object_object_set_prototype_of (ecma_value_t arg1, ecma_value_t arg2);

ecma_value_t ecma_builtin_object_object_set_proto (ecma_value_t arg1, ecma_value_t arg2);

ecma_value_t ecma_builtin_object_object_prevent_extensions (ecma_object_t *obj_p);

ecma_value_t ecma_builtin_object_object_is_extensible (ecma_object_t *obj_p);

ecma_value_t ecma_builtin_object_object_get_own_property_descriptor (ecma_object_t *obj_p, ecma_string_t *name_str_p);
ecma_value_t
ecma_builtin_object_object_define_property (ecma_object_t *obj_p, ecma_string_t *name_str_p, ecma_value_t arg3);

#endif /* !ECMA_BUILTIN_OBJECT_H */
