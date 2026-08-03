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

#include "ecma-builtin-object.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"
#include "ecma-proxy-object.h"

#include "jcontext.h"
#include "jrt.h"

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

/**
 * This object has a custom dispatch function.
 */
#define BUILTIN_CUSTOM_DISPATCH

/**
 * List of built-in routine identifiers.
 */
enum
{
  ECMA_OBJECT_ROUTINE_START = 0,

  ECMA_OBJECT_ROUTINE_CREATE,
  ECMA_OBJECT_ROUTINE_IS,
  ECMA_OBJECT_ROUTINE_SET_PROTOTYPE_OF,

  /* These should be in this order. */
  ECMA_OBJECT_ROUTINE_DEFINE_PROPERTY,
  ECMA_OBJECT_ROUTINE_DEFINE_PROPERTIES,

  /* These should be in this order. */
  ECMA_OBJECT_ROUTINE_ASSIGN,
  ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_DESCRIPTOR,
  ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_DESCRIPTORS,
  ECMA_OBJECT_ROUTINE_HAS_OWN,
  ECMA_OBJECT_ROUTINE_GET_PROTOTYPE_OF,
  ECMA_OBJECT_ROUTINE_FROM_ENTRIES,
  ECMA_OBJECT_ROUTINE_KEYS,
  ECMA_OBJECT_ROUTINE_VALUES,
  ECMA_OBJECT_ROUTINE_ENTRIES,

  /* These should be in this order. */
  ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_NAMES,
  ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_SYMBOLS,

  /* These should be in this order. */
  ECMA_OBJECT_ROUTINE_FREEZE,
  ECMA_OBJECT_ROUTINE_PREVENT_EXTENSIONS,
  ECMA_OBJECT_ROUTINE_SEAL,

  /* These should be in this order. */
  ECMA_OBJECT_ROUTINE_IS_EXTENSIBLE,
  ECMA_OBJECT_ROUTINE_IS_FROZEN,
  ECMA_OBJECT_ROUTINE_IS_SEALED,
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-object.inc.h"
#define BUILTIN_UNDERSCORED_ID  object
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup object ECMA Object object built-in
 * @{
 */

/**
 * Handle calling [[Call]] of built-in Object object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_object_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                   uint32_t arguments_list_len) /**< number of arguments */
{
  if (arguments_list_len == 0 || ecma_is_value_undefined (arguments_list_p[0])
      || ecma_is_value_null (arguments_list_p[0]))
  {
    return ecma_make_object_value (ecma_op_create_object_object_noarg ());
  }

  return ecma_op_to_object (arguments_list_p[0]);
} /* ecma_builtin_object_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Object object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_object_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                        uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (JERRY_CONTEXT (current_new_target_p) != ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT))
  {
    ecma_object_t *prototype_obj_p =
      ecma_op_get_prototype_from_constructor (JERRY_CONTEXT (current_new_target_p), ECMA_BUILTIN_ID_OBJECT_PROTOTYPE);
    if (JERRY_UNLIKELY (prototype_obj_p == NULL))
    {
      return ECMA_VALUE_ERROR;
    }

    ecma_object_t *object_p = ecma_create_object (prototype_obj_p, 0, ECMA_OBJECT_TYPE_GENERAL);
    ecma_deref_object (prototype_obj_p);

    return ecma_make_object_value (object_p);
  }

  return ecma_builtin_object_dispatch_call (arguments_list_p, arguments_list_len);
} /* ecma_builtin_object_dispatch_construct */

/**
 * The Object object's 'getPrototypeOf' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_object_get_prototype_of (ecma_object_t *obj_p) /**< routine's argument */
{
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    return ecma_proxy_object_get_prototype_of (obj_p);
  }
#endif /* JERRY_BUILTIN_PROXY */

  jmem_cpointer_t proto_cp = ecma_op_ordinary_object_get_prototype_of (obj_p);

  if (proto_cp != JMEM_CP_NULL)
  {
    ecma_object_t *prototype_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, proto_cp);
    ecma_ref_object (prototype_p);
    return ecma_make_object_value (prototype_p);
  }

  return ECMA_VALUE_NULL;
} /* ecma_builtin_object_object_get_prototype_of */

/**
 * The Object object's 'setPrototypeOf' routine
 *
 * See also:
 *          ES2015 19.1.2.18
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_object_set_prototype_of (ecma_value_t arg1, /**< routine's first argument */
                                             ecma_value_t arg2) /**< routine's second argument */
{
  /* 1., 2. */
  if (!ecma_op_require_object_coercible (arg1))
  {
    return ECMA_VALUE_ERROR;
  }

  /* 3. */
  if (!ecma_is_value_object (arg2) && !ecma_is_value_null (arg2))
  {
    return ecma_raise_type_error (ECMA_ERR_PROTOTYPE_IS_NEITHER_OBJECT_NOR_NULL);
  }

  /* 4. */
  if (!ecma_is_value_object (arg1))
  {
    return ecma_copy_value (arg1);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (arg1);
  ecma_value_t status;

  /* 5. */
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    status = ecma_proxy_object_set_prototype_of (obj_p, arg2);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      return status;
    }
  }
  else
  {
#endif /* JERRY_BUILTIN_PROXY */
    status = ecma_op_ordinary_object_set_prototype_of (obj_p, arg2);
#if JERRY_BUILTIN_PROXY
  }
