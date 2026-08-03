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

#include "vm.h"

#include "ecma-alloc.h"
#include "ecma-arguments-object.h"
#include "ecma-array-object.h"
#include "ecma-bigint.h"
#include "ecma-builtin-object.h"
#include "ecma-builtins.h"
#include "ecma-comparison.h"
#include "ecma-conversion.h"
#include "ecma-errors.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-helpers.h"
#include "ecma-iterator-object.h"
#include "ecma-lcache.h"
#include "ecma-lex-env.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"
#include "ecma-promise-object.h"
#include "ecma-regexp-object.h"

#include "common.h"
#include "jcontext.h"
#include "opcodes.h"
#include "vm-stack.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup vm_executor Executor
 * @{
 */

JERRY_STATIC_ASSERT ((sizeof (vm_frame_ctx_t) % sizeof (ecma_value_t)) == 0,
                     sizeof_vm_frame_ctx_must_be_sizeof_ecma_value_t_aligned);

/**
 * Get the value of object[property].
 *
 * @return ecma value
 */
static ecma_value_t
vm_op_get_value (ecma_value_t object, /**< base object */
                 ecma_value_t property) /**< property name */
{
  if (ecma_is_value_object (object))
  {
    ecma_object_t *object_p = ecma_get_object_from_value (object);
    ecma_string_t *property_name_p = NULL;

    if (ecma_is_value_integer_number (property))
    {
      ecma_integer_value_t int_value = ecma_get_integer_from_value (property);

      if (int_value >= 0 && int_value <= ECMA_DIRECT_STRING_MAX_IMM)
      {
        if (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_ARRAY)
        {
          ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

          if (JERRY_LIKELY (ecma_op_array_is_fast_array (ext_object_p)
                            && (uint32_t) int_value < ext_object_p->u.array.length))
          {
            ecma_value_t *values_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, object_p->u1.property_list_cp);

            if (JERRY_LIKELY (!ecma_is_value_array_hole (values_p[int_value])))
            {
              return ecma_fast_copy_value (values_p[int_value]);
            }
          }
        }

        property_name_p = (ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_UINT, (uintptr_t) int_value);
      }
    }
    else if (ecma_is_value_string (property))
    {
      property_name_p = ecma_get_string_from_value (property);
    }

    if (ecma_is_value_symbol (property))
    {
      property_name_p = ecma_get_symbol_from_value (property);
    }

    if (property_name_p != NULL)
    {
#if JERRY_LCACHE
      ecma_property_t *property_p = ecma_lcache_lookup (object_p, property_name_p);

      if (property_p != NULL && (*property_p & ECMA_PROPERTY_FLAG_DATA))
      {
        JERRY_ASSERT (!ECMA_PROPERTY_IS_INTERNAL (*property_p));
        return ecma_fast_copy_value (ECMA_PROPERTY_VALUE_PTR (property_p)->value);
      }
#endif /* JERRY_LCACHE */

      /* There is no need to free the name. */
      return ecma_op_object_get (object_p, property_name_p);
    }
  }

  if (JERRY_UNLIKELY (ecma_is_value_undefined (object) || ecma_is_value_null (object)))
  {
#if JERRY_ERROR_MESSAGES
    ecma_value_t error_value =
      ecma_raise_standard_error_with_format (JERRY_ERROR_TYPE, "Cannot read property '%' of %", property, object);
#else /* !JERRY_ERROR_MESSAGES */
    ecma_value_t error_value = ecma_raise_type_error (ECMA_ERR_EMPTY);
#endif /* JERRY_ERROR_MESSAGES */
    return error_value;
  }

  ecma_string_t *property_name_p = ecma_op_to_property_key (property);

  if (property_name_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t get_value_result = ecma_op_get_value_object_base (object, property_name_p);

  ecma_deref_ecma_string (property_name_p);
  return get_value_result;
} /* vm_op_get_value */

/**
 * Set the value of object[property].
 *
 * Note:
 *  this function frees its object and property arguments
 *
 * @return an ecma value which contains an error
 *         if the property setting is unsuccessful
 */
static ecma_value_t
vm_op_set_value (ecma_value_t base, /**< base object */
                 ecma_value_t property, /**< property name */
                 ecma_value_t value, /**< ecma value */
                 bool is_strict) /**< strict mode */
{
  ecma_value_t result = ECMA_VALUE_EMPTY;
  ecma_object_t *object_p;
  ecma_string_t *property_p;

  if (JERRY_UNLIKELY (!ecma_is_value_object (base)))
  {
    if (JERRY_UNLIKELY (ecma_is_value_null (base) || ecma_is_value_undefined (base)))
    {
#if JERRY_ERROR_MESSAGES
      result = ecma_raise_standard_error_with_format (JERRY_ERROR_TYPE, "Cannot set property '%' of %", property, base);
#else /* !JERRY_ERROR_MESSAGES */
      result = ecma_raise_type_error (ECMA_ERR_EMPTY);
#endif /* JERRY_ERROR_MESSAGES */
      ecma_free_value (property);
      return result;
    }

    if (JERRY_UNLIKELY (!ecma_is_value_prop_name (property)))
    {
      property_p = ecma_op_to_string (property);
      ecma_fast_free_value (property);

      if (JERRY_UNLIKELY (property_p == NULL))
      {
        ecma_free_value (base);
        return ECMA_VALUE_ERROR;
      }
    }
    else
    {
      property_p = ecma_get_prop_name_from_value (property);
    }

    ecma_value_t object = ecma_op_to_object (base);
    JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (object));

    object_p = ecma_get_object_from_value (object);
    ecma_op_ordinary_object_prevent_extensions (object_p);

    result = ecma_op_object_put_with_receiver (object_p, property_p, value, base, is_strict);

    ecma_free_value (base);
  }
  else
  {
    object_p = ecma_get_object_from_value (base);

    if (JERRY_UNLIKELY (!ecma_is_value_prop_name (property)))
    {
      property_p = ecma_op_to_string (property);
      ecma_fast_free_value (property);

      if (JERRY_UNLIKELY (property_p == NULL))
      {
        ecma_deref_object (object_p);
        return ECMA_VALUE_ERROR;
      }
    }
    else
    {
      property_p = ecma_get_prop_name_from_value (property);
    }

    if (!ecma_is_lexical_environment (object_p))
    {
      result = ecma_op_object_put_with_receiver (object_p, property_p, value, base, is_strict);
    }
    else
    {
      result = ecma_op_set_mutable_binding (object_p, property_p, value, is_strict);
    }
  }

  ecma_deref_object (object_p);
  ecma_deref_ecma_string (property_p);
  return result;
} /* vm_op_set_value */

/** Compact bytecode define */
#define CBC_OPCODE(arg1, arg2, arg3, arg4) arg4,

/**
 * Decode table for both opcodes and extended opcodes.
 */
static const uint16_t vm_decode_table[] JERRY_ATTR_CONST_DATA = { CBC_OPCODE_LIST CBC_EXT_OPCODE_LIST };

#undef CBC_OPCODE

/**
 * Run global code
 *
 * Note:
 *      returned value must be freed with ecma_free_value, when it is no longer needed.
 *
 * @return ecma value
 */
ecma_value_t
vm_run_global (const ecma_compiled_code_t *bytecode_p, /**< pointer to bytecode to run */
               ecma_object_t *function_object_p) /**< function object if available */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
#if JERRY_BUILTIN_REALMS
  ecma_object_t *global_obj_p = (ecma_object_t *) ecma_op_function_get_realm (bytecode_p);
#else /* !JERRY_BUILTIN_REALMS */
  ecma_object_t *global_obj_p = ecma_builtin_get_global ();
#endif /* JERRY_BUILTIN_REALMS */

  if (bytecode_p->status_flags & CBC_CODE_FLAGS_LEXICAL_BLOCK_NEEDED)
  {
    ecma_create_global_lexical_block (global_obj_p);
  }

  ecma_object_t *const global_scope_p = ecma_get_global_scope (global_obj_p);

  vm_frame_ctx_shared_t shared;
  shared.bytecode_header_p = bytecode_p;
  shared.function_object_p = function_object_p;
  shared.status_flags = 0;

#if JERRY_BUILTIN_REALMS
  ecma_value_t this_binding = ((ecma_global_object_t *) global_obj_p)->this_binding;

  ecma_global_object_t *saved_global_object_p = JERRY_CONTEXT (global_object_p);
  JERRY_CONTEXT (global_object_p) = (ecma_global_object_t *) global_obj_p;
#else /* !JERRY_BUILTIN_REALMS */
  ecma_value_t this_binding = ecma_make_object_value (global_obj_p);
#endif /* JERRY_BUILTIN_REALMS */

  ecma_value_t result = vm_run (&shared, this_binding, global_scope_p);

#if JERRY_BUILTIN_REALMS
  JERRY_CONTEXT (global_object_p) = saved_global_object_p;
#endif /* JERRY_BUILTIN_REALMS */

  return result;
} /* vm_run_global */

/**
 * Run specified eval-mode bytecode
 *
 * @return ecma value
 */
ecma_value_t
vm_run_eval (ecma_compiled_code_t *bytecode_data_p, /**< byte-code data */
             uint32_t parse_opts) /**< ecma_parse_opts_t option bits */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_value_t this_binding;
  ecma_object_t *lex_env_p;

  /* ECMA-262 v5, 10.4.2 */
  if (parse_opts & ECMA_PARSE_DIRECT_EVAL)
  {
    this_binding = ecma_copy_value (JERRY_CONTEXT (vm_top_context_p)->this_binding);
    lex_env_p = JERRY_CONTEXT (vm_top_context_p)->lex_env_p;
  }
  else
  {
#if JERRY_BUILTIN_REALMS
    ecma_object_t *global_obj_p = (ecma_object_t *) ecma_op_function_get_realm (bytecode_data_p);
    this_binding = ((ecma_global_object_t *) global_obj_p)->this_binding;
    ecma_ref_object (ecma_get_object_from_value (this_binding));
#else /* !JERRY_BUILTIN_REALMS */
    ecma_object_t *global_obj_p = ecma_builtin_get_global ();
    ecma_ref_object (global_obj_p);
    this_binding = ecma_make_object_value (global_obj_p);
#endif /* JERRY_BUILTIN_REALMS */
    lex_env_p = ecma_get_global_scope (global_obj_p);
  }

  ecma_ref_object (lex_env_p);

  if ((bytecode_data_p->status_flags & CBC_CODE_FLAGS_STRICT_MODE) != 0)
  {
    ecma_object_t *strict_lex_env_p = ecma_create_decl_lex_env (lex_env_p);

    ecma_deref_object (lex_env_p);
    lex_env_p = strict_lex_env_p;
  }

  if ((bytecode_data_p->status_flags & CBC_CODE_FLAGS_LEXICAL_BLOCK_NEEDED) != 0)
  {
    ecma_object_t *lex_block_p = ecma_create_decl_lex_env (lex_env_p);
    lex_block_p->type_flags_refs |= ECMA_OBJECT_FLAG_BLOCK;

    ecma_deref_object (lex_env_p);
    lex_env_p = lex_block_p;
  }

  vm_frame_ctx_shared_t shared;
  shared.bytecode_header_p = bytecode_data_p;
  shared.function_object_p = NULL;
  shared.status_flags = (parse_opts & ECMA_PARSE_DIRECT_EVAL) ? VM_FRAME_CTX_SHARED_DIRECT_EVAL : 0;

  ecma_value_t completion_value = vm_run (&shared, this_binding, lex_env_p);

  ecma_deref_object (lex_env_p);
  ecma_free_value (this_binding);

#if JERRY_SNAPSHOT_EXEC
  if (!(bytecode_data_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION))
  {
    ecma_bytecode_deref (bytecode_data_p);
  }
#else /* !JERRY_SNAPSHOT_EXEC */
  ecma_bytecode_deref (bytecode_data_p);
#endif /* JERRY_SNAPSHOT_EXEC */

  return completion_value;
} /* vm_run_eval */

#if JERRY_MODULE_SYSTEM

/**
 * Run module code
 *
 * Note:
 *      returned value must be freed with ecma_free_value, when it is no longer needed.
 *
 * @return ecma value
 */
ecma_value_t
vm_run_module (ecma_module_t *module_p) /**< module to be executed */
{
  const ecma_value_t module_init_result = ecma_module_initialize (module_p);

  if (ECMA_IS_VALUE_ERROR (module_init_result))
  {
    return module_init_result;
  }

  vm_frame_ctx_shared_t shared;
  shared.bytecode_header_p = module_p->u.compiled_code_p;
  shared.function_object_p = &module_p->header.object;
  shared.status_flags = 0;

  return vm_run (&shared, ECMA_VALUE_UNDEFINED, module_p->scope_p);
} /* vm_run_module */

#endif /* JERRY_MODULE_SYSTEM */

/**
 * Construct object
 *
 * @return object value
 */
static ecma_value_t
vm_construct_literal_object (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                             ecma_value_t lit_value) /**< literal */
{
  ecma_compiled_code_t *bytecode_p;

#if JERRY_SNAPSHOT_EXEC
  if (JERRY_LIKELY (!(frame_ctx_p->shared_p->bytecode_header_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION)))
  {
#endif /* JERRY_SNAPSHOT_EXEC */
    bytecode_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, lit_value);
#if JERRY_SNAPSHOT_EXEC
  }
  else
  {
    uint8_t *byte_p = ((uint8_t *) frame_ctx_p->shared_p->bytecode_header_p) + lit_value;
    bytecode_p = (ecma_compiled_code_t *) byte_p;
  }
#endif /* JERRY_SNAPSHOT_EXEC */

#if JERRY_BUILTIN_REGEXP
  if (JERRY_UNLIKELY (!CBC_IS_FUNCTION (bytecode_p->status_flags)))
  {
    ecma_object_t *regexp_obj_p = ecma_op_regexp_alloc (NULL);

    if (JERRY_UNLIKELY (regexp_obj_p == NULL))
    {
      return ECMA_VALUE_ERROR;
    }

    return ecma_op_create_regexp_from_bytecode (regexp_obj_p, (re_compiled_code_t *) bytecode_p);
  }
#else /* !JERRY_BUILTIN_REGEXP */
  JERRY_ASSERT (CBC_IS_FUNCTION (bytecode_p->status_flags));
#endif /* JERRY_BUILTIN_REGEXP */

  ecma_object_t *func_obj_p;

  if (JERRY_UNLIKELY (CBC_FUNCTION_IS_ARROW (bytecode_p->status_flags)))
  {
    func_obj_p = ecma_op_create_arrow_function_object (frame_ctx_p->lex_env_p, bytecode_p, frame_ctx_p->this_binding);
  }
  else
  {
    func_obj_p = ecma_op_create_any_function_object (frame_ctx_p->lex_env_p, bytecode_p);
  }

  return ecma_make_object_value (func_obj_p);
} /* vm_construct_literal_object */

/**
 * Get implicit this value
 *
 * @return true - if the implicit 'this' value is updated,
 *         false - otherwise
 */
static inline bool
vm_get_implicit_this_value (ecma_value_t *this_value_p) /**< [in,out] this value */
{
  if (ecma_is_value_object (*this_value_p))
  {
    ecma_object_t *this_obj_p = ecma_get_object_from_value (*this_value_p);

    if (ecma_is_lexical_environment (this_obj_p))
    {
      ecma_value_t completion_value = ecma_op_implicit_this_value (this_obj_p);

      JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (completion_value));

      *this_value_p = completion_value;
      return true;
    }
  }
  return false;
} /* vm_get_implicit_this_value */

/**
 * Special bytecode sequence for error handling while the vm_loop
 * is preserved for an execute operation
 */
static const uint8_t vm_error_byte_code_p[] = { CBC_EXT_OPCODE, CBC_EXT_ERROR };

/**
 * Get class function object
 *
 * @return the pointer to the object
 */
static ecma_object_t *
vm_get_class_function (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  JERRY_ASSERT (frame_ctx_p != NULL);

  if (frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_NON_ARROW_FUNC)
  {
    return frame_ctx_p->shared_p->function_object_p;
  }

  ecma_environment_record_t *environment_record_p = ecma_op_get_environment_record (frame_ctx_p->lex_env_p);

  JERRY_ASSERT (environment_record_p != NULL);
  return ecma_get_object_from_value (environment_record_p->function_object);
} /* vm_get_class_function */

/**
 * 'super(...)' function call handler.
 */
static void
vm_super_call (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (frame_ctx_p->call_operation == VM_EXEC_SUPER_CALL);
  JERRY_ASSERT (frame_ctx_p->byte_code_p[0] == CBC_EXT_OPCODE);

  const uint8_t *byte_code_p = frame_ctx_p->byte_code_p + 3;
  uint8_t opcode = byte_code_p[-2];
  uint32_t arguments_list_len;

  bool spread_arguments = opcode >= CBC_EXT_SPREAD_SUPER_CALL;

  ecma_collection_t *collection_p = NULL;
  ecma_value_t *arguments_p;

  if (spread_arguments)
  {
    ecma_value_t collection = *(--frame_ctx_p->stack_top_p);
    collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, collection);
    arguments_p = collection_p->buffer_p;
    arguments_list_len = collection_p->item_count;
  }
  else
  {
    arguments_list_len = byte_code_p[-1];
    arguments_p = frame_ctx_p->stack_top_p;
  }

  ecma_value_t func_value = *(--frame_ctx_p->stack_top_p);
  ecma_value_t completion_value;

  ecma_environment_record_t *environment_record_p = ecma_op_get_environment_record (frame_ctx_p->lex_env_p);
  JERRY_ASSERT (environment_record_p);

  if (!ecma_is_constructor (func_value))
  {
    completion_value = ecma_raise_type_error (ECMA_ERR_VALUE_FOR_CLASS_HERITAGE_IS_NOT_A_CONSTRUCTOR);
  }
  else
  {
    ecma_object_t *func_obj_p = ecma_get_object_from_value (func_value);
    completion_value =
      ecma_op_function_construct (func_obj_p, JERRY_CONTEXT (current_new_target_p), arguments_p, arguments_list_len);

    if (!ECMA_IS_VALUE_ERROR (completion_value) && ecma_op_this_binding_is_initialized (environment_record_p))
    {
      ecma_free_value (completion_value);
      completion_value = ecma_raise_reference_error (ECMA_ERR_SUPER_CONSTRUCTOR_MAY_ONLY_BE_CALLED_ONCE);
    }
  }

  /* Free registers. */
  for (uint32_t i = 0; i < arguments_list_len; i++)
  {
    ecma_fast_free_value (arguments_p[i]);
  }

  if (collection_p != NULL)
  {
    ecma_collection_destroy (collection_p);
  }

  if (ecma_is_value_object (completion_value))
  {
    ecma_op_bind_this_value (environment_record_p, completion_value);
    frame_ctx_p->this_binding = completion_value;

    ecma_value_t fields_value = opfunc_init_class_fields (vm_get_class_function (frame_ctx_p), completion_value);

    if (ECMA_IS_VALUE_ERROR (fields_value))
    {
      ecma_free_value (completion_value);
      completion_value = ECMA_VALUE_ERROR;
    }
  }

  ecma_free_value (func_value);

  if (JERRY_UNLIKELY (ECMA_IS_VALUE_ERROR (completion_value)))
  {
    frame_ctx_p->byte_code_p = (uint8_t *) vm_error_byte_code_p;
  }
  else
  {
    frame_ctx_p->byte_code_p = byte_code_p;
    uint32_t opcode_data = vm_decode_table[(CBC_END + 1) + opcode];

    if (!(opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK)))
    {
      ecma_fast_free_value (completion_value);
    }
    else if (opcode_data & VM_OC_PUT_STACK)
    {
      *frame_ctx_p->stack_top_p++ = completion_value;
    }
    else
    {
      ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, 0));
      VM_GET_REGISTERS (frame_ctx_p)[0] = completion_value;
    }
  }
} /* vm_super_call */

/**
 * Perform one of the following call/construct operation with spread argument list
 *   - f(...args)
 *   - o.f(...args)
 *   - new O(...args)
 */
