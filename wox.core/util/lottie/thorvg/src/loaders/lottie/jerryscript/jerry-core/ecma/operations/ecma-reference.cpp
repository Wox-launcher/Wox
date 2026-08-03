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

#include "ecma-reference.h"

#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-lcache.h"
#include "ecma-lex-env.h"
#include "ecma-objects.h"
#include "ecma-proxy-object.h"

#include "jrt.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup references ECMA-Reference
 * @{
 */

/**
 * Resolve syntactic reference.
 *
 * @return ECMA_OBJECT_POINTER_ERROR - if the operation fails
 *         pointer to lexical environment - if the reference's base is resolved successfully,
 *         NULL - otherwise.
 */
ecma_object_t *
ecma_op_resolve_reference_base (ecma_object_t *lex_env_p, /**< starting lexical environment */
                                ecma_string_t *name_p) /**< identifier's name */
{
  JERRY_ASSERT (lex_env_p != NULL);

  while (true)
  {
    ecma_value_t has_binding = ecma_op_has_binding (lex_env_p, name_p);

#if JERRY_BUILTIN_PROXY
    if (ECMA_IS_VALUE_ERROR (has_binding))
    {
      return ECMA_OBJECT_POINTER_ERROR;
    }
#endif /* JERRY_BUILTIN_PROXY */

    if (ecma_is_value_true (has_binding))
    {
      return lex_env_p;
    }

    if (lex_env_p->u2.outer_reference_cp == JMEM_CP_NULL)
    {
      return NULL;
    }

    lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
  }
} /* ecma_op_resolve_reference_base */

/**
 * Check if the passed lexical environment is a global lexical environment
 *
 * @return true  - if the lexical environment is a global lexical environment
 *         false - otherwise
 */
static inline bool 
ecma_op_is_global_environment (ecma_object_t *lex_env_p) /**< lexical environment */
{
  JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);
#if JERRY_BUILTIN_REALMS
  JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL
                || (ecma_make_object_value (ecma_get_lex_env_binding_object (lex_env_p))
                    == ((ecma_global_object_t *) ecma_builtin_get_global ())->this_binding));
#else /* !JERRY_BUILTIN_REALMS */
  JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL
                || ecma_get_lex_env_binding_object (lex_env_p) == ecma_builtin_get_global ());
#endif /* JERRY_BUILTIN_REALMS */

  return lex_env_p->u2.outer_reference_cp == JMEM_CP_NULL;
} /* ecma_op_is_global_environment */

/**
 * Perform GetThisEnvironment and GetSuperBase operations
 *
 * See also: ECMAScript v6, 8.1.1.3.5
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_UNDEFINED - if the home object is null
 *         value of the [[HomeObject]].[[Prototype]] internal slot - otherwise
 */
ecma_value_t
ecma_op_resolve_super_base (ecma_object_t *lex_env_p) /**< starting lexical environment */
{
  while (true)
  {
    JERRY_ASSERT (lex_env_p != NULL);

    if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_CLASS
        && !ECMA_LEX_ENV_CLASS_IS_MODULE (lex_env_p))
    {
      ecma_object_t *home_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u1.home_object_cp);

#if JERRY_BUILTIN_PROXY
      if (ECMA_OBJECT_IS_PROXY (home_p))
      {
        return ecma_proxy_object_get_prototype_of (home_p);
      }
#endif /* JERRY_BUILTIN_PROXY */

      jmem_cpointer_t proto_cp = ecma_op_ordinary_object_get_prototype_of (home_p);

      if (proto_cp == JMEM_CP_NULL)
      {
        return ECMA_VALUE_NULL;
      }

      ecma_object_t *proto_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp);
      ecma_ref_object (proto_p);

      return ecma_make_object_value (proto_p);
    }

    if (lex_env_p->u2.outer_reference_cp == JMEM_CP_NULL)
    {
      break;
    }

    lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
  }

  return ECMA_VALUE_UNDEFINED;
} /* ecma_op_resolve_super_base */

/**
 * Helper method for HasBindig operation
 *
 * See also:
 *         ECMA-262 v6, 8.1.1.2.1 steps 7-9;
 *
 * @return ECMA_VALUE_TRUE - if the property is unscopable
 *         ECMA_VALUE_FALSE - if a the property is not unscopable
 *         ECMA_VALUE_ERROR - otherwise
 */
static ecma_value_t
ecma_op_is_prop_unscopable (ecma_object_t *binding_obj_p, /**< binding object */
                            ecma_string_t *prop_name_p) /**< property's name */
{
  ecma_value_t unscopables = ecma_op_object_get_by_symbol_id (binding_obj_p, LIT_GLOBAL_SYMBOL_UNSCOPABLES);

  if (ECMA_IS_VALUE_ERROR (unscopables))
  {
    return unscopables;
  }

  if (ecma_is_value_object (unscopables))
  {
    ecma_object_t *unscopables_obj_p = ecma_get_object_from_value (unscopables);
    ecma_value_t get_unscopables_value = ecma_op_object_get (unscopables_obj_p, prop_name_p);
    ecma_deref_object (unscopables_obj_p);

    if (ECMA_IS_VALUE_ERROR (get_unscopables_value))
    {
      return get_unscopables_value;
    }

    bool is_blocked = ecma_op_to_boolean (get_unscopables_value);

    ecma_free_value (get_unscopables_value);

    return ecma_make_boolean_value (is_blocked);
  }

  ecma_free_value (unscopables);

  return ECMA_VALUE_FALSE;
} /* ecma_op_is_prop_unscopable */

