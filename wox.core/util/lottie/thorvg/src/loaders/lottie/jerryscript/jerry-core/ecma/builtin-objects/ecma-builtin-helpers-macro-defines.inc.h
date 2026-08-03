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

#ifndef SIMPLE_VALUE
#define SIMPLE_VALUE(name, simple_value, prop_attributes)
#endif /* !SIMPLE_VALUE */

#ifndef NUMBER_VALUE
#define NUMBER_VALUE(name, number_value, prop_attributes)
#endif /* !NUMBER_VALUE */

#ifndef STRING_VALUE
#define STRING_VALUE(name, magic_string_id, prop_attributes)
#endif /* !STRING_VALUE */

#ifndef SYMBOL_VALUE
#define SYMBOL_VALUE(symbol, desc_magic_string_id)
#endif /* !SYMBOL_VALUE */

#ifndef INTRINSIC_PROPERTY
#define INTRINSIC_PROPERTY(name, magic_string_id, prop_attributes)
#endif /* !INTRINSIC_PROPERTY */

#ifndef ACCESSOR_BUILTIN_FUNCTION_OBJECT
#define ACCESSOR_BUILTIN_FUNCTION_OBJECT(name, getter_builtin_id, setter_builtin_id, prop_attributes)
#endif /* !ACCESSOR_BUILTIN_FUNCTION_OBJECT */

#ifndef OBJECT_VALUE
#define OBJECT_VALUE(name, obj_builtin_id, prop_attributes)
#endif /* !OBJECT_VALUE */

#ifndef ROUTINE
#define ROUTINE(name, c_function_name, args_number, length_prop_value)
#endif /* !ROUTINE */

#ifndef ROUTINE_CONFIGURABLE_ONLY
#define ROUTINE_CONFIGURABLE_ONLY(name, c_function_name, args_number, length_prop_value)
#endif /* !ROUTINE_CONFIGURABLE_ONLY */

#ifndef ROUTINE_WITH_FLAGS
#define ROUTINE_WITH_FLAGS(name, c_function_name, args_number, length_prop_value, flags)
#endif /* !ROUTINE_WITH_FLAGS */

#ifndef ACCESSOR_READ_WRITE
#define ACCESSOR_READ_WRITE(name, c_getter_func_name, c_setter_func_name, prop_attributes)
#endif /* !ACCESSOR_READ_WRITE */

#ifndef ACCESSOR_READ_ONLY
#define ACCESSOR_READ_ONLY(name, c_getter_func_name, prop_attributes)
#endif /* !ACCESSOR_READ_ONLY */