static void
vm_spread_operation (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  JERRY_ASSERT (frame_ctx_p->byte_code_p[0] == CBC_EXT_OPCODE);

  uint8_t opcode = frame_ctx_p->byte_code_p[1];
  ecma_value_t completion_value;
  ecma_value_t collection = *(--frame_ctx_p->stack_top_p);

  ecma_collection_t *collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, collection);
  ecma_value_t func_value = *(--frame_ctx_p->stack_top_p);
  bool is_call_prop = opcode >= CBC_EXT_SPREAD_CALL_PROP;

  if (frame_ctx_p->byte_code_p[1] == CBC_EXT_SPREAD_NEW)
  {
    ecma_error_msg_t constructor_message_id = ecma_check_constructor (func_value);
    if (constructor_message_id != ECMA_IS_VALID_CONSTRUCTOR)
    {
      completion_value = ecma_raise_type_error (constructor_message_id);
    }
    else
    {
      ecma_object_t *constructor_obj_p = ecma_get_object_from_value (func_value);

      completion_value = ecma_op_function_construct (constructor_obj_p,
                                                     constructor_obj_p,
                                                     collection_p->buffer_p,
                                                     collection_p->item_count);
    }
  }
  else
  {
    ecma_value_t this_value = is_call_prop ? frame_ctx_p->stack_top_p[-2] : ECMA_VALUE_UNDEFINED;

    if (!ecma_is_value_object (func_value) || !ecma_op_object_is_callable (ecma_get_object_from_value (func_value)))
    {
      completion_value = ecma_raise_type_error (ECMA_ERR_EXPECTED_A_FUNCTION);
    }
    else
    {
      ecma_object_t *func_obj_p = ecma_get_object_from_value (func_value);

      completion_value =
        ecma_op_function_call (func_obj_p, this_value, collection_p->buffer_p, collection_p->item_count);
    }

    if (is_call_prop)
    {
      ecma_free_value (*(--frame_ctx_p->stack_top_p));
      ecma_free_value (*(--frame_ctx_p->stack_top_p));
    }
  }

  ecma_collection_free (collection_p);
  ecma_free_value (func_value);

  if (JERRY_UNLIKELY (ECMA_IS_VALUE_ERROR (completion_value)))
  {
    frame_ctx_p->byte_code_p = (uint8_t *) vm_error_byte_code_p;
  }
  else
  {
    uint32_t opcode_data = vm_decode_table[(CBC_END + 1) + opcode];

    if (!(opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK)))
    {
      ecma_fast_free_value (completion_value);
    }
    else if (opcode_data & VM_OC_PUT_STACK)
    {
      *frame_ctx_p->stack_top_p++ = completion_value;
    }
    else
    {
      ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, 0));
      VM_GET_REGISTERS (frame_ctx_p)[0] = completion_value;
    }

    /* EXT_OPCODE, SPREAD_OPCODE, BYTE_ARG */
    frame_ctx_p->byte_code_p += 3;
  }
} /* vm_spread_operation */

/**
 * 'Function call' opcode handler.
 *
 * See also: ECMA-262 v5, 11.2.3
 */
static void
opfunc_call (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  const uint8_t *byte_code_p = frame_ctx_p->byte_code_p + 1;
  uint8_t opcode = byte_code_p[-1];
  uint32_t arguments_list_len;

  if (opcode >= CBC_CALL0)
  {
    arguments_list_len = (unsigned int) ((opcode - CBC_CALL0) / 6);
  }
  else
  {
    arguments_list_len = *byte_code_p++;
  }

  bool is_call_prop = ((opcode - CBC_CALL) % 6) >= 3;

  ecma_value_t *stack_top_p = frame_ctx_p->stack_top_p - arguments_list_len;
  ecma_value_t this_value = is_call_prop ? stack_top_p[-3] : ECMA_VALUE_UNDEFINED;
  ecma_value_t func_value = stack_top_p[-1];

  ecma_value_t completion_value =
    ecma_op_function_validated_call (func_value, this_value, stack_top_p, arguments_list_len);

  JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_DIRECT_EVAL;

  /* Free registers. */
  for (uint32_t i = 0; i < arguments_list_len; i++)
  {
    ecma_fast_free_value (stack_top_p[i]);
  }

  if (is_call_prop)
  {
    ecma_free_value (*(--stack_top_p));
    ecma_free_value (*(--stack_top_p));
  }

  if (JERRY_UNLIKELY (ECMA_IS_VALUE_ERROR (completion_value)))
  {
    frame_ctx_p->byte_code_p = (uint8_t *) vm_error_byte_code_p;
  }
  else
  {
    frame_ctx_p->byte_code_p = byte_code_p;
    ecma_free_value (*(--stack_top_p));
    uint32_t opcode_data = vm_decode_table[opcode];

    if (!(opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK)))
    {
      ecma_fast_free_value (completion_value);
    }
    else if (opcode_data & VM_OC_PUT_STACK)
    {
      *stack_top_p++ = completion_value;
    }
    else
    {
      ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, 0));
      VM_GET_REGISTERS (frame_ctx_p)[0] = completion_value;
    }
  }

  frame_ctx_p->stack_top_p = stack_top_p;
} /* opfunc_call */

/**
 * 'Constructor call' opcode handler.
 *
 * See also: ECMA-262 v5, 11.2.2
 */
static void
opfunc_construct (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  const uint8_t *byte_code_p = frame_ctx_p->byte_code_p + 1;
  uint8_t opcode = byte_code_p[-1];
  unsigned int arguments_list_len;

  if (opcode >= CBC_NEW0)
  {
    arguments_list_len = (unsigned int) (opcode - CBC_NEW0);
  }
  else
  {
    arguments_list_len = *byte_code_p++;
  }

  ecma_value_t *stack_top_p = frame_ctx_p->stack_top_p - arguments_list_len;
  ecma_value_t constructor_value = stack_top_p[-1];
  ecma_value_t completion_value;

  ecma_error_msg_t constructor_message_id = ecma_check_constructor (constructor_value);
  if (constructor_message_id != ECMA_IS_VALID_CONSTRUCTOR)
  {
    completion_value = ecma_raise_type_error (constructor_message_id);
  }
  else
  {
    ecma_object_t *constructor_obj_p = ecma_get_object_from_value (constructor_value);

    completion_value =
      ecma_op_function_construct (constructor_obj_p, constructor_obj_p, stack_top_p, arguments_list_len);
  }

  /* Free registers. */
  for (uint32_t i = 0; i < arguments_list_len; i++)
  {
    ecma_fast_free_value (stack_top_p[i]);
  }

  if (JERRY_UNLIKELY (ECMA_IS_VALUE_ERROR (completion_value)))
  {
    frame_ctx_p->byte_code_p = (uint8_t *) vm_error_byte_code_p;
  }
  else
  {
    ecma_free_value (stack_top_p[-1]);
    frame_ctx_p->byte_code_p = byte_code_p;
    stack_top_p[-1] = completion_value;
  }

  frame_ctx_p->stack_top_p = stack_top_p;
} /* opfunc_construct */

/**
 * Read literal index from the byte code stream into destination.
 *
 * @param destination destination
 */
#define READ_LITERAL_INDEX(destination)                                                      \
  do                                                                                         \
  {                                                                                          \
    (destination) = *byte_code_p++;                                                          \
    if ((destination) >= encoding_limit)                                                     \
    {                                                                                        \
      (destination) = (uint16_t) ((((destination) << 8) | *byte_code_p++) - encoding_delta); \
    }                                                                                        \
  } while (0)

/**
 * Get literal value by literal index.
 *
 * @param literal_index literal index
 * @param target_value target value
 *
 * TODO: For performance reasons, we define this as a macro.
 * When we are able to construct a function with similar speed,
 * we can remove this macro.
 */
#define READ_LITERAL(literal_index, target_value)                                                 \
  do                                                                                              \
  {                                                                                               \
    if ((literal_index) < ident_end)                                                              \
    {                                                                                             \
      if ((literal_index) < register_end)                                                         \
      {                                                                                           \
        /* Note: There should be no specialization for arguments. */                              \
        (target_value) = ecma_fast_copy_value (VM_GET_REGISTER (frame_ctx_p, literal_index));     \
      }                                                                                           \
      else                                                                                        \
      {                                                                                           \
        ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);      \
                                                                                                  \
        result = ecma_op_resolve_reference_value (frame_ctx_p->lex_env_p, name_p);                \
                                                                                                  \
        if (ECMA_IS_VALUE_ERROR (result))                                                         \
        {                                                                                         \
          goto error;                                                                             \
        }                                                                                         \
        (target_value) = result;                                                                  \
      }                                                                                           \
    }                                                                                             \
    else if (literal_index < const_literal_end)                                                   \
    {                                                                                             \
      (target_value) = ecma_fast_copy_value (literal_start_p[literal_index]);                     \
    }                                                                                             \
    else                                                                                          \
    {                                                                                             \
      /* Object construction. */                                                                  \
      (target_value) = vm_construct_literal_object (frame_ctx_p, literal_start_p[literal_index]); \
    }                                                                                             \
  } while (0)

/**
 * Store the original value for post increase/decrease operators
 *
 * @param value original value
 */
#define POST_INCREASE_DECREASE_PUT_RESULT(value)                                                             \
  if (opcode_data & VM_OC_PUT_STACK)                                                                         \
  {                                                                                                          \
    if (opcode_flags & VM_OC_IDENT_INCR_DECR_OPERATOR_FLAG)                                                  \
    {                                                                                                        \
      JERRY_ASSERT (opcode == CBC_POST_INCR_IDENT_PUSH_RESULT || opcode == CBC_POST_DECR_IDENT_PUSH_RESULT); \
      *stack_top_p++ = (value);                                                                              \
    }                                                                                                        \
    else                                                                                                     \
    {                                                                                                        \
      /* The parser ensures there is enough space for the                                                    \
       * extra value on the stack. See js-parser-expr.c. */                                                  \
      JERRY_ASSERT (opcode == CBC_POST_INCR_PUSH_RESULT || opcode == CBC_POST_DECR_PUSH_RESULT);             \
      stack_top_p++;                                                                                         \
      stack_top_p[-1] = stack_top_p[-2];                                                                     \
      stack_top_p[-2] = stack_top_p[-3];                                                                     \
      stack_top_p[-3] = (value);                                                                             \
    }                                                                                                        \
    opcode_data &= (uint32_t) ~VM_OC_PUT_STACK;                                                              \
  }                                                                                                          \
  else                                                                                                       \
  {                                                                                                          \
    JERRY_ASSERT (opcode_data &VM_OC_PUT_BLOCK);                                                             \
    ecma_free_value (VM_GET_REGISTER (frame_ctx_p, 0));                                                      \
    VM_GET_REGISTERS (frame_ctx_p)[0] = (value);                                                             \
    opcode_data &= (uint32_t) ~VM_OC_PUT_BLOCK;                                                              \
  }

/**
 * Get the end of the existing topmost context
 */
#define VM_LAST_CONTEXT_END() (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth)

/**
 * Run generic byte code.
 *
 * @return ecma value
 */
static ecma_value_t JERRY_ATTR_NOINLINE
vm_loop (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  const ecma_compiled_code_t *bytecode_header_p = frame_ctx_p->shared_p->bytecode_header_p;
  const uint8_t *byte_code_p = frame_ctx_p->byte_code_p;
  ecma_value_t *literal_start_p = frame_ctx_p->literal_start_p;

  ecma_value_t *stack_top_p;
  uint16_t encoding_limit;
  uint16_t encoding_delta;
  uint16_t register_end;
  uint16_t ident_end;
  uint16_t const_literal_end;
  int32_t branch_offset = 0;
  uint8_t branch_offset_length = 0;
  ecma_value_t left_value;
  ecma_value_t right_value;
  ecma_value_t result = ECMA_VALUE_EMPTY;
  bool is_strict = ((bytecode_header_p->status_flags & CBC_CODE_FLAGS_STRICT_MODE) != 0);

  /* Prepare for byte code execution. */
  if (!(bytecode_header_p->status_flags & CBC_CODE_FLAGS_FULL_LITERAL_ENCODING))
  {
    encoding_limit = CBC_SMALL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_SMALL_LITERAL_ENCODING_DELTA;
  }
  else
  {
    encoding_limit = CBC_FULL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_FULL_LITERAL_ENCODING_DELTA;
  }

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) (bytecode_header_p);
    register_end = args_p->register_end;
    ident_end = args_p->ident_end;
    const_literal_end = args_p->const_literal_end;
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) (bytecode_header_p);
    register_end = args_p->register_end;
    ident_end = args_p->ident_end;
    const_literal_end = args_p->const_literal_end;
  }

  stack_top_p = frame_ctx_p->stack_top_p;

  /* Outer loop for exception handling. */
  while (true)
  {
    /* Internal loop for byte code execution. */
    while (true)
    {
      const uint8_t *byte_code_start_p = byte_code_p;
      uint8_t opcode = *byte_code_p++;
      uint32_t opcode_data = opcode;

      if (opcode == CBC_EXT_OPCODE)
      {
        opcode = *byte_code_p++;
        opcode_data = (uint32_t) ((CBC_END + 1) + opcode);
      }

      opcode_data = vm_decode_table[opcode_data];

      left_value = ECMA_VALUE_UNDEFINED;
      right_value = ECMA_VALUE_UNDEFINED;

      uint32_t operands = VM_OC_GET_ARGS_INDEX (opcode_data);

      if (operands >= VM_OC_GET_LITERAL)
      {
        uint16_t literal_index;
        READ_LITERAL_INDEX (literal_index);
        READ_LITERAL (literal_index, left_value);

        if (operands != VM_OC_GET_LITERAL)
        {
          switch (operands)
          {
            case VM_OC_GET_LITERAL_LITERAL:
            {
              uint16_t second_literal_index;
              READ_LITERAL_INDEX (second_literal_index);
              READ_LITERAL (second_literal_index, right_value);
              break;
            }
            case VM_OC_GET_STACK_LITERAL:
            {
              JERRY_ASSERT (stack_top_p > VM_GET_REGISTERS (frame_ctx_p) + register_end);
              right_value = left_value;
              left_value = *(--stack_top_p);
              break;
            }
            default:
            {
              JERRY_ASSERT (operands == VM_OC_GET_THIS_LITERAL);

              right_value = left_value;
              left_value = ecma_copy_value (frame_ctx_p->this_binding);
              break;
            }
          }
        }
      }
      else if (operands >= VM_OC_GET_STACK)
      {
        JERRY_ASSERT (operands == VM_OC_GET_STACK || operands == VM_OC_GET_STACK_STACK);

        JERRY_ASSERT (stack_top_p > VM_GET_REGISTERS (frame_ctx_p) + register_end);
        left_value = *(--stack_top_p);

        if (operands == VM_OC_GET_STACK_STACK)
        {
          JERRY_ASSERT (stack_top_p > VM_GET_REGISTERS (frame_ctx_p) + register_end);
          right_value = left_value;
          left_value = *(--stack_top_p);
        }
      }
      else if (operands == VM_OC_GET_BRANCH)
      {
        branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (opcode);
        JERRY_ASSERT (branch_offset_length >= 1 && branch_offset_length <= 3);

        branch_offset = *(byte_code_p++);

        if (JERRY_UNLIKELY (branch_offset_length != 1))
        {
          branch_offset <<= 8;
          branch_offset |= *(byte_code_p++);

          if (JERRY_UNLIKELY (branch_offset_length == 3))
          {
            branch_offset <<= 8;
            branch_offset |= *(byte_code_p++);
          }
        }

        if (opcode_data & VM_OC_BACKWARD_BRANCH)
        {
#if JERRY_VM_HALT
          if (JERRY_CONTEXT (vm_exec_stop_cb) != NULL && --JERRY_CONTEXT (vm_exec_stop_counter) == 0)
          {
            result = JERRY_CONTEXT (vm_exec_stop_cb) (JERRY_CONTEXT (vm_exec_stop_user_p));

            if (ecma_is_value_undefined (result))
            {
              JERRY_CONTEXT (vm_exec_stop_counter) = JERRY_CONTEXT (vm_exec_stop_frequency);
            }
            else
            {
              JERRY_CONTEXT (vm_exec_stop_counter) = 1;

              if (ecma_is_value_exception (result))
              {
                ecma_throw_exception (result);
              }
              else
              {
                jcontext_raise_exception (result);
              }

              JERRY_ASSERT (jcontext_has_pending_exception ());
              jcontext_set_abort_flag (true);
              result = ECMA_VALUE_ERROR;
              goto error;
            }
          }
#endif /* JERRY_VM_HALT */

          branch_offset = -branch_offset;
        }
      }

      switch (VM_OC_GROUP_GET_INDEX (opcode_data))
      {
        case VM_OC_POP:
        {
          JERRY_ASSERT (stack_top_p > VM_GET_REGISTERS (frame_ctx_p) + register_end);
          ecma_free_value (*(--stack_top_p));
          continue;
        }
        case VM_OC_POP_BLOCK:
        {
          ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, 0));
          VM_GET_REGISTERS (frame_ctx_p)[0] = *(--stack_top_p);
          continue;
        }
        case VM_OC_PUSH:
        {
          *stack_top_p++ = left_value;
          continue;
        }
        case VM_OC_PUSH_TWO:
        {
          *stack_top_p++ = left_value;
          *stack_top_p++ = right_value;
          continue;
        }
        case VM_OC_PUSH_THREE:
        {
          uint16_t literal_index;

          *stack_top_p++ = left_value;
          left_value = ECMA_VALUE_UNDEFINED;

          READ_LITERAL_INDEX (literal_index);
          READ_LITERAL (literal_index, left_value);

          *stack_top_p++ = right_value;
          *stack_top_p++ = left_value;
          continue;
        }
        case VM_OC_PUSH_UNDEFINED:
        {
          *stack_top_p++ = ECMA_VALUE_UNDEFINED;
          continue;
        }
        case VM_OC_PUSH_TRUE:
        {
          *stack_top_p++ = ECMA_VALUE_TRUE;
          continue;
        }
        case VM_OC_PUSH_FALSE:
        {
          *stack_top_p++ = ECMA_VALUE_FALSE;
          continue;
        }
        case VM_OC_PUSH_NULL:
        {
          *stack_top_p++ = ECMA_VALUE_NULL;
          continue;
        }
        case VM_OC_PUSH_THIS:
        {
          *stack_top_p++ = ecma_copy_value (frame_ctx_p->this_binding);
          continue;
        }
        case VM_OC_PUSH_0:
        {
          *stack_top_p++ = ecma_make_integer_value (0);
          continue;
        }
        case VM_OC_PUSH_POS_BYTE:
        {
          ecma_integer_value_t number = *byte_code_p++;
          *stack_top_p++ = ecma_make_integer_value (number + 1);
          continue;
        }
        case VM_OC_PUSH_NEG_BYTE:
        {
          ecma_integer_value_t number = *byte_code_p++;
          *stack_top_p++ = ecma_make_integer_value (-(number + 1));
          continue;
        }
        case VM_OC_PUSH_LIT_0:
        {
          stack_top_p[0] = left_value;
          stack_top_p[1] = ecma_make_integer_value (0);
          stack_top_p += 2;
          continue;
        }
        case VM_OC_PUSH_LIT_POS_BYTE:
        {
          ecma_integer_value_t number = *byte_code_p++;
          stack_top_p[0] = left_value;
          stack_top_p[1] = ecma_make_integer_value (number + 1);
          stack_top_p += 2;
          continue;
        }
        case VM_OC_PUSH_LIT_NEG_BYTE:
        {
          ecma_integer_value_t number = *byte_code_p++;
          stack_top_p[0] = left_value;
          stack_top_p[1] = ecma_make_integer_value (-(number + 1));
          stack_top_p += 2;
          continue;
        }
        case VM_OC_PUSH_OBJECT:
        {
          ecma_object_t *obj_p =
            ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE), 0, ECMA_OBJECT_TYPE_GENERAL);

          *stack_top_p++ = ecma_make_object_value (obj_p);
          continue;
        }
        case VM_OC_PUSH_NAMED_FUNC_EXPR:
        {
          ecma_object_t *func_p = ecma_get_object_from_value (left_value);

          JERRY_ASSERT (ecma_get_object_type (func_p) == ECMA_OBJECT_TYPE_FUNCTION);

          ecma_extended_object_t *ext_func_p = (ecma_extended_object_t *) func_p;

          JERRY_ASSERT (frame_ctx_p->lex_env_p
                        == ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, ext_func_p->u.function.scope_cp));

          ecma_object_t *name_lex_env = ecma_create_decl_lex_env (frame_ctx_p->lex_env_p);

          ecma_op_create_immutable_binding (name_lex_env, ecma_get_string_from_value (right_value), left_value);

          ECMA_SET_NON_NULL_POINTER_TAG (ext_func_p->u.function.scope_cp, name_lex_env, 0);

          ecma_free_value (right_value);
          ecma_deref_object (name_lex_env);
          *stack_top_p++ = left_value;
          continue;
        }
        case VM_OC_CREATE_BINDING:
        {
          uint32_t literal_index;

          READ_LITERAL_INDEX (literal_index);

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

          JERRY_ASSERT (ecma_get_lex_env_type (frame_ctx_p->lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);
          JERRY_ASSERT (ecma_find_named_property (frame_ctx_p->lex_env_p, name_p) == NULL);

          uint8_t prop_attributes = ECMA_PROPERTY_FLAG_WRITABLE;

          if (opcode == CBC_CREATE_LET)
          {
            prop_attributes = ECMA_PROPERTY_ENUMERABLE_WRITABLE;
          }
          else if (opcode == CBC_CREATE_CONST)
          {
            prop_attributes = ECMA_PROPERTY_FLAG_ENUMERABLE;
          }

          ecma_property_value_t *property_value_p;
          property_value_p = ecma_create_named_data_property (frame_ctx_p->lex_env_p, name_p, prop_attributes, NULL);

          if (opcode != CBC_CREATE_VAR)
          {
            property_value_p->value = ECMA_VALUE_UNINITIALIZED;
          }

          continue;
        }
        case VM_OC_VAR_EVAL:
        {
          uint32_t literal_index;
          ecma_value_t lit_value = ECMA_VALUE_UNDEFINED;

          if (opcode == CBC_CREATE_VAR_FUNC_EVAL)
          {
            uint32_t value_index;
            READ_LITERAL_INDEX (value_index);
            JERRY_ASSERT (value_index >= const_literal_end);

            lit_value = vm_construct_literal_object (frame_ctx_p, literal_start_p[value_index]);
          }

          READ_LITERAL_INDEX (literal_index);
          JERRY_ASSERT (literal_index >= register_end);

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);
          ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;

          while (lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK)
          {
#if !(defined JERRY_NDEBUG)
            if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
            {
              ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

              JERRY_ASSERT (property_p == NULL || !(*property_p & ECMA_PROPERTY_FLAG_ENUMERABLE));
            }
#endif /* !JERRY_NDEBUG */

            JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
            lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
          }

#if !(defined JERRY_NDEBUG)
          if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
          {
            ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

            JERRY_ASSERT (property_p == NULL || !(*property_p & ECMA_PROPERTY_FLAG_ENUMERABLE));
          }
#endif /* !JERRY_NDEBUG */

          /* 'Variable declaration' */
          result = ecma_op_has_binding (lex_env_p, name_p);

#if JERRY_BUILTIN_PROXY
          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
#endif /* JERRY_BUILTIN_PROXY */

          ecma_property_t *prop_p = NULL;

          if (ecma_is_value_false (result))
          {
            bool is_configurable = (frame_ctx_p->status_flags & VM_FRAME_CTX_DIRECT_EVAL) != 0;
            prop_p = ecma_op_create_mutable_binding (lex_env_p, name_p, is_configurable);

            if (JERRY_UNLIKELY (prop_p == ECMA_PROPERTY_POINTER_ERROR))
            {
              result = ECMA_VALUE_ERROR;
              goto error;
            }
          }

          if (lit_value != ECMA_VALUE_UNDEFINED)
          {
            JERRY_ASSERT (ecma_is_value_object (lit_value));

            if (prop_p != NULL)
            {
              JERRY_ASSERT (ecma_is_value_undefined (ECMA_PROPERTY_VALUE_PTR (prop_p)->value));
              JERRY_ASSERT (ecma_is_property_writable (*prop_p));
              ECMA_PROPERTY_VALUE_PTR (prop_p)->value = lit_value;
              ecma_free_object (lit_value);
            }
            else
            {
              result = ecma_op_put_value_lex_env_base (lex_env_p, name_p, is_strict, lit_value);
              ecma_free_object (lit_value);

              if (ECMA_IS_VALUE_ERROR (result))
              {
                goto error;
              }
            }
          }
          continue;
        }
        case VM_OC_EXT_VAR_EVAL:
        {
          uint32_t literal_index;
          ecma_value_t lit_value = ECMA_VALUE_UNDEFINED;

          JERRY_ASSERT (byte_code_start_p[0] == CBC_EXT_OPCODE);

          if (opcode == CBC_EXT_CREATE_VAR_FUNC_EVAL)
          {
            uint32_t value_index;
            READ_LITERAL_INDEX (value_index);
            JERRY_ASSERT (value_index >= const_literal_end);

            lit_value = vm_construct_literal_object (frame_ctx_p, literal_start_p[value_index]);
          }

          READ_LITERAL_INDEX (literal_index);
          JERRY_ASSERT (literal_index >= register_end);

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);
          ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;
          ecma_object_t *prev_lex_env_p = NULL;

          while (lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK)
          {
#if !(defined JERRY_NDEBUG)
            if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
            {
              ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

              JERRY_ASSERT (property_p == NULL || !(*property_p & ECMA_PROPERTY_FLAG_ENUMERABLE));
            }
#endif /* !JERRY_NDEBUG */

            JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
            prev_lex_env_p = lex_env_p;
            lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
          }

          JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);
          JERRY_ASSERT (prev_lex_env_p != NULL
                        && ecma_get_lex_env_type (prev_lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);

          ecma_property_t *property_p = ecma_find_named_property (prev_lex_env_p, name_p);
          ecma_property_value_t *property_value_p;

          if (property_p == NULL)
          {
            property_value_p =
              ecma_create_named_data_property (prev_lex_env_p, name_p, ECMA_PROPERTY_CONFIGURABLE_WRITABLE, NULL);

            if (lit_value == ECMA_VALUE_UNDEFINED)
            {
              continue;
            }
          }
          else
          {
            if (lit_value == ECMA_VALUE_UNDEFINED)
            {
              continue;
            }

            property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
            ecma_free_value_if_not_object (property_value_p->value);
          }

          property_value_p->value = lit_value;
          ecma_deref_object (ecma_get_object_from_value (lit_value));
          continue;
        }
        case VM_OC_CREATE_ARGUMENTS:
        {
          uint32_t literal_index;
          READ_LITERAL_INDEX (literal_index);

          JERRY_ASSERT (frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_HAS_ARG_LIST);

          result = ecma_op_create_arguments_object ((vm_frame_ctx_shared_args_t *) (frame_ctx_p->shared_p),
                                                    frame_ctx_p->lex_env_p);

          if (literal_index < register_end)
          {
            JERRY_ASSERT (VM_GET_REGISTER (frame_ctx_p, literal_index) == ECMA_VALUE_UNDEFINED);
            VM_GET_REGISTER (frame_ctx_p, literal_index) = result;
            continue;
          }

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

          JERRY_ASSERT (ecma_find_named_property (frame_ctx_p->lex_env_p, name_p) == NULL);

          uint8_t prop_attributes = ECMA_PROPERTY_FLAG_WRITABLE;
          ecma_property_value_t *property_value_p;

          property_value_p = ecma_create_named_data_property (frame_ctx_p->lex_env_p, name_p, prop_attributes, NULL);
          property_value_p->value = result;

          ecma_deref_object (ecma_get_object_from_value (result));
          continue;
        }
