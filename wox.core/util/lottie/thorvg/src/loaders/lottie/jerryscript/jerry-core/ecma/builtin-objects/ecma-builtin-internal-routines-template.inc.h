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

#ifndef BUILTIN_UNDERSCORED_ID
#error "Please, define BUILTIN_UNDERSCORED_ID"
#endif /* !BUILTIN_UNDERSCORED_ID */

#ifndef BUILTIN_INC_HEADER_NAME
#error "Please, define BUILTIN_INC_HEADER_NAME"
#endif /* !BUILTIN_INC_HEADER_NAME */

#include "ecma-objects.h"

#define PASTE__(x, y) x##y
#define PASTE_(x, y)  PASTE__ (x, y)
#define PASTE(x, y)   PASTE_ (x, y)

#define PROPERTY_DESCRIPTOR_LIST_NAME PASTE (PASTE (ecma_builtin_, BUILTIN_UNDERSCORED_ID), _property_descriptor_list)
#define DISPATCH_ROUTINE_ROUTINE_NAME PASTE (PASTE (ecma_builtin_, BUILTIN_UNDERSCORED_ID), _dispatch_routine)

#ifndef BUILTIN_CUSTOM_DISPATCH

#define ROUTINE_ARG(n)             , ecma_value_t arg##n
#define ROUTINE_ARG_LIST_0         ecma_value_t this_arg
#define ROUTINE_ARG_LIST_1         ROUTINE_ARG_LIST_0 ROUTINE_ARG (1)
#define ROUTINE_ARG_LIST_2         ROUTINE_ARG_LIST_1 ROUTINE_ARG (2)
#define ROUTINE_ARG_LIST_3         ROUTINE_ARG_LIST_2 ROUTINE_ARG (3)
#define ROUTINE_ARG_LIST_NON_FIXED ROUTINE_ARG_LIST_0, const ecma_value_t *arguments_list_p, uint32_t arguments_list_len
#define ROUTINE(name, c_function_name, args_number, length_prop_value) \
  static ecma_value_t c_function_name (ROUTINE_ARG_LIST_##args_number);
#define ROUTINE_CONFIGURABLE_ONLY(name, c_function_name, args_number, length_prop_value) \
  static ecma_value_t c_function_name (ROUTINE_ARG_LIST_##args_number);
#define ROUTINE_WITH_FLAGS(name, c_function_name, args_number, length_prop_value, flags) \
  static ecma_value_t c_function_name (ROUTINE_ARG_LIST_##args_number);
#define ACCESSOR_READ_WRITE(name, c_getter_func_name, c_setter_func_name, prop_attributes) \
  static ecma_value_t c_getter_func_name (ROUTINE_ARG_LIST_0);                             \
  static ecma_value_t c_setter_func_name (ROUTINE_ARG_LIST_1);
#define ACCESSOR_READ_ONLY(name, c_getter_func_name, prop_attributes) \
  static ecma_value_t c_getter_func_name (ROUTINE_ARG_LIST_0);
#include BUILTIN_INC_HEADER_NAME
#undef ROUTINE_ARG_LIST_NON_FIXED
#undef ROUTINE_ARG_LIST_3
#undef ROUTINE_ARG_LIST_2
#undef ROUTINE_ARG_LIST_1
#undef ROUTINE_ARG_LIST_0
#undef ROUTINE_ARG

/**
 * List of built-in routine identifiers.
 */
enum
{
  PASTE (ECMA_ROUTINE_START_, BUILTIN_UNDERSCORED_ID) = 0,
#define ROUTINE(name, c_function_name, args_number, length_prop_value) ECMA_ROUTINE_##name##c_function_name,
#define ROUTINE_CONFIGURABLE_ONLY(name, c_function_name, args_number, length_prop_value) \
  ECMA_ROUTINE_##name##c_function_name,
#define ROUTINE_WITH_FLAGS(name, c_function_name, args_number, length_prop_value, flags) \
  ECMA_ROUTINE_##name##c_function_name,
#define ACCESSOR_READ_WRITE(name, c_getter_func_name, c_setter_func_name, prop_attributes) \
  ECMA_ACCESSOR_##name##c_getter_func_name, ECMA_ACCESSOR_##name##c_setter_func_name,
#define ACCESSOR_READ_ONLY(name, c_getter_func_name, prop_attributes) ECMA_ACCESSOR_##name##c_getter_func_name,
#include BUILTIN_INC_HEADER_NAME
};

#endif /* !BUILTIN_CUSTOM_DISPATCH */

/**
 * Built-in property list of the built-in object.
 */
const ecma_builtin_property_descriptor_t PROPERTY_DESCRIPTOR_LIST_NAME[] = {
#ifndef BUILTIN_CUSTOM_DISPATCH
#define ROUTINE(name, c_function_name, args_number, length_prop_value) \
  { name,                                                              \
    ECMA_BUILTIN_PROPERTY_ROUTINE,                                     \
    ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_WRITABLE,                      \
    ECMA_ROUTINE_VALUE (ECMA_ROUTINE_##name##c_function_name, length_prop_value) },
#define ROUTINE_CONFIGURABLE_ONLY(name, c_function_name, args_number, length_prop_value) \
  { name,                                                                                \
    ECMA_BUILTIN_PROPERTY_ROUTINE,                                                       \
    ECMA_PROPERTY_BUILT_IN_CONFIGURABLE,                                                 \
    ECMA_ROUTINE_VALUE (ECMA_ROUTINE_##name##c_function_name, length_prop_value) },
#define ROUTINE_WITH_FLAGS(name, c_function_name, args_number, length_prop_value, prop_attributes) \
  { name,                                                                                          \
    ECMA_BUILTIN_PROPERTY_ROUTINE,                                                                 \
    (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN,                                               \
    ECMA_ROUTINE_VALUE (ECMA_ROUTINE_##name##c_function_name, length_prop_value) },
#define ACCESSOR_READ_ONLY(name, c_getter_func_name, prop_attributes) \
  { name,                                                             \
    ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_ONLY,                         \
    (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN,                  \
    ECMA_ACCESSOR_##name##c_getter_func_name },
#define ACCESSOR_READ_WRITE(name, c_getter_func_name, c_setter_func_name, prop_attributes) \
  { name,                                                                                  \
    ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_WRITE,                                             \
    (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN,                                       \
    ECMA_ACCESSOR_READ_WRITE (ECMA_ACCESSOR_##name##c_getter_func_name, ECMA_ACCESSOR_##name##c_setter_func_name) },
#else /* BUILTIN_CUSTOM_DISPATCH */
#define ROUTINE(name, c_function_name, args_number, length_prop_value) \
  { name,                                                              \
    ECMA_BUILTIN_PROPERTY_ROUTINE,                                     \
    ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_WRITABLE,                      \
    ECMA_ROUTINE_VALUE (c_function_name, length_prop_value) },
#define ROUTINE_CONFIGURABLE_ONLY(name, c_function_name, args_number, length_prop_value) \
  { name,                                                                                \
    ECMA_BUILTIN_PROPERTY_ROUTINE,                                                       \
    ECMA_PROPERTY_BUILT_IN_CONFIGURABLE,                                                 \
    ECMA_ROUTINE_VALUE (c_function_name, length_prop_value) },
#define ROUTINE_WITH_FLAGS(name, c_function_name, args_number, length_prop_value, prop_attributes) \
  { name,                                                                                          \
    ECMA_BUILTIN_PROPERTY_ROUTINE,                                                                 \
    (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN,                                               \
    ECMA_ROUTINE_VALUE (c_function_name, length_prop_value) },
#define ACCESSOR_READ_ONLY(name, c_getter_func_name, prop_attributes) \
  { name,                                                             \
    ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_ONLY,                         \
    (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN,                  \
    c_getter_func_name },
#define ACCESSOR_READ_WRITE(name, c_getter_func_name, c_setter_func_name, prop_attributes) \
  { name,                                                                                  \
    ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_WRITE,                                             \
    (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN,                                       \
    ECMA_ACCESSOR_READ_WRITE (c_getter_func_name, c_setter_func_name) },
#endif /* !BUILTIN_CUSTOM_DISPATCH */
#define OBJECT_VALUE(name, obj_builtin_id, prop_attributes) \
  { name, ECMA_BUILTIN_PROPERTY_OBJECT, (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN, obj_builtin_id },
#define SIMPLE_VALUE(name, simple_value, prop_attributes) \
  { name, ECMA_BUILTIN_PROPERTY_SIMPLE, (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN, simple_value },
#define NUMBER_VALUE(name, number_value, prop_attributes) \
  { name, ECMA_BUILTIN_PROPERTY_NUMBER, (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN, number_value },
#define STRING_VALUE(name, magic_string_id, prop_attributes) \
  { name, ECMA_BUILTIN_PROPERTY_STRING, (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN, magic_string_id },
#define SYMBOL_VALUE(name, symbol) { name, ECMA_BUILTIN_PROPERTY_SYMBOL, ECMA_PROPERTY_BUILT_IN_FIXED, symbol },
#define INTRINSIC_PROPERTY(name, magic_string_id, prop_attributes) \
  { name, ECMA_BUILTIN_PROPERTY_INTRINSIC_PROPERTY, (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN, magic_string_id },
#define ACCESSOR_BUILTIN_FUNCTION(name, getter_builtin_id, setter_builtin_id, prop_attributes) \
  { name,                                                                                      \
    ECMA_BUILTIN_PROPERTY_ACCESSOR_BUILTIN_FUNCTION,                                           \
    (prop_attributes) | ECMA_PROPERTY_FLAG_BUILT_IN,                                           \
    ECMA_ACCESSOR_READ_WRITE (getter_builtin_id, setter_builtin_id) },
#include BUILTIN_INC_HEADER_NAME
  { LIT_MAGIC_STRING__COUNT, ECMA_BUILTIN_PROPERTY_END, 0, 0 }
};

#ifndef BUILTIN_CUSTOM_DISPATCH

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
DISPATCH_ROUTINE_ROUTINE_NAME (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                identifier */
                               ecma_value_t this_arg_value, /**< 'this' argument
                                                                 value */
                               const ecma_value_t arguments_list[], /**< list of arguments
                                                                         passed to routine */
                               uint32_t arguments_number) /**< length of
                                                           *   arguments' list */
{
  /* the arguments may be unused for some built-ins */
  JERRY_UNUSED (this_arg_value);
  JERRY_UNUSED (arguments_list);
  JERRY_UNUSED (arguments_number);

  switch (builtin_routine_id)
  {
#define ROUTINE_ARG(n) (arguments_list[n - 1])
#define ROUTINE_ARG_LIST_0
#define ROUTINE_ARG_LIST_1         , ROUTINE_ARG (1)
#define ROUTINE_ARG_LIST_2         ROUTINE_ARG_LIST_1, ROUTINE_ARG (2)
#define ROUTINE_ARG_LIST_3         ROUTINE_ARG_LIST_2, ROUTINE_ARG (3)
#define ROUTINE_ARG_LIST_NON_FIXED , arguments_list, arguments_number
#define ROUTINE(name, c_function_name, args_number, length_prop_value)      \
  case ECMA_ROUTINE_##name##c_function_name:                                \
  {                                                                         \
    return c_function_name (this_arg_value ROUTINE_ARG_LIST_##args_number); \
  }
#define ROUTINE_CONFIGURABLE_ONLY(name, c_function_name, args_number, length_prop_value) \
  case ECMA_ROUTINE_##name##c_function_name:                                             \
  {                                                                                      \
    return c_function_name (this_arg_value ROUTINE_ARG_LIST_##args_number);              \
  }
#define ROUTINE_WITH_FLAGS(name, c_function_name, args_number, length_prop_value, flags) \
  case ECMA_ROUTINE_##name##c_function_name:                                             \
  {                                                                                      \
    return c_function_name (this_arg_value ROUTINE_ARG_LIST_##args_number);              \
  }
#define ACCESSOR_READ_WRITE(name, c_getter_func_name, c_setter_func_name, prop_attributes) \
  case ECMA_ACCESSOR_##name##c_getter_func_name:                                           \
  {                                                                                        \
    return c_getter_func_name (this_arg_value);                                            \
  }                                                                                        \
  case ECMA_ACCESSOR_##name##c_setter_func_name:                                           \
  {                                                                                        \
    return c_setter_func_name (this_arg_value ROUTINE_ARG_LIST_1);                         \
  }
#define ACCESSOR_READ_ONLY(name, c_getter_func_name, prop_attributes) \
  case ECMA_ACCESSOR_##name##c_getter_func_name:                      \
  {                                                                   \
    return c_getter_func_name (this_arg_value);                       \
  }
#include BUILTIN_INC_HEADER_NAME
#undef ROUTINE_ARG
#undef ROUTINE_ARG_LIST_0
#undef ROUTINE_ARG_LIST_1
#undef ROUTINE_ARG_LIST_2
#undef ROUTINE_ARG_LIST_3
#undef ROUTINE_ARG_LIST_NON_FIXED

    default:
    {
      JERRY_UNREACHABLE ();
    }
  }
} /* DISPATCH_ROUTINE_ROUTINE_NAME */

#endif /* !BUILTIN_CUSTOM_DISPATCH */

#undef BUILTIN_INC_HEADER_NAME
#undef BUILTIN_CUSTOM_DISPATCH
#undef BUILTIN_UNDERSCORED_ID
#undef DISPATCH_ROUTINE_ROUTINE_NAME
#undef ECMA_BUILTIN_PROPERTY_NAME_INDEX
#undef PASTE__
#undef PASTE_
#undef PASTE
#undef PROPERTY_DESCRIPTOR_LIST_NAME
