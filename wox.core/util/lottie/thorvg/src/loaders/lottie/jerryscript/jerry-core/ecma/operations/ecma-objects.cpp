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

#include "ecma-objects.h"

#include "ecma-arguments-object.h"
#include "ecma-array-object.h"
#include "ecma-bigint.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-lcache.h"
#include "ecma-lex-env.h"
#include "ecma-objects-general.h"
#include "ecma-proxy-object.h"
#include "ecma-string-object.h"

#include "jcontext.h"

#if JERRY_BUILTIN_TYPEDARRAY
#include "ecma-arraybuffer-object.h"
#include "ecma-typedarray-object.h"
#endif /* JERRY_BUILTIN_TYPEDARRAY */

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaobjectsinternalops ECMA objects' operations
 * @{
 */

/**
 * Hash bitmap size for ecma objects
 */
#define ECMA_OBJECT_HASH_BITMAP_SIZE 256

/**
 * Assert that specified object type value is valid
 *
 * @param type object's implementation-defined type
 */
#ifndef JERRY_NDEBUG
#define JERRY_ASSERT_OBJECT_TYPE_IS_VALID(type) JERRY_ASSERT (type < ECMA_OBJECT_TYPE__MAX);
#else /* JERRY_NDEBUG */
#define JERRY_ASSERT_OBJECT_TYPE_IS_VALID(type)
#endif /* !JERRY_NDEBUG */

/**
 * [[GetOwnProperty]] ecma object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *
 * @return pointer to a property - if it exists,
 *         NULL (i.e. ecma-undefined) - otherwise.
 */
ecma_property_t
ecma_op_object_get_own_property (ecma_object_t *object_p, /**< the object */
                                 ecma_string_t *property_name_p, /**< property name */
                                 ecma_property_ref_t *property_ref_p, /**< property reference */
                                 uint32_t options) /**< option bits */
{
  JERRY_ASSERT (object_p != NULL && !ecma_is_lexical_environment (object_p));
#if JERRY_BUILTIN_PROXY
  JERRY_ASSERT (!ECMA_OBJECT_IS_PROXY (object_p));
#endif /* JERRY_BUILTIN_PROXY */
  JERRY_ASSERT (property_name_p != NULL);
  JERRY_ASSERT (options == ECMA_PROPERTY_GET_NO_OPTIONS || property_ref_p != NULL);

  ecma_object_base_type_t base_type = ecma_get_object_base_type (object_p);

  switch (base_type)
  {
    case ECMA_OBJECT_BASE_TYPE_CLASS:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      switch (ext_object_p->u.cls.type)
      {
        case ECMA_OBJECT_CLASS_STRING:
        {
          if (ecma_string_is_length (property_name_p))
          {
            if (options & ECMA_PROPERTY_GET_VALUE)
            {
              ecma_value_t prim_value_p = ext_object_p->u.cls.u3.value;
              ecma_string_t *prim_value_str_p = ecma_get_string_from_value (prim_value_p);

              lit_utf8_size_t length = ecma_string_get_length (prim_value_str_p);
              property_ref_p->virtual_value = ecma_make_uint32_value (length);
            }

            return ECMA_PROPERTY_VIRTUAL;
          }

          uint32_t index = ecma_string_get_array_index (property_name_p);

          if (index != ECMA_STRING_NOT_ARRAY_INDEX)
          {
            ecma_value_t prim_value_p = ext_object_p->u.cls.u3.value;
            ecma_string_t *prim_value_str_p = ecma_get_string_from_value (prim_value_p);

            if (index < ecma_string_get_length (prim_value_str_p))
            {
              if (options & ECMA_PROPERTY_GET_VALUE)
              {
                ecma_char_t char_at_idx = ecma_string_get_char_at_pos (prim_value_str_p, index);
                ecma_string_t *char_str_p = ecma_new_ecma_string_from_code_unit (char_at_idx);
                property_ref_p->virtual_value = ecma_make_string_value (char_str_p);
              }

              return ECMA_PROPERTY_FLAG_ENUMERABLE | ECMA_PROPERTY_VIRTUAL;
            }
          }
          break;
        }
#if JERRY_BUILTIN_TYPEDARRAY
        /* ES2015 9.4.5.1 */
        case ECMA_OBJECT_CLASS_TYPEDARRAY:
        {
          if (ecma_prop_name_is_symbol (property_name_p))
          {
            break;
          }

          uint32_t index = ecma_string_get_array_index (property_name_p);

          if (index == ECMA_STRING_NOT_ARRAY_INDEX)
          {
            JERRY_ASSERT (index == UINT32_MAX);

            if (!ecma_typedarray_is_element_index (property_name_p))
            {
              break;
            }
          }

          ecma_typedarray_info_t info = ecma_typedarray_get_info (object_p);
          ecma_value_t value = ecma_get_typedarray_element (&info, index);

          if (ECMA_IS_VALUE_ERROR (value))
          {
            return ECMA_PROPERTY_TYPE_NOT_FOUND_AND_THROW;
          }

          if (JERRY_UNLIKELY (ecma_is_value_undefined (value)))
          {
            return ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP;
          }

          if (options & ECMA_PROPERTY_GET_VALUE)
          {
            property_ref_p->virtual_value = value;
          }
          else
          {
            ecma_fast_free_value (value);
          }

          return ECMA_PROPERTY_ENUMERABLE_WRITABLE | ECMA_PROPERTY_VIRTUAL;
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_MODULE_SYSTEM
        case ECMA_OBJECT_CLASS_MODULE_NAMESPACE:
        {
          if (JERRY_UNLIKELY (ecma_prop_name_is_symbol (property_name_p)))
          {
            if (!ecma_op_compare_string_to_global_symbol (property_name_p, LIT_GLOBAL_SYMBOL_TO_STRING_TAG))
            {
              return ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP;
            }

            /* ECMA-262 v11, 26.3.1 */
            if (options & ECMA_PROPERTY_GET_VALUE)
            {
              property_ref_p->virtual_value = ecma_make_magic_string_value (LIT_MAGIC_STRING_MODULE_UL);
            }

            return ECMA_PROPERTY_VIRTUAL;
          }

          ecma_property_t *property_p = ecma_find_named_property (object_p, property_name_p);

          if (property_p == NULL)
          {
            return ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP;
          }

          JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

          if (*property_p & ECMA_PROPERTY_FLAG_DATA)
          {
            if (options & ECMA_PROPERTY_GET_EXT_REFERENCE)
            {
              ((ecma_extended_property_ref_t *) property_ref_p)->property_p = property_p;
            }

            if (property_ref_p != NULL)
            {
              property_ref_p->value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
            }

            return *property_p;
          }

          if (options & ECMA_PROPERTY_GET_VALUE)
          {
            ecma_property_value_t *prop_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
            prop_value_p = ecma_get_property_value_from_named_reference (prop_value_p);
            property_ref_p->virtual_value = ecma_fast_copy_value (prop_value_p->value);
          }

          return ECMA_PROPERTY_ENUMERABLE_WRITABLE | ECMA_PROPERTY_VIRTUAL;
        }
#endif /* JERRY_MODULE_SYSTEM */
      }
      break;
    }
    case ECMA_OBJECT_BASE_TYPE_ARRAY:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      if (ecma_string_is_length (property_name_p))
      {
        if (options & ECMA_PROPERTY_GET_VALUE)
        {
          property_ref_p->virtual_value = ecma_make_uint32_value (ext_object_p->u.array.length);
        }

        uint32_t length_prop = ext_object_p->u.array.length_prop_and_hole_count;
        return length_prop & (ECMA_PROPERTY_FLAG_WRITABLE | ECMA_PROPERTY_VIRTUAL);
      }

      if (ecma_op_array_is_fast_array (ext_object_p))
      {
        uint32_t index = ecma_string_get_array_index (property_name_p);

        if (index != ECMA_STRING_NOT_ARRAY_INDEX)
        {
          if (JERRY_LIKELY (index < ext_object_p->u.array.length))
          {
            ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

            if (ecma_is_value_array_hole (values_p[index]))
            {
              return ECMA_PROPERTY_TYPE_NOT_FOUND;
            }

            if (options & ECMA_PROPERTY_GET_VALUE)
            {
              property_ref_p->virtual_value = ecma_fast_copy_value (values_p[index]);
            }

            return ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE | ECMA_PROPERTY_VIRTUAL;
          }
        }

        return ECMA_PROPERTY_TYPE_NOT_FOUND;
      }

      break;
    }
    default:
    {
      break;
    }
  }

  ecma_property_t *property_p = ecma_find_named_property (object_p, property_name_p);
  ecma_object_type_t type = ecma_get_object_type (object_p);

  if (property_p == NULL)
  {
    switch (type)
    {
      case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
      {
        if (ecma_builtin_function_is_routine (object_p))
        {
          property_p = ecma_builtin_routine_try_to_instantiate_property (object_p, property_name_p);
          break;
        }
        /* FALLTHRU */
      }
      case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
      case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
      case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
      {
        property_p = ecma_builtin_try_to_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_CLASS:
      {
        if (((ecma_extended_object_t *) object_p)->u.cls.type == ECMA_OBJECT_CLASS_ARGUMENTS)
        {
          property_p = ecma_op_arguments_object_try_to_lazy_instantiate_property (object_p, property_name_p);
        }
        break;
      }
      case ECMA_OBJECT_TYPE_FUNCTION:
      {
        /* Get prototype physical property. */
        property_p = ecma_op_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_NATIVE_FUNCTION:
      {
        property_p = ecma_op_external_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
      {
        property_p = ecma_op_bound_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      default:
      {
        break;
      }
    }

    if (property_p == NULL)
    {
      return ECMA_PROPERTY_TYPE_NOT_FOUND;
    }
  }
  else if (type == ECMA_OBJECT_TYPE_CLASS
           && ((ecma_extended_object_t *) object_p)->u.cls.type == ECMA_OBJECT_CLASS_ARGUMENTS
           && (((ecma_extended_object_t *) object_p)->u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED))
  {
    ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

    uint32_t index = ecma_string_get_array_index (property_name_p);

    if (index < ext_object_p->u.cls.u2.formal_params_number)
    {
      ecma_mapped_arguments_t *mapped_arguments_p = (ecma_mapped_arguments_t *) ext_object_p;

      ecma_value_t *argv_p = (ecma_value_t *) (mapped_arguments_p + 1);

      if (!ecma_is_value_empty (argv_p[index]) && argv_p[index] != ECMA_VALUE_ARGUMENT_NO_TRACK)
      {
#if JERRY_LCACHE
        /* Mapped arguments initialized properties MUST not be lcached */
        if (ecma_is_property_lcached (property_p))
        {
          jmem_cpointer_t prop_name_cp;

          if (JERRY_UNLIKELY (ECMA_IS_DIRECT_STRING (property_name_p)))
          {
            prop_name_cp = (jmem_cpointer_t) ECMA_GET_DIRECT_STRING_VALUE (property_name_p);
          }
          else
          {
            ECMA_SET_NON_NULL_POINTER (prop_name_cp, property_name_p);
          }
          ecma_lcache_invalidate (object_p, prop_name_cp, property_p);
        }
#endif /* JERRY_LCACHE */
        ecma_string_t *name_p = ecma_op_arguments_object_get_formal_parameter (mapped_arguments_p, index);
        ecma_object_t *lex_env_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, mapped_arguments_p->lex_env);

        ecma_value_t binding_value = ecma_op_get_binding_value (lex_env_p, name_p, true);

        ecma_named_data_property_assign_value (object_p, ECMA_PROPERTY_VALUE_PTR (property_p), binding_value);
        ecma_free_value (binding_value);
      }
    }
  }

  if (options & ECMA_PROPERTY_GET_EXT_REFERENCE)
  {
    ((ecma_extended_property_ref_t *) property_ref_p)->property_p = property_p;
  }

  if (property_ref_p != NULL)
  {
    property_ref_p->value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
  }

  return *property_p;
} /* ecma_op_object_get_own_property */

/**
 * Generic [[HasProperty]] operation
 *
 * See also:
 *          ECMAScript v6, 9.1.7.1
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE_FALSE} - whether the property is found
 */
ecma_value_t
ecma_op_object_has_property (ecma_object_t *object_p, /**< the object */
                             ecma_string_t *property_name_p) /**< property name */
{
  while (true)
  {
#if JERRY_BUILTIN_PROXY
    if (ECMA_OBJECT_IS_PROXY (object_p))
    {
      return ecma_proxy_object_has (object_p, property_name_p);
    }
#endif /* JERRY_BUILTIN_PROXY */

    /* 2 - 3. */
    ecma_property_t property =
      ecma_op_object_get_own_property (object_p, property_name_p, NULL, ECMA_PROPERTY_GET_NO_OPTIONS);

    if (property != ECMA_PROPERTY_TYPE_NOT_FOUND)
    {
#if JERRY_BUILTIN_TYPEDARRAY
      if (JERRY_UNLIKELY (property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_THROW))
      {
        return ECMA_VALUE_ERROR;
      }
#endif /* JERRY_BUILTIN_TYPEDARRAY */

      JERRY_ASSERT (property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP || ECMA_PROPERTY_IS_FOUND (property));

      return ecma_make_boolean_value (property != ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP);
    }

    jmem_cpointer_t proto_cp = ecma_op_ordinary_object_get_prototype_of (object_p);

    /* 7. */
    if (proto_cp == JMEM_CP_NULL)
    {
      return ECMA_VALUE_FALSE;
    }

    object_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp);
  }
} /* ecma_op_object_has_property */

/**
 * Search the value corresponding to a property name
 *
 * Note: search includes prototypes
 *
 * @return ecma value if property is found
 *         ECMA_VALUE_NOT_FOUND if property is not found
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_find_own (ecma_value_t base_value, /**< base value */
                         ecma_object_t *object_p, /**< target object */
                         ecma_string_t *property_name_p) /**< property name */
{
  JERRY_ASSERT (object_p != NULL && !ecma_is_lexical_environment (object_p));
  JERRY_ASSERT (property_name_p != NULL);
  JERRY_ASSERT (!ECMA_OBJECT_IS_PROXY (object_p));

  ecma_object_base_type_t base_type = ecma_get_object_base_type (object_p);

  switch (base_type)
  {
    case ECMA_OBJECT_BASE_TYPE_CLASS:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      switch (ext_object_p->u.cls.type)
      {
        case ECMA_OBJECT_CLASS_STRING:
        {
          if (ecma_string_is_length (property_name_p))
          {
            ecma_value_t prim_value_p = ext_object_p->u.cls.u3.value;

            ecma_string_t *prim_value_str_p = ecma_get_string_from_value (prim_value_p);
            lit_utf8_size_t length = ecma_string_get_length (prim_value_str_p);

            return ecma_make_uint32_value (length);
          }

          uint32_t index = ecma_string_get_array_index (property_name_p);

          if (index != ECMA_STRING_NOT_ARRAY_INDEX)
          {
            ecma_value_t prim_value_p = ext_object_p->u.cls.u3.value;

            ecma_string_t *prim_value_str_p = ecma_get_string_from_value (prim_value_p);

            if (index < ecma_string_get_length (prim_value_str_p))
            {
              ecma_char_t char_at_idx = ecma_string_get_char_at_pos (prim_value_str_p, index);
              return ecma_make_string_value (ecma_new_ecma_string_from_code_unit (char_at_idx));
            }
          }
          break;
        }
        case ECMA_OBJECT_CLASS_ARGUMENTS:
        {
          if (!(ext_object_p->u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED))
          {
            break;
          }

          uint32_t index = ecma_string_get_array_index (property_name_p);

          if (index < ext_object_p->u.cls.u2.formal_params_number)
          {
            ecma_mapped_arguments_t *mapped_arguments_p = (ecma_mapped_arguments_t *) ext_object_p;

            ecma_value_t *argv_p = (ecma_value_t *) (mapped_arguments_p + 1);

            if (!ecma_is_value_empty (argv_p[index]) && argv_p[index] != ECMA_VALUE_ARGUMENT_NO_TRACK)
            {
              ecma_string_t *name_p = ecma_op_arguments_object_get_formal_parameter (mapped_arguments_p, index);
              ecma_object_t *lex_env_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, mapped_arguments_p->lex_env);

              return ecma_op_get_binding_value (lex_env_p, name_p, true);
            }
          }
          break;
        }
#if JERRY_BUILTIN_TYPEDARRAY
        /* ES2015 9.4.5.4 */
        case ECMA_OBJECT_CLASS_TYPEDARRAY:
        {
          if (ecma_prop_name_is_symbol (property_name_p))
          {
            break;
          }

          uint32_t index = ecma_string_get_array_index (property_name_p);

          if (index == ECMA_STRING_NOT_ARRAY_INDEX)
          {
            JERRY_ASSERT (index == UINT32_MAX);

            if (!ecma_typedarray_is_element_index (property_name_p))
            {
              break;
            }
          }

          ecma_typedarray_info_t info = ecma_typedarray_get_info (object_p);
          return ecma_get_typedarray_element (&info, index);
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_MODULE_SYSTEM
        case ECMA_OBJECT_CLASS_MODULE_NAMESPACE:
        {
          if (JERRY_UNLIKELY (ecma_prop_name_is_symbol (property_name_p)))
          {
            /* ECMA-262 v11, 26.3.1 */
            if (ecma_op_compare_string_to_global_symbol (property_name_p, LIT_GLOBAL_SYMBOL_TO_STRING_TAG))
            {
              return ecma_make_magic_string_value (LIT_MAGIC_STRING_MODULE_UL);
            }

            return ECMA_VALUE_NOT_FOUND;
          }

          ecma_property_t *property_p = ecma_find_named_property (object_p, property_name_p);

          if (property_p == NULL)
          {
            return ECMA_VALUE_NOT_FOUND;
          }

          JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

          ecma_property_value_t *prop_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

          if (!(*property_p & ECMA_PROPERTY_FLAG_DATA))
          {
            prop_value_p = ecma_get_property_value_from_named_reference (prop_value_p);

            if (JERRY_UNLIKELY (prop_value_p->value == ECMA_VALUE_UNINITIALIZED))
            {
              return ecma_raise_reference_error (ECMA_ERR_LET_CONST_NOT_INITIALIZED);
            }
          }

          return ecma_fast_copy_value (prop_value_p->value);
        }
#endif /* JERRY_MODULE_SYSTEM */
      }
      break;
    }
    case ECMA_OBJECT_BASE_TYPE_ARRAY:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      if (ecma_string_is_length (property_name_p))
      {
        return ecma_make_uint32_value (ext_object_p->u.array.length);
      }

      if (JERRY_LIKELY (ecma_op_array_is_fast_array (ext_object_p)))
      {
        uint32_t index = ecma_string_get_array_index (property_name_p);

        if (JERRY_LIKELY (index != ECMA_STRING_NOT_ARRAY_INDEX))
        {
          if (JERRY_LIKELY (index < ext_object_p->u.array.length))
          {
            ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

            return (ecma_is_value_array_hole (values_p[index]) ? ECMA_VALUE_NOT_FOUND
                                                               : ecma_fast_copy_value (values_p[index]));
          }
        }
        return ECMA_VALUE_NOT_FOUND;
      }

      break;
    }
    default:
    {
      break;
    }
  }

  ecma_property_t *property_p = ecma_find_named_property (object_p, property_name_p);

  if (property_p == NULL)
  {
    switch (ecma_get_object_type (object_p))
    {
      case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
      {
        if (ecma_builtin_function_is_routine (object_p))
        {
          property_p = ecma_builtin_routine_try_to_instantiate_property (object_p, property_name_p);
          break;
        }
        /* FALLTHRU */
      }
      case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
      case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
      case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
      {
        property_p = ecma_builtin_try_to_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_CLASS:
      {
        if (((ecma_extended_object_t *) object_p)->u.cls.type == ECMA_OBJECT_CLASS_ARGUMENTS)
        {
          property_p = ecma_op_arguments_object_try_to_lazy_instantiate_property (object_p, property_name_p);
        }
        break;
      }
      case ECMA_OBJECT_TYPE_FUNCTION:
      {
        /* Get prototype physical property. */
        property_p = ecma_op_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_NATIVE_FUNCTION:
      {
        property_p = ecma_op_external_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
      {
        property_p = ecma_op_bound_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      default:
      {
        break;
      }
    }

    if (property_p == NULL)
    {
      return ECMA_VALUE_NOT_FOUND;
    }
  }

  JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

  ecma_property_value_t *prop_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

  if (*property_p & ECMA_PROPERTY_FLAG_DATA)
  {
    return ecma_fast_copy_value (prop_value_p->value);
  }

  ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (prop_value_p);

  if (get_set_pair_p->getter_cp == JMEM_CP_NULL)
  {
    return ECMA_VALUE_UNDEFINED;
  }

  ecma_object_t *getter_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->getter_cp);

  return ecma_op_function_call (getter_p, base_value, NULL, 0);
} /* ecma_op_object_find_own */

/**
 * Search the value corresponding to a property index
 *
 * Note: this method falls back to the general ecma_op_object_find
 *
 * @return ecma value if property is found
 *         ECMA_VALUE_NOT_FOUND if property is not found
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_find_by_index (ecma_object_t *object_p, /**< the object */
                              ecma_length_t index) /**< property index */
{
  if (JERRY_LIKELY (index <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return ecma_op_object_find (object_p, ECMA_CREATE_DIRECT_UINT32_STRING (index));
  }

  ecma_string_t *index_str_p = ecma_new_ecma_string_from_length (index);
  ecma_value_t ret_value = ecma_op_object_find (object_p, index_str_p);
  ecma_deref_ecma_string (index_str_p);

  return ret_value;
} /* ecma_op_object_find_by_index */

/**
 * Search the value corresponding to a property name
 *
 * Note: search includes prototypes
 *
 * @return ecma value if property is found
 *         ECMA_VALUE_NOT_FOUND if property is not found
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_find (ecma_object_t *object_p, /**< the object */
                     ecma_string_t *property_name_p) /**< property name */
{
  ecma_value_t base_value = ecma_make_object_value (object_p);

  while (true)
  {
#if JERRY_BUILTIN_PROXY
    if (ECMA_OBJECT_IS_PROXY (object_p))
    {
      return ecma_proxy_object_find (object_p, property_name_p);
    }
#endif /* JERRY_BUILTIN_PROXY */

    ecma_value_t value = ecma_op_object_find_own (base_value, object_p, property_name_p);

    if (ecma_is_value_found (value))
    {
      return value;
    }

    if (object_p->u2.prototype_cp == JMEM_CP_NULL)
    {
      break;
    }

    object_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, object_p->u2.prototype_cp);
  }

  return ECMA_VALUE_NOT_FOUND;
} /* ecma_op_object_find */

