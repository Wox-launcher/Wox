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

#include "opcodes.h"

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
#include "ecma-lex-env.h"
#include "ecma-objects.h"
#include "ecma-promise-object.h"
#include "ecma-proxy-object.h"

#include "jcontext.h"
#include "vm-defines.h"
#include "vm-stack.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup vm_opcodes Opcodes
 * @{
 */

/**
 * 'typeof' opcode handler.
 *
 * See also: ECMA-262 v5, 11.4.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
opfunc_typeof (ecma_value_t left_value) /**< left value */
{
  return ecma_make_magic_string_value (ecma_get_typeof_lit_id (left_value));
} /* opfunc_typeof */

/**
 * Update data property for object literals.
 */
void
opfunc_set_data_property (ecma_object_t *object_p, /**< object */
                          ecma_string_t *prop_name_p, /**< data property name */
                          ecma_value_t value) /**< new value */
{
  JERRY_ASSERT (!ecma_op_object_is_fast_array (object_p));

  ecma_property_t *property_p = ecma_find_named_property (object_p, prop_name_p);
  ecma_property_value_t *prop_value_p;

  if (property_p == NULL)
  {
    prop_value_p =
      ecma_create_named_data_property (object_p, prop_name_p, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE, NULL);
  }
  else
  {
    JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

    prop_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

    if (!(*property_p & ECMA_PROPERTY_FLAG_DATA))
    {
      *property_p |= ECMA_PROPERTY_FLAG_DATA | ECMA_PROPERTY_FLAG_WRITABLE;
      prop_value_p->value = ecma_copy_value_if_not_object (value);
      return;
    }
  }

  ecma_named_data_property_assign_value (object_p, prop_value_p, value);
} /* opfunc_set_data_property */

/**
 * Update getter or setter for object literals.
 */
void
opfunc_set_accessor (bool is_getter, /**< is getter accessor */
                     ecma_value_t object, /**< object value */
                     ecma_string_t *accessor_name_p, /**< accessor name */
                     ecma_value_t accessor) /**< accessor value */
{
  ecma_object_t *object_p = ecma_get_object_from_value (object);

  JERRY_ASSERT (!ecma_op_object_is_fast_array (object_p));

  ecma_property_t *property_p = ecma_find_named_property (object_p, accessor_name_p);
  ecma_object_t *accessor_p = ecma_get_object_from_value (accessor);

  ecma_object_t *getter_func_p = NULL;
  ecma_object_t *setter_func_p = NULL;

  if (is_getter)
  {
    getter_func_p = accessor_p;
  }
  else
  {
    setter_func_p = accessor_p;
  }

  if (property_p == NULL)
  {
    ecma_create_named_accessor_property (object_p,
                                         accessor_name_p,
                                         getter_func_p,
                                         setter_func_p,
                                         ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE,
                                         NULL);
  }
  else
  {
    JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

    ecma_property_value_t *prop_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

    if (*property_p & ECMA_PROPERTY_FLAG_DATA)
    {
      ecma_free_value_if_not_object (prop_value_p->value);
      *property_p = (uint8_t) (*property_p & ~(ECMA_PROPERTY_FLAG_DATA | ECMA_PROPERTY_FLAG_WRITABLE));

      ECMA_SET_POINTER (prop_value_p->getter_setter_pair.getter_cp, getter_func_p);
      ECMA_SET_POINTER (prop_value_p->getter_setter_pair.setter_cp, setter_func_p);
      return;
    }

    if (is_getter)
    {
      ecma_set_named_accessor_property_getter (object_p, prop_value_p, accessor_p);
    }
    else
    {
      ecma_set_named_accessor_property_setter (object_p, prop_value_p, accessor_p);
    }
  }
} /* opfunc_set_accessor */

/**
 * Deletes an object property.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
vm_op_delete_prop (ecma_value_t object, /**< base object */
                   ecma_value_t property, /**< property name */
                   bool is_strict) /**< strict mode */
{
  if (!ecma_op_require_object_coercible (object))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_string_t *name_string_p = ecma_op_to_property_key (property);

  if (JERRY_UNLIKELY (name_string_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t obj_value = ecma_op_to_object (object);
  /* The ecma_op_require_object_coercible call already checked the op_to_object error cases. */
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (obj_value));
  JERRY_ASSERT (ecma_is_value_object (obj_value));
  ecma_object_t *obj_p = ecma_get_object_from_value (obj_value);
  JERRY_ASSERT (!ecma_is_lexical_environment (obj_p));

  ecma_value_t delete_op_ret = ecma_op_object_delete (obj_p, name_string_p, is_strict);
  JERRY_ASSERT (ecma_is_value_boolean (delete_op_ret) || ECMA_IS_VALUE_ERROR (delete_op_ret));
  ecma_deref_object (obj_p);
  ecma_deref_ecma_string (name_string_p);

  if (is_strict && ecma_is_value_false (delete_op_ret))
  {
    return ecma_raise_type_error (ECMA_ERR_OPERATOR_DELETE_RETURNED_FALSE_IN_STRICT_MODE);
  }

  return delete_op_ret;
} /* vm_op_delete_prop */

/**
 * Deletes a variable.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
vm_op_delete_var (ecma_value_t name_literal, /**< name literal */
                  ecma_object_t *lex_env_p) /**< lexical environment */
{
  ecma_value_t completion_value = ECMA_VALUE_EMPTY;

  ecma_string_t *var_name_str_p = ecma_get_string_from_value (name_literal);

  ecma_object_t *ref_base_lex_env_p = ecma_op_resolve_reference_base (lex_env_p, var_name_str_p);

#if JERRY_BUILTIN_PROXY
  if (JERRY_UNLIKELY (ref_base_lex_env_p == ECMA_OBJECT_POINTER_ERROR))
  {
    return ECMA_VALUE_ERROR;
  }
#endif /* JERRY_BUILTIN_PROXY */

  if (ref_base_lex_env_p == NULL)
  {
    completion_value = ECMA_VALUE_TRUE;
  }
  else
  {
    JERRY_ASSERT (ecma_is_lexical_environment (ref_base_lex_env_p));

    completion_value = ecma_op_delete_binding (ref_base_lex_env_p, var_name_str_p);
  }

  return completion_value;
} /* vm_op_delete_var */

/**
 * 'for-in' opcode handler
 *
 *  Note: from ES2015 (ES6) the for-in can trigger error when
 *        the property names are not available (ex.: via Proxy ownKeys).
 *        In these cases an error must be returned.
 *
 *        This error is returned as the `result_obj_p` and the
 *        function's return value is NULL.
 *
 * See also:
 *          ECMA-262 v5, 12.6.4
 *
 * @return - chain list of property names
 *         - In case of error: NULL is returned and the `result_obj_p`
 *           must be checked.
 */
ecma_collection_t *
opfunc_for_in (ecma_value_t iterable_value, /**< ideally an iterable value */
               ecma_value_t *result_obj_p) /**< expression object */
{
  /* 3. */
  if (ecma_is_value_undefined (iterable_value) || ecma_is_value_null (iterable_value))
  {
    return NULL;
  }

  /* 4. */
  ecma_value_t obj_expr_value = ecma_op_to_object (iterable_value);
  /* ecma_op_to_object will only raise error on null/undefined values but those are handled above. */
  JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (obj_expr_value));
  ecma_object_t *obj_p = ecma_get_object_from_value (obj_expr_value);
  ecma_collection_t *prop_names_p = ecma_op_object_enumerate (obj_p);

  if (JERRY_UNLIKELY (prop_names_p == NULL))
  {
    ecma_deref_object (obj_p);
    *result_obj_p = ECMA_VALUE_ERROR;
    return NULL;
  }

  if (prop_names_p->item_count != 0)
  {
    *result_obj_p = ecma_make_object_value (obj_p);
    return prop_names_p;
  }

  ecma_deref_object (obj_p);
  ecma_collection_destroy (prop_names_p);

  return NULL;
} /* opfunc_for_in */

/**
 * 'VM_OC_APPEND_ARRAY' opcode handler specialized for spread objects
 *
 * @return ECMA_VALUE_ERROR - if the operation failed
 *         ECMA_VALUE_EMPTY, otherwise
 */
