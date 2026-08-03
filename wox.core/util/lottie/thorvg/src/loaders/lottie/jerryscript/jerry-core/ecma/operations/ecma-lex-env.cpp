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

#include "ecma-lex-env.h"

#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-proxy-object.h"

#include "jcontext.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup lexicalenvironment Lexical environment
 * @{
 *
 * \addtogroup globallexicalenvironment Global lexical environment
 * @{
 */

/**
 * Initialize Global environment
 */
void
ecma_init_global_environment (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_CONTEXT (global_object_p) = ecma_builtin_create_global_object ();
} /* ecma_init_global_environment */

/**
 * Finalize Global environment
 */
void
ecma_finalize_global_environment (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  /* After this point the gc can free the global object, but the global_object_p pointer
   * is not set to NULL because the global object might still be used before the free. */
  ecma_deref_object ((ecma_object_t *) JERRY_CONTEXT (global_object_p));
} /* ecma_finalize_global_environment */

/**
 * Get reference to Global lexical environment
 * without increasing its reference count.
 *
 * @return pointer to the object's instance
 */
ecma_object_t *
ecma_get_global_environment (ecma_object_t *global_object_p) /**< global object */
{
  JERRY_ASSERT (global_object_p != NULL && ecma_builtin_is_global (global_object_p));
  return ECMA_GET_NON_NULL_POINTER (ecma_object_t, ((ecma_global_object_t *) global_object_p)->global_env_cp);
} /* ecma_get_global_environment */

/**
 * Create the global lexical block on top of the global environment.
 */
void
ecma_create_global_lexical_block (ecma_object_t *global_object_p) /**< global object */
{
  JERRY_ASSERT (global_object_p != NULL && ecma_builtin_is_global (global_object_p));

  ecma_global_object_t *real_global_object_p = (ecma_global_object_t *) global_object_p;

  if (real_global_object_p->global_scope_cp == real_global_object_p->global_env_cp)
  {
    ecma_object_t *global_scope_p = ecma_create_decl_lex_env (ecma_get_global_environment (global_object_p));
    global_scope_p->type_flags_refs |= ECMA_OBJECT_FLAG_BLOCK;
    ECMA_SET_NON_NULL_POINTER (real_global_object_p->global_scope_cp, global_scope_p);
    ecma_deref_object (global_scope_p);
  }
} /* ecma_create_global_lexical_block */

/**
 * Raise the appropriate error when setting a binding is failed
 *
 * @return ECMA_VALUE_EMPTY or ECMA_VALUE_ERROR
 */
ecma_value_t
ecma_op_raise_set_binding_error (ecma_property_t *property_p, /**< property */
                                 bool is_strict) /**< flag indicating strict mode */
{
  JERRY_UNUSED (property_p);

  const ecma_property_t expected_bits = (ECMA_PROPERTY_FLAG_DATA | ECMA_PROPERTY_FLAG_ENUMERABLE);

  if ((*property_p & expected_bits) == expected_bits)
  {
    ecma_property_value_t *property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

    if (JERRY_UNLIKELY (property_value_p->value == ECMA_VALUE_UNINITIALIZED))
    {
      return ecma_raise_reference_error (ECMA_ERR_LET_CONST_NOT_INITIALIZED);
    }

    JERRY_ASSERT (!ecma_is_property_writable (*property_p));

    return ecma_raise_type_error (ECMA_ERR_CONSTANT_BINDINGS_CANNOT_BE_REASSIGNED);
  }

  if (is_strict)
  {
    return ecma_raise_type_error (ECMA_ERR_BINDING_CANNOT_SET);
  }
  return ECMA_VALUE_EMPTY;
} /* ecma_op_raise_set_binding_error */

/**
 * Get reference to Global lexical scope
 * without increasing its reference count.
 *
 * @return pointer to the object's instance
 */
ecma_object_t *
ecma_get_global_scope (ecma_object_t *global_object_p) /**< global object */
{
  JERRY_ASSERT (global_object_p != NULL && ecma_builtin_is_global (global_object_p));
  return ECMA_GET_NON_NULL_POINTER (ecma_object_t, ((ecma_global_object_t *) global_object_p)->global_scope_cp);
} /* ecma_get_global_scope */

/**
 * @}
 */

/**
 * HasBinding operation.
 *
 * See also: ECMA-262 v5, 10.2.1
 *
 * @return true / false
 */
ecma_value_t
ecma_op_has_binding (ecma_object_t *lex_env_p, /**< lexical environment */
                     ecma_string_t *name_p) /**< argument N */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));

  ecma_lexical_environment_type_t lex_env_type = ecma_get_lex_env_type (lex_env_p);

  switch (lex_env_type)
  {
    case ECMA_LEXICAL_ENVIRONMENT_CLASS:
    {
      if (!ECMA_LEX_ENV_CLASS_IS_MODULE (lex_env_p))
      {
        return ECMA_VALUE_FALSE;
      }
      /* FALLTHRU */
    }
    case ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE:
    {
      ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

      return ecma_make_boolean_value (property_p != NULL);
    }
    default:
    {
      JERRY_ASSERT (lex_env_type == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);

      ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);

      return ecma_op_object_has_property (binding_obj_p, name_p);
    }
  }
} /* ecma_op_has_binding */