/**
 * [[Get]] operation of ecma object
 *
 * This function returns the value of a named property, or undefined
 * if the property is not found in the prototype chain. If the property
 * is an accessor, it calls the "get" callback function and returns
 * with its result (including error throws).
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_get (ecma_object_t *object_p, /**< the object */
                    ecma_string_t *property_name_p) /**< property name */
{
  return ecma_op_object_get_with_receiver (object_p, property_name_p, ecma_make_object_value (object_p));
} /* ecma_op_object_get */

/**
 * [[Get]] operation of ecma object with the specified receiver
 *
 * This function returns the value of a named property, or undefined
 * if the property is not found in the prototype chain. If the property
 * is an accessor, it calls the "get" callback function and returns
 * with its result (including error throws).
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_get_with_receiver (ecma_object_t *object_p, /**< the object */
                                  ecma_string_t *property_name_p, /**< property name */
                                  ecma_value_t receiver) /**< receiver to invoke getter function */
{
  while (true)
  {
#if JERRY_BUILTIN_PROXY
    if (ECMA_OBJECT_IS_PROXY (object_p))
    {
      return ecma_proxy_object_get (object_p, property_name_p, receiver);
    }
#endif /* JERRY_BUILTIN_PROXY */

    ecma_value_t value = ecma_op_object_find_own (receiver, object_p, property_name_p);

    if (ecma_is_value_found (value))
    {
      return value;
    }

    jmem_cpointer_t proto_cp = ecma_op_ordinary_object_get_prototype_of (object_p);

    if (proto_cp == JMEM_CP_NULL)
    {
      break;
    }

    object_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp);
  }

  return ECMA_VALUE_UNDEFINED;
} /* ecma_op_object_get_with_receiver */

/**
 * [[Get]] operation of ecma object specified for property index
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_get_by_index (ecma_object_t *object_p, /**< the object */
                             ecma_length_t index) /**< property index */
{
  if (JERRY_LIKELY (index <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return ecma_op_object_get (object_p, ECMA_CREATE_DIRECT_UINT32_STRING (index));
  }

  ecma_string_t *index_str_p = ecma_new_ecma_string_from_length (index);
  ecma_value_t ret_value = ecma_op_object_get (object_p, index_str_p);
  ecma_deref_ecma_string (index_str_p);

  return ret_value;
} /* ecma_op_object_get_by_index */

/**
 * Perform ToLength(O.[[Get]]("length")) operation
 *
 * The property is converted to uint32 during the operation
 *
 * @return ECMA_VALUE_ERROR - if there was any error during the operation
 *         ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
ecma_op_object_get_length (ecma_object_t *object_p, /**< the object */
                           ecma_length_t *length_p) /**< [out] length value converted to uint32 */
{
  if (JERRY_LIKELY (ecma_get_object_base_type (object_p) == ECMA_OBJECT_BASE_TYPE_ARRAY))
  {
    *length_p = (ecma_length_t) ecma_array_get_length (object_p);
    return ECMA_VALUE_EMPTY;
  }

  ecma_value_t len_value = ecma_op_object_get_by_magic_id (object_p, LIT_MAGIC_STRING_LENGTH);
  ecma_value_t len_number = ecma_op_to_length (len_value, length_p);
  ecma_free_value (len_value);

  JERRY_ASSERT (ECMA_IS_VALUE_ERROR (len_number) || ecma_is_value_empty (len_number));

  return len_number;
} /* ecma_op_object_get_length */

/**
 * [[Get]] operation of ecma object where the property name is a magic string
 *
 * This function returns the value of a named property, or undefined
 * if the property is not found in the prototype chain. If the property
 * is an accessor, it calls the "get" callback function and returns
 * with its result (including error throws).
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_get_by_magic_id (ecma_object_t *object_p, /**< the object */
                                lit_magic_string_id_t property_id) /**< property magic string id */
{
  return ecma_op_object_get (object_p, ecma_get_magic_string (property_id));
} /* ecma_op_object_get_by_magic_id */

/**
 * Descriptor string for each global symbol
 */
static const uint16_t ecma_global_symbol_descriptions[] = {
  LIT_MAGIC_STRING_ASYNC_ITERATOR, LIT_MAGIC_STRING_HAS_INSTANCE,  LIT_MAGIC_STRING_IS_CONCAT_SPREADABLE,
  LIT_MAGIC_STRING_ITERATOR,       LIT_MAGIC_STRING_MATCH,         LIT_MAGIC_STRING_REPLACE,
  LIT_MAGIC_STRING_SEARCH,         LIT_MAGIC_STRING_SPECIES,       LIT_MAGIC_STRING_SPLIT,
  LIT_MAGIC_STRING_TO_PRIMITIVE,   LIT_MAGIC_STRING_TO_STRING_TAG, LIT_MAGIC_STRING_UNSCOPABLES,
  LIT_MAGIC_STRING_MATCH_ALL,
};

