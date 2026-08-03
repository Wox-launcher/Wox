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
 * Function.prototype built-in description
 */

#include "ecma-builtin-helpers-macro-defines.inc.h"

/* Object properties:
 *  (property name, object pointer getter) */

/* ECMA-262 v5, 15.3.4.1 */
OBJECT_VALUE (LIT_MAGIC_STRING_CONSTRUCTOR, ECMA_BUILTIN_ID_FUNCTION, ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/* Number properties:
 *  (property name, object pointer getter) */

NUMBER_VALUE (LIT_MAGIC_STRING_LENGTH, 0, ECMA_PROPERTY_FLAG_DEFAULT_LENGTH)
STRING_VALUE (LIT_MAGIC_STRING_NAME, LIT_MAGIC_STRING__EMPTY, ECMA_PROPERTY_FLAG_CONFIGURABLE)

/* Routine properties:
 *  (property name, C routine name, arguments number or NON_FIXED, value of the routine's length property) */
ROUTINE (LIT_MAGIC_STRING_TO_STRING_UL, ECMA_FUNCTION_PROTOTYPE_TO_STRING, 0, 0)
ROUTINE (LIT_MAGIC_STRING_APPLY, ECMA_FUNCTION_PROTOTYPE_APPLY, 2, 2)
ROUTINE (LIT_MAGIC_STRING_CALL, ECMA_FUNCTION_PROTOTYPE_CALL, NON_FIXED, 1)
ROUTINE (LIT_MAGIC_STRING_BIND, ECMA_FUNCTION_PROTOTYPE_BIND, NON_FIXED, 1)

/**
 * ECMA-262 v6.0 19.2.3.6 @@hasInstance
 *  the property attributes are: { [[Writable]]: false, [[Enumerable]]: false, [[Configurable]]: false }.
 */
ROUTINE_WITH_FLAGS (LIT_GLOBAL_SYMBOL_HAS_INSTANCE, ECMA_FUNCTION_PROTOTYPE_SYMBOL_HAS_INSTANCE, 1, 1, 0 /* flags */)
ACCESSOR_BUILTIN_FUNCTION (LIT_MAGIC_STRING_ARGUMENTS,
                           ECMA_BUILTIN_ID_TYPE_ERROR_THROWER,
                           ECMA_BUILTIN_ID_TYPE_ERROR_THROWER,
                           ECMA_PROPERTY_FLAG_CONFIGURABLE)
ACCESSOR_BUILTIN_FUNCTION (LIT_MAGIC_STRING_CALLER,
                           ECMA_BUILTIN_ID_TYPE_ERROR_THROWER,
                           ECMA_BUILTIN_ID_TYPE_ERROR_THROWER,
                           ECMA_PROPERTY_FLAG_CONFIGURABLE)

#include "ecma-builtin-helpers-macro-undefs.inc.h"