#endif /* JERRY_BUILTIN_PROXY */

  if (ecma_is_value_false (status))
  {
    return ecma_raise_type_error (ECMA_ERR_SET_PROTOTYPE);
  }

  JERRY_ASSERT (ecma_is_value_true (status));
  ecma_ref_object (obj_p);

  return arg1;
} /* ecma_builtin_object_object_set_prototype_of */

/**
 * The Object object's set __proto__ routine
 *
 * See also:
 *          ECMA-262 v6, B.2.2.1.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_object_set_proto (ecma_value_t arg1, /**< routine's first argument */
                                      ecma_value_t arg2) /**< routine's second argument */
{
  /* 1., 2. */
  if (!ecma_op_require_object_coercible (arg1))
  {
    return ECMA_VALUE_ERROR;
  }

  /* 3. */
  if (!ecma_is_value_object (arg2) && !ecma_is_value_null (arg2))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  /* 4. */
  if (!ecma_is_value_object (arg1))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (arg1);
  ecma_value_t status;

  /* 5. */
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    status = ecma_proxy_object_set_prototype_of (obj_p, arg2);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      return status;
    }
  }
  else
  {
#endif /* JERRY_BUILTIN_PROXY */
    status = ecma_op_ordinary_object_set_prototype_of (obj_p, arg2);
#if JERRY_BUILTIN_PROXY
  }
#endif /* JERRY_BUILTIN_PROXY */

  if (ecma_is_value_false (status))
  {
    return ecma_raise_type_error (ECMA_ERR_SET_PROTOTYPE);
  }

  JERRY_ASSERT (ecma_is_value_true (status));

  return ECMA_VALUE_UNDEFINED;
} /* ecma_builtin_object_object_set_proto */

/**
 * SetIntegrityLevel operation
 *
 * See also:
 *          ECMA-262 v6, 7.3.14
 *
 * @return ECMA_VALUE_ERROR - if the operation raised an error
 *         ECMA_VALUE_{TRUE/FALSE} - depends on whether the integrity level has been set successfully
 */
static ecma_value_t
ecma_builtin_object_set_integrity_level (ecma_object_t *obj_p, /**< object */
                                         bool is_seal) /**< true - set "sealed"
                                                        *   false - set "frozen" */
{
  /* 3. */
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    ecma_value_t status = ecma_proxy_object_prevent_extensions (obj_p);

    if (!ecma_is_value_true (status))
    {
      return status;
    }
  }
  else
#endif /* JERRY_BUILTIN_PROXY */
  {
    ecma_op_ordinary_object_prevent_extensions (obj_p);
  }

  /* 6. */
  ecma_collection_t *props_p = ecma_op_object_own_property_keys (obj_p, JERRY_PROPERTY_FILTER_ALL);

#if JERRY_BUILTIN_PROXY
  if (props_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_value_t *buffer_p = props_p->buffer_p;

  if (is_seal)
  {
    /* 8.a */
    for (uint32_t i = 0; i < props_p->item_count; i++)
    {
      ecma_string_t *property_name_p = ecma_get_prop_name_from_value (buffer_p[i]);

      ecma_property_descriptor_t prop_desc;
      ecma_value_t status = ecma_op_object_get_own_property_descriptor (obj_p, property_name_p, &prop_desc);

#if JERRY_BUILTIN_PROXY
      if (ECMA_IS_VALUE_ERROR (status))
      {
        ecma_collection_free (props_p);
        return ECMA_VALUE_ERROR;
      }
#endif /* JERRY_BUILTIN_PROXY */

      if (ecma_is_value_false (status))
      {
        continue;
      }

      prop_desc.flags &= (uint16_t) ~JERRY_PROP_IS_CONFIGURABLE;
      prop_desc.flags |= JERRY_PROP_SHOULD_THROW;

      /* 8.a.i */
      ecma_value_t define_own_prop_ret = ecma_op_object_define_own_property (obj_p, property_name_p, &prop_desc);

      ecma_free_property_descriptor (&prop_desc);

      /* 8.a.ii */
      if (ECMA_IS_VALUE_ERROR (define_own_prop_ret))
      {
        ecma_collection_free (props_p);
        return define_own_prop_ret;
      }

      ecma_free_value (define_own_prop_ret);
    }
  }
  else
  {
    /* 9.a */
    for (uint32_t i = 0; i < props_p->item_count; i++)
    {
      ecma_string_t *property_name_p = ecma_get_prop_name_from_value (buffer_p[i]);

      /* 9.1 */
      ecma_property_descriptor_t prop_desc;
      ecma_value_t status = ecma_op_object_get_own_property_descriptor (obj_p, property_name_p, &prop_desc);

#if JERRY_BUILTIN_PROXY
      if (ECMA_IS_VALUE_ERROR (status))
      {
        ecma_collection_free (props_p);
        return ECMA_VALUE_ERROR;
      }
#endif /* JERRY_BUILTIN_PROXY */

      if (ecma_is_value_false (status))
      {
        continue;
      }

      /* 9.2 */
      if ((prop_desc.flags & (JERRY_PROP_IS_WRITABLE_DEFINED | JERRY_PROP_IS_WRITABLE))
          == (JERRY_PROP_IS_WRITABLE_DEFINED | JERRY_PROP_IS_WRITABLE))
      {
        prop_desc.flags &= (uint16_t) ~JERRY_PROP_IS_WRITABLE;
      }

      prop_desc.flags &= (uint16_t) ~JERRY_PROP_IS_CONFIGURABLE;
      prop_desc.flags |= JERRY_PROP_SHOULD_THROW;

      /* 9.3 */
      ecma_value_t define_own_prop_ret = ecma_op_object_define_own_property (obj_p, property_name_p, &prop_desc);

      ecma_free_property_descriptor (&prop_desc);

      /* 9.4 */
      if (ECMA_IS_VALUE_ERROR (define_own_prop_ret))
      {
        ecma_collection_free (props_p);
        return define_own_prop_ret;
      }

      ecma_free_value (define_own_prop_ret);
    }
  }

  ecma_collection_free (props_p);

  return ECMA_VALUE_TRUE;
} /* ecma_builtin_object_set_integrity_level */