JERRY_STATIC_ASSERT ((sizeof (ecma_global_symbol_descriptions) / sizeof (uint16_t) == ECMA_BUILTIN_GLOBAL_SYMBOL_COUNT),
                     ecma_global_symbol_descriptions_must_have_global_symbol_count_elements);

/**
 * [[Get]] a well-known symbol by the given property id
 *
 * @return pointer to the requested well-known symbol
 */
ecma_string_t *
ecma_op_get_global_symbol (lit_magic_string_id_t property_id) /**< property symbol id */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (LIT_IS_GLOBAL_SYMBOL (property_id));

  uint32_t symbol_index = (uint32_t) property_id - (uint32_t) LIT_GLOBAL_SYMBOL__FIRST;
  jmem_cpointer_t symbol_cp = JERRY_CONTEXT (global_symbols_cp)[symbol_index];

  if (symbol_cp != JMEM_CP_NULL)
  {
    ecma_string_t *symbol_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, symbol_cp);
    ecma_ref_ecma_string (symbol_p);
    return symbol_p;
  }

  ecma_string_t *symbol_dot_p = ecma_get_magic_string (LIT_MAGIC_STRING_SYMBOL_DOT_UL);
  uint16_t description = ecma_global_symbol_descriptions[symbol_index];
  ecma_string_t *name_p = ecma_get_magic_string ((lit_magic_string_id_t) description);
  ecma_string_t *descriptor_p = ecma_concat_ecma_strings (symbol_dot_p, name_p);

  ecma_string_t *symbol_p = ecma_new_symbol_from_descriptor_string (ecma_make_string_value (descriptor_p));
  symbol_p->u.hash = (uint16_t) ((property_id << ECMA_SYMBOL_FLAGS_SHIFT) | ECMA_SYMBOL_FLAG_GLOBAL);

  ECMA_SET_NON_NULL_POINTER (JERRY_CONTEXT (global_symbols_cp)[symbol_index], symbol_p);

  ecma_ref_ecma_string (symbol_p);
  return symbol_p;
} /* ecma_op_get_global_symbol */

/**
 * Checks whether the string equals to the global symbol.
 *
 * @return true - if the string equals to the global symbol
 *         false - otherwise
 */
bool
ecma_op_compare_string_to_global_symbol (ecma_string_t *string_p, /**< string to compare */
                                         lit_magic_string_id_t property_id) /**< property symbol id */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (LIT_IS_GLOBAL_SYMBOL (property_id));

  uint32_t symbol_index = (uint32_t) property_id - (uint32_t) LIT_GLOBAL_SYMBOL__FIRST;
  jmem_cpointer_t symbol_cp = JERRY_CONTEXT (global_symbols_cp)[symbol_index];

  return (symbol_cp != JMEM_CP_NULL && string_p == ECMA_GET_NON_NULL_POINTER (ecma_string_t, symbol_cp));
} /* ecma_op_compare_string_to_global_symbol */

/**
 * [[Get]] operation of ecma object where the property is a well-known symbol
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_get_by_symbol_id (ecma_object_t *object_p, /**< the object */
                                 lit_magic_string_id_t property_id) /**< property symbol id */
{
  ecma_string_t *symbol_p = ecma_op_get_global_symbol (property_id);
  ecma_value_t ret_value = ecma_op_object_get (object_p, symbol_p);
  ecma_deref_ecma_string (symbol_p);

  return ret_value;
} /* ecma_op_object_get_by_symbol_id */

/**
 * GetMethod operation
 *
 * See also: ECMA-262 v6, 7.3.9
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator function object - if success
 *         raised error - otherwise
 */
static ecma_value_t
ecma_op_get_method (ecma_value_t value, /**< ecma value */
                    ecma_string_t *prop_name_p) /**< property name */
{
  /* 2. */
  ecma_value_t obj_value = ecma_op_to_object (value);

  if (ECMA_IS_VALUE_ERROR (obj_value))
  {
    return obj_value;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (obj_value);
  ecma_value_t func;

  func = ecma_op_object_get (obj_p, prop_name_p);
  ecma_deref_object (obj_p);

  /* 3. */
  if (ECMA_IS_VALUE_ERROR (func))
  {
    return func;
  }

  /* 4. */
  if (ecma_is_value_undefined (func) || ecma_is_value_null (func))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  /* 5. */
  if (!ecma_op_is_callable (func))
  {
    ecma_free_value (func);
    return ecma_raise_type_error (ECMA_ERR_ITERATOR_IS_NOT_CALLABLE);
  }

  /* 6. */
  return func;
} /* ecma_op_get_method */

/**
 * GetMethod operation when the property is a well-known symbol
 *
 * See also: ECMA-262 v6, 7.3.9
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator function object - if success
 *         raised error - otherwise
 */
ecma_value_t
ecma_op_get_method_by_symbol_id (ecma_value_t value, /**< ecma value */
                                 lit_magic_string_id_t symbol_id) /**< property symbol id */
{
  ecma_string_t *prop_name_p = ecma_op_get_global_symbol (symbol_id);
  ecma_value_t ret_value = ecma_op_get_method (value, prop_name_p);
  ecma_deref_ecma_string (prop_name_p);

  return ret_value;
} /* ecma_op_get_method_by_symbol_id */

/**
 * GetMethod operation when the property is a magic string
 *
 * See also: ECMA-262 v6, 7.3.9
 *
 * Note:
 *      Returned value must be freed with ecma_free_value.
 *
 * @return iterator function object - if success
 *         raised error - otherwise
 */
ecma_value_t
ecma_op_get_method_by_magic_id (ecma_value_t value, /**< ecma value */
                                lit_magic_string_id_t magic_id) /**< property magic id */
{
  return ecma_op_get_method (value, ecma_get_magic_string (magic_id));
} /* ecma_op_get_method_by_magic_id */

/**
 * [[Put]] ecma general object's operation specialized for property index
 *
 * Note: This function falls back to the general ecma_op_object_put
 *
 * @return ecma value
 *         The returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_op_object_put_by_index (ecma_object_t *object_p, /**< the object */
                             ecma_length_t index, /**< property index */
                             ecma_value_t value, /**< ecma value */
                             bool is_throw) /**< flag that controls failure handling */
{
  if (JERRY_LIKELY (index <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return ecma_op_object_put (object_p, ECMA_CREATE_DIRECT_UINT32_STRING (index), value, is_throw);
  }

  ecma_string_t *index_str_p = ecma_new_ecma_string_from_length (index);
  ecma_value_t ret_value = ecma_op_object_put (object_p, index_str_p, value, is_throw);
  ecma_deref_ecma_string (index_str_p);

  return ret_value;
} /* ecma_op_object_put_by_index */

/**
 * [[Put]] ecma general object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *          ECMA-262 v5, 8.12.5
 *          Also incorporates [[CanPut]] ECMA-262 v5, 8.12.4
 *
 * @return ecma value
 *         The returned value must be freed with ecma_free_value.
 *
 *         Returns with ECMA_VALUE_TRUE if the operation is
 *         successful. Otherwise it returns with an error object
 *         or ECMA_VALUE_FALSE.
 *
 *         Note: even if is_throw is false, the setter can throw an
 *         error, and this function returns with that error.
 */
ecma_value_t
ecma_op_object_put (ecma_object_t *object_p, /**< the object */
                    ecma_string_t *property_name_p, /**< property name */
                    ecma_value_t value, /**< ecma value */
                    bool is_throw) /**< flag that controls failure handling */
{
  return ecma_op_object_put_with_receiver (object_p,
                                           property_name_p,
                                           value,
                                           ecma_make_object_value (object_p),
                                           is_throw);
} /* ecma_op_object_put */

/**
 * [[Set]] ( P, V, Receiver) operation part for ordinary objects
 *
 * See also: ECMAScript v6, 9.19.9
 *
 * @return ecma value
 *         The returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_op_object_put_apply_receiver (ecma_value_t receiver, /**< receiver */
                                   ecma_string_t *property_name_p, /**< property name */
                                   ecma_value_t value, /**< value to set */
                                   bool is_throw) /**< flag that controls failure handling */
{
  /* 5.b */
  if (!ecma_is_value_object (receiver))
  {
    return ECMA_REJECT (is_throw, ECMA_ERR_RECEIVER_MUST_BE_AN_OBJECT);
  }

  ecma_object_t *receiver_obj_p = ecma_get_object_from_value (receiver);

  ecma_property_descriptor_t prop_desc;
  /* 5.c */
  ecma_value_t status = ecma_op_object_get_own_property_descriptor (receiver_obj_p, property_name_p, &prop_desc);

  /* 5.d */
  if (ECMA_IS_VALUE_ERROR (status))
  {
    return status;
  }

  /* 5.e */
  if (ecma_is_value_true (status))
  {
    ecma_value_t result;

    /* 5.e.i - 5.e.ii */
    if (prop_desc.flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED)
        || !(prop_desc.flags & JERRY_PROP_IS_WRITABLE))
    {
      result = ecma_raise_property_redefinition (property_name_p, prop_desc.flags);
    }
    else
    {
      /* 5.e.iii */
      JERRY_ASSERT (prop_desc.flags & JERRY_PROP_IS_VALUE_DEFINED);
      ecma_free_value (prop_desc.value);
      prop_desc.value = ecma_copy_value (value);

      /* 5.e.iv */
      result = ecma_op_object_define_own_property (receiver_obj_p, property_name_p, &prop_desc);

      if (JERRY_UNLIKELY (ecma_is_value_false (result)))
      {
        result = ECMA_REJECT (is_throw, ECMA_ERR_PROXY_TRAP_RETURNED_FALSISH);
      }
    }

    ecma_free_property_descriptor (&prop_desc);

    return result;
  }

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (receiver_obj_p))
  {
    ecma_property_descriptor_t desc;
    /* Based on: ES6 9.1.9 [[Set]] 4.d.i. / ES11 9.1.9.2 OrdinarySetWithOwnDescriptor 2.c.i. */
    desc.flags = (JERRY_PROP_IS_CONFIGURABLE | JERRY_PROP_IS_CONFIGURABLE_DEFINED | JERRY_PROP_IS_ENUMERABLE
                  | JERRY_PROP_IS_ENUMERABLE_DEFINED | JERRY_PROP_IS_WRITABLE | JERRY_PROP_IS_WRITABLE_DEFINED
                  | JERRY_PROP_IS_VALUE_DEFINED);
    desc.value = value;
    ecma_value_t ret_value = ecma_proxy_object_define_own_property (receiver_obj_p, property_name_p, &desc);

    if (JERRY_UNLIKELY (ecma_is_value_false (ret_value)))
    {
      ret_value = ECMA_REJECT (is_throw, ECMA_ERR_PROXY_TRAP_RETURNED_FALSISH);
    }

    return ret_value;
  }
#endif /* JERRY_BUILTIN_PROXY */

  if (JERRY_UNLIKELY (ecma_op_object_is_fast_array (receiver_obj_p)))
  {
    ecma_fast_array_convert_to_normal (receiver_obj_p);
  }

  /* 5.f.i */
  ecma_property_value_t *new_prop_value_p;
  new_prop_value_p = ecma_create_named_data_property (receiver_obj_p,
                                                      property_name_p,
                                                      ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                      NULL);
  JERRY_ASSERT (ecma_is_value_undefined (new_prop_value_p->value));
  new_prop_value_p->value = ecma_copy_value_if_not_object (value);

  return ECMA_VALUE_TRUE;
} /* ecma_op_object_put_apply_receiver */

/**
 * [[Put]] ecma general object's operation with given receiver
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *          ECMA-262 v5, 8.12.5
 *          ECMA-262 v6, 9.1.9
 *          Also incorporates [[CanPut]] ECMA-262 v5, 8.12.4
 *
 * @return ecma value
 *         The returned value must be freed with ecma_free_value.
 *
 *         Returns with ECMA_VALUE_TRUE if the operation is
 *         successful. Otherwise it returns with an error object
 *         or ECMA_VALUE_FALSE.
 *
 *         Note: even if is_throw is false, the setter can throw an
 *         error, and this function returns with that error.
 */
