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
 * Proxy object built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

#if JERRY_BUILTIN_PROXY

/* Number properties:
 *  (property name, number value, writable, enumerable, configurable) */

NUMBER_VALUE (LIT_MAGIC_STRING_LENGTH, 2, ECMA_PROPERTY_FLAG_CONFIGURABLE)

STRING_VALUE (LIT_MAGIC_STRING_NAME, LIT_MAGIC_STRING_PROXY_UL, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */

ROUTINE (LIT_MAGIC_STRING_REVOCABLE, ECMA_BUILTIN_PROXY_OBJECT_REVOCABLE, 2, 2)

#endif /* JERRY_BUILTIN_PROXY */

#include "ecma-builtin-helpers-macro-undefs.inc.h"
