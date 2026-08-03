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

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_REFLECT

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_APPLY, ECMA_REFLECT_OBJECT_APPLY, 3, 3)
ROUTINE (LIT_MAGIC_STRING_DEFINE_PROPERTY_UL, ECMA_REFLECT_OBJECT_DEFINE_PROPERTY, 3, 3)
ROUTINE (LIT_MAGIC_STRING_GET_OWN_PROPERTY_DESCRIPTOR_UL, ECMA_REFLECT_OBJECT_GET_OWN_PROPERTY_DESCRIPTOR, 2, 2)
ROUTINE (LIT_MAGIC_STRING_GET, ECMA_REFLECT_OBJECT_GET, NON_FIXED, 2)
ROUTINE (LIT_MAGIC_STRING_SET, ECMA_REFLECT_OBJECT_SET, NON_FIXED, 3)
ROUTINE (LIT_MAGIC_STRING_HAS, ECMA_REFLECT_OBJECT_HAS, 2, 2)
ROUTINE (LIT_MAGIC_STRING_DELETE_PROPERTY_UL, ECMA_REFLECT_OBJECT_DELETE_PROPERTY, 2, 2)
ROUTINE (LIT_MAGIC_STRING_OWN_KEYS_UL, ECMA_REFLECT_OBJECT_OWN_KEYS, NON_FIXED, 1)
ROUTINE (LIT_MAGIC_STRING_CONSTRUCT, ECMA_REFLECT_OBJECT_CONSTRUCT, NON_FIXED, 2)
ROUTINE (LIT_MAGIC_STRING_GET_PROTOTYPE_OF_UL, ECMA_REFLECT_OBJECT_GET_PROTOTYPE_OF, 1, 1)
ROUTINE (LIT_MAGIC_STRING_IS_EXTENSIBLE, ECMA_REFLECT_OBJECT_IS_EXTENSIBLE, 1, 1)
ROUTINE (LIT_MAGIC_STRING_PREVENT_EXTENSIONS_UL, ECMA_REFLECT_OBJECT_PREVENT_EXTENSIONS, 1, 1)
ROUTINE (LIT_MAGIC_STRING_SET_PROTOTYPE_OF_UL, ECMA_REFLECT_OBJECT_SET_PROTOTYPE_OF, 2, 2)

#endif /* JERRY_BUILTIN_REFLECT */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