ecma_value_t
ecma_op_object_put_with_receiver (ecma_object_t *object_p, /**< the object */
                                  ecma_string_t *property_name_p, /**< property name */
                                  ecma_value_t value, /**< ecma value */
                                  ecma_value_t receiver, /**< receiver */
                                  bool is_throw) /**< flag that controls failure handling */
{
  JERRY_ASSERT (object_p != NULL && !ecma_is_lexical_environment (object_p));
  JERRY_ASSERT (property_name_p != NULL);

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (object_p))
  {
    return ecma_proxy_object_set (object_p, property_name_p, value, receiver, is_throw);
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_object_base_type_t base_type = ecma_get_object_base_type (object_p);

  switch (base_type)
  {
    case ECMA_OBJECT_BASE_TYPE_CLASS:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      switch (ext_object_p->u.cls.type)
      {
        case ECMA_OBJECT_CLASS_ARGUMENTS:
        {
          if (!(ext_object_p->u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED))
          {
            break;
          }

          uint32_t index = ecma_string_get_array_index (property_name_p);

          if (index < ext_object_p->u.cls.u2.formal_params_number)
          {
            ecma_mapped_arguments_t *mapped_arguments_p = (ecma_mapped_arguments_t *) ext_object_p;

            ecma_value_t *argv_p = (ecma_value_t *) (mapped_arguments_p + 1);

            if (!ecma_is_value_empty (argv_p[index]) && argv_p[index] != ECMA_VALUE_ARGUMENT_NO_TRACK)
            {
              ecma_string_t *name_p = ecma_op_arguments_object_get_formal_parameter (mapped_arguments_p, index);
              ecma_object_t *lex_env_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, mapped_arguments_p->lex_env);
              ecma_op_set_mutable_binding (lex_env_p, name_p, value, true);
              return ECMA_VALUE_TRUE;
            }
          }
          break;
        }
#if JERRY_BUILTIN_TYPEDARRAY
        /* ES2015 9.4.5.5 */
        case ECMA_OBJECT_CLASS_TYPEDARRAY:
        {
          if (ecma_prop_name_is_symbol (property_name_p))
          {
            break;
          }

          uint32_t index = ecma_string_get_array_index (property_name_p);

          if (index == ECMA_STRING_NOT_ARRAY_INDEX)
          {
            JERRY_ASSERT (index == UINT32_MAX);

            if (!ecma_typedarray_is_element_index (property_name_p))
            {
              break;
            }
          }

          ecma_typedarray_info_t info = ecma_typedarray_get_info (object_p);
          return ecma_set_typedarray_element (&info, value, index);
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_MODULE_SYSTEM
        case ECMA_OBJECT_CLASS_MODULE_NAMESPACE:
        {
          return ecma_raise_readonly_assignment (property_name_p, is_throw);
        }
#endif /* JERRY_MODULE_SYSTEM */
      }
      break;
    }
    case ECMA_OBJECT_BASE_TYPE_ARRAY:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

      if (ecma_string_is_length (property_name_p))
      {
        if (ecma_is_property_writable ((ecma_property_t) ext_object_p->u.array.length_prop_and_hole_count))
        {
          return ecma_op_array_object_set_length (object_p, value, 0);
        }

        return ecma_raise_readonly_assignment (property_name_p, is_throw);
      }

      if (JERRY_LIKELY (ecma_op_array_is_fast_array (ext_object_p)))
      {
        uint32_t index = ecma_string_get_array_index (property_name_p);

        if (JERRY_UNLIKELY (index == ECMA_STRING_NOT_ARRAY_INDEX))
        {
          ecma_fast_array_convert_to_normal (object_p);
        }
        else if (ecma_fast_array_set_property (object_p, index, value))
        {
          return ECMA_VALUE_TRUE;
        }
      }

      JERRY_ASSERT (!ecma_op_object_is_fast_array (object_p));
      break;
    }
    default:
    {
      break;
    }
  }

  ecma_property_t *property_p = ecma_find_named_property (object_p, property_name_p);

  if (property_p == NULL)
  {
    switch (ecma_get_object_type (object_p))
    {
      case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
      {
        if (ecma_builtin_function_is_routine (object_p))
        {
          property_p = ecma_builtin_routine_try_to_instantiate_property (object_p, property_name_p);
          break;
        }
        /* FALLTHRU */
      }
      case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
      case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
      case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
      {
        property_p = ecma_builtin_try_to_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_CLASS:
      {
        ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

        switch (ext_object_p->u.cls.type)
        {
          case ECMA_OBJECT_CLASS_STRING:
          {
            uint32_t index = ecma_string_get_array_index (property_name_p);

            if (index != ECMA_STRING_NOT_ARRAY_INDEX)
            {
              ecma_value_t prim_value_p = ext_object_p->u.cls.u3.value;
              ecma_string_t *prim_value_str_p = ecma_get_string_from_value (prim_value_p);

              if (index < ecma_string_get_length (prim_value_str_p))
              {
                return ecma_raise_readonly_assignment (property_name_p, is_throw);
              }
            }
            break;
          }
          case ECMA_OBJECT_CLASS_ARGUMENTS:
          {
            property_p = ecma_op_arguments_object_try_to_lazy_instantiate_property (object_p, property_name_p);
            break;
          }
        }
        break;
      }
      case ECMA_OBJECT_TYPE_FUNCTION:
      {
        if (ecma_string_is_length (property_name_p))
        {
          /* Uninitialized 'length' property is non-writable (ECMA-262 v6, 19.2.4.1) */
          if (!ECMA_GET_FIRST_BIT_FROM_POINTER_TAG (((ecma_extended_object_t *) object_p)->u.function.scope_cp))
          {
            return ecma_raise_readonly_assignment (property_name_p, is_throw);
          }
        }

        /* Get prototype physical property. */
        property_p = ecma_op_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_NATIVE_FUNCTION:
      {
        property_p = ecma_op_external_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
      {
        property_p = ecma_op_bound_function_try_to_lazy_instantiate_property (object_p, property_name_p);
        break;
      }
      default:
      {
        break;
      }
    }
  }

  jmem_cpointer_t setter_cp = JMEM_CP_NULL;

  if (property_p != NULL)
  {
    JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

    if (*property_p & ECMA_PROPERTY_FLAG_DATA)
    {
      if (ecma_is_property_writable (*property_p))
      {
        if (ecma_make_object_value (object_p) != receiver)
        {
          return ecma_op_object_put_apply_receiver (receiver, property_name_p, value, is_throw);
        }

        /* There is no need for special casing arrays here because changing the
         * value of an existing property never changes the length of an array. */
        ecma_named_data_property_assign_value (object_p, ECMA_PROPERTY_VALUE_PTR (property_p), value);
        return ECMA_VALUE_TRUE;
      }
    }
    else
    {
      ecma_getter_setter_pointers_t *get_set_pair_p;
      get_set_pair_p = ecma_get_named_accessor_property (ECMA_PROPERTY_VALUE_PTR (property_p));
      setter_cp = get_set_pair_p->setter_cp;
    }
  }
  else
  {
    bool create_new_property = true;

    jmem_cpointer_t obj_cp;
    ECMA_SET_NON_NULL_POINTER (obj_cp, object_p);
    ecma_object_t *proto_p = object_p;

    while (true)
    {
      obj_cp = ecma_op_ordinary_object_get_prototype_of (proto_p);

      if (obj_cp == JMEM_CP_NULL)
      {
        break;
      }

      ecma_property_ref_t property_ref = { NULL };
      proto_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, obj_cp);

#if JERRY_BUILTIN_PROXY
      if (ECMA_OBJECT_IS_PROXY (proto_p))
      {
        return ecma_op_object_put_with_receiver (proto_p, property_name_p, value, receiver, is_throw);
      }
#endif /* JERRY_BUILTIN_PROXY */

      ecma_property_t inherited_property =
        ecma_op_object_get_own_property (proto_p, property_name_p, &property_ref, ECMA_PROPERTY_GET_NO_OPTIONS);

      if (ECMA_PROPERTY_IS_FOUND (inherited_property))
      {
        JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (inherited_property));

        if (!(inherited_property & ECMA_PROPERTY_FLAG_DATA))
        {
          setter_cp = ecma_get_named_accessor_property (property_ref.value_p)->setter_cp;
          create_new_property = false;
          break;
        }

        create_new_property = ecma_is_property_writable (inherited_property);
        break;
      }

      JERRY_ASSERT (inherited_property == ECMA_PROPERTY_TYPE_NOT_FOUND
                    || inherited_property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP);
    }

#if JERRY_BUILTIN_PROXY
    if (create_new_property && ecma_is_value_object (receiver)
        && ECMA_OBJECT_IS_PROXY (ecma_get_object_from_value (receiver)))
    {
      return ecma_op_object_put_apply_receiver (receiver, property_name_p, value, is_throw);
    }
#endif /* JERRY_BUILTIN_PROXY */

    if (create_new_property && ecma_op_ordinary_object_is_extensible (object_p))
    {
      const ecma_object_base_type_t obj_base_type = ecma_get_object_base_type (object_p);

      if (obj_base_type == ECMA_OBJECT_BASE_TYPE_CLASS)
      {
        ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

        if (ext_object_p->u.cls.type == ECMA_OBJECT_CLASS_ARGUMENTS
            && ext_object_p->u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED)
        {
          const uint32_t flags = ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE | JERRY_PROP_SHOULD_THROW;
          return ecma_builtin_helper_def_prop (object_p, property_name_p, value, flags);
        }
      }

      uint32_t index = ecma_string_get_array_index (property_name_p);

      if (obj_base_type == ECMA_OBJECT_BASE_TYPE_ARRAY && index != ECMA_STRING_NOT_ARRAY_INDEX)
      {
        ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

        if (index < UINT32_MAX && index >= ext_object_p->u.array.length)
        {
          if (!ecma_is_property_writable ((ecma_property_t) ext_object_p->u.array.length_prop_and_hole_count))
          {
            return ecma_raise_readonly_assignment (property_name_p, is_throw);
          }

          ext_object_p->u.array.length = index + 1;
        }
      }

      return ecma_op_object_put_apply_receiver (receiver, property_name_p, value, is_throw);

      ecma_property_value_t *new_prop_value_p;
      new_prop_value_p = ecma_create_named_data_property (object_p,
                                                          property_name_p,
                                                          ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                          NULL);

      JERRY_ASSERT (ecma_is_value_undefined (new_prop_value_p->value));
      new_prop_value_p->value = ecma_copy_value_if_not_object (value);
      return ECMA_VALUE_TRUE;
    }
  }

  if (setter_cp == JMEM_CP_NULL)
  {
    return ecma_raise_readonly_assignment (property_name_p, is_throw);
  }

  ecma_value_t ret_value =
    ecma_op_function_call (ECMA_GET_NON_NULL_POINTER (ecma_object_t, setter_cp), receiver, &value, 1);

  if (!ECMA_IS_VALUE_ERROR (ret_value))
  {
    ecma_fast_free_value (ret_value);
    ret_value = ECMA_VALUE_TRUE;
  }

  return ret_value;
} /* ecma_op_object_put_with_receiver */

/**
 * [[Delete]] ecma object's operation specialized for property index
 *
 * Note:
 *      This method falls back to the general ecma_op_object_delete
 *
 * @return true - if deleted successfully
 *         false - or type error otherwise (based in 'is_throw')
 */
ecma_value_t
ecma_op_object_delete_by_index (ecma_object_t *obj_p, /**< the object */
                                ecma_length_t index, /**< property index */
                                bool is_throw) /**< flag that controls failure handling */
{
  if (JERRY_LIKELY (index <= ECMA_DIRECT_STRING_MAX_IMM))
  {
    return ecma_op_object_delete (obj_p, ECMA_CREATE_DIRECT_UINT32_STRING (index), is_throw);
    ;
  }

  ecma_string_t *index_str_p = ecma_new_ecma_string_from_length (index);
  ecma_value_t ret_value = ecma_op_object_delete (obj_p, index_str_p, is_throw);
  ecma_deref_ecma_string (index_str_p);

  return ret_value;
} /* ecma_op_object_delete_by_index */

/**
 * [[Delete]] ecma object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *
 * Note:
 *      returned value must be freed with ecma_free_value
 *
 * @return true - if deleted successfully
 *         false - or type error otherwise (based in 'is_throw')
 */
ecma_value_t
ecma_op_object_delete (ecma_object_t *obj_p, /**< the object */
                       ecma_string_t *property_name_p, /**< property name */
                       bool is_strict) /**< flag that controls failure handling */
{
  JERRY_ASSERT (obj_p != NULL && !ecma_is_lexical_environment (obj_p));
  JERRY_ASSERT (property_name_p != NULL);

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    return ecma_proxy_object_delete_property (obj_p, property_name_p, is_strict);
  }
#endif /* JERRY_BUILTIN_PROXY */

  JERRY_ASSERT_OBJECT_TYPE_IS_VALID (ecma_get_object_type (obj_p));

  return ecma_op_general_object_delete (obj_p, property_name_p, is_strict);
} /* ecma_op_object_delete */

/**
 * [[DefaultValue]] ecma object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_default_value (ecma_object_t *obj_p, /**< the object */
                              ecma_preferred_type_hint_t hint) /**< hint on preferred result type */
{
  JERRY_ASSERT (obj_p != NULL && !ecma_is_lexical_environment (obj_p));

  JERRY_ASSERT_OBJECT_TYPE_IS_VALID (ecma_get_object_type (obj_p));

  /*
   * typedef ecma_property_t * (*default_value_ptr_t) (ecma_object_t *, ecma_string_t *);
   * static const default_value_ptr_t default_value [ECMA_OBJECT_TYPE__COUNT] =
   * {
   *   [ECMA_OBJECT_TYPE_GENERAL]           = &ecma_op_general_object_default_value,
   *   [ECMA_OBJECT_TYPE_CLASS]             = &ecma_op_general_object_default_value,
   *   [ECMA_OBJECT_TYPE_FUNCTION]          = &ecma_op_general_object_default_value,
   *   [ECMA_OBJECT_TYPE_NATIVE_FUNCTION]   = &ecma_op_general_object_default_value,
   *   [ECMA_OBJECT_TYPE_ARRAY]             = &ecma_op_general_object_default_value,
   *   [ECMA_OBJECT_TYPE_BOUND_FUNCTION]    = &ecma_op_general_object_default_value,
   *   [ECMA_OBJECT_TYPE_PSEUDO_ARRAY]      = &ecma_op_general_object_default_value
   * };
   *
   * return default_value[type] (obj_p, property_name_p);
   */

  return ecma_op_general_object_default_value (obj_p, hint);
} /* ecma_op_object_default_value */

/**
 * [[DefineOwnProperty]] ecma object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_define_own_property (ecma_object_t *obj_p, /**< the object */
                                    ecma_string_t *property_name_p, /**< property name */
                                    const ecma_property_descriptor_t *property_desc_p) /**< property
                                                                                        *   descriptor */
{
  JERRY_ASSERT (obj_p != NULL && !ecma_is_lexical_environment (obj_p));
  JERRY_ASSERT (property_name_p != NULL);

  const ecma_object_type_t type = ecma_get_object_type (obj_p);

  switch (type)
  {
    case ECMA_OBJECT_TYPE_CLASS:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

      switch (ext_object_p->u.cls.type)
      {
        case ECMA_OBJECT_CLASS_ARGUMENTS:
        {
          return ecma_op_arguments_object_define_own_property (obj_p, property_name_p, property_desc_p);
        }
#if JERRY_BUILTIN_TYPEDARRAY
        /* ES2015 9.4.5.1 */
        case ECMA_OBJECT_CLASS_TYPEDARRAY:
        {
          return ecma_op_typedarray_define_own_property (obj_p, property_name_p, property_desc_p);
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
      }
      break;
    }
    case ECMA_OBJECT_TYPE_ARRAY:
    case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
    {
      return ecma_op_array_object_define_own_property (obj_p, property_name_p, property_desc_p);
    }
#if JERRY_BUILTIN_PROXY
    case ECMA_OBJECT_TYPE_PROXY:
    {
      return ecma_proxy_object_define_own_property (obj_p, property_name_p, property_desc_p);
    }
#endif /* JERRY_BUILTIN_PROXY */
    default:
    {
      break;
    }
  }

  return ecma_op_general_object_define_own_property (obj_p, property_name_p, property_desc_p);
} /* ecma_op_object_define_own_property */