/**
 * The Object object's 'seal' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.8
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_seal (ecma_object_t *obj_p) /**< routine's argument */
{
  ecma_value_t status = ecma_builtin_object_set_integrity_level (obj_p, true);

  if (ECMA_IS_VALUE_ERROR (status))
  {
    return status;
  }

#if JERRY_BUILTIN_PROXY
  if (ecma_is_value_false (status))
  {
    return ecma_raise_type_error (ECMA_ERR_OBJECT_CANNOT_BE_SEALED);
  }
#endif /* JERRY_BUILTIN_PROXY */

  /* 4. */
  ecma_ref_object (obj_p);
  return ecma_make_object_value (obj_p);
} /* ecma_builtin_object_object_seal */

/**
 * The Object object's 'freeze' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.9
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_freeze (ecma_object_t *obj_p) /**< routine's argument */
{
  ecma_value_t status = ecma_builtin_object_set_integrity_level (obj_p, false);

  if (ECMA_IS_VALUE_ERROR (status))
  {
    return status;
  }

#if JERRY_BUILTIN_PROXY
  if (ecma_is_value_false (status))
  {
    return ecma_raise_type_error (ECMA_ERR_OBJECT_CANNOT_BE_FROZEN);
  }
#endif /* JERRY_BUILTIN_PROXY */

  /* 4. */
  ecma_ref_object (obj_p);
  return ecma_make_object_value (obj_p);
} /* ecma_builtin_object_object_freeze */

/**
 * The Object object's 'preventExtensions' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.10
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_object_prevent_extensions (ecma_object_t *obj_p) /**< routine's argument */
{
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    ecma_value_t status = ecma_proxy_object_prevent_extensions (obj_p);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      return status;
    }

    if (ecma_is_value_false (status))
    {
      return ecma_raise_type_error (ECMA_ERR_SET_EXTENSIBLE_PROPERTY);
    }

    JERRY_ASSERT (ecma_is_value_true (status));
  }
  else
  {
#endif /* JERRY_BUILTIN_PROXY */
    ecma_op_ordinary_object_prevent_extensions (obj_p);
#if JERRY_BUILTIN_PROXY
  }
#endif /* JERRY_BUILTIN_PROXY */
  ecma_ref_object (obj_p);

  return ecma_make_object_value (obj_p);
} /* ecma_builtin_object_object_prevent_extensions */

/**
 * The Object object's 'isSealed' and 'isFrozen' routines
 *
 * See also:
 *         ECMA-262 v5, 15.2.3.11
 *         ECMA-262 v5, 15.2.3.12
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_test_integrity_level (ecma_object_t *obj_p, /**< routine's argument */
                                          int mode) /**< routine mode */
{
  JERRY_ASSERT (mode == ECMA_OBJECT_ROUTINE_IS_FROZEN || mode == ECMA_OBJECT_ROUTINE_IS_SEALED);

  /* 3. */
  bool is_extensible;
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    ecma_value_t status = ecma_proxy_object_is_extensible (obj_p);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      return status;
    }

    is_extensible = ecma_is_value_true (status);
  }
  else
#endif /* JERRY_BUILTIN_PROXY */
  {
    is_extensible = ecma_op_ordinary_object_is_extensible (obj_p);
  }

  if (is_extensible)
  {
    return ECMA_VALUE_FALSE;
  }

  /* the value can be updated in the loop below */
  ecma_value_t ret_value = ECMA_VALUE_TRUE;

  /* 2. */
  ecma_collection_t *props_p = ecma_op_object_own_property_keys (obj_p, JERRY_PROPERTY_FILTER_ALL);

#if JERRY_BUILTIN_PROXY
  if (props_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_value_t *buffer_p = props_p->buffer_p;

  for (uint32_t i = 0; i < props_p->item_count; i++)
  {
    ecma_string_t *property_name_p = ecma_get_prop_name_from_value (buffer_p[i]);

    /* 2.a */
    ecma_property_descriptor_t prop_desc;
    ecma_value_t status = ecma_op_object_get_own_property_descriptor (obj_p, property_name_p, &prop_desc);

#if JERRY_BUILTIN_PROXY
    if (ECMA_IS_VALUE_ERROR (status))
    {
      ret_value = status;
      break;
    }
#endif /* JERRY_BUILTIN_PROXY */

    if (ecma_is_value_false (status))
    {
      continue;
    }

    bool is_writable_data = ((prop_desc.flags & (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE))
                             == (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE));
    bool is_configurable = (prop_desc.flags & JERRY_PROP_IS_CONFIGURABLE);

    ecma_free_property_descriptor (&prop_desc);

    /* 2.b for isFrozen */
    /* 2.b for isSealed, 2.c for isFrozen */
    if ((mode == ECMA_OBJECT_ROUTINE_IS_FROZEN && is_writable_data) || is_configurable)
    {
      ret_value = ECMA_VALUE_FALSE;
      break;
    }
  }

  ecma_collection_free (props_p);

  return ret_value;
} /* ecma_builtin_object_test_integrity_level */

