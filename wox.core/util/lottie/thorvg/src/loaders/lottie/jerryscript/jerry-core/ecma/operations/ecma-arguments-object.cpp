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

#include "ecma-arguments-object.h"

#include "ecma-alloc.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-lex-env.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#include "jrt.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaargumentsobject ECMA arguments object related routines
 * @{
 */

/**
 * Arguments object creation operation.
 *
 * See also: ECMA-262 v5, 10.6
 *
 * @return ecma value of arguments object
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_create_arguments_object (vm_frame_ctx_shared_args_t *shared_p, /**< shared context data */
                                 ecma_object_t *lex_env_p) /**< lexical environment the Arguments
                                                            *   object is created for */
{
  ecma_object_t *func_obj_p = shared_p->header.function_object_p;
  const ecma_compiled_code_t *bytecode_data_p = shared_p->header.bytecode_header_p;
  uint16_t formal_params_number;

  if (bytecode_data_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    formal_params_number = ((cbc_uint16_arguments_t *) bytecode_data_p)->argument_end;
  }
  else
  {
    formal_params_number = ((cbc_uint8_arguments_t *) bytecode_data_p)->argument_end;
  }

  uint32_t object_size = sizeof (ecma_unmapped_arguments_t);
  uint32_t saved_arg_count = JERRY_MAX (shared_p->arg_list_len, formal_params_number);

  if (bytecode_data_p->status_flags & CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED)
  {
    object_size = sizeof (ecma_mapped_arguments_t);
  }

  ecma_object_t *obj_p = ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE),
                                             object_size + (saved_arg_count * sizeof (ecma_value_t)),
                                             ECMA_OBJECT_TYPE_CLASS);

  ecma_unmapped_arguments_t *arguments_p = (ecma_unmapped_arguments_t *) obj_p;

  arguments_p->header.u.cls.type = ECMA_OBJECT_CLASS_ARGUMENTS;
  arguments_p->header.u.cls.u1.arguments_flags = ECMA_ARGUMENTS_OBJECT_NO_FLAGS;
  arguments_p->header.u.cls.u2.formal_params_number = formal_params_number;
  arguments_p->header.u.cls.u3.arguments_number = 0;
  arguments_p->callee = ecma_make_object_value (func_obj_p);

  ecma_value_t *argv_p = (ecma_value_t *) (((uint8_t *) obj_p) + object_size);

  for (uint32_t i = 0; i < shared_p->arg_list_len; i++)
  {
    argv_p[i] = ecma_copy_value_if_not_object (shared_p->arg_list_p[i]);
  }

  for (uint32_t i = shared_p->arg_list_len; i < saved_arg_count; i++)
  {
    argv_p[i] = ECMA_VALUE_UNDEFINED;
  }

  arguments_p->header.u.cls.u3.arguments_number = shared_p->arg_list_len;

  if (bytecode_data_p->status_flags & CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED)
  {
    ecma_mapped_arguments_t *mapped_arguments_p = (ecma_mapped_arguments_t *) obj_p;

    ECMA_SET_INTERNAL_VALUE_POINTER (mapped_arguments_p->lex_env, lex_env_p);
    arguments_p->header.u.cls.u1.arguments_flags |= ECMA_ARGUMENTS_OBJECT_MAPPED;

#if JERRY_SNAPSHOT_EXEC
    if (bytecode_data_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION)
    {
      arguments_p->header.u.cls.u1.arguments_flags |= ECMA_ARGUMENTS_OBJECT_STATIC_BYTECODE;
      mapped_arguments_p->u.byte_code_p = (ecma_compiled_code_t *) bytecode_data_p;
    }
    else
#endif /* JERRY_SNAPSHOT_EXEC */
    {
      ECMA_SET_INTERNAL_VALUE_POINTER (mapped_arguments_p->u.byte_code, bytecode_data_p);
    }

    /* Static snapshots are not ref counted. */
    if ((bytecode_data_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION) == 0)
    {
      ecma_bytecode_ref ((ecma_compiled_code_t *) bytecode_data_p);
    }

    ecma_value_t *formal_parameter_start_p;
    formal_parameter_start_p = ecma_compiled_code_resolve_arguments_start ((ecma_compiled_code_t *) bytecode_data_p);

    for (uint32_t i = 0; i < formal_params_number; i++)
    {
      /* For legacy (non-strict) argument definition the trailing duplicated arguments cannot be lazy instantiated
         E.g: function f (a,a,a,a) {} */
      if (JERRY_UNLIKELY (ecma_is_value_empty (formal_parameter_start_p[i])))
      {
        ecma_property_value_t *prop_value_p;
        ecma_string_t *prop_name_p = ecma_new_ecma_string_from_uint32 (i);

        prop_value_p =
          ecma_create_named_data_property (obj_p, prop_name_p, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE, NULL);

        ecma_deref_ecma_string (prop_name_p);

        prop_value_p->value = argv_p[i];
        argv_p[i] = ECMA_VALUE_EMPTY;
      }
    }
  }

  return ecma_make_object_value (obj_p);
} /* ecma_op_create_arguments_object */

