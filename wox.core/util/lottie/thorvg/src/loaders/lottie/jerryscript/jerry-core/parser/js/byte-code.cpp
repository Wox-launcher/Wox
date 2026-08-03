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

#include "js-parser-internal.h"

/* These two checks only checks the compiler, they have no effect on the code. */
JERRY_STATIC_ASSERT ((sizeof (cbc_uint8_arguments_t) == 16), sizeof_cbc_uint8_arguments_t_must_be_16_byte_long);

JERRY_STATIC_ASSERT ((sizeof (cbc_uint16_arguments_t) == 24), sizeof_cbc_uint16_arguments_t_must_be_24_byte_long);

JERRY_STATIC_ASSERT ((offsetof (cbc_uint8_arguments_t, script_value) == offsetof (cbc_uint16_arguments_t, script_value)),
                     script_value_in_cbc_uint8_arguments_and_cbc_uint16_arguments_must_be_in_the_same_offset);

/**
 * The reason of these two static asserts to notify the developer to increase the JERRY_SNAPSHOT_VERSION
 * whenever new bytecodes are introduced or existing ones have been deleted.
 */
JERRY_STATIC_ASSERT ((((int) CBC_END) == 238), number_of_cbc_opcodes_changed);
JERRY_STATIC_ASSERT ((((int) CBC_EXT_END) == 167), number_of_cbc_ext_opcodes_changed);

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_bytecode Bytecode
 * @{
 */

#if JERRY_PARSER || JERRY_PARSER_DUMP_BYTE_CODE

/**
 * Compact bytecode definition
 */
#define CBC_OPCODE(arg1, arg2, arg3, arg4) ((arg2) | (((arg3) + CBC_STACK_ADJUST_BASE) << CBC_STACK_ADJUST_SHIFT)),

/**
 * Flags of the opcodes.
 */
const uint8_t cbc_flags[] JERRY_ATTR_CONST_DATA = { CBC_OPCODE_LIST };

/**
 * Flags of the extended opcodes.
 */
const uint8_t cbc_ext_flags[] = { CBC_EXT_OPCODE_LIST };

#undef CBC_OPCODE

#endif /* JERRY_PARSER || JERRY_PARSER_DUMP_BYTE_CODE */

#if JERRY_PARSER_DUMP_BYTE_CODE

#define CBC_OPCODE(arg1, arg2, arg3, arg4) #arg1,

/**
 * Names of the opcodes.
 */
const char* const cbc_names[] = { CBC_OPCODE_LIST };

/**
 * Names of the extended opcodes.
 */
const char* const cbc_ext_names[] = { CBC_EXT_OPCODE_LIST };

#undef CBC_OPCODE

#endif /* JERRY_PARSER_DUMP_BYTE_CODE */

/**
 * @}
 * @}
 * @}
 */
