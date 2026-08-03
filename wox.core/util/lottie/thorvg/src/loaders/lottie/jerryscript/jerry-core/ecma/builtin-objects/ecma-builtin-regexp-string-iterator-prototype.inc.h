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
 * %RegExpStringIteratorPrototype% built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#ifdef JERRY_BUILTIN_REGEXP

STRING_VALUE (LIT_GLOBAL_SYMBOL_TO_STRING_TAG,
              LIT_MAGIC_STRING_REGEXP_STRING_ITERATOR_UL,
              ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_NEXT, ecma_builtin_regexp_string_iterator_prototype_object_next, 0, 0)

#endif

#include "ecma-builtin-helpers-macro-undefs.inc.h"