/**
 * [[DefineOwnProperty]] ecma Arguments object's operation
 *
 * See also:
 *          ECMA-262 v5, 8.6.2; ECMA-262 v5, Table 8
 *          ECMA-262 v5, 10.6
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_arguments_object_define_own_property (ecma_object_t *object_p, /**< the object */
                                              ecma_string_t *property_name_p, /**< property name */
                                              const ecma_property_descriptor_t *property_desc_p) /**< property
                                                                                                  *   descriptor */
{
  /* 3. */
  ecma_value_t ret_value = ecma_op_general_object_define_own_property (object_p, property_name_p, property_desc_p);

  if (ECMA_IS_VALUE_ERROR (ret_value)
      || !(((ecma_extended_object_t *) object_p)->u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED))
  {
    return ret_value;
  }

  ecma_mapped_arguments_t *mapped_arguments_p = (ecma_mapped_arguments_t *) object_p;
  uint32_t index = ecma_string_get_array_index (property_name_p);

  if (index >= mapped_arguments_p->unmapped.header.u.cls.u2.formal_params_number)
  {
    return ret_value;
  }

  ecma_value_t *argv_p = (ecma_value_t *) (mapped_arguments_p + 1);

  if (ecma_is_value_empty (argv_p[index]) || argv_p[index] == ECMA_VALUE_ARGUMENT_NO_TRACK)
  {
    return ret_value;
  }

  if (property_desc_p->flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED))
  {
    ecma_free_value_if_not_object (argv_p[index]);
    argv_p[index] = ECMA_VALUE_ARGUMENT_NO_TRACK;
    return ret_value;
  }

  if (property_desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
  {
    ecma_string_t *name_p = ecma_op_arguments_object_get_formal_parameter (mapped_arguments_p, index);
    ecma_object_t *lex_env_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_object_t, mapped_arguments_p->lex_env);
    ecma_op_set_mutable_binding (lex_env_p, name_p, property_desc_p->value, true);
  }

  if ((property_desc_p->flags & JERRY_PROP_IS_WRITABLE_DEFINED) && !(property_desc_p->flags & JERRY_PROP_IS_WRITABLE))
  {
    ecma_free_value_if_not_object (argv_p[index]);
    argv_p[index] = ECMA_VALUE_ARGUMENT_NO_TRACK;
  }

  return ret_value;
} /* ecma_op_arguments_object_define_own_property */

/**
 * Try to lazy instantiate the given property of a mapped/unmapped arguments object
 *
 * @return pointer property, if one was instantiated,
 *         NULL - otherwise.
 */