#if JERRY_SNAPSHOT_EXEC
        case VM_OC_SET_BYTECODE_PTR:
        {
          memcpy (&byte_code_p, byte_code_p++, sizeof (uintptr_t));
          frame_ctx_p->byte_code_start_p = byte_code_p;
          continue;
        }
#endif /* JERRY_SNAPSHOT_EXEC */
        case VM_OC_INIT_ARG_OR_FUNC:
        {
          uint32_t literal_index, value_index;
          ecma_value_t lit_value;
          bool release = false;

          READ_LITERAL_INDEX (value_index);

          if (value_index < register_end)
          {
            /* Take (not copy) the reference. */
            lit_value = ecma_copy_value_if_not_object (VM_GET_REGISTER (frame_ctx_p, value_index));
          }
          else
          {
            lit_value = vm_construct_literal_object (frame_ctx_p, literal_start_p[value_index]);
            release = true;
          }

          READ_LITERAL_INDEX (literal_index);

          JERRY_ASSERT (value_index != literal_index);
          JERRY_ASSERT (value_index >= register_end || literal_index >= register_end);

          if (literal_index < register_end)
          {
            ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, literal_index));
            JERRY_ASSERT (release);
            VM_GET_REGISTER (frame_ctx_p, literal_index) = lit_value;
            continue;
          }

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

          JERRY_ASSERT (ecma_get_lex_env_type (frame_ctx_p->lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);
          JERRY_ASSERT (ecma_find_named_property (frame_ctx_p->lex_env_p, name_p) == NULL);

          ecma_property_value_t *property_value_p;
          property_value_p =
            ecma_create_named_data_property (frame_ctx_p->lex_env_p, name_p, ECMA_PROPERTY_FLAG_WRITABLE, NULL);

          JERRY_ASSERT (property_value_p->value == ECMA_VALUE_UNDEFINED);
          property_value_p->value = lit_value;

          if (release)
          {
            ecma_deref_object (ecma_get_object_from_value (lit_value));
          }
          continue;
        }
        case VM_OC_CHECK_VAR:
        {
          JERRY_ASSERT (CBC_FUNCTION_GET_TYPE (frame_ctx_p->shared_p->bytecode_header_p->status_flags)
                        == CBC_FUNCTION_SCRIPT);

          uint32_t literal_index;
          READ_LITERAL_INDEX (literal_index);

          if ((frame_ctx_p->lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK) == 0)
          {
            continue;
          }

          ecma_string_t *const literal_name_p = ecma_get_string_from_value (literal_start_p[literal_index]);
          ecma_property_t *const binding_p = ecma_find_named_property (frame_ctx_p->lex_env_p, literal_name_p);

          if (binding_p != NULL)
          {
            result = ecma_raise_syntax_error (ECMA_ERR_LOCAL_VARIABLE_IS_REDECLARED);
            goto error;
          }

          continue;
        }
        case VM_OC_CHECK_LET:
        {
          JERRY_ASSERT (CBC_FUNCTION_GET_TYPE (frame_ctx_p->shared_p->bytecode_header_p->status_flags)
                        == CBC_FUNCTION_SCRIPT);

          uint32_t literal_index;
          READ_LITERAL_INDEX (literal_index);

          ecma_string_t *literal_name_p = ecma_get_string_from_value (literal_start_p[literal_index]);
          ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;

          if (lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK)
          {
            result = opfunc_lexical_scope_has_restricted_binding (frame_ctx_p, literal_name_p);

            if (!ecma_is_value_false (result))
            {
              if (ecma_is_value_true (result))
              {
                result = ecma_raise_syntax_error (ECMA_ERR_LOCAL_VARIABLE_IS_REDECLARED);
              }

              JERRY_ASSERT (ECMA_IS_VALUE_ERROR (result));
              goto error;
            }

            continue;
          }

          result = ecma_op_has_binding (lex_env_p, literal_name_p);

#if JERRY_BUILTIN_PROXY
          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
#endif /* JERRY_BUILTIN_PROXY */

          if (ecma_is_value_true (result))
          {
            result = ecma_raise_syntax_error (ECMA_ERR_LOCAL_VARIABLE_IS_REDECLARED);
            goto error;
          }

          continue;
        }
        case VM_OC_ASSIGN_LET_CONST:
        {
          uint32_t literal_index;
          READ_LITERAL_INDEX (literal_index);

          JERRY_ASSERT (literal_index >= register_end);
          JERRY_ASSERT (ecma_get_lex_env_type (frame_ctx_p->lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE
                        || (ecma_get_lex_env_type (frame_ctx_p->lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_CLASS
                            && ECMA_LEX_ENV_CLASS_IS_MODULE (frame_ctx_p->lex_env_p)));

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);
          ecma_property_t *property_p = ecma_find_named_property (frame_ctx_p->lex_env_p, name_p);

          JERRY_ASSERT (property_p != NULL && ECMA_PROPERTY_IS_RAW_DATA (*property_p)
                        && (*property_p & ECMA_PROPERTY_FLAG_DATA));
          JERRY_ASSERT (ECMA_PROPERTY_VALUE_PTR (property_p)->value == ECMA_VALUE_UNINITIALIZED);

          ECMA_PROPERTY_VALUE_PTR (property_p)->value = left_value;

          if (ecma_is_value_object (left_value))
          {
            ecma_deref_object (ecma_get_object_from_value (left_value));
          }
          continue;
        }
        case VM_OC_INIT_BINDING:
        {
          uint32_t literal_index;

          READ_LITERAL_INDEX (literal_index);

          JERRY_ASSERT (literal_index >= register_end);

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

          JERRY_ASSERT (ecma_get_lex_env_type (frame_ctx_p->lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);
          JERRY_ASSERT (ecma_find_named_property (frame_ctx_p->lex_env_p, name_p) == NULL);

          uint8_t prop_attributes = ECMA_PROPERTY_FLAG_WRITABLE;

          if (opcode == CBC_INIT_LET)
          {
            prop_attributes = ECMA_PROPERTY_ENUMERABLE_WRITABLE;
          }
          else if (opcode == CBC_INIT_CONST)
          {
            prop_attributes = ECMA_PROPERTY_FLAG_ENUMERABLE;
          }

          ecma_property_value_t *property_value_p;
          property_value_p = ecma_create_named_data_property (frame_ctx_p->lex_env_p, name_p, prop_attributes, NULL);

          JERRY_ASSERT (property_value_p->value == ECMA_VALUE_UNDEFINED);

          ecma_value_t value = *(--stack_top_p);

          property_value_p->value = value;
          ecma_deref_if_object (value);
          continue;
        }
        case VM_OC_THROW_CONST_ERROR:
        {
          result = ecma_raise_type_error (ECMA_ERR_CONSTANT_BINDINGS_CANNOT_BE_REASSIGNED);
          goto error;
        }
        case VM_OC_COPY_TO_GLOBAL:
        {
          uint32_t literal_index;
          READ_LITERAL_INDEX (literal_index);

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);
          ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;

          while (lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK)
          {
#ifndef JERRY_NDEBUG
            if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
            {
              ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);

              JERRY_ASSERT (property_p == NULL || !(*property_p & ECMA_PROPERTY_FLAG_ENUMERABLE));
            }
#endif /* !JERRY_NDEBUG */

            JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
            lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
          }

          if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE)
          {
            ecma_property_t *property_p = ecma_find_named_property (lex_env_p, name_p);
            ecma_property_value_t *prop_value_p;

            if (property_p == NULL)
            {
              prop_value_p = ecma_create_named_data_property (lex_env_p, name_p, ECMA_PROPERTY_FLAG_WRITABLE, NULL);
            }
            else
            {
#ifndef JERRY_NDEBUG
              JERRY_ASSERT (!(*property_p & ECMA_PROPERTY_FLAG_ENUMERABLE));
#endif /* !JERRY_NDEBUG */
              prop_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
            }

            ecma_named_data_property_assign_value (lex_env_p, prop_value_p, left_value);
          }
          else
          {
            result = ecma_op_set_mutable_binding (lex_env_p, name_p, left_value, is_strict);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              goto error;
            }
          }

          goto free_left_value;
        }
        case VM_OC_COPY_FROM_ARG:
        {
          uint32_t literal_index;
          READ_LITERAL_INDEX (literal_index);
          JERRY_ASSERT (literal_index >= register_end);

          ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);
          ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;
          ecma_object_t *arg_lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);

          JERRY_ASSERT ((lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK)
                        && ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);
          JERRY_ASSERT (arg_lex_env_p != NULL && !(arg_lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK)
                        && ecma_get_lex_env_type (arg_lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);

          ecma_property_value_t *property_value_p;
          property_value_p = ecma_create_named_data_property (lex_env_p, name_p, ECMA_PROPERTY_FLAG_WRITABLE, NULL);

          ecma_property_t *property_p = ecma_find_named_property (arg_lex_env_p, name_p);
          JERRY_ASSERT (property_p != NULL);

          ecma_property_value_t *arg_prop_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
          property_value_p->value = ecma_copy_value_if_not_object (arg_prop_value_p->value);
          continue;
        }
        case VM_OC_CLONE_CONTEXT:
        {
          JERRY_ASSERT (byte_code_start_p[0] == CBC_EXT_OPCODE);

          bool copy_values = (byte_code_start_p[1] == CBC_EXT_CLONE_FULL_CONTEXT);
          frame_ctx_p->lex_env_p = ecma_clone_decl_lexical_environment (frame_ctx_p->lex_env_p, copy_values);
          continue;
        }
        case VM_OC_SET__PROTO__:
        {
          result = ecma_builtin_object_object_set_proto (stack_top_p[-1], left_value);
          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          goto free_left_value;
        }
        case VM_OC_CLASS_CALL_STATIC_BLOCK:
        {
          result = ecma_op_function_call (ecma_get_object_from_value (left_value), frame_ctx_p->this_binding, NULL, 0);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          goto free_left_value;
        }
        case VM_OC_PUSH_STATIC_FIELD_FUNC:
        {
          JERRY_ASSERT (byte_code_start_p[0] == CBC_EXT_OPCODE
                        && (byte_code_start_p[1] == CBC_EXT_PUSH_STATIC_FIELD_FUNC
                            || byte_code_start_p[1] == CBC_EXT_PUSH_STATIC_COMPUTED_FIELD_FUNC));

          bool push_computed = (byte_code_start_p[1] == CBC_EXT_PUSH_STATIC_COMPUTED_FIELD_FUNC);
          ecma_value_t value = stack_top_p[-1];

          if (!push_computed)
          {
            stack_top_p++;
          }

          memmove (stack_top_p - 3, stack_top_p - 4, 3 * sizeof (ecma_value_t));
          stack_top_p[-4] = left_value;

          ecma_object_t *class_object_p = ecma_get_object_from_value (stack_top_p[-2]);
          ecma_object_t *initializer_func_p = ecma_get_object_from_value (left_value);
          opfunc_bind_class_environment (frame_ctx_p->lex_env_p, class_object_p, class_object_p, initializer_func_p);

          if (!push_computed)
          {
            continue;
          }

          left_value = value;
          /* FALLTHRU */
        }
        case VM_OC_ADD_COMPUTED_FIELD:
        {
          JERRY_ASSERT (byte_code_start_p[0] == CBC_EXT_OPCODE
                        && (byte_code_start_p[1] == CBC_EXT_PUSH_STATIC_COMPUTED_FIELD_FUNC
                            || byte_code_start_p[1] == CBC_EXT_ADD_COMPUTED_FIELD
                            || byte_code_start_p[1] == CBC_EXT_ADD_STATIC_COMPUTED_FIELD));

          int index = (byte_code_start_p[1] == CBC_EXT_ADD_COMPUTED_FIELD) ? -2 : -4;
          result = opfunc_add_computed_field (stack_top_p[index], left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          goto free_left_value;
        }
        case VM_OC_COPY_DATA_PROPERTIES:
        {
          left_value = *(--stack_top_p);

          if (ecma_is_value_undefined (left_value) || ecma_is_value_null (left_value))
          {
            continue;
          }

          result = opfunc_copy_data_properties (stack_top_p[-1], left_value, ECMA_VALUE_UNDEFINED);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          goto free_left_value;
        }
        case VM_OC_SET_COMPUTED_PROPERTY:
        {
          /* Swap values. */
          left_value ^= right_value;
          right_value ^= left_value;
          left_value ^= right_value;
          /* FALLTHRU */
        }
        case VM_OC_SET_PROPERTY:
        {
          JERRY_STATIC_ASSERT ((VM_OC_NON_STATIC_FLAG == VM_OC_BACKWARD_BRANCH),
                               vm_oc_non_static_flag_must_be_equal_to_vm_oc_backward_branch);

          JERRY_ASSERT ((opcode_data >> VM_OC_NON_STATIC_SHIFT) <= 0x1);

          ecma_string_t *prop_name_p = ecma_op_to_property_key (right_value);

          if (JERRY_UNLIKELY (prop_name_p == NULL))
          {
            result = ECMA_VALUE_ERROR;
            goto error;
          }

          if (JERRY_UNLIKELY (ecma_compare_ecma_string_to_magic_id (prop_name_p, LIT_MAGIC_STRING_PROTOTYPE))
              && !(opcode_data & VM_OC_NON_STATIC_FLAG))
          {
            result = ecma_raise_type_error (ECMA_ERR_CLASS_IS_NON_CONFIGURABLE);
            goto error;
          }

          const int index = (int) (opcode_data >> VM_OC_NON_STATIC_SHIFT) - 2;

          ecma_object_t *object_p = ecma_get_object_from_value (stack_top_p[index]);

          opfunc_set_data_property (object_p, prop_name_p, left_value);
          ecma_deref_ecma_string (prop_name_p);

          goto free_both_values;
        }
        case VM_OC_SET_GETTER:
        case VM_OC_SET_SETTER:
        {
          JERRY_ASSERT ((opcode_data >> VM_OC_NON_STATIC_SHIFT) <= 0x1);

          ecma_string_t *prop_name_p = ecma_op_to_property_key (left_value);

          if (JERRY_UNLIKELY (prop_name_p == NULL))
          {
            result = ECMA_VALUE_ERROR;
            goto error;
          }

          if (JERRY_UNLIKELY (ecma_compare_ecma_string_to_magic_id (prop_name_p, LIT_MAGIC_STRING_PROTOTYPE))
              && !(opcode_data & VM_OC_NON_STATIC_FLAG))
          {
            result = ecma_raise_type_error (ECMA_ERR_CLASS_IS_NON_CONFIGURABLE);
            goto error;
          }

          const int index = (int) (opcode_data >> VM_OC_NON_STATIC_SHIFT) - 2;
          opfunc_set_accessor (VM_OC_GROUP_GET_INDEX (opcode_data) == VM_OC_SET_GETTER,
                               stack_top_p[index],
                               prop_name_p,
                               right_value);

          ecma_deref_ecma_string (prop_name_p);

          goto free_both_values;
        }
        case VM_OC_PUSH_ARRAY:
        {
          /* Note: this operation cannot throw an exception */
          *stack_top_p++ = ecma_make_object_value (ecma_op_new_array_object (0));
          continue;
        }
        case VM_OC_LOCAL_EVAL:
        {
          ECMA_CLEAR_LOCAL_PARSE_OPTS ();
          uint8_t parse_opts = *byte_code_p++;
          ECMA_SET_LOCAL_PARSE_OPTS (parse_opts);
          continue;
        }
        case VM_OC_SUPER_CALL:
        {
          uint8_t arguments_list_len = *byte_code_p++;

          if (opcode >= CBC_EXT_SPREAD_SUPER_CALL)
          {
            stack_top_p -= arguments_list_len;
            ecma_collection_t *arguments_p = opfunc_spread_arguments (stack_top_p, arguments_list_len);

            if (JERRY_UNLIKELY (arguments_p == NULL))
            {
              result = ECMA_VALUE_ERROR;
              goto error;
            }

            stack_top_p++;
            ECMA_SET_INTERNAL_VALUE_POINTER (stack_top_p[-1], arguments_p);
          }
          else
          {
            stack_top_p -= arguments_list_len;
          }

          frame_ctx_p->call_operation = VM_EXEC_SUPER_CALL;
          frame_ctx_p->byte_code_p = byte_code_start_p;
          frame_ctx_p->stack_top_p = stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_PUSH_CLASS_ENVIRONMENT:
        {
          uint16_t literal_index;

          READ_LITERAL_INDEX (literal_index);
          opfunc_push_class_environment (frame_ctx_p, &stack_top_p, literal_start_p[literal_index]);
          continue;
        }
        case VM_OC_PUSH_IMPLICIT_CTOR:
        {
          *stack_top_p++ = opfunc_create_implicit_class_constructor (opcode, frame_ctx_p->shared_p->bytecode_header_p);
          continue;
        }
        case VM_OC_DEFINE_FIELD:
        {
          result = opfunc_define_field (frame_ctx_p->this_binding, right_value, left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          goto free_both_values;
        }
        case VM_OC_ASSIGN_PRIVATE:
        {
          result = opfunc_private_set (stack_top_p[-3], stack_top_p[-2], stack_top_p[-1]);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          ecma_free_value (stack_top_p[-3]);
          ecma_free_value (stack_top_p[-2]);
          ecma_free_value (stack_top_p[-1]);
          stack_top_p -= 3;

          if (opcode_data & VM_OC_PUT_STACK)
          {
            *stack_top_p++ = result;
          }
          else if (opcode_data & VM_OC_PUT_BLOCK)
          {
            ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, 0));
            VM_GET_REGISTERS (frame_ctx_p)[0] = result;
          }
          else
          {
            ecma_free_value (result);
          }

          goto free_both_values;
        }
        case VM_OC_PRIVATE_FIELD_ADD:
        {
          result = opfunc_private_field_add (frame_ctx_p->this_binding, right_value, left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          goto free_both_values;
        }
        case VM_OC_PRIVATE_PROP_GET:
        {
          result = opfunc_private_get (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_PRIVATE_PROP_REFERENCE:
        {
          result = opfunc_private_get (stack_top_p[-1], left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = left_value;
          *stack_top_p++ = result;
          continue;
        }
        case VM_OC_PRIVATE_IN:
        {
          result = opfunc_private_in (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_COLLECT_PRIVATE_PROPERTY:
        {
          opfunc_collect_private_properties (stack_top_p[-2], left_value, right_value, opcode);
          continue;
        }
        case VM_OC_INIT_CLASS:
        {
          result = opfunc_init_class (frame_ctx_p, stack_top_p);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          continue;
        }
        case VM_OC_FINALIZE_CLASS:
        {
          JERRY_ASSERT (opcode == CBC_EXT_FINALIZE_NAMED_CLASS || opcode == CBC_EXT_FINALIZE_ANONYMOUS_CLASS);

          if (opcode == CBC_EXT_FINALIZE_NAMED_CLASS)
          {
            uint16_t literal_index;
            READ_LITERAL_INDEX (literal_index);
            left_value = literal_start_p[literal_index];
          }

          opfunc_finalize_class (frame_ctx_p, &stack_top_p, left_value);
          continue;
        }
        case VM_OC_SET_FIELD_INIT:
        {
          ecma_string_t *property_name_p = ecma_get_magic_string (LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_INIT);
          ecma_object_t *proto_object_p = ecma_get_object_from_value (stack_top_p[-1]);
          ecma_object_t *class_object_p = ecma_get_object_from_value (stack_top_p[-2]);
          ecma_object_t *initializer_func_p = ecma_get_object_from_value (left_value);

          opfunc_bind_class_environment (frame_ctx_p->lex_env_p, proto_object_p, class_object_p, initializer_func_p);

          ecma_property_value_t *property_value_p =
            ecma_create_named_data_property (class_object_p, property_name_p, ECMA_PROPERTY_FIXED, NULL);
          property_value_p->value = left_value;

          property_name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_COMPUTED);
          ecma_property_t *property_p = ecma_find_named_property (class_object_p, property_name_p);

          if (property_p != NULL)
          {
            property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
            ecma_value_t *compact_collection_p =
              ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, property_value_p->value);
            compact_collection_p = ecma_compact_collection_shrink (compact_collection_p);
            ECMA_SET_INTERNAL_VALUE_POINTER (property_value_p->value, compact_collection_p);
          }

          goto free_left_value;
        }
        case VM_OC_RUN_FIELD_INIT:
        {
          JERRY_ASSERT (frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_NON_ARROW_FUNC);
          result = opfunc_init_class_fields (frame_ctx_p->shared_p->function_object_p, frame_ctx_p->this_binding);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          continue;
        }
        case VM_OC_RUN_STATIC_FIELD_INIT:
        {
          left_value = stack_top_p[-2];
          stack_top_p[-2] = stack_top_p[-1];
          stack_top_p--;

          result = opfunc_init_static_class_fields (left_value, stack_top_p[-1]);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          goto free_left_value;
        }
        case VM_OC_SET_NEXT_COMPUTED_FIELD:
        {
          ecma_integer_value_t next_index = ecma_get_integer_from_value (stack_top_p[-2]) + 1;
          stack_top_p[-2] = ecma_make_integer_value (next_index);

          JERRY_ASSERT (frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_HAS_CLASS_FIELDS);

          ecma_value_t *computed_class_fields_p = VM_GET_COMPUTED_CLASS_FIELDS (frame_ctx_p);
          JERRY_ASSERT ((ecma_value_t) next_index < ECMA_COMPACT_COLLECTION_GET_SIZE (computed_class_fields_p));
          ecma_value_t prop_name = computed_class_fields_p[next_index];

          if (opcode == CBC_EXT_SET_NEXT_COMPUTED_FIELD_ANONYMOUS_FUNC)
          {
            ecma_object_t *func_obj_p = ecma_get_object_from_value (stack_top_p[-1]);

            JERRY_ASSERT (ecma_find_named_property (func_obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_NAME)) == NULL);
            ecma_property_value_t *value_p;
            value_p = ecma_create_named_data_property (func_obj_p,
                                                       ecma_get_magic_string (LIT_MAGIC_STRING_NAME),
                                                       ECMA_PROPERTY_FLAG_CONFIGURABLE,
                                                       NULL);

            if (ecma_get_object_type (func_obj_p) == ECMA_OBJECT_TYPE_FUNCTION)
            {
              ECMA_SET_SECOND_BIT_TO_POINTER_TAG (((ecma_extended_object_t *) func_obj_p)->u.function.scope_cp);
            }

            value_p->value = ecma_copy_value (prop_name);
          }

          result = opfunc_define_field (frame_ctx_p->this_binding, prop_name, stack_top_p[-1]);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          ecma_free_value (*(--stack_top_p));
          continue;
        }
        case VM_OC_PUSH_SUPER_CONSTRUCTOR:
        {
          result = ecma_op_function_get_super_constructor (vm_get_class_function (frame_ctx_p));

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          continue;
        }
        case VM_OC_RESOLVE_LEXICAL_THIS:
        {
          result = ecma_op_get_this_binding (frame_ctx_p->lex_env_p);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          continue;
        }
        case VM_OC_OBJECT_LITERAL_HOME_ENV:
        {
          if (opcode == CBC_EXT_PUSH_OBJECT_SUPER_ENVIRONMENT)
          {
            ecma_value_t obj_value = stack_top_p[-1];
            ecma_object_t *obj_env_p = ecma_create_lex_env_class (frame_ctx_p->lex_env_p, 0);

            ECMA_SET_NON_NULL_POINTER (obj_env_p->u1.bound_object_cp, ecma_get_object_from_value (obj_value));
            stack_top_p[-1] = ecma_make_object_value (obj_env_p);
            *stack_top_p++ = obj_value;
          }
          else
          {
            JERRY_ASSERT (opcode == CBC_EXT_POP_OBJECT_SUPER_ENVIRONMENT);
            ecma_deref_object (ecma_get_object_from_value (stack_top_p[-2]));
            stack_top_p[-2] = stack_top_p[-1];
            stack_top_p--;
          }
          continue;
        }
        case VM_OC_SET_HOME_OBJECT:
        {
          int offset = opcode == CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT_COMPUTED ? -1 : 0;
          opfunc_set_home_object (ecma_get_object_from_value (stack_top_p[-1]),
                                  ecma_get_object_from_value (stack_top_p[-3 + offset]));
          continue;
        }
        case VM_OC_SUPER_REFERENCE:
        {
          result = opfunc_form_super_reference (&stack_top_p, frame_ctx_p, left_value, opcode);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          goto free_left_value;
        }
        case VM_OC_SET_FUNCTION_NAME:
        {
          const char *prefix_p = NULL;
          lit_utf8_size_t prefix_size = 0;

          if (opcode != CBC_EXT_SET_FUNCTION_NAME)
          {
            ecma_value_t prop_name_value;

            if (opcode == CBC_EXT_SET_CLASS_NAME)
            {
              uint16_t literal_index;
              READ_LITERAL_INDEX (literal_index);
              prop_name_value = literal_start_p[literal_index];
            }
            else
            {
              prop_name_value = stack_top_p[-2];
            }

            ecma_string_t *prop_name_p = ecma_op_to_property_key (prop_name_value);

            if (JERRY_UNLIKELY (prop_name_p == NULL))
            {
              result = ECMA_VALUE_ERROR;
              goto error;
            }

            left_value = ecma_make_prop_name_value (prop_name_p);

            if (opcode != CBC_EXT_SET_CLASS_NAME)
            {
              ecma_ref_ecma_string (prop_name_p);
              ecma_free_value (stack_top_p[-2]);
              stack_top_p[-2] = left_value;
            }

            if (opcode == CBC_EXT_SET_COMPUTED_GETTER_NAME || opcode == CBC_EXT_SET_COMPUTED_SETTER_NAME)
            {
              prefix_p = (opcode == CBC_EXT_SET_COMPUTED_GETTER_NAME) ? "get " : "set ";
              prefix_size = 4;
            }
          }

          ecma_object_t *func_obj_p = ecma_get_object_from_value (stack_top_p[-1]);

          if (ecma_find_named_property (func_obj_p, ecma_get_magic_string (LIT_MAGIC_STRING_NAME)) != NULL)
          {
            ecma_free_value (left_value);
            continue;
          }

          ecma_property_value_t *value_p;
          value_p = ecma_create_named_data_property (func_obj_p,
                                                     ecma_get_magic_string (LIT_MAGIC_STRING_NAME),
                                                     ECMA_PROPERTY_FLAG_CONFIGURABLE,
                                                     NULL);

          if (ecma_get_object_type (func_obj_p) == ECMA_OBJECT_TYPE_FUNCTION)
          {
            ECMA_SET_SECOND_BIT_TO_POINTER_TAG (((ecma_extended_object_t *) func_obj_p)->u.function.scope_cp);
          }

          value_p->value =
            ecma_op_function_form_name (ecma_get_prop_name_from_value (left_value), (char*) prefix_p, prefix_size);
          ecma_free_value (left_value);
          continue;
        }
        case VM_OC_PUSH_SPREAD_ELEMENT:
        {
          *stack_top_p++ = ECMA_VALUE_SPREAD_ELEMENT;
          continue;
        }
        case VM_OC_PUSH_REST_OBJECT:
        {
          vm_frame_ctx_shared_t *shared_p = frame_ctx_p->shared_p;

          JERRY_ASSERT (shared_p->status_flags & VM_FRAME_CTX_SHARED_HAS_ARG_LIST);

          const ecma_value_t *arg_list_p = ((vm_frame_ctx_shared_args_t *) shared_p)->arg_list_p;
          uint32_t arg_list_len = ((vm_frame_ctx_shared_args_t *) shared_p)->arg_list_len;
          uint16_t argument_end;

          if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
          {
            argument_end = ((cbc_uint16_arguments_t *) bytecode_header_p)->argument_end;
          }
          else
          {
            argument_end = ((cbc_uint8_arguments_t *) bytecode_header_p)->argument_end;
          }

          if (arg_list_len < argument_end)
          {
            arg_list_len = argument_end;
          }

          result = ecma_op_new_array_object_from_buffer (arg_list_p + argument_end, arg_list_len - argument_end);

          JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (result));
          *stack_top_p++ = result;
          continue;
        }
        case VM_OC_ITERATOR_CONTEXT_CREATE:
        {
          result = ecma_op_get_iterator (stack_top_p[-1], ECMA_VALUE_SYNC_ITERATOR, &left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          uint32_t context_size =
            (uint32_t) (stack_top_p + PARSER_ITERATOR_CONTEXT_STACK_ALLOCATION - VM_LAST_CONTEXT_END ());
          stack_top_p += PARSER_ITERATOR_CONTEXT_STACK_ALLOCATION;
          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, context_size);

          stack_top_p[-1] = VM_CREATE_CONTEXT (VM_CONTEXT_ITERATOR, context_size) | VM_CONTEXT_CLOSE_ITERATOR;
          stack_top_p[-2] = result;
          stack_top_p[-3] = left_value;

          continue;
        }
        case VM_OC_ITERATOR_STEP:
        {
          ecma_value_t *last_context_end_p = VM_LAST_CONTEXT_END ();

          ecma_value_t iterator = last_context_end_p[-2];
          ecma_value_t next_method = last_context_end_p[-3];

          result = ecma_op_iterator_step (iterator, next_method);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            last_context_end_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
            goto error;
          }

          ecma_value_t value = ECMA_VALUE_UNDEFINED;

          if (!ecma_is_value_false (result))
          {
            value = ecma_op_iterator_value (result);
            ecma_free_value (result);

            if (ECMA_IS_VALUE_ERROR (value))
            {
              last_context_end_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
              result = value;
              goto error;
            }
          }
          else
          {
            last_context_end_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
          }

          *stack_top_p++ = value;
          continue;
        }
        case VM_OC_ITERATOR_CONTEXT_END:
        {
          JERRY_ASSERT (VM_LAST_CONTEXT_END () == stack_top_p);

          if (stack_top_p[-1] & VM_CONTEXT_CLOSE_ITERATOR)
          {
            stack_top_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
            result = ecma_op_iterator_close (stack_top_p[-2]);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              goto error;
            }
          }

          stack_top_p =
            vm_stack_context_abort_variable_length (frame_ctx_p, stack_top_p, PARSER_ITERATOR_CONTEXT_STACK_ALLOCATION);
          continue;
        }
        case VM_OC_DEFAULT_INITIALIZER:
        {
          JERRY_ASSERT (stack_top_p > VM_GET_REGISTERS (frame_ctx_p) + register_end);

          if (stack_top_p[-1] != ECMA_VALUE_UNDEFINED)
          {
            byte_code_p = byte_code_start_p + branch_offset;
            continue;
          }

          stack_top_p--;
          continue;
        }
        case VM_OC_REST_INITIALIZER:
        {
          ecma_object_t *array_p = ecma_op_new_array_object (0);
          JERRY_ASSERT (ecma_op_object_is_fast_array (array_p));

          ecma_value_t *last_context_end_p = VM_LAST_CONTEXT_END ();
          ecma_value_t iterator = last_context_end_p[-2];
          ecma_value_t next_method = last_context_end_p[-3];
          uint32_t index = 0;

          while (true)
          {
            result = ecma_op_iterator_step (iterator, next_method);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              last_context_end_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
              ecma_deref_object (array_p);
              goto error;
            }

            if (ecma_is_value_false (result))
            {
              last_context_end_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
              break;
            }

            ecma_value_t value = ecma_op_iterator_value (result);
            ecma_free_value (result);

            if (ECMA_IS_VALUE_ERROR (value))
            {
              ecma_deref_object (array_p);
              result = value;
              goto error;
            }

            ecma_fast_array_set_property (array_p, index++, value);
            ecma_free_value (value);
          }

          *stack_top_p++ = ecma_make_object_value (array_p);
          continue;
        }
        case VM_OC_OBJ_INIT_CONTEXT_CREATE:
        {
          left_value = stack_top_p[-1];
          vm_stack_context_type_t context_type = VM_CONTEXT_OBJ_INIT;
          uint32_t context_stack_allocation = PARSER_OBJ_INIT_CONTEXT_STACK_ALLOCATION;

          if (opcode == CBC_EXT_OBJ_INIT_REST_CONTEXT_CREATE)
          {
            context_type = VM_CONTEXT_OBJ_INIT_REST;
            context_stack_allocation = PARSER_OBJ_INIT_REST_CONTEXT_STACK_ALLOCATION;
          }

          uint32_t context_size = (uint32_t) (stack_top_p + context_stack_allocation - VM_LAST_CONTEXT_END ());
          stack_top_p += context_stack_allocation;
          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, context_size);

          stack_top_p[-1] = VM_CREATE_CONTEXT (context_type, context_size);
          stack_top_p[-2] = left_value;

          if (context_type == VM_CONTEXT_OBJ_INIT_REST)
          {
            stack_top_p[-3] = ecma_make_object_value (ecma_op_new_array_object (0));
          }
          continue;
        }
        case VM_OC_OBJ_INIT_CONTEXT_END:
        {
          JERRY_ASSERT (stack_top_p == VM_LAST_CONTEXT_END ());

          uint32_t context_stack_allocation = PARSER_OBJ_INIT_CONTEXT_STACK_ALLOCATION;

          if (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_OBJ_INIT_REST)
          {
            context_stack_allocation = PARSER_OBJ_INIT_REST_CONTEXT_STACK_ALLOCATION;
          }

          stack_top_p = vm_stack_context_abort_variable_length (frame_ctx_p, stack_top_p, context_stack_allocation);
          continue;
        }
        case VM_OC_OBJ_INIT_PUSH_REST:
        {
          ecma_value_t *last_context_end_p = VM_LAST_CONTEXT_END ();
          if (!ecma_op_require_object_coercible (last_context_end_p[-2]))
          {
            result = ECMA_VALUE_ERROR;
            goto error;
          }

          ecma_object_t *prototype_p = ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE);
          ecma_object_t *result_object_p = ecma_create_object (prototype_p, 0, ECMA_OBJECT_TYPE_GENERAL);

          left_value = ecma_make_object_value (result_object_p);
          result = opfunc_copy_data_properties (left_value, last_context_end_p[-2], last_context_end_p[-3]);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          ecma_free_value (last_context_end_p[-3]);
          last_context_end_p[-3] = last_context_end_p[-2];
          last_context_end_p[-2] = ECMA_VALUE_UNDEFINED;

          *stack_top_p++ = left_value;
          continue;
        }
        case VM_OC_INITIALIZER_PUSH_NAME:
        {
          if (JERRY_UNLIKELY (!ecma_is_value_prop_name (left_value)))
          {
            ecma_string_t *property_key = ecma_op_to_property_key (left_value);

            if (property_key == NULL)
            {
              result = ECMA_VALUE_ERROR;
              goto error;
            }

            ecma_free_value (left_value);
            left_value = ecma_make_string_value (property_key);
          }

          ecma_value_t *last_context_end_p = VM_LAST_CONTEXT_END ();
          ecma_object_t *array_obj_p = ecma_get_object_from_value (last_context_end_p[-3]);
          JERRY_ASSERT (ecma_get_object_type (array_obj_p) == ECMA_OBJECT_TYPE_ARRAY);

          ecma_extended_object_t *ext_array_obj_p = (ecma_extended_object_t *) array_obj_p;
          ecma_fast_array_set_property (array_obj_p, ext_array_obj_p->u.array.length, left_value);
          /* FALLTHRU */
        }
        case VM_OC_INITIALIZER_PUSH_PROP:
        {
          ecma_value_t *last_context_end_p = VM_LAST_CONTEXT_END ();
          ecma_value_t base = last_context_end_p[-2];

          if (opcode == CBC_EXT_INITIALIZER_PUSH_PROP)
          {
            left_value = *last_context_end_p++;
            while (last_context_end_p < stack_top_p)
            {
              last_context_end_p[-1] = *last_context_end_p;
              last_context_end_p++;
            }
            stack_top_p--;
          }

          result = vm_op_get_value (base, left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_left_value;
        }
        case VM_OC_SPREAD_ARGUMENTS:
        {
          uint8_t arguments_list_len = *byte_code_p++;
          stack_top_p -= arguments_list_len;

          ecma_collection_t *arguments_p = opfunc_spread_arguments (stack_top_p, arguments_list_len);

          if (JERRY_UNLIKELY (arguments_p == NULL))
          {
            result = ECMA_VALUE_ERROR;
            goto error;
          }

          stack_top_p++;
          ECMA_SET_INTERNAL_VALUE_POINTER (stack_top_p[-1], arguments_p);

          frame_ctx_p->call_operation = VM_EXEC_SPREAD_OP;
          frame_ctx_p->byte_code_p = byte_code_start_p;
          frame_ctx_p->stack_top_p = stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_CREATE_GENERATOR:
        {
          frame_ctx_p->call_operation = VM_EXEC_RETURN;
          frame_ctx_p->byte_code_p = byte_code_p;
          frame_ctx_p->stack_top_p = stack_top_p;

          vm_executable_object_t *executable_object_p;
          executable_object_p = opfunc_create_executable_object (frame_ctx_p, VM_CREATE_EXECUTABLE_OBJECT_GENERATOR);

          return ecma_make_object_value ((ecma_object_t *) executable_object_p);
        }
        case VM_OC_YIELD:
        {
          frame_ctx_p->call_operation = VM_EXEC_RETURN;
          frame_ctx_p->byte_code_p = byte_code_p;
          frame_ctx_p->stack_top_p = --stack_top_p;
          return *stack_top_p;
        }
        case VM_OC_ASYNC_YIELD:
        {
          ecma_extended_object_t *async_generator_object_p = VM_GET_EXECUTABLE_OBJECT (frame_ctx_p);

          opfunc_async_generator_yield (async_generator_object_p, stack_top_p[-1]);

          frame_ctx_p->call_operation = VM_EXEC_RETURN;
          frame_ctx_p->byte_code_p = byte_code_p;
          frame_ctx_p->stack_top_p = --stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_ASYNC_YIELD_ITERATOR:
        {
          ecma_extended_object_t *async_generator_object_p = VM_GET_EXECUTABLE_OBJECT (frame_ctx_p);

          JERRY_ASSERT (
            !(async_generator_object_p->u.cls.u2.executable_obj_flags & ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD));

          /* Byte code is executed at the first time. */
          left_value = stack_top_p[-1];
          result = ecma_op_get_iterator (left_value, ECMA_VALUE_ASYNC_ITERATOR, stack_top_p - 1);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          ecma_free_value (left_value);
          left_value = result;
          result = ecma_op_iterator_next (left_value, stack_top_p[-1], ECMA_VALUE_UNDEFINED);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          result = ecma_promise_async_await (async_generator_object_p, result);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          async_generator_object_p->u.cls.u2.executable_obj_flags |= ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD;
          *VM_GET_EXECUTABLE_ITERATOR (frame_ctx_p) = left_value;

          frame_ctx_p->call_operation = VM_EXEC_RETURN;
          frame_ctx_p->byte_code_p = byte_code_p;
          frame_ctx_p->stack_top_p = stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_AWAIT:
        {
          if (JERRY_UNLIKELY (!(frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_EXECUTABLE)))
          {
            frame_ctx_p->call_operation = VM_EXEC_RETURN;
            frame_ctx_p->byte_code_p = byte_code_p;
            frame_ctx_p->stack_top_p = --stack_top_p;

            result = opfunc_async_create_and_await (frame_ctx_p, *stack_top_p, 0);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              goto error;
            }
            return result;
          }
          /* FALLTHRU */
        }
        case VM_OC_GENERATOR_AWAIT:
        {
          ecma_extended_object_t *async_generator_object_p = VM_GET_EXECUTABLE_OBJECT (frame_ctx_p);

          result = ecma_promise_async_await (async_generator_object_p, *(--stack_top_p));

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          frame_ctx_p->call_operation = VM_EXEC_RETURN;
          frame_ctx_p->byte_code_p = byte_code_p;
          frame_ctx_p->stack_top_p = stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_EXT_RETURN:
        {
          result = left_value;
          left_value = ECMA_VALUE_UNDEFINED;

          ecma_value_t *stack_bottom_p = VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth;

          while (stack_top_p > stack_bottom_p)
          {
            ecma_fast_free_value (*(--stack_top_p));
          }

          goto error;
        }
        case VM_OC_ASYNC_EXIT:
        {
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          if (!(frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_EXECUTABLE))
          {
            result = ecma_op_create_promise_object (ECMA_VALUE_EMPTY, ECMA_VALUE_UNDEFINED, NULL);
          }
          else
          {
            result = *VM_GET_EXECUTABLE_ITERATOR (frame_ctx_p);
            *VM_GET_EXECUTABLE_ITERATOR (frame_ctx_p) = ECMA_VALUE_UNDEFINED;
          }

          vm_stack_context_type_t context_type = VM_GET_CONTEXT_TYPE (stack_top_p[-1]);

          if (context_type == VM_CONTEXT_TRY)
          {
            JERRY_ASSERT (frame_ctx_p->context_depth == PARSER_TRY_CONTEXT_STACK_ALLOCATION);
            left_value = ECMA_VALUE_UNDEFINED;
          }
          else
          {
            JERRY_ASSERT (frame_ctx_p->context_depth == PARSER_FINALLY_CONTEXT_STACK_ALLOCATION);
            left_value = stack_top_p[-2];
          }

          if (context_type == VM_CONTEXT_FINALLY_THROW)
          {
            ecma_reject_promise (result, left_value);
          }
          else
          {
            JERRY_ASSERT (context_type == VM_CONTEXT_TRY || context_type == VM_CONTEXT_FINALLY_RETURN);
            ecma_fulfill_promise (result, left_value);
          }

          ecma_free_value (left_value);

          frame_ctx_p->context_depth = 0;
          frame_ctx_p->call_operation = VM_NO_EXEC_OP;
          return result;
        }
        case VM_OC_STRING_CONCAT:
        {
          ecma_string_t *left_str_p = ecma_op_to_string (left_value);

          if (JERRY_UNLIKELY (left_str_p == NULL))
          {
            result = ECMA_VALUE_ERROR;
            goto error;
          }
          ecma_string_t *right_str_p = ecma_op_to_string (right_value);

          if (JERRY_UNLIKELY (right_str_p == NULL))
          {
            ecma_deref_ecma_string (left_str_p);
            result = ECMA_VALUE_ERROR;
            goto error;
          }

          ecma_string_t *result_str_p = ecma_concat_ecma_strings (left_str_p, right_str_p);
          ecma_deref_ecma_string (right_str_p);

          *stack_top_p++ = ecma_make_string_value (result_str_p);
          goto free_both_values;
        }
        case VM_OC_GET_TEMPLATE_OBJECT:
        {
          uint8_t tagged_idx = *byte_code_p++;
          ecma_collection_t *collection_p = ecma_compiled_code_get_tagged_template_collection (bytecode_header_p);
          JERRY_ASSERT (tagged_idx < collection_p->item_count);

          *stack_top_p++ = ecma_copy_value (collection_p->buffer_p[tagged_idx]);
          continue;
        }
        case VM_OC_PUSH_NEW_TARGET:
        {
          ecma_object_t *new_target_object_p = JERRY_CONTEXT (current_new_target_p);
          if (new_target_object_p == NULL)
          {
            *stack_top_p++ = ECMA_VALUE_UNDEFINED;
          }
          else
          {
            ecma_ref_object (new_target_object_p);
            *stack_top_p++ = ecma_make_object_value (new_target_object_p);
          }
          continue;
        }
        case VM_OC_REQUIRE_OBJECT_COERCIBLE:
        {
          if (!ecma_op_require_object_coercible (stack_top_p[-1]))
          {
            result = ECMA_VALUE_ERROR;
            goto error;
          }
          continue;
        }
        case VM_OC_ASSIGN_SUPER:
        {
          result = opfunc_assign_super_reference (&stack_top_p, frame_ctx_p, opcode_data);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          continue;
        }
        case VM_OC_PUSH_ELISON:
        {
          *stack_top_p++ = ECMA_VALUE_ARRAY_HOLE;
          continue;
        }
        case VM_OC_APPEND_ARRAY:
        {
          uint16_t values_length = *byte_code_p++;
          stack_top_p -= values_length;

          if (*byte_code_start_p == CBC_EXT_OPCODE)
          {
            values_length = (uint16_t) (values_length | OPFUNC_HAS_SPREAD_ELEMENT);
          }
          result = opfunc_append_array (stack_top_p, values_length);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          continue;
        }
        case VM_OC_IDENT_REFERENCE:
        {
          uint16_t literal_index;

          READ_LITERAL_INDEX (literal_index);

          JERRY_ASSERT (literal_index < ident_end);

          if (literal_index < register_end)
          {
            *stack_top_p++ = ECMA_VALUE_REGISTER_REF;
            *stack_top_p++ = ecma_make_integer_value (literal_index);
            *stack_top_p++ = ecma_fast_copy_value (VM_GET_REGISTER (frame_ctx_p, literal_index));
          }
          else
          {
            ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

            ecma_object_t *ref_base_lex_env_p;

            result = ecma_op_get_value_lex_env_base (frame_ctx_p->lex_env_p, &ref_base_lex_env_p, name_p);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              goto error;
            }

            ecma_ref_object (ref_base_lex_env_p);
            ecma_ref_ecma_string (name_p);
            *stack_top_p++ = ecma_make_object_value (ref_base_lex_env_p);
            *stack_top_p++ = ecma_make_string_value (name_p);
            *stack_top_p++ = result;
          }
          continue;
        }
        case VM_OC_PROP_GET:
        {
          result = vm_op_get_value (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_PROP_REFERENCE:
        {
          /* Forms with reference requires preserving the base and offset. */

          if (opcode == CBC_PUSH_PROP_REFERENCE)
          {
            left_value = stack_top_p[-2];
            right_value = stack_top_p[-1];
          }
          else if (opcode == CBC_PUSH_PROP_LITERAL_REFERENCE)
          {
            *stack_top_p++ = left_value;
            right_value = left_value;
            left_value = stack_top_p[-2];
          }
          else
          {
            JERRY_ASSERT (opcode == CBC_PUSH_PROP_LITERAL_LITERAL_REFERENCE
                          || opcode == CBC_PUSH_PROP_THIS_LITERAL_REFERENCE);
            *stack_top_p++ = left_value;
            *stack_top_p++ = right_value;
          }
          /* FALLTHRU */
        }
        case VM_OC_PROP_PRE_INCR:
        case VM_OC_PROP_PRE_DECR:
        case VM_OC_PROP_POST_INCR:
        case VM_OC_PROP_POST_DECR:
        {
          result = vm_op_get_value (left_value, right_value);

          if (opcode < CBC_PRE_INCR)
          {
            left_value = ECMA_VALUE_UNDEFINED;
            right_value = ECMA_VALUE_UNDEFINED;
          }

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          if (opcode < CBC_PRE_INCR)
          {
            break;
          }

          stack_top_p += 2;
          left_value = result;
          right_value = ECMA_VALUE_UNDEFINED;
          /* FALLTHRU */
        }
        case VM_OC_PRE_INCR:
        case VM_OC_PRE_DECR:
        case VM_OC_POST_INCR:
        case VM_OC_POST_DECR:
        {
          uint32_t opcode_flags = VM_OC_GROUP_GET_INDEX (opcode_data) - VM_OC_PROP_PRE_INCR;
          ecma_number_t result_number;

          byte_code_p = byte_code_start_p + 1;

          if (ecma_is_value_integer_number (left_value))
          {
            result = left_value;
            left_value = ECMA_VALUE_UNDEFINED;

            ecma_integer_value_t int_value = (ecma_integer_value_t) result;
            ecma_integer_value_t int_increase = 0;

            if (opcode_flags & VM_OC_DECREMENT_OPERATOR_FLAG)
            {
              if (int_value > ECMA_INTEGER_NUMBER_MIN_SHIFTED)
              {
                int_increase = -(1 << ECMA_DIRECT_SHIFT);
              }
            }
            else if (int_value < ECMA_INTEGER_NUMBER_MAX_SHIFTED)
            {
              int_increase = 1 << ECMA_DIRECT_SHIFT;
            }

            if (JERRY_LIKELY (int_increase != 0))
            {
              /* Postfix operators require the unmodified number value. */
              if (opcode_flags & VM_OC_POST_INCR_DECR_OPERATOR_FLAG)
              {
                POST_INCREASE_DECREASE_PUT_RESULT (result);
              }

              result = (ecma_value_t) (int_value + int_increase);
              break;
            }
            result_number = (ecma_number_t) ecma_get_integer_from_value (result);
          }
          else if (ecma_is_value_float_number (left_value))
          {
            result = left_value;
            left_value = ECMA_VALUE_UNDEFINED;
            result_number = ecma_get_number_from_value (result);
          }
          else
          {
            result = ecma_op_to_numeric (left_value, &result_number, ECMA_TO_NUMERIC_ALLOW_BIGINT);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              goto error;
            }

            ecma_free_value (left_value);
            left_value = ECMA_VALUE_UNDEFINED;

#if JERRY_BUILTIN_BIGINT
            if (JERRY_UNLIKELY (ecma_is_value_bigint (result)))
            {
              ecma_bigint_unary_operation_type operation_type = ECMA_BIGINT_UNARY_INCREASE;

              if (opcode_flags & VM_OC_DECREMENT_OPERATOR_FLAG)
              {
                operation_type = ECMA_BIGINT_UNARY_DECREASE;
              }

              /* Postfix operators require the unmodified number value. */
              if (opcode_flags & VM_OC_POST_INCR_DECR_OPERATOR_FLAG)
              {
                POST_INCREASE_DECREASE_PUT_RESULT (result);

                result = ecma_bigint_unary (result, operation_type);
              }
              else
              {
                ecma_value_t original_value = result;
                result = ecma_bigint_unary (original_value, operation_type);
                ecma_free_value (original_value);
              }

              if (ECMA_IS_VALUE_ERROR (result))
              {
                goto error;
              }
              break;
            }
#endif /* JERRY_BUILTIN_BIGINT */

            result = ecma_make_number_value (result_number);
          }

          ecma_number_t increase = ECMA_NUMBER_ONE;

          if (opcode_flags & VM_OC_DECREMENT_OPERATOR_FLAG)
          {
            /* For decrement operators */
            increase = ECMA_NUMBER_MINUS_ONE;
          }

          /* Postfix operators require the unmodified number value. */
          if (opcode_flags & VM_OC_POST_INCR_DECR_OPERATOR_FLAG)
          {
            POST_INCREASE_DECREASE_PUT_RESULT (result);

            result = ecma_make_number_value (result_number + increase);
            break;
          }

          if (ecma_is_value_integer_number (result))
          {
            result = ecma_make_number_value (result_number + increase);
          }
          else
          {
            result = ecma_update_float_number (result, result_number + increase);
          }
          break;
        }
        case VM_OC_ASSIGN:
        {
          result = left_value;
          left_value = ECMA_VALUE_UNDEFINED;
          break;
        }
        case VM_OC_MOV_IDENT:
        {
          uint32_t literal_index;

          READ_LITERAL_INDEX (literal_index);

          JERRY_ASSERT (literal_index < register_end);
          JERRY_ASSERT (!(opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK)));

          ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, literal_index));
          VM_GET_REGISTER (frame_ctx_p, literal_index) = left_value;
          continue;
        }
        case VM_OC_ASSIGN_PROP:
        {
          result = stack_top_p[-1];
          stack_top_p[-1] = left_value;
          left_value = ECMA_VALUE_UNDEFINED;
          break;
        }
        case VM_OC_ASSIGN_PROP_THIS:
        {
          result = stack_top_p[-1];
          stack_top_p[-1] = ecma_copy_value (frame_ctx_p->this_binding);
          *stack_top_p++ = left_value;
          left_value = ECMA_VALUE_UNDEFINED;
          break;
        }
        case VM_OC_RETURN_FUNCTION_END:
        {
          if (CBC_FUNCTION_GET_TYPE (bytecode_header_p->status_flags) == CBC_FUNCTION_SCRIPT)
          {
            result = VM_GET_REGISTER (frame_ctx_p, 0);
            VM_GET_REGISTERS (frame_ctx_p)[0] = ECMA_VALUE_UNDEFINED;
          }
          else
          {
            result = ECMA_VALUE_UNDEFINED;
          }

          goto error;
        }
        case VM_OC_RETURN:
        {
          JERRY_ASSERT (opcode == CBC_RETURN || opcode == CBC_RETURN_WITH_LITERAL);

          result = left_value;
          left_value = ECMA_VALUE_UNDEFINED;
          goto error;
        }
        case VM_OC_THROW:
        {
          jcontext_raise_exception (left_value);

          result = ECMA_VALUE_ERROR;
          left_value = ECMA_VALUE_UNDEFINED;
          goto error;
        }
        case VM_OC_THROW_REFERENCE_ERROR:
        {
          result = ecma_raise_reference_error (ECMA_ERR_UNDEFINED_REFERENCE);
          goto error;
        }
        case VM_OC_EVAL:
        {
          JERRY_CONTEXT (status_flags) |= ECMA_STATUS_DIRECT_EVAL;
          JERRY_ASSERT ((*byte_code_p >= CBC_CALL && *byte_code_p <= CBC_CALL2_PROP_BLOCK)
                        || (*byte_code_p == CBC_EXT_OPCODE && byte_code_p[1] >= CBC_EXT_SPREAD_CALL
                            && byte_code_p[1] <= CBC_EXT_SPREAD_CALL_PROP_BLOCK));
          continue;
        }
        case VM_OC_CALL:
        {
          frame_ctx_p->call_operation = VM_EXEC_CALL;
          frame_ctx_p->byte_code_p = byte_code_start_p;
          frame_ctx_p->stack_top_p = stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_NEW:
        {
          frame_ctx_p->call_operation = VM_EXEC_CONSTRUCT;
          frame_ctx_p->byte_code_p = byte_code_start_p;
          frame_ctx_p->stack_top_p = stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_ERROR:
        {
          JERRY_ASSERT (frame_ctx_p->byte_code_p[1] == CBC_EXT_ERROR);

          result = ECMA_VALUE_ERROR;
          goto error;
        }
        case VM_OC_RESOLVE_BASE_FOR_CALL:
        {
          ecma_value_t this_value = stack_top_p[-3];

          if (this_value == ECMA_VALUE_REGISTER_REF)
          {
            /* Lexical environment cannot be 'this' value. */
            stack_top_p[-2] = ECMA_VALUE_UNDEFINED;
            stack_top_p[-3] = ECMA_VALUE_UNDEFINED;
          }
          else if (vm_get_implicit_this_value (&this_value))
          {
            ecma_free_value (stack_top_p[-3]);
            stack_top_p[-3] = this_value;
          }

          continue;
        }
        case VM_OC_PROP_DELETE:
        {
          result = vm_op_delete_prop (left_value, right_value, is_strict);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          JERRY_ASSERT (ecma_is_value_boolean (result));

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_DELETE:
        {
          uint16_t literal_index;

          READ_LITERAL_INDEX (literal_index);

          if (literal_index < register_end)
          {
            *stack_top_p++ = ECMA_VALUE_FALSE;
            continue;
          }

          result = vm_op_delete_var (literal_start_p[literal_index], frame_ctx_p->lex_env_p);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          JERRY_ASSERT (ecma_is_value_boolean (result));

          *stack_top_p++ = result;
          continue;
        }
        case VM_OC_JUMP:
        {
          byte_code_p = byte_code_start_p + branch_offset;
          continue;
        }
        case VM_OC_BRANCH_IF_STRICT_EQUAL:
        {
          ecma_value_t value = *(--stack_top_p);

          JERRY_ASSERT (stack_top_p > VM_GET_REGISTERS (frame_ctx_p) + register_end);

          if (ecma_op_strict_equality_compare (value, stack_top_p[-1]))
          {
            byte_code_p = byte_code_start_p + branch_offset;
            ecma_free_value (*--stack_top_p);
          }
          ecma_free_value (value);
          continue;
        }
        case VM_OC_BRANCH_IF_TRUE:
        case VM_OC_BRANCH_IF_FALSE:
        case VM_OC_BRANCH_IF_LOGICAL_TRUE:
        case VM_OC_BRANCH_IF_LOGICAL_FALSE:
        {
          uint32_t opcode_flags = VM_OC_GROUP_GET_INDEX (opcode_data) - VM_OC_BRANCH_IF_TRUE;
          ecma_value_t value = *(--stack_top_p);

          bool boolean_value = ecma_op_to_boolean (value);

          if (opcode_flags & VM_OC_BRANCH_IF_FALSE_FLAG)
          {
            boolean_value = !boolean_value;
          }

          if (boolean_value)
          {
            byte_code_p = byte_code_start_p + branch_offset;
            if (opcode_flags & VM_OC_LOGICAL_BRANCH_FLAG)
            {
              /* "Push" the value back to the stack. */
              ++stack_top_p;
              continue;
            }
          }

          ecma_fast_free_value (value);
          continue;
        }
        case VM_OC_POP_REFERENCE:
        {
          ecma_free_value (stack_top_p[-2]);
          ecma_free_value (stack_top_p[-3]);
          stack_top_p[-3] = stack_top_p[-1];
          stack_top_p -= 2;
          continue;
        }
        case VM_OC_BRANCH_IF_NULLISH:
        {
          left_value = stack_top_p[-1];

          if (!ecma_is_value_null (left_value) && !ecma_is_value_undefined (left_value))
          {
            byte_code_p = byte_code_start_p + branch_offset;
            continue;
          }
          --stack_top_p;
          continue;
        }
        case VM_OC_PLUS:
        case VM_OC_MINUS:
        {
          result = opfunc_unary_operation (left_value, VM_OC_GROUP_GET_INDEX (opcode_data) == VM_OC_PLUS);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_left_value;
        }
        case VM_OC_NOT:
        {
          *stack_top_p++ = ecma_make_boolean_value (!ecma_op_to_boolean (left_value));
          JERRY_ASSERT (ecma_is_value_boolean (stack_top_p[-1]));
          goto free_left_value;
        }
        case VM_OC_BIT_NOT:
        {
          JERRY_STATIC_ASSERT ((ECMA_DIRECT_TYPE_MASK == ((1 << ECMA_DIRECT_SHIFT) - 1)),
                               direct_type_mask_must_fill_all_bits_before_the_value_starts);

          if (ecma_is_value_integer_number (left_value))
          {
            *stack_top_p++ = (~ECMA_DIRECT_TYPE_MASK) ^ left_value;
            goto free_left_value;
          }

          result = do_number_bitwise_not (left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_left_value;
        }
        case VM_OC_VOID:
        {
          *stack_top_p++ = ECMA_VALUE_UNDEFINED;
          goto free_left_value;
        }
        case VM_OC_TYPEOF_IDENT:
        {
          uint16_t literal_index;

          READ_LITERAL_INDEX (literal_index);

          JERRY_ASSERT (literal_index < ident_end);

          if (literal_index < register_end)
          {
            left_value = ecma_copy_value (VM_GET_REGISTER (frame_ctx_p, literal_index));
          }
          else
          {
            ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

            ecma_object_t *ref_base_lex_env_p;

            result = ecma_op_get_value_lex_env_base (frame_ctx_p->lex_env_p, &ref_base_lex_env_p, name_p);

            if (ref_base_lex_env_p == NULL)
            {
              jcontext_release_exception ();
              result = ECMA_VALUE_UNDEFINED;
            }
            else if (ECMA_IS_VALUE_ERROR (result))
            {
              goto error;
            }

            left_value = result;
          }
          /* FALLTHRU */
        }
        case VM_OC_TYPEOF:
        {
          result = opfunc_typeof (left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_left_value;
        }
        case VM_OC_ADD:
        {
          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = ecma_get_integer_from_value (left_value);
            ecma_integer_value_t right_integer = ecma_get_integer_from_value (right_value);
            *stack_top_p++ = ecma_make_int32_value ((int32_t) (left_integer + right_integer));
            continue;
          }

          if (ecma_is_value_float_number (left_value) && ecma_is_value_number (right_value))
          {
            ecma_number_t new_value =
              (ecma_get_float_from_value (left_value) + ecma_get_number_from_value (right_value));

            *stack_top_p++ = ecma_update_float_number (left_value, new_value);
            ecma_free_number (right_value);
            continue;
          }

          if (ecma_is_value_float_number (right_value) && ecma_is_value_integer_number (left_value))
          {
            ecma_number_t new_value =
              ((ecma_number_t) ecma_get_integer_from_value (left_value) + ecma_get_float_from_value (right_value));

            *stack_top_p++ = ecma_update_float_number (right_value, new_value);
            continue;
          }

          result = opfunc_addition (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_SUB:
        {
          JERRY_STATIC_ASSERT (ECMA_INTEGER_NUMBER_MAX * 2 <= INT32_MAX && ECMA_INTEGER_NUMBER_MIN * 2 >= INT32_MIN,
                               doubled_ecma_numbers_must_fit_into_int32_range);

          JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (left_value) && !ECMA_IS_VALUE_ERROR (right_value));

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = ecma_get_integer_from_value (left_value);
            ecma_integer_value_t right_integer = ecma_get_integer_from_value (right_value);
            *stack_top_p++ = ecma_make_int32_value ((int32_t) (left_integer - right_integer));
            continue;
          }

          if (ecma_is_value_float_number (left_value) && ecma_is_value_number (right_value))
          {
            ecma_number_t new_value =
              (ecma_get_float_from_value (left_value) - ecma_get_number_from_value (right_value));

            *stack_top_p++ = ecma_update_float_number (left_value, new_value);
            ecma_free_number (right_value);
            continue;
          }

          if (ecma_is_value_float_number (right_value) && ecma_is_value_integer_number (left_value))
          {
            ecma_number_t new_value =
              ((ecma_number_t) ecma_get_integer_from_value (left_value) - ecma_get_float_from_value (right_value));

            *stack_top_p++ = ecma_update_float_number (right_value, new_value);
            continue;
          }

          result = do_number_arithmetic (NUMBER_ARITHMETIC_SUBTRACTION, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_MUL:
        {
          JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (left_value) && !ECMA_IS_VALUE_ERROR (right_value));

          JERRY_STATIC_ASSERT (ECMA_INTEGER_MULTIPLY_MAX * ECMA_INTEGER_MULTIPLY_MAX <= ECMA_INTEGER_NUMBER_MAX
                                 && -(ECMA_INTEGER_MULTIPLY_MAX * ECMA_INTEGER_MULTIPLY_MAX) >= ECMA_INTEGER_NUMBER_MIN,
                               square_of_integer_multiply_max_must_fit_into_integer_value_range);

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = ecma_get_integer_from_value (left_value);
            ecma_integer_value_t right_integer = ecma_get_integer_from_value (right_value);

            if (-ECMA_INTEGER_MULTIPLY_MAX <= left_integer && left_integer <= ECMA_INTEGER_MULTIPLY_MAX
                && -ECMA_INTEGER_MULTIPLY_MAX <= right_integer && right_integer <= ECMA_INTEGER_MULTIPLY_MAX
                && left_integer != 0 && right_integer != 0)
            {
              *stack_top_p++ = ecma_integer_multiply (left_integer, right_integer);
              continue;
            }

            ecma_number_t multiply = (ecma_number_t) left_integer * (ecma_number_t) right_integer;
            *stack_top_p++ = ecma_make_number_value (multiply);
            continue;
          }

          if (ecma_is_value_float_number (left_value) && ecma_is_value_number (right_value))
          {
            ecma_number_t new_value =
              (ecma_get_float_from_value (left_value) * ecma_get_number_from_value (right_value));

            *stack_top_p++ = ecma_update_float_number (left_value, new_value);
            ecma_free_number (right_value);
            continue;
          }

          if (ecma_is_value_float_number (right_value) && ecma_is_value_integer_number (left_value))
          {
            ecma_number_t new_value =
              ((ecma_number_t) ecma_get_integer_from_value (left_value) * ecma_get_float_from_value (right_value));

            *stack_top_p++ = ecma_update_float_number (right_value, new_value);
            continue;
          }

          result = do_number_arithmetic (NUMBER_ARITHMETIC_MULTIPLICATION, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_DIV:
        {
          JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (left_value) && !ECMA_IS_VALUE_ERROR (right_value));

          result = do_number_arithmetic (NUMBER_ARITHMETIC_DIVISION, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_MOD:
        {
          JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (left_value) && !ECMA_IS_VALUE_ERROR (right_value));

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = ecma_get_integer_from_value (left_value);
            ecma_integer_value_t right_integer = ecma_get_integer_from_value (right_value);

            if (right_integer != 0)
            {
              ecma_integer_value_t mod_result = left_integer % right_integer;

              if (mod_result != 0 || left_integer >= 0)
              {
                *stack_top_p++ = ecma_make_integer_value (mod_result);
                continue;
              }
            }
          }

          result = do_number_arithmetic (NUMBER_ARITHMETIC_REMAINDER, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_EXP:
        {
          result = do_number_arithmetic (NUMBER_ARITHMETIC_EXPONENTIATION, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_EQUAL:
        {
          result = opfunc_equality (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_NOT_EQUAL:
        {
          result = opfunc_equality (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = ecma_invert_boolean_value (result);
          goto free_both_values;
        }
        case VM_OC_STRICT_EQUAL:
        {
          bool is_equal = ecma_op_strict_equality_compare (left_value, right_value);

          result = ecma_make_boolean_value (is_equal);

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_STRICT_NOT_EQUAL:
        {
          bool is_equal = ecma_op_strict_equality_compare (left_value, right_value);

          result = ecma_make_boolean_value (!is_equal);

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_BIT_OR:
        {
          JERRY_STATIC_ASSERT ((ECMA_DIRECT_TYPE_MASK == ((1 << ECMA_DIRECT_SHIFT) - 1)),
                               direct_type_mask_must_fill_all_bits_before_the_value_starts);

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            *stack_top_p++ = left_value | right_value;
            continue;
          }

          result = do_number_bitwise_logic (NUMBER_BITWISE_LOGIC_OR, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_BIT_XOR:
        {
          JERRY_STATIC_ASSERT ((ECMA_DIRECT_TYPE_MASK == ((1 << ECMA_DIRECT_SHIFT) - 1)),
                               direct_type_mask_must_fill_all_bits_before_the_value_starts);

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            *stack_top_p++ = left_value ^ right_value;
            continue;
          }

          result = do_number_bitwise_logic (NUMBER_BITWISE_LOGIC_XOR, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_BIT_AND:
        {
          JERRY_STATIC_ASSERT ((ECMA_DIRECT_TYPE_MASK == ((1 << ECMA_DIRECT_SHIFT) - 1)),
                               direct_type_mask_must_fill_all_bits_before_the_value_starts);

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            *stack_top_p++ = left_value & right_value;
            continue;
          }

          result = do_number_bitwise_logic (NUMBER_BITWISE_LOGIC_AND, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_LEFT_SHIFT:
        {
          JERRY_STATIC_ASSERT ((ECMA_DIRECT_TYPE_MASK == ((1 << ECMA_DIRECT_SHIFT) - 1)),
                               direct_type_mask_must_fill_all_bits_before_the_value_starts);

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = ecma_get_integer_from_value (left_value);
            ecma_integer_value_t right_integer = ecma_get_integer_from_value (right_value);

            *stack_top_p++ = ecma_make_int32_value ((int32_t) ((uint32_t) left_integer << (right_integer & 0x1f)));
            continue;
          }

          result = do_number_bitwise_logic (NUMBER_BITWISE_SHIFT_LEFT, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_RIGHT_SHIFT:
        {
          JERRY_STATIC_ASSERT ((ECMA_DIRECT_TYPE_MASK == ((1 << ECMA_DIRECT_SHIFT) - 1)),
                               direct_type_mask_must_fill_all_bits_before_the_value_starts);

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = ecma_get_integer_from_value (left_value);
            ecma_integer_value_t right_integer = ecma_get_integer_from_value (right_value);
            *stack_top_p++ = ecma_make_integer_value (left_integer >> (right_integer & 0x1f));
            continue;
          }

          result = do_number_bitwise_logic (NUMBER_BITWISE_SHIFT_RIGHT, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_UNS_RIGHT_SHIFT:
        {
          JERRY_STATIC_ASSERT ((ECMA_DIRECT_TYPE_MASK == ((1 << ECMA_DIRECT_SHIFT) - 1)),
                               direct_type_mask_must_fill_all_bits_before_the_value_starts);

          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            uint32_t left_uint32 = (uint32_t) ecma_get_integer_from_value (left_value);
            ecma_integer_value_t right_integer = ecma_get_integer_from_value (right_value);
            *stack_top_p++ = ecma_make_uint32_value (left_uint32 >> (right_integer & 0x1f));
            continue;
          }

          result = do_number_bitwise_logic (NUMBER_BITWISE_SHIFT_URIGHT, left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_LESS:
        {
          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            bool is_less = (ecma_integer_value_t) left_value < (ecma_integer_value_t) right_value;
#if !JERRY_VM_HALT
            /* This is a lookahead to the next opcode to improve performance.
             * If it is CBC_BRANCH_IF_TRUE_BACKWARD, execute it. */
            if (*byte_code_p <= CBC_BRANCH_IF_TRUE_BACKWARD_3 && *byte_code_p >= CBC_BRANCH_IF_TRUE_BACKWARD)
            {
              byte_code_start_p = byte_code_p++;
              branch_offset_length = CBC_BRANCH_OFFSET_LENGTH (*byte_code_start_p);
              JERRY_ASSERT (branch_offset_length >= 1 && branch_offset_length <= 3);

              if (is_less)
              {
                branch_offset = *(byte_code_p++);

                if (JERRY_UNLIKELY (branch_offset_length != 1))
                {
                  branch_offset <<= 8;
                  branch_offset |= *(byte_code_p++);
                  if (JERRY_UNLIKELY (branch_offset_length == 3))
                  {
                    branch_offset <<= 8;
                    branch_offset |= *(byte_code_p++);
                  }
                }

                /* Note: The opcode is a backward branch. */
                byte_code_p = byte_code_start_p - branch_offset;
              }
              else
              {
                byte_code_p += branch_offset_length;
              }

              continue;
            }
#endif /* !JERRY_VM_HALT */
            *stack_top_p++ = ecma_make_boolean_value (is_less);
            continue;
          }

          if (ecma_is_value_number (left_value) && ecma_is_value_number (right_value))
          {
            ecma_number_t left_number = ecma_get_number_from_value (left_value);
            ecma_number_t right_number = ecma_get_number_from_value (right_value);

            *stack_top_p++ = ecma_make_boolean_value (left_number < right_number);
            goto free_both_values;
          }

          result = opfunc_relation (left_value, right_value, true, false);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_GREATER:
        {
          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = (ecma_integer_value_t) left_value;
            ecma_integer_value_t right_integer = (ecma_integer_value_t) right_value;

            *stack_top_p++ = ecma_make_boolean_value (left_integer > right_integer);
            continue;
          }

          if (ecma_is_value_number (left_value) && ecma_is_value_number (right_value))
          {
            ecma_number_t left_number = ecma_get_number_from_value (left_value);
            ecma_number_t right_number = ecma_get_number_from_value (right_value);

            *stack_top_p++ = ecma_make_boolean_value (left_number > right_number);
            goto free_both_values;
          }

          result = opfunc_relation (left_value, right_value, false, false);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_LESS_EQUAL:
        {
          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = (ecma_integer_value_t) left_value;
            ecma_integer_value_t right_integer = (ecma_integer_value_t) right_value;

            *stack_top_p++ = ecma_make_boolean_value (left_integer <= right_integer);
            continue;
          }

          if (ecma_is_value_number (left_value) && ecma_is_value_number (right_value))
          {
            ecma_number_t left_number = ecma_get_number_from_value (left_value);
            ecma_number_t right_number = ecma_get_number_from_value (right_value);

            *stack_top_p++ = ecma_make_boolean_value (left_number <= right_number);
            goto free_both_values;
          }

          result = opfunc_relation (left_value, right_value, false, true);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_GREATER_EQUAL:
        {
          if (ecma_are_values_integer_numbers (left_value, right_value))
          {
            ecma_integer_value_t left_integer = (ecma_integer_value_t) left_value;
            ecma_integer_value_t right_integer = (ecma_integer_value_t) right_value;

            *stack_top_p++ = ecma_make_boolean_value (left_integer >= right_integer);
            continue;
          }

          if (ecma_is_value_number (left_value) && ecma_is_value_number (right_value))
          {
            ecma_number_t left_number = ecma_get_number_from_value (left_value);
            ecma_number_t right_number = ecma_get_number_from_value (right_value);

            *stack_top_p++ = ecma_make_boolean_value (left_number >= right_number);
            goto free_both_values;
          }

          result = opfunc_relation (left_value, right_value, true, true);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_IN:
        {
          result = opfunc_in (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_INSTANCEOF:
        {
          result = opfunc_instanceof (left_value, right_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          goto free_both_values;
        }
        case VM_OC_BLOCK_CREATE_CONTEXT:
        {
          ecma_value_t *stack_context_top_p;
          stack_context_top_p = VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth;

          JERRY_ASSERT (stack_context_top_p == stack_top_p || stack_context_top_p == stack_top_p - 1);

          if (byte_code_start_p[0] != CBC_EXT_OPCODE)
          {
            branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

            if (stack_context_top_p != stack_top_p)
            {
              /* Preserve the value of switch statement. */
              stack_context_top_p[1] = stack_context_top_p[0];
            }

            stack_context_top_p[0] = VM_CREATE_CONTEXT_WITH_ENV (VM_CONTEXT_BLOCK, branch_offset);

            VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION);
            stack_top_p += PARSER_BLOCK_CONTEXT_STACK_ALLOCATION;
          }
          else
          {
            JERRY_ASSERT (byte_code_start_p[1] == CBC_EXT_TRY_CREATE_ENV);

            JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_context_top_p[-1]) == VM_CONTEXT_TRY
                          || VM_GET_CONTEXT_TYPE (stack_context_top_p[-1]) == VM_CONTEXT_CATCH
                          || VM_GET_CONTEXT_TYPE (stack_context_top_p[-1]) == VM_CONTEXT_FINALLY_JUMP
                          || VM_GET_CONTEXT_TYPE (stack_context_top_p[-1]) == VM_CONTEXT_FINALLY_THROW
                          || VM_GET_CONTEXT_TYPE (stack_context_top_p[-1]) == VM_CONTEXT_FINALLY_RETURN);

            JERRY_ASSERT (!(stack_context_top_p[-1] & VM_CONTEXT_HAS_LEX_ENV));

            stack_context_top_p[-1] |= VM_CONTEXT_HAS_LEX_ENV;
          }

          frame_ctx_p->lex_env_p = ecma_create_decl_lex_env (frame_ctx_p->lex_env_p);
          frame_ctx_p->lex_env_p->type_flags_refs |= ECMA_OBJECT_FLAG_BLOCK;

          continue;
        }
        case VM_OC_WITH:
        {
          ecma_value_t value = *(--stack_top_p);
          ecma_object_t *object_p;
          ecma_object_t *with_env_p;

          branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          result = ecma_op_to_object (value);
          ecma_free_value (value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          object_p = ecma_get_object_from_value (result);

          with_env_p = ecma_create_object_lex_env (frame_ctx_p->lex_env_p, object_p);
          ecma_deref_object (object_p);

          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_WITH_CONTEXT_STACK_ALLOCATION);
          stack_top_p += PARSER_WITH_CONTEXT_STACK_ALLOCATION;

          stack_top_p[-1] = VM_CREATE_CONTEXT_WITH_ENV (VM_CONTEXT_WITH, branch_offset);

          with_env_p->type_flags_refs |= ECMA_OBJECT_FLAG_BLOCK;
          frame_ctx_p->lex_env_p = with_env_p;
          continue;
        }
        case VM_OC_FOR_IN_INIT:
        {
          ecma_value_t value = *(--stack_top_p);

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          ecma_value_t expr_obj_value = ECMA_VALUE_UNDEFINED;
          ecma_collection_t *prop_names_p = opfunc_for_in (value, &expr_obj_value);
          ecma_free_value (value);

          if (prop_names_p == NULL)
          {
            if (JERRY_UNLIKELY (ECMA_IS_VALUE_ERROR (expr_obj_value)))
            {
              result = expr_obj_value;
              goto error;
            }

            /* The collection is already released */
            byte_code_p = byte_code_start_p + branch_offset;
            continue;
          }

          branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION);
          stack_top_p += PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION;
          stack_top_p[-1] = VM_CREATE_CONTEXT (VM_CONTEXT_FOR_IN, branch_offset);
          ECMA_SET_INTERNAL_VALUE_ANY_POINTER (stack_top_p[-2], prop_names_p);
          stack_top_p[-3] = 0;
          stack_top_p[-4] = expr_obj_value;

          if (byte_code_p[0] == CBC_EXT_OPCODE && byte_code_p[1] == CBC_EXT_CLONE_CONTEXT)
          {
            /* No need to duplicate the first context. */
            byte_code_p += 2;
          }

          continue;
        }
        case VM_OC_FOR_IN_GET_NEXT:
        {
          ecma_value_t *context_top_p = VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth;

          ecma_collection_t *collection_p;
          collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, context_top_p[-2]);

          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (context_top_p[-1]) == VM_CONTEXT_FOR_IN);

          uint32_t index = context_top_p[-3];
          ecma_value_t *buffer_p = collection_p->buffer_p;

          *stack_top_p++ = buffer_p[index];
          context_top_p[-3]++;
          continue;
        }
        case VM_OC_FOR_IN_HAS_NEXT:
        {
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          ecma_collection_t *collection_p;
          collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, stack_top_p[-2]);

          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FOR_IN);

          ecma_value_t *buffer_p = collection_p->buffer_p;
          ecma_object_t *object_p = ecma_get_object_from_value (stack_top_p[-4]);
          uint32_t index = stack_top_p[-3];

          while (index < collection_p->item_count)
          {
            ecma_string_t *prop_name_p = ecma_get_prop_name_from_value (buffer_p[index]);

            result = ecma_op_object_has_property (object_p, prop_name_p);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              stack_top_p[-3] = index;
              goto error;
            }

            if (JERRY_LIKELY (ecma_is_value_true (result)))
            {
              byte_code_p = byte_code_start_p + branch_offset;
              break;
            }

            ecma_deref_ecma_string (prop_name_p);
            index++;
          }

          if (index == collection_p->item_count)
          {
            ecma_deref_object (object_p);
            ecma_collection_destroy (collection_p);
            VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION);
            stack_top_p -= PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION;
          }
          else
          {
            stack_top_p[-3] = index;
          }
          continue;
        }
        case VM_OC_FOR_OF_INIT:
        {
          ecma_value_t value = *(--stack_top_p);

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          ecma_value_t next_method;
          ecma_value_t iterator = ecma_op_get_iterator (value, ECMA_VALUE_SYNC_ITERATOR, &next_method);

          ecma_free_value (value);

          if (ECMA_IS_VALUE_ERROR (iterator))
          {
            result = iterator;
            goto error;
          }

          result = ecma_op_iterator_step (iterator, next_method);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            ecma_free_value (iterator);
            ecma_free_value (next_method);
            goto error;
          }

          if (ecma_is_value_false (result))
          {
            ecma_free_value (iterator);
            ecma_free_value (next_method);
            byte_code_p = byte_code_start_p + branch_offset;
            continue;
          }

          ecma_value_t next_value = ecma_op_iterator_value (result);
          ecma_free_value (result);

          if (ECMA_IS_VALUE_ERROR (next_value))
          {
            result = next_value;
            ecma_free_value (iterator);
            ecma_free_value (next_method);
            goto error;
          }

          branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION);
          stack_top_p += PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION;
          stack_top_p[-1] = VM_CREATE_CONTEXT (VM_CONTEXT_FOR_OF, branch_offset) | VM_CONTEXT_CLOSE_ITERATOR;
          stack_top_p[-2] = next_value;
          stack_top_p[-3] = iterator;
          stack_top_p[-4] = next_method;

          if (byte_code_p[0] == CBC_EXT_OPCODE && byte_code_p[1] == CBC_EXT_CLONE_CONTEXT)
          {
            /* No need to duplicate the first context. */
            byte_code_p += 2;
          }
          continue;
        }
        case VM_OC_FOR_OF_GET_NEXT:
        {
          ecma_value_t *context_top_p = VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth;
          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (context_top_p[-1]) == VM_CONTEXT_FOR_OF
                        || VM_GET_CONTEXT_TYPE (context_top_p[-1]) == VM_CONTEXT_FOR_AWAIT_OF);
          JERRY_ASSERT (context_top_p[-1] & VM_CONTEXT_CLOSE_ITERATOR);

          *stack_top_p++ = context_top_p[-2];
          context_top_p[-2] = ECMA_VALUE_UNDEFINED;
          continue;
        }
        case VM_OC_FOR_OF_HAS_NEXT:
        {
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FOR_OF);
          JERRY_ASSERT (stack_top_p[-1] & VM_CONTEXT_CLOSE_ITERATOR);

          stack_top_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
          result = ecma_op_iterator_step (stack_top_p[-3], stack_top_p[-4]);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          if (ecma_is_value_false (result))
          {
            ecma_free_value (stack_top_p[-2]);
            ecma_free_value (stack_top_p[-3]);
            ecma_free_value (stack_top_p[-4]);
            VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION);
            stack_top_p -= PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION;
            continue;
          }

          ecma_value_t next_value = ecma_op_iterator_value (result);
          ecma_free_value (result);

          if (ECMA_IS_VALUE_ERROR (next_value))
          {
            result = next_value;
            goto error;
          }

          JERRY_ASSERT (stack_top_p[-2] == ECMA_VALUE_UNDEFINED);
          stack_top_p[-1] |= VM_CONTEXT_CLOSE_ITERATOR;
          stack_top_p[-2] = next_value;
          byte_code_p = byte_code_start_p + branch_offset;
          continue;
        }
        case VM_OC_FOR_AWAIT_OF_INIT:
        {
          ecma_value_t value = *(--stack_top_p);

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          ecma_value_t next_method;
          result = ecma_op_get_iterator (value, ECMA_VALUE_ASYNC_ITERATOR, &next_method);

          ecma_free_value (value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          ecma_value_t iterator = result;
          result = ecma_op_iterator_next (result, next_method, ECMA_VALUE_EMPTY);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            ecma_free_value (iterator);
            ecma_free_value (next_method);
            goto error;
          }

          branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION);
          stack_top_p += PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION;
          stack_top_p[-1] = VM_CREATE_CONTEXT (VM_CONTEXT_FOR_AWAIT_OF, branch_offset);
          stack_top_p[-2] = ECMA_VALUE_UNDEFINED;
          stack_top_p[-3] = iterator;
          stack_top_p[-4] = next_method;

          if (byte_code_p[0] == CBC_EXT_OPCODE && byte_code_p[1] == CBC_EXT_CLONE_CONTEXT)
          {
            /* No need to duplicate the first context. */
            byte_code_p += 2;
          }

          frame_ctx_p->call_operation = VM_EXEC_RETURN;
          frame_ctx_p->byte_code_p = byte_code_p;
          frame_ctx_p->stack_top_p = stack_top_p;

          uint16_t extra_flags =
            (ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD | (ECMA_AWAIT_FOR_NEXT << ECMA_AWAIT_STATE_SHIFT));

          if (CBC_FUNCTION_GET_TYPE (bytecode_header_p->status_flags) == CBC_FUNCTION_ASYNC_GENERATOR
              || (frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_EXECUTABLE))
          {
            ecma_extended_object_t *executable_object_p = VM_GET_EXECUTABLE_OBJECT (frame_ctx_p);
            result = ecma_promise_async_await (executable_object_p, result);

            if (ECMA_IS_VALUE_ERROR (result))
            {
              goto error;
            }

            executable_object_p->u.cls.u2.executable_obj_flags |= extra_flags;
            return ECMA_VALUE_UNDEFINED;
          }

          result = opfunc_async_create_and_await (frame_ctx_p, result, extra_flags);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }
          return result;
        }
        case VM_OC_FOR_AWAIT_OF_HAS_NEXT:
        {
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FOR_AWAIT_OF);
          JERRY_ASSERT (stack_top_p[-1] & VM_CONTEXT_CLOSE_ITERATOR);

          stack_top_p[-1] &= (uint32_t) ~VM_CONTEXT_CLOSE_ITERATOR;
          result = ecma_op_iterator_next (stack_top_p[-3], stack_top_p[-4], ECMA_VALUE_EMPTY);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          ecma_extended_object_t *executable_object_p = VM_GET_EXECUTABLE_OBJECT (frame_ctx_p);
          result = ecma_promise_async_await (executable_object_p, result);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          uint16_t extra_flags =
            (ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD | (ECMA_AWAIT_FOR_NEXT << ECMA_AWAIT_STATE_SHIFT));
          executable_object_p->u.cls.u2.executable_obj_flags |= extra_flags;

          frame_ctx_p->call_operation = VM_EXEC_RETURN;
          frame_ctx_p->byte_code_p = byte_code_start_p + branch_offset;
          frame_ctx_p->stack_top_p = stack_top_p;
          return ECMA_VALUE_UNDEFINED;
        }
        case VM_OC_TRY:
        {
          /* Try opcode simply creates the try context. */
          branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_TRY_CONTEXT_STACK_ALLOCATION);
          stack_top_p += PARSER_TRY_CONTEXT_STACK_ALLOCATION;

          stack_top_p[-1] = VM_CREATE_CONTEXT (VM_CONTEXT_TRY, branch_offset);
          continue;
        }
        case VM_OC_CATCH:
        {
          /* Catches are ignored and turned to jumps. */
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_TRY);

          byte_code_p = byte_code_start_p + branch_offset;
          continue;
        }
        case VM_OC_FINALLY:
        {
          branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_TRY
                        || VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_CATCH);

          if (stack_top_p[-1] & VM_CONTEXT_HAS_LEX_ENV)
          {
            ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;
            JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
            frame_ctx_p->lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
            ecma_deref_object (lex_env_p);
          }

          VM_PLUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FINALLY_CONTEXT_EXTRA_STACK_ALLOCATION);
          stack_top_p += PARSER_FINALLY_CONTEXT_EXTRA_STACK_ALLOCATION;

          stack_top_p[-1] = VM_CREATE_CONTEXT (VM_CONTEXT_FINALLY_JUMP, branch_offset);
          stack_top_p[-2] = (ecma_value_t) branch_offset;
          continue;
        }
        case VM_OC_CONTEXT_END:
        {
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          JERRY_ASSERT (!(stack_top_p[-1] & VM_CONTEXT_CLOSE_ITERATOR));

          ecma_value_t context_type = VM_GET_CONTEXT_TYPE (stack_top_p[-1]);

          if (!VM_CONTEXT_IS_FINALLY (context_type))
          {
            stack_top_p = vm_stack_context_abort (frame_ctx_p, stack_top_p);

            JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
            continue;
          }

          if (stack_top_p[-1] & VM_CONTEXT_HAS_LEX_ENV)
          {
            ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;
            JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);
            frame_ctx_p->lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
            ecma_deref_object (lex_env_p);
          }

          VM_MINUS_EQUAL_U16 (frame_ctx_p->context_depth, PARSER_FINALLY_CONTEXT_STACK_ALLOCATION);
          stack_top_p -= PARSER_FINALLY_CONTEXT_STACK_ALLOCATION;

          if (context_type == VM_CONTEXT_FINALLY_RETURN)
          {
            result = *stack_top_p;
            goto error;
          }

          if (context_type == VM_CONTEXT_FINALLY_THROW)
          {
            jcontext_raise_exception (*stack_top_p);
#if JERRY_VM_THROW
            JERRY_CONTEXT (status_flags) |= ECMA_STATUS_ERROR_THROWN;
#endif /* JERRY_VM_THROW */
            result = ECMA_VALUE_ERROR;
            goto error;
          }

          JERRY_ASSERT (context_type == VM_CONTEXT_FINALLY_JUMP);

          uint32_t jump_target = *stack_top_p;

          vm_stack_found_type type =
            vm_stack_find_finally (frame_ctx_p, stack_top_p, VM_CONTEXT_FINALLY_JUMP, jump_target);
          stack_top_p = frame_ctx_p->stack_top_p;
          switch (type)
          {
            case VM_CONTEXT_FOUND_FINALLY:
            {
              byte_code_p = frame_ctx_p->byte_code_p;

              JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FINALLY_JUMP);
              stack_top_p[-2] = jump_target;
              break;
            }
            case VM_CONTEXT_FOUND_ERROR:
            {
              JERRY_ASSERT (jcontext_has_pending_exception ());
              result = ECMA_VALUE_ERROR;
              goto error;
            }
            case VM_CONTEXT_FOUND_AWAIT:
            {
              JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FINALLY_JUMP);
              stack_top_p[-2] = jump_target;
              return ECMA_VALUE_UNDEFINED;
            }
            default:
            {
              byte_code_p = frame_ctx_p->byte_code_start_p + jump_target;
              break;
            }
          }

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          continue;
        }
        case VM_OC_JUMP_AND_EXIT_CONTEXT:
        {
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          JERRY_ASSERT (!jcontext_has_pending_exception ());

          branch_offset += (int32_t) (byte_code_start_p - frame_ctx_p->byte_code_start_p);

          vm_stack_found_type type =
            vm_stack_find_finally (frame_ctx_p, stack_top_p, VM_CONTEXT_FINALLY_JUMP, (uint32_t) branch_offset);
          stack_top_p = frame_ctx_p->stack_top_p;
          switch (type)
          {
            case VM_CONTEXT_FOUND_FINALLY:
            {
              byte_code_p = frame_ctx_p->byte_code_p;

              JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FINALLY_JUMP);
              stack_top_p[-2] = (uint32_t) branch_offset;
              break;
            }
            case VM_CONTEXT_FOUND_ERROR:
            {
              JERRY_ASSERT (jcontext_has_pending_exception ());
              result = ECMA_VALUE_ERROR;
              goto error;
            }
            case VM_CONTEXT_FOUND_AWAIT:
            {
              JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FINALLY_JUMP);
              stack_top_p[-2] = (uint32_t) branch_offset;
              return ECMA_VALUE_UNDEFINED;
            }
            default:
            {
              byte_code_p = frame_ctx_p->byte_code_start_p + branch_offset;
              break;
            }
          }

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          continue;
        }
