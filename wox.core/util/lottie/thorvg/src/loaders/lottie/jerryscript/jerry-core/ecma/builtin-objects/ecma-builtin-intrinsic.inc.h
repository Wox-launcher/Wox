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
 * Intrinsic built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_INTERNAL_MAGIC_STRING_ARRAY_PROTOTYPE_VALUES, ECMA_INTRINSIC_ARRAY_PROTOTYPE_VALUES, 0, 0)
ROUTINE (LIT_INTERNAL_MAGIC_STRING_TYPEDARRAY_PROTOTYPE_VALUES, ECMA_INTRINSIC_TYPEDARRAY_PROTOTYPE_VALUES, 0, 0)
ROUTINE (LIT_INTERNAL_MAGIC_STRING_SET_PROTOTYPE_VALUES, ECMA_INTRINSIC_SET_PROTOTYPE_VALUES, 0, 0)
ROUTINE (LIT_INTERNAL_MAGIC_STRING_MAP_PROTOTYPE_ENTRIES, ECMA_INTRINSIC_MAP_PROTOTYPE_ENTRIES, 0, 0)
ROUTINE (LIT_MAGIC_STRING_TRIM_START, ECMA_INTRINSIC_STRING_TRIM_START, 0, 0)
ROUTINE (LIT_MAGIC_STRING_TRIM_END, ECMA_INTRINSIC_STRING_TRIM_END, 0, 0)
ROUTINE (LIT_MAGIC_STRING_TO_STRING_UL, ECMA_INTRINSIC_ARRAY_TO_STRING, 0, 0)
ROUTINE (LIT_MAGIC_STRING_TO_UTC_STRING_UL, ECMA_INTRINSIC_DATE_TO_UTC_STRING, 0, 0)
ROUTINE (LIT_MAGIC_STRING_PARSE_FLOAT, ECMA_INTRINSIC_PARSE_FLOAT, 1, 1)
ROUTINE (LIT_MAGIC_STRING_PARSE_INT, ECMA_INTRINSIC_PARSE_INT, 2, 2)

#include "ecma-builtin-helpers-macro-undefs.inc.h"
