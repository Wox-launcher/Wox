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

#ifndef ECMA_SHARED_ARRAYBUFFER_OBJECT_H
#define ECMA_SHARED_ARRAYBUFFER_OBJECT_H

#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmasharedarraybufferobject ECMA SharedArrayBuffer object related routines
 * @{
 */

#if JERRY_BUILTIN_SHAREDARRAYBUFFER

ecma_value_t ecma_op_create_shared_arraybuffer_object (const ecma_value_t *, uint32_t);

/**
 * Helper functions for SharedArrayBuffer.
 */
ecma_object_t *ecma_shared_arraybuffer_new_object (uint32_t length);
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */
bool ecma_is_shared_arraybuffer (ecma_value_t val);
bool ecma_object_is_shared_arraybuffer (ecma_object_t *val);

/**
 * @}
 * @}
 */

#endif /* !ECMA_SHARED_ARRAYBUFFER_OBJECT_H */