/**
 * The Object object's 'isExtensible' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.13
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_object_is_extensible (ecma_object_t *obj_p) /**< routine's argument */
{
#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (obj_p))
  {
    return ecma_proxy_object_is_extensible (obj_p);
  }
#endif /* JERRY_BUILTIN_PROXY */

  return ecma_make_boolean_value (ecma_op_ordinary_object_is_extensible (obj_p));
} /* ecma_builtin_object_object_is_extensible */

/**
 * Common implementation of the Object object's 'keys', 'values', 'entries' routines
 *
 * See also:
 *          ECMA-262 v11, 19.1.2.17
 *          ECMA-262 v11, 19.1.2.22
 *          ECMA-262 v11, 19.1.2.5
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_keys_values_helper (ecma_object_t *obj_p, /**< routine's first argument */
                                               ecma_enumerable_property_names_options_t option) /**< listing option */
{
  /* 2. */
  ecma_collection_t *props_p = ecma_op_object_get_enumerable_property_names (obj_p, option);

  if (props_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  /* 3. */
  return ecma_op_new_array_object_from_collection (props_p, option != ECMA_ENUMERABLE_PROPERTY_KEYS);
} /* ecma_builtin_object_object_keys_values_helper */

/**
 * The Object object's 'getOwnPropertyDescriptor' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_object_get_own_property_descriptor (ecma_object_t *obj_p, /**< routine's first argument */
                                                        ecma_string_t *name_str_p) /**< routine's second argument */
{
  /* 3. */
  ecma_property_descriptor_t prop_desc;

  ecma_value_t status = ecma_op_object_get_own_property_descriptor (obj_p, name_str_p, &prop_desc);

#if JERRY_BUILTIN_PROXY
  if (ECMA_IS_VALUE_ERROR (status))
  {
    return status;
  }
#endif /* JERRY_BUILTIN_PROXY */

  if (ecma_is_value_true (status))
  {
    /* 4. */
    ecma_object_t *desc_obj_p = ecma_op_from_property_descriptor (&prop_desc);

    ecma_free_property_descriptor (&prop_desc);

    return ecma_make_object_value (desc_obj_p);
  }

  return ECMA_VALUE_UNDEFINED;
} /* ecma_builtin_object_object_get_own_property_descriptor */

/**
 * The Object object's 'getOwnPropertyDescriptors' routine
 *
 * See also:
 *          ECMA-262 v11, 19.1.2.9
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_get_own_property_descriptors (ecma_object_t *obj_p) /**< routine's first argument */
{
  /* 2 */
  ecma_collection_t *prop_names_p = ecma_op_object_own_property_keys (obj_p, JERRY_PROPERTY_FILTER_ALL);

#if JERRY_BUILTIN_PROXY
  if (prop_names_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_value_t *names_buffer_p = prop_names_p->buffer_p;

  /* 3 */
  ecma_object_t *object_prototype_p = ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE);
  ecma_object_t *descriptors_p = ecma_create_object (object_prototype_p, 0, ECMA_OBJECT_TYPE_GENERAL);

  /* 4 */
  for (uint32_t i = 0; i < prop_names_p->item_count; i++)
  {
    ecma_string_t *property_name_p = ecma_get_prop_name_from_value (names_buffer_p[i]);

    /* 4.a */
    ecma_property_descriptor_t prop_desc;
    ecma_value_t status = ecma_op_object_get_own_property_descriptor (obj_p, property_name_p, &prop_desc);

#if JERRY_BUILTIN_PROXY
    if (ECMA_IS_VALUE_ERROR (status))
    {
      ecma_deref_object (descriptors_p);
      ecma_collection_free (prop_names_p);

      return status;
    }
#endif /* JERRY_BUILTIN_PROXY */

    if (ecma_is_value_true (status))
    {
      /* 4.b */
      ecma_object_t *desc_obj_p = ecma_op_from_property_descriptor (&prop_desc);
      /* 4.c */
      ecma_property_value_t *value_p = ecma_create_named_data_property (descriptors_p,
                                                                        property_name_p,
                                                                        ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                                        NULL);
      value_p->value = ecma_make_object_value (desc_obj_p);

      ecma_deref_object (desc_obj_p);
      ecma_free_property_descriptor (&prop_desc);
    }
  }

  ecma_collection_free (prop_names_p);

  return ecma_make_object_value (descriptors_p);
} /* ecma_builtin_object_object_get_own_property_descriptors */

