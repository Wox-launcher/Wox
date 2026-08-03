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
 * Object.prototype built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

/* Object properties:
 *  (property name, object pointer getter) */

/* ECMA-262 v5, 15.2.4.1 */
OBJECT_VALUE (LIT_MAGIC_STRING_CONSTRUCTOR, ECMA_BUILTIN_ID_OBJECT, ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

ACCESSOR_READ_WRITE (LIT_MAGIC_STRING__PROTO__,
                     ECMA_OBJECT_PROTOTYPE_GET_PROTO,
                     ECMA_OBJECT_PROTOTYPE_SET_PROTO,
                     ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_TO_STRING_UL, ECMA_OBJECT_PROTOTYPE_TO_STRING, 0, 0)
ROUTINE (LIT_MAGIC_STRING_VALUE_OF_UL, ECMA_OBJECT_PROTOTYPE_VALUE_OF, 0, 0)
ROUTINE (LIT_MAGIC_STRING_TO_LOCALE_STRING_UL, ECMA_OBJECT_PROTOTYPE_TO_LOCALE_STRING, 0, 0)
ROUTINE (LIT_MAGIC_STRING_HAS_OWN_PROPERTY_UL, ECMA_OBJECT_PROTOTYPE_HAS_OWN_PROPERTY, 1, 1)
ROUTINE (LIT_MAGIC_STRING_IS_PROTOTYPE_OF_UL, ECMA_OBJECT_PROTOTYPE_IS_PROTOTYPE_OF, 1, 1)
ROUTINE (LIT_MAGIC_STRING_PROPERTY_IS_ENUMERABLE_UL, ECMA_OBJECT_PROTOTYPE_PROPERTY_IS_ENUMERABLE, 1, 1)

#if JERRY_BUILTIN_ANNEXB
ROUTINE (LIT_MAGIC_STRING_DEFINE_GETTER, ECMA_OBJECT_PROTOTYPE_DEFINE_GETTER, 2, 2)
ROUTINE (LIT_MAGIC_STRING_DEFINE_SETTER, ECMA_OBJECT_PROTOTYPE_DEFINE_SETTER, 2, 2)
ROUTINE (LIT_MAGIC_STRING_LOOKUP_GETTER, ECMA_OBJECT_PROTOTYPE_LOOKUP_GETTER, 1, 1)
ROUTINE (LIT_MAGIC_STRING_LOOKUP_SETTER, ECMA_OBJECT_PROTOTYPE_LOOKUP_SETTER, 1, 1)
#endif /* JERRY_BUILTIN_ANNEXB*/

#include "ecma-builtin-helpers-macro-undefs.inc.h"