#if JERRY_MODULE_SYSTEM
        case VM_OC_MODULE_IMPORT:
        {
          left_value = *(--stack_top_p);

          ecma_value_t user_value = ECMA_VALUE_UNDEFINED;
          ecma_value_t script_value = ((cbc_uint8_arguments_t *) bytecode_header_p)->script_value;

#if JERRY_SNAPSHOT_EXEC
          if (JERRY_UNLIKELY (!(bytecode_header_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION)))
          {
#endif /* JERRY_SNAPSHOT_EXEC */
            cbc_script_t *script_p = ECMA_GET_INTERNAL_VALUE_POINTER (cbc_script_t, script_value);

            if (script_p->refs_and_type & CBC_SCRIPT_HAS_USER_VALUE)
            {
              user_value = CBC_SCRIPT_GET_USER_VALUE (script_p);
            }
#if JERRY_SNAPSHOT_EXEC
          }
#endif /* JERRY_SNAPSHOT_EXEC */

          result = ecma_module_import (left_value, user_value);
          ecma_free_value (left_value);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto error;
          }

          *stack_top_p++ = result;
          continue;
        }
        case VM_OC_MODULE_IMPORT_META:
        {
          ecma_value_t script_value = ((cbc_uint8_arguments_t *) bytecode_header_p)->script_value;
          cbc_script_t *script_p = ECMA_GET_INTERNAL_VALUE_POINTER (cbc_script_t, script_value);

          JERRY_ASSERT (script_p->refs_and_type & CBC_SCRIPT_HAS_IMPORT_META);

          ecma_value_t import_meta = CBC_SCRIPT_GET_IMPORT_META (script_p, script_p->refs_and_type);
          ecma_object_t *import_meta_object_p = ecma_get_object_from_value (import_meta);

          if (ecma_get_object_type (import_meta_object_p) != ECMA_OBJECT_TYPE_GENERAL)
          {
            JERRY_ASSERT (ecma_object_class_is (import_meta_object_p, ECMA_OBJECT_CLASS_MODULE));

            ecma_value_t module = import_meta;
            import_meta_object_p = ecma_create_object (NULL, 0, ECMA_OBJECT_TYPE_GENERAL);
            import_meta = ecma_make_object_value (import_meta_object_p);

            if (JERRY_CONTEXT (module_import_meta_callback_p) != NULL)
            {
              void *user_p = JERRY_CONTEXT (module_import_meta_callback_user_p);
              JERRY_CONTEXT (module_import_meta_callback_p) (module, import_meta, user_p);
            }

            CBC_SCRIPT_GET_IMPORT_META (script_p, script_p->refs_and_type) = import_meta;
          }
          else
          {
            ecma_ref_object (import_meta_object_p);
          }

          *stack_top_p++ = import_meta;
          continue;
        }