/**
 * Helper method for HasBindig operation
 *
 * See also:
 *         ECMA-262 v6, 8.1.1.2.1 steps 7-9;
 *
 * @return ECMA_VALUE_TRUE - if the property is unscopable
 *         ECMA_VALUE_FALSE - if a the property is not unscopable
 *         ECMA_VALUE_ERROR - otherwise
 */

/**
 * Resolve value corresponding to the given object environment reference.
 *
 * Note: the steps are already include the HasBindig operation steps
 *
 *  See also:
 *         ECMA-262 v6, 8.1.1.2.1
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_NOT_FOUND - if the binding not exists or blocked via @@unscopables
 *         result of the binding - otherwise
 */
ecma_value_t
ecma_op_object_bound_environment_resolve_reference_value (ecma_object_t *lex_env_p, /**< lexical environment */
                                                          ecma_string_t *name_p) /**< variable name */
{
  ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);
  ecma_value_t found_binding;

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (binding_obj_p))
  {
    found_binding = ecma_proxy_object_has (binding_obj_p, name_p);

    if (!ecma_is_value_true (found_binding))
    {
      return ECMA_IS_VALUE_ERROR (found_binding) ? found_binding : ECMA_VALUE_NOT_FOUND;
    }
  }
  else
  {
#endif /* JERRY_BUILTIN_PROXY */
    found_binding = ecma_op_object_find (binding_obj_p, name_p);

    if (ECMA_IS_VALUE_ERROR (found_binding) || !ecma_is_value_found (found_binding))
    {
      return found_binding;
    }

    if (JERRY_LIKELY (ecma_op_is_global_environment (lex_env_p)))
    {
      return found_binding;
    }
#if JERRY_BUILTIN_PROXY
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_value_t blocked = ecma_op_is_prop_unscopable (binding_obj_p, name_p);

  if (ecma_is_value_false (blocked))
  {
#if JERRY_BUILTIN_PROXY
    if (ECMA_OBJECT_IS_PROXY (binding_obj_p))
    {
      return ecma_proxy_object_get (binding_obj_p, name_p, ecma_make_object_value (binding_obj_p));
    }
#endif /* JERRY_BUILTIN_PROXY */
    return found_binding;
  }

#if JERRY_BUILTIN_PROXY
  if (!ECMA_OBJECT_IS_PROXY (binding_obj_p))
  {
    ecma_free_value (found_binding);
  }
#endif /* JERRY_BUILTIN_PROXY */

  return ECMA_IS_VALUE_ERROR (blocked) ? blocked : ECMA_VALUE_NOT_FOUND;
} /* ecma_op_object_bound_environment_resolve_reference_value */

/**
 * Resolve value corresponding to reference.
 *
 * @return value of the reference
 */
ecma_value_t
ecma_op_resolve_reference_value (ecma_object_t *lex_env_p, /**< starting lexical environment */
                                 ecma_string_t *name_p) /**< identifier's name */
{
  JERRY_ASSERT (lex_env_p != NULL);

  while (true)
  {
    switch (ecma_get_lex_env_type (lex_env_p))
    {
      case ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE:
      {
        ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

        if (property_p == NULL)
        {
          break;
        }

        ecma_property_value_t *property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

        if (JERRY_UNLIKELY (property_value_p->value == ECMA_VALUE_UNINITIALIZED))
        {
          return ecma_raise_reference_error (ECMA_ERR_LET_CONST_NOT_INITIALIZED);
        }

        return ecma_fast_copy_value (property_value_p->value);
      }
      case ECMA_LEXICAL_ENVIRONMENT_CLASS:
      {
#if JERRY_MODULE_SYSTEM
        if (ECMA_LEX_ENV_CLASS_IS_MODULE (lex_env_p))
        {
          ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

          if (property_p == NULL)
          {
            break;
          }

          ecma_property_value_t *property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

          if (!(*property_p & ECMA_PROPERTY_FLAG_DATA))
          {
            property_value_p = ecma_get_property_value_from_named_reference (property_value_p);
          }

          if (JERRY_UNLIKELY (property_value_p->value == ECMA_VALUE_UNINITIALIZED))
          {
            return ecma_raise_reference_error (ECMA_ERR_LET_CONST_NOT_INITIALIZED);
          }

          return ecma_fast_copy_value (property_value_p->value);
        }
#endif /* JERRY_MODULE_SYSTEM */
        break;
      }
      default:
      {
        JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);

        if (ecma_op_is_global_environment (lex_env_p))
        {
#if JERRY_LCACHE
          ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);
          ecma_property_t *property_p = ecma_lcache_lookup (binding_obj_p, name_p);

          if (property_p != NULL)
          {
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

            ecma_value_t base_value = ecma_make_object_value (binding_obj_p);
            return ecma_op_function_call (getter_p, base_value, NULL, 0);
          }
#endif /* JERRY_LCACHE */
        }

        ecma_value_t result = ecma_op_object_bound_environment_resolve_reference_value (lex_env_p, name_p);

        if (ecma_is_value_found (result))
        {
          /* Note: the result may contains ECMA_VALUE_ERROR */
          return result;
        }
        break;
      }
    }

    if (lex_env_p->u2.outer_reference_cp == JMEM_CP_NULL)
    {
      break;
    }

    lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
  }

#if JERRY_ERROR_MESSAGES
  ecma_value_t name_val = ecma_make_string_value (name_p);
  ecma_value_t error_value =
    ecma_raise_standard_error_with_format (JERRY_ERROR_REFERENCE, "% is not defined", name_val);
#else /* JERRY_ERROR_MESSAGES */
  ecma_value_t error_value = ecma_raise_reference_error (ECMA_ERR_EMPTY);
#endif /* !JERRY_ERROR_MESSAGES */
  return error_value;
} /* ecma_op_resolve_reference_value */

/**
 * @}
 * @}
 */
