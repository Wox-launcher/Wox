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

#include "ecma-alloc.h"
#include "ecma-builtin-helpers.h"
#include "ecma-builtin-object.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-proxy-object.h"
#include "ecma-string-object.h"

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
  /* Note: these 6 routines must be in this order */
  ECMA_OBJECT_PROTOTYPE_ROUTINE_START = 0,
  ECMA_OBJECT_PROTOTYPE_TO_STRING,
  ECMA_OBJECT_PROTOTYPE_VALUE_OF,
  ECMA_OBJECT_PROTOTYPE_TO_LOCALE_STRING,
  ECMA_OBJECT_PROTOTYPE_GET_PROTO,
  ECMA_OBJECT_PROTOTYPE_IS_PROTOTYPE_OF,
  ECMA_OBJECT_PROTOTYPE_HAS_OWN_PROPERTY,
  ECMA_OBJECT_PROTOTYPE_PROPERTY_IS_ENUMERABLE,
  ECMA_OBJECT_PROTOTYPE_SET_PROTO,
#if JERRY_BUILTIN_ANNEXB
  ECMA_OBJECT_PROTOTYPE_DEFINE_GETTER,
  ECMA_OBJECT_PROTOTYPE_DEFINE_SETTER,
  ECMA_OBJECT_PROTOTYPE_LOOKUP_GETTER,
  ECMA_OBJECT_PROTOTYPE_LOOKUP_SETTER,
#endif /* JERRY_BUILTIN_ANNEXB */
};

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-object-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  object_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup objectprototype ECMA Object.prototype object built-in
 * @{
 */

/**
 * The Object.prototype object's 'toString' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.2
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_prototype_object_to_string (ecma_value_t this_arg) /**< this argument */
{
  return ecma_builtin_helper_object_to_string (this_arg);
} /* ecma_builtin_object_prototype_object_to_string */

/**
 * The Object.prototype object's 'valueOf' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.4
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_prototype_object_value_of (ecma_value_t this_arg) /**< this argument */
{
  return ecma_op_to_object (this_arg);
} /* ecma_builtin_object_prototype_object_value_of */

/**
 * The Object.prototype object's 'toLocaleString' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_prototype_object_to_locale_string (ecma_value_t this_arg) /**< this argument */
{
  return ecma_op_invoke_by_magic_id (this_arg, LIT_MAGIC_STRING_TO_STRING_UL, &this_arg, 1);
} /* ecma_builtin_object_prototype_object_to_locale_string */

/**
 * The Object.prototype object's 'hasOwnProperty' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.5
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_prototype_object_has_own_property (ecma_object_t *obj_p, /**< this argument */
                                                       ecma_string_t *prop_name_p) /**< first argument */
{
  return ecma_op_object_has_own_property (obj_p, prop_name_p);
} /* ecma_builtin_object_prototype_object_has_own_property */

/**
 * The Object.prototype object's 'isPrototypeOf' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.6
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_prototype_object_is_prototype_of (ecma_object_t *obj_p, /**< this argument */
                                                      ecma_value_t arg) /**< routine's first argument */
{
  /* 3. Compare prototype to object */
  ecma_value_t v_obj_value = ecma_op_to_object (arg);

  if (ECMA_IS_VALUE_ERROR (v_obj_value))
  {
    return v_obj_value;
  }

  ecma_object_t *v_obj_p = ecma_get_object_from_value (v_obj_value);

  ecma_value_t ret_value = ecma_op_object_is_prototype_of (obj_p, v_obj_p);

  ecma_deref_object (v_obj_p);

  return ret_value;
} /* ecma_builtin_object_prototype_object_is_prototype_of */

/**
 * The Object.prototype object's 'propertyIsEnumerable' routine
 *
 * See also:
 *          ECMA-262 v5, 15.2.4.7
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_object_prototype_object_property_is_enumerable (ecma_object_t *obj_p, /**< this argument */
                                                             ecma_string_t *prop_name_p) /**< first argument */
{
  ecma_property_descriptor_t prop_desc;
  ecma_value_t status = ecma_op_object_get_own_property_descriptor (obj_p, prop_name_p, &prop_desc);

  if (!ecma_is_value_true (status))
  {
    return status;
  }

  bool is_enumerable = (prop_desc.flags & JERRY_PROP_IS_ENUMERABLE);

  ecma_free_property_descriptor (&prop_desc);

  return ecma_make_boolean_value (is_enumerable);
} /* ecma_builtin_object_prototype_object_property_is_enumerable */

#if JERRY_BUILTIN_ANNEXB
/**
 * The Object.prototype object's '__defineGetter__' and '__defineSetter__' routine
 *
 * See also:
 *          ECMA-262 v11, B.2.2.2
 *          ECMA-262 v11, B.2.2.3
 *
 * @return ECMA_VALUE_ERROR - if the operation fails,
 *         ECMA_VALUE_UNDEFINED - otherwise
 */
