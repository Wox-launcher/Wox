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

#ifndef JERRYSCRIPT_CORE_H
#define JERRYSCRIPT_CORE_H

#include "jerryscript-types.h"

JERRY_C_API_BEGIN

void jerry_init (jerry_init_flag_t flags);
void jerry_cleanup (void);
jerry_value_t jerry_current_realm (void);
jerry_value_t jerry_set_realm (jerry_value_t realm);
jerry_value_t jerry_eval (const jerry_char_t *source_p, size_t source_size, uint32_t flags);
jerry_value_t jerry_run (const jerry_value_t script);
bool jerry_value_is_undefined (const jerry_value_t value);
bool jerry_value_is_number (const jerry_value_t value);
bool jerry_value_is_object (const jerry_value_t value);
bool jerry_value_is_string (const jerry_value_t value);
bool jerry_value_is_exception (const jerry_value_t value);
jerry_value_t jerry_value_to_object (const jerry_value_t value);
jerry_value_t jerry_value_to_string (const jerry_value_t value);
float jerry_value_as_number (const jerry_value_t value);
int32_t jerry_value_as_int32 (const jerry_value_t value);
uint32_t jerry_value_as_uint32 (const jerry_value_t value);
jerry_value_t JERRY_ATTR_CONST jerry_undefined (void);
jerry_value_t JERRY_ATTR_CONST jerry_null (void);
jerry_value_t JERRY_ATTR_CONST jerry_boolean (bool value);
jerry_value_t jerry_number (float value);
jerry_value_t jerry_string (const jerry_char_t *buffer_p, jerry_size_t buffer_size, jerry_encoding_t encoding);
jerry_value_t jerry_string_sz (const char *str_p);
jerry_length_t jerry_string_length (const jerry_value_t value);
jerry_size_t jerry_string_to_buffer (const jerry_value_t value,
                                     jerry_encoding_t encoding,
                                     jerry_char_t *buffer_p,
                                     jerry_size_t buffer_size);
jerry_value_t jerry_object (void);
jerry_value_t jerry_object_set (jerry_value_t object, const jerry_value_t key, const jerry_value_t value);
jerry_value_t jerry_object_set_sz (jerry_value_t object, const char *key_p, const jerry_value_t value);
jerry_value_t jerry_object_set_index (jerry_value_t object, uint32_t index, const jerry_value_t value);
void jerry_object_set_native_ptr (jerry_value_t object,
                                  const jerry_object_native_info_t *native_info_p,
                                  void *native_pointer_p);
jerry_value_t jerry_object_get (const jerry_value_t object, const jerry_value_t key);
jerry_value_t jerry_object_get_sz (const jerry_value_t object, const char *key_p);
jerry_value_t jerry_object_get_index (const jerry_value_t object, uint32_t index);
void *jerry_object_get_native_ptr (const jerry_value_t object, const jerry_object_native_info_t *native_info_p);
jerry_value_t jerry_function_external (jerry_external_handler_t handler);
void jerry_value_free (jerry_value_t value);
void jerry_register_magic_strings (const jerry_char_t *const *ext_strings_p, uint32_t count, const jerry_length_t *str_lengths_p);

JERRY_C_API_END

#endif /* !JERRYSCRIPT_CORE_H */

/* vim: set fdm=marker fmr=@{,@}: */
