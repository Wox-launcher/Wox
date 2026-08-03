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
 * Map.prototype built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_CONTAINER

/* Object properties:
 *  (property name, object pointer getter) */

/* ECMA-262 v6, 23.1.3.2 */
OBJECT_VALUE (LIT_MAGIC_STRING_CONSTRUCTOR, ECMA_BUILTIN_ID_MAP, ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/* ECMA-262 v6, 23.1.3.13 */
STRING_VALUE (LIT_GLOBAL_SYMBOL_TO_STRING_TAG, LIT_MAGIC_STRING_MAP_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_CLEAR, ECMA_CONTAINER_ROUTINE_CLEAR, 0, 0)
ROUTINE (LIT_MAGIC_STRING_DELETE, ECMA_CONTAINER_ROUTINE_DELETE, 1, 1)
ROUTINE (LIT_MAGIC_STRING_FOR_EACH_UL, ECMA_CONTAINER_ROUTINE_FOREACH, 2, 1)
ROUTINE (LIT_MAGIC_STRING_GET, ECMA_CONTAINER_ROUTINE_GET, 1, 1)
ROUTINE (LIT_MAGIC_STRING_HAS, ECMA_CONTAINER_ROUTINE_HAS, 1, 1)
ROUTINE (LIT_MAGIC_STRING_SET, ECMA_CONTAINER_ROUTINE_SET, 2, 2)
ROUTINE (LIT_MAGIC_STRING_VALUES, ECMA_CONTAINER_ROUTINE_VALUES, 0, 0)
ROUTINE (LIT_MAGIC_STRING_KEYS, ECMA_CONTAINER_ROUTINE_KEYS, 0, 0)
INTRINSIC_PROPERTY (LIT_MAGIC_STRING_ENTRIES,
                    LIT_INTERNAL_MAGIC_STRING_MAP_PROTOTYPE_ENTRIES,
                    ECMA_PROPERTY_CONFIGURABLE_WRITABLE)
INTRINSIC_PROPERTY (LIT_GLOBAL_SYMBOL_ITERATOR,
                    LIT_INTERNAL_MAGIC_STRING_MAP_PROTOTYPE_ENTRIES,
                    ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/* ECMA-262 v6, 23.1.3.10 */
ACCESSOR_READ_ONLY (LIT_MAGIC_STRING_SIZE, ECMA_CONTAINER_ROUTINE_SIZE_GETTER, ECMA_PROPERTY_FLAG_CONFIGURABLE)

#endif /* JERRY_BUILTIN_CONTAINER */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