ecma_property_t *
ecma_op_arguments_object_try_to_lazy_instantiate_property (ecma_object_t *object_p, /**< object */
                                                           ecma_string_t *property_name_p) /**< property's name */
{
  JERRY_ASSERT (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_ARGUMENTS));

  ecma_unmapped_arguments_t *arguments_p = (ecma_unmapped_arguments_t *) object_p;
  ecma_value_t *argv_p = (ecma_value_t *) (arguments_p + 1);
  ecma_property_value_t *prop_value_p;
  ecma_property_t *prop_p;
  uint32_t arguments_number = arguments_p->header.u.cls.u3.arguments_number;
  uint8_t flags = arguments_p->header.u.cls.u1.arguments_flags;

  if (flags & ECMA_ARGUMENTS_OBJECT_MAPPED)
  {
    argv_p = (ecma_value_t *) (((ecma_mapped_arguments_t *) object_p) + 1);
  }

  uint32_t index = ecma_string_get_array_index (property_name_p);

  if (index != ECMA_STRING_NOT_ARRAY_INDEX)
  {
    if (index >= arguments_number || ecma_is_value_empty (argv_p[index]))
    {
      return NULL;
    }

    JERRY_ASSERT (argv_p[index] != ECMA_VALUE_ARGUMENT_NO_TRACK);

    prop_value_p = ecma_create_named_data_property (object_p,
                                                    property_name_p,
                                                    ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                    &prop_p);

    /* Passing the reference */
    prop_value_p->value = argv_p[index];

    argv_p[index] = ECMA_VALUE_UNDEFINED;
    return prop_p;
  }

  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_LENGTH)
      && !(flags & ECMA_ARGUMENTS_OBJECT_LENGTH_INITIALIZED))
  {
    prop_value_p = ecma_create_named_data_property (object_p,
                                                    ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH),
                                                    ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_WRITABLE,
                                                    &prop_p);

    prop_value_p->value = ecma_make_uint32_value (arguments_number);
    return prop_p;
  }

  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_CALLEE)
      && !(flags & ECMA_ARGUMENTS_OBJECT_CALLEE_INITIALIZED))
  {
    if (flags & ECMA_ARGUMENTS_OBJECT_MAPPED)
    {
      prop_value_p = ecma_create_named_data_property (object_p,
                                                      property_name_p,
                                                      ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_WRITABLE,
                                                      &prop_p);

      prop_value_p->value = arguments_p->callee;
    }
    else
    {
      ecma_object_t *thrower_p = ecma_builtin_get (ECMA_BUILTIN_ID_TYPE_ERROR_THROWER);

      ecma_create_named_accessor_property (object_p,
                                           ecma_get_magic_string (LIT_MAGIC_STRING_CALLEE),
                                           thrower_p,
                                           thrower_p,
                                           ECMA_PROPERTY_BUILT_IN_FIXED,
                                           &prop_p);
    }
    return prop_p;
  }

  if (ecma_op_compare_string_to_global_symbol (property_name_p, LIT_GLOBAL_SYMBOL_ITERATOR)
      && !(flags & ECMA_ARGUMENTS_OBJECT_ITERATOR_INITIALIZED))
  {
    prop_value_p = ecma_create_named_data_property (object_p,
                                                    property_name_p,
                                                    ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_WRITABLE,
                                                    &prop_p);

    prop_value_p->value = ecma_op_object_get_by_magic_id (ecma_builtin_get (ECMA_BUILTIN_ID_INTRINSIC_OBJECT),
                                                          LIT_INTERNAL_MAGIC_STRING_ARRAY_PROTOTYPE_VALUES);

    JERRY_ASSERT (ecma_is_value_object (prop_value_p->value));
    ecma_deref_object (ecma_get_object_from_value (prop_value_p->value));
    return prop_p;
  }

  return NULL;
} /* ecma_op_arguments_object_try_to_lazy_instantiate_property */

/**
 * Delete configurable properties of arguments object
 */
void
ecma_op_arguments_delete_built_in_property (ecma_object_t *object_p, /**< the object */
                                            ecma_string_t *property_name_p) /**< property name */
{
  ecma_unmapped_arguments_t *arguments_p = (ecma_unmapped_arguments_t *) object_p;

  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_LENGTH))
  {
    JERRY_ASSERT (!(arguments_p->header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_LENGTH_INITIALIZED));

    arguments_p->header.u.cls.u1.arguments_flags |= ECMA_ARGUMENTS_OBJECT_LENGTH_INITIALIZED;
    return;
  }

  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_CALLEE))
  {
    JERRY_ASSERT (!(arguments_p->header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_CALLEE_INITIALIZED));
    JERRY_ASSERT (arguments_p->header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED);

    arguments_p->header.u.cls.u1.arguments_flags |= ECMA_ARGUMENTS_OBJECT_CALLEE_INITIALIZED;
    return;
  }

  if (ecma_prop_name_is_symbol (property_name_p))
  {
    JERRY_ASSERT (!(arguments_p->header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_ITERATOR_INITIALIZED));
    JERRY_ASSERT (ecma_op_compare_string_to_global_symbol (property_name_p, LIT_GLOBAL_SYMBOL_ITERATOR));

    arguments_p->header.u.cls.u1.arguments_flags |= ECMA_ARGUMENTS_OBJECT_ITERATOR_INITIALIZED;
    return;
  }

  uint32_t index = ecma_string_get_array_index (property_name_p);

  ecma_value_t *argv_p = (ecma_value_t *) (arguments_p + 1);

  if (arguments_p->header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED)
  {
    argv_p = (ecma_value_t *) (((ecma_mapped_arguments_t *) object_p) + 1);
  }

  JERRY_ASSERT (argv_p[index] == ECMA_VALUE_UNDEFINED || argv_p[index] == ECMA_VALUE_ARGUMENT_NO_TRACK);

  argv_p[index] = ECMA_VALUE_EMPTY;
} /* ecma_op_arguments_delete_built_in_property */

