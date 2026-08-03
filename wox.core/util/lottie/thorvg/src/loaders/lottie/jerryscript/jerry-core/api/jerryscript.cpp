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

#include "jerryscript.h"

#include <math.h>

#include "ecma-eval.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-init-finalize.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"
#include "jcontext.h"

/** \addtogroup jerry Jerry engine interface
 * @{
 */


/**
 * Turn on API availability
 *
 * @return void
 */
static inline void
jerry_api_enable (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#ifndef JERRY_NDEBUG
  JERRY_CONTEXT (status_flags) |= ECMA_STATUS_API_ENABLED;
#endif /* JERRY_NDEBUG */
} /* jerry_make_api_available */

/**
 * Turn off API availability
 *
 * @return void
 */
static inline void
jerry_api_disable (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#ifndef JERRY_NDEBUG
  JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_API_ENABLED;
#endif /* JERRY_NDEBUG */
} /* jerry_make_api_unavailable */

/**
 * Create an API compatible return value.
 *
 * @return return value for Jerry API functions
 */
static jerry_value_t
jerry_return (const jerry_value_t value) /**< return value */
{
  if (ECMA_IS_VALUE_ERROR (value))
  {
    return ecma_create_exception_from_context ();
  }

  return value;
} /* jerry_return */

/**
 * Jerry engine initialization
 */
void
jerry_init (jerry_init_flag_t flags) /**< combination of Jerry flags */
{
#if JERRY_EXTERNAL_CONTEXT
  size_t total_size = jerry_port_context_alloc (sizeof (jerry_context_t));
  JERRY_UNUSED (total_size);
#endif /* JERRY_EXTERNAL_CONTEXT */

  JERRY_DEFINE_CURRENT_CONTEXT ();
  jerry_context_t *context_p = &JERRY_CONTEXT_STRUCT;
  memset (context_p, 0, sizeof (jerry_context_t));

#if JERRY_EXTERNAL_CONTEXT && !JERRY_SYSTEM_ALLOCATOR
  uint32_t heap_start_offset = JERRY_ALIGNUP (sizeof (jerry_context_t), JMEM_ALIGNMENT);
  uint8_t *heap_p = ((uint8_t *) context_p) + heap_start_offset;
  uint32_t heap_size = JERRY_ALIGNDOWN (total_size - heap_start_offset, JMEM_ALIGNMENT);

  JERRY_ASSERT (heap_p + heap_size <= ((uint8_t *) context_p) + total_size);

  context_p->heap_p = (jmem_heap_t *) heap_p;
  context_p->heap_size = heap_size;
#endif /* JERRY_EXTERNAL_CONTEXT && !JERRY_SYSTEM_ALLOCATOR */

  JERRY_CONTEXT (jerry_init_flags) = flags;

  jerry_api_enable ();

  jmem_init ();
  ecma_init ();
} /* jerry_init */

/**
 * Terminate Jerry engine
 */
void
jerry_cleanup (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  for (jerry_context_data_header_t *this_p = JERRY_CONTEXT (context_data_p); this_p != NULL; this_p = this_p->next_p)
  {
    if (this_p->manager_p->deinit_cb)
    {
      void *data = (this_p->manager_p->bytes_needed > 0) ? JERRY_CONTEXT_DATA_HEADER_USER_DATA (this_p) : NULL;
      this_p->manager_p->deinit_cb (data);
    }
  }

  ecma_free_all_enqueued_jobs ();
  ecma_finalize ();
  jerry_api_disable ();

  for (jerry_context_data_header_t *this_p = JERRY_CONTEXT (context_data_p), *next_p = NULL; this_p != NULL;
       this_p = next_p)
  {
    next_p = this_p->next_p;

    if (this_p->manager_p->finalize_cb)
    {
      void *data = (this_p->manager_p->bytes_needed > 0) ? JERRY_CONTEXT_DATA_HEADER_USER_DATA (this_p) : NULL;
      this_p->manager_p->finalize_cb (data);
    }

    jmem_heap_free_block (this_p, sizeof (jerry_context_data_header_t) + this_p->manager_p->bytes_needed);
  }

  jmem_finalize ();
#if JERRY_EXTERNAL_CONTEXT
  jerry_port_context_free ();
#endif /* JERRY_EXTERNAL_CONTEXT */
} /* jerry_cleanup */

/**
 * Perform eval
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return result of eval, may be error value.
 */
