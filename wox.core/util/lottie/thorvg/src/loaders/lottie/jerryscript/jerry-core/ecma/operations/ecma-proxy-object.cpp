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

#include "ecma-proxy-object.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-builtin-handlers.h"
#include "ecma-builtin-object.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#include "jcontext.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaproxyobject ECMA Proxy object related routines
 * @{
 */

#if JERRY_BUILTIN_PROXY
/**
 * ProxyCreate operation for create a new proxy object
 *
 * See also:
 *         ES2015 9.5.15
 *         ES11+: 9.5.14 ProxyCreate
 *
 * @return created Proxy object as an ecma-value - if success
 *         raised error - otherwise
 */
ecma_object_t *
ecma_proxy_create (ecma_value_t target, /**< proxy target */
                   ecma_value_t handler, /**< proxy handler */
                   uint32_t options) /**< ecma_proxy_flag_types_t option bits */
{
  /* ES2015: 1, 3. */
  /* ES11+: 1 - 2. */
  if (!ecma_is_value_object (target) || !ecma_is_value_object (handler))
  {
    ecma_raise_type_error (ECMA_ERR_CANNOT_CREATE_PROXY);
    return NULL;
  }

  /* ES2015: 5 - 6. */
  /* ES11+: 3 - 4. */
  /* A Proxy does not have [[Prototype]] value as per standard */
  ecma_object_t *obj_p = ecma_create_object (NULL, sizeof (ecma_proxy_object_t), ECMA_OBJECT_TYPE_PROXY);

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  obj_p->u2.prototype_cp = (jmem_cpointer_t) options;

  /* ES2015: 7. */
  /* ES11+: 5. */
  if (ecma_op_is_callable (target))
  {
    obj_p->u2.prototype_cp |= ECMA_PROXY_IS_CALLABLE;

    /* ES2015: 7.b. */
    /* ES11+: 5.b. */
    if (ecma_is_constructor (target))
    {
      obj_p->u2.prototype_cp |= ECMA_PROXY_IS_CONSTRUCTABLE;
    }
  }

  /* ES2015: 8. */
  /* ES11+: 6. */
  proxy_obj_p->target = target;

  /* ES2015: 9. */
  /* ES11+: 7. */
  proxy_obj_p->handler = handler;

  /* ES2015: 10. */
  /* ES11+: 8 */
  return obj_p;
} /* ecma_proxy_create */

/**
 * Definition of Proxy Revocation Function
 *
 * See also:
 *         ES2015 26.2.2.1.1
 *
 * @return ECMA_VALUE_UNDEFINED
 */
ecma_value_t
ecma_proxy_revoke_cb (ecma_object_t *function_obj_p, /**< function object */
                      const ecma_value_t args_p[], /**< argument list */
                      const uint32_t args_count) /**< argument number */
{
  JERRY_UNUSED_2 (args_p, args_count);

  /* 1. */
  ecma_revocable_proxy_object_t *rev_proxy_p = (ecma_revocable_proxy_object_t *) function_obj_p;

  /* 2. */
  if (ecma_is_value_null (rev_proxy_p->proxy))
  {
    return ECMA_VALUE_UNDEFINED;
  }

  /* 4. */
  ecma_proxy_object_t *proxy_p = (ecma_proxy_object_t *) ecma_get_object_from_value (rev_proxy_p->proxy);
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY ((ecma_object_t *) proxy_p));

  /* 3. */
  rev_proxy_p->proxy = ECMA_VALUE_NULL;

  /* 5. */
  proxy_p->target = ECMA_VALUE_NULL;

  /* 6. */
  proxy_p->handler = ECMA_VALUE_NULL;

  /* 7. */
  return ECMA_VALUE_UNDEFINED;
} /* ecma_proxy_revoke_cb */

/**
 * Proxy.revocable operation for create a new revocable proxy object
 *
 * See also:
 *         ES2015 26.2.2.1
 *
 * @return NULL - if the operation fails
 *         pointer to the newly created revocable proxy object - otherwise
 */
ecma_object_t *
ecma_proxy_create_revocable (ecma_value_t target, /**< target argument */
                             ecma_value_t handler) /**< handler argument */
{
  /* 1. */
  ecma_object_t *proxy_p = ecma_proxy_create (target, handler, 0);

  /* 2. */
  if (proxy_p == NULL)
  {
    return proxy_p;
  }

  ecma_value_t proxy_value = ecma_make_object_value (proxy_p);

  /* 3. */
  ecma_object_t *func_obj_p;
  func_obj_p = ecma_op_create_native_handler (ECMA_NATIVE_HANDLER_PROXY_REVOKE, sizeof (ecma_revocable_proxy_object_t));

  /* 4. */
  ecma_revocable_proxy_object_t *rev_proxy_p = (ecma_revocable_proxy_object_t *) func_obj_p;
  rev_proxy_p->proxy = proxy_value;

  ecma_property_value_t *prop_value_p;
  ecma_value_t revoker = ecma_make_object_value (func_obj_p);

  /* 5. */
  ecma_object_t *obj_p =
    ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE), 0, ECMA_OBJECT_TYPE_GENERAL);

  /* 6. */
  prop_value_p = ecma_create_named_data_property (obj_p,
                                                  ecma_get_magic_string (LIT_MAGIC_STRING_PROXY),
                                                  ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                  NULL);
  prop_value_p->value = proxy_value;

  /* 7. */
  prop_value_p = ecma_create_named_data_property (obj_p,
                                                  ecma_get_magic_string (LIT_MAGIC_STRING_REVOKE),
                                                  ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                  NULL);
  prop_value_p->value = revoker;

  ecma_deref_object (proxy_p);
  ecma_deref_object (func_obj_p);

  /* 8. */
  return obj_p;
} /* ecma_proxy_create_revocable */

/**
 * Internal find property operation for Proxy object
 *
 * Note: Returned value must be freed with ecma_free_value.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_NOT_FOUND - if the property is not found
 *         value of the property - otherwise
 */