static ecma_value_t JERRY_ATTR_NOINLINE
opfunc_append_to_spread_array (ecma_value_t *stack_top_p, /**< current stack top */
                               uint16_t values_length) /**< number of elements to set */
{
  JERRY_ASSERT (!(values_length & OPFUNC_HAS_SPREAD_ELEMENT));

  ecma_object_t *array_obj_p = ecma_get_object_from_value (stack_top_p[-1]);
  JERRY_ASSERT (ecma_get_object_type (array_obj_p) == ECMA_OBJECT_TYPE_ARRAY);

  ecma_extended_object_t *ext_array_obj_p = (ecma_extended_object_t *) array_obj_p;
  uint32_t old_length = ext_array_obj_p->u.array.length;

  for (uint32_t i = 0, idx = old_length; i < values_length; i++, idx++)
  {
    if (ecma_is_value_array_hole (stack_top_p[i]))
    {
      continue;
    }

    if (stack_top_p[i] == ECMA_VALUE_SPREAD_ELEMENT)
    {
      i++;
      ecma_value_t ret_value = ECMA_VALUE_ERROR;
      ecma_value_t spread_value = stack_top_p[i];

      ecma_value_t next_method;
      ecma_value_t iterator = ecma_op_get_iterator (spread_value, ECMA_VALUE_SYNC_ITERATOR, &next_method);

      if (!ECMA_IS_VALUE_ERROR (iterator))
      {
        while (true)
        {
          ecma_value_t next_value = ecma_op_iterator_step (iterator, next_method);

          if (ECMA_IS_VALUE_ERROR (next_value))
          {
            break;
          }

          if (ecma_is_value_false (next_value))
          {
            idx--;
            ret_value = ECMA_VALUE_EMPTY;
            break;
          }

          ecma_value_t value = ecma_op_iterator_value (next_value);

          ecma_free_value (next_value);

          if (ECMA_IS_VALUE_ERROR (value))
          {
            break;
          }

          ecma_builtin_helper_def_prop_by_index (array_obj_p, idx++, value, ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
          ecma_free_value (value);
        }
      }

      ecma_free_value (iterator);
      ecma_free_value (next_method);
      ecma_free_value (spread_value);

      if (ECMA_IS_VALUE_ERROR (ret_value))
      {
        for (uint32_t k = i + 1; k < values_length; k++)
        {
          ecma_free_value (stack_top_p[k]);
        }

        return ret_value;
      }
    }
    else
    {
      ecma_builtin_helper_def_prop_by_index (array_obj_p, idx, stack_top_p[i], ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
      ecma_free_value (stack_top_p[i]);
    }
  }

  return ECMA_VALUE_EMPTY;
} /* opfunc_append_to_spread_array */

/**
 * Spread function call/construct arguments into an ecma-collection
 *
 * @return NULL - if the operation failed
 *         pointer to the ecma-collection with the spreaded arguments, otherwise
 */
JERRY_ATTR_NOINLINE ecma_collection_t *
opfunc_spread_arguments (ecma_value_t *stack_top_p, /**< pointer to the current stack top */
                         uint8_t arguments_list_len) /**< number of arguments */
{
  ecma_collection_t *buff_p = ecma_new_collection ();

  for (uint32_t i = 0; i < arguments_list_len; i++)
  {
    ecma_value_t arg = *stack_top_p++;

    if (arg != ECMA_VALUE_SPREAD_ELEMENT)
    {
      ecma_collection_push_back (buff_p, arg);
      continue;
    }

    ecma_value_t ret_value = ECMA_VALUE_ERROR;
    ecma_value_t spread_value = *stack_top_p++;
    i++;

    ecma_value_t next_method;
    ecma_value_t iterator = ecma_op_get_iterator (spread_value, ECMA_VALUE_SYNC_ITERATOR, &next_method);

    if (!ECMA_IS_VALUE_ERROR (iterator))
    {
      while (true)
      {
        ecma_value_t next_value = ecma_op_iterator_step (iterator, next_method);

        if (ECMA_IS_VALUE_ERROR (next_value))
        {
          break;
        }

        if (ecma_is_value_false (next_value))
        {
          ret_value = ECMA_VALUE_EMPTY;
          break;
        }

        ecma_value_t value = ecma_op_iterator_value (next_value);

        ecma_free_value (next_value);

        if (ECMA_IS_VALUE_ERROR (value))
        {
          break;
        }

        ecma_collection_push_back (buff_p, value);
      }
    }

    ecma_free_value (iterator);
    ecma_free_value (next_method);
    ecma_free_value (spread_value);

    if (ECMA_IS_VALUE_ERROR (ret_value))
    {
      for (uint32_t k = i + 1; k < arguments_list_len; k++)
      {
        ecma_free_value (*stack_top_p++);
      }

      ecma_collection_free (buff_p);
      buff_p = NULL;
      break;
    }
  }

  return buff_p;
} /* opfunc_spread_arguments */

/**
 * 'VM_OC_APPEND_ARRAY' opcode handler, for setting array object properties
 *
 * @return ECMA_VALUE_ERROR - if the operation failed
 *         ECMA_VALUE_EMPTY, otherwise
 */
ecma_value_t JERRY_ATTR_NOINLINE
opfunc_append_array (ecma_value_t *stack_top_p, /**< current stack top */
                     uint16_t values_length) /**< number of elements to set
                                              *   with potential OPFUNC_HAS_SPREAD_ELEMENT flag */
{
  if (values_length >= OPFUNC_HAS_SPREAD_ELEMENT)
  {
    return opfunc_append_to_spread_array (stack_top_p, (uint16_t) (values_length & ~OPFUNC_HAS_SPREAD_ELEMENT));
  }

  ecma_object_t *array_obj_p = ecma_get_object_from_value (stack_top_p[-1]);
  JERRY_ASSERT (ecma_get_object_type (array_obj_p) == ECMA_OBJECT_TYPE_ARRAY);

  ecma_extended_object_t *ext_array_obj_p = (ecma_extended_object_t *) array_obj_p;
  uint32_t old_length = ext_array_obj_p->u.array.length;

  if (JERRY_LIKELY (ecma_op_array_is_fast_array (ext_array_obj_p)))
  {
    uint32_t filled_holes = 0;
    ecma_value_t *values_p = ecma_fast_array_extend (array_obj_p, old_length + values_length);

    for (uint32_t i = 0; i < values_length; i++)
    {
      values_p[old_length + i] = stack_top_p[i];

      if (!ecma_is_value_array_hole (stack_top_p[i]))
      {
        filled_holes++;

        ecma_deref_if_object (stack_top_p[i]);
      }
    }

    ext_array_obj_p->u.array.length_prop_and_hole_count -= filled_holes * ECMA_FAST_ARRAY_HOLE_ONE;

    if (JERRY_UNLIKELY ((values_length - filled_holes) > ECMA_FAST_ARRAY_MAX_NEW_HOLES_COUNT))
    {
      ecma_fast_array_convert_to_normal (array_obj_p);
    }
  }
  else
  {
    for (uint32_t i = 0; i < values_length; i++)
    {
      if (!ecma_is_value_array_hole (stack_top_p[i]))
      {
        ecma_string_t *index_str_p = ecma_new_ecma_string_from_uint32 (old_length + i);

        ecma_property_value_t *prop_value_p;

        prop_value_p = ecma_create_named_data_property (array_obj_p,
                                                        index_str_p,
                                                        ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE,
                                                        NULL);

        ecma_deref_ecma_string (index_str_p);
        prop_value_p->value = stack_top_p[i];
        ecma_deref_if_object (stack_top_p[i]);
      }
    }
    ext_array_obj_p->u.array.length = old_length + values_length;
  }

  return ECMA_VALUE_EMPTY;
} /* opfunc_append_array */

/**
 * Create an executable object using the current frame context
 *
 * @return executable object
 */
vm_executable_object_t *
opfunc_create_executable_object (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                                 vm_create_executable_object_type_t type) /**< executable object type */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  const ecma_compiled_code_t *bytecode_header_p = frame_ctx_p->shared_p->bytecode_header_p;
  size_t size, register_end;

  ecma_bytecode_ref ((ecma_compiled_code_t *) bytecode_header_p);

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_header_p;
    register_end = (size_t) args_p->register_end;
    size = (register_end + (size_t) args_p->stack_limit) * sizeof (ecma_value_t);
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_header_p;
    register_end = (size_t) args_p->register_end;
    size = (register_end + (size_t) args_p->stack_limit) * sizeof (ecma_value_t);
  }

  size_t total_size = JERRY_ALIGNUP (sizeof (vm_executable_object_t) + size, sizeof (uintptr_t));

  ecma_object_t *proto_p = NULL;
  /* Async function objects are not accessible, so their class_id is not relevant. */
  uint8_t class_type = ECMA_OBJECT_CLASS_GENERATOR;

  if (type == VM_CREATE_EXECUTABLE_OBJECT_GENERATOR)
  {
    ecma_builtin_id_t default_proto_id = ECMA_BUILTIN_ID_GENERATOR_PROTOTYPE;

    if (CBC_FUNCTION_GET_TYPE (bytecode_header_p->status_flags) == CBC_FUNCTION_ASYNC_GENERATOR)
    {
      default_proto_id = ECMA_BUILTIN_ID_ASYNC_GENERATOR_PROTOTYPE;
      class_type = ECMA_OBJECT_CLASS_ASYNC_GENERATOR;
    }

    JERRY_ASSERT (frame_ctx_p->shared_p->status_flags & VM_FRAME_CTX_SHARED_NON_ARROW_FUNC);
    proto_p = ecma_op_get_prototype_from_constructor (frame_ctx_p->shared_p->function_object_p, default_proto_id);
  }

  ecma_object_t *object_p = ecma_create_object (proto_p, total_size, ECMA_OBJECT_TYPE_CLASS);

  vm_executable_object_t *executable_object_p = (vm_executable_object_t *) object_p;

  if (type == VM_CREATE_EXECUTABLE_OBJECT_GENERATOR)
  {
    ecma_deref_object (proto_p);
  }

  executable_object_p->extended_object.u.cls.type = class_type;
  executable_object_p->extended_object.u.cls.u2.executable_obj_flags = 0;
  ECMA_SET_INTERNAL_VALUE_ANY_POINTER (executable_object_p->extended_object.u.cls.u3.head, NULL);
  executable_object_p->iterator = ECMA_VALUE_UNDEFINED;

  JERRY_ASSERT (!(frame_ctx_p->status_flags & VM_FRAME_CTX_DIRECT_EVAL));

  /* Copy shared data and frame context. */
  vm_frame_ctx_shared_t *new_shared_p = &(executable_object_p->shared);
  *new_shared_p = *(frame_ctx_p->shared_p);
  new_shared_p->status_flags &= (uint32_t) ~VM_FRAME_CTX_SHARED_HAS_ARG_LIST;
  new_shared_p->status_flags |= VM_FRAME_CTX_SHARED_EXECUTABLE;

  vm_frame_ctx_t *new_frame_ctx_p = &(executable_object_p->frame_ctx);
  *new_frame_ctx_p = *frame_ctx_p;
  new_frame_ctx_p->shared_p = new_shared_p;

  /* The old register values are discarded. */
  ecma_value_t *new_registers_p = VM_GET_REGISTERS (new_frame_ctx_p);
  memcpy (new_registers_p, VM_GET_REGISTERS (frame_ctx_p), size);

  size_t stack_top = (size_t) (frame_ctx_p->stack_top_p - VM_GET_REGISTERS (frame_ctx_p));
  ecma_value_t *new_stack_top_p = new_registers_p + stack_top;

  new_frame_ctx_p->stack_top_p = new_stack_top_p;

  /* Initial state is "not running", so all object references are released. */

  if (frame_ctx_p->context_depth > 0)
  {
    JERRY_ASSERT (type != VM_CREATE_EXECUTABLE_OBJECT_GENERATOR);

    ecma_value_t *register_end_p = new_registers_p + register_end;

    JERRY_ASSERT (register_end_p <= new_stack_top_p);

    while (new_registers_p < register_end_p)
    {
      ecma_deref_if_object (*new_registers_p++);
    }

    vm_ref_lex_env_chain (frame_ctx_p->lex_env_p, frame_ctx_p->context_depth, new_registers_p, false);

    new_registers_p += frame_ctx_p->context_depth;

    JERRY_ASSERT (new_registers_p <= new_stack_top_p);
  }

  while (new_registers_p < new_stack_top_p)
  {
    ecma_deref_if_object (*new_registers_p++);
  }

  JERRY_ASSERT (*VM_GET_EXECUTABLE_ITERATOR (new_frame_ctx_p) == ECMA_VALUE_UNDEFINED);

  new_frame_ctx_p->this_binding = ecma_copy_value_if_not_object (new_frame_ctx_p->this_binding);

  JERRY_CONTEXT (vm_top_context_p) = new_frame_ctx_p->prev_context_p;

  return executable_object_p;
} /* opfunc_create_executable_object */

