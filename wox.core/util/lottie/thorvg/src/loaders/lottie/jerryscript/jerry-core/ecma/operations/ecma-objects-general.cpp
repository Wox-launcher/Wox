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

#include "ecma-objects-general.h"

#include "ecma-arguments-object.h"
#include "ecma-array-object.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-proxy-object.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaobjectsinternalops ECMA objects' operations
 * @{
 */

/**
 * 'Object' object creation operation with no arguments.
 *
 * See also: ECMA-262 v5, 15.2.2.1
 *
 * @return pointer to newly created 'Object' object
 */
ecma_object_t *
ecma_op_create_object_object_noarg (void)
{
  ecma_object_t *object_prototype_p = ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE);

  /* 3., 4., 6., 7. */
  return ecma_op_create_object_object_noarg_and_set_prototype (object_prototype_p);
} /* ecma_op_create_object_object_noarg */

/**
 * Object creation operation with no arguments.
 * It sets the given prototype to the newly created object.
 *
 * See also: ECMA-262 v5, 15.2.2.1, 15.2.3.5
 *
 * @return pointer to newly created object
 */
ecma_object_t *
ecma_op_create_object_object_noarg_and_set_prototype (ecma_object_t *object_prototype_p) /**< pointer to prototype of
                                                                                              the object
                                                                                              (can be NULL) */
{
  ecma_object_t *obj_p = ecma_create_object (object_prototype_p, 0, ECMA_OBJECT_TYPE_GENERAL);

  /*
   * [[Class]] property of ECMA_OBJECT_TYPE_GENERAL type objects
   * without ECMA_INTERNAL_PROPERTY_CLASS internal property
   * is "Object".
   *
   * See also: ecma_object_get_class_name
   */

  return obj_p;
} /* ecma_op_create_object_object_noarg_and_set_prototype */