ecma_value_t
ecma_proxy_object_find (ecma_object_t *obj_p, /**< proxy object */
                        ecma_string_t *prop_name_p) /**< property name */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));

  ecma_value_t has_result = ecma_proxy_object_has (obj_p, prop_name_p);

  if (ECMA_IS_VALUE_ERROR (has_result))
  {
    return has_result;
  }

  if (ecma_is_value_false (has_result))
  {
    return ECMA_VALUE_NOT_FOUND;
  }

  return ecma_proxy_object_get (obj_p, prop_name_p, ecma_make_object_value (obj_p));
} /* ecma_proxy_object_find */

/**
 * Helper method for validate the proxy object
 *
 * @return proxy trap - if the validation is successful
 *         ECMA_VALUE_ERROR - otherwise
 */
static ecma_value_t
ecma_validate_proxy_object (ecma_value_t handler, /**< proxy handler */
                            lit_magic_string_id_t magic_id) /**< routine magic id */
{
  if (ecma_is_value_null (handler))
  {
    return ecma_raise_type_error (ECMA_ERR_HANDLER_CANNOT_BE_NULL);
  }

  JERRY_ASSERT (ecma_is_value_object (handler));

  return ecma_op_get_method_by_magic_id (handler, magic_id);
} /* ecma_validate_proxy_object */

/* Internal operations */

/**
 * The Proxy object [[GetPrototypeOf]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.1
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_NULL or valid object (prototype) otherwise
 */
ecma_value_t
ecma_proxy_object_get_prototype_of (ecma_object_t *obj_p) /**< proxy object */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 1. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 2-5. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_GET_PROTOTYPE_OF_UL);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 7. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t result = ecma_builtin_object_object_get_prototype_of (target_obj_p);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);

  /* 8. */
  ecma_value_t handler_proto = ecma_op_function_call (func_obj_p, handler, &target, 1);

  ecma_deref_object (func_obj_p);

  /* 9. */
  if (ECMA_IS_VALUE_ERROR (handler_proto))
  {
    return handler_proto;
  }

  /* 10. */
  if (!ecma_is_value_object (handler_proto) && !ecma_is_value_null (handler_proto))
  {
    ecma_free_value (handler_proto);

    return ecma_raise_type_error (ECMA_ERR_TRAP_RETURNED_NEITHER_OBJECT_NOR_NULL);
  }

  if (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION)
  {
    return handler_proto;
  }

  /* 11. */
  ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

  /* 12. */
  if (ECMA_IS_VALUE_ERROR (extensible_target))
  {
    ecma_free_value (handler_proto);

    return extensible_target;
  }

  /* 13. */
  if (ecma_is_value_true (extensible_target))
  {
    return handler_proto;
  }

  /* 14. */
  ecma_value_t target_proto = ecma_builtin_object_object_get_prototype_of (target_obj_p);

  /* 15. */
  if (ECMA_IS_VALUE_ERROR (target_proto))
  {
    return target_proto;
  }

  ecma_value_t ret_value = handler_proto;

  /* 16. */
  if (handler_proto != target_proto)
  {
    ecma_free_value (handler_proto);

    ret_value = ecma_raise_type_error (ECMA_ERR_TARGET_NOT_EXTENSIBLE_NOT_RETURNED_ITS_PROTOTYPE);
  }

  ecma_free_value (target_proto);

  /* 17. */
  return ret_value;
} /* ecma_proxy_object_get_prototype_of */

/**
 * The Proxy object [[SetPrototypeOf]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.2
 *          ECMAScript v11: 9.5.2
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE/FALSE} - depends on whether the new prototype can be set for the given object
 */
ecma_value_t
ecma_proxy_object_set_prototype_of (ecma_object_t *obj_p, /**< proxy object */
                                    ecma_value_t proto) /**< new prototype object */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  /* 1. */
  JERRY_ASSERT (ecma_is_value_object (proto) || ecma_is_value_null (proto));

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 2. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 3-6. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_SET_PROTOTYPE_OF_UL);

  /* 7.*/
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 8. */
  if (ecma_is_value_undefined (trap))
  {
    if (ECMA_OBJECT_IS_PROXY (target_obj_p))
    {
      ecma_value_t result = ecma_proxy_object_set_prototype_of (target_obj_p, proto);
      JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
      return result;
    }

    ecma_value_t result = ecma_op_ordinary_object_set_prototype_of (target_obj_p, proto);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t args[] = { target, proto };

  /* 9. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, args, 2);

  ecma_deref_object (func_obj_p);

  /* 10. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  bool boolean_trap_result = ecma_op_to_boolean (trap_result);

  ecma_free_value (trap_result);

  /* ES11: 9 */
  if (!boolean_trap_result)
  {
    return ecma_make_boolean_value (false);
  }

  if (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION)
  {
    return ecma_make_boolean_value (boolean_trap_result);
  }

  /* 11. */
  ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

  /* 12. */
  if (ECMA_IS_VALUE_ERROR (extensible_target))
  {
    return extensible_target;
  }

  /* 13. */
  if (ecma_is_value_true (extensible_target))
  {
    return ecma_make_boolean_value (boolean_trap_result);
  }

  /* 14. */
  ecma_value_t target_proto = ecma_builtin_object_object_get_prototype_of (target_obj_p);

  /* 15. */
  if (ECMA_IS_VALUE_ERROR (target_proto))
  {
    return target_proto;
  }

  ecma_value_t ret_value = ecma_make_boolean_value (boolean_trap_result);

  /* 16. */
  if (boolean_trap_result && (target_proto != proto))
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_TARGET_NOT_EXTENSIBLE_DIFFERENT_PROTOTYPE_RETURNED);
  }

  ecma_free_value (target_proto);

  /* 17. */
  return ret_value;
} /* ecma_proxy_object_set_prototype_of */

