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

#ifndef OPCODES_H
#define OPCODES_H

#include "ecma-globals.h"

#include "vm-defines.h"

/** \addtogroup vm Virtual machine
 * @{
 *
 * \addtogroup vm_opcodes Opcodes
 * @{
 */

/**
 * Number arithmetic operations.
 */
typedef enum
{
  NUMBER_ARITHMETIC_SUBTRACTION, /**< subtraction */
  NUMBER_ARITHMETIC_MULTIPLICATION, /**< multiplication */
  NUMBER_ARITHMETIC_DIVISION, /**< division */
  NUMBER_ARITHMETIC_REMAINDER, /**< remainder calculation */
  NUMBER_ARITHMETIC_EXPONENTIATION, /**< exponentiation */
} number_arithmetic_op;

/**
 * Number bitwise logic operations.
 */
typedef enum
{
  NUMBER_BITWISE_LOGIC_AND, /**< bitwise AND calculation */
  NUMBER_BITWISE_LOGIC_OR, /**< bitwise OR calculation */
  NUMBER_BITWISE_LOGIC_XOR, /**< bitwise XOR calculation */
  NUMBER_BITWISE_SHIFT_LEFT, /**< bitwise LEFT SHIFT calculation */
  NUMBER_BITWISE_SHIFT_RIGHT, /**< bitwise RIGHT_SHIFT calculation */
  NUMBER_BITWISE_SHIFT_URIGHT, /**< bitwise UNSIGNED RIGHT SHIFT calculation */
} number_bitwise_logic_op;

/**
 * Types for opfunc_create_executable_object.
 */
typedef enum
{
  VM_CREATE_EXECUTABLE_OBJECT_GENERATOR, /**< create a generator function */
  VM_CREATE_EXECUTABLE_OBJECT_ASYNC, /**< create an async function */
} vm_create_executable_object_type_t;

/**
 * The stack contains spread object during the upcoming APPEND_ARRAY operation
 */
#define OPFUNC_HAS_SPREAD_ELEMENT (1 << 8)

ecma_value_t opfunc_equality (ecma_value_t left_value, ecma_value_t right_value);

ecma_value_t do_number_arithmetic (number_arithmetic_op op, ecma_value_t left_value, ecma_value_t right_value);

ecma_value_t opfunc_unary_operation (ecma_value_t left_value, bool is_plus);

ecma_value_t do_number_bitwise_logic (number_bitwise_logic_op op, ecma_value_t left_value, ecma_value_t right_value);

ecma_value_t do_number_bitwise_not (ecma_value_t value);

ecma_value_t opfunc_addition (ecma_value_t left_value, ecma_value_t right_value);

ecma_value_t opfunc_relation (ecma_value_t left_value, ecma_value_t right_value, bool left_first, bool is_invert);

ecma_value_t opfunc_in (ecma_value_t left_value, ecma_value_t right_value);

ecma_value_t opfunc_instanceof (ecma_value_t left_value, ecma_value_t right_value);

ecma_value_t opfunc_typeof (ecma_value_t left_value);

void opfunc_set_data_property (ecma_object_t *object_p, ecma_string_t *prop_name_p, ecma_value_t value);

void opfunc_set_accessor (bool is_getter, ecma_value_t object, ecma_string_t *accessor_name_p, ecma_value_t accessor);

ecma_value_t vm_op_delete_prop (ecma_value_t object, ecma_value_t property, bool is_strict);

ecma_value_t vm_op_delete_var (ecma_value_t name_literal, ecma_object_t *lex_env_p);

ecma_collection_t *opfunc_for_in (ecma_value_t left_value, ecma_value_t *result_obj_p);

ecma_collection_t *opfunc_spread_arguments (ecma_value_t *stack_top_p, uint8_t argument_list_len);

ecma_value_t opfunc_append_array (ecma_value_t *stack_top_p, uint16_t values_length);

vm_executable_object_t *opfunc_create_executable_object (vm_frame_ctx_t *frame_ctx_p,
                                                         vm_create_executable_object_type_t type);

extern const uint8_t opfunc_resume_executable_object_with_throw[];
extern const uint8_t opfunc_resume_executable_object_with_return[];

ecma_value_t opfunc_resume_executable_object (vm_executable_object_t *executable_object_p, ecma_value_t value);

void opfunc_async_generator_yield (ecma_extended_object_t *async_generator_object_p, ecma_value_t value);

ecma_value_t opfunc_async_create_and_await (vm_frame_ctx_t *frame_ctx_p, ecma_value_t value, uint16_t extra_flags);

ecma_value_t opfunc_init_class_fields (ecma_object_t *class_object_p, ecma_value_t this_val);

ecma_value_t opfunc_init_static_class_fields (ecma_value_t function_object, ecma_value_t this_val);

ecma_value_t opfunc_add_computed_field (ecma_value_t class_object, ecma_value_t name);

ecma_value_t opfunc_create_implicit_class_constructor (uint8_t opcode, const ecma_compiled_code_t *bytecode_p);

void opfunc_set_home_object (ecma_object_t *func_p, ecma_object_t *parent_env_p);

ecma_value_t opfunc_define_field (ecma_value_t base, ecma_value_t property, ecma_value_t value);

ecma_string_t *opfunc_make_private_key (ecma_value_t descriptor);

ecma_value_t opfunc_private_in (ecma_value_t base, ecma_value_t property);

ecma_value_t opfunc_private_field_add (ecma_value_t base, ecma_value_t property, ecma_value_t value);

ecma_value_t opfunc_private_set (ecma_value_t base, ecma_value_t property, ecma_value_t value);

ecma_value_t opfunc_private_get (ecma_value_t base, ecma_value_t property);

void opfunc_collect_private_properties (ecma_value_t constructor,
                                        ecma_value_t prop_name,
                                        ecma_value_t method,
                                        uint8_t opcode);

void opfunc_push_class_environment (vm_frame_ctx_t *frame_ctx_p, ecma_value_t **vm_stack_top, ecma_value_t class_name);

ecma_value_t opfunc_init_class (vm_frame_ctx_t *frame_context_p, ecma_value_t *stack_top_p);

ecma_object_t *opfunc_bind_class_environment (ecma_object_t *lex_env_p,
                                              ecma_object_t *home_object_p,
                                              ecma_object_t *ctor_p,
                                              ecma_object_t *func_obj_p);

void opfunc_pop_lexical_environment (vm_frame_ctx_t *frame_ctx_p);

void opfunc_finalize_class (vm_frame_ctx_t *frame_ctx_p, ecma_value_t **vm_stack_top_p, ecma_value_t class_name);

ecma_value_t opfunc_form_super_reference (ecma_value_t **vm_stack_top_p,
                                          vm_frame_ctx_t *frame_ctx_p,
                                          ecma_value_t prop_name,
                                          uint8_t opcode);

ecma_value_t
opfunc_assign_super_reference (ecma_value_t **vm_stack_top_p, vm_frame_ctx_t *frame_ctx_p, uint32_t opcode_data);

ecma_value_t
opfunc_copy_data_properties (ecma_value_t target_object, ecma_value_t source_object, ecma_value_t filter_array);

ecma_value_t opfunc_lexical_scope_has_restricted_binding (vm_frame_ctx_t *vm_frame_ctx_p, ecma_string_t *name_p);

/**
 * @}
 * @}
 */

#endif /* !OPCODES_H */