/**
 * The Object object's 'defineProperties' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.7
 *          ECMA-262 v11, 19.1.2.3.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_define_properties (ecma_object_t *obj_p, /**< routine's first argument */
                                              ecma_value_t arg2) /**< routine's second argument */
{
  /* 2. */
  ecma_value_t props = ecma_op_to_object (arg2);

  if (ECMA_IS_VALUE_ERROR (props))
  {
    return props;
  }

  ecma_object_t *props_p = ecma_get_object_from_value (props);

  /* 3. */
  ecma_collection_t *prop_names_p = ecma_op_object_own_property_keys (props_p, JERRY_PROPERTY_FILTER_ALL);
  ecma_value_t ret_value = ECMA_VALUE_ERROR;

#if JERRY_BUILTIN_PROXY
  if (prop_names_p == NULL)
  {
    ecma_deref_object (props_p);
    return ret_value;
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_value_t *buffer_p = prop_names_p->buffer_p;

  /* 4. */
  JMEM_DEFINE_LOCAL_ARRAY (property_descriptors, prop_names_p->item_count, ecma_property_descriptor_t);
  uint32_t property_descriptor_number = 0;
  ecma_collection_t *enum_prop_names = ecma_new_collection ();

  /* 5. */
  for (uint32_t i = 0; i < prop_names_p->item_count; i++)
  {
    ecma_string_t *prop_name_p = ecma_get_prop_name_from_value (buffer_p[i]);

    ecma_property_descriptor_t prop_desc;
    ecma_value_t get_desc = ecma_op_object_get_own_property_descriptor (props_p, prop_name_p, &prop_desc);

    if (ECMA_IS_VALUE_ERROR (get_desc))
    {
      goto cleanup;
    }

    if (ecma_is_value_true (get_desc))
    {
      if (prop_desc.flags & JERRY_PROP_IS_ENUMERABLE)
      {
        ecma_value_t desc_obj = ecma_op_object_get (props_p, prop_name_p);

        if (ECMA_IS_VALUE_ERROR (desc_obj))
        {
          ecma_free_property_descriptor (&prop_desc);
          goto cleanup;
        }

        ecma_value_t conv_result =
          ecma_op_to_property_descriptor (desc_obj, &property_descriptors[property_descriptor_number]);

        property_descriptors[property_descriptor_number].flags |= JERRY_PROP_SHOULD_THROW;

        ecma_free_value (desc_obj);

        if (ECMA_IS_VALUE_ERROR (conv_result))
        {
          ecma_free_property_descriptor (&prop_desc);
          goto cleanup;
        }

        property_descriptor_number++;
        ecma_free_value (conv_result);
        ecma_ref_ecma_string (prop_name_p);
        ecma_collection_push_back (enum_prop_names, buffer_p[i]);
      }

      ecma_free_property_descriptor (&prop_desc);
    }
  }

  /* 6. */
  for (uint32_t i = 0; i < enum_prop_names->item_count; i++)
  {
    ecma_string_t *prop_name_p = ecma_get_prop_name_from_value (enum_prop_names->buffer_p[i]);

    ecma_value_t define_own_prop_ret =
      ecma_op_object_define_own_property (obj_p, prop_name_p, &property_descriptors[i]);
    if (ECMA_IS_VALUE_ERROR (define_own_prop_ret))
    {
      goto cleanup;
    }

    ecma_free_value (define_own_prop_ret);
  }

  ecma_ref_object (obj_p);
  ret_value = ecma_make_object_value (obj_p);

cleanup:
  /* Clean up. */
  for (uint32_t index = 0; index < property_descriptor_number; index++)
  {
    ecma_free_property_descriptor (&property_descriptors[index]);
  }

  ecma_collection_free (enum_prop_names);

  JMEM_FINALIZE_LOCAL_ARRAY (property_descriptors);

  ecma_collection_free (prop_names_p);

  ecma_deref_object (props_p);

  return ret_value;
} /* ecma_builtin_object_object_define_properties */

/**
 * The Object object's 'create' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.5
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_create (ecma_value_t arg1, /**< routine's first argument */
                                   ecma_value_t arg2) /**< routine's second argument */
{
  /* 1. */
  if (!ecma_is_value_object (arg1) && !ecma_is_value_null (arg1))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_IS_NOT_AN_OBJECT);
  }

  ecma_object_t *obj_p = NULL;

  if (!ecma_is_value_null (arg1))
  {
    obj_p = ecma_get_object_from_value (arg1);
  }
  /* 2-3. */
  ecma_object_t *result_obj_p = ecma_op_create_object_object_noarg_and_set_prototype (obj_p);

  /* 4. */
  if (!ecma_is_value_undefined (arg2))
  {
    ecma_value_t obj = ecma_builtin_object_object_define_properties (result_obj_p, arg2);

    if (ECMA_IS_VALUE_ERROR (obj))
    {
      ecma_deref_object (result_obj_p);
      return obj;
    }

    ecma_free_value (obj);
  }

  /* 5. */
  return ecma_make_object_value (result_obj_p);
} /* ecma_builtin_object_object_create */

/**
 * The Object object's 'defineProperty' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.3.6
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_object_define_property (ecma_object_t *obj_p, /**< routine's first argument */
                                            ecma_string_t *name_str_p, /**< routine's second argument */
                                            ecma_value_t arg3) /**< routine's third argument */
{
  ecma_property_descriptor_t prop_desc;

  ecma_value_t conv_result = ecma_op_to_property_descriptor (arg3, &prop_desc);

  if (ECMA_IS_VALUE_ERROR (conv_result))
  {
    return conv_result;
  }

  prop_desc.flags |= JERRY_PROP_SHOULD_THROW;

  ecma_value_t define_own_prop_ret = ecma_op_object_define_own_property (obj_p, name_str_p, &prop_desc);

  ecma_free_property_descriptor (&prop_desc);
  ecma_free_value (conv_result);

  if (ECMA_IS_VALUE_ERROR (define_own_prop_ret))
  {
    return define_own_prop_ret;
  }

  if (ecma_is_value_false (define_own_prop_ret))
  {
    return ecma_raise_type_error (ECMA_ERR_THE_REQUESTED_PROPERTY_UPDATE_CANNOT_BE_PERFORMED);
  }

  JERRY_ASSERT (ecma_is_value_true (define_own_prop_ret));

  ecma_ref_object (obj_p);
  ecma_free_value (define_own_prop_ret);

  return ecma_make_object_value (obj_p);
} /* ecma_builtin_object_object_define_property */