/**
 * Byte code which resumes an executable object with throw
 */
const uint8_t opfunc_resume_executable_object_with_throw[1] = { CBC_THROW };

/**
 * Byte code which resumes an executable object with return
 */
const uint8_t opfunc_resume_executable_object_with_return[2] = { CBC_EXT_OPCODE, CBC_EXT_RETURN };

/**
 * Resume the execution of an inactive executable object
 *
 * @return value provided by the execution
 */
ecma_value_t
opfunc_resume_executable_object (vm_executable_object_t *executable_object_p, /**< executable object */
                                 ecma_value_t value) /**< value pushed onto the stack (takes the reference) */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  const ecma_compiled_code_t *bytecode_header_p = executable_object_p->shared.bytecode_header_p;
  ecma_value_t *register_p = VM_GET_REGISTERS (&executable_object_p->frame_ctx);
  ecma_value_t *register_end_p;

  if (bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
  {
    cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_header_p;
    register_end_p = register_p + args_p->register_end;
  }
  else
  {
    cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_header_p;
    register_end_p = register_p + args_p->register_end;
  }

  ecma_value_t *stack_top_p = executable_object_p->frame_ctx.stack_top_p;

  if (value != ECMA_VALUE_EMPTY)
  {
    *stack_top_p = value;
    executable_object_p->frame_ctx.stack_top_p = stack_top_p + 1;
  }

  if (executable_object_p->frame_ctx.context_depth > 0)
  {
    while (register_p < register_end_p)
    {
      ecma_ref_if_object (*register_p++);
    }

    vm_ref_lex_env_chain (executable_object_p->frame_ctx.lex_env_p,
                          executable_object_p->frame_ctx.context_depth,
                          register_p,
                          true);

    register_p += executable_object_p->frame_ctx.context_depth;
  }

  while (register_p < stack_top_p)
  {
    ecma_ref_if_object (*register_p++);
  }

  ecma_ref_if_object (executable_object_p->iterator);

  JERRY_ASSERT (ECMA_EXECUTABLE_OBJECT_IS_SUSPENDED (executable_object_p));

  executable_object_p->extended_object.u.cls.u2.executable_obj_flags |= ECMA_EXECUTABLE_OBJECT_RUNNING;

  executable_object_p->frame_ctx.prev_context_p = JERRY_CONTEXT (vm_top_context_p);
  JERRY_CONTEXT (vm_top_context_p) = &executable_object_p->frame_ctx;

  /* inside the generators the "new.target" is always "undefined" as it can't be invoked with "new" */
  ecma_object_t *old_new_target = JERRY_CONTEXT (current_new_target_p);
  JERRY_CONTEXT (current_new_target_p) = NULL;

#if JERRY_BUILTIN_REALMS
  ecma_global_object_t *saved_global_object_p = JERRY_CONTEXT (global_object_p);
  JERRY_CONTEXT (global_object_p) = ecma_op_function_get_realm (bytecode_header_p);
#endif /* JERRY_BUILTIN_REALMS */

  ecma_value_t result = vm_execute (&executable_object_p->frame_ctx);

#if JERRY_BUILTIN_REALMS
  JERRY_CONTEXT (global_object_p) = saved_global_object_p;
#endif /* JERRY_BUILTIN_REALMS */

  JERRY_CONTEXT (current_new_target_p) = old_new_target;
  executable_object_p->extended_object.u.cls.u2.executable_obj_flags &= (uint8_t) ~ECMA_EXECUTABLE_OBJECT_RUNNING;

  if (executable_object_p->frame_ctx.call_operation != VM_EXEC_RETURN)
  {
    JERRY_ASSERT (executable_object_p->frame_ctx.call_operation == VM_NO_EXEC_OP);

    /* All resources are released. */
    executable_object_p->extended_object.u.cls.u2.executable_obj_flags |= ECMA_EXECUTABLE_OBJECT_COMPLETED;
    return result;
  }

  JERRY_CONTEXT (vm_top_context_p) = executable_object_p->frame_ctx.prev_context_p;

  register_p = VM_GET_REGISTERS (&executable_object_p->frame_ctx);
  stack_top_p = executable_object_p->frame_ctx.stack_top_p;

  if (executable_object_p->frame_ctx.context_depth > 0)
  {
    while (register_p < register_end_p)
    {
      ecma_deref_if_object (*register_p++);
    }

    vm_ref_lex_env_chain (executable_object_p->frame_ctx.lex_env_p,
                          executable_object_p->frame_ctx.context_depth,
                          register_p,
                          false);

    register_p += executable_object_p->frame_ctx.context_depth;
  }

  while (register_p < stack_top_p)
  {
    ecma_deref_if_object (*register_p++);
  }

  ecma_deref_if_object (executable_object_p->iterator);

  return result;
} /* opfunc_resume_executable_object */

/**
 * Fulfill the next promise of the async generator with the value
 */
void
opfunc_async_generator_yield (ecma_extended_object_t *async_generator_object_p, /**< async generator object */
                              ecma_value_t value) /**< value (takes the reference) */
{
  ecma_async_generator_task_t *task_p;
  task_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_async_generator_task_t, async_generator_object_p->u.cls.u3.head);

  ecma_value_t iter_result = ecma_create_iter_result_object (value, ECMA_VALUE_FALSE);
  ecma_fulfill_promise (task_p->promise, iter_result);

  ecma_free_value (iter_result);
  ecma_free_value (value);

  ecma_value_t next = task_p->next;
  async_generator_object_p->u.cls.u3.head = next;

  JERRY_ASSERT (task_p->operation_value == ECMA_VALUE_UNDEFINED);
  jmem_heap_free_block (task_p, sizeof (ecma_async_generator_task_t));

  if (!ECMA_IS_INTERNAL_VALUE_NULL (next))
  {
    ecma_value_t executable_object = ecma_make_object_value ((ecma_object_t *) async_generator_object_p);
    ecma_enqueue_promise_async_generator_job (executable_object);
  }
} /* opfunc_async_generator_yield */

/**
 * Creates a new executable object and awaits for the value
 *
 * Note:
 *   extra_flags can be used to set additional extra_info flags
 *
 * @return a new Promise object on success, error otherwise
 */
ecma_value_t
opfunc_async_create_and_await (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                               ecma_value_t value, /**< awaited value (takes reference) */
                               uint16_t extra_flags) /**< extra flags */
{
  JERRY_ASSERT (
    CBC_FUNCTION_GET_TYPE (frame_ctx_p->shared_p->bytecode_header_p->status_flags) == CBC_FUNCTION_ASYNC
    || (CBC_FUNCTION_GET_TYPE (frame_ctx_p->shared_p->bytecode_header_p->status_flags) == CBC_FUNCTION_ASYNC_ARROW));

  ecma_object_t *promise_p = ecma_builtin_get (ECMA_BUILTIN_ID_PROMISE);
  ecma_value_t result = ecma_promise_reject_or_resolve (ecma_make_object_value (promise_p), value, true);
  ecma_free_value (value);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  vm_executable_object_t *executable_object_p;
  executable_object_p = opfunc_create_executable_object (frame_ctx_p, VM_CREATE_EXECUTABLE_OBJECT_ASYNC);

  executable_object_p->extended_object.u.cls.u2.executable_obj_flags |= extra_flags;

  ecma_promise_async_then (result, ecma_make_object_value ((ecma_object_t *) executable_object_p));
  ecma_deref_object ((ecma_object_t *) executable_object_p);
  ecma_free_value (result);

  result = ecma_op_create_promise_object (ECMA_VALUE_EMPTY, ECMA_VALUE_UNDEFINED, promise_p);

  JERRY_ASSERT (ecma_is_value_object (result));
  executable_object_p->iterator = result;

  return result;
} /* opfunc_async_create_and_await */

/**
 * PrivateMethodOrAccessorAdd abstract operation.
 *
 * See also: ECMAScript v12, 7.3.29.
 *
 * @return ECMA_VALUE_ERROR - initialization fails
 *         ECMA_VALUE_UNDEFINED - otherwise
 */