/**
 * CreateMutableBinding operation.
 *
 * See also: ECMA-262 v5, 10.2.1
 *
 * @return ECMA_PROPERTY_POINTER_ERROR - if the operation raises error
 *         pointer to the created property - if the binding was created into a declarative environment
 *         NULL - otherwise
 */
ecma_property_t *
ecma_op_create_mutable_binding (ecma_object_t *lex_env_p, /**< lexical environment */
                                ecma_string_t *name_p, /**< argument N */
                                bool is_deletable) /**< argument D */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));
  JERRY_ASSERT (name_p != NULL);

  if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
  {
    uint8_t prop_attributes = ECMA_PROPERTY_FLAG_WRITABLE;

    if (is_deletable)
    {
      prop_attributes = (uint8_t) (prop_attributes | ECMA_PROPERTY_FLAG_CONFIGURABLE);
    }

    ecma_property_t *prop_p;

    ecma_create_named_data_property (lex_env_p, name_p, prop_attributes, &prop_p);
    return prop_p;
  }
  else
  {
    JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);

    ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);

#if JERRY_BUILTIN_PROXY && JERRY_BUILTIN_REALMS
    if (ECMA_OBJECT_IS_PROXY (binding_obj_p))
    {
      ecma_value_t result = ecma_proxy_object_is_extensible (binding_obj_p);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        return ECMA_PROPERTY_POINTER_ERROR;
      }

      if (result == ECMA_VALUE_FALSE)
      {
        return NULL;
      }
    }
    else if (!ecma_op_ordinary_object_is_extensible (binding_obj_p))
    {
      return NULL;
    }
#else /* !JERRY_BUILTIN_PROXY || !JERRY_BUILTIN_REALMS */
    if (!ecma_op_ordinary_object_is_extensible (binding_obj_p))
    {
      return NULL;
    }
#endif /* JERRY_BUILTIN_PROXY && JERRY_BUILTIN_REALMS */

    const uint32_t flags = ECMA_PROPERTY_ENUMERABLE_WRITABLE | JERRY_PROP_SHOULD_THROW;

    ecma_value_t completion =
      ecma_builtin_helper_def_prop (binding_obj_p,
                                    name_p,
                                    ECMA_VALUE_UNDEFINED,
                                    is_deletable ? flags | ECMA_PROPERTY_FLAG_CONFIGURABLE : flags);

    if (ECMA_IS_VALUE_ERROR (completion))
    {
      return ECMA_PROPERTY_POINTER_ERROR;
    }
    else
    {
      JERRY_ASSERT (ecma_is_value_boolean (completion));
    }
  }

  return NULL;
} /* ecma_op_create_mutable_binding */

/**
 * SetMutableBinding operation.
 *
 * See also: ECMA-262 v5, 10.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_op_set_mutable_binding (ecma_object_t *lex_env_p, /**< lexical environment */
                             ecma_string_t *name_p, /**< argument N */
                             ecma_value_t value, /**< argument V */
                             bool is_strict) /**< argument S */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));
  JERRY_ASSERT (name_p != NULL);

  switch (ecma_get_lex_env_type (lex_env_p))
  {
    case ECMA_LEXICAL_ENVIRONMENT_CLASS:
    {
      if (!ECMA_LEX_ENV_CLASS_IS_MODULE (lex_env_p))
      {
        return ECMA_VALUE_EMPTY;
      }
      /* FALLTHRU */
    }
    case ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE:
    {
      ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

      if (JERRY_UNLIKELY (property_p == NULL))
      {
        property_p = ecma_op_create_mutable_binding (lex_env_p, name_p, is_strict);
        JERRY_ASSERT (property_p != ECMA_PROPERTY_POINTER_ERROR);
      }

      JERRY_ASSERT (property_p != NULL && ECMA_PROPERTY_IS_RAW_DATA (*property_p));
      JERRY_ASSERT (!(*property_p & ECMA_PROPERTY_FLAG_WRITABLE) || (*property_p & ECMA_PROPERTY_FLAG_DATA));

      if ((*property_p & ECMA_PROPERTY_FLAG_WRITABLE))
      {
        ecma_property_value_t *property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

        JERRY_ASSERT (property_value_p->value != ECMA_VALUE_UNINITIALIZED);

        ecma_named_data_property_assign_value (lex_env_p, property_value_p, value);
        return ECMA_VALUE_EMPTY;
      }

      return ecma_op_raise_set_binding_error (property_p, is_strict);
    }
    default:
    {
      JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);

      ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);

      ecma_value_t completion = ecma_op_object_put (binding_obj_p, name_p, value, is_strict);

      if (ECMA_IS_VALUE_ERROR (completion))
      {
        return completion;
      }

      JERRY_ASSERT (ecma_is_value_boolean (completion));
      return ECMA_VALUE_EMPTY;
    }
  }
} /* ecma_op_set_mutable_binding */

