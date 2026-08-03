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
 * Atomics built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_ATOMICS

/* ECMA-262 v11, 24.4.14 */
STRING_VALUE (LIT_GLOBAL_SYMBOL_TO_STRING_TAG, LIT_MAGIC_STRING_ATOMICS_U, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_ADD, ECMA_ATOMICS_ROUTINE_ADD, 3, 3)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_AND, ECMA_ATOMICS_ROUTINE_AND, 3, 3)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_COMPAREEXCHANGE, ECMA_ATOMICS_ROUTINE_COMPAREEXCHANGE, 4, 4)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_EXCHANGE, ECMA_ATOMICS_ROUTINE_EXCHANGE, 3, 3)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_ISLOCKFREE, ECMA_ATOMICS_ROUTINE_ISLOCKFREE, 1, 1)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_LOAD, ECMA_ATOMICS_ROUTINE_LOAD, 2, 2)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_OR, ECMA_ATOMICS_ROUTINE_OR, 3, 3)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_STORE, ECMA_ATOMICS_ROUTINE_STORE, 3, 3)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_SUB, ECMA_ATOMICS_ROUTINE_SUB, 3, 3)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_WAIT, ECMA_ATOMICS_ROUTINE_WAIT, 4, 4)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_NOTIFY, ECMA_ATOMICS_ROUTINE_NOTIFY, 3, 3)
ROUTINE (LIT_MAGIC_STRING_ATOMICS_XOR, ECMA_ATOMICS_ROUTINE_XOR, 3, 3)

#endif /* JERRY_BUILTIN_ATOMICS */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