static ecma_value_t
opfunc_private_method_or_accessor_add (ecma_object_t *class_object_p, /**< the function itself */
                                       ecma_object_t *this_obj_p, /**< this object */
                                       uint32_t static_flag) /**< static_flag */
{
  ecma_string_t *internal_string_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS);
  ecma_property_t *prop_p = ecma_find_named_property (class_object_p, internal_string_p);

  if (prop_p == NULL)
  {
    return ECMA_VALUE_UNDEFINED;
  }

  ecma_value_t *collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, ECMA_PROPERTY_VALUE_PTR (prop_p)->value);
  ecma_value_t *current_p = collection_p + 1;
  ecma_value_t *end_p = ecma_compact_collection_end (collection_p);

  while (current_p < end_p)
  {
    uint32_t prop_desc = *current_p++;
    ecma_private_property_kind_t kind = ECMA_PRIVATE_PROPERTY_KIND (prop_desc);

    if ((prop_desc & ECMA_PRIVATE_PROPERTY_STATIC_FLAG) != static_flag || kind == ECMA_PRIVATE_FIELD)
    {
      current_p += 2;
      continue;
    }

    ecma_string_t *prop_name_p = ecma_get_symbol_from_value (*current_p++);
    ecma_value_t method = *current_p++;

    JERRY_ASSERT (prop_name_p->u.hash & ECMA_SYMBOL_FLAG_PRIVATE_INSTANCE_METHOD);

    prop_p = ecma_find_named_property (this_obj_p, prop_name_p);
    ecma_object_t *method_p = ecma_get_object_from_value (method);

    if (kind == ECMA_PRIVATE_METHOD)
    {
      if (prop_p != NULL)
      {
        return ecma_raise_type_error (ECMA_ERR_CANNOT_DECLARE_SAME_PRIVATE_FIELD_TWICE);
      }

      ecma_property_value_t *prop_value_p =
        ecma_create_named_data_property (this_obj_p, prop_name_p, ECMA_PROPERTY_FIXED, NULL);
      prop_value_p->value = method;
      continue;
    }

    if (prop_p == NULL)
    {
      ecma_object_t *getter_p = (kind == ECMA_PRIVATE_GETTER) ? method_p : NULL;
      ecma_object_t *setter_p = (kind == ECMA_PRIVATE_SETTER) ? method_p : NULL;
      ecma_create_named_accessor_property (this_obj_p, prop_name_p, getter_p, setter_p, ECMA_PROPERTY_FIXED, NULL);
      continue;
    }

    ecma_property_value_t *accessor_objs_p = ECMA_PROPERTY_VALUE_PTR (prop_p);
    ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (accessor_objs_p);

    if (kind == ECMA_PRIVATE_GETTER)
    {
      ECMA_SET_POINTER (get_set_pair_p->getter_cp, method_p);
    }
    else
    {
      JERRY_ASSERT (kind == ECMA_PRIVATE_SETTER);
      ECMA_SET_POINTER (get_set_pair_p->setter_cp, method_p);
    }
  }

  return ECMA_VALUE_UNDEFINED;
} /* opfunc_private_method_or_accessor_add */

/**
 * DefineField abstract operation.
 *
 * See also: ECMAScript v12, 7.3.32.
 *
 * @return ECMA_VALUE_ERROR - operation fails
 *         ECMA_VALUE_{TRUE/FALSE} - otherwise
 */
ecma_value_t
opfunc_define_field (ecma_value_t base, /**< base */
                     ecma_value_t property, /**< property */
                     ecma_value_t value) /**< value */
{
  ecma_string_t *property_key_p = ecma_op_to_property_key (property);

  JERRY_ASSERT (property_key_p != NULL);
  ecma_object_t *obj_p = ecma_get_object_from_value (base);
  ecma_property_descriptor_t desc = ecma_make_empty_property_descriptor ();
  desc.value = value;
  desc.flags = (JERRY_PROP_IS_WRITABLE | JERRY_PROP_IS_WRITABLE_DEFINED | JERRY_PROP_IS_ENUMERABLE
                | JERRY_PROP_IS_ENUMERABLE_DEFINED | JERRY_PROP_IS_CONFIGURABLE | JERRY_PROP_IS_CONFIGURABLE_DEFINED
                | JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_SHOULD_THROW);

  ecma_value_t result = ecma_op_object_define_own_property (obj_p, property_key_p, &desc);
  ecma_deref_ecma_string (property_key_p);

  return result;
} /* opfunc_define_field */

/**
 * Initialize class fields.
 *
 * @return ECMA_VALUE_ERROR - initialization fails
 *         ECMA_VALUE_UNDEFINED - otherwise
 */
ecma_value_t
opfunc_init_class_fields (ecma_object_t *class_object_p, /**< the function itself */
                          ecma_value_t this_val) /**< this_arg of the function */
{
  JERRY_ASSERT (ecma_is_value_object (this_val));
  ecma_object_t *this_obj_p = ecma_get_object_from_value (this_val);

  ecma_value_t result = opfunc_private_method_or_accessor_add (class_object_p, this_obj_p, 0);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  ecma_string_t *name_p = ecma_get_magic_string (LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_INIT);
  ecma_property_t *property_p = ecma_find_named_property (class_object_p, name_p);

  if (property_p == NULL)
  {
    return ECMA_VALUE_UNDEFINED;
  }

  vm_frame_ctx_shared_class_fields_t shared_class_fields;
  shared_class_fields.header.status_flags = VM_FRAME_CTX_SHARED_HAS_CLASS_FIELDS;
  shared_class_fields.computed_class_fields_p = NULL;

  name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_COMPUTED);
  ecma_property_t *class_field_property_p = ecma_find_named_property (class_object_p, name_p);

  if (class_field_property_p != NULL)
  {
    ecma_value_t value = ECMA_PROPERTY_VALUE_PTR (class_field_property_p)->value;
    shared_class_fields.computed_class_fields_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, value);
  }

  ecma_property_value_t *property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
  JERRY_ASSERT (ecma_op_is_callable (property_value_p->value));

  ecma_extended_object_t *ext_function_p;
  ext_function_p = (ecma_extended_object_t *) ecma_get_object_from_value (property_value_p->value);
  shared_class_fields.header.bytecode_header_p = ecma_op_function_get_compiled_code (ext_function_p);
  shared_class_fields.header.function_object_p = &ext_function_p->object;

  ecma_object_t *scope_p =
    ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, ext_function_p->u.function.scope_cp);

  result = vm_run (&shared_class_fields.header, this_val, scope_p);

  JERRY_ASSERT (ECMA_IS_VALUE_ERROR (result) || result == ECMA_VALUE_UNDEFINED);
  return result;
} /* opfunc_init_class_fields */

/**
 * Initialize static class fields.
 *
 * @return ECMA_VALUE_ERROR - initialization fails
 *         ECMA_VALUE_UNDEFINED - otherwise
 */
ecma_value_t
opfunc_init_static_class_fields (ecma_value_t function_object, /**< the function itself */
                                 ecma_value_t this_val) /**< this_arg of the function */
{
  JERRY_ASSERT (ecma_op_is_callable (function_object));
  JERRY_ASSERT (ecma_is_value_object (this_val));

  ecma_string_t *name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_COMPUTED);
  ecma_object_t *function_object_p = ecma_get_object_from_value (function_object);
  ecma_property_t *class_field_property_p = ecma_find_named_property (function_object_p, name_p);

  vm_frame_ctx_shared_class_fields_t shared_class_fields;
  shared_class_fields.header.function_object_p = function_object_p;
  shared_class_fields.header.status_flags = VM_FRAME_CTX_SHARED_HAS_CLASS_FIELDS;
  shared_class_fields.computed_class_fields_p = NULL;

  if (class_field_property_p != NULL)
  {
    ecma_value_t value = ECMA_PROPERTY_VALUE_PTR (class_field_property_p)->value;
    shared_class_fields.computed_class_fields_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, value);
  }

  ecma_extended_object_t *ext_function_p = (ecma_extended_object_t *) function_object_p;
  shared_class_fields.header.bytecode_header_p = ecma_op_function_get_compiled_code (ext_function_p);

  ecma_object_t *scope_p =
    ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, ext_function_p->u.function.scope_cp);

  ecma_value_t result = vm_run (&shared_class_fields.header, this_val, scope_p);

  JERRY_ASSERT (ECMA_IS_VALUE_ERROR (result) || result == ECMA_VALUE_UNDEFINED);
  return result;
} /* opfunc_init_static_class_fields */

/**
 * Add the name of a computed field to a name list
 *
 * @return ECMA_VALUE_ERROR - name is not a valid property name
 *         ECMA_VALUE_UNDEFINED - otherwise
 */
ecma_value_t
opfunc_add_computed_field (ecma_value_t class_object, /**< class object */
                           ecma_value_t name) /**< name of the property */
{
  ecma_string_t *prop_name_p = ecma_op_to_property_key (name);

  if (JERRY_UNLIKELY (prop_name_p == NULL))
  {
    return ECMA_VALUE_ERROR;
  }

  if (ecma_prop_name_is_symbol (prop_name_p))
  {
    name = ecma_make_symbol_value (prop_name_p);
  }
  else
  {
    name = ecma_make_string_value (prop_name_p);
  }

  ecma_string_t *name_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_COMPUTED);
  ecma_object_t *class_object_p = ecma_get_object_from_value (class_object);

  ecma_property_t *property_p = ecma_find_named_property (class_object_p, name_p);
  ecma_value_t *compact_collection_p;
  ecma_property_value_t *property_value_p;

  if (property_p == NULL)
  {
    ECMA_CREATE_INTERNAL_PROPERTY (class_object_p, name_p, property_p, property_value_p);
    compact_collection_p = ecma_new_compact_collection ();
  }
  else
  {
    property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);
    compact_collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, property_value_p->value);
  }

  compact_collection_p = ecma_compact_collection_push_back (compact_collection_p, name);
  ECMA_SET_INTERNAL_VALUE_POINTER (property_value_p->value, compact_collection_p);
  return ECMA_VALUE_UNDEFINED;
} /* opfunc_add_computed_field */

/**
 * Create implicit class constructor
 *
 * See also: ECMAScript v6, 14.5.14
 *
 * @return - new external function ecma-object
 */
