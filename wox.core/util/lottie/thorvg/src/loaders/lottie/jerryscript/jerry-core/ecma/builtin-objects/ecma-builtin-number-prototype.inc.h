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
 * Number.prototype built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_NUMBER

/* Object properties:
 *  (property name, object pointer getter) */

/* ECMA-262 v5, 15.7.4.1 */
OBJECT_VALUE (LIT_MAGIC_STRING_CONSTRUCTOR, ECMA_BUILTIN_ID_NUMBER, ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_TO_STRING_UL, ECMA_NUMBER_PROTOTYPE_TO_STRING, NON_FIXED, 1)
ROUTINE (LIT_MAGIC_STRING_VALUE_OF_UL, ECMA_NUMBER_PROTOTYPE_VALUE_OF, 0, 0)
ROUTINE (LIT_MAGIC_STRING_TO_LOCALE_STRING_UL, ECMA_NUMBER_PROTOTYPE_TO_LOCALE_STRING, 0, 0)
ROUTINE (LIT_MAGIC_STRING_TO_FIXED_UL, ECMA_NUMBER_PROTOTYPE_TO_FIXED, 1, 1)
ROUTINE (LIT_MAGIC_STRING_TO_EXPONENTIAL_UL, ECMA_NUMBER_PROTOTYPE_TO_EXPONENTIAL, 1, 1)
ROUTINE (LIT_MAGIC_STRING_TO_PRECISION_UL, ECMA_NUMBER_PROTOTYPE_TO_PRECISION, 1, 1)

#endif /* JERRY_BUILTIN_NUMBER */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