/**
 * GetBindingValue operation.
 *
 * See also: ECMA-262 v5, 10.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_op_get_binding_value (ecma_object_t *lex_env_p, /**< lexical environment */
                           ecma_string_t *name_p, /**< argument N */
                           bool is_strict) /**< argument S */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));
  JERRY_ASSERT (name_p != NULL);

  if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
  {
    ecma_property_value_t *prop_value_p = ecma_get_named_data_property (lex_env_p, name_p);

    return ecma_copy_value (prop_value_p->value);
  }
  else
  {
    JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);

    ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);

    ecma_value_t result = ecma_op_object_find (binding_obj_p, name_p);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      return result;
    }

    if (!ecma_is_value_found (result))
    {
      if (is_strict)
      {
        result = ecma_raise_reference_error (ECMA_ERR_BINDING_NOT_EXIST_OR_UNINITIALIZED);
      }
      else
      {
        result = ECMA_VALUE_UNDEFINED;
      }
    }

    return result;
  }
} /* ecma_op_get_binding_value */

/**
 * DeleteBinding operation.
 *
 * See also: ECMA-262 v5, 10.2.1
 *
 * @return ecma value
 *         Return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE/FALSE} - depends on whether the binding can be deleted
 */
ecma_value_t
ecma_op_delete_binding (ecma_object_t *lex_env_p, /**< lexical environment */
                        ecma_string_t *name_p) /**< argument N */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));
  JERRY_ASSERT (name_p != NULL);

  if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
  {
    ecma_property_t *prop_p = ecma_find_named_property (lex_env_p, name_p);
    ecma_value_t ret_val;

    if (prop_p == NULL)
    {
      ret_val = ECMA_VALUE_TRUE;
    }
    else
    {
      JERRY_ASSERT (ECMA_PROPERTY_IS_RAW_DATA (*prop_p));

      if (!ecma_is_property_configurable (*prop_p))
      {
        ret_val = ECMA_VALUE_FALSE;
      }
      else
      {
        ecma_delete_property (lex_env_p, ECMA_PROPERTY_VALUE_PTR (prop_p));

        ret_val = ECMA_VALUE_TRUE;
      }
    }

    return ret_val;
  }
  else
  {
    JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);

    ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);

    return ecma_op_object_delete (binding_obj_p, name_p, false);
  }
} /* ecma_op_delete_binding */

/**
 * ImplicitThisValue operation.
 *
 * See also: ECMA-262 v5, 10.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_op_implicit_this_value (ecma_object_t *lex_env_p) /**< lexical environment */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));

  if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
  {
    return ECMA_VALUE_UNDEFINED;
  }
  else
  {
    JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND);

    ecma_object_t *binding_obj_p = ecma_get_lex_env_binding_object (lex_env_p);
    ecma_ref_object (binding_obj_p);

    return ecma_make_object_value (binding_obj_p);
  }
} /* ecma_op_implicit_this_value */

/**
 * CreateImmutableBinding operation.
 *
 * See also: ECMA-262 v5, 10.2.1
 */
void
ecma_op_create_immutable_binding (ecma_object_t *lex_env_p, /**< lexical environment */
                                  ecma_string_t *name_p, /**< argument N */
                                  ecma_value_t value) /**< argument V */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));
  JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);

  /*
   * Warning:
   *         Whether immutable bindings are deletable seems not to be defined by ECMA v5.
   */
  ecma_property_value_t *prop_value_p = ecma_create_named_data_property (lex_env_p, name_p, ECMA_PROPERTY_FIXED, NULL);

  prop_value_p->value = ecma_copy_value_if_not_object (value);
} /* ecma_op_create_immutable_binding */

/**
 * InitializeBinding operation.
 *
 * See also: ECMA-262 v6, 8.1.1.1.4
 */
void
ecma_op_initialize_binding (ecma_object_t *lex_env_p, /**< lexical environment */
                            ecma_string_t *name_p, /**< argument N */
                            ecma_value_t value) /**< argument V */
{
  JERRY_ASSERT (lex_env_p != NULL && ecma_is_lexical_environment (lex_env_p));
  JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);

  ecma_property_t *prop_p = ecma_find_named_property (lex_env_p, name_p);
  JERRY_ASSERT (prop_p != NULL && ECMA_PROPERTY_IS_RAW_DATA (*prop_p));

  ecma_property_value_t *prop_value_p = ECMA_PROPERTY_VALUE_PTR (prop_p);
  JERRY_ASSERT (prop_value_p->value == ECMA_VALUE_UNINITIALIZED);

  prop_value_p->value = ecma_copy_value_if_not_object (value);
} /* ecma_op_initialize_binding */