ecma_value_t
opfunc_create_implicit_class_constructor (uint8_t opcode, /**< current cbc opcode */
                                          const ecma_compiled_code_t *bytecode_p) /**< current byte code */
{
  /* 8. */
  ecma_value_t script_value = ((cbc_uint8_arguments_t *) bytecode_p)->script_value;
  cbc_script_t *script_p = ECMA_GET_INTERNAL_VALUE_POINTER (cbc_script_t, script_value);

  if (JERRY_UNLIKELY (script_p->refs_and_type >= CBC_SCRIPT_REF_MAX))
  {
    jerry_fatal (JERRY_FATAL_REF_COUNT_LIMIT);
  }

  ecma_object_t *function_object_p = ecma_create_object (ecma_builtin_get (ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE),
                                                         sizeof (ecma_extended_object_t),
                                                         ECMA_OBJECT_TYPE_CONSTRUCTOR_FUNCTION);

  ecma_extended_object_t *constructor_object_p = (ecma_extended_object_t *) function_object_p;

  script_p->refs_and_type += CBC_SCRIPT_REF_ONE;
  constructor_object_p->u.constructor_function.script_value = script_value;
  constructor_object_p->u.constructor_function.flags = 0;

  /* 10.a.i */
  if (opcode == CBC_EXT_PUSH_IMPLICIT_CONSTRUCTOR_HERITAGE)
  {
    constructor_object_p->u.constructor_function.flags |= ECMA_CONSTRUCTOR_FUNCTION_HAS_HERITAGE;
  }

  ecma_property_value_t *prop_value_p;
  prop_value_p = ecma_create_named_data_property (function_object_p,
                                                  ecma_get_magic_string (LIT_MAGIC_STRING_LENGTH),
                                                  ECMA_PROPERTY_FLAG_CONFIGURABLE,
                                                  NULL);
  prop_value_p->value = ecma_make_uint32_value (0);

  return ecma_make_object_value (function_object_p);
} /* opfunc_create_implicit_class_constructor */

/**
 * Set the [[HomeObject]] attribute of the given function object
 *
 * @return void
 */
void
opfunc_set_home_object (ecma_object_t *func_p, /**< function object */
                        ecma_object_t *parent_env_p) /**< parent environment */
{
  JERRY_ASSERT (ecma_is_lexical_environment (parent_env_p));

  if (ecma_get_object_type (func_p) == ECMA_OBJECT_TYPE_FUNCTION)
  {
    ecma_extended_object_t *ext_func_p = (ecma_extended_object_t *) func_p;
    ECMA_SET_NON_NULL_POINTER_TAG (ext_func_p->u.function.scope_cp,
                                   parent_env_p,
                                   JMEM_CP_GET_POINTER_TAG_BITS (ext_func_p->u.function.scope_cp));
  }
} /* opfunc_set_home_object */

/**
 * Make private key from descriptor
 *
 * @return pointer to private key
 */
ecma_string_t *
opfunc_make_private_key (ecma_value_t descriptor) /**< descriptor */
{
  ecma_string_t *private_key_p = ecma_new_symbol_from_descriptor_string (descriptor);
  private_key_p->u.hash |= ECMA_SYMBOL_FLAG_PRIVATE_KEY;

  return (ecma_string_t *) private_key_p;
} /* opfunc_make_private_key */

/**
 * Find a private property in the private elements internal property given the key
 *
 * @return pointer to the private property - if it is found,
 *         NULL - otherwise
 */
static ecma_property_t *
opfunc_find_private_key (ecma_object_t *class_object_p, /**< class environment */
                         ecma_object_t *obj_p, /**< object */
                         ecma_string_t *search_key_p, /**< key */
                         ecma_string_t **out_private_key_p) /**< [out] private key */
{
  if (ecma_op_object_is_fast_array (obj_p))
  {
    return NULL;
  }

  ecma_string_t *internal_string_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS);
  ecma_property_t *prop_p = ecma_find_named_property (class_object_p, internal_string_p);

  if (prop_p == NULL)
  {
    return NULL;
  }

  ecma_value_t *collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, ECMA_PROPERTY_VALUE_PTR (prop_p)->value);
  ecma_value_t *current_p = collection_p + 1;
  ecma_value_t *end_p = ecma_compact_collection_end (collection_p);

  while (current_p < end_p)
  {
    current_p++; /* skip kind */
    ecma_string_t *private_key_p = ecma_get_prop_name_from_value (*current_p++);
    current_p++; /* skip value */

    JERRY_ASSERT (ecma_prop_name_is_symbol (private_key_p));

    ecma_string_t *private_key_desc_p =
      ecma_get_string_from_value (((ecma_extended_string_t *) private_key_p)->u.symbol_descriptor);

    if (ecma_compare_ecma_strings (private_key_desc_p, search_key_p))
    {
      prop_p = ecma_find_named_property (obj_p, private_key_p);

      if (out_private_key_p)
      {
        *out_private_key_p = private_key_p;
      }

      return prop_p;
    }
  }

  return NULL;
} /* opfunc_find_private_key */

/**
 * PrivateElementFind abstract operation
 *
 * See also: ECMAScript v12, 7.3.27
 *
 * @return - ECMA_VALUE_ERROR - if the operation fails
 *           ECMA_VALUE_EMPTY - otherwise
 */
static ecma_property_t *
opfunc_find_private_element (ecma_object_t *obj_p, /**< object */
                             ecma_string_t *key_p, /**< key */
                             ecma_string_t **private_key_p, /**< [out] private key */
                             bool allow_heritage) /**< heritage flag */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (private_key_p != NULL);
  JERRY_ASSERT (*private_key_p == NULL);
  ecma_object_t *lex_env_p = JERRY_CONTEXT (vm_top_context_p)->lex_env_p;

  while (true)
  {
    JERRY_ASSERT (lex_env_p != NULL);

    if (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_CLASS
        && (lex_env_p->type_flags_refs & ECMA_OBJECT_FLAG_LEXICAL_ENV_HAS_DATA) != 0
        && !ECMA_LEX_ENV_CLASS_IS_MODULE (lex_env_p))
    {
      ecma_object_t *class_object_p = ((ecma_lexical_environment_class_t *) lex_env_p)->object_p;

      ecma_property_t *prop_p = opfunc_find_private_key (class_object_p, obj_p, key_p, private_key_p);

      if (prop_p || *private_key_p != NULL)
      {
        /* Found non shadowed property */
        return prop_p;
      }

      if (!allow_heritage)
      {
        return NULL;
      }
    }

    if (lex_env_p->u2.outer_reference_cp == JMEM_CP_NULL)
    {
      break;
    }

    lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
  }

  return NULL;
} /* opfunc_find_private_element */

/**
 * In expression runtime evaluation in case of private identifiers
 *
 * See also: ECMAScript v12, 13.10.1
 *
 * @return - ECMA_VALUE_ERROR - if the operation fails
 *           ECMA_VALUE_TRUE - if the property was found in the base object
 *           ECMA_VALUE_FALSE - otherwise
 */
ecma_value_t
opfunc_private_in (ecma_value_t base, /**< base */
                   ecma_value_t property) /**< property */
{
  if (!ecma_is_value_object (base))
  {
    return ecma_raise_type_error (ECMA_ERR_RIGHT_VALUE_OF_IN_MUST_BE_AN_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (base);
  ecma_string_t *prop_name_p = ecma_get_prop_name_from_value (property);
  ecma_string_t *private_key_p = NULL;

  ecma_property_t *prop_p = opfunc_find_private_element (obj_p, prop_name_p, &private_key_p, false);

  return ecma_make_boolean_value (prop_p != NULL);
} /* opfunc_private_in */

/**
 * PrivateFieldAdd abstract operation
 *
 * See also: ECMAScript v12, 7.3.28
 *
 * @return - ECMA_VALUE_ERROR - if the operation fails
 *           ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
opfunc_private_field_add (ecma_value_t base, /**< base object */
                          ecma_value_t property, /**< property name */
                          ecma_value_t value) /**< ecma value */
{
  ecma_object_t *obj_p = ecma_get_object_from_value (base);
  ecma_string_t *prop_name_p = ecma_get_string_from_value (property);
  ecma_string_t *private_key_p = NULL;

  ecma_property_t *prop_p = opfunc_find_private_element (obj_p, prop_name_p, &private_key_p, false);

  if (prop_p != NULL)
  {
    return ecma_raise_type_error (ECMA_ERR_CANNOT_DECLARE_SAME_PRIVATE_FIELD_TWICE);
  }

  ecma_property_value_t *value_p =
    ecma_create_named_data_property (obj_p, private_key_p, ECMA_PROPERTY_FLAG_WRITABLE, NULL);

  value_p->value = ecma_copy_value_if_not_object (value);

  return ECMA_VALUE_EMPTY;
} /* opfunc_private_field_add */

/**
 * PrivateSet abstract operation
 *
 * See also: ECMAScript v12, 7.3.31
 *
 * @return - ECMA_VALUE_ERROR - if the operation fails
 *           ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
opfunc_private_set (ecma_value_t base, /**< this object */
                    ecma_value_t property, /**< property name */
                    ecma_value_t value) /**< ecma value */
{
  ecma_value_t base_obj = ecma_op_to_object (base);

  if (ECMA_IS_VALUE_ERROR (base_obj))
  {
    return base_obj;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (base_obj);
  ecma_string_t *prop_name_p = ecma_get_string_from_value (property);
  ecma_string_t *private_key_p = NULL;

  ecma_property_t *prop_p = opfunc_find_private_element (obj_p, prop_name_p, &private_key_p, true);

  ecma_value_t result;

  if (prop_p == NULL)
  {
    result = ecma_raise_type_error (ECMA_ERR_CANNOT_WRITE_PRIVATE_MEMBER_TO_AN_OBJECT_WHOSE_CLASS_DID_NOT_DECLARE_IT);
  }
  else if (*prop_p & ECMA_PROPERTY_FLAG_DATA)
  {
    JERRY_ASSERT (ecma_prop_name_is_symbol (private_key_p));

    if (private_key_p->u.hash & ECMA_SYMBOL_FLAG_PRIVATE_INSTANCE_METHOD)
    {
      result = ecma_raise_type_error (ECMA_ERR_PRIVATE_METHOD_IS_NOT_WRITABLE);
    }
    else
    {
      ecma_value_assign_value (&ECMA_PROPERTY_VALUE_PTR (prop_p)->value, value);
      result = ecma_copy_value (value);
    }
  }
  else
  {
    ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (ECMA_PROPERTY_VALUE_PTR (prop_p));

    if (get_set_pair_p->setter_cp == JMEM_CP_NULL)
    {
      result = ecma_raise_type_error (ECMA_ERR_PRIVATE_FIELD_WAS_DEFINED_WITHOUT_A_SETTER);
    }
    else
    {
      ecma_object_t *setter_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->setter_cp);

      result = ecma_op_function_call (setter_p, base, &value, 1);
    }
  }

  ecma_deref_object (obj_p);

  return result;
} /* opfunc_private_set */

