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
 * Set.prototype built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_CONTAINER

/* Object properties:
 *  (property name, object pointer getter) */

/* ECMA-262 v6, 23.2.3.2 */
OBJECT_VALUE (LIT_MAGIC_STRING_CONSTRUCTOR, ECMA_BUILTIN_ID_SET, ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/* ECMA-262 v6, 23.1.3.13 */
STRING_VALUE (LIT_GLOBAL_SYMBOL_TO_STRING_TAG, LIT_MAGIC_STRING_SET_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_CLEAR, ECMA_CONTAINER_ROUTINE_CLEAR, 0, 0)
ROUTINE (LIT_MAGIC_STRING_ADD, ECMA_CONTAINER_ROUTINE_ADD, 1, 1)
ROUTINE (LIT_MAGIC_STRING_DELETE, ECMA_CONTAINER_ROUTINE_DELETE, 1, 1)
ROUTINE (LIT_MAGIC_STRING_FOR_EACH_UL, ECMA_CONTAINER_ROUTINE_FOREACH, 2, 1)
ROUTINE (LIT_MAGIC_STRING_HAS, ECMA_CONTAINER_ROUTINE_HAS, 1, 1)
ROUTINE (LIT_MAGIC_STRING_ENTRIES, ECMA_CONTAINER_ROUTINE_ENTRIES, 0, 0)
INTRINSIC_PROPERTY (LIT_MAGIC_STRING_VALUES,
                    LIT_INTERNAL_MAGIC_STRING_SET_PROTOTYPE_VALUES,
                    ECMA_PROPERTY_CONFIGURABLE_WRITABLE)
INTRINSIC_PROPERTY (LIT_GLOBAL_SYMBOL_ITERATOR,
                    LIT_INTERNAL_MAGIC_STRING_SET_PROTOTYPE_VALUES,
                    ECMA_PROPERTY_CONFIGURABLE_WRITABLE)
INTRINSIC_PROPERTY (LIT_MAGIC_STRING_KEYS,
                    LIT_INTERNAL_MAGIC_STRING_SET_PROTOTYPE_VALUES,
                    ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

ACCESSOR_READ_ONLY (LIT_MAGIC_STRING_SIZE, ECMA_CONTAINER_ROUTINE_SIZE_GETTER, ECMA_PROPERTY_FLAG_CONFIGURABLE)

#endif /* JERRY_BUILTIN_CONTAINER */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
