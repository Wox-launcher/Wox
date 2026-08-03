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

#if JERRY_BUILTIN_TYPEDARRAY

#ifndef TYPEDARRAY_BYTES_PER_ELEMENT
#error "Please define TYPEDARRAY_BYTES_PER_ELEMENT"
#endif /* !TYPEDARRAY_BYTES_PER_ELEMENT */

#ifndef TYPEDARRAY_BUILTIN_ID
#error "Please define TYPEDARRAY_BUILTIN_ID"
#endif /* !TYPEDARRAY_BUILTIN_ID */

#include "ecma-builtin-helpers-macro-defines.inc.h"

/* ES2015 22.2.3.4 */
OBJECT_VALUE (LIT_MAGIC_STRING_CONSTRUCTOR, TYPEDARRAY_BUILTIN_ID, ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/* ES2015 22.2.6.1 */
NUMBER_VALUE (LIT_MAGIC_STRING_BYTES_PER_ELEMENT_U, TYPEDARRAY_BYTES_PER_ELEMENT, ECMA_PROPERTY_FIXED)

#include "ecma-builtin-helpers-macro-undefs.inc.h"

#undef TYPEDARRAY_BUILTIN_ID
#undef TYPEDARRAY_BYTES_PER_ELEMENT

#endif /* JERRY_BUILTIN_TYPEDARRAY */