/**
 * The Proxy object [[isExtensible]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.3
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE/FALSE} - depends on whether the object is extensible
 */
ecma_value_t
ecma_proxy_object_is_extensible (ecma_object_t *obj_p) /**< proxy object */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 1. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 2-5. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_IS_EXTENSIBLE);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 7. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t result = ecma_builtin_object_object_is_extensible (target_obj_p);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);

  /* 8. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, &target, 1);

  ecma_deref_object (func_obj_p);

  /* 9. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  bool boolean_trap_result = ecma_op_to_boolean (trap_result);

  ecma_free_value (trap_result);

  if (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION)
  {
    return ecma_make_boolean_value (boolean_trap_result);
  }

  bool target_result;

  /* 10. */
  if (ECMA_OBJECT_IS_PROXY (target_obj_p))
  {
    ecma_value_t proxy_is_ext = ecma_proxy_object_is_extensible (target_obj_p);

    if (ECMA_IS_VALUE_ERROR (proxy_is_ext))
    {
      return proxy_is_ext;
    }

    target_result = ecma_is_value_true (proxy_is_ext);
  }
  else
  {
    target_result = ecma_op_ordinary_object_is_extensible (target_obj_p);
  }

  /* 12. */
  if (boolean_trap_result != target_result)
  {
    return ecma_raise_type_error (ECMA_ERR_TRAP_RESULT_NOT_REFLECT_TARGET_EXTENSIBILITY);
  }

  return ecma_make_boolean_value (boolean_trap_result);
} /* ecma_proxy_object_is_extensible */

/**
 * The Proxy object [[PreventExtensions]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.4
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE/FALSE} - depends on whether the object can be set as inextensible
 */
ecma_value_t
ecma_proxy_object_prevent_extensions (ecma_object_t *obj_p) /**< proxy object */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 1. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 2-5. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_PREVENT_EXTENSIONS_UL);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 7. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t ret_value = ecma_builtin_object_object_prevent_extensions (target_obj_p);

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      return ret_value;
    }

    ecma_deref_object (target_obj_p);

    return ECMA_VALUE_TRUE;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);

  /* 8. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, &target, 1);

  ecma_deref_object (func_obj_p);

  /* 9. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  bool boolean_trap_result = ecma_op_to_boolean (trap_result);

  ecma_free_value (trap_result);

  /* 10. */
  if (boolean_trap_result && !(obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION))
  {
    ecma_value_t target_is_ext = ecma_builtin_object_object_is_extensible (target_obj_p);

    if (ECMA_IS_VALUE_ERROR (target_is_ext))
    {
      return target_is_ext;
    }

    if (ecma_is_value_true (target_is_ext))
    {
      return ecma_raise_type_error (ECMA_ERR_TRAP_RESULT_NOT_REFLECT_TARGET_INEXTENSIBILITY);
    }
  }

  /* 11. */
  return ecma_make_boolean_value (boolean_trap_result);
} /* ecma_proxy_object_prevent_extensions */

/**
 * The Proxy object [[GetOwnProperty]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.5
 *
 * Note: - Returned value is always a simple value so freeing it is unnecessary.
 *       - If the operation does not fail, freeing the filled property descriptor is the caller's responsibility
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE_FALSE} - depends on whether object has property with the given name
 */