#endif /* JERRY_MODULE_SYSTEM */
        case VM_OC_NONE:
        default:
        {
          JERRY_ASSERT (VM_OC_GROUP_GET_INDEX (opcode_data) == VM_OC_NONE);

          jerry_fatal (JERRY_FATAL_DISABLED_BYTE_CODE);
        }
      }

      JERRY_ASSERT (VM_OC_HAS_PUT_RESULT (opcode_data));

      if (opcode_data & VM_OC_PUT_IDENT)
      {
        uint16_t literal_index;

        READ_LITERAL_INDEX (literal_index);

        if (literal_index < register_end)
        {
          ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, literal_index));
          VM_GET_REGISTER (frame_ctx_p, literal_index) = result;

          if (opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK))
          {
            result = ecma_fast_copy_value (result);
          }
        }
        else
        {
          ecma_string_t *var_name_str_p = ecma_get_string_from_value (literal_start_p[literal_index]);

          ecma_value_t put_value_result =
            ecma_op_put_value_lex_env_base (frame_ctx_p->lex_env_p, var_name_str_p, is_strict, result);

          if (ECMA_IS_VALUE_ERROR (put_value_result))
          {
            ecma_free_value (result);
            result = put_value_result;
            goto error;
          }

          if (!(opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK)))
          {
            ecma_fast_free_value (result);
          }
        }
      }
      else if (opcode_data & VM_OC_PUT_REFERENCE)
      {
        ecma_value_t property = *(--stack_top_p);
        ecma_value_t base = *(--stack_top_p);

        if (base == ECMA_VALUE_REGISTER_REF)
        {
          property = (ecma_value_t) ecma_get_integer_from_value (property);
          ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, property));
          VM_GET_REGISTER (frame_ctx_p, property) = result;

          if (!(opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK)))
          {
            goto free_both_values;
          }
          result = ecma_fast_copy_value (result);
        }
        else
        {
          ecma_value_t set_value_result = vm_op_set_value (base, property, result, is_strict);

          if (ECMA_IS_VALUE_ERROR (set_value_result))
          {
            ecma_free_value (result);
            result = set_value_result;
            goto error;
          }

          if (!(opcode_data & (VM_OC_PUT_STACK | VM_OC_PUT_BLOCK)))
          {
            ecma_fast_free_value (result);
            goto free_both_values;
          }
        }
      }

      if (opcode_data & VM_OC_PUT_STACK)
      {
        *stack_top_p++ = result;
      }
      else if (opcode_data & VM_OC_PUT_BLOCK)
      {
        ecma_fast_free_value (VM_GET_REGISTER (frame_ctx_p, 0));
        VM_GET_REGISTERS (frame_ctx_p)[0] = result;
      }