/**
 * Get property descriptor from specified property
 *
 * depending on the property type the following fields are set:
 *   - for named data properties: { [Value], [Writable], [Enumerable], [Configurable] };
 *   - for named accessor properties: { [Get] - if defined,
 *                                      [Set] - if defined,
 *                                      [Enumerable], [Configurable]
 *                                    }.
 *
 * The output property descriptor will always be initialized to an empty descriptor.
 *
 * @return ECMA_VALUE_ERROR - if the Proxy.[[GetOwnProperty]] operation raises error
 *         ECMA_VALUE_{TRUE, FALSE} - if property found or not
 */
ecma_value_t
ecma_op_object_get_own_property_descriptor (ecma_object_t *object_p, /**< the object */
                                            ecma_string_t *property_name_p, /**< property name */
                                            ecma_property_descriptor_t *prop_desc_p) /**< property descriptor */
{
  *prop_desc_p = ecma_make_empty_property_descriptor ();

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (object_p))
  {
    return ecma_proxy_object_get_own_property_descriptor (object_p, property_name_p, prop_desc_p);
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_property_ref_t property_ref;
  property_ref.virtual_value = ECMA_VALUE_EMPTY;
  ecma_property_t property =
    ecma_op_object_get_own_property (object_p, property_name_p, &property_ref, ECMA_PROPERTY_GET_VALUE);

  if (!ECMA_PROPERTY_IS_FOUND (property))
  {
#if JERRY_BUILTIN_TYPEDARRAY
    if (JERRY_UNLIKELY (property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_THROW))
    {
      return ECMA_VALUE_ERROR;
    }
#endif /* JERRY_BUILTIN_TYPEDARRAY */

    JERRY_ASSERT (property == ECMA_PROPERTY_TYPE_NOT_FOUND || property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP);

    return ECMA_VALUE_FALSE;
  }

  uint32_t flags = ecma_is_property_enumerable (property) ? JERRY_PROP_IS_ENUMERABLE : JERRY_PROP_NO_OPTS;
  flags |= ecma_is_property_configurable (property) ? JERRY_PROP_IS_CONFIGURABLE : JERRY_PROP_NO_OPTS;

  prop_desc_p->flags = (uint16_t) (JERRY_PROP_IS_ENUMERABLE_DEFINED | JERRY_PROP_IS_CONFIGURABLE_DEFINED | flags);

  if (property & ECMA_PROPERTY_FLAG_DATA)
  {
    if (!ECMA_PROPERTY_IS_VIRTUAL (property))
    {
      prop_desc_p->value = ecma_copy_value (property_ref.value_p->value);
    }
    else
    {
#if JERRY_MODULE_SYSTEM
      if (JERRY_UNLIKELY (property_ref.virtual_value == ECMA_VALUE_UNINITIALIZED))
      {
        return ecma_raise_reference_error (ECMA_ERR_LET_CONST_NOT_INITIALIZED);
      }
#endif /* JERRY_MODULE_SYSTEM */
      prop_desc_p->value = property_ref.virtual_value;
    }

    prop_desc_p->flags |=
      (uint16_t) (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE_DEFINED
                  | (ecma_is_property_writable (property) ? JERRY_PROP_IS_WRITABLE : JERRY_PROP_NO_OPTS));
  }
  else
  {
    ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (property_ref.value_p);
    prop_desc_p->flags |= (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED);

    if (get_set_pair_p->getter_cp == JMEM_CP_NULL)
    {
      prop_desc_p->get_p = NULL;
    }
    else
    {
      prop_desc_p->get_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->getter_cp);
      ecma_ref_object (prop_desc_p->get_p);
    }

    if (get_set_pair_p->setter_cp == JMEM_CP_NULL)
    {
      prop_desc_p->set_p = NULL;
    }
    else
    {
      prop_desc_p->set_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->setter_cp);
      ecma_ref_object (prop_desc_p->set_p);
    }
  }

  return ECMA_VALUE_TRUE;
} /* ecma_op_object_get_own_property_descriptor */

#if JERRY_BUILTIN_PROXY
/**
 * Get property descriptor from a target value for a specified property.
 *
 * For more details see ecma_op_object_get_own_property_descriptor
 *
 * @return ECMA_VALUE_ERROR - if the Proxy.[[GetOwnProperty]] operation raises error
 *         ECMA_VALUE_{TRUE, FALSE} - if property found or not
 */
ecma_value_t
ecma_op_get_own_property_descriptor (ecma_value_t target, /**< target value */
                                     ecma_string_t *property_name_p, /**< property name */
                                     ecma_property_descriptor_t *prop_desc_p) /**< property descriptor */
{
  if (!ecma_is_value_object (target))
  {
    return ECMA_VALUE_FALSE;
  }

  return ecma_op_object_get_own_property_descriptor (ecma_get_object_from_value (target), property_name_p, prop_desc_p);
} /* ecma_op_get_own_property_descriptor */
#endif /* JERRY_BUILTIN_PROXY */

/**
 * [[HasInstance]] ecma object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 9
 *
 * @return ecma value containing a boolean value or an error
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_object_has_instance (ecma_object_t *obj_p, /**< the object */
                             ecma_value_t value) /**< argument 'V' */
{
  JERRY_ASSERT (obj_p != NULL && !ecma_is_lexical_environment (obj_p));

  JERRY_ASSERT_OBJECT_TYPE_IS_VALID (ecma_get_object_type (obj_p));

  if (ecma_op_object_is_callable (obj_p))
  {
    return ecma_op_function_has_instance (obj_p, value);
  }

  return ecma_raise_type_error (ECMA_ERR_EXPECTED_A_FUNCTION_OBJECT);
} /* ecma_op_object_has_instance */

/**
 * General [[GetPrototypeOf]] abstract operation
 *
 * Note: returned valid object must be freed.
 *
 * @return ecma_object_t * - prototype of the input object.
 *         ECMA_OBJECT_POINTER_ERROR - error reported during Proxy resolve.
 *         NULL - the input object does not have a prototype.
 */
ecma_object_t *
ecma_op_object_get_prototype_of (ecma_object_t *obj_p) /**< input object */
{
  JERRY_ASSERT (obj_p != NULL);

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    ecma_value_t proto = ecma_proxy_object_get_prototype_of (obj_p);

    if (ECMA_IS_VALUE_ERROR (proto))
    {
      return ECMA_OBJECT_POINTER_ERROR;
    }
    if (ecma_is_value_null (proto))
    {
      return NULL;
    }

    JERRY_ASSERT (ecma_is_value_object (proto));

    return ecma_get_object_from_value (proto);
  }
  else
#endif /* JERRY_BUILTIN_PROXY */
  {
    jmem_cpointer_t proto_cp = ecma_op_ordinary_object_get_prototype_of (obj_p);

    if (proto_cp == JMEM_CP_NULL)
    {
      return NULL;
    }

    ecma_object_t *proto_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp);
    ecma_ref_object (proto_p);

    return proto_p;
  }
} /* ecma_op_object_get_prototype_of */

/**
 * Object's isPrototypeOf operation
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.6; 3
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_TRUE - if the target object is prototype of the base object
 *         ECMA_VALUE_FALSE - if the target object is not prototype of the base object
 */
ecma_value_t
ecma_op_object_is_prototype_of (ecma_object_t *base_p, /**< base object */
                                ecma_object_t *target_p) /**< target object */
{
  ecma_ref_object (target_p);

  do
  {
    ecma_object_t *proto_p = ecma_op_object_get_prototype_of (target_p);
    ecma_deref_object (target_p);

    if (proto_p == NULL)
    {
      return ECMA_VALUE_FALSE;
    }
    else if (proto_p == ECMA_OBJECT_POINTER_ERROR)
    {
      return ECMA_VALUE_ERROR;
    }
    else if (proto_p == base_p)
    {
      ecma_deref_object (proto_p);
      return ECMA_VALUE_TRUE;
    }

    /* Advance up on prototype chain. */
    target_p = proto_p;
  } while (true);
} /* ecma_op_object_is_prototype_of */

/**
 * Object's EnumerableOwnPropertyNames operation
 *
 * See also:
 *          ECMA-262 v11, 7.3.23
 *
 * @return NULL - if operation fails
 *         collection of property names / values / name-value pairs - otherwise
 */
ecma_collection_t *
ecma_op_object_get_enumerable_property_names (ecma_object_t *obj_p, /**< routine's first argument */
                                              ecma_enumerable_property_names_options_t option) /**< listing option */
{
  /* 2. */
  ecma_collection_t *prop_names_p = ecma_op_object_own_property_keys (obj_p, JERRY_PROPERTY_FILTER_EXCLUDE_SYMBOLS);

#if JERRY_BUILTIN_PROXY
  if (JERRY_UNLIKELY (prop_names_p == NULL))
  {
    return prop_names_p;
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_value_t *names_buffer_p = prop_names_p->buffer_p;
  /* 3. */
  ecma_collection_t *properties_p = ecma_new_collection ();

  /* 4. */
  for (uint32_t i = 0; i < prop_names_p->item_count; i++)
  {
    /* 4.a */
    if (ecma_is_value_string (names_buffer_p[i]))
    {
      ecma_string_t *key_p = ecma_get_string_from_value (names_buffer_p[i]);

      /* 4.a.i */
      ecma_property_descriptor_t prop_desc;
      ecma_value_t status = ecma_op_object_get_own_property_descriptor (obj_p, key_p, &prop_desc);

      if (ECMA_IS_VALUE_ERROR (status))
      {
        ecma_collection_free (prop_names_p);
        ecma_collection_free (properties_p);

        return NULL;
      }

      const bool is_enumerable = (prop_desc.flags & JERRY_PROP_IS_ENUMERABLE) != 0;
      ecma_free_property_descriptor (&prop_desc);
      /* 4.a.ii */
      if (is_enumerable)
      {
        /* 4.a.ii.1 */
        if (option == ECMA_ENUMERABLE_PROPERTY_KEYS)
        {
          ecma_collection_push_back (properties_p, ecma_copy_value (names_buffer_p[i]));
        }
        else
        {
          /* 4.a.ii.2.a */
          ecma_value_t value = ecma_op_object_get (obj_p, key_p);

          if (ECMA_IS_VALUE_ERROR (value))
          {
            ecma_collection_free (prop_names_p);
            ecma_collection_free (properties_p);

            return NULL;
          }

          /* 4.a.ii.2.b */
          if (option == ECMA_ENUMERABLE_PROPERTY_VALUES)
          {
            ecma_collection_push_back (properties_p, value);
          }
          else
          {
            /* 4.a.ii.2.c.i */
            JERRY_ASSERT (option == ECMA_ENUMERABLE_PROPERTY_ENTRIES);

            /* 4.a.ii.2.c.ii */
            ecma_object_t *entry_p = ecma_op_new_array_object (2);

            ecma_builtin_helper_def_prop_by_index (entry_p,
                                                   0,
                                                   names_buffer_p[i],
                                                   ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
            ecma_builtin_helper_def_prop_by_index (entry_p, 1, value, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
            ecma_free_value (value);

            /* 4.a.ii.2.c.iii */
            ecma_collection_push_back (properties_p, ecma_make_object_value (entry_p));
          }
        }
      }
    }
  }

  ecma_collection_free (prop_names_p);

  return properties_p;
} /* ecma_op_object_get_enumerable_property_names */

/**
 * Helper method for getting lazy instantiated properties for [[OwnPropertyKeys]]
 */
static void
ecma_object_list_lazy_property_names (ecma_object_t *obj_p, /**< object */
                                      ecma_collection_t *prop_names_p, /**< prop name collection */
                                      ecma_property_counter_t *prop_counter_p, /**< property counters */
                                      jerry_property_filter_t filter) /**< property name filter options */
{
  switch (ecma_get_object_type (obj_p))
  {
    case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
    {
      if (ecma_builtin_function_is_routine (obj_p))
      {
        ecma_builtin_routine_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
        break;
      }
      /* FALLTHRU */
    }
    case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
    case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
    case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
    {
      ecma_builtin_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
      break;
    }
    case ECMA_OBJECT_TYPE_CLASS:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

      switch (ext_object_p->u.cls.type)
      {
        case ECMA_OBJECT_CLASS_STRING:
        {
          ecma_op_string_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
          break;
        }
        case ECMA_OBJECT_CLASS_ARGUMENTS:
        {
          ecma_op_arguments_object_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
          break;
        }
#if JERRY_BUILTIN_TYPEDARRAY
        /* ES2015 9.4.5.1 */
        case ECMA_OBJECT_CLASS_TYPEDARRAY:
        {
          ecma_op_typedarray_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
          break;
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
      }
      break;
    }
    case ECMA_OBJECT_TYPE_FUNCTION:
    {
      ecma_op_function_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
      break;
    }
    case ECMA_OBJECT_TYPE_NATIVE_FUNCTION:
    {
      ecma_op_external_function_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
      break;
    }
    case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
    {
      ecma_op_bound_function_list_lazy_property_names (obj_p, prop_names_p, prop_counter_p, filter);
      break;
    }
    case ECMA_OBJECT_TYPE_ARRAY:
    {
      if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS))
      {
        ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_LENGTH));
        prop_counter_p->string_named_props++;
      }
      break;
    }
    default:
    {
      JERRY_ASSERT (ecma_get_object_type (obj_p) == ECMA_OBJECT_TYPE_GENERAL
                    || ecma_get_object_type (obj_p) == ECMA_OBJECT_TYPE_CONSTRUCTOR_FUNCTION);
      break;
    }
  }
} /* ecma_object_list_lazy_property_names */