/**
 * The Object object's 'assign' routine
 *
 * See also:
 *          ECMA-262 v6, 19.1.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_assign (ecma_object_t *target_p, /**< target object */
                                   const ecma_value_t arguments_list_p[], /**< arguments list */
                                   uint32_t arguments_list_len) /**< number of arguments */
{
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

  /* 4-5. */
  for (uint32_t i = 0; i < arguments_list_len && ecma_is_value_empty (ret_value); i++)
  {
    ecma_value_t next_source = arguments_list_p[i];

    /* 5.a */
    if (ecma_is_value_undefined (next_source) || ecma_is_value_null (next_source))
    {
      continue;
    }

    /* 5.b.i */
    ecma_value_t from_value = ecma_op_to_object (next_source);
    /* null and undefined cases are handled above, so this must be a valid object */
    JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (from_value));

    ecma_object_t *from_obj_p = ecma_get_object_from_value (from_value);

    /* 5.b.iii */
    ecma_collection_t *props_p = ecma_op_object_own_property_keys (from_obj_p, JERRY_PROPERTY_FILTER_ALL);

#if JERRY_BUILTIN_PROXY
    if (props_p == NULL)
    {
      ecma_deref_object (from_obj_p);
      return ECMA_VALUE_ERROR;
    }
#endif /* JERRY_BUILTIN_PROXY */

    ecma_value_t *buffer_p = props_p->buffer_p;

    for (uint32_t j = 0; (j < props_p->item_count) && ecma_is_value_empty (ret_value); j++)
    {
      ecma_string_t *property_name_p = ecma_get_prop_name_from_value (buffer_p[j]);

      /* 5.c.i-ii */
      ecma_property_descriptor_t prop_desc;
      ecma_value_t desc_status = ecma_op_object_get_own_property_descriptor (from_obj_p, property_name_p, &prop_desc);

#if JERRY_BUILTIN_PROXY
      if (ECMA_IS_VALUE_ERROR (desc_status))
      {
        ret_value = desc_status;
        break;
      }
#endif /* JERRY_BUILTIN_PROXY */

      if (ecma_is_value_false (desc_status))
      {
        continue;
      }

      /* 5.c.iii */
      if (prop_desc.flags & JERRY_PROP_IS_ENUMERABLE)
      {
        /* 5.c.iii.1 */
        ecma_value_t prop_value = ecma_op_object_get (from_obj_p, property_name_p);

        /* 5.c.iii.2 */
        if (ECMA_IS_VALUE_ERROR (prop_value))
        {
          ret_value = prop_value;
        }
        else
        {
          /* 5.c.iii.3 */
          ecma_value_t status = ecma_op_object_put (target_p, property_name_p, prop_value, true);

          /* 5.c.iii.4 */
          if (ECMA_IS_VALUE_ERROR (status))
          {
            ret_value = status;
          }
        }

        ecma_free_value (prop_value);
      }

      ecma_free_property_descriptor (&prop_desc);
    }

    ecma_deref_object (from_obj_p);
    ecma_collection_free (props_p);
  }

  /* 6. */
  if (ecma_is_value_empty (ret_value))
  {
    ecma_ref_object (target_p);
    return ecma_make_object_value (target_p);
  }

  return ret_value;
} /* ecma_builtin_object_object_assign */

/**
 * The Object object's 'is' routine
 *
 * See also:
 *          ECMA-262 v6, 19.1.2.10
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_object_is (ecma_value_t arg1, /**< routine's first argument */
                               ecma_value_t arg2) /**< routine's second argument */
{
  return ecma_op_same_value (arg1, arg2) ? ECMA_VALUE_TRUE : ECMA_VALUE_FALSE;
} /* ecma_builtin_object_object_is */