ecma_value_t
ecma_proxy_object_get_own_property_descriptor (ecma_object_t *obj_p, /**< proxy object */
                                               ecma_string_t *prop_name_p, /**< property name */
                                               ecma_property_descriptor_t *prop_desc_p) /**< [out] property
                                                                                         *   descriptor */
{
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 2. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 3-6. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_GET_OWN_PROPERTY_DESCRIPTOR_UL);

  ecma_value_t target = proxy_obj_p->target;

  /* 7. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 8. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t result = ecma_op_object_get_own_property_descriptor (target_obj_p, prop_name_p, prop_desc_p);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t prop_value = ecma_make_prop_name_value (prop_name_p);
  ecma_value_t args[] = { target, prop_value };

  /* 9. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, args, 2);
  ecma_deref_object (func_obj_p);

  /* 10. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  /* 11. */
  if (!ecma_is_value_object (trap_result) && !ecma_is_value_undefined (trap_result))
  {
    ecma_free_value (trap_result);
    return ecma_raise_type_error (ECMA_ERR_TRAP_IS_NEITHER_AN_OBJECT_NOR_UNDEFINED);
  }

  if (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION)
  {
    if (ecma_is_value_undefined (trap_result))
    {
      return ECMA_VALUE_FALSE;
    }

    ecma_value_t result_val = ecma_op_to_property_descriptor (trap_result, prop_desc_p);
    ecma_free_value (trap_result);

    if (ECMA_IS_VALUE_ERROR (result_val))
    {
      return result_val;
    }

    ecma_op_to_complete_property_descriptor (prop_desc_p);
    return ECMA_VALUE_TRUE;
  }

  /* 12. */
  ecma_property_descriptor_t target_desc;
  ecma_value_t target_status = ecma_op_object_get_own_property_descriptor (target_obj_p, prop_name_p, &target_desc);

  /* 13. */
  if (ECMA_IS_VALUE_ERROR (target_status))
  {
    ecma_free_value (trap_result);
    return target_status;
  }

  /* 14. */
  if (ecma_is_value_undefined (trap_result))
  {
    /* .a */
    if (ecma_is_value_false (target_status))
    {
      return ECMA_VALUE_FALSE;
    }
    /* .b */
    if (!(target_desc.flags & JERRY_PROP_IS_CONFIGURABLE))
    {
      ecma_free_property_descriptor (&target_desc);
      return ecma_raise_type_error (ECMA_ERR_GIVEN_PROPERTY_IS_A_NON_CONFIGURABLE);
    }

    /* .c */
    ecma_free_property_descriptor (&target_desc);
    ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

    /* .d */
    if (ECMA_IS_VALUE_ERROR (extensible_target))
    {
      return extensible_target;
    }

    /* .e */
    JERRY_ASSERT (ecma_is_value_boolean (extensible_target));

    /* .f */
    if (ecma_is_value_false (extensible_target))
    {
      return ecma_raise_type_error (ECMA_ERR_TARGET_NOT_EXTENSIBLE);
    }

    /* .g */
    return ECMA_VALUE_FALSE;
  }

  /* 15. */
  ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

  /* 16. */
  if (ECMA_IS_VALUE_ERROR (extensible_target))
  {
    if (ecma_is_value_true (target_status))
    {
      ecma_free_property_descriptor (&target_desc);
    }
    ecma_free_value (trap_result);
    return extensible_target;
  }

  /* 17, 19 */
  ecma_value_t result_val = ecma_op_to_property_descriptor (trap_result, prop_desc_p);

  ecma_op_to_complete_property_descriptor (prop_desc_p);
  ecma_free_value (trap_result);

  /* 18. */
  if (ECMA_IS_VALUE_ERROR (result_val))
  {
    if (ecma_is_value_true (target_status))
    {
      ecma_free_property_descriptor (&target_desc);
    }
    return result_val;
  }

  /* 20. */
  bool is_extensible = ecma_is_value_true (extensible_target);

  bool is_valid = ecma_op_is_compatible_property_descriptor (prop_desc_p,
                                                             (ecma_is_value_true (target_status) ? &target_desc : NULL),
                                                             is_extensible);

  bool target_has_desc = ecma_is_value_true (target_status);
  bool target_is_writable = (target_desc.flags & JERRY_PROP_IS_WRITABLE);
  bool target_is_configurable = false;

  if (target_has_desc)
  {
    target_is_configurable = ((target_desc.flags & JERRY_PROP_IS_CONFIGURABLE) != 0);
    ecma_free_property_descriptor (&target_desc);
  }

  /* 21. */
  if (!is_valid)
  {
    ecma_free_property_descriptor (prop_desc_p);
    return ecma_raise_type_error (ECMA_ERR_THE_TWO_DESCRIPTORS_ARE_INCOMPATIBLE);
  }

  /* 22. */
  else if (!(prop_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE))
  {
    const uint16_t mask = (JERRY_PROP_IS_WRITABLE_DEFINED | JERRY_PROP_IS_WRITABLE);

    if (!target_has_desc || target_is_configurable
        || ((prop_desc_p->flags & mask) == JERRY_PROP_IS_WRITABLE_DEFINED && target_is_writable))
    {
      ecma_free_property_descriptor (prop_desc_p);
      return ecma_raise_type_error (ECMA_ERR_THE_TWO_DESCRIPTORS_ARE_INCOMPATIBLE);
    }
  }
  return ECMA_VALUE_TRUE;
} /* ecma_proxy_object_get_own_property_descriptor */

/**
 * The Proxy object [[DefineOwnProperty]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.6
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE_FALSE} - depends on whether the property can be defined for the given object
 */
ecma_value_t
ecma_proxy_object_define_own_property (ecma_object_t *obj_p, /**< proxy object */
                                       ecma_string_t *prop_name_p, /**< property name */
                                       const ecma_property_descriptor_t *prop_desc_p) /**< property descriptor */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 2. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 3-6. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_DEFINE_PROPERTY_UL);

  /* 7. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 8. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t result = ecma_op_object_define_own_property (target_obj_p, prop_name_p, prop_desc_p);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  /* 9. */
  ecma_object_t *desc_obj = ecma_op_from_property_descriptor (prop_desc_p);

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t prop_value = ecma_make_prop_name_value (prop_name_p);
  ecma_value_t desc_obj_value = ecma_make_object_value (desc_obj);
  ecma_value_t args[] = { target, prop_value, desc_obj_value };

  /* 10. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, args, 3);

  ecma_deref_object (func_obj_p);
  ecma_deref_object (desc_obj);

  /* 11. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  bool boolean_trap_result = ecma_op_to_boolean (trap_result);

  ecma_free_value (trap_result);

  /* 12. */
  if (!boolean_trap_result)
  {
    return ECMA_VALUE_FALSE;
  }

  if (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION)
  {
    return ECMA_VALUE_TRUE;
  }

  /* 13. */
  ecma_property_descriptor_t target_desc;

  ecma_value_t status = ecma_op_object_get_own_property_descriptor (target_obj_p, prop_name_p, &target_desc);

  /* 14. */
  if (ECMA_IS_VALUE_ERROR (status))
  {
    return status;
  }

  bool target_prop_found = ecma_is_value_true (status);

  /* 15. */
  ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

  bool is_target_ext = ecma_is_value_true (extensible_target);

  /* 16. */
  if (ECMA_IS_VALUE_ERROR (extensible_target))
  {
    if (target_prop_found)
    {
      ecma_free_property_descriptor (&target_desc);
    }

    return extensible_target;
  }

  /* 17. */
  bool setting_config_false =
    ((prop_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE_DEFINED) && !(prop_desc_p->flags & JERRY_PROP_IS_CONFIGURABLE));

  /* 19. */
  if (!target_prop_found)
  {
    if (!is_target_ext)
    {
      return ecma_raise_type_error (ECMA_ERR_TRAP_TRUISH_ADDING_PROPERTY_NON_EXTENSIBLE_TARGET);
    }

    if (setting_config_false)
    {
      return ecma_raise_type_error (ECMA_ERR_TRAP_TRUISH_DEFINING_NON_EXISTENT_PROPERTY);
    }
  }
  /* 20. */
  else
  {
    ecma_value_t ret_value = ECMA_VALUE_EMPTY;

    if (!ecma_op_is_compatible_property_descriptor (prop_desc_p, &target_desc, is_target_ext))
    {
      ret_value = ecma_raise_type_error (ECMA_ERR_TRAP_TRUISH_ADD_PROPERTY_INCOMPATIBLE_OTHER_PROP);
    }
    else if (setting_config_false && (target_desc.flags & JERRY_PROP_IS_CONFIGURABLE))
    {
      ret_value = ecma_raise_type_error (ECMA_ERR_TRAP_TRUISH_DEFINING_NON_EXISTENT_PROPERTY);
    }
    /* ES11: 16.c */
    else if ((target_desc.flags & (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_WRITABLE_DEFINED)) != 0
             && (prop_desc_p->flags & (JERRY_PROP_IS_WRITABLE_DEFINED | JERRY_PROP_IS_WRITABLE))
                  == JERRY_PROP_IS_WRITABLE_DEFINED
             && (target_desc.flags & (JERRY_PROP_IS_WRITABLE | JERRY_PROP_IS_CONFIGURABLE)) == JERRY_PROP_IS_WRITABLE)

    {
      ret_value = ecma_raise_type_error (ECMA_ERR_TRAP_TRUISH_DEFINING_NON_EXISTENT_PROPERTY);
    }

    ecma_free_property_descriptor (&target_desc);

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      return ret_value;
    }
  }

  return ECMA_VALUE_TRUE;
} /* ecma_proxy_object_define_own_property */