jerry_value_t
jerry_eval (const jerry_char_t *source_p, /**< source code */
            size_t source_size, /**< length of source code */
            uint32_t flags) /**< jerry_parse_opts_t flags */
{
  parser_source_char_t source_char;
  source_char.source_p = source_p;
  source_char.source_size = source_size;

  return jerry_return (ecma_op_eval_chars_buffer ((void *) &source_char, flags));
} /* jerry_eval */

/**
 * Get global object
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return api value of global object
 */
jerry_value_t
jerry_current_realm (void)
{
  ecma_object_t *global_obj_p = ecma_builtin_get_global ();
  ecma_ref_object (global_obj_p);
  return ecma_make_object_value (global_obj_p);
} /* jerry_current_realm */

/**
 * Check if the specified value is an error or abort value.
 *
 * @return true  - if the specified value is an error value,
 *         false - otherwise
 */
bool
jerry_value_is_exception (const jerry_value_t value) /**< api value */
{
  return ecma_is_value_exception (value);
} /* jerry_value_is_exception */

/**
 * Check if the specified value is number.
 *
 * @return true  - if the specified value is number,
 *         false - otherwise
 */
bool
jerry_value_is_number (const jerry_value_t value) /**< api value */
{
  return ecma_is_value_number (value);
} /* jerry_value_is_number */

/**
 * Check if the specified value is object.
 *
 * @return true  - if the specified value is object,
 *         false - otherwise
 */
bool
jerry_value_is_object (const jerry_value_t value) /**< api value */
{
  return ecma_is_value_object (value);
} /* jerry_value_is_object */

/**
 * Check if the specified value is string.
 *
 * @return true  - if the specified value is string,
 *         false - otherwise
 */
bool
jerry_value_is_string (const jerry_value_t value) /**< api value */
{
  return ecma_is_value_string (value);
} /* jerry_value_is_string */

/**
 * Check if the specified value is undefined.
 *
 * @return true  - if the specified value is undefined,
 *         false - otherwise
 */
bool
jerry_value_is_undefined (const jerry_value_t value) /**< api value */
{
  return ecma_is_value_undefined (value);
} /* jerry_value_is_undefined */

/**
 * Get number from the specified value as a double.
 *
 * @return stored number as double
 */
float
jerry_value_as_number (const jerry_value_t value) /**< api value */
{
  return (float) ecma_get_number_from_value (value);
} /* jerry_value_as_number */

/**
 * Call ToObject operation on the api value.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return converted object value - if success
 *         thrown error - otherwise
 */
jerry_value_t
jerry_value_to_object (const jerry_value_t value) /**< input value */
{
  return jerry_return (ecma_op_to_object (value));
} /* jerry_value_to_object */

/**
 * Call the ToString ecma builtin operation on the api value.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return converted string value - if success
 *         thrown error - otherwise
 */
jerry_value_t
jerry_value_to_string (const jerry_value_t value) /**< input value */
{
  ecma_string_t *str_p = ecma_op_to_string (value);
  return ecma_make_string_value (str_p);
} /* jerry_value_to_string */

/**
 * Convert any number to int32 number.
 *
 * Note:
 *      For non-number values 0 is returned.
 *
 * @return int32 representation of the number.
 */
int32_t
jerry_value_as_int32 (const jerry_value_t value) /**< input value */
{
  return ecma_number_to_int32 (ecma_get_number_from_value (value));
} /* jerry_value_as_int32 */

/**
 * Convert any number to uint32 number.
 *
 * Note:
 *      For non-number values 0 is returned.
 *
 * @return uint32 representation of the number.
 */
uint32_t
jerry_value_as_uint32 (const jerry_value_t value) /**< input value */
{
  return ecma_number_to_uint32 (ecma_get_number_from_value (value));
} /* jerry_value_as_uint32 */

/**
 * Release ownership of the argument value
 */
void
jerry_value_free (jerry_value_t value) /**< value */
{
  ecma_free_value (value);
} /* jerry_value_free */

/**
 * Create a jerry_value_t representing a boolean value from the given boolean parameter.
 *
 * @return value of the created boolean
 */
jerry_value_t
jerry_boolean (bool value) /**< bool value from which a jerry_value_t will be created */
{
  return ecma_make_boolean_value (value);
} /* jerry_boolean */

/**
 * Create an external function object
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return value of the constructed function object
 */
