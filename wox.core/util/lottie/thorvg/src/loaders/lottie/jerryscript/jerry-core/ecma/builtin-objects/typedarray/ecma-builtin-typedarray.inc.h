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
 * %TypedArray% description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_TYPEDARRAY

/* ES2015 22.2.2 */
/* ES11 22.2.1.1 - value of length changed to 0 */
NUMBER_VALUE (LIT_MAGIC_STRING_LENGTH, 0, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* ES2015 22.2.2 */
STRING_VALUE (LIT_MAGIC_STRING_NAME, LIT_MAGIC_STRING_TYPED_ARRAY_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* ES2015 22.2.2.3 */
OBJECT_VALUE (LIT_MAGIC_STRING_PROTOTYPE, ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE, ECMA_PROPERTY_FIXED)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */

/* ES2015 22.2.2.1 */
ROUTINE (LIT_MAGIC_STRING_FROM, ecma_builtin_typedarray_from, NON_FIXED, 1)

/* ES2015 22.2.2.2 */
ROUTINE (LIT_MAGIC_STRING_OF, ecma_builtin_typedarray_of, NON_FIXED, 0)

/* ES2015 22.2.2.4 */
ACCESSOR_READ_ONLY (LIT_GLOBAL_SYMBOL_SPECIES, ecma_builtin_typedarray_species_get, ECMA_PROPERTY_FLAG_CONFIGURABLE)

#endif /* JERRY_BUILTIN_TYPEDARRAY */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