/**
 * [[Delete]] ecma general object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *          ECMA-262 v5, 8.12.7
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_general_object_delete (ecma_object_t *obj_p, /**< the object */
                               ecma_string_t *property_name_p, /**< property name */
                               bool is_throw) /**< flag that controls failure handling */
{
  JERRY_ASSERT (obj_p != NULL && !ecma_is_lexical_environment (obj_p));
  JERRY_ASSERT (property_name_p != NULL);

  /* 1. */
  ecma_property_ref_t property_ref;

  ecma_property_t property =
    ecma_op_object_get_own_property (obj_p, property_name_p, &property_ref, ECMA_PROPERTY_GET_NO_OPTIONS);

  /* 2. */
  if (!ECMA_PROPERTY_IS_FOUND (property))
  {
    JERRY_ASSERT (property == ECMA_PROPERTY_TYPE_NOT_FOUND || property == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP);
    return ECMA_VALUE_TRUE;
  }

  /* 3. */
  if (!ecma_is_property_configurable (property))
  {
    /* 4. */
    if (is_throw)
    {
      return ecma_raise_type_error (ECMA_ERR_EXPECTED_A_CONFIGURABLE_PROPERTY);
    }

    /* 5. */
    return ECMA_VALUE_FALSE;
  }

  ecma_object_type_t type = ecma_get_object_type (obj_p);

  if (type == ECMA_OBJECT_TYPE_ARRAY && ecma_array_object_delete_property (obj_p, property_name_p))
  {
    return ECMA_VALUE_TRUE;
  }

  /* a. */
  ecma_delete_property (obj_p, property_ref.value_p);

  if (property & ECMA_PROPERTY_FLAG_BUILT_IN)
  {
    switch (type)
    {
      case ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION:
      {
        if (ecma_builtin_function_is_routine (obj_p))
        {
          ecma_builtin_routine_delete_built_in_property (obj_p, property_name_p);
          break;
        }
        /* FALLTHRU */
      }
      case ECMA_OBJECT_TYPE_BUILT_IN_GENERAL:
      case ECMA_OBJECT_TYPE_BUILT_IN_CLASS:
      case ECMA_OBJECT_TYPE_BUILT_IN_ARRAY:
      {
        ecma_builtin_delete_built_in_property (obj_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_CLASS:
      {
        JERRY_ASSERT (ecma_object_class_is (obj_p, ECMA_OBJECT_CLASS_ARGUMENTS));
        ecma_op_arguments_delete_built_in_property (obj_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_FUNCTION:
      {
        ecma_op_function_delete_built_in_property (obj_p, property_name_p);
        break;
      }
      case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
      {
        ecma_op_bound_function_delete_built_in_property (obj_p, property_name_p);
        break;
      }
      default:
      {
        JERRY_UNREACHABLE ();
        break;
      }
    }
  }

  /* b. */
  return ECMA_VALUE_TRUE;
} /* ecma_op_general_object_delete */

/**
 * Property invocation order during [[DefaultValue]] operation with string hint
 */
static const lit_magic_string_id_t to_primitive_string_hint_method_names[2] = {
  LIT_MAGIC_STRING_TO_STRING_UL, /**< toString operation */
  LIT_MAGIC_STRING_VALUE_OF_UL, /**< valueOf operation */
};

/**
 * Property invocation order during [[DefaultValue]] operation with non string hint
 */
static const lit_magic_string_id_t to_primitive_non_string_hint_method_names[2] = {
  LIT_MAGIC_STRING_VALUE_OF_UL, /**< valueOf operation */
  LIT_MAGIC_STRING_TO_STRING_UL, /**< toString operation */
};

/**
 * Hints for the ecma general object's toPrimitive operation
 */
static const lit_magic_string_id_t hints[3] = {
  LIT_MAGIC_STRING_DEFAULT, /**< "default" hint */
  LIT_MAGIC_STRING_NUMBER, /**< "number" hint */
  LIT_MAGIC_STRING_STRING, /**< "string" hint */
};

/**
 * [[DefaultValue]] ecma general object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *          ECMA-262 v5, 8.12.8
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_general_object_default_value (ecma_object_t *obj_p, /**< the object */
                                      ecma_preferred_type_hint_t hint) /**< hint on preferred result type */
{
  JERRY_ASSERT (obj_p != NULL && !ecma_is_lexical_environment (obj_p));

  ecma_value_t obj_value = ecma_make_object_value (obj_p);

  ecma_value_t exotic_to_prim = ecma_op_get_method_by_symbol_id (obj_value, LIT_GLOBAL_SYMBOL_TO_PRIMITIVE);

  if (ECMA_IS_VALUE_ERROR (exotic_to_prim))
  {
    return exotic_to_prim;
  }

  if (!ecma_is_value_undefined (exotic_to_prim))
  {
    ecma_object_t *call_func_p = ecma_get_object_from_value (exotic_to_prim);
    ecma_value_t argument = ecma_make_magic_string_value (hints[hint]);

    ecma_value_t result = ecma_op_function_call (call_func_p, obj_value, &argument, 1);

    ecma_free_value (exotic_to_prim);

    if (ECMA_IS_VALUE_ERROR (result) || !ecma_is_value_object (result))
    {
      return result;
    }

    ecma_free_value (result);

    return ecma_raise_type_error (ECMA_ERR_RESULT_OF_DEFAULTVALUE_IS_INVALID);
  }

  ecma_free_value (exotic_to_prim);

  if (hint == ECMA_PREFERRED_TYPE_NO)
  {
    hint = ECMA_PREFERRED_TYPE_NUMBER;
  }

  return ecma_op_general_object_ordinary_value (obj_p, hint);
} /* ecma_op_general_object_default_value */

/**
 * Ecma general object's OrdinaryToPrimitive operation
 *
 * See also:
 *          ECMA-262 v6 7.1.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_general_object_ordinary_value (ecma_object_t *obj_p, /**< the object */
                                       ecma_preferred_type_hint_t hint) /**< hint on preferred result type */
{
  const lit_magic_string_id_t *function_name_ids_p =
    (hint == ECMA_PREFERRED_TYPE_STRING ? to_primitive_string_hint_method_names
                                        : to_primitive_non_string_hint_method_names);

  for (uint32_t i = 0; i < 2; i++)
  {
    ecma_value_t function_value = ecma_op_object_get_by_magic_id (obj_p, function_name_ids_p[i]);

    if (ECMA_IS_VALUE_ERROR (function_value))
    {
      return function_value;
    }

    ecma_value_t call_completion = ECMA_VALUE_EMPTY;

    if (ecma_op_is_callable (function_value))
    {
      ecma_object_t *func_obj_p = ecma_get_object_from_value (function_value);

      call_completion = ecma_op_function_call (func_obj_p, ecma_make_object_value (obj_p), NULL, 0);
    }

    ecma_free_value (function_value);

    if (ECMA_IS_VALUE_ERROR (call_completion)
        || (!ecma_is_value_empty (call_completion) && !ecma_is_value_object (call_completion)))
    {
      return call_completion;
    }

    ecma_free_value (call_completion);
  }

  return ecma_raise_type_error (ECMA_ERR_RESULT_OF_DEFAULTVALUE_IS_INVALID);
} /* ecma_op_general_object_ordinary_value */

/**
 * Special types for ecma_op_general_object_define_own_property.
 */
typedef enum
{
  ECMA_OP_OBJECT_DEFINE_GENERIC = 1, /**< generic property */
  ECMA_OP_OBJECT_DEFINE_ACCESSOR = 0, /**< accessor property */
  ECMA_OP_OBJECT_DEFINE_DATA = ECMA_PROPERTY_FLAG_DATA /**< data property */
} ecma_op_object_define_own_property_type_t;

/**
 * [[DefineOwnProperty]] ecma general object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *          ECMA-262 v5, 8.12.9
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_general_object_define_own_property (ecma_object_t *object_p, /**< the object */
                                            ecma_string_t *property_name_p, /**< property name */
                                            const ecma_property_descriptor_t *property_desc_p) /**< property
                                                                                                *   descriptor */
{
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (object_p))
  {
    return ecma_proxy_object_define_own_property (object_p, property_name_p, property_desc_p);
  }
#endif /* JERRY_BUILTIN_PROXY */

  JERRY_ASSERT (object_p != NULL && !ecma_is_lexical_environment (object_p));
  JERRY_ASSERT (!ecma_op_object_is_fast_array (object_p));
  JERRY_ASSERT (property_name_p != NULL);

  uint8_t property_desc_type = ECMA_OP_OBJECT_DEFINE_GENERIC;

  if (property_desc_p->flags & (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE_DEFINED))
  {
    /* A property descriptor cannot be both named data and named accessor. */
    JERRY_ASSERT ((property_desc_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED))
                  != (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED));
    property_desc_type = ECMA_OP_OBJECT_DEFINE_DATA;
  }
  else if (property_desc_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED))
  {
    JERRY_ASSERT (!(property_desc_p->flags & JERRY_PROP_IS_WRITABLE_DEFINED));
    property_desc_type = ECMA_OP_OBJECT_DEFINE_ACCESSOR;
  }

  /* These three asserts ensures that a new property is created with the appropriate default flags.
   * E.g. if JERRY_PROP_IS_CONFIGURABLE_DEFINED is false, the newly created property must be non-configurable. */
  JERRY_ASSERT ((property_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE_DEFINED)
                || !(property_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE));
  JERRY_ASSERT ((property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE_DEFINED)
                || !(property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE));
  JERRY_ASSERT ((property_desc_p->flags & JERRY_PROP_IS_WRITABLE_DEFINED)
                || !(property_desc_p->flags & JERRY_PROP_IS_WRITABLE));

  /* 1. */
  ecma_extended_property_ref_t ext_property_ref = { {NULL, }, NULL };
  ecma_property_t current_prop;

  current_prop = ecma_op_object_get_own_property (object_p,
                                                  property_name_p,
                                                  &ext_property_ref.property_ref,
                                                  ECMA_PROPERTY_GET_VALUE | ECMA_PROPERTY_GET_EXT_REFERENCE);

  if (!ECMA_PROPERTY_IS_FOUND (current_prop))
  {
    JERRY_ASSERT (current_prop == ECMA_PROPERTY_TYPE_NOT_FOUND
                  || current_prop == ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP);

    /* 3. */
    if (!ecma_op_ordinary_object_is_extensible (object_p))
    {
      /* 2. */
      return ECMA_REJECT_WITH_FORMAT (property_desc_p->flags & JERRY_PROP_SHOULD_THROW,
                                      "Cannot define property '%', object is not extensible",
                                      ecma_make_prop_name_value (property_name_p));
    }

    /* 4. */
    uint8_t prop_attributes = (uint8_t) (property_desc_p->flags & ECMA_PROPERTY_FLAGS_MASK);

    if (property_desc_type != ECMA_OP_OBJECT_DEFINE_ACCESSOR)
    {
      /* a. */
      JERRY_ASSERT (property_desc_type == ECMA_OP_OBJECT_DEFINE_GENERIC
                    || property_desc_type == ECMA_OP_OBJECT_DEFINE_DATA);

      ecma_property_value_t *new_prop_value_p =
        ecma_create_named_data_property (object_p, property_name_p, prop_attributes, NULL);

      JERRY_ASSERT ((property_desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
                    || ecma_is_value_undefined (property_desc_p->value));

      new_prop_value_p->value = ecma_copy_value_if_not_object (property_desc_p->value);
    }
    else
    {
      /* b. */
      ecma_create_named_accessor_property (object_p,
                                           property_name_p,
                                           property_desc_p->get_p,
                                           property_desc_p->set_p,
                                           prop_attributes,
                                           NULL);
    }

    return ECMA_VALUE_TRUE;
  }

  /* 6. */
  const bool is_current_configurable = ecma_is_property_configurable (current_prop);

  /* 7. a., b. */
  bool is_enumerable = (property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE) != 0;
  if (!is_current_configurable
      && ((property_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE)
          || ((property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE_DEFINED)
              && (is_enumerable != ecma_is_property_enumerable (current_prop)))))
  {
    if (ECMA_PROPERTY_IS_VIRTUAL (current_prop))
    {
      ecma_free_value (ext_property_ref.property_ref.virtual_value);
    }

    return ecma_raise_property_redefinition (property_name_p, property_desc_p->flags);
  }

  if (ECMA_PROPERTY_IS_VIRTUAL (current_prop))
  {
    bool writable_check_failed = (property_desc_p->flags & JERRY_PROP_IS_WRITABLE);

#if JERRY_MODULE_SYSTEM
    if (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_MODULE_NAMESPACE))
    {
      if (JERRY_UNLIKELY (ext_property_ref.property_ref.virtual_value == ECMA_VALUE_UNINITIALIZED))
      {
        return ecma_raise_reference_error (ECMA_ERR_LET_CONST_NOT_INITIALIZED);
      }

      if (property_desc_p->flags & JERRY_PROP_IS_WRITABLE_DEFINED)
      {
        writable_check_failed = ((property_desc_p->flags ^ current_prop) & JERRY_PROP_IS_WRITABLE) != 0;
      }
    }
    else
    {
      JERRY_ASSERT (!is_current_configurable && !ecma_is_property_writable (current_prop));
    }
#else /* !JERRY_MODULE_SYSTEM */
    JERRY_ASSERT (!is_current_configurable && !ecma_is_property_writable (current_prop));
#endif /* JERRY_MODULE_SYSTEM */

    ecma_value_t result = ECMA_VALUE_TRUE;

    if (property_desc_type == ECMA_OP_OBJECT_DEFINE_ACCESSOR || writable_check_failed
        || ((property_desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
            && !ecma_op_same_value (property_desc_p->value, ext_property_ref.property_ref.virtual_value)))
    {
      result = ecma_raise_property_redefinition (property_name_p, property_desc_p->flags);
    }

    ecma_free_value (ext_property_ref.property_ref.virtual_value);
    return result;
  }

  /* 8. */
  if (property_desc_type == ECMA_OP_OBJECT_DEFINE_GENERIC)
  {
    /* No action required. */
  }
  else if (JERRY_LIKELY (property_desc_type == (current_prop & ECMA_PROPERTY_FLAG_DATA)))
  {
    /* If property is configurable, there is no need for checks. */
    if (JERRY_UNLIKELY (!is_current_configurable))
    {
      if (property_desc_type == ECMA_OP_OBJECT_DEFINE_DATA)
      {
        /* 10. a. i. & ii. */
        if (!ecma_is_property_writable (current_prop)
            && ((property_desc_p->flags & JERRY_PROP_IS_WRITABLE)
                || ((property_desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
                    && !ecma_op_same_value (property_desc_p->value, ext_property_ref.property_ref.value_p->value))))
        {
          return ecma_raise_property_redefinition (property_name_p, property_desc_p->flags);
        }
      }
      else
      {
        /* 11. */

        /* a. */
        ecma_property_value_t *value_p = ext_property_ref.property_ref.value_p;

        ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (value_p);
        jmem_cpointer_t prop_desc_getter_cp, prop_desc_setter_cp;
        ECMA_SET_POINTER (prop_desc_getter_cp, property_desc_p->get_p);
        ECMA_SET_POINTER (prop_desc_setter_cp, property_desc_p->set_p);

        if (((property_desc_p->flags & JERRY_PROP_IS_GET_DEFINED) && prop_desc_getter_cp != get_set_pair_p->getter_cp)
            || ((property_desc_p->flags & JERRY_PROP_IS_SET_DEFINED)
                && prop_desc_setter_cp != get_set_pair_p->setter_cp))
        {
          /* i., ii. */
          return ecma_raise_property_redefinition (property_name_p, property_desc_p->flags);
        }
      }
    }
  }
  else
  {
    /* 9. */
    if (!is_current_configurable)
    {
      /* a. */
      return ecma_raise_property_redefinition (property_name_p, property_desc_p->flags);
    }

    ecma_property_value_t *value_p = ext_property_ref.property_ref.value_p;

    if (property_desc_type == ECMA_OP_OBJECT_DEFINE_ACCESSOR)
    {
      JERRY_ASSERT (current_prop & ECMA_PROPERTY_FLAG_DATA);
      ecma_free_value_if_not_object (value_p->value);

      value_p->getter_setter_pair.getter_cp = JMEM_CP_NULL;
      value_p->getter_setter_pair.setter_cp = JMEM_CP_NULL;
    }
    else
    {
      JERRY_ASSERT (!(current_prop & ECMA_PROPERTY_FLAG_DATA));
      value_p->value = ECMA_VALUE_UNDEFINED;
    }

    /* Update flags */
    ecma_property_t prop_flags = *(ext_property_ref.property_p);
    prop_flags = (ecma_property_t) (prop_flags & ~ECMA_PROPERTY_FLAG_WRITABLE);
    prop_flags ^= ECMA_PROPERTY_FLAG_DATA;
    *(ext_property_ref.property_p) = prop_flags;
  }

  /* 12. */
  if (property_desc_type == ECMA_OP_OBJECT_DEFINE_DATA)
  {
    JERRY_ASSERT (ECMA_PROPERTY_IS_RAW_DATA (*ext_property_ref.property_p));

    if (property_desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
    {
      ecma_named_data_property_assign_value (object_p, ext_property_ref.property_ref.value_p, property_desc_p->value);
    }

    if (property_desc_p->flags & JERRY_PROP_IS_WRITABLE_DEFINED)
    {
      ecma_set_property_writable_attr (ext_property_ref.property_p, (property_desc_p->flags & JERRY_PROP_IS_WRITABLE));
    }
  }
  else if (property_desc_type == ECMA_OP_OBJECT_DEFINE_ACCESSOR)
  {
    JERRY_ASSERT (!(*ext_property_ref.property_p & ECMA_PROPERTY_FLAG_DATA));

    if (property_desc_p->flags & JERRY_PROP_IS_GET_DEFINED)
    {
      ecma_set_named_accessor_property_getter (object_p, ext_property_ref.property_ref.value_p, property_desc_p->get_p);
    }

    if (property_desc_p->flags & JERRY_PROP_IS_SET_DEFINED)
    {
      ecma_set_named_accessor_property_setter (object_p, ext_property_ref.property_ref.value_p, property_desc_p->set_p);
    }
  }

  if (property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE_DEFINED)
  {
    ecma_set_property_enumerable_attr (ext_property_ref.property_p,
                                       (property_desc_p->flags & JERRY_PROP_IS_ENUMERABLE));
  }

  if (property_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE_DEFINED)
  {
    ecma_set_property_configurable_attr (ext_property_ref.property_p,
                                         (property_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE));
  }

  return ECMA_VALUE_TRUE;
} /* ecma_op_general_object_define_own_property */

/**
 * The IsCompatiblePropertyDescriptor method for Proxy object internal methods
 *
 * See also:
 *          ECMAScript v6, 9.1.6.2
 *
 * @return bool
 */
bool
ecma_op_is_compatible_property_descriptor (const ecma_property_descriptor_t *desc_p, /**< target descriptor */
                                           const ecma_property_descriptor_t *current_p, /**< current descriptor */
                                           bool is_extensible) /**< true - if target object is extensible
                                                                    false - otherwise */
{
  JERRY_ASSERT (desc_p != NULL);

  /* 2. */
  if (current_p == NULL)
  {
    return is_extensible;
  }

  /* 3. */
  if (desc_p->flags == 0)
  {
    return true;
  }

  /* 4. */
  if ((current_p->flags & desc_p->flags) == desc_p->flags)
  {
    if ((current_p->flags & JERRY_PROP_IS_VALUE_DEFINED) && ecma_op_same_value (current_p->value, desc_p->value))
    {
      return true;
    }

    if ((current_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED) && current_p->get_p == desc_p->get_p
         && current_p->set_p == desc_p->set_p))
    {
      return true;
    }
  }

  /* 5. */
  if (!(current_p->flags & JERRY_PROP_IS_CONFIGURABLE))
  {
    if (desc_p->flags & JERRY_PROP_IS_CONFIGURABLE)
    {
      return false;
    }
    if ((desc_p->flags & JERRY_PROP_IS_ENUMERABLE_DEFINED)
        && ((current_p->flags & JERRY_PROP_IS_ENUMERABLE) != (desc_p->flags & JERRY_PROP_IS_ENUMERABLE)))
    {
      return false;
    }
  }

  const uint32_t accessor_desc_flags = (JERRY_PROP_IS_SET_DEFINED | JERRY_PROP_IS_GET_DEFINED);
  const uint32_t data_desc_flags = (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE_DEFINED);

  bool desc_is_accessor = (desc_p->flags & accessor_desc_flags) != 0;
  bool desc_is_data = (desc_p->flags & data_desc_flags) != 0;
  bool current_is_data = (current_p->flags & data_desc_flags) != 0;

  /* 6. */
  if (!desc_is_accessor && !desc_is_data)
  {
    return true;
  }

  /* 7. */
  if (current_is_data != desc_is_data)
  {
    return (current_p->flags & JERRY_PROP_IS_CONFIGURABLE) != 0;
  }

  /* 8. */
  if (current_is_data)
  {
    if (!(current_p->flags & JERRY_PROP_IS_CONFIGURABLE))
    {
      if (!(current_p->flags & JERRY_PROP_IS_WRITABLE) && (desc_p->flags & JERRY_PROP_IS_WRITABLE))
      {
        return false;
      }

      if (!(current_p->flags & JERRY_PROP_IS_WRITABLE) && (desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
          && !ecma_op_same_value (desc_p->value, current_p->value))
      {
        return false;
      }
    }

    return true;
  }

  JERRY_ASSERT ((current_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED)) != 0);
  JERRY_ASSERT ((desc_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED)) != 0);

  /* 9. */
  if (!(current_p->flags & JERRY_PROP_IS_CONFIGURABLE))
  {
    if ((desc_p->flags & JERRY_PROP_IS_SET_DEFINED) && desc_p->set_p != current_p->set_p)
    {
      return false;
    }

    if ((desc_p->flags & JERRY_PROP_IS_GET_DEFINED) && desc_p->get_p != current_p->get_p)
    {
      return false;
    }
  }

  return true;
} /* ecma_op_is_compatible_property_descriptor */

/**
 * CompletePropertyDescriptor method for proxy internal method
 *
 * See also:
 *          ECMA-262 v6, 6.2.4.5
 */
void
ecma_op_to_complete_property_descriptor (ecma_property_descriptor_t *desc_p) /**< target descriptor */
{
  /* 4. */
  if (!(desc_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED)))
  {
    /* a. */
    desc_p->flags |= JERRY_PROP_IS_VALUE_DEFINED;
  }
  /* 5. */
  else
  {
    desc_p->flags |= (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED);
  }
} /* ecma_op_to_complete_property_descriptor */

/**
 * @}
 * @}
 */