free_both_values:
      ecma_fast_free_value (right_value);
free_left_value:
      ecma_fast_free_value (left_value);
    }

error:
    ecma_fast_free_value (left_value);
    ecma_fast_free_value (right_value);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      JERRY_ASSERT (jcontext_has_pending_exception ());
      ecma_value_t *stack_bottom_p = VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth;

      while (stack_top_p > stack_bottom_p)
      {
        ecma_value_t stack_item = *(--stack_top_p);
        if (stack_item == ECMA_VALUE_RELEASE_LEX_ENV)
        {
          opfunc_pop_lexical_environment (frame_ctx_p);
          continue;
        }

        ecma_fast_free_value (stack_item);
      }

#if JERRY_VM_THROW
      if (!(JERRY_CONTEXT (status_flags) & ECMA_STATUS_ERROR_THROWN))
      {
        JERRY_CONTEXT (status_flags) |= ECMA_STATUS_ERROR_THROWN;

        jerry_throw_cb_t vm_throw_callback_p = JERRY_CONTEXT (vm_throw_callback_p);

        if (vm_throw_callback_p != NULL)
        {
          vm_throw_callback_p (JERRY_CONTEXT (error_value), JERRY_CONTEXT (vm_throw_callback_user_p));
        }
      }
#endif /* JERRY_VM_THROW */
    }

    JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

    if (frame_ctx_p->context_depth == 0)
    {
      /* In most cases there is no context. */
      frame_ctx_p->call_operation = VM_NO_EXEC_OP;
      return result;
    }

    if (!ECMA_IS_VALUE_ERROR (result))
    {
      switch (vm_stack_find_finally (frame_ctx_p, stack_top_p, VM_CONTEXT_FINALLY_RETURN, 0))
      {
        case VM_CONTEXT_FOUND_FINALLY:
        {
          stack_top_p = frame_ctx_p->stack_top_p;
          byte_code_p = frame_ctx_p->byte_code_p;

          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FINALLY_RETURN);
          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          stack_top_p[-2] = result;
          continue;
        }
        case VM_CONTEXT_FOUND_ERROR:
        {
          JERRY_ASSERT (jcontext_has_pending_exception ());

          ecma_free_value (result);
          stack_top_p = frame_ctx_p->stack_top_p;
          result = ECMA_VALUE_ERROR;
          break;
        }
        case VM_CONTEXT_FOUND_AWAIT:
        {
          stack_top_p = frame_ctx_p->stack_top_p;

          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FINALLY_RETURN);
          stack_top_p[-2] = result;
          return ECMA_VALUE_UNDEFINED;
        }
        default:
        {
          goto finish;
        }
      }
    }

    JERRY_ASSERT (jcontext_has_pending_exception ());

    if (!jcontext_has_pending_abort ())
    {
      switch (vm_stack_find_finally (frame_ctx_p, stack_top_p, VM_CONTEXT_FINALLY_THROW, 0))
      {
        case VM_CONTEXT_FOUND_FINALLY:
        {
          stack_top_p = frame_ctx_p->stack_top_p;
          byte_code_p = frame_ctx_p->byte_code_p;

          JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);
          JERRY_ASSERT (!(stack_top_p[-1] & VM_CONTEXT_HAS_LEX_ENV));

          result = jcontext_take_exception ();

          if (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_FINALLY_THROW)
          {
            stack_top_p[-2] = result;
            continue;
          }

          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (stack_top_p[-1]) == VM_CONTEXT_CATCH);

          *stack_top_p++ = result;
          continue;
        }
        case VM_CONTEXT_FOUND_AWAIT:
        {
          JERRY_ASSERT (VM_GET_CONTEXT_TYPE (frame_ctx_p->stack_top_p[-1]) == VM_CONTEXT_FINALLY_THROW);
          return ECMA_VALUE_UNDEFINED;
        }
        default:
        {
          break;
        }
      }
    }
    else
    {
      do
      {
        JERRY_ASSERT (VM_GET_REGISTERS (frame_ctx_p) + register_end + frame_ctx_p->context_depth == stack_top_p);

        stack_top_p = vm_stack_context_abort (frame_ctx_p, stack_top_p);
      } while (frame_ctx_p->context_depth > 0);
    }