/**
 * Helper routine for heapsort algorithm.
 */
static void
ecma_op_object_heap_sort_shift_down (ecma_value_t *buffer_p, /**< array of items */
                                     uint32_t item_count, /**< number of items */
                                     uint32_t item_index) /**< index of updated item */
{
  while (true)
  {
    uint32_t highest_index = item_index;
    uint32_t current_index = (item_index << 1) + 1;

    if (current_index >= item_count)
    {
      return;
    }

    uint32_t value = ecma_string_get_array_index (ecma_get_string_from_value (buffer_p[highest_index]));
    uint32_t left_value = ecma_string_get_array_index (ecma_get_string_from_value (buffer_p[current_index]));

    if (value < left_value)
    {
      highest_index = current_index;
      value = left_value;
    }

    current_index++;

    if (current_index < item_count
        && value < ecma_string_get_array_index (ecma_get_string_from_value (buffer_p[current_index])))
    {
      highest_index = current_index;
    }

    if (highest_index == item_index)
    {
      return;
    }

    ecma_value_t tmp = buffer_p[highest_index];
    buffer_p[highest_index] = buffer_p[item_index];
    buffer_p[item_index] = tmp;

    item_index = highest_index;
  }
} /* ecma_op_object_heap_sort_shift_down */

/**
 * Object's [[OwnPropertyKeys]] internal method
 *
 * Order of names in the collection:
 *  - integer indices in ascending order
 *  - other indices in creation order (for built-ins: the order of the properties are listed in specification).
 *
 * Note:
 *      Implementation of the routine assumes that new properties are appended to beginning of corresponding object's
 *      property list, and the list is not reordered (in other words, properties are stored in order that is reversed
 *      to the properties' addition order).
 *
 * @return NULL - if the Proxy.[[OwnPropertyKeys]] operation raises error
 *         collection of property names - otherwise
 */
ecma_collection_t *
ecma_op_object_own_property_keys (ecma_object_t *obj_p, /**< object */
                                  jerry_property_filter_t filter) /**< name filters */
{
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    return ecma_proxy_object_own_property_keys (obj_p);
  }
#endif /* JERRY_BUILTIN_PROXY */

  if (ecma_op_object_is_fast_array (obj_p))
  {
    return ecma_fast_array_object_own_property_keys (obj_p, filter);
  }

  ecma_collection_t *prop_names_p = ecma_new_collection ();
  ecma_property_counter_t prop_counter = { 0, 0, 0 };

  ecma_object_list_lazy_property_names (obj_p, prop_names_p, &prop_counter, filter);

  jmem_cpointer_t prop_iter_cp = obj_p->u1.property_list_cp;

#if JERRY_PROPERTY_HASHMAP
  if (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

    if (prop_iter_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
    {
      prop_iter_cp = prop_iter_p->next_property_cp;
    }
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  jmem_cpointer_t counter_prop_iter_cp = prop_iter_cp;

  uint32_t array_index_named_props = 0;
  uint32_t string_named_props = 0;
  uint32_t symbol_named_props = 0;

  while (counter_prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, counter_prop_iter_cp);
    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

    for (int i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      ecma_property_t *property_p = prop_iter_p->types + i;

      if (!ECMA_PROPERTY_IS_RAW (*property_p) || (*property_p & ECMA_PROPERTY_FLAG_BUILT_IN))
      {
        continue;
      }

      ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

      if (ECMA_PROPERTY_GET_NAME_TYPE (*property_p) == ECMA_DIRECT_STRING_MAGIC
          && prop_pair_p->names_cp[i] >= LIT_NON_INTERNAL_MAGIC_STRING__COUNT
          && prop_pair_p->names_cp[i] < LIT_MAGIC_STRING__COUNT)
      {
        continue;
      }

      ecma_string_t *name_p = ecma_string_from_property_name (*property_p, prop_pair_p->names_cp[i]);

      if (ecma_string_get_array_index (name_p) != ECMA_STRING_NOT_ARRAY_INDEX)
      {
        array_index_named_props++;
      }
      else if (ecma_prop_name_is_symbol (name_p))
      {
        if (!(name_p->u.hash & ECMA_SYMBOL_FLAG_PRIVATE_KEY))
        {
          symbol_named_props++;
        }
      }
      else
      {
        string_named_props++;
      }

      ecma_deref_ecma_string (name_p);
    }

    counter_prop_iter_cp = prop_iter_p->next_property_cp;
  }

  if (filter & JERRY_PROPERTY_FILTER_EXCLUDE_INTEGER_INDICES)
  {
    JERRY_ASSERT (prop_counter.array_index_named_props == 0);
    array_index_named_props = 0;
  }

  if (filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS)
  {
    JERRY_ASSERT (prop_counter.string_named_props == 0);
    string_named_props = 0;
  }

  if (filter & JERRY_PROPERTY_FILTER_EXCLUDE_SYMBOLS)
  {
    JERRY_ASSERT (prop_counter.symbol_named_props == 0);
    symbol_named_props = 0;
  }

  uint32_t total = array_index_named_props + string_named_props + symbol_named_props;

  if (total == 0)
  {
    return prop_names_p;
  }

  ecma_collection_reserve (prop_names_p, total);
  prop_names_p->item_count += total;

  ecma_value_t *buffer_p = prop_names_p->buffer_p;
  ecma_value_t *array_index_current_p = buffer_p + array_index_named_props + prop_counter.array_index_named_props;
  ecma_value_t *string_current_p = array_index_current_p + string_named_props + prop_counter.string_named_props;

  ecma_value_t *symbol_current_p = string_current_p + symbol_named_props + prop_counter.symbol_named_props;

  if (prop_counter.symbol_named_props > 0 && (array_index_named_props + string_named_props) > 0)
  {
    memmove ((void *) string_current_p,
             (void *) (buffer_p + prop_counter.array_index_named_props + prop_counter.string_named_props),
             prop_counter.symbol_named_props * sizeof (ecma_value_t));
  }

  if (prop_counter.string_named_props > 0 && array_index_named_props > 0)
  {
    memmove ((void *) array_index_current_p,
             (void *) (buffer_p + prop_counter.array_index_named_props),
             prop_counter.string_named_props * sizeof (ecma_value_t));
  }

  while (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);
    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

    for (int i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      ecma_property_t *property_p = prop_iter_p->types + i;

      if (!ECMA_PROPERTY_IS_RAW (*property_p) || (*property_p & ECMA_PROPERTY_FLAG_BUILT_IN))
      {
        continue;
      }

      ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

      if (ECMA_PROPERTY_GET_NAME_TYPE (*property_p) == ECMA_DIRECT_STRING_MAGIC
          && prop_pair_p->names_cp[i] >= LIT_NON_INTERNAL_MAGIC_STRING__COUNT
          && prop_pair_p->names_cp[i] < LIT_MAGIC_STRING__COUNT)
      {
        continue;
      }

      ecma_string_t *name_p = ecma_string_from_property_name (*property_p, prop_pair_p->names_cp[i]);

      if (ecma_string_get_array_index (name_p) != ECMA_STRING_NOT_ARRAY_INDEX)
      {
        if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_INTEGER_INDICES))
        {
          *(--array_index_current_p) = ecma_make_string_value (name_p);
          continue;
        }
      }
      else if (ecma_prop_name_is_symbol (name_p))
      {
        if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_SYMBOLS) && !(name_p->u.hash & ECMA_SYMBOL_FLAG_PRIVATE_KEY))
        {
          *(--symbol_current_p) = ecma_make_symbol_value (name_p);
          continue;
        }
      }
      else
      {
        if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS))
        {
          *(--string_current_p) = ecma_make_string_value (name_p);
          continue;
        }
      }

      ecma_deref_ecma_string (name_p);
    }

    prop_iter_cp = prop_iter_p->next_property_cp;
  }

  if (array_index_named_props > 1 || (array_index_named_props == 1 && prop_counter.array_index_named_props > 0))
  {
    uint32_t prev_value = 0;
    ecma_value_t *array_index_p = buffer_p + prop_counter.array_index_named_props;
    ecma_value_t *array_index_end_p = array_index_p + array_index_named_props;

    if (prop_counter.array_index_named_props > 0)
    {
      prev_value = ecma_string_get_array_index (ecma_get_string_from_value (array_index_p[-1]));
    }

    do
    {
      uint32_t value = ecma_string_get_array_index (ecma_get_string_from_value (*array_index_p++));

      if (value < prev_value)
      {
        uint32_t array_props = prop_counter.array_index_named_props + array_index_named_props;
        uint32_t i = (array_props >> 1) - 1;

        do
        {
          ecma_op_object_heap_sort_shift_down (buffer_p, array_props, i);
        } while (i-- > 0);

        i = array_props - 1;

        do
        {
          ecma_value_t tmp = buffer_p[i];
          buffer_p[i] = buffer_p[0];
          buffer_p[0] = tmp;

          ecma_op_object_heap_sort_shift_down (buffer_p, i, 0);
        } while (--i > 0);

        break;
      }

      prev_value = value;
    } while (array_index_p < array_index_end_p);
  }

  return prop_names_p;
} /* ecma_op_object_own_property_keys */

/**
 * EnumerateObjectProperties abstract method
 *
 * See also:
 *          ECMA-262 v11, 13.7.5.15
 *
 * @return NULL - if the Proxy.[[OwnPropertyKeys]] operation raises error
 *         collection of enumerable property names - otherwise
 */
ecma_collection_t *
ecma_op_object_enumerate (ecma_object_t *obj_p) /**< object */
{
  ecma_collection_t *visited_names_p = ecma_new_collection ();
  ecma_collection_t *return_names_p = ecma_new_collection ();

  ecma_ref_object (obj_p);

  while (true)
  {
    ecma_collection_t *keys = ecma_op_object_own_property_keys (obj_p, JERRY_PROPERTY_FILTER_EXCLUDE_SYMBOLS);

    if (JERRY_UNLIKELY (keys == NULL))
    {
      ecma_collection_free (return_names_p);
      ecma_collection_free (visited_names_p);
      ecma_deref_object (obj_p);
      return keys;
    }

    for (uint32_t i = 0; i < keys->item_count; i++)
    {
      ecma_value_t prop_name = keys->buffer_p[i];
      ecma_string_t *name_p = ecma_get_prop_name_from_value (prop_name);

      if (ecma_prop_name_is_symbol (name_p))
      {
        continue;
      }

      ecma_property_descriptor_t prop_desc;
      ecma_value_t get_desc = ecma_op_object_get_own_property_descriptor (obj_p, name_p, &prop_desc);

      if (ECMA_IS_VALUE_ERROR (get_desc))
      {
        ecma_collection_free (keys);
        ecma_collection_free (return_names_p);
        ecma_collection_free (visited_names_p);
        ecma_deref_object (obj_p);
        return NULL;
      }

      if (ecma_is_value_true (get_desc))
      {
        bool is_enumerable = (prop_desc.flags & JERRY_PROP_IS_ENUMERABLE) != 0;
        ecma_free_property_descriptor (&prop_desc);

        if (ecma_collection_has_string_value (visited_names_p, name_p)
            || ecma_collection_has_string_value (return_names_p, name_p))
        {
          continue;
        }

        ecma_ref_ecma_string (name_p);

        if (is_enumerable)
        {
          ecma_collection_push_back (return_names_p, prop_name);
        }
        else
        {
          ecma_collection_push_back (visited_names_p, prop_name);
        }
      }
    }

    ecma_collection_free (keys);

    /* Query the prototype. */
    ecma_object_t *proto_p = ecma_op_object_get_prototype_of (obj_p);
    ecma_deref_object (obj_p);

    if (proto_p == NULL)
    {
      break;
    }
    else if (JERRY_UNLIKELY (proto_p == ECMA_OBJECT_POINTER_ERROR))
    {
      ecma_collection_free (return_names_p);
      ecma_collection_free (visited_names_p);
      return NULL;
    }

    /* Advance up on prototype chain. */
    obj_p = proto_p;
  }

  ecma_collection_free (visited_names_p);

  return return_names_p;
} /* ecma_op_object_enumerate */

#ifndef JERRY_NDEBUG

/**
 * Check if passed object is the instance of specified built-in.
 *
 * @return true  - if the object is instance of the specified built-in
 *         false - otherwise
 */
static bool
ecma_builtin_is (ecma_object_t *object_p, /**< pointer to an object */
                 ecma_builtin_id_t builtin_id) /**< id of built-in to check on */
{
  JERRY_ASSERT (object_p != NULL && !ecma_is_lexical_environment (object_p));
  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_ID__COUNT);

  ecma_object_type_t type = ecma_get_object_type (object_p);

  switch (type)
  {
    case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
    case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
    {
      ecma_extended_object_t *built_in_object_p = (ecma_extended_object_t *) object_p;

      return (built_in_object_p->u.built_in.id == builtin_id && built_in_object_p->u.built_in.routine_id == 0);
    }
    case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
    case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
    {
      ecma_extended_built_in_object_t *extended_built_in_object_p = (ecma_extended_built_in_object_t *) object_p;

      return (extended_built_in_object_p->built_in.id == builtin_id
              && extended_built_in_object_p->built_in.routine_id == 0);
    }
    default:
    {
      return false;
    }
  }
} /* ecma_builtin_is */