/**
 * The Proxy object [[HasProperty]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.7
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE_FALSE} - depends on whether the property is found
 */
ecma_value_t
ecma_proxy_object_has (ecma_object_t *obj_p, /**< proxy object */
                       ecma_string_t *prop_name_p) /**< property name */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 2. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 3-6. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_HAS);

  /* 7. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 8. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t result = ecma_op_object_has_property (target_obj_p, prop_name_p);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t prop_value = ecma_make_prop_name_value (prop_name_p);
  ecma_value_t args[] = { target, prop_value };

  /* 9. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, args, 2);

  ecma_deref_object (func_obj_p);

  /* 10. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  bool boolean_trap_result = ecma_op_to_boolean (trap_result);

  ecma_free_value (trap_result);

  /* 11. */
  if (!boolean_trap_result && !(obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION))
  {
    ecma_property_descriptor_t target_desc;

    ecma_value_t status = ecma_op_object_get_own_property_descriptor (target_obj_p, prop_name_p, &target_desc);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      return status;
    }

    if (ecma_is_value_true (status))
    {
      bool prop_is_configurable = target_desc.flags & JERRY_PROP_IS_CONFIGURABLE;

      ecma_free_property_descriptor (&target_desc);

      if (!prop_is_configurable)
      {
        return ecma_raise_type_error (ECMA_ERR_TRAP_FALSISH_PROPERTY_NON_CONFIGURABLE);
      }

      ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

      if (ECMA_IS_VALUE_ERROR (extensible_target))
      {
        return extensible_target;
      }

      if (ecma_is_value_false (extensible_target))
      {
        return ecma_raise_type_error (ECMA_ERR_TRAP_FALSISH_PROPERTY_TARGET_NOT_EXTENSIBLE);
      }
    }
  }

  /* 12. */
  return ecma_make_boolean_value (boolean_trap_result);
} /* ecma_proxy_object_has */

/**
 * The Proxy object [[Get]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.8
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         value of the given named data property or the result of the getter function call - otherwise
 */
ecma_value_t
ecma_proxy_object_get (ecma_object_t *obj_p, /**< proxy object */
                       ecma_string_t *prop_name_p, /**< property name */
                       ecma_value_t receiver) /**< receiver to invoke getter function */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 2. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 3-6. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_GET);

  /* 7. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  /* 8. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_object_t *target_obj_p = ecma_get_object_from_value (proxy_obj_p->target);
    ecma_value_t result = ecma_op_object_get_with_receiver (target_obj_p, prop_name_p, receiver);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t prop_value = ecma_make_prop_name_value (prop_name_p);
  ecma_value_t args[] = { proxy_obj_p->target, prop_value, receiver };

  /* 9. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, args, 3);

  ecma_deref_object (func_obj_p);

  /* 10. */
  if (ECMA_IS_VALUE_ERROR (trap_result) || (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION))
  {
    return trap_result;
  }

  /* 11. */
  ecma_property_descriptor_t target_desc;
  ecma_value_t status = ecma_op_get_own_property_descriptor (proxy_obj_p->target, prop_name_p, &target_desc);

  /* 12. */
  if (ECMA_IS_VALUE_ERROR (status))
  {
    ecma_free_value (trap_result);
    return status;
  }

  /* 13. */
  if (ecma_is_value_true (status))
  {
    ecma_value_t ret_value = ECMA_VALUE_EMPTY;

    if ((target_desc.flags & JERRY_PROP_IS_VALUE_DEFINED) && !(target_desc.flags & JERRY_PROP_IS_CONFIGURABLE)
        && !(target_desc.flags & JERRY_PROP_IS_WRITABLE) && !ecma_op_same_value (trap_result, target_desc.value))
    {
      ret_value = ecma_raise_type_error (ECMA_ERR_INCORRECT_RETURN_PROXY_GET_TRAP);
    }
    else if (!(target_desc.flags & JERRY_PROP_IS_CONFIGURABLE)
             && (target_desc.flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED))
             && target_desc.get_p == NULL && !ecma_is_value_undefined (trap_result))
    {
      ret_value = ecma_raise_type_error (ECMA_ERR_PROXY_PROPERTY_NOT_CONFIGURABLE_NOT_HAVE_GETTER);
    }

    ecma_free_property_descriptor (&target_desc);

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      ecma_free_value (trap_result);

      return ret_value;
    }
  }

  /* 14. */
  return trap_result;
} /* ecma_proxy_object_get */

/**
 * The Proxy object [[Set]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.9
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE/FALSE} - depends on whether the property can be set to the given object
 */
