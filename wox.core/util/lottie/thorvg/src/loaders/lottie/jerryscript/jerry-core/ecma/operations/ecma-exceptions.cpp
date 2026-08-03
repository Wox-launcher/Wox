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

#include "ecma-exceptions.h"

#include <stdarg.h>

#include "ecma-array-object.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-objects.h"
#include "ecma-symbol-object.h"

#include "jcontext.h"
#include "jrt.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup exceptions Exceptions
 * @{
 */

/**
 * Standard ecma-error object constructor.
 *
 * Note:
 *    message_string_p can be NULL.
 *
 * Note:
 *    calling with JERRY_ERROR_NONE does not make sense thus it will
 *    cause a fault in the system.
 *
 * @return pointer to ecma-object representing specified error
 *         with reference counter set to one.
 */
ecma_object_t *
ecma_new_standard_error (jerry_error_t error_type, /**< native error type */
                         ecma_string_t *message_string_p) /**< message string */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if JERRY_BUILTIN_ERRORS
  ecma_builtin_id_t prototype_id = ECMA_BUILTIN_ID__COUNT;

  switch (error_type)
  {
    case JERRY_ERROR_EVAL:
    {
      prototype_id = ECMA_BUILTIN_ID_EVAL_ERROR_PROTOTYPE;
      break;
    }

    case JERRY_ERROR_RANGE:
    {
      prototype_id = ECMA_BUILTIN_ID_RANGE_ERROR_PROTOTYPE;
      break;
    }

    case JERRY_ERROR_REFERENCE:
    {
      prototype_id = ECMA_BUILTIN_ID_REFERENCE_ERROR_PROTOTYPE;
      break;
    }

    case JERRY_ERROR_TYPE:
    {
      prototype_id = ECMA_BUILTIN_ID_TYPE_ERROR_PROTOTYPE;
      break;
    }

    case JERRY_ERROR_AGGREGATE:
    {
      prototype_id = ECMA_BUILTIN_ID_AGGREGATE_ERROR_PROTOTYPE;
      break;
    }

    case JERRY_ERROR_URI:
    {
      prototype_id = ECMA_BUILTIN_ID_URI_ERROR_PROTOTYPE;
      break;
    }

    case JERRY_ERROR_SYNTAX:
    {
      prototype_id = ECMA_BUILTIN_ID_SYNTAX_ERROR_PROTOTYPE;
      break;
    }

    default:
    {
      JERRY_ASSERT (error_type == JERRY_ERROR_COMMON);

      prototype_id = ECMA_BUILTIN_ID_ERROR_PROTOTYPE;
      break;
    }
  }
#else /* !JERRY_BUILTIN_ERRORS */
  JERRY_UNUSED (error_type);
  ecma_builtin_id_t prototype_id = ECMA_BUILTIN_ID_ERROR_PROTOTYPE;
#endif /* JERRY_BUILTIN_ERRORS */

  ecma_object_t *prototype_obj_p = ecma_builtin_get (prototype_id);

  ecma_object_t *error_object_p =
    ecma_create_object (prototype_obj_p, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *extended_object_p = (ecma_extended_object_t *) error_object_p;
  extended_object_p->u.cls.type = ECMA_OBJECT_CLASS_ERROR;
  extended_object_p->u.cls.u1.error_type = (uint8_t) error_type;

  if (message_string_p != NULL)
  {
    ecma_property_value_t *prop_value_p;
    prop_value_p = ecma_create_named_data_property (error_object_p,
                                                    ecma_get_magic_string (LIT_MAGIC_STRING_MESSAGE),
                                                    ECMA_PROPERTY_CONFIGURABLE_WRITABLE,
                                                    NULL);

    ecma_ref_ecma_string (message_string_p);
    prop_value_p->value = ecma_make_string_value (message_string_p);
  }

  /* Avoid calling the decorator function recursively. */
  if (JERRY_CONTEXT (error_object_created_callback_p) != NULL
      && !(JERRY_CONTEXT (status_flags) & ECMA_STATUS_ERROR_UPDATE))
  {
    JERRY_CONTEXT (status_flags) |= ECMA_STATUS_ERROR_UPDATE;
    JERRY_CONTEXT (error_object_created_callback_p)
    (ecma_make_object_value (error_object_p), JERRY_CONTEXT (error_object_created_callback_user_p));
    JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_ERROR_UPDATE;
  }

  return error_object_p;
} /* ecma_new_standard_error */

/**
 * aggregate-error object constructor.
 *
 * @return newly constructed aggregate errors
 */
ecma_value_t
ecma_new_aggregate_error (ecma_value_t error_list_val, /**< errors list */
                          ecma_value_t message_val) /**< message string */
{
  ecma_object_t *new_error_object_p;

  if (!ecma_is_value_undefined (message_val))
  {
    ecma_string_t *message_string_p = ecma_op_to_string (message_val);

    if (JERRY_UNLIKELY (message_string_p == NULL))
    {
      return ECMA_VALUE_ERROR;
    }

    new_error_object_p = ecma_new_standard_error (JERRY_ERROR_AGGREGATE, message_string_p);
    ecma_deref_ecma_string (message_string_p);
  }
  else
  {
    new_error_object_p = ecma_new_standard_error (JERRY_ERROR_AGGREGATE, NULL);
  }

  ecma_value_t using_iterator = ecma_op_get_method_by_symbol_id (error_list_val, LIT_GLOBAL_SYMBOL_ITERATOR);

  if (ECMA_IS_VALUE_ERROR (using_iterator))
  {
    ecma_deref_object (new_error_object_p);
    return using_iterator;
  }

  if (!ecma_is_value_undefined (using_iterator))
  {
    ecma_value_t next_method;
    ecma_value_t iterator = ecma_op_get_iterator (error_list_val, using_iterator, &next_method);
    ecma_free_value (using_iterator);

    if (ECMA_IS_VALUE_ERROR (iterator))
    {
      ecma_deref_object (new_error_object_p);
      return iterator;
    }

    ecma_collection_t *error_list_p = ecma_new_collection ();
    ecma_value_t result = ECMA_VALUE_ERROR;

    while (true)
    {
      ecma_value_t next = ecma_op_iterator_step (iterator, next_method);

      if (ECMA_IS_VALUE_ERROR (next))
      {
        break;
      }

      if (next == ECMA_VALUE_FALSE)
      {
        result = ECMA_VALUE_UNDEFINED;
        break;
      }

      /* 8.e.iii */
      ecma_value_t next_error = ecma_op_iterator_value (next);
      ecma_free_value (next);

      if (ECMA_IS_VALUE_ERROR (next_error))
      {
        break;
      }

      ecma_collection_push_back (error_list_p, next_error);
    }

    ecma_free_value (iterator);
    ecma_free_value (next_method);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_collection_free (error_list_p);
      ecma_deref_object (new_error_object_p);
      return result;
    }

    JERRY_ASSERT (ecma_is_value_undefined (result));

    ecma_value_t error_list_arr = ecma_op_new_array_object_from_collection (error_list_p, true);
    ecma_property_value_t *prop_value_p;
    prop_value_p = ecma_create_named_data_property (new_error_object_p,
                                                    ecma_get_magic_string (LIT_MAGIC_STRING_ERRORS_UL),
                                                    ECMA_PROPERTY_CONFIGURABLE_WRITABLE,
                                                    NULL);
    prop_value_p->value = error_list_arr;
    ecma_free_value (error_list_arr);
  }

  return ecma_make_object_value (new_error_object_p);
} /* ecma_new_aggregate_error */

