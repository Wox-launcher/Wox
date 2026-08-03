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
 * Promise built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

/* Number properties:
 *  (property name, number value, writable, enumerable, configurable) */

NUMBER_VALUE (LIT_MAGIC_STRING_LENGTH, 1, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Object properties:
 *  (property name, object pointer getter) */

OBJECT_VALUE (LIT_MAGIC_STRING_PROTOTYPE, ECMA_BUILTIN_ID_PROMISE_PROTOTYPE, ECMA_PROPERTY_FIXED)

STRING_VALUE (LIT_MAGIC_STRING_NAME, LIT_MAGIC_STRING_PROMISE_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_REJECT, ECMA_PROMISE_ROUTINE_REJECT, 1, 1)
ROUTINE (LIT_MAGIC_STRING_RESOLVE, ECMA_PROMISE_ROUTINE_RESOLVE, 1, 1)
ROUTINE (LIT_MAGIC_STRING_RACE, ECMA_PROMISE_ROUTINE_RACE, 1, 1)
ROUTINE (LIT_MAGIC_STRING_ALL, ECMA_PROMISE_ROUTINE_ALL, 1, 1)
ROUTINE (LIT_MAGIC_STRING_ALLSETTLED, ECMA_PROMISE_ROUTINE_ALLSETTLED, 1, 1)
ROUTINE (LIT_MAGIC_STRING_ANY, ECMA_PROMISE_ROUTINE_ANY, 1, 1)

/* ES2015 25.4.4.6 */
ACCESSOR_READ_ONLY (LIT_GLOBAL_SYMBOL_SPECIES, ECMA_PROMISE_ROUTINE_SPECIES_GET, ECMA_PROPERTY_FLAG_CONFIGURABLE)

#include "ecma-builtin-helpers-macro-undefs.inc.h"