ecma_value_t
ecma_proxy_object_set (ecma_object_t *obj_p, /**< proxy object */
                       ecma_string_t *prop_name_p, /**< property name */
                       ecma_value_t value, /**< value to set */
                       ecma_value_t receiver, /**< receiver to invoke setter function */
                       bool is_strict) /**< indicate strict mode */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 2. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 3-6. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_SET);

  /* 7. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 8. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t result = ecma_op_object_put_with_receiver (target_obj_p, prop_name_p, value, receiver, is_strict);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t prop_name_value = ecma_make_prop_name_value (prop_name_p);
  ecma_value_t args[] = { target, prop_name_value, value, receiver };

  /* 9. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, args, 4);

  ecma_deref_object (func_obj_p);

  /* 10. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  bool boolean_trap_result = ecma_op_to_boolean (trap_result);

  ecma_free_value (trap_result);

  /* 11. */
  if (!boolean_trap_result)
  {
    if (is_strict)
    {
      return ecma_raise_type_error (ECMA_ERR_PROXY_TRAP_RETURNED_FALSISH);
    }

    return ECMA_VALUE_FALSE;
  }

  if (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION)
  {
    return ECMA_VALUE_TRUE;
  }

  /* 12. */
  ecma_property_descriptor_t target_desc;

  ecma_value_t status = ecma_op_object_get_own_property_descriptor (target_obj_p, prop_name_p, &target_desc);

  /* 13. */
  if (ECMA_IS_VALUE_ERROR (status))
  {
    return status;
  }

  /* 14. */
  if (ecma_is_value_true (status))
  {
    ecma_value_t ret_value = ECMA_VALUE_EMPTY;

    if ((target_desc.flags & JERRY_PROP_IS_VALUE_DEFINED) && !(target_desc.flags & JERRY_PROP_IS_CONFIGURABLE)
        && !(target_desc.flags & JERRY_PROP_IS_WRITABLE) && !ecma_op_same_value (value, target_desc.value))
    {
      ret_value = ecma_raise_type_error (ECMA_ERR_INCORRECT_RETURN_PROXY_SET_TRAP);
    }
    else if (!(target_desc.flags & JERRY_PROP_IS_CONFIGURABLE)
             && (target_desc.flags & (JERRY_PROP_IS_GET_DEFINED | JERRY_PROP_IS_SET_DEFINED))
             && target_desc.set_p == NULL)
    {
      ret_value = ecma_raise_type_error (ECMA_ERR_TARGET_PROPERTY_CONFIGURE_ACCESSOR_WITHOUT_SETTER);
    }

    ecma_free_property_descriptor (&target_desc);

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      return ret_value;
    }
  }

  /* 15. */
  return ECMA_VALUE_TRUE;
} /* ecma_proxy_object_set */

/**
 * The Proxy object [[Delete]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.10
 *
 * Note: Returned value is always a simple value so freeing it is unnecessary.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_{TRUE/FALSE} - depends on whether the property can be deleted
 */
ecma_value_t
ecma_proxy_object_delete_property (ecma_object_t *obj_p, /**< proxy object */
                                   ecma_string_t *prop_name_p, /**< property name */
                                   bool is_strict) /**< delete in strict mode? */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 2. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 3-6.*/
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_DELETE_PROPERTY_UL);

  /* 7. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 8. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_value_t result = ecma_op_object_delete (target_obj_p, prop_name_p, is_strict);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t prop_name_value = ecma_make_prop_name_value (prop_name_p);
  ecma_value_t args[] = { target, prop_name_value };

  /* 9. */
  ecma_value_t trap_result = ecma_op_function_call (func_obj_p, handler, args, 2);

  ecma_deref_object (func_obj_p);

  /* 10. */
  if (ECMA_IS_VALUE_ERROR (trap_result))
  {
    return trap_result;
  }

  bool boolean_trap_result = ecma_op_to_boolean (trap_result);

  ecma_free_value (trap_result);

  /* 11. */
  if (!boolean_trap_result)
  {
    return ECMA_VALUE_FALSE;
  }

  if (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION)
  {
    return ECMA_VALUE_TRUE;
  }

  /* 12. */
  ecma_property_descriptor_t target_desc;

  ecma_value_t status = ecma_op_object_get_own_property_descriptor (target_obj_p, prop_name_p, &target_desc);

  /* 13. */
  if (ECMA_IS_VALUE_ERROR (status))
  {
    return status;
  }

  /* 14. */
  if (ecma_is_value_false (status))
  {
    return ECMA_VALUE_TRUE;
  }

  ecma_value_t ret_value = ECMA_VALUE_TRUE;

  /* 15. */
  if (!(target_desc.flags & JERRY_PROP_IS_CONFIGURABLE))
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_TRAP_TRUISH_PROPERTY_NON_CONFIGURABLE);
  }
  /* ES11: 13-14 */
  ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

  if (!ecma_is_value_true (extensible_target))
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_TRAP_TRUISH_TARGET_NOT_EXTENSIBLE);
  }

  ecma_free_property_descriptor (&target_desc);

  /* 16. */
  return ret_value;
} /* ecma_proxy_object_delete_property */

/**
 * Helper method for the Proxy object [[OwnPropertyKeys]] operation
 *
 * See also:
 *          ECMAScript v6, 9.5.12 steps 21. 23.
 *
 * @return ECMA_VALUE_ERROR - if a target key is not in the unchecked_result_keys collection
 *         ECMA_VALUE_EMPTY - otherwise
 */
