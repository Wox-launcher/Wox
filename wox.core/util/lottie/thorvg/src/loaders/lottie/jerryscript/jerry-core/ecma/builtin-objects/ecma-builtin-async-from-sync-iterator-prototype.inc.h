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
 * %AsyncFromSyncIteratorPrototype% built-in description (AsyncFunction.prototype)
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_NEXT, ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_NEXT, 1, 1)
ROUTINE (LIT_MAGIC_STRING_RETURN, ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_RETURN, 1, 1)
ROUTINE (LIT_MAGIC_STRING_THROW, ECMA_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE_ROUTINE_THROW, 1, 1)

#include "ecma-builtin-helpers-macro-undefs.inc.h"
