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

/*
 * ArrayBuffer.prototype built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_TYPEDARRAY

/* Object properties:
 *  (property name, object pointer getter) */

OBJECT_VALUE (LIT_MAGIC_STRING_CONSTRUCTOR, ECMA_BUILTIN_ID_ARRAYBUFFER, ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/* Readonly accessor properties */
ACCESSOR_READ_ONLY (LIT_MAGIC_STRING_BYTE_LENGTH_UL,
                    ecma_builtin_arraybuffer_prototype_bytelength_getter,
                    ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* ECMA-262 v6, 24.1.4.4 */
STRING_VALUE (LIT_GLOBAL_SYMBOL_TO_STRING_TAG, LIT_MAGIC_STRING_ARRAY_BUFFER_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_SLICE, ecma_builtin_arraybuffer_prototype_object_slice, NON_FIXED, 2)

#endif /* JERRY_BUILTIN_TYPEDARRAY */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