jerry_value_t
jerry_function_external (jerry_external_handler_t handler) /**< native handler
                                                            *   for the function */
{
  ecma_object_t *func_obj_p = ecma_op_create_external_function_object (handler);
  return ecma_make_object_value (func_obj_p);
} /* jerry_function_external */

/**
 * Creates a jerry_value_t representing a number value.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return jerry_value_t created from the given double argument.
 */
jerry_value_t
jerry_number (float value) /**< double value from which a jerry_value_t will be created */
{
  return ecma_make_number_value ((ecma_number_t) value);
} /* jerry_number */

/**
 * Creates a jerry_value_t representing an undefined value.
 *
 * @return value of undefined
 */
jerry_value_t
jerry_undefined (void)
{
  return ECMA_VALUE_UNDEFINED;
} /* jerry_undefined */

/**
 * Create new JavaScript object, like with new Object().
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return value of the created object
 */
jerry_value_t
jerry_object (void)
{
  return ecma_make_object_value (ecma_op_create_object_object_noarg ());
} /* jerry_object */

/**
 * Create string value from the input zero-terminated ASCII string.
 *
 * @return created string
 */
jerry_value_t
jerry_string_sz (const char *str_p) /**< pointer to string */
{
  const jerry_char_t *data_p = (const jerry_char_t *) str_p;
  return jerry_string (data_p, lit_zt_utf8_string_size (data_p), JERRY_ENCODING_CESU8);
} /* jerry_string_sz */

/**
 * Create a string value from the input buffer using the specified encoding.
 * The content of the buffer is assumed to be valid in the specified encoding, it's the callers responsibility to
 * validate the input.
 *
 * See also: jerry_validate_string
 *
 * @return created string
 */
jerry_value_t
jerry_string (const jerry_char_t *buffer_p, /**< pointer to buffer */
              jerry_size_t buffer_size, /**< buffer size */
              jerry_encoding_t encoding) /**< buffer encoding */
{
  ecma_string_t *ecma_str_p = NULL;
  //JERRY_ASSERT (jerry_validate_string (buffer_p, buffer_size, encoding));

  switch (encoding)
  {
    case JERRY_ENCODING_CESU8:
    {
      ecma_str_p = ecma_new_ecma_string_from_utf8 (buffer_p, buffer_size);
      break;
    }
    case JERRY_ENCODING_UTF8:
    {
      ecma_str_p = ecma_new_ecma_string_from_utf8_converted_to_cesu8 (buffer_p, buffer_size);
      break;
    }
    default:
    {
      return jerry_undefined();
    }
  }

  return ecma_make_string_value (ecma_str_p);
} /* jerry_string */

/**
 * Get length of a string value
 *
 * @return number of characters in the string
 *         0 - if value is not a string
 */
jerry_length_t
jerry_string_length (const jerry_value_t value) /**< input string */
{
  return ecma_string_get_length (ecma_get_string_from_value (value));
} /* jerry_string_length */

/**
 * Copy the characters of a string into the specified buffer using the specified encoding.  The string is truncated to
 * fit the buffer. If the value is not a string, nothing will be copied to the buffer.
 *
 * @return number of bytes copied to the buffer
 */
jerry_size_t
jerry_string_to_buffer (const jerry_value_t value, /**< input string value */
                        jerry_encoding_t encoding, /**< output encoding */
                        jerry_char_t *buffer_p, /**< [out] output characters buffer */
                        jerry_size_t buffer_size) /**< size of output buffer */
{
  ecma_string_t *str_p = ecma_get_string_from_value (value);
  return ecma_string_copy_to_buffer (str_p, (lit_utf8_byte_t *) buffer_p, buffer_size, encoding);
} /* jerry_string_to_char_buffer */

/**
 * Get value of a property to the specified object with the given name.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return value of the property - if success
 *         value marked with error flag - otherwise
 */
jerry_value_t
jerry_object_get (const jerry_value_t object, /**< object value */
                  const jerry_value_t key) /**< property name (string value) */
{
  jerry_value_t ret_value = ecma_op_object_get (ecma_get_object_from_value (object), ecma_get_prop_name_from_value (key));
  return jerry_return (ret_value);
} /* jerry_object_get */

/**
 * Get value of a property to the specified object with the given name.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return value of the property - if success
 *         value marked with error flag - otherwise
 */
jerry_value_t
jerry_object_get_sz (const jerry_value_t object, /**< object value */
                     const char *key_p) /**< property key */
{
  jerry_value_t key_str = jerry_string_sz (key_p);
  jerry_value_t result = jerry_object_get (object, key_str);
  ecma_free_value (key_str);
  return result;
} /* jerry_object_get */

