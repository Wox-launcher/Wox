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

#ifndef JS_PARSER_LIMITS_H
#define JS_PARSER_LIMITS_H

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_internals Internals
 * @{
 */

/**
 * Maximum identifier length accepted by the parser.
 * Limit: LEXER_MAX_STRING_LENGTH.
 */
#ifndef PARSER_MAXIMUM_IDENT_LENGTH
#define PARSER_MAXIMUM_IDENT_LENGTH 255
#endif /* !PARSER_MAXIMUM_IDENT_LENGTH */

/**
 * Maximum string limit.
 * Limit: 2147483647 / 65535.
 */
#define PARSER_MAXIMUM_STRING_LIMIT 65535

/**
 * Maximum string length.
 * Limit: PARSER_MAXIMUM_STRING_LIMIT.
 */
#ifndef PARSER_MAXIMUM_STRING_LENGTH
#define PARSER_MAXIMUM_STRING_LENGTH PARSER_MAXIMUM_STRING_LIMIT
#endif /* !PARSER_MAXIMUM_STRING_LENGTH */

/**
 * Maximum number of registers.
 * Limit: min: 256, max: min(PARSER_MAXIMUM_NUMBER_OF_LITERALS / 2, 16383)
 */
#ifndef PARSER_MAXIMUM_NUMBER_OF_REGISTERS
#define PARSER_MAXIMUM_NUMBER_OF_REGISTERS 256
#endif /* !PARSER_MAXIMUM_NUMBER_OF_REGISTERS */

/**
 * Maximum number of literals.
 * Limit: 32767 - PARSER_MAXIMUM_NUMBER_OF_REGISTERS. Recommended: 32767 - PARSER_MAXIMUM_NUMBER_OF_REGISTERS.
 */
#ifndef PARSER_MAXIMUM_NUMBER_OF_LITERALS
#define PARSER_MAXIMUM_NUMBER_OF_LITERALS (32767 - PARSER_MAXIMUM_NUMBER_OF_REGISTERS)
#endif /* !PARSER_MAXIMUM_NUMBER_OF_LITERALS */

/**
 * Maximum depth of scope stack.
 * Limit: 32767. Recommended: 32767
 */
#ifndef PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK
#define PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK 32767
#endif /* !PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK */

/**
 * Maximum code size.
 * Limit: 16777215. Recommended: 65535, 16777215.
 */
#ifndef PARSER_MAXIMUM_CODE_SIZE
#define PARSER_MAXIMUM_CODE_SIZE (65535 << (JMEM_ALIGNMENT_LOG))
#endif /* !PARSER_MAXIMUM_CODE_SIZE */

/**
 * Maximum number of values pushed onto the stack by a function.
 * Limit: 65500. Recommended: 1024.
 */
#ifndef PARSER_MAXIMUM_STACK_LIMIT
#define PARSER_MAXIMUM_STACK_LIMIT 1024

#endif /* !PARSER_MAXIMUM_STACK_LIMIT */

/* Checks. */

#if (PARSER_MAXIMUM_STRING_LENGTH < 1) || (PARSER_MAXIMUM_STRING_LENGTH > PARSER_MAXIMUM_STRING_LIMIT)
#error "Maximum string length is not within range."
#endif /* (PARSER_MAXIMUM_STRING_LENGTH < 1) || (PARSER_MAXIMUM_STRING_LENGTH > PARSER_MAXIMUM_STRING_LIMIT) */

#if (PARSER_MAXIMUM_IDENT_LENGTH < 1) || (PARSER_MAXIMUM_IDENT_LENGTH > PARSER_MAXIMUM_STRING_LENGTH)
#error "Maximum identifier length is not within range."
#endif /* (PARSER_MAXIMUM_IDENT_LENGTH < 1) || (PARSER_MAXIMUM_IDENT_LENGTH > PARSER_MAXIMUM_STRING_LENGTH) */

#if ((PARSER_MAXIMUM_NUMBER_OF_LITERALS < 1) \
     || (PARSER_MAXIMUM_NUMBER_OF_LITERALS + PARSER_MAXIMUM_NUMBER_OF_REGISTERS > 32767))
#error "Maximum number of literals is not within range."
#endif /* ((PARSER_MAXIMUM_NUMBER_OF_LITERALS < 1) \
           || (PARSER_MAXIMUM_NUMBER_OF_LITERALS > 32767)) */

#if (PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK < 1) || (PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK > 32767)
#error "Maximum depth of scope stack is not within range."
#endif /* (PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK < 1) || (PARSER_MAXIMUM_DEPTH_OF_SCOPE_STACK > 32767) */

#if ((PARSER_MAXIMUM_NUMBER_OF_REGISTERS * 2) > PARSER_MAXIMUM_NUMBER_OF_LITERALS)
#error "Maximum number of registers is not within range."
#endif /* ((PARSER_MAXIMUM_NUMBER_OF_REGISTERS * 2) > PARSER_MAXIMUM_NUMBER_OF_LITERALS) */

#if (PARSER_MAXIMUM_CODE_SIZE < 4096) || (PARSER_MAXIMUM_CODE_SIZE > 16777215)
#error "Maximum code size is not within range."
#endif /* (PARSER_MAXIMUM_CODE_SIZE < 4096) || (PARSER_MAXIMUM_CODE_SIZE > 16777215) */

#if (PARSER_MAXIMUM_STACK_LIMIT < 16) || (PARSER_MAXIMUM_STACK_LIMIT > 65500)
#error "Maximum function stack usage is not within range."
#endif /* (PARSER_MAXIMUM_STACK_LIMIT < 16) || (PARSER_MAXIMUM_STACK_LIMIT > 65500) */

/**
 * @}
 * @}
 * @}
 */

#endif /* !JS_PARSER_LIMITS_H */
