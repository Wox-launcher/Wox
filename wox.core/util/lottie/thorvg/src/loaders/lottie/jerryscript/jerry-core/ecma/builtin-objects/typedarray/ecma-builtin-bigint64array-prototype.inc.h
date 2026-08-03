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
 * BigInt64Array prototype description
 */

#if JERRY_BUILTIN_TYPEDARRAY
#if JERRY_BUILTIN_BIGINT

#define TYPEDARRAY_BYTES_PER_ELEMENT 8
#define TYPEDARRAY_BUILTIN_ID        ECMA_BUILTIN_ID_BIGINT64ARRAY
#include "ecma-builtin-typedarray-prototype-template.inc.h"

#endif /* JERRY_BUILTIN_BIGINT */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