#endif /* !JERRY_NDEBUG */

/**
 * Used by ecma_object_get_class_name to get the magic string id of class objects
 */
static const uint16_t ecma_class_object_magic_string_id[] = {
  /* These objects require custom property resolving. */
  LIT_MAGIC_STRING_STRING_UL, /**< magic string id of ECMA_OBJECT_CLASS_STRING */
  LIT_MAGIC_STRING_ARGUMENTS_UL, /**< magic string id of ECMA_OBJECT_CLASS_ARGUMENTS */
#if JERRY_BUILTIN_TYPEDARRAY
  LIT_MAGIC_STRING__EMPTY, /**< ECMA_OBJECT_CLASS_TYPEDARRAY needs special resolver */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_MODULE_SYSTEM
  LIT_MAGIC_STRING_MODULE_UL, /**< magic string id of ECMA_OBJECT_CLASS_MODULE_NAMESPACE */
#endif /* JERRY_MODULE_SYSTEM */

  /* These objects are marked by Garbage Collector. */
  LIT_MAGIC_STRING_GENERATOR_UL, /**< magic string id of ECMA_OBJECT_CLASS_GENERATOR */
  LIT_MAGIC_STRING_ASYNC_GENERATOR_UL, /**< magic string id of ECMA_OBJECT_CLASS_ASYNC_GENERATOR */
  LIT_MAGIC_STRING_ARRAY_ITERATOR_UL, /**< magic string id of ECMA_OBJECT_CLASS_ARRAY_ITERATOR */
  LIT_MAGIC_STRING_SET_ITERATOR_UL, /**< magic string id of ECMA_OBJECT_CLASS_SET_ITERATOR */
  LIT_MAGIC_STRING_MAP_ITERATOR_UL, /**< magic string id of ECMA_OBJECT_CLASS_MAP_ITERATOR */
#if JERRY_BUILTIN_REGEXP
  LIT_MAGIC_STRING_REGEXP_STRING_ITERATOR_UL, /**< magic string id of ECMA_OBJECT_CLASS_REGEXP_STRING_ITERATOR */
#endif /* JERRY_BUILTIN_REGEXP */
#if JERRY_MODULE_SYSTEM
  LIT_MAGIC_STRING_MODULE_UL, /**< magic string id of ECMA_OBJECT_CLASS_MODULE */
#endif /* JERRY_MODULE_SYSTEM */
  LIT_MAGIC_STRING_PROMISE_UL, /**< magic string id of ECMA_OBJECT_CLASS_PROMISE */
  LIT_MAGIC_STRING_OBJECT_UL, /**< magic string id of ECMA_OBJECT_CLASS_PROMISE_CAPABILITY */
  LIT_MAGIC_STRING_OBJECT_UL, /**< magic string id of ECMA_OBJECT_CLASS_ASYNC_FROM_SYNC_ITERATOR */
#if JERRY_BUILTIN_DATAVIEW
  LIT_MAGIC_STRING_DATAVIEW_UL, /**< magic string id of ECMA_OBJECT_CLASS_DATAVIEW */
#endif /* JERRY_BUILTIN_DATAVIEW */
#if JERRY_BUILTIN_CONTAINER
  LIT_MAGIC_STRING__EMPTY, /**< magic string id of ECMA_OBJECT_CLASS_CONTAINER needs special resolver */
#endif /* JERRY_BUILTIN_CONTAINER */

  /* Normal objects. */
  LIT_MAGIC_STRING_BOOLEAN_UL, /**< magic string id of ECMA_OBJECT_CLASS_BOOLEAN */
  LIT_MAGIC_STRING_NUMBER_UL, /**< magic string id of ECMA_OBJECT_CLASS_NUMBER */
  LIT_MAGIC_STRING_ERROR_UL, /**< magic string id of ECMA_OBJECT_CLASS_ERROR */
  LIT_MAGIC_STRING_OBJECT_UL, /**< magic string id of ECMA_OBJECT_CLASS_INTERNAL_OBJECT */
#if JERRY_PARSER
  LIT_MAGIC_STRING_SCRIPT_UL, /**< magic string id of ECMA_OBJECT_CLASS_SCRIPT */
#endif /* JERRY_PARSER */
#if JERRY_BUILTIN_DATE
  LIT_MAGIC_STRING_DATE_UL, /**< magic string id of ECMA_OBJECT_CLASS_DATE */
#endif /* JERRY_BUILTIN_DATE */
#if JERRY_BUILTIN_REGEXP
  LIT_MAGIC_STRING_REGEXP_UL, /**< magic string id of ECMA_OBJECT_CLASS_REGEXP */
#endif /* JERRY_BUILTIN_REGEXP */
  LIT_MAGIC_STRING_SYMBOL_UL, /**< magic string id of ECMA_OBJECT_CLASS_SYMBOL */
  LIT_MAGIC_STRING_STRING_ITERATOR_UL, /**< magic string id of ECMA_OBJECT_CLASS_STRING_ITERATOR */
#if JERRY_BUILTIN_TYPEDARRAY
  LIT_MAGIC_STRING_ARRAY_BUFFER_UL, /**< magic string id of ECMA_OBJECT_CLASS_ARRAY_BUFFER */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_BUILTIN_SHAREDARRAYBUFFER
  LIT_MAGIC_STRING_SHARED_ARRAY_BUFFER_UL, /**< magic string id of ECMA_OBJECT_CLASS_SHAREDARRAY_BUFFER */
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */
#if JERRY_BUILTIN_BIGINT
  LIT_MAGIC_STRING_BIGINT_UL, /**< magic string id of ECMA_OBJECT_CLASS_BIGINT */
#endif /* JERRY_BUILTIN_BIGINT */
#if JERRY_BUILTIN_WEAKREF
  LIT_MAGIC_STRING_WEAKREF_UL, /**< magic string id of ECMA_OBJECT_CLASS_WEAKREF */
#endif /* JERRY_BUILTIN_WEAKREF */
};

JERRY_STATIC_ASSERT ((sizeof (ecma_class_object_magic_string_id) == ECMA_OBJECT_CLASS__MAX * sizeof (uint16_t)),
                     ecma_class_object_magic_string_id_must_have_object_class_max_elements);

/**
 * Get [[Class]] string of specified object
 *
 * @return class name magic string
 */
lit_magic_string_id_t
ecma_object_get_class_name (ecma_object_t *obj_p) /**< object */
{
  ecma_object_type_t type = ecma_get_object_type (obj_p);

  switch (type)
  {
    case ECMA_OBJECT_TYPE_ARRAY:
    case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
    {
      return LIT_MAGIC_STRING_ARRAY_UL;
    }
    case ECMA_OBJECT_TYPE_CLASS:
    case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
    {
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

      switch (ext_object_p->u.cls.type)
      {
#if JERRY_BUILTIN_TYPEDARRAY
        case ECMA_OBJECT_CLASS_TYPEDARRAY:
        {
          return ecma_get_typedarray_magic_string_id ((ecma_typedarray_type_t) ext_object_p->u.cls.u1.typedarray_type);
        }
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_BUILTIN_CONTAINER
        case ECMA_OBJECT_CLASS_CONTAINER:
        {
          return (lit_magic_string_id_t) ext_object_p->u.cls.u2.container_id;
        }
#endif /* JERRY_BUILTIN_CONTAINER */
        default:
        {
          break;
        }
      }

      JERRY_ASSERT (ext_object_p->u.cls.type < ECMA_OBJECT_CLASS__MAX);
      JERRY_ASSERT (ecma_class_object_magic_string_id[ext_object_p->u.cls.type] != LIT_MAGIC_STRING__EMPTY);

      return (lit_magic_string_id_t) ecma_class_object_magic_string_id[ext_object_p->u.cls.type];
    }
    case ECMA_OBJECT_TYPE_FUNCTION:
    case ECMA_OBJECT_TYPE_NATIVE_FUNCTION:
    case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
    case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
    case ECMA_OBJECT_TYPE_CONSTRUCTOR_FUNCTION:
    {
      return LIT_MAGIC_STRING_FUNCTION_UL;
    }
#if JERRY_BUILTIN_PROXY
    case ECMA_OBJECT_TYPE_PROXY:
    {
      ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

      if (!ecma_is_value_null (proxy_obj_p->target) && ecma_is_value_object (proxy_obj_p->target))
      {
        ecma_object_t *target_obj_p = ecma_get_object_from_value (proxy_obj_p->target);
        return ecma_object_get_class_name (target_obj_p);
      }
      return LIT_MAGIC_STRING_OBJECT_UL;
    }
#endif /* JERRY_BUILTIN_PROXY */
    case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
    {
      ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) obj_p;

      switch (ext_obj_p->u.built_in.id)
      {
#if JERRY_BUILTIN_MATH
        case ECMA_BUILTIN_ID_MATH:
        {
          return LIT_MAGIC_STRING_MATH_UL;
        }
#endif /* JERRY_BUILTIN_MATH */
#if JERRY_BUILTIN_REFLECT
        case ECMA_BUILTIN_ID_REFLECT:
        {
          return LIT_MAGIC_STRING_REFLECT_UL;
        }
#endif /* JERRY_BUILTIN_REFLECT */
        case ECMA_BUILTIN_ID_GENERATOR:
        {
          return LIT_MAGIC_STRING_GENERATOR_UL;
        }
        case ECMA_BUILTIN_ID_ASYNC_GENERATOR:
        {
          return LIT_MAGIC_STRING_ASYNC_GENERATOR_UL;
        }
#if JERRY_BUILTIN_ATOMICS
        case ECMA_BUILTIN_ID_ATOMICS:
        {
          return LIT_MAGIC_STRING_ATOMICS_U;
        }
#endif /* JERRY_BUILTIN_ATOMICS */
        default:
        {
          break;
        }
      }

      return LIT_MAGIC_STRING_OBJECT_UL;
    }
    default:
    {
      JERRY_ASSERT (type == ECMA_OBJECT_TYPE_GENERAL || type == ECMA_OBJECT_TYPE_PROXY);

      return LIT_MAGIC_STRING_OBJECT_UL;
    }
  }
} /* ecma_object_get_class_name */

#if JERRY_BUILTIN_REGEXP
/**
 * Checks if the given argument has [[RegExpMatcher]] internal slot
 *
 * @return true - if the given argument is a regexp
 *         false - otherwise
 */
bool
ecma_object_is_regexp_object (ecma_value_t arg) /**< argument */
{
  return (ecma_is_value_object (arg)
          && ecma_object_class_is (ecma_get_object_from_value (arg), ECMA_OBJECT_CLASS_REGEXP));
} /* ecma_object_is_regexp_object */
#endif /* JERRY_BUILTIN_REGEXP */

/**
 * Object's IsConcatSpreadable operation, used for Array.prototype.concat
 * It checks the argument's [Symbol.isConcatSpreadable] property value
 *
 * See also:
 *          ECMA-262 v6, 22.1.3.1.1;
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_TRUE - if the argument is concatSpreadable
 *         ECMA_VALUE_FALSE - otherwise
 */
ecma_value_t
ecma_op_is_concat_spreadable (ecma_value_t arg) /**< argument */
{
  if (!ecma_is_value_object (arg))
  {
    return ECMA_VALUE_FALSE;
  }

  ecma_value_t spreadable =
    ecma_op_object_get_by_symbol_id (ecma_get_object_from_value (arg), LIT_GLOBAL_SYMBOL_IS_CONCAT_SPREADABLE);

  if (ECMA_IS_VALUE_ERROR (spreadable))
  {
    return spreadable;
  }

  if (!ecma_is_value_undefined (spreadable))
  {
    const bool to_bool = ecma_op_to_boolean (spreadable);
    ecma_free_value (spreadable);
    return ecma_make_boolean_value (to_bool);
  }

  return ecma_is_value_array (arg);
} /* ecma_op_is_concat_spreadable */

/**
 * IsRegExp operation
 *
 * See also:
 *          ECMA-262 v6, 22.1.3.1.1;
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_TRUE - if the argument is regexp
 *         ECMA_VALUE_FALSE - otherwise
 */
ecma_value_t
ecma_op_is_regexp (ecma_value_t arg) /**< argument */
{
#if JERRY_BUILTIN_REGEXP
  if (!ecma_is_value_object (arg))
  {
    return ECMA_VALUE_FALSE;
  }

  ecma_value_t is_regexp = ecma_op_object_get_by_symbol_id (ecma_get_object_from_value (arg), LIT_GLOBAL_SYMBOL_MATCH);

  if (ECMA_IS_VALUE_ERROR (is_regexp))
  {
    return is_regexp;
  }

  if (!ecma_is_value_undefined (is_regexp))
  {
    const bool to_bool = ecma_op_to_boolean (is_regexp);
    ecma_free_value (is_regexp);
    return ecma_make_boolean_value (to_bool);
  }

  return ecma_make_boolean_value (ecma_object_is_regexp_object (arg));
#else
  return ECMA_VALUE_FALSE;
#endif
} /* ecma_op_is_regexp */