finish:
    frame_ctx_p->call_operation = VM_NO_EXEC_OP;
    return result;
  }
} /* vm_loop */

#if JERRY_MODULE_SYSTEM

/**
 * Create and initialize module scope with all data properties
 *
 * @return ECMA_VALUE_EMPTY on success,
 *         ECMA_VALUE_ERROR on failure
 */
ecma_value_t
vm_init_module_scope (ecma_module_t *module_p) /**< module without scope */
{
  ecma_object_t *global_object_p;
#if JERRY_BUILTIN_REALMS
  global_object_p = (ecma_object_t *) ecma_op_function_get_realm (module_p->u.compiled_code_p);
#else /* !JERRY_BUILTIN_REALMS */
  global_object_p = ecma_builtin_get_global ();
#endif /* JERRY_BUILTIN_REALMS */

  ecma_object_t *scope_p = ecma_create_lex_env_class (ecma_get_global_environment (global_object_p),
                                                      sizeof (ecma_lexical_environment_class_t));
  const ecma_compiled_code_t *compiled_code_p = module_p->u.compiled_code_p;
  ecma_value_t *literal_start_p;
  uint8_t *byte_code_p;
  uint16_t encoding_limit;
  uint16_t encoding_delta;

  ((ecma_lexical_environment_class_t *) scope_p)->object_p = (ecma_object_t *) module_p;
  ((ecma_lexical_environment_class_t *) scope_p)->type = ECMA_LEX_ENV_CLASS_TYPE_MODULE;

  module_p->scope_p = scope_p;
  ecma_deref_object (scope_p);

  if (compiled_code_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) compiled_code_p;

    literal_start_p = (ecma_value_t *) (args_p + 1);
    literal_start_p -= args_p->register_end;
    byte_code_p = (uint8_t *) (literal_start_p + args_p->literal_end);
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) compiled_code_p;

    literal_start_p = (ecma_value_t *) (args_p + 1);
    literal_start_p -= args_p->register_end;
    byte_code_p = (uint8_t *) (literal_start_p + args_p->literal_end);
  }

  /* Prepare for byte code execution. */
  if (!(compiled_code_p->status_flags & CBC_CODE_FLAGS_FULL_LITERAL_ENCODING))
  {
    encoding_limit = CBC_SMALL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_SMALL_LITERAL_ENCODING_DELTA;
  }
  else
  {
    encoding_limit = CBC_FULL_LITERAL_ENCODING_LIMIT;
    encoding_delta = CBC_FULL_LITERAL_ENCODING_DELTA;
  }

  JERRY_ASSERT (*byte_code_p >= CBC_JUMP_FORWARD && *byte_code_p <= CBC_JUMP_FORWARD_3);

  byte_code_p += 1 + CBC_BRANCH_OFFSET_LENGTH (*byte_code_p);

  while (true)
  {
    uint8_t opcode = *byte_code_p++;

    switch (opcode)
    {
      case CBC_CREATE_VAR:
      case CBC_CREATE_LET:
      case CBC_CREATE_CONST:
      {
        uint32_t literal_index;

        READ_LITERAL_INDEX (literal_index);

        ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

        JERRY_ASSERT (ecma_find_named_property (scope_p, name_p) == NULL);

        uint8_t prop_attributes = ECMA_PROPERTY_FLAG_WRITABLE;

        if (opcode == CBC_CREATE_LET)
        {
          prop_attributes = ECMA_PROPERTY_ENUMERABLE_WRITABLE;
        }
        else if (opcode == CBC_CREATE_CONST)
        {
          prop_attributes = ECMA_PROPERTY_FLAG_ENUMERABLE;
        }

        ecma_property_value_t *property_value_p;
        property_value_p = ecma_create_named_data_property (scope_p, name_p, prop_attributes, NULL);

        if (opcode != CBC_CREATE_VAR)
        {
          property_value_p->value = ECMA_VALUE_UNINITIALIZED;
        }
        break;
      }
      case CBC_INIT_ARG_OR_FUNC:
      {
        uint32_t literal_index;

        READ_LITERAL_INDEX (literal_index);

        ecma_compiled_code_t *function_bytecode_p;
#if JERRY_SNAPSHOT_EXEC
        if (JERRY_LIKELY (!(compiled_code_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION)))
        {
#endif /* JERRY_SNAPSHOT_EXEC */
          function_bytecode_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, literal_start_p[literal_index]);
#if JERRY_SNAPSHOT_EXEC
        }
        else
        {
          uint8_t *byte_p = ((uint8_t *) compiled_code_p) + literal_start_p[literal_index];
          function_bytecode_p = (ecma_compiled_code_t *) byte_p;
        }
#endif /* JERRY_SNAPSHOT_EXEC */

        JERRY_ASSERT (CBC_IS_FUNCTION (function_bytecode_p->status_flags));

        ecma_object_t *function_obj_p;

        if (JERRY_UNLIKELY (CBC_FUNCTION_IS_ARROW (function_bytecode_p->status_flags)))
        {
          function_obj_p = ecma_op_create_arrow_function_object (scope_p, function_bytecode_p, ECMA_VALUE_UNDEFINED);
        }
        else
        {
          function_obj_p = ecma_op_create_any_function_object (scope_p, function_bytecode_p);
        }

        READ_LITERAL_INDEX (literal_index);
        ecma_string_t *name_p = ecma_get_string_from_value (literal_start_p[literal_index]);

        JERRY_ASSERT (ecma_find_named_property (scope_p, name_p) == NULL);

        ecma_property_value_t *property_value_p;
        property_value_p = ecma_create_named_data_property (scope_p, name_p, ECMA_PROPERTY_FLAG_WRITABLE, NULL);

        JERRY_ASSERT (property_value_p->value == ECMA_VALUE_UNDEFINED);
        property_value_p->value = ecma_make_object_value (function_obj_p);
        ecma_deref_object (function_obj_p);
        break;
      }
      default:
      {
        JERRY_ASSERT (opcode == CBC_RETURN_FUNCTION_END);
        return ECMA_VALUE_EMPTY;
      }
    }
  }
} /* vm_init_module_scope */

