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
 * Symbol built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

/* Number properties:
 *  (property name, number value, writable, enumerable, configurable) */

/* ECMA-262 v6, 19.4.2 */
NUMBER_VALUE (LIT_MAGIC_STRING_LENGTH, 0, ECMA_PROPERTY_FLAG_CONFIGURABLE)

STRING_VALUE (LIT_MAGIC_STRING_NAME, LIT_MAGIC_STRING_SYMBOL_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Object properties:
 *  (property name, object pointer getter) */

/* ECMA-262 v6, 19.4.2.7 */
OBJECT_VALUE (LIT_MAGIC_STRING_PROTOTYPE, ECMA_BUILTIN_ID_SYMBOL_PROTOTYPE, ECMA_PROPERTY_FIXED)

/* ECMA-262 v10, 19.4.2.1 */
SYMBOL_VALUE (LIT_MAGIC_STRING_ASYNC_ITERATOR, LIT_GLOBAL_SYMBOL_ASYNC_ITERATOR)

/* ECMA-262 v6, 19.4.2.2 */
SYMBOL_VALUE (LIT_MAGIC_STRING_HAS_INSTANCE, LIT_GLOBAL_SYMBOL_HAS_INSTANCE)

/* ECMA-262 v6, 19.4.2.3 */
SYMBOL_VALUE (LIT_MAGIC_STRING_IS_CONCAT_SPREADABLE, LIT_GLOBAL_SYMBOL_IS_CONCAT_SPREADABLE)

/* ECMA-262 v6, 19.4.2.4 */
SYMBOL_VALUE (LIT_MAGIC_STRING_ITERATOR, LIT_GLOBAL_SYMBOL_ITERATOR)

/* ECMA-262 v6, 19.4.2.6 */
SYMBOL_VALUE (LIT_MAGIC_STRING_MATCH, LIT_GLOBAL_SYMBOL_MATCH)

/* ECMA-262 v6, 19.4.2.8 */
SYMBOL_VALUE (LIT_MAGIC_STRING_REPLACE, LIT_GLOBAL_SYMBOL_REPLACE)

/* ECMA-262 v6, 19.4.2.9 */
SYMBOL_VALUE (LIT_MAGIC_STRING_SEARCH, LIT_GLOBAL_SYMBOL_SEARCH)

/* ECMA-262 v6, 19.4.2.10 */
SYMBOL_VALUE (LIT_MAGIC_STRING_SPECIES, LIT_GLOBAL_SYMBOL_SPECIES)

/* ECMA-262 v6, 19.4.2.11 */
SYMBOL_VALUE (LIT_MAGIC_STRING_SPLIT, LIT_GLOBAL_SYMBOL_SPLIT)

/* ECMA-262 v6, 19.4.2.12 */
SYMBOL_VALUE (LIT_MAGIC_STRING_TO_PRIMITIVE, LIT_GLOBAL_SYMBOL_TO_PRIMITIVE)

/* ECMA-262 v6, 19.4.2.13 */
SYMBOL_VALUE (LIT_MAGIC_STRING_TO_STRING_TAG, LIT_GLOBAL_SYMBOL_TO_STRING_TAG)

/* ECMA-262 v6, 19.4.2.14 */
SYMBOL_VALUE (LIT_MAGIC_STRING_UNSCOPABLES, LIT_GLOBAL_SYMBOL_UNSCOPABLES)

/* ECMA-262 v11, 19.4.2.8 */
SYMBOL_VALUE (LIT_MAGIC_STRING_MATCH_ALL, LIT_GLOBAL_SYMBOL_MATCH_ALL)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_FOR, ecma_builtin_symbol_for, 1, 1)
ROUTINE (LIT_MAGIC_STRING_KEY_FOR, ecma_builtin_symbol_key_for, 1, 1)

#include "ecma-builtin-helpers-macro-undefs.inc.h"