static ecma_value_t
ecma_proxy_object_own_property_keys_helper (ecma_collection_t *target_collection, /**< target keys */
                                            ecma_collection_t *unchecked_result_keys, /**< unchecked keys */
                                            uint32_t *counter) /**< unchecked property counter */
{
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;

  for (uint32_t i = 0; i < target_collection->item_count; i++)
  {
    ecma_string_t *current_prop_name = ecma_get_prop_name_from_value (target_collection->buffer_p[i]);

    ret_value = ECMA_VALUE_ERROR;

    for (uint32_t j = 0; j < unchecked_result_keys->item_count; j++)
    {
      if (ecma_is_value_empty (unchecked_result_keys->buffer_p[j]))
      {
        continue;
      }

      ecma_string_t *unchecked_prop_name = ecma_get_prop_name_from_value (unchecked_result_keys->buffer_p[j]);

      if (ecma_compare_ecma_strings (current_prop_name, unchecked_prop_name))
      {
        ecma_deref_ecma_string (unchecked_prop_name);
        ret_value = ECMA_VALUE_EMPTY;
        unchecked_result_keys->buffer_p[j] = ECMA_VALUE_EMPTY;
        (*counter)++;
      }
    }

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      break;
    }
  }

  return ret_value;
} /* ecma_proxy_object_own_property_keys_helper */

/**
 * Helper method for checking the invariants in the Proxy object [[OwnPropertyKeys]] operation
 *
 * See also:
 *          ECMAScript v6, 9.5.12 steps 20-25.
 *
 * @return true - if none of the invariants got violated
 *         false - otherwise
 */
static bool
ecma_proxy_check_invariants_for_own_prop_keys (ecma_collection_t *trap_result,
                                               ecma_collection_t *target_non_configurable_keys,
                                               ecma_collection_t *target_configurable_keys,
                                               ecma_value_t extensible_target)
{
  /* 20. */
  ecma_collection_t *unchecked_result_keys = ecma_new_collection ();

  ecma_collection_append (unchecked_result_keys, trap_result->buffer_p, trap_result->item_count);

  for (uint32_t i = 0; i < unchecked_result_keys->item_count; i++)
  {
    ecma_string_t *unchecked_prop_name = ecma_get_prop_name_from_value (unchecked_result_keys->buffer_p[i]);
    ecma_ref_ecma_string (unchecked_prop_name);
  }

  bool check_ok = false;
  uint32_t unchecked_prop_name_counter = 0;

  /* 21. */
  if (ECMA_IS_VALUE_ERROR (ecma_proxy_object_own_property_keys_helper (target_non_configurable_keys,
                                                                       unchecked_result_keys,
                                                                       &unchecked_prop_name_counter)))
  {
    ecma_raise_type_error (ECMA_ERR_TRAP_RESULT_NOT_INCLUDE_ALL_NON_CONFIGURABLE_KEYS);
  }
  /* 22. */
  else if (ecma_is_value_true (extensible_target))
  {
    check_ok = true;
  }
  /* 23. */
  else if (ECMA_IS_VALUE_ERROR (ecma_proxy_object_own_property_keys_helper (target_configurable_keys,
                                                                            unchecked_result_keys,
                                                                            &unchecked_prop_name_counter)))
  {
    ecma_raise_type_error (ECMA_ERR_TRAP_RESULT_NOT_INCLUDE_ALL_CONFIGURABLE_KEYS);
  }
  /* 24. */
  else if (unchecked_result_keys->item_count != unchecked_prop_name_counter)
  {
    ecma_raise_type_error (ECMA_ERR_TRAP_EXTRA_KEYS_FOR_A_NON_EXTENSIBLE_TARGET);
  }
  /* 25. */
  else
  {
    check_ok = true;
  }

  ecma_collection_free (unchecked_result_keys);

  return check_ok;
} /* ecma_proxy_check_invariants_for_own_prop_keys */

/**
 * The Proxy object [[OwnPropertyKeys]] internal routine
 *
 * See also:
 *          ECMAScript v11, 9.5.11
 *
 * Note: If the returned collection is not NULL, it must be freed with
 *       ecma_collection_free if it is no longer needed
 *
 * @return NULL - if the operation fails
 *         pointer to a newly allocated list of property names - otherwise
 */
ecma_collection_t *
ecma_proxy_object_own_property_keys (ecma_object_t *obj_p) /**< proxy object */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE_RETURN (NULL);

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 1. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 2-5. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_OWN_KEYS_UL);

  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return NULL;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 6. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_collection_t *result = ecma_op_object_own_property_keys (target_obj_p, JERRY_PROPERTY_FILTER_ALL);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);

  /* 7. */
  ecma_value_t trap_result_array = ecma_op_function_call (func_obj_p, handler, &target, 1);

  ecma_deref_object (func_obj_p);

  if (ECMA_IS_VALUE_ERROR (trap_result_array))
  {
    return NULL;
  }

  /* 8. */
  ecma_collection_t *trap_result = ecma_op_create_list_from_array_like (trap_result_array, true);

  ecma_free_value (trap_result_array);

  if (trap_result == NULL || (obj_p->u2.prototype_cp & JERRY_PROXY_SKIP_RESULT_VALIDATION))
  {
    return trap_result;
  }

  /* 9. */
  if (ecma_collection_check_duplicated_entries (trap_result))
  {
    ecma_collection_free (trap_result);
    ecma_raise_type_error (ECMA_ERR_TRAP_WITH_DUPLICATED_ENTRIES);
    return NULL;
  }

  /* 10. */
  ecma_value_t extensible_target = ecma_builtin_object_object_is_extensible (target_obj_p);

  if (ECMA_IS_VALUE_ERROR (extensible_target))
  {
    ecma_collection_free (trap_result);
    return NULL;
  }

  /* 11. */
  ecma_collection_t *target_keys = ecma_op_object_own_property_keys (target_obj_p, JERRY_PROPERTY_FILTER_ALL);

  if (target_keys == NULL)
  {
    ecma_collection_free (trap_result);
    return target_keys;
  }

  /* 14. */
  ecma_collection_t *target_configurable_keys = ecma_new_collection ();

  /* 15. */
  ecma_collection_t *target_non_configurable_keys = ecma_new_collection ();

  ecma_collection_t *ret_value = NULL;

  /* 16. */
  for (uint32_t i = 0; i < target_keys->item_count; i++)
  {
    ecma_property_descriptor_t target_desc;

    ecma_string_t *prop_name_p = ecma_get_prop_name_from_value (target_keys->buffer_p[i]);

    ecma_value_t status = ecma_op_object_get_own_property_descriptor (target_obj_p, prop_name_p, &target_desc);

    if (ECMA_IS_VALUE_ERROR (status))
    {
      ecma_collection_free (trap_result);
      goto free_target_collections;
    }

    ecma_value_t prop_value = ecma_make_prop_name_value (prop_name_p);

    if (ecma_is_value_true (status) && !(target_desc.flags & JERRY_PROP_IS_CONFIGURABLE))
    {
      ecma_collection_push_back (target_non_configurable_keys, prop_value);
    }
    else
    {
      ecma_collection_push_back (target_configurable_keys, prop_value);
    }

    if (ecma_is_value_true (status))
    {
      ecma_free_property_descriptor (&target_desc);
    }
  }

  /* 17. */
  if (ecma_is_value_true (extensible_target) && target_non_configurable_keys->item_count == 0)
  {
    ret_value = trap_result;
  }
  /* 18-22. */
  else if (ecma_proxy_check_invariants_for_own_prop_keys (trap_result,
                                                          target_non_configurable_keys,
                                                          target_configurable_keys,
                                                          extensible_target))
  {
    ret_value = trap_result;
  }
  else
  {
    JERRY_ASSERT (ret_value == NULL);
    ecma_collection_free (trap_result);
  }