#endif /* JERRY_MODULE_SYSTEM */

#undef READ_LITERAL
#undef READ_LITERAL_INDEX

JERRY_STATIC_ASSERT (((int)VM_FRAME_CTX_SHARED_DIRECT_EVAL == (int)VM_FRAME_CTX_DIRECT_EVAL),
                     vm_frame_ctx_shared_direct_eval_must_be_equal_to_frame_ctx_direct_eval);

JERRY_STATIC_ASSERT (((int)CBC_CODE_FLAGS_STRICT_MODE == (int)VM_FRAME_CTX_IS_STRICT),
                     cbc_code_flags_strict_mode_must_be_equal_to_vm_frame_ctx_is_strict);

/**
 * Initialize code block execution
 *
 * @return ECMA_VALUE_ERROR - if the initialization fails
 *         ECMA_VALUE_EMPTY - otherwise
 */
static void JERRY_ATTR_NOINLINE
vm_init_exec (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  vm_frame_ctx_shared_t *shared_p = frame_ctx_p->shared_p;
  const ecma_compiled_code_t *bytecode_header_p = shared_p->bytecode_header_p;

  frame_ctx_p->prev_context_p = JERRY_CONTEXT (vm_top_context_p);
  frame_ctx_p->context_depth = 0;
  frame_ctx_p->status_flags = (uint8_t) ((shared_p->status_flags & VM_FRAME_CTX_DIRECT_EVAL)
                                         | (bytecode_header_p->status_flags & VM_FRAME_CTX_IS_STRICT));

  uint16_t argument_end, register_end;
  ecma_value_t *literal_p;

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_header_p;

    argument_end = args_p->argument_end;
    register_end = args_p->register_end;

    literal_p = (ecma_value_t *) (args_p + 1);
    literal_p -= register_end;
    frame_ctx_p->literal_start_p = literal_p;
    literal_p += args_p->literal_end;
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_header_p;

    argument_end = args_p->argument_end;
    register_end = args_p->register_end;

    literal_p = (ecma_value_t *) (args_p + 1);
    literal_p -= register_end;
    frame_ctx_p->literal_start_p = literal_p;
    literal_p += args_p->literal_end;
  }

  frame_ctx_p->byte_code_p = (uint8_t *) literal_p;
  frame_ctx_p->byte_code_start_p = (uint8_t *) literal_p;
  frame_ctx_p->stack_top_p = VM_GET_REGISTERS (frame_ctx_p) + register_end;

  uint32_t arg_list_len = 0;

  if (argument_end > 0)
  {
    JERRY_ASSERT (shared_p->status_flags & VM_FRAME_CTX_SHARED_HAS_ARG_LIST);

    const ecma_value_t *arg_list_p = ((vm_frame_ctx_shared_args_t *) shared_p)->arg_list_p;
    arg_list_len = ((vm_frame_ctx_shared_args_t *) shared_p)->arg_list_len;

    if (arg_list_len > argument_end)
    {
      arg_list_len = argument_end;
    }

    for (uint32_t i = 0; i < arg_list_len; i++)
    {
      VM_GET_REGISTER (frame_ctx_p, i) = ecma_fast_copy_value (arg_list_p[i]);
    }
  }

  /* The arg_list_len contains the end of the copied arguments.
   * Fill everything else with undefined. */
  if (register_end > arg_list_len)
  {
    ecma_value_t *stack_p = VM_GET_REGISTERS (frame_ctx_p) + arg_list_len;

    for (uint32_t i = arg_list_len; i < register_end; i++)
    {
      *stack_p++ = ECMA_VALUE_UNDEFINED;
    }
  }

  JERRY_CONTEXT (status_flags) &= (uint32_t) ~ECMA_STATUS_DIRECT_EVAL;
  JERRY_CONTEXT (vm_top_context_p) = frame_ctx_p;
} /* vm_init_exec */

/**
 * Resume execution of a code block.
 *
 * @return ecma value
 */
ecma_value_t JERRY_ATTR_NOINLINE
vm_execute (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  while (true)
  {
    ecma_value_t completion_value = vm_loop (frame_ctx_p);

    switch (frame_ctx_p->call_operation)
    {
      case VM_EXEC_CALL:
      {
        opfunc_call (frame_ctx_p);
        break;
      }
      case VM_EXEC_SUPER_CALL:
      {
        vm_super_call (frame_ctx_p);
        break;
      }
      case VM_EXEC_SPREAD_OP:
      {
        vm_spread_operation (frame_ctx_p);
        break;
      }
      case VM_EXEC_RETURN:
      {
        return completion_value;
      }
      case VM_EXEC_CONSTRUCT:
      {
        opfunc_construct (frame_ctx_p);
        break;
      }
      default:
      {
        JERRY_ASSERT (frame_ctx_p->call_operation == VM_NO_EXEC_OP);

        const ecma_compiled_code_t *bytecode_header_p = frame_ctx_p->shared_p->bytecode_header_p;
        uint32_t register_end;

        if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
        {
          register_end = ((cbc_uint16_arguments_t *) bytecode_header_p)->register_end;
        }
        else
        {
          register_end = ((cbc_uint8_arguments_t *) bytecode_header_p)->register_end;
        }

        /* Free arguments and registers */
        ecma_value_t *registers_p = VM_GET_REGISTERS (frame_ctx_p);
        for (uint32_t i = 0; i < register_end; i++)
        {
          ecma_fast_free_value (registers_p[i]);
        }
        JERRY_CONTEXT (vm_top_context_p) = frame_ctx_p->prev_context_p;
        return completion_value;
      }
    }
  }
} /* vm_execute */

/**
 * Run the code.
 *
 * @return ecma value
 */
ecma_value_t
vm_run (vm_frame_ctx_shared_t *shared_p, /**< shared data */
        ecma_value_t this_binding_value, /**< value of 'ThisBinding' */
        ecma_object_t *lex_env_p) /**< lexical environment to use */
{
  const ecma_compiled_code_t *bytecode_header_p = shared_p->bytecode_header_p;
  vm_frame_ctx_t *frame_ctx_p;
  size_t frame_size;

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_header_p;
    frame_size = (size_t) (args_p->register_end + args_p->stack_limit);
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_header_p;
    frame_size = (size_t) (args_p->register_end + args_p->stack_limit);
  }

  JERRY_VLA (ecma_value_t, stack, frame_size + (sizeof (vm_frame_ctx_t) / sizeof (ecma_value_t)));

  frame_ctx_p = (vm_frame_ctx_t *) stack;

  frame_ctx_p->shared_p = shared_p;
  frame_ctx_p->lex_env_p = lex_env_p;
  frame_ctx_p->this_binding = this_binding_value;

  vm_init_exec (frame_ctx_p);
  return vm_execute (frame_ctx_p);
} /* vm_run */

/**
 * @}
 * @}
 */
