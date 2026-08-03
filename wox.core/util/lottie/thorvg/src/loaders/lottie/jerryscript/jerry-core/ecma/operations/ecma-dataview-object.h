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

#ifndef ECMA_DATAVIEW_OBJECT_H
#define ECMA_DATAVIEW_OBJECT_H

#include "ecma-globals.h"

#if JERRY_BUILTIN_DATAVIEW

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmadataviewobject ECMA builtin DataView helper functions
 * @{
 */

ecma_value_t ecma_op_dataview_create (const ecma_value_t *arguments_list_p, uint32_t arguments_list_len);
ecma_dataview_object_t *ecma_op_dataview_get_object (ecma_value_t this_arg);
ecma_value_t ecma_op_dataview_get_set_view_value (ecma_value_t view,
                                                  ecma_value_t request_index,
                                                  ecma_value_t little_endian,
                                                  ecma_value_t value_to_set,
                                                  ecma_typedarray_type_t id);
bool ecma_is_dataview (ecma_value_t value);

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_DATAVIEW */

#endif /* !ECMA_DATAVIEW_OBJECT_H */