/**
 * The Object object's 'fromEntries' routine
 *
 * See also:
 *          ECMA-262 v10, 19.1.2.7
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_from_entries (ecma_value_t iterator) /**< object's iterator */
{
  JERRY_ASSERT (ecma_op_require_object_coercible (iterator));
  /* 2 */
  ecma_object_t *object_prototype_p = ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE);
  ecma_object_t *obj_p = ecma_create_object (object_prototype_p, 0, ECMA_OBJECT_TYPE_GENERAL);

  /* 6.a */
  ecma_value_t next_method;
  ecma_value_t result = ecma_op_get_iterator (iterator, ECMA_VALUE_SYNC_ITERATOR, &next_method);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    ecma_deref_object (obj_p);
    return result;
  }

  const ecma_value_t original_iterator = result;

  /* 6.b */
  while (true)
  {
    /* 6.a.i */
    result = ecma_op_iterator_step (original_iterator, next_method);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_iterator;
    }

    /* 6.a.ii */
    if (ecma_is_value_false (result))
    {
      break;
    }

    /* 6.a.iii */
    const ecma_value_t next = result;
    result = ecma_op_iterator_value (next);
    ecma_free_value (next);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      goto cleanup_iterator;
    }

    /* 6.a.iv */
    if (!ecma_is_value_object (result))
    {
      ecma_free_value (result);
      ecma_raise_type_error (ECMA_ERR_ITERATOR_VALUE_IS_NOT_AN_OBJECT);
      result = ecma_op_iterator_close (original_iterator);
      JERRY_ASSERT (ECMA_IS_VALUE_ERROR (result));
      goto cleanup_iterator;
    }

    /* 6.a.v-vi */
    ecma_object_t *next_object_p = ecma_get_object_from_value (result);

    result = ecma_op_object_get_by_index (next_object_p, 0);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_deref_object (next_object_p);
      ecma_op_iterator_close (original_iterator);
      goto cleanup_iterator;
    }

    const ecma_value_t key = result;

    result = ecma_op_object_get_by_index (next_object_p, 1);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      ecma_deref_object (next_object_p);
      ecma_free_value (key);
      ecma_op_iterator_close (original_iterator);
      goto cleanup_iterator;
    }

    /* 6.a.vii */
    const ecma_value_t value = result;
    ecma_string_t *property_key = ecma_op_to_property_key (key);

    if (property_key == NULL)
    {
      ecma_deref_object (next_object_p);
      ecma_free_value (key);
      ecma_op_iterator_close (original_iterator);
      result = ECMA_VALUE_ERROR;
      goto cleanup_iterator;
    }

    ecma_property_t *property_p = ecma_find_named_property (obj_p, property_key);

    if (property_p == NULL)
    {
      ecma_property_value_t *prop;
      prop =
        ecma_create_named_data_property (obj_p, property_key, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE, NULL);
      prop->value = ecma_copy_value_if_not_object (value);
    }
    else
    {
      ecma_named_data_property_assign_value (obj_p, ECMA_PROPERTY_VALUE_PTR (property_p), value);
    }

    ecma_deref_ecma_string (property_key);
    ecma_free_value (key);
    ecma_free_value (value);
    ecma_deref_object (next_object_p);
  }

  ecma_ref_object (obj_p);
  result = ecma_make_object_value (obj_p);

cleanup_iterator:
  ecma_free_value (original_iterator);
  ecma_free_value (next_method);
  ecma_deref_object (obj_p);

  return result;
} /* ecma_builtin_object_from_entries */