/**
 * PrivateGet abstract operation
 *
 * See also: ECMAScript v12, 7.3.30
 *
 * @return - ECMA_VALUE_ERROR - if the operation fails
 *           private property value - otherwise
 */
ecma_value_t
opfunc_private_get (ecma_value_t base, /**< this object */
                    ecma_value_t property) /**< property name */
{
  ecma_value_t base_obj = ecma_op_to_object (base);

  if (ECMA_IS_VALUE_ERROR (base_obj))
  {
    return base_obj;
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (base_obj);
  ecma_string_t *prop_name_p = ecma_get_string_from_value (property);
  ecma_string_t *private_key_p = NULL;

  ecma_property_t *prop_p = opfunc_find_private_element (obj_p, prop_name_p, &private_key_p, true);

  ecma_value_t result;

  if (prop_p == NULL)
  {
    result = ecma_raise_type_error (ECMA_ERR_CANNOT_READ_PRIVATE_MEMBER_TO_AN_OBJECT_WHOSE_CLASS_DID_NOT_DECLARE_IT);
  }
  else if (*prop_p & ECMA_PROPERTY_FLAG_DATA)
  {
    result = ecma_copy_value (ECMA_PROPERTY_VALUE_PTR (prop_p)->value);
  }
  else
  {
    ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (ECMA_PROPERTY_VALUE_PTR (prop_p));

    if (get_set_pair_p->getter_cp == JMEM_CP_NULL)
    {
      result = ecma_raise_type_error (ECMA_ERR_PRIVATE_FIELD_WAS_DEFINED_WITHOUT_A_GETTER);
    }
    else
    {
      ecma_object_t *getter_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->getter_cp);
      result = ecma_op_function_call (getter_p, base, NULL, 0);
    }
  }

  ecma_deref_object (obj_p);

  return result;
} /* opfunc_private_get */

/**
 * Find the private property in the object who's private key descriptor matches the given key
 *
 * @return pointer to private key
 */
static ecma_string_t *
opfunc_create_private_key (ecma_value_t *collection_p, /**< collection of private properties */
                           ecma_value_t search_key, /**< key */
                           ecma_private_property_kind_t search_kind) /**< kind of the property */
{
  if (search_kind < ECMA_PRIVATE_GETTER)
  {
    return opfunc_make_private_key (search_key);
  }

  ecma_string_t *search_key_p = ecma_get_string_from_value (search_key);

  ecma_value_t *current_p = collection_p + 1;
  ecma_value_t *end_p = ecma_compact_collection_end (collection_p);

  while (current_p < end_p)
  {
    ecma_private_property_kind_t kind = ECMA_PRIVATE_PROPERTY_KIND (*current_p++);
    ecma_string_t *private_key_p = ecma_get_prop_name_from_value (*current_p++);
    current_p++; /* skip value */

    if (kind < ECMA_PRIVATE_GETTER)
    {
      continue;
    }

    JERRY_ASSERT (ecma_prop_name_is_symbol (private_key_p));

    ecma_string_t *private_key_desc_p =
      ecma_get_string_from_value (((ecma_extended_string_t *) private_key_p)->u.symbol_descriptor);

    if (ecma_compare_ecma_strings (private_key_desc_p, search_key_p))
    {
      ecma_deref_ecma_string (search_key_p);
      ecma_ref_ecma_string (private_key_p);
      return private_key_p;
    }
  }

  return opfunc_make_private_key (search_key);
} /* opfunc_create_private_key */

/**
 * Collect private members for PrivateMethodOrAccessorAdd and PrivateFieldAdd abstract operations
 */
void
opfunc_collect_private_properties (ecma_value_t constructor, /**< constructor */
                                   ecma_value_t prop_name, /**< property name */
                                   ecma_value_t value, /**< value */
                                   uint8_t opcode) /**< opcode */
{
  ecma_private_property_kind_t kind = ECMA_PRIVATE_FIELD;
  bool is_static = false;

  if (opcode >= CBC_EXT_COLLECT_PRIVATE_STATIC_FIELD)
  {
    opcode = (uint8_t) (opcode - PARSER_STATIC_PRIVATE_TO_PRIVATE_OFFSET);
    is_static = true;
  }

  if (opcode == CBC_EXT_COLLECT_PRIVATE_METHOD)
  {
    prop_name ^= value;
    value ^= prop_name;
    prop_name ^= value;
    kind = ECMA_PRIVATE_METHOD;
  }
  else if (opcode == CBC_EXT_COLLECT_PRIVATE_GETTER)
  {
    kind = ECMA_PRIVATE_GETTER;
  }
  else if (opcode == CBC_EXT_COLLECT_PRIVATE_SETTER)
  {
    kind = ECMA_PRIVATE_SETTER;
  }

  JERRY_ASSERT (ecma_is_value_object (constructor));
  JERRY_ASSERT (ecma_is_value_string (prop_name));
  JERRY_ASSERT (ecma_is_value_object (value) || ecma_is_value_undefined (value));

  ecma_object_t *constructor_p = ecma_get_object_from_value (constructor);
  ecma_string_t *internal_string_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS);
  ecma_property_t *prop_p = ecma_find_named_property (constructor_p, internal_string_p);
  ecma_value_t *collection_p;
  ecma_property_value_t *prop_value_p;

  if (prop_p == NULL)
  {
    collection_p = ecma_new_compact_collection ();
    ECMA_CREATE_INTERNAL_PROPERTY (constructor_p, internal_string_p, prop_p, prop_value_p);
    ECMA_SET_INTERNAL_VALUE_POINTER (prop_value_p->value, collection_p);
  }
  else
  {
    prop_value_p = ECMA_PROPERTY_VALUE_PTR (prop_p);
    collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, prop_value_p->value);
  }

  ecma_string_t *key_p = opfunc_create_private_key (collection_p, prop_name, kind);

  if (kind != ECMA_PRIVATE_FIELD)
  {
    key_p->u.hash |= ECMA_SYMBOL_FLAG_PRIVATE_INSTANCE_METHOD;
  }

  if (is_static)
  {
    kind = (ecma_private_property_kind_t) ((int) kind | (int) ECMA_PRIVATE_PROPERTY_STATIC_FLAG);
  }

  collection_p = ecma_compact_collection_push_back (collection_p, (ecma_value_t) kind);
  collection_p = ecma_compact_collection_push_back (collection_p, ecma_make_symbol_value (key_p));
  collection_p = ecma_compact_collection_push_back (collection_p, value);

#ifndef JERRY_NDEBUG
  ecma_value_t *end_p = ecma_compact_collection_end (collection_p);
  ecma_value_t *current_p = collection_p + 1;

  JERRY_ASSERT ((end_p - current_p) % ECMA_PRIVATE_ELEMENT_LIST_SIZE == 0);
#endif /* !defined (JERRY_NDEBUG) */

  ECMA_SET_INTERNAL_VALUE_POINTER (prop_value_p->value, collection_p);

  ecma_free_value (value);
} /* opfunc_collect_private_properties */

/**
 * ClassDefinitionEvaluation environment initialization part
 *
 * See also: ECMAScript v6, 14.5.14
 */
void
opfunc_push_class_environment (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                               ecma_value_t **vm_stack_top, /**< VM stack top */
                               ecma_value_t class_name) /**< class name */
{
  JERRY_ASSERT (ecma_is_value_string (class_name));
  ecma_object_t *class_env_p = ecma_create_decl_lex_env (frame_ctx_p->lex_env_p);

  /* 4.a */
  ecma_property_value_t *property_value_p;
  property_value_p = ecma_create_named_data_property (class_env_p,
                                                      ecma_get_string_from_value (class_name),
                                                      ECMA_PROPERTY_FLAG_ENUMERABLE,
                                                      NULL);

  property_value_p->value = ECMA_VALUE_UNINITIALIZED;
  frame_ctx_p->lex_env_p = class_env_p;

  *(*vm_stack_top)++ = ECMA_VALUE_RELEASE_LEX_ENV;
} /* opfunc_push_class_environment */

/**
 * ClassDefinitionEvaluation object initialization part
 *
 * See also: ECMAScript v6, 14.5.14
 *
 * @return - ECMA_VALUE_ERROR - if the operation fails
 *           ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
opfunc_init_class (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                   ecma_value_t *stack_top_p) /**< stack top */
{
  /* 5.b, 6.e.ii */
  ecma_object_t *ctor_parent_p = ecma_builtin_get (ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE);
  ecma_object_t *proto_parent_p = NULL;
  bool free_proto_parent = false;

  ecma_value_t super_class = stack_top_p[-2];
  ecma_object_t *ctor_p = ecma_get_object_from_value (stack_top_p[-1]);

  bool heritage_present = !ecma_is_value_array_hole (super_class);

  /* 5. ClassHeritage opt is not present */
  if (!heritage_present)
  {
    /* 5.a */
    proto_parent_p = ecma_builtin_get (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE);
  }
  else if (!ecma_is_value_null (super_class))
  {
    /* 6.f, 6.g.i */
    if (!ecma_is_constructor (super_class))
    {
      return ecma_raise_type_error (ECMA_ERR_CLASS_EXTENDS_NOT_CONSTRUCTOR);
    }

    ecma_object_t *parent_p = ecma_get_object_from_value (super_class);

    /* 6.g.ii */
    ecma_value_t proto_parent = ecma_op_object_get_by_magic_id (parent_p, LIT_MAGIC_STRING_PROTOTYPE);

    /* 6.g.iii */
    if (ECMA_IS_VALUE_ERROR (proto_parent))
    {
      return proto_parent;
    }

    /* 6.g.iv */
    if (ecma_is_value_object (proto_parent))
    {
      proto_parent_p = ecma_get_object_from_value (proto_parent);
      free_proto_parent = true;
    }
    else if (ecma_is_value_null (proto_parent))
    {
      proto_parent_p = NULL;
    }
    else
    {
      ecma_free_value (proto_parent);
      return ecma_raise_type_error (ECMA_ERR_PROPERTY_PROTOTYPE_IS_NOT_AN_OBJECT);
    }

    /* 6.g.v */
    ctor_parent_p = parent_p;
  }

  /* 7. */
  ecma_object_t *proto_p = ecma_create_object (proto_parent_p, 0, ECMA_OBJECT_TYPE_GENERAL);
  ecma_value_t proto = ecma_make_object_value (proto_p);

  ECMA_SET_POINTER (ctor_p->u2.prototype_cp, ctor_parent_p);

  if (free_proto_parent)
  {
    ecma_deref_object (proto_parent_p);
  }
  ecma_free_value (super_class);

  /* 16. */
  ecma_property_value_t *property_value_p;
  property_value_p = ecma_create_named_data_property (ctor_p,
                                                      ecma_get_magic_string (LIT_MAGIC_STRING_PROTOTYPE),
                                                      ECMA_PROPERTY_FIXED,
                                                      NULL);
  property_value_p->value = proto;

  /* 18. */
  property_value_p = ecma_create_named_data_property (proto_p,
                                                      ecma_get_magic_string (LIT_MAGIC_STRING_CONSTRUCTOR),
                                                      ECMA_PROPERTY_CONFIGURABLE_WRITABLE,
                                                      NULL);
  property_value_p->value = ecma_make_object_value (ctor_p);

  if (ecma_get_object_type (ctor_p) == ECMA_OBJECT_TYPE_FUNCTION)
  {
    opfunc_bind_class_environment (frame_ctx_p->lex_env_p, proto_p, ctor_p, ctor_p);

    /* 15. set F’s [[ConstructorKind]] internal slot to "derived". */
    if (heritage_present)
    {
      ECMA_SET_THIRD_BIT_TO_POINTER_TAG (((ecma_extended_object_t *) ctor_p)->u.function.scope_cp);
    }
  }

  stack_top_p[-2] = stack_top_p[-1];
  stack_top_p[-1] = proto;

  return ECMA_VALUE_EMPTY;
} /* opfunc_init_class */