/**
 * Get value by an index from the specified object.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return value of the property specified by the index - if success
 *         value marked with error flag - otherwise
 */
jerry_value_t
jerry_object_get_index (const jerry_value_t object, /**< object value */
                        uint32_t index) /**< index to be written */
{
  ecma_value_t ret_value = ecma_op_object_get_by_index (ecma_get_object_from_value (object), index);
  return jerry_return (ret_value);
} /* jerry_object_get_index */

/**
 * Set a property to the specified object with the given name.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return true value - if the operation was successful
 *         value marked with error flag - otherwise
 */
jerry_value_t
jerry_object_set (jerry_value_t object, /**< object value */
                  const jerry_value_t key, /**< property name (string value) */
                  const jerry_value_t value) /**< value to set */
{
  return jerry_return (ecma_op_object_put (ecma_get_object_from_value (object), ecma_get_prop_name_from_value (key), value, true));
} /* jerry_object_set */

/**
 * Set a property to the specified object with the given name.
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return true value - if the operation was successful
 *         value marked with error flag - otherwise
 */
jerry_value_t
jerry_object_set_sz (jerry_value_t object, /**< object value */
                     const char *key_p, /**< property key */
                     const jerry_value_t value) /**< value to set */
{
  jerry_value_t key_str = jerry_string_sz (key_p);
  jerry_value_t result = jerry_object_set (object, key_str, value);
  ecma_free_value (key_str);
  return result;
} /* jerry_object_set */

/**
 * Set indexed value in the specified object
 *
 * Note:
 *      returned value must be freed with jerry_value_free, when it is no longer needed.
 *
 * @return true value - if the operation was successful
 *         value marked with error flag - otherwise
 */
jerry_value_t
jerry_object_set_index (jerry_value_t object, /**< object value */
                        uint32_t index, /**< index to be written */
                        const jerry_value_t value) /**< value to set */
{
  ecma_value_t ret_value = ecma_op_object_put_by_index (ecma_get_object_from_value (object), index, value, true);
  return jerry_return (ret_value);
} /* jerry_object_set_index */

/**
 * Get native pointer and its type information, associated with the given native type info.
 *
 * Note:
 *  If native pointer is present, its type information is returned in out_native_pointer_p
 *
 * @return found native pointer,
 *         or NULL
 */
void *
jerry_object_get_native_ptr (const jerry_value_t object, /**< object to get native pointer from */
                             const jerry_object_native_info_t *native_info_p) /**< the type info
                                                                               *   of the native pointer */
{
  if (!ecma_is_value_object (object))
  {
    return NULL;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (object);
  ecma_native_pointer_t *native_pointer_p = ecma_get_native_pointer_value (obj_p, (jerry_object_native_info_t *) native_info_p);

  if (native_pointer_p == NULL)
  {
    return NULL;
  }

  return native_pointer_p->native_p;
} /* jerry_object_get_native_ptr */

/**
 * Set native pointer and an optional type info for the specified object.
 *
 *
 * Note:
 *      If native pointer was already set for the object, its value is updated.
 *
 * Note:
 *      If a non-NULL free callback is specified in the native type info,
 *      it will be called by the garbage collector when the object is freed.
 *      Referred values by this method must have at least 1 reference. (Correct API usage satisfies this condition)
 *      The type info always overwrites the previous value, so passing
 *      a NULL value deletes the current type info.
 */
void
jerry_object_set_native_ptr (jerry_value_t object, /**< object to set native pointer in */
                             const jerry_object_native_info_t *native_info_p, /**< object's native type info */
                             void *native_pointer_p) /**< native pointer */
{
  if (ecma_is_value_object (object))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (object);

    ecma_create_native_pointer_property (object_p, native_pointer_p, native_info_p);
  }
} /* jerry_object_set_native_ptr */

/**
 * Register external magic string array
 */
void
jerry_register_magic_strings (const jerry_char_t *const *ext_strings_p, /**< character arrays, representing
                                                                         *   external magic strings' contents */
                              uint32_t count, /**< number of the strings */
                              const jerry_length_t *str_lengths_p) /**< lengths of all strings */
{
  lit_magic_strings_ex_set ((const lit_utf8_byte_t *const *) ext_strings_p,
                            count,
                            (const lit_utf8_size_t *) str_lengths_p);
} /* jerry_register_magic_strings */


/**
 * @}
 */
