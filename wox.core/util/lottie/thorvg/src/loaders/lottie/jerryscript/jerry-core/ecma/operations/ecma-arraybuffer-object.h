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

#ifndef ECMA_ARRAYBUFFER_OBJECT_H
#define ECMA_ARRAYBUFFER_OBJECT_H

#include "ecma-globals.h"

#if JERRY_BUILTIN_TYPEDARRAY

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaarraybufferobject ECMA ArrayBuffer object related routines
 * @{
 */

/**
 * Get array buffer flags.
 */
#define ECMA_ARRAYBUFFER_GET_FLAGS(arraybuffer_p) \
  (((ecma_extended_object_t *) (arraybuffer_p))->u.cls.u1.array_buffer_flags)

/**
 * Check whether the backing store is allocated for an array buffer.
 */
#define ECMA_ARRAYBUFFER_LAZY_ALLOC(arraybuffer_p)                                             \
  (JERRY_UNLIKELY (!(ECMA_ARRAYBUFFER_GET_FLAGS (arraybuffer_p) & ECMA_ARRAYBUFFER_ALLOCATED)) \
   && ecma_arraybuffer_allocate_buffer_throw (arraybuffer_p) == ECMA_VALUE_ERROR)

ecma_value_t ecma_op_create_arraybuffer_object (const ecma_value_t *, uint32_t);

/**
 * Helper functions for arraybuffer.
 */
ecma_object_t *ecma_arraybuffer_create_object (uint8_t type, uint32_t length);
ecma_object_t *ecma_arraybuffer_create_object_with_buffer (uint8_t type, uint32_t length);
ecma_object_t *ecma_arraybuffer_new_object (uint32_t length);
ecma_value_t ecma_arraybuffer_allocate_buffer (ecma_object_t *arraybuffer_p);
ecma_value_t ecma_arraybuffer_allocate_buffer_throw (ecma_object_t *arraybuffer_p);
void ecma_arraybuffer_release_buffer (ecma_object_t *arraybuffer_p);
uint8_t *JERRY_ATTR_PURE ecma_arraybuffer_get_buffer (ecma_object_t *obj_p);
uint32_t JERRY_ATTR_PURE ecma_arraybuffer_get_length (ecma_object_t *obj_p);
bool JERRY_ATTR_PURE ecma_arraybuffer_is_detached (ecma_object_t *obj_p);
bool ecma_arraybuffer_detach (ecma_object_t *obj_p);
bool ecma_is_arraybuffer (ecma_value_t val);
ecma_value_t
ecma_builtin_arraybuffer_slice (ecma_value_t this_arg, const ecma_value_t *argument_list_p, uint32_t arguments_number);

/**
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_TYPEDARRAY */
#endif /* !ECMA_ARRAYBUFFER_OBJECT_H */