/**
 * Creates a new class lexical environment and binds the bound object and the class's object
 *
 * @return newly created class lexical environment - if func_obj_p is not present
 *         NULL - otherwise, also the environment is set as the func_obj_p's scope
 */
ecma_object_t *
opfunc_bind_class_environment (ecma_object_t *lex_env_p, /**< lexical environment */
                               ecma_object_t *home_object_p, /**< bound object */
                               ecma_object_t *ctor_p, /**< constructor object */
                               ecma_object_t *func_obj_p) /**< function object */
{
  ecma_object_t *proto_env_p = ecma_create_lex_env_class (lex_env_p, sizeof (ecma_lexical_environment_class_t));
  ECMA_SET_NON_NULL_POINTER (proto_env_p->u1.bound_object_cp, home_object_p);
  ((ecma_lexical_environment_class_t *) proto_env_p)->object_p = ctor_p;
  ((ecma_lexical_environment_class_t *) proto_env_p)->type = ECMA_LEX_ENV_CLASS_TYPE_CLASS_ENV;

  if (func_obj_p)
  {
    JERRY_ASSERT (ecma_get_object_type (func_obj_p) == ECMA_OBJECT_TYPE_FUNCTION);
    ECMA_SET_NON_NULL_POINTER_TAG (((ecma_extended_object_t *) func_obj_p)->u.function.scope_cp, proto_env_p, 0);
    ecma_deref_object (proto_env_p);
    return NULL;
  }

  return proto_env_p;
} /* opfunc_bind_class_environment */

/**
 * Set [[Enumerable]] and [[HomeObject]] attributes for all class method
 */
static void
opfunc_set_class_attributes (ecma_object_t *obj_p, /**< object */
                             ecma_object_t *parent_env_p) /**< parent environment */
{
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

  while (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);
    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

    ecma_property_pair_t *property_pair_p = (ecma_property_pair_t *) prop_iter_p;

    for (uint32_t index = 0; index < ECMA_PROPERTY_PAIR_ITEM_COUNT; index++)
    {
      uint8_t property = property_pair_p->header.types[index];

      if (!ECMA_PROPERTY_IS_RAW (property))
      {
        JERRY_ASSERT (property == ECMA_PROPERTY_TYPE_DELETED
                      || (ECMA_PROPERTY_IS_INTERNAL (property)
                          && LIT_INTERNAL_MAGIC_STRING_IGNORED (property_pair_p->names_cp[index])));
        continue;
      }

      if (property & ECMA_PROPERTY_FLAG_DATA)
      {
        if (ecma_is_value_object (property_pair_p->values[index].value) && ecma_is_property_enumerable (property))
        {
          property_pair_p->header.types[index] = (uint8_t) (property & ~ECMA_PROPERTY_FLAG_ENUMERABLE);
          opfunc_set_home_object (ecma_get_object_from_value (property_pair_p->values[index].value), parent_env_p);
        }
        continue;
      }

      property_pair_p->header.types[index] = (uint8_t) (property & ~ECMA_PROPERTY_FLAG_ENUMERABLE);
      ecma_property_value_t *accessor_objs_p = property_pair_p->values + index;

      ecma_getter_setter_pointers_t *get_set_pair_p = ecma_get_named_accessor_property (accessor_objs_p);

      if (get_set_pair_p->getter_cp != JMEM_CP_NULL)
      {
        opfunc_set_home_object (ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->getter_cp), parent_env_p);
      }

      if (get_set_pair_p->setter_cp != JMEM_CP_NULL)
      {
        opfunc_set_home_object (ECMA_GET_NON_NULL_POINTER (ecma_object_t, get_set_pair_p->setter_cp), parent_env_p);
      }
    }

    prop_iter_cp = prop_iter_p->next_property_cp;
  }
} /* opfunc_set_class_attributes */

/**
 * Set [[HomeObject]] attributes for all class private elements
 */
static void
opfunc_set_private_instance_method_attributes (ecma_object_t *class_object_p, /**< class constructor  */
                                               ecma_object_t *parent_env_p) /**< parent environment */
{
  ecma_string_t *internal_string_p = ecma_get_internal_string (LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS);
  ecma_property_t *prop_p = ecma_find_named_property (class_object_p, internal_string_p);

  if (prop_p == NULL)
  {
    return;
  }

  ecma_value_t *collection_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_value_t, ECMA_PROPERTY_VALUE_PTR (prop_p)->value);
  ecma_value_t *current_p = collection_p + 1;
  ecma_value_t *end_p = ecma_compact_collection_end (collection_p);

  while (current_p < end_p)
  {
    current_p += 2; /* skip kind, name */
    ecma_value_t value = *current_p++;

    if (!ecma_is_value_undefined (value))
    {
      opfunc_set_home_object (ecma_get_object_from_value (value), parent_env_p);
    }
  }
} /* opfunc_set_private_instance_method_attributes */

/**
 * Pop the current lexical environment referenced by the frame context
 */
void
opfunc_pop_lexical_environment (vm_frame_ctx_t *frame_ctx_p) /**< frame context */
{
  ecma_object_t *outer_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, frame_ctx_p->lex_env_p->u2.outer_reference_cp);
  ecma_deref_object (frame_ctx_p->lex_env_p);
  frame_ctx_p->lex_env_p = outer_env_p;
} /* opfunc_pop_lexical_environment */

/**
 * ClassDefinitionEvaluation finalization part
 *
 * See also: ECMAScript v6, 14.5.14
 */
void
opfunc_finalize_class (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                       ecma_value_t **vm_stack_top_p, /**< current vm stack top */
                       ecma_value_t class_name) /**< class name */
{
  JERRY_ASSERT (ecma_is_value_undefined (class_name) || ecma_is_value_string (class_name));
  ecma_value_t *stack_top_p = *vm_stack_top_p;

  ecma_object_t *ctor_p = ecma_get_object_from_value (stack_top_p[-2]);
  ecma_object_t *proto_p = ecma_get_object_from_value (stack_top_p[-1]);

  ecma_object_t *class_env_p = frame_ctx_p->lex_env_p;

  /* 23.a */
  if (!ecma_is_value_undefined (class_name))
  {
    ecma_op_initialize_binding (class_env_p, ecma_get_string_from_value (class_name), stack_top_p[-2]);
  }

  ecma_object_t *ctor_env_p = opfunc_bind_class_environment (class_env_p, ctor_p, ctor_p, NULL);
  ecma_object_t *proto_env_p = opfunc_bind_class_environment (class_env_p, proto_p, ctor_p, NULL);

  opfunc_set_class_attributes (ctor_p, ctor_env_p);
  opfunc_set_class_attributes (proto_p, proto_env_p);
  opfunc_set_private_instance_method_attributes (ctor_p, proto_env_p);

  ecma_deref_object (proto_env_p);
  ecma_deref_object (ctor_env_p);
  ecma_deref_object (proto_p);

  JERRY_ASSERT ((ecma_is_value_undefined (class_name) ? stack_top_p[-3] == ECMA_VALUE_UNDEFINED
                                                      : stack_top_p[-3] == ECMA_VALUE_RELEASE_LEX_ENV));

  /* only the current class remains on the stack */
  if (stack_top_p[-3] == ECMA_VALUE_RELEASE_LEX_ENV)
  {
    opfunc_pop_lexical_environment (frame_ctx_p);
  }

  opfunc_private_method_or_accessor_add (ctor_p, ctor_p, ECMA_PRIVATE_PROPERTY_STATIC_FLAG);

  stack_top_p[-3] = stack_top_p[-2];
  *vm_stack_top_p -= 2;
} /* opfunc_finalize_class */