static ecma_value_t
ecma_builtin_object_prototype_define_getter_setter (ecma_value_t this_arg, /**< this argument */
                                                    ecma_value_t prop, /**< property */
                                                    ecma_value_t accessor, /**< getter/setter function */
                                                    bool define_getter) /**< true - defineGetter method
                                                                             false - defineSetter method */
{
  /* 1. */
  ecma_value_t to_obj = ecma_op_to_object (this_arg);

  if (ECMA_IS_VALUE_ERROR (to_obj))
  {
    return to_obj;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (to_obj);

  /* 2. */
  if (!ecma_op_is_callable (accessor))
  {
    ecma_deref_object (obj_p);
    return ecma_raise_type_error (ECMA_ERR_GETTER_IS_NOT_CALLABLE);
  }

  ecma_object_t *accessor_obj_p = ecma_get_object_from_value (accessor);

  /* 3. */
  ecma_property_descriptor_t desc = ecma_make_empty_property_descriptor ();
  desc.flags |= (JERRY_PROP_IS_ENUMERABLE | JERRY_PROP_IS_CONFIGURABLE | JERRY_PROP_IS_ENUMERABLE_DEFINED
                 | JERRY_PROP_IS_CONFIGURABLE_DEFINED | JERRY_PROP_SHOULD_THROW);

  if (define_getter)
  {
    desc.get_p = accessor_obj_p;
    desc.flags |= JERRY_PROP_IS_GET_DEFINED;
  }
  else
  {
    desc.set_p = accessor_obj_p;
    desc.flags |= JERRY_PROP_IS_SET_DEFINED;
  }

  /* 4. */
  ecma_string_t *prop_name_p = ecma_op_to_property_key (prop);

  if (JERRY_UNLIKELY (prop_name_p == NULL))
  {
    ecma_deref_object (obj_p);
    return ECMA_VALUE_ERROR;
  }

  /* 5. */
  ecma_value_t define_prop = ecma_op_object_define_own_property (obj_p, prop_name_p, &desc);

  ecma_deref_object (obj_p);
  ecma_deref_ecma_string (prop_name_p);

  if (ECMA_IS_VALUE_ERROR (define_prop))
  {
    return define_prop;
  }

  /* 6. */
  return ECMA_VALUE_UNDEFINED;
} /* ecma_builtin_object_prototype_define_getter_setter */

/**
 * The Object.prototype object's '__lookupGetter__' and '__lookupSetter__' routine
 *
 * See also:
 *          ECMA-262 v11, B.2.2.4
 *          ECMA-262 v11, B.2.2.5
 *
 * @return ECMA_VALUE_ERROR - if the operation fails,
 *         ECMA_VALUE_UNDEFINED - if the property was not found
 *         Accessor property - otherwise
 */
static ecma_value_t
ecma_builtin_object_prototype_lookup_getter_setter (ecma_value_t this_arg, /**< this argument */
                                                    ecma_value_t prop, /**< property */
                                                    bool lookup_getter) /**< true - lookupGetter method
                                                                             false - lookupSetter method */
{
  /* 1. */
  ecma_value_t to_obj = ecma_op_to_object (this_arg);

  if (ECMA_IS_VALUE_ERROR (to_obj))
  {
    return to_obj;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (to_obj);

  /* 2. */
  ecma_string_t *prop_name_p = ecma_op_to_property_key (prop);

  if (JERRY_UNLIKELY (prop_name_p == NULL))
  {
    ecma_deref_object (obj_p);
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t ret_value = ECMA_VALUE_UNDEFINED;

  ecma_ref_object (obj_p);

  /* 3. */
  while (true)
  {
    /* 3.a */
    ecma_property_descriptor_t desc;
    ecma_value_t get_desc = ecma_op_object_get_own_property_descriptor (obj_p, prop_name_p, &desc);

    if (ECMA_IS_VALUE_ERROR (get_desc))
    {
      ret_value = get_desc;
      ecma_deref_object (obj_p);
      break;
    }

    /* 3.b */
    if (ecma_is_value_true (get_desc))
    {
      if ((desc.flags & JERRY_PROP_IS_SET_DEFINED) || (desc.flags & JERRY_PROP_IS_GET_DEFINED))
      {
        if (lookup_getter && desc.get_p != NULL)
        {
          ecma_ref_object (desc.get_p);
          ret_value = ecma_make_object_value (desc.get_p);
        }
        else if (!lookup_getter && desc.set_p != NULL)
        {
          ecma_ref_object (desc.set_p);
          ret_value = ecma_make_object_value (desc.set_p);
        }
      }

      ecma_free_property_descriptor (&desc);
      ecma_deref_object (obj_p);
      break;
    }

    /* 3.c */
    ecma_object_t *proto_p = ecma_op_object_get_prototype_of (obj_p);
    ecma_deref_object (obj_p);

    if (proto_p == NULL)
    {
      break;
    }
    else if (JERRY_UNLIKELY (proto_p == ECMA_OBJECT_POINTER_ERROR))
    {
      ret_value = ECMA_VALUE_ERROR;
      break;
    }

    /* Advance up on prototype chain. */
    obj_p = proto_p;
  }

  ecma_free_value (to_obj);
  ecma_deref_ecma_string (prop_name_p);

  return ret_value;
} /* ecma_builtin_object_prototype_lookup_getter_setter */
#endif /* JERRY_BUILTIN_ANNEXB */

/**
 * Dispatcher of the built-in's routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_object_prototype_dispatch_routine (uint8_t builtin_routine_id, /**< built-in wide routine
                                                                             *   identifier */
                                                ecma_value_t this_arg, /**< 'this' argument value */
                                                const ecma_value_t arguments_list_p[], /**< list of arguments
                                                                                        *   passed to routine */
                                                uint32_t arguments_number) /**< length of arguments' list */
{
  JERRY_UNUSED (arguments_number);

  /* no specialization */
  if (builtin_routine_id <= ECMA_OBJECT_PROTOTYPE_VALUE_OF)
  {
    if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_TO_STRING)
    {
      return ecma_builtin_object_prototype_object_to_string (this_arg);
    }

    JERRY_ASSERT (builtin_routine_id <= ECMA_OBJECT_PROTOTYPE_VALUE_OF);

    return ecma_builtin_object_prototype_object_value_of (this_arg);
  }

  if (builtin_routine_id <= ECMA_OBJECT_PROTOTYPE_IS_PROTOTYPE_OF)
  {
    if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_IS_PROTOTYPE_OF)
    {
      /* 15.2.4.6.1. */
      if (!ecma_is_value_object (arguments_list_p[0]))
      {
        return ECMA_VALUE_FALSE;
      }
    }

    if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_TO_LOCALE_STRING)
    {
      return ecma_builtin_object_prototype_object_to_locale_string (this_arg);
    }

    ecma_value_t to_object = ecma_op_to_object (this_arg);

    if (ECMA_IS_VALUE_ERROR (to_object))
    {
      return to_object;
    }

    ecma_object_t *obj_p = ecma_get_object_from_value (to_object);

    ecma_value_t ret_value;

    if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_GET_PROTO)
    {
      ret_value = ecma_builtin_object_object_get_prototype_of (obj_p);
    }
    else
    {
      ret_value = ecma_builtin_object_prototype_object_is_prototype_of (obj_p, arguments_list_p[0]);
    }

    ecma_deref_object (obj_p);

    return ret_value;
  }

  JERRY_ASSERT (builtin_routine_id >= ECMA_OBJECT_PROTOTYPE_HAS_OWN_PROPERTY);

  if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_SET_PROTO)
  {
    return ecma_builtin_object_object_set_proto (this_arg, arguments_list_p[0]);
  }