/**
 * Return the error type for an Error object.
 *
 * @return one of the jerry_error_t value
 *         if it is not an Error object then JERRY_ERROR_NONE will be returned
 */
jerry_error_t
ecma_get_error_type (ecma_object_t *error_object_p) /**< possible error object */
{
  if (!ecma_object_class_is (error_object_p, ECMA_OBJECT_CLASS_ERROR))
  {
    return JERRY_ERROR_NONE;
  }

  return (jerry_error_t) ((ecma_extended_object_t *) error_object_p)->u.cls.u1.error_type;
} /* ecma_get_error_type */

/**
 * Raise a standard ecma-error with the given type and message.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_standard_error (jerry_error_t error_type, /**< error type */
                           ecma_error_msg_t msg) /**< error message */
{
  ecma_object_t *error_obj_p;
  const lit_utf8_byte_t *str_p = (lit_utf8_byte_t *) ecma_get_error_msg (msg);

  if (msg != ECMA_ERR_EMPTY)
  {
    ecma_string_t *error_msg_p = ecma_new_ecma_external_string_from_cesu8 (str_p, ecma_get_error_size (msg), NULL);
    error_obj_p = ecma_new_standard_error (error_type, error_msg_p);
    ecma_deref_ecma_string (error_msg_p);
  }
  else
  {
    error_obj_p = ecma_new_standard_error (error_type, NULL);
  }

  jcontext_raise_exception (ecma_make_object_value (error_obj_p));
  return ECMA_VALUE_ERROR;
} /* ecma_raise_standard_error */

#if JERRY_ERROR_MESSAGES