/**
 * MakeSuperPropertyReference operation
 *
 * See also: ECMAScript v6, 12.3.5.3
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
opfunc_form_super_reference (ecma_value_t **vm_stack_top_p, /**< current vm stack top */
                             vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                             ecma_value_t prop_name, /**< property name to resolve */
                             uint8_t opcode) /**< current cbc opcode */
{
  ecma_environment_record_t *environment_record_p = ecma_op_get_environment_record (frame_ctx_p->lex_env_p);

  if (environment_record_p && !ecma_op_this_binding_is_initialized (environment_record_p))
  {
    return ecma_raise_reference_error (ECMA_ERR_CALL_SUPER_CONSTRUCTOR_DERIVED_CLASS_BEFORE_THIS);
  }

  ecma_value_t parent = ecma_op_resolve_super_base (frame_ctx_p->lex_env_p);

  if (ECMA_IS_VALUE_ERROR (parent))
  {
    return ecma_raise_type_error (ECMA_ERR_INVOKE_NULLABLE_SUPER_METHOD);
  }

  if (!ecma_op_require_object_coercible (parent))
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t *stack_top_p = *vm_stack_top_p;

  if (opcode >= CBC_EXT_SUPER_PROP_ASSIGNMENT_REFERENCE)
  {
    JERRY_ASSERT (opcode == CBC_EXT_SUPER_PROP_ASSIGNMENT_REFERENCE
                  || opcode == CBC_EXT_SUPER_PROP_LITERAL_ASSIGNMENT_REFERENCE);
    *stack_top_p++ = parent;
    *stack_top_p++ = ecma_copy_value (prop_name);
    *vm_stack_top_p = stack_top_p;

    return ECMA_VALUE_EMPTY;
  }

  ecma_object_t *parent_p = ecma_get_object_from_value (parent);
  ecma_string_t *prop_name_p = ecma_op_to_property_key (prop_name);

  if (prop_name_p == NULL)
  {
    ecma_deref_object (parent_p);
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t result = ecma_op_object_get_with_receiver (parent_p, prop_name_p, frame_ctx_p->this_binding);
  ecma_deref_ecma_string (prop_name_p);
  ecma_deref_object (parent_p);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  if (opcode == CBC_EXT_SUPER_PROP_LITERAL_REFERENCE || opcode == CBC_EXT_SUPER_PROP_REFERENCE)
  {
    *stack_top_p++ = ecma_copy_value (frame_ctx_p->this_binding);
    *stack_top_p++ = ECMA_VALUE_UNDEFINED;
  }

  *stack_top_p++ = result;
  *vm_stack_top_p = stack_top_p;

  return ECMA_VALUE_EMPTY;
} /* opfunc_form_super_reference */

/**
 * Assignment operation for SuperRefence base
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
opfunc_assign_super_reference (ecma_value_t **vm_stack_top_p, /**< vm stack top */
                               vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                               uint32_t opcode_data) /**< opcode data to store the result */
{
  ecma_value_t *stack_top_p = *vm_stack_top_p;

  ecma_value_t base_obj = ecma_op_to_object (stack_top_p[-3]);

  if (ECMA_IS_VALUE_ERROR (base_obj))
  {
    return base_obj;
  }

  ecma_object_t *base_obj_p = ecma_get_object_from_value (base_obj);
  ecma_string_t *prop_name_p = ecma_op_to_property_key (stack_top_p[-2]);

  if (prop_name_p == NULL)
  {
    ecma_deref_object (base_obj_p);
    return ECMA_VALUE_ERROR;
  }

  bool is_strict = (frame_ctx_p->status_flags & VM_FRAME_CTX_IS_STRICT) != 0;

  ecma_value_t result =
    ecma_op_object_put_with_receiver (base_obj_p, prop_name_p, stack_top_p[-1], frame_ctx_p->this_binding, is_strict);

  ecma_deref_ecma_string (prop_name_p);
  ecma_deref_object (base_obj_p);

  if (ECMA_IS_VALUE_ERROR (result))
  {
    return result;
  }

  for (int32_t i = 1; i <= 3; i++)
  {
    ecma_free_value (stack_top_p[-i]);
  }

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

  *vm_stack_top_p = stack_top_p;

  return result;
} /* opfunc_assign_super_reference */

/**
 * Copy data properties of an object
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
opfunc_copy_data_properties (ecma_value_t target_object, /**< target object */
                             ecma_value_t source_object, /**< source object */
                             ecma_value_t filter_array) /**< filter array */
{
  bool source_to_object = false;

  if (!ecma_is_value_object (source_object))
  {
    source_object = ecma_op_to_object (source_object);

    if (ECMA_IS_VALUE_ERROR (source_object))
    {
      return source_object;
    }

    source_to_object = true;
  }

  ecma_object_t *source_object_p = ecma_get_object_from_value (source_object);
  ecma_collection_t *names_p = ecma_op_object_own_property_keys (source_object_p, JERRY_PROPERTY_FILTER_ALL);

#if JERRY_BUILTIN_PROXY
  if (names_p == NULL)
  {
    JERRY_ASSERT (!source_to_object);
    return ECMA_VALUE_ERROR;
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_object_t *target_object_p = ecma_get_object_from_value (target_object);
  ecma_value_t *buffer_p = names_p->buffer_p;
  ecma_value_t *buffer_end_p = buffer_p + names_p->item_count;
  ecma_value_t *filter_start_p = NULL;
  ecma_value_t *filter_end_p = NULL;
  ecma_value_t result = ECMA_VALUE_EMPTY;

  if (filter_array != ECMA_VALUE_UNDEFINED)
  {
    ecma_object_t *filter_array_p = ecma_get_object_from_value (filter_array);

    JERRY_ASSERT (ecma_get_object_type (filter_array_p) == ECMA_OBJECT_TYPE_ARRAY);
    JERRY_ASSERT (ecma_op_object_is_fast_array (filter_array_p));

    if (filter_array_p->u1.property_list_cp != JMEM_CP_NULL)
    {
      filter_start_p = ECMA_GET_NON_NULL_POINTER (ecma_value_t, filter_array_p->u1.property_list_cp);
      filter_end_p = filter_start_p + ((ecma_extended_object_t *) filter_array_p)->u.array.length;
    }
  }

  while (buffer_p < buffer_end_p)
  {
    ecma_string_t *property_name_p = ecma_get_prop_name_from_value (*buffer_p++);

    if (filter_start_p != NULL)
    {
      ecma_value_t *filter_p = filter_start_p;

      do
      {
        if (ecma_compare_ecma_strings (property_name_p, ecma_get_prop_name_from_value (*filter_p)))
        {
          break;
        }
      } while (++filter_p < filter_end_p);

      if (filter_p != filter_end_p)
      {
        continue;
      }
    }

    ecma_property_descriptor_t descriptor;
    result = ecma_op_object_get_own_property_descriptor (source_object_p, property_name_p, &descriptor);

    if (ECMA_IS_VALUE_ERROR (result))
    {
      break;
    }

    if (result == ECMA_VALUE_FALSE)
    {
      continue;
    }

    if (!(descriptor.flags & JERRY_PROP_IS_ENUMERABLE))
    {
      ecma_free_property_descriptor (&descriptor);
      continue;
    }

    if ((descriptor.flags & JERRY_PROP_IS_VALUE_DEFINED) && !ECMA_OBJECT_IS_PROXY (source_object_p))
    {
      result = descriptor.value;
    }
    else
    {
      ecma_free_property_descriptor (&descriptor);

      result = ecma_op_object_get (source_object_p, property_name_p);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        break;
      }
    }

    opfunc_set_data_property (target_object_p, property_name_p, result);
    ecma_free_value (result);

    result = ECMA_VALUE_EMPTY;
  }

  if (JERRY_UNLIKELY (source_to_object))
  {
    ecma_deref_object (source_object_p);
  }

  ecma_collection_free (names_p);
  return result;
} /* opfunc_copy_data_properties */

/**
 * Check whether the current lexical scope has restricted binding declaration with the given name
 *
 * Steps are include ES11: 8.1.1.4.14 HasRestrictedGlobalProperty abstract operation
 *
 * @return ECMA_VALUE_ERROR - if the operation fails
 *         ECMA_VALUE_TRUE - if it has restricted property binding
 *         ECMA_VALUE_FALSE - otherwise
 */
ecma_value_t
opfunc_lexical_scope_has_restricted_binding (vm_frame_ctx_t *frame_ctx_p, /**< frame context */
                                             ecma_string_t *name_p) /**< binding name */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (ecma_get_lex_env_type (frame_ctx_p->lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);

  ecma_object_t *lex_env_p = frame_ctx_p->lex_env_p;
  ecma_property_t *binding_p = ecma_find_named_property (lex_env_p, name_p);

  if (binding_p != NULL)
  {
    return ECMA_VALUE_TRUE;
  }

  ecma_object_t *global_obj_p = ecma_get_object_from_value (frame_ctx_p->this_binding);

#if JERRY_BUILTIN_REALMS
  ecma_object_t *const global_scope_p = ecma_get_global_scope ((ecma_object_t *) JERRY_CONTEXT (global_object_p));
#else /* !JERRY_BUILTIN_REALMS */
  ecma_object_t *const global_scope_p = ecma_get_global_scope (global_obj_p);
#endif /* JERRY_BUILTIN_REALMS */

  if (global_scope_p != lex_env_p)
  {
    return ECMA_VALUE_FALSE;
  }

#if JERRY_BUILTIN_PROXY
  if (ECMA_OBJECT_IS_PROXY (global_obj_p))
  {
    ecma_property_descriptor_t prop_desc;
    ecma_value_t status = ecma_proxy_object_get_own_property_descriptor (global_obj_p, name_p, &prop_desc);

    if (ecma_is_value_true (status))
    {
      status = ecma_make_boolean_value ((prop_desc.flags & JERRY_PROP_IS_CONFIGURABLE) == 0);
      ecma_free_property_descriptor (&prop_desc);
    }

    return status;
  }
#endif /* JERRY_BUILTIN_PROXY */

  ecma_property_t property = ecma_op_object_get_own_property (global_obj_p, name_p, NULL, ECMA_PROPERTY_GET_NO_OPTIONS);

  JERRY_ASSERT (property == ECMA_PROPERTY_TYPE_NOT_FOUND || ECMA_PROPERTY_IS_FOUND (property));

  return ecma_make_boolean_value (property != ECMA_PROPERTY_TYPE_NOT_FOUND
                                  && !ecma_is_property_configurable (property));
} /* opfunc_lexical_scope_has_restricted_binding */

/**
 * @}
 * @}
 */