#if JERRY_BUILTIN_ANNEXB
  else if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_LOOKUP_GETTER)
  {
    return ecma_builtin_object_prototype_lookup_getter_setter (this_arg, arguments_list_p[0], true);
  }
  else if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_LOOKUP_SETTER)
  {
    return ecma_builtin_object_prototype_lookup_getter_setter (this_arg, arguments_list_p[0], false);
  }
  else if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_DEFINE_GETTER)
  {
    return ecma_builtin_object_prototype_define_getter_setter (this_arg,
                                                               arguments_list_p[0],
                                                               arguments_list_p[1],
                                                               true);
  }
  else if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_DEFINE_SETTER)
  {
    return ecma_builtin_object_prototype_define_getter_setter (this_arg,
                                                               arguments_list_p[0],
                                                               arguments_list_p[1],
                                                               false);
  }
#endif /* JERRY_BUILTIN_ANNEXB */

  ecma_string_t *prop_name_p = ecma_op_to_property_key (arguments_list_p[0]);

  if (prop_name_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t to_object = ecma_op_to_object (this_arg);

  if (ECMA_IS_VALUE_ERROR (to_object))
  {
    ecma_deref_ecma_string (prop_name_p);
    return to_object;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (to_object);

  ecma_value_t ret_value;

  if (builtin_routine_id == ECMA_OBJECT_PROTOTYPE_HAS_OWN_PROPERTY)
  {
    ret_value = ecma_builtin_object_prototype_object_has_own_property (obj_p, prop_name_p);
  }
  else
  {
    ret_value = ecma_builtin_object_prototype_object_property_is_enumerable (obj_p, prop_name_p);
  }

  ecma_deref_ecma_string (prop_name_p);
  ecma_deref_object (obj_p);

  return ret_value;
} /* ecma_builtin_object_prototype_dispatch_routine */

/**
 * @}
 * @}
 * @}
 */
