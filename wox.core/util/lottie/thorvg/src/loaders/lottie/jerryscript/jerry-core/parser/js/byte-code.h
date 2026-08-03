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

#ifndef BYTE_CODE_H
#define BYTE_CODE_H

#include "ecma-globals.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_bytecode Bytecode
 * @{
 */

/**
 * Compact byte code (CBC) is a byte code representation
 * of EcmaScript which is designed for low memory
 * environments. Most opcodes are only one or sometimes
 * two byte long so the CBC provides a small binary size.
 *
 * The execution engine of CBC is a stack machine, where
 * the maximum stack size is known in advance for each
 * function.
 */

/**
 * Byte code flags. Only the lower 5 bit can be used
 * since the stack change is encoded in the upper
 * three bits for each instruction between -4 and 3
 * (except for call / construct opcodes).
 */
#define CBC_STACK_ADJUST_BASE         4
#define CBC_STACK_ADJUST_SHIFT        5
#define CBC_STACK_ADJUST_VALUE(value) (((value) >> CBC_STACK_ADJUST_SHIFT) - CBC_STACK_ADJUST_BASE)

#define CBC_NO_FLAG          0x00u
#define CBC_HAS_LITERAL_ARG  0x01u
#define CBC_HAS_LITERAL_ARG2 0x02u
#define CBC_HAS_BYTE_ARG     0x04u
#define CBC_HAS_BRANCH_ARG   0x08u

/* These flags are shared */
#define CBC_FORWARD_BRANCH_ARG 0x10u
#define CBC_POP_STACK_BYTE_ARG 0x10u

#define CBC_ARG_TYPES (CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2 | CBC_HAS_BYTE_ARG | CBC_HAS_BRANCH_ARG)

#define CBC_HAS_POP_STACK_BYTE_ARG (CBC_HAS_BYTE_ARG | CBC_POP_STACK_BYTE_ARG)

/**
 * CBC_NO_RESULT_OPERATION for ext opcodes
 */
#define CBC_EXT_NO_RESULT_OPERATION(opcode)                \
  ((opcode) >= PARSER_TO_EXT_OPCODE (CBC_EXT_ASSIGN_SUPER) \
   && (opcode) <= PARSER_TO_EXT_OPCODE (CBC_EXT_SPREAD_CALL_PROP_BLOCK))

/* Debug macro. */
#define CBC_ARGS_EQ(op, types) ((cbc_flags[op] & CBC_ARG_TYPES) == (types))

/* Debug macro. */
#define CBC_SAME_ARGS(op1, op2)                                                                          \
  (CBC_EXT_NO_RESULT_OPERATION (op1) ? ((cbc_ext_flags[PARSER_GET_EXT_OPCODE (op1)] & CBC_ARG_TYPES)     \
                                        == (cbc_ext_flags[PARSER_GET_EXT_OPCODE (op2)] & CBC_ARG_TYPES)) \
                                     : ((cbc_flags[op1] & CBC_ARG_TYPES) == (cbc_flags[op2] & CBC_ARG_TYPES)))

#define CBC_UNARY_OPERATION(name, group)                                                 \
  CBC_OPCODE (name, CBC_NO_FLAG, 0, (VM_OC_##group) | VM_OC_GET_STACK | VM_OC_PUT_STACK) \
  CBC_OPCODE (name##_LITERAL, CBC_HAS_LITERAL_ARG, 1, (VM_OC_##group) | VM_OC_GET_LITERAL | VM_OC_PUT_STACK)

#define CBC_BINARY_OPERATION(name, group)                                                       \
  CBC_OPCODE (name, CBC_NO_FLAG, -1, (VM_OC_##group) | VM_OC_GET_STACK_STACK | VM_OC_PUT_STACK) \
  CBC_OPCODE (name##_RIGHT_LITERAL,                                                             \
              CBC_HAS_LITERAL_ARG,                                                              \
              0,                                                                                \
              (VM_OC_##group) | VM_OC_GET_STACK_LITERAL | VM_OC_PUT_STACK)                      \
  CBC_OPCODE (name##_TWO_LITERALS,                                                              \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                       \
              1,                                                                                \
              (VM_OC_##group) | VM_OC_GET_LITERAL_LITERAL | VM_OC_PUT_STACK)

#define CBC_UNARY_LVALUE_OPERATION(name, group)                                                            \
  CBC_OPCODE (name, CBC_NO_FLAG, -2, (VM_OC_PROP_##group) | VM_OC_GET_STACK_STACK | VM_OC_PUT_REFERENCE)   \
  CBC_OPCODE (name##_PUSH_RESULT,                                                                          \
              CBC_NO_FLAG,                                                                                 \
              -1,                                                                                          \
              (VM_OC_PROP_##group) | VM_OC_GET_STACK_STACK | VM_OC_PUT_REFERENCE | VM_OC_PUT_STACK)        \
  CBC_OPCODE (name##_BLOCK,                                                                                \
              CBC_NO_FLAG,                                                                                 \
              -2,                                                                                          \
              (VM_OC_PROP_##group) | VM_OC_GET_STACK_STACK | VM_OC_PUT_REFERENCE | VM_OC_PUT_BLOCK)        \
  CBC_OPCODE (name##_IDENT, CBC_HAS_LITERAL_ARG, 0, (VM_OC_##group) | VM_OC_GET_LITERAL | VM_OC_PUT_IDENT) \
  CBC_OPCODE (name##_IDENT_PUSH_RESULT,                                                                    \
              CBC_HAS_LITERAL_ARG,                                                                         \
              1,                                                                                           \
              (VM_OC_##group) | VM_OC_GET_LITERAL | VM_OC_PUT_IDENT | VM_OC_PUT_STACK)                     \
  CBC_OPCODE (name##_IDENT_BLOCK,                                                                          \
              CBC_HAS_LITERAL_ARG,                                                                         \
              0,                                                                                           \
              (VM_OC_##group) | VM_OC_GET_LITERAL | VM_OC_PUT_IDENT | VM_OC_PUT_BLOCK)

#define CBC_UNARY_LVALUE_WITH_IDENT 3

#define CBC_BINARY_WITH_LITERAL      1
#define CBC_BINARY_WITH_TWO_LITERALS 2

/**
 * Several opcodes (mostly call and assignment opcodes) have
 * two forms: one which does not push a return value onto
 * the stack, and another which does. The reason is that
 * the return value of these opcodes are often not used
 * and the first form provides smaller byte code.
 *
 * The following rules must be kept by the code generator:
 *  - only the opcode without return value can be emitted
 *    by the code generator
 *  - the first form can be converted to the second form
 *    by adding 1 to the opcode
 *  - after the conversion the opcode must be immediately
 *    flushed, so no further changes are possible
 *
 * Hence CBC_NO_RESULT_OPERATION (context_p->last_cbc_opcode)
 * cannot be true for an opcode which has a result
 */
#define CBC_NO_RESULT_OPERATION(opcode) \
  (((opcode) >= CBC_PRE_INCR && (opcode) < CBC_END) || CBC_EXT_NO_RESULT_OPERATION ((opcode)))

/**
 * Branch instructions are organized in group of 8 opcodes.
 *  - 1st opcode: unused, can be used for other purpose
 *  - 2nd opcode: forward branch with 1 byte offset
 *  - 3rd opcode: forward branch with 2 byte offset
 *  - 4th opcode: forward branch with 3 byte offset
 *  - 5th opcode: unused, can be used for other purpose
 *  - 6th opcode: backward branch with 1 byte offset
 *  - 7th opcode: backward branch with 2 byte offset
 *  - 8th opcode: backward branch with 3 byte offset
 *
 * Reasons:
 *  The branch_opcode & 0x3 tells the length in bytes of the offset
 *  If branch offset & 0x4 == 0, it is a forward branch. Otherwise
 *  it is backward.
 *
 * The offset bytes are encoded in higher to lower order.
 */

#define CBC_FORWARD_BRANCH(name, stack, vm_oc)                                                          \
  CBC_OPCODE (name, CBC_HAS_BRANCH_ARG | CBC_FORWARD_BRANCH_ARG, stack, (vm_oc) | VM_OC_GET_BRANCH)     \
  CBC_OPCODE (name##_2, CBC_HAS_BRANCH_ARG | CBC_FORWARD_BRANCH_ARG, stack, (vm_oc) | VM_OC_GET_BRANCH) \
  CBC_OPCODE (name##_3, CBC_HAS_BRANCH_ARG | CBC_FORWARD_BRANCH_ARG, stack, (vm_oc) | VM_OC_GET_BRANCH)

#define CBC_BACKWARD_BRANCH(name, stack, vm_oc)                                                        \
  CBC_OPCODE (name, CBC_HAS_BRANCH_ARG, stack, (vm_oc) | VM_OC_GET_BRANCH | VM_OC_BACKWARD_BRANCH)     \
  CBC_OPCODE (name##_2, CBC_HAS_BRANCH_ARG, stack, (vm_oc) | VM_OC_GET_BRANCH | VM_OC_BACKWARD_BRANCH) \
  CBC_OPCODE (name##_3, CBC_HAS_BRANCH_ARG, stack, (vm_oc) | VM_OC_GET_BRANCH | VM_OC_BACKWARD_BRANCH)

#define CBC_BRANCH_OFFSET_LENGTH(opcode) ((opcode) &0x3)

#define CBC_BRANCH_IS_BACKWARD(flags) (!((flags) &CBC_FORWARD_BRANCH_ARG))

#define CBC_BRANCH_IS_FORWARD(flags) ((flags) &CBC_FORWARD_BRANCH_ARG)

/* Stack consumption of opcodes with context. */

/* PARSER_TRY_CONTEXT_STACK_ALLOCATION must be <= 3 */
#define PARSER_TRY_CONTEXT_STACK_ALLOCATION 1
/* PARSER_FINALLY_CONTEXT_STACK_ALLOCATION must be <= 3 */
#define PARSER_FINALLY_CONTEXT_STACK_ALLOCATION 2
/* PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION must be <= 4 */
#define PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION 4
/* PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION must be <= 4 */
#define PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION 4
/* PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION must be <= 4 */
#define PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION 4
/* PARSER_WITH_CONTEXT_STACK_ALLOCATION must be <= 4 */
#define PARSER_WITH_CONTEXT_STACK_ALLOCATION 1
/* PARSER_BLOCK_CONTEXT_STACK_ALLOCATION must be <= 3 */
#define PARSER_BLOCK_CONTEXT_STACK_ALLOCATION 1
/* PARSER_ITERATOR_CONTEXT_STACK_ALLOCATION must be <= 3 */
#define PARSER_ITERATOR_CONTEXT_STACK_ALLOCATION 3
/* PARSER_OBJECT_INITIALIZER_CONTEXT_STACK_ALLOCATION must be <= 2 */
#define PARSER_OBJ_INIT_CONTEXT_STACK_ALLOCATION 1
/* PARSER_OBJECT_INITIALIZER_CONTEXT_STACK_ALLOCATION must be <= 2 */
#define PARSER_OBJ_INIT_REST_CONTEXT_STACK_ALLOCATION 2

/**
 * Extra stack consumption for finally context.
 */
#define PARSER_FINALLY_CONTEXT_EXTRA_STACK_ALLOCATION \
  (PARSER_FINALLY_CONTEXT_STACK_ALLOCATION - PARSER_TRY_CONTEXT_STACK_ALLOCATION)

#define PARSER_STATIC_PRIVATE_TO_PRIVATE_OFFSET (CBC_EXT_COLLECT_PRIVATE_STATIC_FIELD - CBC_EXT_COLLECT_PRIVATE_FIELD)

/**
 * Opcode definitions.
 */
#define CBC_OPCODE_LIST                                                                                             \
  /* Branch opcodes first. Some other opcodes are mixed. */                                                         \
  CBC_OPCODE (CBC_EXT_OPCODE, CBC_NO_FLAG, 0, VM_OC_NONE)                                                           \
  CBC_FORWARD_BRANCH (CBC_JUMP_FORWARD, 0, VM_OC_JUMP)                                                              \
  CBC_OPCODE (CBC_POP, CBC_NO_FLAG, -1, VM_OC_POP)                                                                  \
  CBC_BACKWARD_BRANCH (CBC_JUMP_BACKWARD, 0, VM_OC_JUMP)                                                            \
  CBC_OPCODE (CBC_POP_BLOCK, CBC_NO_FLAG, -1, VM_OC_POP_BLOCK | VM_OC_PUT_BLOCK)                                    \
  CBC_FORWARD_BRANCH (CBC_BRANCH_IF_TRUE_FORWARD, -1, VM_OC_BRANCH_IF_TRUE)                                         \
  CBC_OPCODE (CBC_THROW, CBC_NO_FLAG, -1, VM_OC_THROW | VM_OC_GET_STACK)                                            \
  CBC_BACKWARD_BRANCH (CBC_BRANCH_IF_TRUE_BACKWARD, -1, VM_OC_BRANCH_IF_TRUE)                                       \
  CBC_OPCODE (CBC_CONTEXT_END, CBC_NO_FLAG, 0, VM_OC_CONTEXT_END)                                                   \
  CBC_FORWARD_BRANCH (CBC_BRANCH_IF_FALSE_FORWARD, -1, VM_OC_BRANCH_IF_FALSE)                                       \
  CBC_OPCODE (CBC_CREATE_OBJECT, CBC_NO_FLAG, 1, VM_OC_PUSH_OBJECT | VM_OC_PUT_STACK)                               \
  CBC_BACKWARD_BRANCH (CBC_BRANCH_IF_FALSE_BACKWARD, -1, VM_OC_BRANCH_IF_FALSE)                                     \
  CBC_OPCODE (CBC_SET_PROPERTY,                                                                                     \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              -1,                                                                                                   \
              VM_OC_SET_PROPERTY | VM_OC_NON_STATIC_FLAG | VM_OC_GET_STACK_LITERAL)                                 \
  CBC_FORWARD_BRANCH (CBC_JUMP_FORWARD_EXIT_CONTEXT, 0, VM_OC_JUMP_AND_EXIT_CONTEXT)                                \
  CBC_OPCODE (CBC_CREATE_ARRAY, CBC_NO_FLAG, 1, VM_OC_PUSH_ARRAY | VM_OC_PUT_STACK)                                 \
  CBC_FORWARD_BRANCH (CBC_BRANCH_IF_LOGICAL_TRUE, -1, VM_OC_BRANCH_IF_LOGICAL_TRUE)                                 \
  CBC_OPCODE (CBC_ARRAY_APPEND, CBC_HAS_POP_STACK_BYTE_ARG, 0, VM_OC_APPEND_ARRAY)                                  \
  CBC_FORWARD_BRANCH (CBC_BRANCH_IF_LOGICAL_FALSE, -1, VM_OC_BRANCH_IF_LOGICAL_FALSE)                               \
  CBC_OPCODE (CBC_PUSH_ELISION, CBC_NO_FLAG, 1, VM_OC_PUSH_ELISON | VM_OC_PUT_STACK)                                \
  CBC_FORWARD_BRANCH (CBC_BRANCH_IF_STRICT_EQUAL, -1, VM_OC_BRANCH_IF_STRICT_EQUAL)                                 \
  CBC_OPCODE (CBC_PUSH_NULL, CBC_NO_FLAG, 1, VM_OC_PUSH_NULL | VM_OC_PUT_STACK)                                     \
  CBC_FORWARD_BRANCH (CBC_BLOCK_CREATE_CONTEXT, PARSER_BLOCK_CONTEXT_STACK_ALLOCATION, VM_OC_BLOCK_CREATE_CONTEXT)  \
                                                                                                                    \
  /* Basic opcodes. Note: These 4 opcodes must me in this order */                                                  \
  CBC_OPCODE (CBC_PUSH_LITERAL, CBC_HAS_LITERAL_ARG, 1, VM_OC_PUSH | VM_OC_GET_LITERAL)                             \
  CBC_OPCODE (CBC_PUSH_TWO_LITERALS,                                                                                \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              2,                                                                                                    \
              VM_OC_PUSH_TWO | VM_OC_GET_LITERAL_LITERAL)                                                           \
  CBC_OPCODE (CBC_PUSH_THIS_LITERAL, CBC_HAS_LITERAL_ARG, 2, VM_OC_PUSH_TWO | VM_OC_GET_THIS_LITERAL)               \
  CBC_OPCODE (CBC_PUSH_THREE_LITERALS, CBC_HAS_LITERAL_ARG2, 3, VM_OC_PUSH_THREE | VM_OC_GET_LITERAL_LITERAL)       \
  CBC_OPCODE (CBC_PUSH_UNDEFINED, CBC_NO_FLAG, 1, VM_OC_PUSH_UNDEFINED | VM_OC_PUT_STACK)                           \
  CBC_OPCODE (CBC_PUSH_TRUE, CBC_NO_FLAG, 1, VM_OC_PUSH_TRUE | VM_OC_PUT_STACK)                                     \
  CBC_OPCODE (CBC_PUSH_FALSE, CBC_NO_FLAG, 1, VM_OC_PUSH_FALSE | VM_OC_PUT_STACK)                                   \
  CBC_OPCODE (CBC_PUSH_THIS, CBC_NO_FLAG, 1, VM_OC_PUSH_THIS | VM_OC_PUT_STACK)                                     \
  CBC_OPCODE (CBC_PUSH_NUMBER_0, CBC_NO_FLAG, 1, VM_OC_PUSH_0 | VM_OC_PUT_STACK)                                    \
  CBC_OPCODE (CBC_PUSH_NUMBER_POS_BYTE, CBC_HAS_BYTE_ARG, 1, VM_OC_PUSH_POS_BYTE | VM_OC_PUT_STACK)                 \
  CBC_OPCODE (CBC_PUSH_NUMBER_NEG_BYTE, CBC_HAS_BYTE_ARG, 1, VM_OC_PUSH_NEG_BYTE | VM_OC_PUT_STACK)                 \
  CBC_OPCODE (CBC_PUSH_LITERAL_PUSH_NUMBER_0, CBC_HAS_LITERAL_ARG, 2, VM_OC_PUSH_LIT_0 | VM_OC_GET_LITERAL)         \
  CBC_OPCODE (CBC_PUSH_LITERAL_PUSH_NUMBER_POS_BYTE,                                                                \
              CBC_HAS_LITERAL_ARG | CBC_HAS_BYTE_ARG,                                                               \
              2,                                                                                                    \
              VM_OC_PUSH_LIT_POS_BYTE | VM_OC_GET_LITERAL)                                                          \
  CBC_OPCODE (CBC_PUSH_LITERAL_PUSH_NUMBER_NEG_BYTE,                                                                \
              CBC_HAS_LITERAL_ARG | CBC_HAS_BYTE_ARG,                                                               \
              2,                                                                                                    \
              VM_OC_PUSH_LIT_NEG_BYTE | VM_OC_GET_LITERAL)                                                          \
  /* Note: These 4 opcodes must me in this order */                                                                 \
  CBC_OPCODE (CBC_PUSH_PROP, CBC_NO_FLAG, -1, VM_OC_PROP_GET | VM_OC_GET_STACK_STACK | VM_OC_PUT_STACK)             \
  CBC_OPCODE (CBC_PUSH_PROP_LITERAL,                                                                                \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              0,                                                                                                    \
              VM_OC_PROP_GET | VM_OC_GET_STACK_LITERAL | VM_OC_PUT_STACK)                                           \
  CBC_OPCODE (CBC_PUSH_PROP_LITERAL_LITERAL,                                                                        \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              1,                                                                                                    \
              VM_OC_PROP_GET | VM_OC_GET_LITERAL_LITERAL | VM_OC_PUT_STACK)                                         \
  CBC_OPCODE (CBC_PUSH_PROP_THIS_LITERAL,                                                                           \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              1,                                                                                                    \
              VM_OC_PROP_GET | VM_OC_GET_THIS_LITERAL | VM_OC_PUT_STACK)                                            \
  CBC_OPCODE (CBC_PUSH_IDENT_REFERENCE, CBC_HAS_LITERAL_ARG, 3, VM_OC_IDENT_REFERENCE | VM_OC_PUT_STACK)            \
  /* Note: These 4 opcodes must me in this order */                                                                 \
  CBC_OPCODE (CBC_PUSH_PROP_REFERENCE, CBC_NO_FLAG, 1, VM_OC_PROP_REFERENCE | VM_OC_PUT_STACK)                      \
  CBC_OPCODE (CBC_PUSH_PROP_LITERAL_REFERENCE,                                                                      \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              2,                                                                                                    \
              VM_OC_PROP_REFERENCE | VM_OC_GET_LITERAL | VM_OC_PUT_STACK)                                           \
  CBC_OPCODE (CBC_PUSH_PROP_LITERAL_LITERAL_REFERENCE,                                                              \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              3,                                                                                                    \
              VM_OC_PROP_REFERENCE | VM_OC_GET_LITERAL_LITERAL | VM_OC_PUT_STACK)                                   \
  CBC_OPCODE (CBC_PUSH_PROP_THIS_LITERAL_REFERENCE,                                                                 \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              3,                                                                                                    \
              VM_OC_PROP_REFERENCE | VM_OC_GET_THIS_LITERAL | VM_OC_PUT_STACK)                                      \
  CBC_OPCODE (CBC_NEW, CBC_HAS_POP_STACK_BYTE_ARG, 0, VM_OC_NEW | VM_OC_PUT_STACK)                                  \
  CBC_OPCODE (CBC_NEW0, CBC_NO_FLAG, 0, VM_OC_NEW | VM_OC_PUT_STACK)                                                \
  CBC_OPCODE (CBC_NEW1, CBC_NO_FLAG, -1, VM_OC_NEW | VM_OC_PUT_STACK)                                               \
  CBC_OPCODE (CBC_EVAL, CBC_NO_FLAG, 0, VM_OC_EVAL)                                                                 \
  CBC_OPCODE (CBC_CHECK_VAR, CBC_HAS_LITERAL_ARG, 0, VM_OC_CHECK_VAR)                                               \
  CBC_OPCODE (CBC_CHECK_LET, CBC_HAS_LITERAL_ARG, 0, VM_OC_CHECK_LET)                                               \
  CBC_OPCODE (CBC_CREATE_VAR, CBC_HAS_LITERAL_ARG, 0, VM_OC_CREATE_BINDING)                                         \
  CBC_OPCODE (CBC_CREATE_LET, CBC_HAS_LITERAL_ARG, 0, VM_OC_CREATE_BINDING)                                         \
  CBC_OPCODE (CBC_CREATE_CONST, CBC_HAS_LITERAL_ARG, 0, VM_OC_CREATE_BINDING)                                       \
  CBC_OPCODE (CBC_CREATE_LOCAL, CBC_HAS_LITERAL_ARG, 0, VM_OC_CREATE_BINDING)                                       \
  CBC_OPCODE (CBC_INIT_ARG_OR_CATCH, CBC_HAS_LITERAL_ARG, -1, VM_OC_INIT_BINDING)                                   \
  CBC_OPCODE (CBC_INIT_LET, CBC_HAS_LITERAL_ARG, -1, VM_OC_INIT_BINDING)                                            \
  CBC_OPCODE (CBC_INIT_CONST, CBC_HAS_LITERAL_ARG, -1, VM_OC_INIT_BINDING)                                          \
  CBC_OPCODE (CBC_INIT_ARG_OR_FUNC, CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2, 0, VM_OC_INIT_ARG_OR_FUNC)          \
  CBC_OPCODE (CBC_CREATE_VAR_EVAL, CBC_HAS_LITERAL_ARG, 0, VM_OC_VAR_EVAL)                                          \
  CBC_OPCODE (CBC_CREATE_VAR_FUNC_EVAL, CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2, 0, VM_OC_VAR_EVAL)              \
  CBC_OPCODE (CBC_SET_VAR_FUNC,                                                                                     \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              0,                                                                                                    \
              VM_OC_ASSIGN | VM_OC_GET_LITERAL | VM_OC_PUT_IDENT)                                                   \
  CBC_OPCODE (CBC_SET_BYTECODE_PTR, CBC_NO_FLAG, 0, VM_OC_SET_BYTECODE_PTR)                                         \
  CBC_OPCODE (CBC_RETURN, CBC_NO_FLAG, -1, VM_OC_RETURN | VM_OC_GET_STACK)                                          \
  CBC_OPCODE (CBC_RETURN_FUNCTION_END, CBC_NO_FLAG, 0, VM_OC_RETURN_FUNCTION_END)                                   \
  CBC_OPCODE (CBC_RETURN_WITH_LITERAL, CBC_HAS_LITERAL_ARG, 0, VM_OC_RETURN | VM_OC_GET_LITERAL)                    \
  CBC_OPCODE (CBC_SET_LITERAL_PROPERTY,                                                                             \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              0,                                                                                                    \
              VM_OC_SET_PROPERTY | VM_OC_NON_STATIC_FLAG | VM_OC_GET_LITERAL_LITERAL)                               \
  CBC_OPCODE (CBC_COPY_TO_GLOBAL,                                                                                   \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              0,                                                                                                    \
              VM_OC_COPY_TO_GLOBAL | VM_OC_GET_LITERAL)                                                             \
                                                                                                                    \
  /* Unary opcodes. */                                                                                              \
  CBC_UNARY_OPERATION (CBC_PLUS, PLUS)                                                                              \
  CBC_UNARY_OPERATION (CBC_NEGATE, MINUS)                                                                           \
  CBC_UNARY_OPERATION (CBC_LOGICAL_NOT, NOT)                                                                        \
  CBC_UNARY_OPERATION (CBC_BIT_NOT, BIT_NOT)                                                                        \
  CBC_UNARY_OPERATION (CBC_VOID, VOID)                                                                              \
  CBC_OPCODE (CBC_TYPEOF, CBC_NO_FLAG, 0, VM_OC_TYPEOF | VM_OC_GET_STACK | VM_OC_PUT_STACK)                         \
  CBC_OPCODE (CBC_TYPEOF_IDENT, CBC_HAS_LITERAL_ARG, 1, VM_OC_TYPEOF_IDENT | VM_OC_PUT_STACK)                       \
                                                                                                                    \
  /* Binary opcodes. */                                                                                             \
  CBC_BINARY_OPERATION (CBC_BIT_OR, BIT_OR)                                                                         \
  CBC_BINARY_OPERATION (CBC_BIT_XOR, BIT_XOR)                                                                       \
  CBC_BINARY_OPERATION (CBC_BIT_AND, BIT_AND)                                                                       \
  CBC_BINARY_OPERATION (CBC_EQUAL, EQUAL)                                                                           \
  CBC_BINARY_OPERATION (CBC_NOT_EQUAL, NOT_EQUAL)                                                                   \
  CBC_BINARY_OPERATION (CBC_STRICT_EQUAL, STRICT_EQUAL)                                                             \
  CBC_BINARY_OPERATION (CBC_STRICT_NOT_EQUAL, STRICT_NOT_EQUAL)                                                     \
  CBC_BINARY_OPERATION (CBC_LESS, LESS)                                                                             \
  CBC_BINARY_OPERATION (CBC_GREATER, GREATER)                                                                       \
  CBC_BINARY_OPERATION (CBC_LESS_EQUAL, LESS_EQUAL)                                                                 \
  CBC_BINARY_OPERATION (CBC_GREATER_EQUAL, GREATER_EQUAL)                                                           \
  CBC_BINARY_OPERATION (CBC_IN, IN)                                                                                 \
  CBC_BINARY_OPERATION (CBC_INSTANCEOF, INSTANCEOF)                                                                 \
  CBC_BINARY_OPERATION (CBC_LEFT_SHIFT, LEFT_SHIFT)                                                                 \
  CBC_BINARY_OPERATION (CBC_RIGHT_SHIFT, RIGHT_SHIFT)                                                               \
  CBC_BINARY_OPERATION (CBC_UNS_RIGHT_SHIFT, UNS_RIGHT_SHIFT)                                                       \
  CBC_BINARY_OPERATION (CBC_ADD, ADD)                                                                               \
  CBC_BINARY_OPERATION (CBC_SUBTRACT, SUB)                                                                          \
  CBC_BINARY_OPERATION (CBC_MULTIPLY, MUL)                                                                          \
  CBC_BINARY_OPERATION (CBC_DIVIDE, DIV)                                                                            \
  CBC_BINARY_OPERATION (CBC_MODULO, MOD)                                                                            \
  CBC_BINARY_OPERATION (CBC_EXPONENTIATION, EXP)                                                                    \
                                                                                                                    \
  /* Unary lvalue opcodes. */                                                                                       \
  CBC_OPCODE (CBC_DELETE_PUSH_RESULT, CBC_NO_FLAG, -1, VM_OC_PROP_DELETE | VM_OC_GET_STACK_STACK | VM_OC_PUT_STACK) \
  CBC_OPCODE (CBC_DELETE_IDENT_PUSH_RESULT, CBC_HAS_LITERAL_ARG, 1, VM_OC_DELETE | VM_OC_PUT_STACK)                 \
  CBC_UNARY_LVALUE_OPERATION (CBC_PRE_INCR, PRE_INCR)                                                               \
  CBC_UNARY_LVALUE_OPERATION (CBC_PRE_DECR, PRE_DECR)                                                               \
  CBC_UNARY_LVALUE_OPERATION (CBC_POST_INCR, POST_INCR)                                                             \
  CBC_UNARY_LVALUE_OPERATION (CBC_POST_DECR, POST_DECR)                                                             \
                                                                                                                    \
  /* Call opcodes. */                                                                                               \
  CBC_OPCODE (CBC_CALL, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_CALL)                                                 \
  CBC_OPCODE (CBC_CALL_PUSH_RESULT, CBC_HAS_POP_STACK_BYTE_ARG, 0, VM_OC_CALL | VM_OC_PUT_STACK)                    \
  CBC_OPCODE (CBC_CALL_BLOCK, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_CALL | VM_OC_PUT_BLOCK)                         \
  CBC_OPCODE (CBC_CALL_PROP, CBC_HAS_POP_STACK_BYTE_ARG, -3, VM_OC_CALL)                                            \
  CBC_OPCODE (CBC_CALL_PROP_PUSH_RESULT, CBC_HAS_POP_STACK_BYTE_ARG, -2, VM_OC_CALL | VM_OC_PUT_STACK)              \
  CBC_OPCODE (CBC_CALL_PROP_BLOCK, CBC_HAS_POP_STACK_BYTE_ARG, -3, VM_OC_CALL | VM_OC_PUT_BLOCK)                    \
  CBC_OPCODE (CBC_CALL0, CBC_NO_FLAG, -1, VM_OC_CALL)                                                               \
  CBC_OPCODE (CBC_CALL0_PUSH_RESULT, CBC_NO_FLAG, 0, VM_OC_CALL | VM_OC_PUT_STACK)                                  \
  CBC_OPCODE (CBC_CALL0_BLOCK, CBC_NO_FLAG, -1, VM_OC_CALL | VM_OC_PUT_BLOCK)                                       \
  CBC_OPCODE (CBC_CALL0_PROP, CBC_NO_FLAG, -3, VM_OC_CALL)                                                          \
  CBC_OPCODE (CBC_CALL0_PROP_PUSH_RESULT, CBC_NO_FLAG, -2, VM_OC_CALL | VM_OC_PUT_STACK)                            \
  CBC_OPCODE (CBC_CALL0_PROP_BLOCK, CBC_NO_FLAG, -3, VM_OC_CALL | VM_OC_PUT_BLOCK)                                  \
  CBC_OPCODE (CBC_CALL1, CBC_NO_FLAG, -2, VM_OC_CALL)                                                               \
  CBC_OPCODE (CBC_CALL1_PUSH_RESULT, CBC_NO_FLAG, -1, VM_OC_CALL | VM_OC_PUT_STACK)                                 \
  CBC_OPCODE (CBC_CALL1_BLOCK, CBC_NO_FLAG, -2, VM_OC_CALL | VM_OC_PUT_BLOCK)                                       \
  CBC_OPCODE (CBC_CALL1_PROP, CBC_NO_FLAG, -4, VM_OC_CALL)                                                          \
  CBC_OPCODE (CBC_CALL1_PROP_PUSH_RESULT, CBC_NO_FLAG, -3, VM_OC_CALL | VM_OC_PUT_STACK)                            \
  CBC_OPCODE (CBC_CALL1_PROP_BLOCK, CBC_NO_FLAG, -4, VM_OC_CALL | VM_OC_PUT_BLOCK)                                  \
  CBC_OPCODE (CBC_CALL2, CBC_NO_FLAG, -3, VM_OC_CALL)                                                               \
  CBC_OPCODE (CBC_CALL2_PUSH_RESULT, CBC_NO_FLAG, -2, VM_OC_CALL | VM_OC_PUT_STACK)                                 \
  CBC_OPCODE (CBC_CALL2_BLOCK, CBC_NO_FLAG, -3, VM_OC_CALL | VM_OC_PUT_BLOCK)                                       \
  CBC_OPCODE (CBC_CALL2_PROP, CBC_NO_FLAG, -4, VM_OC_CALL)                                                          \
  CBC_OPCODE (CBC_CALL2_PROP_PUSH_RESULT, CBC_NO_FLAG, -3, VM_OC_CALL | VM_OC_PUT_STACK)                            \
  CBC_OPCODE (CBC_CALL2_PROP_BLOCK, CBC_NO_FLAG, -4, VM_OC_CALL | VM_OC_PUT_BLOCK)                                  \
                                                                                                                    \
  /* Binary assignment opcodes. */                                                                                  \
  CBC_OPCODE (CBC_ASSIGN, CBC_NO_FLAG, -3, VM_OC_ASSIGN | VM_OC_GET_STACK | VM_OC_PUT_REFERENCE)                    \
  CBC_OPCODE (CBC_ASSIGN_PUSH_RESULT,                                                                               \
              CBC_NO_FLAG,                                                                                          \
              -2,                                                                                                   \
              VM_OC_ASSIGN | VM_OC_GET_STACK | VM_OC_PUT_REFERENCE | VM_OC_PUT_STACK)                               \
  CBC_OPCODE (CBC_ASSIGN_BLOCK,                                                                                     \
              CBC_NO_FLAG,                                                                                          \
              -3,                                                                                                   \
              VM_OC_ASSIGN | VM_OC_GET_STACK | VM_OC_PUT_REFERENCE | VM_OC_PUT_BLOCK)                               \
  CBC_OPCODE (CBC_ASSIGN_SET_IDENT, CBC_HAS_LITERAL_ARG, -1, VM_OC_ASSIGN | VM_OC_GET_STACK | VM_OC_PUT_IDENT)      \
  CBC_OPCODE (CBC_ASSIGN_SET_IDENT_PUSH_RESULT,                                                                     \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              0,                                                                                                    \
              VM_OC_ASSIGN | VM_OC_GET_STACK | VM_OC_PUT_IDENT | VM_OC_PUT_STACK)                                   \
  CBC_OPCODE (CBC_ASSIGN_SET_IDENT_BLOCK,                                                                           \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              -1,                                                                                                   \
              VM_OC_ASSIGN | VM_OC_GET_STACK | VM_OC_PUT_IDENT | VM_OC_PUT_BLOCK)                                   \
  CBC_OPCODE (CBC_ASSIGN_LITERAL_SET_IDENT,                                                                         \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              0,                                                                                                    \
              VM_OC_ASSIGN | VM_OC_GET_LITERAL | VM_OC_PUT_IDENT)                                                   \
  CBC_OPCODE (CBC_ASSIGN_LITERAL_SET_IDENT_PUSH_RESULT,                                                             \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              1,                                                                                                    \
              VM_OC_ASSIGN | VM_OC_GET_LITERAL | VM_OC_PUT_IDENT | VM_OC_PUT_STACK)                                 \
  CBC_OPCODE (CBC_ASSIGN_LITERAL_SET_IDENT_BLOCK,                                                                   \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              0,                                                                                                    \
              VM_OC_ASSIGN | VM_OC_GET_LITERAL | VM_OC_PUT_IDENT | VM_OC_PUT_BLOCK)                                 \
  CBC_OPCODE (CBC_ASSIGN_PROP_LITERAL,                                                                              \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              -2,                                                                                                   \
              VM_OC_ASSIGN_PROP | VM_OC_GET_LITERAL | VM_OC_PUT_REFERENCE)                                          \
  CBC_OPCODE (CBC_ASSIGN_PROP_LITERAL_PUSH_RESULT,                                                                  \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              -1,                                                                                                   \
              VM_OC_ASSIGN_PROP | VM_OC_GET_LITERAL | VM_OC_PUT_REFERENCE | VM_OC_PUT_STACK)                        \
  CBC_OPCODE (CBC_ASSIGN_PROP_LITERAL_BLOCK,                                                                        \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              -2,                                                                                                   \
              VM_OC_ASSIGN_PROP | VM_OC_GET_LITERAL | VM_OC_PUT_REFERENCE | VM_OC_PUT_BLOCK)                        \
  CBC_OPCODE (CBC_ASSIGN_PROP_THIS_LITERAL,                                                                         \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              -1,                                                                                                   \
              VM_OC_ASSIGN_PROP_THIS | VM_OC_GET_LITERAL | VM_OC_PUT_REFERENCE)                                     \
  CBC_OPCODE (CBC_ASSIGN_PROP_THIS_LITERAL_PUSH_RESULT,                                                             \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              0,                                                                                                    \
              VM_OC_ASSIGN_PROP_THIS | VM_OC_GET_LITERAL | VM_OC_PUT_REFERENCE | VM_OC_PUT_STACK)                   \
  CBC_OPCODE (CBC_ASSIGN_PROP_THIS_LITERAL_BLOCK,                                                                   \
              CBC_HAS_LITERAL_ARG,                                                                                  \
              -1,                                                                                                   \
              VM_OC_ASSIGN_PROP_THIS | VM_OC_GET_LITERAL | VM_OC_PUT_REFERENCE | VM_OC_PUT_BLOCK)                   \
  CBC_OPCODE (CBC_MOV_IDENT, CBC_HAS_LITERAL_ARG, -1, VM_OC_MOV_IDENT | VM_OC_GET_STACK | VM_OC_PUT_IDENT)          \
  CBC_OPCODE (CBC_ASSIGN_LET_CONST, CBC_HAS_LITERAL_ARG, -1, VM_OC_ASSIGN_LET_CONST | VM_OC_GET_STACK)              \
  CBC_OPCODE (CBC_ASSIGN_LET_CONST_LITERAL,                                                                         \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                           \
              0,                                                                                                    \
              VM_OC_ASSIGN_LET_CONST | VM_OC_GET_LITERAL)                                                           \
                                                                                                                    \
  /* Last opcode (not a real opcode). */                                                                            \
  CBC_OPCODE (CBC_END, CBC_NO_FLAG, 0, VM_OC_NONE)

/* All EXT branches are statement block end
 * marks, so they are always forward branches. */

#define CBC_EXT_OPCODE_LIST                                                                                            \
  /* Branch opcodes first. Some other opcodes are mixed. */                                                            \
  CBC_OPCODE (CBC_EXT_NOP, CBC_NO_FLAG, 0, VM_OC_NONE)                                                                 \
  CBC_FORWARD_BRANCH (CBC_EXT_WITH_CREATE_CONTEXT, -1 + PARSER_WITH_CONTEXT_STACK_ALLOCATION, VM_OC_WITH)              \
  CBC_OPCODE (CBC_EXT_FOR_IN_GET_NEXT, CBC_NO_FLAG, 1, VM_OC_FOR_IN_GET_NEXT | VM_OC_PUT_STACK)                        \
  CBC_FORWARD_BRANCH (CBC_EXT_FOR_IN_INIT, -1 + PARSER_FOR_IN_CONTEXT_STACK_ALLOCATION, VM_OC_FOR_IN_INIT)             \
  CBC_OPCODE (CBC_EXT_SET_GETTER,                                                                                      \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_SET_GETTER | VM_OC_NON_STATIC_FLAG | VM_OC_GET_LITERAL_LITERAL)                                    \
  CBC_BACKWARD_BRANCH (CBC_EXT_BRANCH_IF_FOR_IN_HAS_NEXT, 0, VM_OC_FOR_IN_HAS_NEXT)                                    \
  CBC_OPCODE (CBC_EXT_FOR_OF_GET_NEXT, CBC_NO_FLAG, 1, VM_OC_FOR_OF_GET_NEXT | VM_OC_PUT_STACK)                        \
  CBC_FORWARD_BRANCH (CBC_EXT_FOR_OF_INIT, -1 + PARSER_FOR_OF_CONTEXT_STACK_ALLOCATION, VM_OC_FOR_OF_INIT)             \
  CBC_OPCODE (CBC_EXT_PUSH_NAMED_FUNC_EXPRESSION,                                                                      \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              1,                                                                                                       \
              VM_OC_PUSH_NAMED_FUNC_EXPR | VM_OC_GET_LITERAL_LITERAL)                                                  \
  CBC_BACKWARD_BRANCH (CBC_EXT_BRANCH_IF_FOR_OF_HAS_NEXT, 0, VM_OC_FOR_OF_HAS_NEXT)                                    \
  CBC_OPCODE (CBC_EXT_CLONE_CONTEXT, CBC_NO_FLAG, 0, VM_OC_CLONE_CONTEXT)                                              \
  CBC_FORWARD_BRANCH (CBC_EXT_FOR_AWAIT_OF_INIT,                                                                       \
                      -1 + PARSER_FOR_AWAIT_OF_CONTEXT_STACK_ALLOCATION,                                               \
                      VM_OC_FOR_AWAIT_OF_INIT)                                                                         \
  CBC_OPCODE (CBC_EXT_CLONE_FULL_CONTEXT, CBC_NO_FLAG, 0, VM_OC_CLONE_CONTEXT)                                         \
  CBC_BACKWARD_BRANCH (CBC_EXT_BRANCH_IF_FOR_AWAIT_OF_HAS_NEXT, 0, VM_OC_FOR_AWAIT_OF_HAS_NEXT)                        \
  CBC_OPCODE (CBC_EXT_SET_SETTER,                                                                                      \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_SET_SETTER | VM_OC_NON_STATIC_FLAG | VM_OC_GET_LITERAL_LITERAL)                                    \
  CBC_FORWARD_BRANCH (CBC_EXT_TRY_CREATE_CONTEXT, PARSER_TRY_CONTEXT_STACK_ALLOCATION, VM_OC_TRY)                      \
  CBC_OPCODE (CBC_EXT_TRY_CREATE_ENV, CBC_NO_FLAG, 0, VM_OC_BLOCK_CREATE_CONTEXT)                                      \
  CBC_FORWARD_BRANCH (CBC_EXT_CATCH, 1, VM_OC_CATCH)                                                                   \
  CBC_OPCODE (CBC_EXT_RESOLVE_BASE, CBC_NO_FLAG, 0, VM_OC_RESOLVE_BASE_FOR_CALL)                                       \
  CBC_FORWARD_BRANCH (CBC_EXT_FINALLY, PARSER_FINALLY_CONTEXT_EXTRA_STACK_ALLOCATION, VM_OC_FINALLY)                   \
  CBC_OPCODE (CBC_EXT_INITIALIZER_PUSH_PROP, CBC_NO_FLAG, 0, VM_OC_INITIALIZER_PUSH_PROP)                              \
  CBC_FORWARD_BRANCH (CBC_EXT_DEFAULT_INITIALIZER, -1, VM_OC_DEFAULT_INITIALIZER)                                      \
  CBC_OPCODE (CBC_EXT_ERROR, CBC_NO_FLAG, 0, VM_OC_ERROR)                                                              \
  CBC_FORWARD_BRANCH (CBC_EXT_BRANCH_IF_NULLISH, -1, VM_OC_BRANCH_IF_NULLISH)                                          \
                                                                                                                       \
  /* Basic opcodes. */                                                                                                 \
  CBC_OPCODE (CBC_EXT_POP_REFERENCE, CBC_NO_FLAG, -2, VM_OC_POP_REFERENCE)                                             \
  CBC_OPCODE (CBC_EXT_CREATE_ARGUMENTS, CBC_HAS_LITERAL_ARG, 0, VM_OC_CREATE_ARGUMENTS)                                \
  CBC_OPCODE (CBC_EXT_CREATE_VAR_EVAL, CBC_HAS_LITERAL_ARG, 0, VM_OC_EXT_VAR_EVAL)                                     \
  CBC_OPCODE (CBC_EXT_CREATE_VAR_FUNC_EVAL, CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2, 0, VM_OC_EXT_VAR_EVAL)         \
  CBC_OPCODE (CBC_EXT_COPY_FROM_ARG, CBC_HAS_LITERAL_ARG, 0, VM_OC_COPY_FROM_ARG)                                      \
  CBC_OPCODE (CBC_EXT_PUSH_REST_OBJECT, CBC_NO_FLAG, 1, VM_OC_PUSH_REST_OBJECT)                                        \
  CBC_OPCODE (CBC_EXT_MODULE_IMPORT, CBC_NO_FLAG, 0, VM_OC_MODULE_IMPORT)                                              \
  CBC_OPCODE (CBC_EXT_MODULE_IMPORT_META, CBC_NO_FLAG, 1, VM_OC_MODULE_IMPORT_META)                                    \
  CBC_OPCODE (CBC_EXT_STRING_CONCAT, CBC_NO_FLAG, -1, VM_OC_STRING_CONCAT | VM_OC_GET_STACK_STACK | VM_OC_PUT_STACK)   \
  CBC_OPCODE (CBC_EXT_STRING_CONCAT_RIGHT_LITERAL,                                                                     \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              0,                                                                                                       \
              VM_OC_STRING_CONCAT | VM_OC_GET_STACK_LITERAL | VM_OC_PUT_STACK)                                         \
  CBC_OPCODE (CBC_EXT_STRING_CONCAT_TWO_LITERALS,                                                                      \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              1,                                                                                                       \
              VM_OC_STRING_CONCAT | VM_OC_GET_LITERAL_LITERAL | VM_OC_PUT_STACK)                                       \
  CBC_OPCODE (CBC_EXT_GET_TAGGED_TEMPLATE_LITERAL, CBC_HAS_BYTE_ARG, 1, VM_OC_GET_TEMPLATE_OBJECT | VM_OC_PUT_STACK)   \
  CBC_OPCODE (CBC_EXT_THROW_REFERENCE_ERROR, CBC_NO_FLAG, 1, VM_OC_THROW_REFERENCE_ERROR)                              \
  CBC_OPCODE (CBC_EXT_THROW_ASSIGN_CONST_ERROR, CBC_NO_FLAG, 0, VM_OC_THROW_CONST_ERROR)                               \
  CBC_OPCODE (CBC_EXT_REQUIRE_OBJECT_COERCIBLE, CBC_NO_FLAG, 0, VM_OC_REQUIRE_OBJECT_COERCIBLE)                        \
  CBC_OPCODE (CBC_EXT_COPY_DATA_PROPERTIES, CBC_NO_FLAG, -1, VM_OC_COPY_DATA_PROPERTIES)                               \
  CBC_OPCODE (CBC_EXT_SET_FUNCTION_NAME, CBC_HAS_LITERAL_ARG, 0, VM_OC_SET_FUNCTION_NAME | VM_OC_GET_LITERAL)          \
  CBC_OPCODE (CBC_EXT_SET_CLASS_NAME, CBC_HAS_LITERAL_ARG, 0, VM_OC_SET_FUNCTION_NAME)                                 \
  CBC_OPCODE (CBC_EXT_SET_COMPUTED_FUNCTION_NAME, CBC_NO_FLAG, 0, VM_OC_SET_FUNCTION_NAME)                             \
  CBC_OPCODE (CBC_EXT_SET_COMPUTED_GETTER_NAME, CBC_NO_FLAG, 0, VM_OC_SET_FUNCTION_NAME)                               \
  CBC_OPCODE (CBC_EXT_SET_COMPUTED_SETTER_NAME, CBC_NO_FLAG, 0, VM_OC_SET_FUNCTION_NAME)                               \
                                                                                                                       \
  /* Computed / class property related opcodes. */                                                                     \
  CBC_OPCODE (CBC_EXT_SET_COMPUTED_PROPERTY,                                                                           \
              CBC_NO_FLAG,                                                                                             \
              -2,                                                                                                      \
              VM_OC_SET_COMPUTED_PROPERTY | VM_OC_NON_STATIC_FLAG | VM_OC_GET_STACK_STACK)                             \
  CBC_OPCODE (CBC_EXT_SET_COMPUTED_PROPERTY_LITERAL,                                                                   \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              -1,                                                                                                      \
              VM_OC_SET_COMPUTED_PROPERTY | VM_OC_NON_STATIC_FLAG | VM_OC_GET_STACK_LITERAL)                           \
  CBC_OPCODE (CBC_EXT_SET_COMPUTED_GETTER,                                                                             \
              CBC_NO_FLAG,                                                                                             \
              -2,                                                                                                      \
              VM_OC_SET_GETTER | VM_OC_NON_STATIC_FLAG | VM_OC_GET_STACK_STACK)                                        \
  CBC_OPCODE (CBC_EXT_SET_COMPUTED_SETTER,                                                                             \
              CBC_NO_FLAG,                                                                                             \
              -2,                                                                                                      \
              VM_OC_SET_SETTER | VM_OC_NON_STATIC_FLAG | VM_OC_GET_STACK_STACK)                                        \
  CBC_OPCODE (CBC_EXT_SET_STATIC_PROPERTY, CBC_HAS_LITERAL_ARG, -1, VM_OC_SET_PROPERTY | VM_OC_GET_STACK_LITERAL)      \
  CBC_OPCODE (CBC_EXT_SET_STATIC_PROPERTY_LITERAL,                                                                     \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_SET_PROPERTY | VM_OC_GET_LITERAL_LITERAL)                                                          \
  CBC_OPCODE (CBC_EXT_SET_STATIC_COMPUTED_PROPERTY,                                                                    \
              CBC_NO_FLAG,                                                                                             \
              -2,                                                                                                      \
              VM_OC_SET_COMPUTED_PROPERTY | VM_OC_GET_STACK_STACK)                                                     \
  CBC_OPCODE (CBC_EXT_SET_STATIC_GETTER,                                                                               \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_SET_GETTER | VM_OC_GET_LITERAL_LITERAL)                                                            \
  CBC_OPCODE (CBC_EXT_SET_STATIC_SETTER,                                                                               \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_SET_SETTER | VM_OC_GET_LITERAL_LITERAL)                                                            \
  CBC_OPCODE (CBC_EXT_SET_STATIC_COMPUTED_GETTER, CBC_NO_FLAG, -2, VM_OC_SET_GETTER | VM_OC_GET_STACK_STACK)           \
  CBC_OPCODE (CBC_EXT_SET_STATIC_COMPUTED_SETTER, CBC_NO_FLAG, -2, VM_OC_SET_SETTER | VM_OC_GET_STACK_STACK)           \
  CBC_OPCODE (CBC_EXT_SET__PROTO__, CBC_NO_FLAG, -1, VM_OC_SET__PROTO__ | VM_OC_GET_STACK)                             \
  CBC_OPCODE (CBC_EXT_PUSH_STATIC_FIELD_FUNC,                                                                          \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              1,                                                                                                       \
              VM_OC_PUSH_STATIC_FIELD_FUNC | VM_OC_GET_LITERAL)                                                        \
  CBC_OPCODE (CBC_EXT_PUSH_STATIC_COMPUTED_FIELD_FUNC,                                                                 \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              0,                                                                                                       \
              VM_OC_PUSH_STATIC_FIELD_FUNC | VM_OC_GET_LITERAL)                                                        \
  CBC_OPCODE (CBC_EXT_ADD_COMPUTED_FIELD, CBC_NO_FLAG, -1, VM_OC_ADD_COMPUTED_FIELD | VM_OC_GET_STACK)                 \
  CBC_OPCODE (CBC_EXT_ADD_STATIC_COMPUTED_FIELD, CBC_NO_FLAG, -1, VM_OC_ADD_COMPUTED_FIELD | VM_OC_GET_STACK)          \
  CBC_OPCODE (CBC_EXT_CLASS_CALL_STATIC_BLOCK,                                                                         \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              0,                                                                                                       \
              VM_OC_CLASS_CALL_STATIC_BLOCK | VM_OC_GET_LITERAL)                                                       \
  /* Class private property related opcodes */                                                                         \
  CBC_OPCODE (CBC_EXT_PUSH_PRIVATE_PROP_LITERAL_REFERENCE,                                                             \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              2,                                                                                                       \
              VM_OC_PRIVATE_PROP_REFERENCE | VM_OC_GET_LITERAL | VM_OC_PUT_STACK)                                      \
  CBC_OPCODE (CBC_EXT_PUSH_PRIVATE_PROP_LITERAL,                                                                       \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              0,                                                                                                       \
              VM_OC_PRIVATE_PROP_GET | VM_OC_GET_STACK_LITERAL | VM_OC_PUT_STACK)                                      \
  CBC_OPCODE (CBC_EXT_PUSH_PRIVATE_PROP_LITERAL_IN,                                                                    \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              0,                                                                                                       \
              VM_OC_PRIVATE_IN | VM_OC_GET_STACK_LITERAL | VM_OC_PUT_STACK)                                            \
  CBC_OPCODE (CBC_EXT_PRIVATE_FIELD_ADD, CBC_HAS_LITERAL_ARG, -1, VM_OC_PRIVATE_FIELD_ADD | VM_OC_GET_STACK_LITERAL)   \
  /* These 8 opcodes must be in this order  */                                                                         \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_FIELD,                                                                           \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL)                                                      \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_METHOD,                                                                          \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL_LITERAL)                                              \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_GETTER,                                                                          \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL_LITERAL)                                              \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_SETTER,                                                                          \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL_LITERAL)                                              \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_STATIC_FIELD,                                                                    \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL)                                                      \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_STATIC_METHOD,                                                                   \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL_LITERAL)                                              \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_STATIC_GETTER,                                                                   \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL_LITERAL)                                              \
  CBC_OPCODE (CBC_EXT_COLLECT_PRIVATE_STATIC_SETTER,                                                                   \
              CBC_HAS_LITERAL_ARG | CBC_HAS_LITERAL_ARG2,                                                              \
              0,                                                                                                       \
              VM_OC_COLLECT_PRIVATE_PROPERTY | VM_OC_GET_LITERAL_LITERAL)                                              \
  /* Class related opcodes. */                                                                                         \
  CBC_OPCODE (CBC_EXT_PUSH_NAMED_CLASS_ENV, CBC_HAS_LITERAL_ARG, 1, VM_OC_PUSH_CLASS_ENVIRONMENT)                      \
  CBC_OPCODE (CBC_EXT_DEFINE_FIELD, CBC_HAS_LITERAL_ARG, -1, VM_OC_DEFINE_FIELD | VM_OC_GET_STACK_LITERAL)             \
  CBC_OPCODE (CBC_EXT_PUSH_IMPLICIT_CONSTRUCTOR, CBC_NO_FLAG, 1, VM_OC_PUSH_IMPLICIT_CTOR | VM_OC_PUT_STACK)           \
  CBC_OPCODE (CBC_EXT_PUSH_IMPLICIT_CONSTRUCTOR_HERITAGE, CBC_NO_FLAG, 1, VM_OC_PUSH_IMPLICIT_CTOR | VM_OC_PUT_STACK)  \
  CBC_OPCODE (CBC_EXT_INIT_CLASS, CBC_NO_FLAG, 0, VM_OC_INIT_CLASS | VM_OC_PUT_STACK)                                  \
  CBC_OPCODE (CBC_EXT_FINALIZE_NAMED_CLASS, CBC_HAS_LITERAL_ARG, -2, VM_OC_FINALIZE_CLASS)                             \
  CBC_OPCODE (CBC_EXT_FINALIZE_ANONYMOUS_CLASS, CBC_NO_FLAG, -2, VM_OC_FINALIZE_CLASS)                                 \
  CBC_OPCODE (CBC_EXT_SET_FIELD_INIT, CBC_HAS_LITERAL_ARG, 0, VM_OC_SET_FIELD_INIT | VM_OC_GET_LITERAL)                \
  CBC_OPCODE (CBC_EXT_RUN_FIELD_INIT, CBC_NO_FLAG, 0, VM_OC_RUN_FIELD_INIT)                                            \
  CBC_OPCODE (CBC_EXT_RUN_STATIC_FIELD_INIT, CBC_NO_FLAG, -1, VM_OC_RUN_STATIC_FIELD_INIT)                             \
  CBC_OPCODE (CBC_EXT_SET_NEXT_COMPUTED_FIELD_ANONYMOUS_FUNC,                                                          \
              CBC_NO_FLAG,                                                                                             \
              -1,                                                                                                      \
              VM_OC_SET_NEXT_COMPUTED_FIELD | VM_OC_PUT_REFERENCE)                                                     \
  CBC_OPCODE (CBC_EXT_SET_NEXT_COMPUTED_FIELD, CBC_NO_FLAG, -1, VM_OC_SET_NEXT_COMPUTED_FIELD | VM_OC_PUT_REFERENCE)   \
  CBC_OPCODE (CBC_EXT_PUSH_SUPER, CBC_NO_FLAG, 1, VM_OC_NONE)                                                          \
  CBC_OPCODE (CBC_EXT_PUSH_SUPER_CONSTRUCTOR, CBC_NO_FLAG, 1, VM_OC_PUSH_SUPER_CONSTRUCTOR)                            \
  CBC_OPCODE (CBC_EXT_PUSH_SUPER_PROP, CBC_NO_FLAG, 0, VM_OC_SUPER_REFERENCE | VM_OC_GET_STACK)                        \
  CBC_OPCODE (CBC_EXT_SUPER_PROP_REFERENCE, CBC_NO_FLAG, 2, VM_OC_SUPER_REFERENCE | VM_OC_GET_STACK)                   \
  CBC_OPCODE (CBC_EXT_PUSH_SUPER_PROP_LITERAL, CBC_HAS_LITERAL_ARG, 1, VM_OC_SUPER_REFERENCE | VM_OC_GET_LITERAL)      \
  CBC_OPCODE (CBC_EXT_SUPER_PROP_LITERAL_REFERENCE, CBC_HAS_LITERAL_ARG, 3, VM_OC_SUPER_REFERENCE | VM_OC_GET_LITERAL) \
  CBC_OPCODE (CBC_EXT_SUPER_PROP_ASSIGNMENT_REFERENCE, CBC_NO_FLAG, 1, VM_OC_SUPER_REFERENCE | VM_OC_GET_STACK)        \
  CBC_OPCODE (CBC_EXT_SUPER_PROP_LITERAL_ASSIGNMENT_REFERENCE,                                                         \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              2,                                                                                                       \
              VM_OC_SUPER_REFERENCE | VM_OC_GET_LITERAL)                                                               \
  CBC_OPCODE (CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT, CBC_NO_FLAG, 0, VM_OC_SET_HOME_OBJECT)                           \
  CBC_OPCODE (CBC_EXT_OBJECT_LITERAL_SET_HOME_OBJECT_COMPUTED, CBC_NO_FLAG, 0, VM_OC_SET_HOME_OBJECT)                  \
  CBC_OPCODE (CBC_EXT_PUSH_OBJECT_SUPER_ENVIRONMENT, CBC_NO_FLAG, 1, VM_OC_OBJECT_LITERAL_HOME_ENV)                    \
  CBC_OPCODE (CBC_EXT_POP_OBJECT_SUPER_ENVIRONMENT, CBC_NO_FLAG, -1, VM_OC_OBJECT_LITERAL_HOME_ENV)                    \
  CBC_OPCODE (CBC_EXT_RESOLVE_LEXICAL_THIS, CBC_NO_FLAG, 1, VM_OC_RESOLVE_LEXICAL_THIS | VM_OC_PUT_STACK)              \
  CBC_OPCODE (CBC_EXT_LOCAL_EVAL, CBC_HAS_BYTE_ARG, 0, VM_OC_LOCAL_EVAL)                                               \
  CBC_OPCODE (CBC_EXT_ASSIGN_SUPER, CBC_NO_FLAG, -3, VM_OC_ASSIGN_SUPER)                                               \
  CBC_OPCODE (CBC_EXT_ASSIGN_SUPER_PUSH_RESULT, CBC_NO_FLAG, -2, VM_OC_ASSIGN_SUPER | VM_OC_PUT_STACK)                 \
  CBC_OPCODE (CBC_EXT_ASSIGN_SUPER_BLOCK, CBC_NO_FLAG, -3, VM_OC_ASSIGN_SUPER | VM_OC_PUT_BLOCK)                       \
  CBC_OPCODE (CBC_EXT_ASSIGN_PRIVATE, CBC_NO_FLAG, -3, VM_OC_ASSIGN_PRIVATE)                                           \
  CBC_OPCODE (CBC_EXT_ASSIGN_PRIVATE_PUSH_RESULT, CBC_NO_FLAG, -2, VM_OC_ASSIGN_PRIVATE | VM_OC_PUT_STACK)             \
  CBC_OPCODE (CBC_EXT_ASSIGN_PRIVATE_BLOCK, CBC_NO_FLAG, -3, VM_OC_ASSIGN_PRIVATE | VM_OC_PUT_BLOCK)                   \
  CBC_OPCODE (CBC_EXT_SUPER_CALL, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_SUPER_CALL)                                    \
  CBC_OPCODE (CBC_EXT_SUPER_CALL_PUSH_RESULT, CBC_HAS_POP_STACK_BYTE_ARG, 0, VM_OC_SUPER_CALL | VM_OC_PUT_STACK)       \
  CBC_OPCODE (CBC_EXT_SUPER_CALL_BLOCK, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_SUPER_CALL | VM_OC_PUT_BLOCK)            \
  CBC_OPCODE (CBC_EXT_SPREAD_SUPER_CALL, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_SUPER_CALL)                             \
  CBC_OPCODE (CBC_EXT_SPREAD_SUPER_CALL_PUSH_RESULT,                                                                   \
              CBC_HAS_POP_STACK_BYTE_ARG,                                                                              \
              0,                                                                                                       \
              VM_OC_SUPER_CALL | VM_OC_PUT_STACK)                                                                      \
  CBC_OPCODE (CBC_EXT_SPREAD_SUPER_CALL_BLOCK, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_SUPER_CALL | VM_OC_PUT_BLOCK)     \
                                                                                                                       \
  /* Spread / rest operation related opcodes. */                                                                       \
  CBC_OPCODE (CBC_EXT_SPREAD_CALL, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_SPREAD_ARGUMENTS)                             \
  CBC_OPCODE (CBC_EXT_SPREAD_CALL_PUSH_RESULT,                                                                         \
              CBC_HAS_POP_STACK_BYTE_ARG,                                                                              \
              0,                                                                                                       \
              VM_OC_SPREAD_ARGUMENTS | VM_OC_PUT_STACK)                                                                \
  CBC_OPCODE (CBC_EXT_SPREAD_CALL_BLOCK, CBC_HAS_POP_STACK_BYTE_ARG, -1, VM_OC_SPREAD_ARGUMENTS | VM_OC_PUT_BLOCK)     \
  CBC_OPCODE (CBC_EXT_SPREAD_CALL_PROP, CBC_HAS_POP_STACK_BYTE_ARG, -3, VM_OC_SPREAD_ARGUMENTS)                        \
  CBC_OPCODE (CBC_EXT_SPREAD_CALL_PROP_PUSH_RESULT,                                                                    \
              CBC_HAS_POP_STACK_BYTE_ARG,                                                                              \
              -2,                                                                                                      \
              VM_OC_SPREAD_ARGUMENTS | VM_OC_PUT_STACK)                                                                \
  CBC_OPCODE (CBC_EXT_SPREAD_CALL_PROP_BLOCK,                                                                          \
              CBC_HAS_POP_STACK_BYTE_ARG,                                                                              \
              -3,                                                                                                      \
              VM_OC_SPREAD_ARGUMENTS | VM_OC_PUT_BLOCK)                                                                \
  CBC_OPCODE (CBC_EXT_PUSH_SPREAD_ELEMENT, CBC_NO_FLAG, 1, VM_OC_PUSH_SPREAD_ELEMENT)                                  \
  CBC_OPCODE (CBC_EXT_SPREAD_ARRAY_APPEND, CBC_HAS_POP_STACK_BYTE_ARG, 0, VM_OC_APPEND_ARRAY)                          \
  CBC_OPCODE (CBC_EXT_REST_INITIALIZER, CBC_NO_FLAG, 1, VM_OC_REST_INITIALIZER)                                        \
  CBC_OPCODE (CBC_EXT_INITIALIZER_PUSH_PROP_LITERAL,                                                                   \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              1,                                                                                                       \
              VM_OC_INITIALIZER_PUSH_PROP | VM_OC_GET_LITERAL)                                                         \
  CBC_OPCODE (CBC_EXT_SPREAD_NEW, CBC_HAS_POP_STACK_BYTE_ARG, 0, VM_OC_SPREAD_ARGUMENTS | VM_OC_PUT_STACK)             \
                                                                                                                       \
  /* Iterator related opcodes. */                                                                                      \
  CBC_OPCODE (CBC_EXT_ITERATOR_CONTEXT_CREATE,                                                                         \
              CBC_NO_FLAG,                                                                                             \
              PARSER_ITERATOR_CONTEXT_STACK_ALLOCATION,                                                                \
              VM_OC_ITERATOR_CONTEXT_CREATE)                                                                           \
  CBC_OPCODE (CBC_EXT_ITERATOR_CONTEXT_END,                                                                            \
              CBC_NO_FLAG,                                                                                             \
              -PARSER_ITERATOR_CONTEXT_STACK_ALLOCATION,                                                               \
              VM_OC_ITERATOR_CONTEXT_END)                                                                              \
  CBC_OPCODE (CBC_EXT_ITERATOR_STEP, CBC_NO_FLAG, 1, VM_OC_ITERATOR_STEP)                                              \
                                                                                                                       \
  /* Object initializer related opcodes. */                                                                            \
  CBC_OPCODE (CBC_EXT_OBJ_INIT_CONTEXT_CREATE,                                                                         \
              CBC_NO_FLAG,                                                                                             \
              PARSER_OBJ_INIT_CONTEXT_STACK_ALLOCATION,                                                                \
              VM_OC_OBJ_INIT_CONTEXT_CREATE)                                                                           \
  CBC_OPCODE (CBC_EXT_OBJ_INIT_REST_CONTEXT_CREATE,                                                                    \
              CBC_NO_FLAG,                                                                                             \
              PARSER_OBJ_INIT_REST_CONTEXT_STACK_ALLOCATION,                                                           \
              VM_OC_OBJ_INIT_CONTEXT_CREATE)                                                                           \
  CBC_OPCODE (CBC_EXT_OBJ_INIT_PUSH_REST, CBC_NO_FLAG, 1, VM_OC_OBJ_INIT_PUSH_REST)                                    \
  CBC_OPCODE (CBC_EXT_OBJ_INIT_CONTEXT_END,                                                                            \
              CBC_NO_FLAG,                                                                                             \
              -PARSER_OBJ_INIT_CONTEXT_STACK_ALLOCATION,                                                               \
              VM_OC_OBJ_INIT_CONTEXT_END)                                                                              \
  CBC_OPCODE (CBC_EXT_INITIALIZER_PUSH_NAME, CBC_NO_FLAG, 0, VM_OC_INITIALIZER_PUSH_NAME | VM_OC_GET_STACK)            \
  CBC_OPCODE (CBC_EXT_INITIALIZER_PUSH_NAME_LITERAL,                                                                   \
              CBC_HAS_LITERAL_ARG,                                                                                     \
              1,                                                                                                       \
              VM_OC_INITIALIZER_PUSH_NAME | VM_OC_GET_LITERAL)                                                         \
                                                                                                                       \
  /* Executable object related opcodes. */                                                                             \
  CBC_OPCODE (CBC_EXT_CREATE_GENERATOR, CBC_NO_FLAG, 1, VM_OC_CREATE_GENERATOR)                                        \
  CBC_OPCODE (CBC_EXT_YIELD, CBC_NO_FLAG, 0, VM_OC_YIELD)                                                              \
  CBC_OPCODE (CBC_EXT_YIELD_ITERATOR, CBC_NO_FLAG, 0, VM_OC_YIELD)                                                     \
  CBC_OPCODE (CBC_EXT_ASYNC_YIELD, CBC_NO_FLAG, 0, VM_OC_ASYNC_YIELD)                                                  \
  CBC_OPCODE (CBC_EXT_ASYNC_YIELD_ITERATOR, CBC_NO_FLAG, 0, VM_OC_ASYNC_YIELD_ITERATOR)                                \
  CBC_OPCODE (CBC_EXT_AWAIT, CBC_NO_FLAG, 0, VM_OC_AWAIT)                                                              \
  CBC_OPCODE (CBC_EXT_GENERATOR_AWAIT, CBC_NO_FLAG, 0, VM_OC_GENERATOR_AWAIT)                                          \
  CBC_OPCODE (CBC_EXT_ASYNC_EXIT, CBC_NO_FLAG, 0, VM_OC_ASYNC_EXIT)                                                    \
  CBC_OPCODE (CBC_EXT_RETURN, CBC_NO_FLAG, -1, VM_OC_EXT_RETURN | VM_OC_GET_STACK)                                     \
  CBC_OPCODE (CBC_EXT_RETURN_UNDEFINED, CBC_NO_FLAG, 0, VM_OC_EXT_RETURN)                                              \
  CBC_OPCODE (CBC_EXT_PUSH_NEW_TARGET, CBC_NO_FLAG, 1, VM_OC_PUSH_NEW_TARGET | VM_OC_PUT_STACK)                        \
                                                                                                                       \
  /* Last opcode (not a real opcode). */                                                                               \
  CBC_OPCODE (CBC_EXT_END, CBC_NO_FLAG, 0, VM_OC_NONE)

#define CBC_MAXIMUM_BYTE_VALUE  255
#define CBC_MAXIMUM_SMALL_VALUE 510
#define CBC_MAXIMUM_FULL_VALUE  32767

#define CBC_PUSH_NUMBER_BYTE_RANGE_END 256

#define CBC_HIGHEST_BIT_MASK     0x80
#define CBC_LOWER_SEVEN_BIT_MASK 0x7f

/**
 * Literal encoding limit when full literal encoding mode is enabled
 */
#define CBC_FULL_LITERAL_ENCODING_LIMIT 128

/**
 * Literal encoding delta when full literal encoding mode is enabled
 */
#define CBC_FULL_LITERAL_ENCODING_DELTA 0x8000

/**
 * Literal encoding limit when full literal encoding mode is disabled
 */
#define CBC_SMALL_LITERAL_ENCODING_LIMIT 255

/**
 * Literal encoding delta when full literal encoding mode is disabled
 */
#define CBC_SMALL_LITERAL_ENCODING_DELTA 0xfe01

/**
 * Literal indices belong to one of the following groups:
 *
 * 0 <= index < argument_end                    : arguments
 * argument_end <= index < register_end         : registers
 * register_end <= index < ident_end            : identifiers
 * ident_end <= index < const_literal_end       : constant literals
 * const_literal_end <= index < literal_end     : template literals
 */

/**
 * Compiled byte code arguments.
 */
typedef struct
{
  ecma_compiled_code_t header; /**< compiled code header */
  uint8_t stack_limit; /**< maximum number of values stored on the stack */
  uint8_t argument_end; /**< number of arguments expected by the function */
  ecma_value_t script_value; /**< script value */
  uint8_t register_end; /**< end position of the register group */
  uint8_t ident_end; /**< end position of the identifier group */
  uint8_t const_literal_end; /**< end position of the const literal group */
  uint8_t literal_end; /**< end position of the literal group */
} cbc_uint8_arguments_t;

/**
 * Compiled byte code arguments.
 */
typedef struct
{
  ecma_compiled_code_t header; /**< compiled code header */
  uint16_t stack_limit; /**< maximum number of values stored on the stack */
  ecma_value_t script_value; /**< script value */
  uint16_t argument_end; /**< number of arguments expected by the function */
  uint16_t register_end; /**< end position of the register group */
  uint16_t ident_end; /**< end position of the identifier group */
  uint16_t const_literal_end; /**< end position of the const literal group */
  uint16_t literal_end; /**< end position of the literal group */
  uint16_t padding; /**< an unused value */
} cbc_uint16_arguments_t;

/**
 * Compact byte code status flags.
 */
typedef enum
{
  CBC_CODE_FLAGS_FULL_LITERAL_ENCODING = (1u << 0), /**< full literal encoding mode is enabled */
  CBC_CODE_FLAGS_UINT16_ARGUMENTS = (1u << 1), /**< compiled code data is cbc_uint16_arguments_t */
  CBC_CODE_FLAGS_STRICT_MODE = (1u << 2), /**< strict mode is enabled */
  CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED = (1u << 3), /**< mapped arguments object must be constructed */
  CBC_CODE_FLAGS_LEXICAL_ENV_NOT_NEEDED = (1u << 4), /**< no need to create a lexical environment */
  CBC_CODE_FLAGS_HAS_EXTENDED_INFO = (1u << 5), /**< this function has extended info block */
  CBC_CODE_FLAGS_HAS_TAGGED_LITERALS = (1u << 6), /**< this function has tagged template literal list */
  CBC_CODE_FLAGS_HAS_LINE_INFO = (1u << 7), /**< this function has line info block */
  CBC_CODE_FLAGS_STATIC_FUNCTION = (1u << 8), /**< this function is a static snapshot function */
  CBC_CODE_FLAGS_DEBUGGER_IGNORE = (1u << 9), /**< this function should be ignored by debugger */
  CBC_CODE_FLAGS_LEXICAL_BLOCK_NEEDED = (1u << 10), /**< compiled code needs a lexical block */

  /* Bits from bit 12 is reserved for function types (see CBC_FUNCTION_TYPE_SHIFT).
   * Note: the last bits are used for type flags because < and >= operators can be used to
           check a range of types without decoding the actual type. */
} cbc_code_flags_t;

/**
 * Optional byte code fields. These fields are stored in a reversed
 * order from the end of the byte code data.
 *
 * Value fields:
 *   - when CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED is set:
 *     argument_end number of argument names encoded as strings
 *   - when function type is not CBC_FUNCTION_CONSTRUCTOR:
 *     function name encoded as string
 *   - when CBC_CODE_FLAGS_HAS_TAGGED_LITERALS is set:
 *     pointer to the tagged template collection encoded as value
 *
 * Byte fields when CBC_CODE_FLAGS_HAS_EXTENDED_INFO is set:
 *   - always available:
 *     a byte which contains a combination of CBC_EXTENDED_CODE_FLAGS bits
 *   - when CBC_EXTENDED_CODE_FLAGS_HAS_ARGUMENT_LENGTH is set:
 *     a vlq encoded default value for function length
 *   - when CBC_EXTENDED_CODE_FLAGS_HAS_SOURCE_CODE_RANGE is set:
 *     a pair of vlq encoded values, representing the start and size of the range
 */

/**
 * Compact byte code function types.
 */
typedef enum
{
  /* The first type must be regular expression (see CBC_IS_FUNCTION) */
  CBC_REGULAR_EXPRESSION, /**< regular expression literal */
  CBC_FUNCTION_NORMAL, /**< function without special properties */
  CBC_FUNCTION_CONSTRUCTOR, /**< constructor function */

  /* The following functions cannot be constructed (see CBC_FUNCTION_IS_CONSTRUCTABLE) */
  CBC_FUNCTION_SCRIPT, /**< script (global) function */
  CBC_FUNCTION_GENERATOR, /**< generator function */
  CBC_FUNCTION_ASYNC_GENERATOR, /**< async generator function */

  /* The following functions has no prototype (see CBC_FUNCTION_HAS_PROTOTYPE) */
  CBC_FUNCTION_ACCESSOR, /**< property accessor function */
  CBC_FUNCTION_ASYNC, /**< async function */
  CBC_FUNCTION_METHOD, /**< method */

  /* The following functions are arrow function (see CBC_FUNCTION_IS_ARROW) */
  CBC_FUNCTION_ARROW, /**< arrow function */
  CBC_FUNCTION_ASYNC_ARROW, /**< arrow function */
} cbc_code_function_types_t;

/**
 * Shift for getting / setting the function type of a byte code.
 */
#define CBC_FUNCTION_TYPE_SHIFT 12

/**
 * Compute function type bits in code flags.
 */
#define CBC_FUNCTION_TO_TYPE_BITS(name) ((name) << CBC_FUNCTION_TYPE_SHIFT)

/**
 * Get function type from code flags.
 */
#define CBC_FUNCTION_GET_TYPE(flags) ((uint16_t) ((flags) >> CBC_FUNCTION_TYPE_SHIFT))

/**
 * Checks whether the byte code is a function or a regular expression.
 */
#define CBC_IS_FUNCTION(flags) ((flags) >= (CBC_FUNCTION_NORMAL << CBC_FUNCTION_TYPE_SHIFT))

/**
 * Checks whether the function can be constructed with new operator.
 */
#define CBC_FUNCTION_IS_CONSTRUCTABLE(flags) ((flags) < (CBC_FUNCTION_GENERATOR << CBC_FUNCTION_TYPE_SHIFT))

/**
 * Checks whether the function has prototype property.
 */
#define CBC_FUNCTION_HAS_PROTOTYPE(flags) ((flags) < (CBC_FUNCTION_ACCESSOR << CBC_FUNCTION_TYPE_SHIFT))

/**
 * Checks whether the function is an arrow function.
 */
#define CBC_FUNCTION_IS_ARROW(flags) ((flags) >= (CBC_FUNCTION_ARROW << CBC_FUNCTION_TYPE_SHIFT))

/**
 * Compact byte code extended status flags.
 */
typedef enum
{
  CBC_EXTENDED_CODE_FLAGS_HAS_ARGUMENT_LENGTH = (1u << 0), /**< has argument length */
  CBC_EXTENDED_CODE_FLAGS_HAS_SOURCE_CODE_RANGE = (1u << 1), /**< has source code range (start, end) */
  CBC_EXTENDED_CODE_FLAGS_SOURCE_CODE_IN_ARGUMENTS = (1u << 2), /**< source code range is inside
                                                                 *   the function arguments */
} cbc_extended_code_flags_t;

/**
 * Shared script data.
 */
typedef enum
{
  CBC_SCRIPT_HAS_USER_VALUE = (1 << 0), /**< script has user value */
  CBC_SCRIPT_USER_VALUE_IS_OBJECT = (1 << 1), /**< user value is object */
  CBC_SCRIPT_HAS_FUNCTION_ARGUMENTS = (1 << 2), /**< script is a function with arguments source code */
  CBC_SCRIPT_HAS_IMPORT_META = (1 << 3), /**< script is a module with import.meta object */
  CBC_SCRIPT_IS_EVAL_CODE = (1 << 4), /**< script is compiled by eval like (eval, new Function, etc.) expression */
} cbc_script_type;

/**
 * Value for increasing or decreasing the script reference counter.
 */
#define CBC_SCRIPT_REF_ONE 0x20

/**
 * Maximum value of script reference counter.
 */
#define CBC_SCRIPT_REF_MAX (UINT32_MAX - CBC_SCRIPT_REF_ONE + 1)

/**
 * Sets the type of a script using the user_value.
 */
#define CBC_SCRIPT_SET_TYPE(script_p, user_value, ref_count)          \
  do                                                                  \
  {                                                                   \
    (script_p)->refs_and_type = (ref_count);                          \
    if ((user_value) != ECMA_VALUE_EMPTY)                             \
    {                                                                 \
      (script_p)->refs_and_type |= CBC_SCRIPT_HAS_USER_VALUE;         \
      if (ecma_is_value_object (user_value))                          \
      {                                                               \
        (script_p)->refs_and_type |= CBC_SCRIPT_USER_VALUE_IS_OBJECT; \
      }                                                               \
    }                                                                 \
  } while (false)

/**
 * Shared script data.
 */
typedef struct
{
#if JERRY_BUILTIN_REALMS
  ecma_object_t *realm_p; /**< realm object */
#endif /* JERRY_BUILTIN_REALMS */
  uint32_t refs_and_type; /**< reference counter and type of the function */
#if JERRY_SOURCE_NAME
  ecma_value_t source_name; /**< source name */
#endif /* JERRY_SOURCE_NAME */
} cbc_script_t;

/**
 * Get the array of optional values assigned to a script.
 *
 * First value: user value
 * Second value: function arguments value
 */
#define CBC_SCRIPT_GET_OPTIONAL_VALUES(script_p) ((ecma_value_t *) ((script_p) + 1))

/**
 * Get user value.
 */
#define CBC_SCRIPT_GET_USER_VALUE(script_p) (CBC_SCRIPT_GET_OPTIONAL_VALUES (script_p)[0])

/**
 * Get function arguments.
 */
#define CBC_SCRIPT_GET_FUNCTION_ARGUMENTS(script_p, type) \
  (CBC_SCRIPT_GET_OPTIONAL_VALUES (script_p)[((type) &CBC_SCRIPT_HAS_USER_VALUE) ? 1 : 0])

/**
 * Get import.meta object.
 */
#define CBC_SCRIPT_GET_IMPORT_META(script_p, type) \
  (CBC_SCRIPT_GET_OPTIONAL_VALUES (script_p)[((type) &CBC_SCRIPT_HAS_USER_VALUE) ? 1 : 0])

#define CBC_OPCODE(arg1, arg2, arg3, arg4) arg1,

/**
 * Opcode list.
 */
typedef enum
{
  CBC_OPCODE_LIST /**< list of opcodes */
} cbc_opcode_t;

/**
 * Extended opcode list.
 */
typedef enum
{
  CBC_EXT_OPCODE_LIST /**< list extended opcodes */
} cbc_ext_opcode_t;

#undef CBC_OPCODE

/**
 * Opcode flags.
 */
extern const uint8_t cbc_flags[];
extern const uint8_t cbc_ext_flags[];

#if JERRY_PARSER_DUMP_BYTE_CODE

/**
 * Opcode names for debugging.
 */
extern const char *const cbc_names[];
extern const char *const cbc_ext_names[];

#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

/**
 * @}
 * @}
 * @}
 */

#endif /* !BYTE_CODE_H */