/**
 * GetOwnPropertyKeys abstract method
 *
 * See also:
 *          ECMA-262 v11, 19.1.2.11.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_op_object_get_own_property_keys (ecma_value_t this_arg, /**< this argument */
                                      uint16_t type) /**< routine type */
{
  /* 1. */
  ecma_value_t object = ecma_op_to_object (this_arg);

  if (ECMA_IS_VALUE_ERROR (object))
  {
    return object;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (object);

  /* 2. */
  jerry_property_filter_t filter = JERRY_PROPERTY_FILTER_EXCLUDE_SYMBOLS;

  if (type == ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_SYMBOLS)
  {
    filter = (jerry_property_filter_t) ((unsigned int) JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS | (unsigned int) JERRY_PROPERTY_FILTER_EXCLUDE_INTEGER_INDICES);
  }

  ecma_collection_t *props_p = ecma_op_object_own_property_keys (obj_p, filter);

  if (props_p == NULL)
  {
    ecma_deref_object (obj_p);
    return ECMA_VALUE_ERROR;
  }

  /* 3. */
  ecma_collection_t *name_list_p = ecma_new_collection ();

  /* 4. */
  for (uint32_t i = 0; i < props_p->item_count; i++)
  {
    ecma_value_t prop_name = props_p->buffer_p[i];
    ecma_string_t *name_p = ecma_get_prop_name_from_value (prop_name);

    if ((ecma_prop_name_is_symbol (name_p) && type == ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_SYMBOLS)
        || (ecma_is_value_string (prop_name) && type == ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_NAMES))
    {
      ecma_ref_ecma_string (name_p);
      ecma_collection_push_back (name_list_p, prop_name);
    }
  }

  ecma_value_t result_array = ecma_op_new_array_object_from_collection (name_list_p, false);

  ecma_deref_object (obj_p);
  ecma_collection_free (props_p);

  return result_array;
} /* ecma_op_object_get_own_property_keys */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine identifier */
                                      ecma_value_t this_arg, /**< 'this' argument value */
                                      const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                              *   passed to routine */
                                      uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED_2 (this_arg, arguments_number);

  ecma_value_t arg1 = arguments_list_p[0];
  ecma_value_t arg2 = arguments_list_p[1];

  /* No specialization for the arguments */
  switch (builtin_routine_id)
  {
    case ECMA_OBJECT_ROUTINE_CREATE:
    {
      return ecma_builtin_object_object_create (arg1, arg2);
    }
    case ECMA_OBJECT_ROUTINE_SET_PROTOTYPE_OF:
    {
      return ecma_builtin_object_object_set_prototype_of (arg1, arg2);
    }
    case ECMA_OBJECT_ROUTINE_IS:
    {
      return ecma_builtin_object_object_is (arg1, arg2);
    }
    default:
    {
      break;
    }
  }

  ecma_object_t *obj_p;

  if (builtin_routine_id <= ECMA_OBJECT_ROUTINE_DEFINE_PROPERTIES)
  {
    if (!ecma_is_value_object (arg1))
    {
      return ecma_raise_type_error (ECMA_ERR_ARGUMENT_IS_NOT_AN_OBJECT);
    }

    obj_p = ecma_get_object_from_value (arg1);

    if (builtin_routine_id == ECMA_OBJECT_ROUTINE_DEFINE_PROPERTY)
    {
      ecma_string_t *prop_name_p = ecma_op_to_property_key (arg2);

      if (prop_name_p == NULL)
      {
        return ECMA_VALUE_ERROR;
      }

      ecma_value_t result = ecma_builtin_object_object_define_property (obj_p, prop_name_p, arguments_list_p[2]);

      ecma_deref_ecma_string (prop_name_p);
      return result;
    }

    JERRY_ASSERT (builtin_routine_id == ECMA_OBJECT_ROUTINE_DEFINE_PROPERTIES);
    return ecma_builtin_object_object_define_properties (obj_p, arg2);
  }
  else if (builtin_routine_id <= ECMA_OBJECT_ROUTINE_ENTRIES)
  {
    ecma_value_t object = ecma_op_to_object (arg1);
    if (ECMA_IS_VALUE_ERROR (object))
    {
      return object;
    }

    obj_p = ecma_get_object_from_value (object);

    ecma_value_t result;
    switch (builtin_routine_id)
    {
      case ECMA_OBJECT_ROUTINE_GET_PROTOTYPE_OF:
      {
        result = ecma_builtin_object_object_get_prototype_of (obj_p);
        break;
      }
      case ECMA_OBJECT_ROUTINE_ASSIGN:
      {
        result = ecma_builtin_object_object_assign (obj_p, arguments_list_p + 1, arguments_number - 1);
        break;
      }
      case ECMA_OBJECT_ROUTINE_ENTRIES:
      case ECMA_OBJECT_ROUTINE_VALUES:
      case ECMA_OBJECT_ROUTINE_KEYS:
      {
        JERRY_ASSERT (builtin_routine_id - ECMA_OBJECT_ROUTINE_KEYS < ECMA_ENUMERABLE_PROPERTY__COUNT);

        const int option = builtin_routine_id - ECMA_OBJECT_ROUTINE_KEYS;
        result =
          ecma_builtin_object_object_keys_values_helper (obj_p, (ecma_enumerable_property_names_options_t) option);
        break;
      }
      case ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_DESCRIPTOR:
      {
        ecma_string_t *prop_name_p = ecma_op_to_property_key (arg2);

        if (prop_name_p == NULL)
        {
          result = ECMA_VALUE_ERROR;
          break;
        }

        result = ecma_builtin_object_object_get_own_property_descriptor (obj_p, prop_name_p);
        ecma_deref_ecma_string (prop_name_p);
        break;
      }
      case ECMA_OBJECT_ROUTINE_HAS_OWN:
      {
        ecma_string_t *prop_name_p = ecma_op_to_property_key (arg2);

        if (prop_name_p == NULL)
        {
          result = ECMA_VALUE_ERROR;
          break;
        }

        result = ecma_op_object_has_own_property (obj_p, prop_name_p);
        ecma_deref_ecma_string (prop_name_p);
        break;
      }
      case ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_DESCRIPTORS:
      {
        result = ecma_builtin_object_object_get_own_property_descriptors (obj_p);
        break;
      }
      case ECMA_OBJECT_ROUTINE_FROM_ENTRIES:
      {
        result = ecma_builtin_object_from_entries (arg1);
        break;
      }
      default:
      {
        JERRY_UNREACHABLE ();
      }
    }

    ecma_deref_object (obj_p);
    return result;
  }
  else if (builtin_routine_id <= ECMA_OBJECT_ROUTINE_GET_OWN_PROPERTY_SYMBOLS)
  {
    return ecma_op_object_get_own_property_keys (arg1, builtin_routine_id);
  }
  else if (builtin_routine_id <= ECMA_OBJECT_ROUTINE_SEAL)
  {
    if (!ecma_is_value_object (arg1))
    {
      return ecma_copy_value (arg1);
    }

    obj_p = ecma_get_object_from_value (arg1);
    switch (builtin_routine_id)
    {
      case ECMA_OBJECT_ROUTINE_SEAL:
      {
        return ecma_builtin_object_object_seal (obj_p);
      }
      case ECMA_OBJECT_ROUTINE_FREEZE:
      {
        return ecma_builtin_object_object_freeze (obj_p);
      }
      case ECMA_OBJECT_ROUTINE_PREVENT_EXTENSIONS:
      {
        return ecma_builtin_object_object_prevent_extensions (obj_p);
      }
      default:
      {
        JERRY_UNREACHABLE ();
      }
    }
  }
  else
  {
    JERRY_ASSERT (builtin_routine_id <= ECMA_OBJECT_ROUTINE_IS_SEALED);

    if (!ecma_is_value_object (arg1))
    {
      return ecma_make_boolean_value (builtin_routine_id != ECMA_OBJECT_ROUTINE_IS_EXTENSIBLE);
    }

    obj_p = ecma_get_object_from_value (arg1);
    switch (builtin_routine_id)
    {
      case ECMA_OBJECT_ROUTINE_IS_SEALED:
      case ECMA_OBJECT_ROUTINE_IS_FROZEN:
      {
        return ecma_builtin_object_test_integrity_level (obj_p, builtin_routine_id);
      }
      case ECMA_OBJECT_ROUTINE_IS_EXTENSIBLE:
      {
        return ecma_builtin_object_object_is_extensible (obj_p);
      }
      default:
      {
        JERRY_UNREACHABLE ();
      }
    }
  }
} /* ecma_builtin_object_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