free_target_collections:
  ecma_collection_destroy (target_keys);
  ecma_collection_free (target_configurable_keys);
  ecma_collection_free (target_non_configurable_keys);

  /* 23. */
  return ret_value;
} /* ecma_proxy_object_own_property_keys */

/**
 * The Proxy object [[Call]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.13
 *
 * Note: Returned value must be freed with ecma_free_value.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         result of the function call - otherwise
 */
ecma_value_t
ecma_proxy_object_call (ecma_object_t *obj_p, /**< proxy object */
                        ecma_value_t this_argument, /**< this argument to invoke the function */
                        const ecma_value_t *args_p, /**< argument list */
                        uint32_t argc) /**< number of arguments */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));

  if (!ecma_op_proxy_object_is_callable (obj_p))
  {
    return ecma_raise_type_error (ECMA_ERR_EXPECTED_A_FUNCTION);
  }

  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 1. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 2-5.*/
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_APPLY);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;

  /* 7. */
  if (ecma_is_value_undefined (trap))
  {
    ecma_object_t *target_obj_p = ecma_get_object_from_value (target);
    ecma_value_t result = ecma_op_function_call (target_obj_p, this_argument, args_p, argc);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  /* 8. */
  ecma_value_t args_array = ecma_op_new_array_object_from_buffer (args_p, argc);
  ecma_value_t value_array[] = { target, this_argument, args_array };
  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  /* 9. */
  ecma_value_t ret_value = ecma_op_function_call (func_obj_p, handler, value_array, 3);
  ecma_deref_object (func_obj_p);
  ecma_free_object (args_array);

  return ret_value;
} /* ecma_proxy_object_call */

/**
 * The Proxy object [[Construct]] internal routine
 *
 * See also:
 *          ECMAScript v6, 9.5.14
 *
 * Note: Returned value must be freed with ecma_free_value.
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         result of the construct call - otherwise
 */
ecma_value_t
ecma_proxy_object_construct (ecma_object_t *obj_p, /**< proxy object */
                             ecma_object_t *new_target_p, /**< new target */
                             const ecma_value_t *args_p, /**< argument list */
                             uint32_t argc) /**< number of arguments */
{
  JERRY_ASSERT (ECMA_OBJECT_IS_PROXY (obj_p));
  ECMA_CHECK_STACK_USAGE ();

  ecma_proxy_object_t *proxy_obj_p = (ecma_proxy_object_t *) obj_p;

  /* 1. */
  ecma_value_t handler = proxy_obj_p->handler;

  /* 2-5. */
  ecma_value_t trap = ecma_validate_proxy_object (handler, LIT_MAGIC_STRING_CONSTRUCT);

  /* 6. */
  if (ECMA_IS_VALUE_ERROR (trap))
  {
    return trap;
  }

  ecma_value_t target = proxy_obj_p->target;
  ecma_object_t *target_obj_p = ecma_get_object_from_value (target);

  /* 7. */
  if (ecma_is_value_undefined (trap))
  {
    JERRY_ASSERT (ecma_object_is_constructor (target_obj_p));

    ecma_value_t result = ecma_op_function_construct (target_obj_p, new_target_p, args_p, argc);
    JERRY_BLOCK_TAIL_CALL_OPTIMIZATION ();
    return result;
  }

  /* 8. */
  ecma_value_t args_array = ecma_op_new_array_object_from_buffer (args_p, argc);

  ecma_object_t *func_obj_p = ecma_get_object_from_value (trap);
  ecma_value_t new_target_value = ecma_make_object_value (new_target_p);
  ecma_value_t function_call_args[] = { target, args_array, new_target_value };

  /* 9. */
  ecma_value_t new_obj = ecma_op_function_call (func_obj_p, handler, function_call_args, 3);

  ecma_free_object (args_array);
  ecma_deref_object (func_obj_p);

  /* 10 .*/
  if (ECMA_IS_VALUE_ERROR (new_obj))
  {
    return new_obj;
  }

  /* 11. */
  if (!ecma_is_value_object (new_obj))
  {
    ecma_free_value (new_obj);

    return ecma_raise_type_error (ECMA_ERR_TRAP_MUST_RETURN_WITH_AN_OBJECT);
  }

  /* 12. */
  return new_obj;
} /* ecma_proxy_object_construct */

#endif /* JERRY_BUILTIN_PROXY */

/**
 * @}
 * @}
 */