/**
 * SpeciesConstructor operation
 * See also:
 *          ECMA-262 v6, 7.3.20;
 *
 * @return ecma_value
 *         returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_species_constructor (ecma_object_t *this_value, /**< This Value */
                             ecma_builtin_id_t default_constructor_id) /**< Builtin ID of default constructor */
{
  ecma_object_t *default_constructor_p = ecma_builtin_get (default_constructor_id);
  ecma_value_t constructor = ecma_op_object_get_by_magic_id (this_value, LIT_MAGIC_STRING_CONSTRUCTOR);
  if (ECMA_IS_VALUE_ERROR (constructor))
  {
    return constructor;
  }

  if (ecma_is_value_undefined (constructor))
  {
    ecma_ref_object (default_constructor_p);
    return ecma_make_object_value (default_constructor_p);
  }

  if (!ecma_is_value_object (constructor))
  {
    ecma_free_value (constructor);
    return ecma_raise_type_error (ECMA_ERR_CONSTRUCTOR_NOT_AN_OBJECT);
  }

  ecma_object_t *ctor_object_p = ecma_get_object_from_value (constructor);
  ecma_value_t species = ecma_op_object_get_by_symbol_id (ctor_object_p, LIT_GLOBAL_SYMBOL_SPECIES);
  ecma_deref_object (ctor_object_p);

  if (ECMA_IS_VALUE_ERROR (species))
  {
    return species;
  }

  if (ecma_is_value_undefined (species) || ecma_is_value_null (species))
  {
    ecma_ref_object (default_constructor_p);
    return ecma_make_object_value (default_constructor_p);
  }

  if (!ecma_is_constructor (species))
  {
    ecma_free_value (species);
    return ecma_raise_type_error (ECMA_ERR_SPECIES_MUST_BE_A_CONSTRUCTOR);
  }

  return species;
} /* ecma_op_species_constructor */

/**
 * 7.3.18 Abstract operation Invoke when property name is a magic string
 *
 * @return ecma_value result of the invoked function or raised error
 *         note: returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_invoke_by_symbol_id (ecma_value_t object, /**< Object value */
                             lit_magic_string_id_t symbol_id, /**< Symbol ID */
                             ecma_value_t *args_p, /**< Argument list */
                             uint32_t args_len) /**< Argument list length */
{
  ecma_string_t *symbol_p = ecma_op_get_global_symbol (symbol_id);
  ecma_value_t ret_value = ecma_op_invoke (object, symbol_p, args_p, args_len);
  ecma_deref_ecma_string (symbol_p);

  return ret_value;
} /* ecma_op_invoke_by_symbol_id */

/**
 * 7.3.18 Abstract operation Invoke when property name is a magic string
 *
 * @return ecma_value result of the invoked function or raised error
 *         note: returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_invoke_by_magic_id (ecma_value_t object, /**< Object value */
                            lit_magic_string_id_t magic_string_id, /**< Magic string ID */
                            ecma_value_t *args_p, /**< Argument list */
                            uint32_t args_len) /**< Argument list length */
{
  return ecma_op_invoke (object, ecma_get_magic_string (magic_string_id), args_p, args_len);
} /* ecma_op_invoke_by_magic_id */

/**
 * 7.3.18 Abstract operation Invoke
 *
 * @return ecma_value result of the invoked function or raised error
 *         note: returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_invoke (ecma_value_t object, /**< Object value */
                ecma_string_t *property_name_p, /**< Property name */
                ecma_value_t *args_p, /**< Argument list */
                uint32_t args_len) /**< Argument list length */
{
  /* 3. */
  ecma_value_t object_value = ecma_op_to_object (object);
  if (ECMA_IS_VALUE_ERROR (object_value))
  {
    return object_value;
  }

  ecma_object_t *object_p = ecma_get_object_from_value (object_value);

  ecma_value_t func = ecma_op_object_get_with_receiver (object_p, property_name_p, object);

  if (ECMA_IS_VALUE_ERROR (func))
  {
    ecma_deref_object (object_p);
    return func;
  }

  /* 4. */
  ecma_value_t call_result = ecma_op_function_validated_call (func, object, args_p, args_len);
  ecma_free_value (func);

  ecma_deref_object (object_p);

  return call_result;
} /* ecma_op_invoke */

/**
 * Ordinary object [[GetPrototypeOf]] operation
 *
 * See also:
 *          ECMAScript v6, 9.1.1
 *
 * @return the value of the [[Prototype]] internal slot of the given object.
 */
jmem_cpointer_t
ecma_op_ordinary_object_get_prototype_of (ecma_object_t *obj_p) /**< object */
{
  JERRY_ASSERT (!ecma_is_lexical_environment (obj_p));
  JERRY_ASSERT (!ECMA_OBJECT_IS_PROXY (obj_p));

  return obj_p->u2.prototype_cp;
} /* ecma_op_ordinary_object_get_prototype_of */

/**
 * Ordinary object [[SetPrototypeOf]] operation
 *
 * See also:
 *          ECMAScript v6, 9.1.2
 *
 * @return ECMA_VALUE_FALSE - if the operation fails
 *         ECMA_VALUE_TRUE - otherwise
 */
ecma_value_t
ecma_op_ordinary_object_set_prototype_of (ecma_object_t *obj_p, /**< base object */
                                          ecma_value_t proto) /**< prototype object */
{
  JERRY_ASSERT (!ecma_is_lexical_environment (obj_p));
  JERRY_ASSERT (!ECMA_OBJECT_IS_PROXY (obj_p));

  /* 1. */
  JERRY_ASSERT (ecma_is_value_object (proto) || ecma_is_value_null (proto));

  /* 3. */
  ecma_object_t *current_proto_p = ECMA_GET_POINTER (ecma_object_t, ecma_op_ordinary_object_get_prototype_of (obj_p));
  ecma_object_t *new_proto_p = ecma_is_value_null (proto) ? NULL : ecma_get_object_from_value (proto);

  /* 4. */
  if (new_proto_p == current_proto_p)
  {
    return ECMA_VALUE_TRUE;
  }

  /* 2 - 5. */
  if (!ecma_op_ordinary_object_is_extensible (obj_p))
  {
    return ECMA_VALUE_FALSE;
  }

  /**
   * When the prototype of a fast array changes, it is required to convert the
   * array to a "normal" array. This ensures that all [[Get]]/[[Set]]/etc.
   * calls works as expected.
   */
  if (ecma_op_object_is_fast_array (obj_p))
  {
    ecma_fast_array_convert_to_normal (obj_p);
  }

  /* 6. */
  ecma_object_t *iter_p = new_proto_p;

  /* 7 - 8. */
  while (true)
  {
    /* 8.a */
    if (iter_p == NULL)
    {
      break;
    }

    /* 8.b */
    if (obj_p == iter_p)
    {
      return ECMA_VALUE_FALSE;
    }

    /* 8.c.i */
#if JERRY_BUILTIN_PROXY
    if (ECMA_OBJECT_IS_PROXY (iter_p))
    {
      /**
       * Prevent setting 'Object.prototype.__proto__'
       * to avoid circular referencing in the prototype chain.
       */
      if (obj_p == ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE))
      {
        return ECMA_VALUE_FALSE;
      }

      break;
    }
#endif /* JERRY_BUILTIN_PROXY */

    /* 8.c.ii */
    iter_p = ECMA_GET_POINTER (ecma_object_t, ecma_op_ordinary_object_get_prototype_of (iter_p));
  }

  /* 9. */
  ECMA_SET_POINTER (obj_p->u2.prototype_cp, new_proto_p);

  /* 10. */
  return ECMA_VALUE_TRUE;
} /* ecma_op_ordinary_object_set_prototype_of */

/**
 * [[IsExtensible]] operation for Ordinary object.
 *
 * See also:
 *          ECMAScript v6, 9.1.2
 *
 * @return true  - if object is extensible
 *         false - otherwise
 */
bool JERRY_ATTR_PURE
ecma_op_ordinary_object_is_extensible (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (!ECMA_OBJECT_IS_PROXY (object_p));

  bool is_extensible = (object_p->type_flags_refs & ECMA_OBJECT_FLAG_EXTENSIBLE) != 0;

  JERRY_ASSERT (!ecma_op_object_is_fast_array (object_p) || is_extensible);

  return is_extensible;
} /* ecma_op_ordinary_object_is_extensible */

/**
 * Set value of [[Extensible]] object's internal property.
 *
 * @return void
 */
void JERRY_ATTR_NOINLINE
ecma_op_ordinary_object_prevent_extensions (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (!ECMA_OBJECT_IS_PROXY (object_p));

  if (JERRY_UNLIKELY (ecma_op_object_is_fast_array (object_p)))
  {
    ecma_fast_array_convert_to_normal (object_p);
  }

  object_p->type_flags_refs &= (ecma_object_descriptor_t) ~ECMA_OBJECT_FLAG_EXTENSIBLE;
} /* ecma_op_ordinary_object_prevent_extensions */

/**
 * Checks whether an object (excluding prototypes) has a named property
 *
 * @return true - if property is found
 *         false - otherwise
 */
ecma_value_t
ecma_op_ordinary_object_has_own_property (ecma_object_t *object_p, /**< the object */
                                          ecma_string_t *property_name_p) /**< property name */
{
  JERRY_ASSERT (!ECMA_OBJECT_IS_PROXY (object_p));

  ecma_property_t property =
    ecma_op_object_get_own_property (object_p, property_name_p, NULL, ECMA_PROPERTY_GET_NO_OPTIONS);

#if JERRY_BUILTIN_TYPEDARRAY
  if (JERRY_UNLIKELY (property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_THROW))
  {
    return ECMA_VALUE_ERROR;
  }
#endif /* JERRY_BUILTIN_TYPEDARRAY */

  JERRY_ASSERT (ECMA_PROPERTY_IS_FOUND (property) || property == ECMA_PROPERTY_TYPE_NOT_FOUND
                || property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP);

  return ecma_make_boolean_value (ECMA_PROPERTY_IS_FOUND (property));
} /* ecma_op_ordinary_object_has_own_property */

/**
 * Checks whether an object (excluding prototypes) has a named property. Handles proxy objects too.
 *
 * @return true - if property is found
 *         false - otherwise
 */
ecma_value_t
ecma_op_object_has_own_property (ecma_object_t *object_p, /**< the object */
                                 ecma_string_t *property_name_p) /**< property name */
{
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (object_p))
  {
    ecma_property_descriptor_t prop_desc;

    ecma_value_t status = ecma_proxy_object_get_own_property_descriptor (object_p, property_name_p, &prop_desc);

    if (ecma_is_value_true (status))
    {
      ecma_free_property_descriptor (&prop_desc);
    }

    return status;
  }
#endif /* JERRY_BUILTIN_PROXY */

  return ecma_op_ordinary_object_has_own_property (object_p, property_name_p);
} /* ecma_op_object_has_own_property */

#if JERRY_BUILTIN_WEAKREF || JERRY_BUILTIN_CONTAINER

/**
 * Set a weak reference from a container or WeakRefObject to a key object
 */
void
ecma_op_object_set_weak (ecma_object_t *object_p, /**< key object */
                         ecma_object_t *target_p) /**< target object */
{
  if (JERRY_UNLIKELY (ecma_op_object_is_fast_array (object_p)))
  {
    ecma_fast_array_convert_to_normal (object_p);
  }

  ecma_string_t *weak_refs_string_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_WEAK_REFS);
  ecma_property_t *property_p = ecma_find_named_property (object_p, weak_refs_string_p);
  ecma_collection_t *refs_p;

  if (property_p == NULL)
  {
    refs_p = ecma_new_collection ();

    ecma_property_value_t *value_p;
    ECMA_CREATE_INTERNAL_PROPERTY (object_p, weak_refs_string_p, property_p, value_p);
    ECMA_SET_INTERNAL_VALUE_POINTER (value_p->value, refs_p);
  }
  else
  {
    refs_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, (ECMA_PROPERTY_VALUE_PTR (property_p)->value));
  }

  const ecma_value_t target_value = ecma_make_object_value ((ecma_object_t *) target_p);
  for (uint32_t i = 0; i < refs_p->item_count; i++)
  {
    if (ecma_is_value_empty (refs_p->buffer_p[i]))
    {
      refs_p->buffer_p[i] = target_value;
      return;
    }
  }

  ecma_collection_push_back (refs_p, target_value);
} /* ecma_op_object_set_weak */

/**
 * Helper function to remove a weak reference to an object.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
void
ecma_op_object_unref_weak (ecma_object_t *object_p, /**< this argument */
                           ecma_value_t ref_holder) /**< key argument */
{
  ecma_string_t *weak_refs_string_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_WEAK_REFS);

  ecma_property_t *property_p = ecma_find_named_property (object_p, weak_refs_string_p);
  JERRY_ASSERT (property_p != NULL);

  ecma_collection_t *refs_p =
    ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, ECMA_PROPERTY_VALUE_PTR (property_p)->value);
  ecma_value_t *buffer_p = refs_p->buffer_p;

  while (true)
  {
    if (*buffer_p == ref_holder)
    {
      *buffer_p = ECMA_VALUE_EMPTY;
      return;
    }
    JERRY_ASSERT (buffer_p < refs_p->buffer_p + refs_p->item_count);
    buffer_p++;
  }
} /* ecma_op_object_unref_weak */

#endif /* JERRY_BUILTIN_WEAKREF || JERRY_BUILTIN_CONTAINER */
/**
 * Raise property redefinition error
 *
 * @return ECMA_VALUE_FALSE - if JERRY_PROP_SHOULD_THROW is not set
 *         raised TypeError - otherwise
 */
ecma_value_t
ecma_raise_property_redefinition (ecma_string_t *property_name_p, /**< property name */
                                  uint16_t flags) /**< property descriptor flags */
{
  JERRY_UNUSED (property_name_p);

  return ECMA_REJECT_WITH_FORMAT (flags & JERRY_PROP_SHOULD_THROW,
                                  "Cannot redefine property: %",
                                  ecma_make_prop_name_value (property_name_p));
} /* ecma_raise_property_redefinition */

/**
 * Raise readonly assignment error
 *
 * @return ECMA_VALUE_FALSE - if is_throw is true
 *         raised TypeError - otherwise
 */
ecma_value_t
ecma_raise_readonly_assignment (ecma_string_t *property_name_p, /**< property name */
                                bool is_throw) /**< is throw flag */
{
  JERRY_UNUSED (property_name_p);

  return ECMA_REJECT_WITH_FORMAT (is_throw,
                                  "Cannot assign to read only property '%'",
                                  ecma_make_prop_name_value (property_name_p));
} /* ecma_raise_readonly_assignment */

/**
 * @}
 * @}
 */