/**
 * List names of an arguments object's lazy instantiated properties
 */
void
ecma_op_arguments_object_list_lazy_property_names (ecma_object_t *obj_p, /**< arguments object */
                                                   ecma_collection_t *prop_names_p, /**< prop name collection */
                                                   ecma_property_counter_t *prop_counter_p, /**< property counters */
                                                   jerry_property_filter_t filter) /**< property name filter options */
{
  JERRY_ASSERT (ecma_object_class_is (obj_p, ECMA_OBJECT_CLASS_ARGUMENTS));

  ecma_unmapped_arguments_t *arguments_p = (ecma_unmapped_arguments_t *) obj_p;

  uint32_t arguments_number = arguments_p->header.u.cls.u3.arguments_number;
  uint8_t flags = arguments_p->header.u.cls.u1.arguments_flags;

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_INTEGER_INDICES))
  {
    ecma_value_t *argv_p = (ecma_value_t *) (arguments_p + 1);

    if (flags & ECMA_ARGUMENTS_OBJECT_MAPPED)
    {
      argv_p = (ecma_value_t *) (((ecma_mapped_arguments_t *) obj_p) + 1);
    }

    for (uint32_t index = 0; index < arguments_number; index++)
    {
      if (!ecma_is_value_empty (argv_p[index]))
      {
        ecma_string_t *index_string_p = ecma_new_ecma_string_from_uint32 (index);
        ecma_collection_push_back (prop_names_p, ecma_make_string_value (index_string_p));
        prop_counter_p->array_index_named_props++;
      }
    }
  }

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS))
  {
    if (!(flags & ECMA_ARGUMENTS_OBJECT_LENGTH_INITIALIZED))
    {
      ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_LENGTH));
      prop_counter_p->string_named_props++;
    }

    if (!(flags & ECMA_ARGUMENTS_OBJECT_CALLEE_INITIALIZED))
    {
      ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_CALLEE));
      prop_counter_p->string_named_props++;
    }
  }

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_SYMBOLS) && !(flags & ECMA_ARGUMENTS_OBJECT_ITERATOR_INITIALIZED))
  {
    ecma_string_t *symbol_p = ecma_op_get_global_symbol (LIT_GLOBAL_SYMBOL_ITERATOR);
    ecma_collection_push_back (prop_names_p, ecma_make_symbol_value (symbol_p));
    prop_counter_p->symbol_named_props++;
  }
} /* ecma_op_arguments_object_list_lazy_property_names */

/**
 * Get the formal parameter name corresponding to the given property index
 *
 * @return pointer to the formal parameter name
 */
ecma_string_t *
ecma_op_arguments_object_get_formal_parameter (ecma_mapped_arguments_t *mapped_arguments_p, /**< mapped arguments
                                                                                             *   object */
                                               uint32_t index) /**< formal parameter index */
{
  JERRY_ASSERT (mapped_arguments_p->unmapped.header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_MAPPED);
  JERRY_ASSERT (index < mapped_arguments_p->unmapped.header.u.cls.u2.formal_params_number);

  ecma_compiled_code_t *byte_code_p;

#if JERRY_SNAPSHOT_EXEC
  if (mapped_arguments_p->unmapped.header.u.cls.u1.arguments_flags & ECMA_ARGUMENTS_OBJECT_STATIC_BYTECODE)
  {
    byte_code_p = mapped_arguments_p->u.byte_code_p;
  }
  else
#endif /* JERRY_SNAPSHOT_EXEC */
  {
    byte_code_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, mapped_arguments_p->u.byte_code);
  }

  ecma_value_t *formal_param_names_p = ecma_compiled_code_resolve_arguments_start (byte_code_p);

  return ecma_get_string_from_value (formal_param_names_p[index]);
} /* ecma_op_arguments_object_get_formal_parameter */

/**
 * @}
 * @}
 */