/**
 * BindThisValue operation for an empty lexical environment
 *
 * See also: ECMA-262 v6, 8.1.1.3.1
 */
void
ecma_op_create_environment_record (ecma_object_t *lex_env_p, /**< lexical environment */
                                   ecma_value_t this_binding, /**< this binding value */
                                   ecma_object_t *func_obj_p) /**< function object */
{
  JERRY_ASSERT (lex_env_p != NULL);
  JERRY_ASSERT (ecma_is_value_object (this_binding) || this_binding == ECMA_VALUE_UNINITIALIZED);

  ecma_environment_record_t *environment_record_p;
  environment_record_p = (ecma_environment_record_t *) jmem_heap_alloc_block (sizeof (ecma_environment_record_t));

  environment_record_p->this_binding = this_binding;
  environment_record_p->function_object = ecma_make_object_value (func_obj_p);

  ecma_string_t *property_name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_ENVIRONMENT_RECORD);

  ecma_property_t *property_p;
  ecma_property_value_t *prop_value_p;
  ECMA_CREATE_INTERNAL_PROPERTY (lex_env_p, property_name_p, property_p, prop_value_p);

  ECMA_SET_INTERNAL_VALUE_POINTER (prop_value_p->value, environment_record_p);
} /* ecma_op_create_environment_record */

/**
 * GetThisEnvironment operation.
 *
 * See also: ECMA-262 v6, 8.3.2
 *
 * @return property pointer for the internal [[ThisBindingValue]] property
 */
ecma_environment_record_t *
ecma_op_get_environment_record (ecma_object_t *lex_env_p) /**< lexical environment */
{
  JERRY_ASSERT (lex_env_p != NULL);

  ecma_string_t *property_name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_ENVIRONMENT_RECORD);
  while (true)
  {
    if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
    {
      ecma_property_t *property_p = ecma_find_named_property (lex_env_p, property_name_p);

      if (property_p != NULL)
      {
        ecma_property_value_t *property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
        return ECMA_GET_INTERNAL_VALUE_POINTER (ecma_environment_record_t, property_value_p->value);
      }
    }

    if (lex_env_p->u2.outer_reference_cp == JMEM_CP_NULL)
    {
      break;
    }

    lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
  }

  return NULL;
} /* ecma_op_get_environment_record */

/**
 * Get the environment record [[ThisBindingStatus]] internal property.
 *
 * See also: ECMA-262 v6, 8.1.1.3
 *
 * @return true - if the status is "initialized"
 *         false - otherwise
 */
bool
ecma_op_this_binding_is_initialized (ecma_environment_record_t *environment_record_p) /**< environment record */
{
  JERRY_ASSERT (environment_record_p != NULL);

  return environment_record_p->this_binding != ECMA_VALUE_UNINITIALIZED;
} /* ecma_op_this_binding_is_initialized */

/**
 * BindThisValue operation.
 *
 * See also: ECMA-262 v6, 8.1.1.3.1
 */
void
ecma_op_bind_this_value (ecma_environment_record_t *environment_record_p, /**< environment record */
                         ecma_value_t this_binding) /**< this binding value */
{
  JERRY_ASSERT (environment_record_p != NULL);
  JERRY_ASSERT (ecma_is_value_object (this_binding));
  JERRY_ASSERT (!ecma_op_this_binding_is_initialized (environment_record_p));

  environment_record_p->this_binding = this_binding;
} /* ecma_op_bind_this_value */

/**
 * GetThisBinding operation.
 *
 * See also: ECMA-262 v6, 8.1.1.3.4
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ecma-object - otherwise
 */
ecma_value_t
ecma_op_get_this_binding (ecma_object_t *lex_env_p) /**< lexical environment */
{
  JERRY_ASSERT (lex_env_p != NULL);

  ecma_environment_record_t *environment_record_p = ecma_op_get_environment_record (lex_env_p);
  JERRY_ASSERT (environment_record_p != NULL);

  ecma_value_t this_value = environment_record_p->this_binding;

  if (this_value == ECMA_VALUE_UNINITIALIZED)
  {
    return ecma_raise_reference_error (ECMA_ERR_CALL_SUPER_CONSTRUCTOR_DERIVED_CLASS_BEFORE_THIS);
  }

  ecma_ref_object (ecma_get_object_from_value (this_value));

  return this_value;
} /* ecma_op_get_this_binding */

/**
 * @}
 * @}
 */
