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
 * Array description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_ARRAY

/* Object properties:
 *  (property name, object pointer getter) */

/* ECMA-262 v5, 15.4.3.1 */
OBJECT_VALUE (LIT_MAGIC_STRING_PROTOTYPE, ECMA_BUILTIN_ID_ARRAY_PROTOTYPE, ECMA_PROPERTY_FIXED)

/* Number properties:
 *  (property name, object pointer getter) */

NUMBER_VALUE (LIT_MAGIC_STRING_LENGTH, 1, ECMA_PROPERTY_FLAG_DEFAULT_LENGTH)
STRING_VALUE (LIT_MAGIC_STRING_NAME, LIT_MAGIC_STRING_ARRAY_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_IS_ARRAY_UL, ECMA_ARRAY_ROUTINE_IS_ARRAY, 1, 1)
ROUTINE (LIT_MAGIC_STRING_FROM, ECMA_ARRAY_ROUTINE_FROM, NON_FIXED, 1)
ROUTINE (LIT_MAGIC_STRING_OF, ECMA_ARRAY_ROUTINE_OF, NON_FIXED, 0)

/* ECMA-262 v6, 22.1.2.5 */
ACCESSOR_READ_ONLY (LIT_GLOBAL_SYMBOL_SPECIES, ECMA_ARRAY_ROUTINE_SPECIES_GET, ECMA_PROPERTY_FLAG_CONFIGURABLE)

#endif /* !(JERRY_BUILTIN_ARRAY) */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