/**
 * Raise a standard ecma-error with the given format string and arguments.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_standard_error_with_format (jerry_error_t error_type, /**< error type */
                                       const char *format, /**< format string */
                                       ...) /**< ecma-values */
{
  JERRY_ASSERT (format != NULL);

  ecma_stringbuilder_t builder = ecma_stringbuilder_create ();

  const char *start_p = format;
  const char *end_p = format;

  va_list args;

  va_start (args, format);

  while (*end_p)
  {
    if (*end_p == '%')
    {
      /* Concat template string. */
      if (end_p > start_p)
      {
        ecma_stringbuilder_append_raw (&builder, (lit_utf8_byte_t *) start_p, (lit_utf8_size_t) (end_p - start_p));
      }

      /* Convert an argument to string without side effects. */
      ecma_string_t *arg_string_p;
      const ecma_value_t arg_val = va_arg (args, ecma_value_t);

      if (JERRY_UNLIKELY (ecma_is_value_object (arg_val)))
      {
        ecma_object_t *arg_object_p = ecma_get_object_from_value (arg_val);
        lit_magic_string_id_t class_name = ecma_object_get_class_name (arg_object_p);
        arg_string_p = ecma_get_magic_string (class_name);
      }
      else if (ecma_is_value_symbol (arg_val))
      {
        ecma_value_t symbol_desc_value = ecma_get_symbol_descriptive_string (arg_val);
        arg_string_p = ecma_get_string_from_value (symbol_desc_value);
      }
      else
      {
        arg_string_p = ecma_op_to_string (arg_val);
        JERRY_ASSERT (arg_string_p != NULL);
      }

      /* Concat argument. */
      ecma_stringbuilder_append (&builder, arg_string_p);

      ecma_deref_ecma_string (arg_string_p);

      start_p = end_p + 1;
    }

    end_p++;
  }

  va_end (args);

  /* Concat reset of template string. */
  if (start_p < end_p)
  {
    ecma_stringbuilder_append_raw (&builder, (lit_utf8_byte_t *) start_p, (lit_utf8_size_t) (end_p - start_p));
  }

  ecma_string_t *builder_str_p = ecma_stringbuilder_finalize (&builder);

  ecma_object_t *error_obj_p = ecma_new_standard_error (error_type, builder_str_p);

  ecma_deref_ecma_string (builder_str_p);

  jcontext_raise_exception (ecma_make_object_value (error_obj_p));
  return ECMA_VALUE_ERROR;
} /* ecma_raise_standard_error_with_format */

#endif /* JERRY_ERROR_MESSAGES */

/**
 * Raise a common error with the given message.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_common_error (ecma_error_msg_t msg) /**< error message */
{
  return ecma_raise_standard_error (JERRY_ERROR_COMMON, msg);
} /* ecma_raise_common_error */

/**
 * Raise a RangeError with the given message.
 *
 * See also: ECMA-262 v5, 15.11.6.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_range_error (ecma_error_msg_t msg) /**< error message */
{
  return ecma_raise_standard_error (JERRY_ERROR_RANGE, msg);
} /* ecma_raise_range_error */

/**
 * Raise a ReferenceError with the given message.
 *
 * See also: ECMA-262 v5, 15.11.6.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_reference_error (ecma_error_msg_t msg) /**< error message */
{
  return ecma_raise_standard_error (JERRY_ERROR_REFERENCE, msg);
} /* ecma_raise_reference_error */

/**
 * Raise a SyntaxError with the given message.
 *
 * See also: ECMA-262 v5, 15.11.6.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_syntax_error (ecma_error_msg_t msg) /**< error message */
{
  return ecma_raise_standard_error (JERRY_ERROR_SYNTAX, msg);
} /* ecma_raise_syntax_error */

/**
 * Raise a TypeError with the given message.
 *
 * See also: ECMA-262 v5, 15.11.6.5
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_type_error (ecma_error_msg_t msg) /**< error message */
{
  return ecma_raise_standard_error (JERRY_ERROR_TYPE, msg);
} /* ecma_raise_type_error */

/**
 * Raise a URIError with the given message.
 *
 * See also: ECMA-262 v5, 15.11.6.6
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_uri_error (ecma_error_msg_t msg) /**< error message */
{
  return ecma_raise_standard_error (JERRY_ERROR_URI, msg);
} /* ecma_raise_uri_error */

#if (JERRY_STACK_LIMIT != 0)
/**
 * Raise a RangeError with "Maximum call stack size exceeded" message.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_maximum_callstack_error (void)
{
  return ecma_raise_range_error (ECMA_ERR_MAXIMUM_CALL_STACK_SIZE_EXCEEDED);
} /* ecma_raise_maximum_callstack_error */
#endif /* (JERRY_STACK_LIMIT != 0) */

/**
 * Raise a AggregateError with the given errors and message.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_raise_aggregate_error (ecma_value_t error_list_val, /**< errors list */
                            ecma_value_t message_val) /**< error message */
{
  ecma_value_t aggre_val = ecma_new_aggregate_error (error_list_val, message_val);
  jcontext_raise_exception (aggre_val);

  return ECMA_VALUE_ERROR;
} /* ecma_raise_aggregate_error */

/**
 * @}
 * @}
 */
