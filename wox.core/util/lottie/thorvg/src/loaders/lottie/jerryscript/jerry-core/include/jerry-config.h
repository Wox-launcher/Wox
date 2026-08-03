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

#ifndef JERRYSCRIPT_CONFIG_H
#define JERRYSCRIPT_CONFIG_H

// @JERRY_BUILD_CFG@

/**
 * Built-in configurations
 *
 * Allowed values for built-in defines:
 *  0: Disable the given built-in.
 *  1: Enable the given built-in.
 */
/*
 * By default all built-ins are enabled if they are not defined.
 */
#ifndef JERRY_BUILTINS
#define JERRY_BUILTINS 1
#endif /* !defined (JERRY_BUILTINS) */

#ifndef JERRY_BUILTIN_ARRAY
#define JERRY_BUILTIN_ARRAY JERRY_BUILTINS
#endif /* !defined (JERRY_BUILTIN_ARRAY) */

#ifndef JERRY_BUILTIN_BOOLEAN
#define JERRY_BUILTIN_BOOLEAN JERRY_BUILTINS
#endif /* !defined (JERRY_BUILTIN_BOOLEAN) */

#ifndef JERRY_BUILTIN_MATH
#define JERRY_BUILTIN_MATH JERRY_BUILTINS
#endif /* !defined (JERRY_BUILTIN_MATH) */

#ifndef JERRY_BUILTIN_NUMBER
#define JERRY_BUILTIN_NUMBER JERRY_BUILTINS
#endif /* !defined (JERRY_BUILTIN_NUMBER) */

#ifndef JERRY_BUILTIN_STRING
#define JERRY_BUILTIN_STRING JERRY_BUILTINS
#endif /* !defined (JERRY_BUILTIN_STRING) */

#ifndef JERRY_BUILTIN_CONTAINER
#define JERRY_BUILTIN_CONTAINER JERRY_BUILTINS
#endif /* !defined (JERRY_BUILTIN_CONTAINER) */

#ifndef JERRY_BUILTIN_TYPEDARRAY
#define JERRY_BUILTIN_TYPEDARRAY JERRY_BUILTINS
#endif /* !defined (JERRY_BUILTIN_TYPEDARRAY) */

/**
 * Engine internal and misc configurations.
 */

/**
 * Enable/Disable built-in error messages for error objects.
 *
 * Allowed values:
 *  0: Disable error messages.
 *  1: Enable error message.
 *
 * Default value: 0
 */
#ifndef JERRY_ERROR_MESSAGES
#define JERRY_ERROR_MESSAGES 0
#endif /* !defined (JERRY_ERROR_MESSAGES) */

/**
 * Enable/Disable external context.
 *
 * Allowed values:
 *  0: Disable external context.
 *  1: Enable external context support.
 *
 * Default value: 0
 */
#ifndef JERRY_EXTERNAL_CONTEXT
#define JERRY_EXTERNAL_CONTEXT 0
#endif /* !defined (JERRY_EXTERNAL_CONTEXT) */

/**
 * Maximum size of heap in kilobytes
 *
 * Default value: 512 KiB
 */
#ifndef JERRY_GLOBAL_HEAP_SIZE
#define JERRY_GLOBAL_HEAP_SIZE (512)
#endif /* !defined (JERRY_GLOBAL_HEAP_SIZE) */

/**
 * The allowed heap usage limit until next garbage collection, in bytes.
 *
 * If value is 0, the default is 1/32 of JERRY_HEAP_SIZE
 */
#ifndef JERRY_GC_LIMIT
#define JERRY_GC_LIMIT 0
#endif /* !defined (JERRY_GC_LIMIT) */

/**
 * Maximum stack usage size in kilobytes
 *
 * Note: This feature cannot be used when 'detect_stack_use_after_return=1' ASAN option is enabled.
 * For more detailed description:
 *   - https://github.com/google/sanitizers/wiki/AddressSanitizerUseAfterReturn#compatibility
 *
 * Default value: 0, unlimited
 */
#ifndef JERRY_STACK_LIMIT
#define JERRY_STACK_LIMIT (0)
#endif /* !defined (JERRY_STACK_LIMIT) */

/**
 * Maximum depth of recursion during GC mark phase
 *
 * Default value: 8
 */
#ifndef JERRY_GC_MARK_LIMIT
#define JERRY_GC_MARK_LIMIT (8)
#endif /* !defined (JERRY_GC_MARK_LIMIT) */

/**
 * Enable/Disable property lookup cache.
 *
 * Allowed values:
 *  0: Disable lookup cache.
 *  1: Enable lookup cache.
 *
 * Default value: 1
 */
#ifndef JERRY_LCACHE
#define JERRY_LCACHE 1
#endif /* !defined (JERRY_LCACHE) */

/**
 * Enable/Disable the JavaScript parser.
 *
 * Allowed values:
 *  0: Disable the JavaScript parser and all related functionality.
 *  1: Enable the JavaScript parser.
 *
 * Default value: 1
 */
#ifndef JERRY_PARSER
#define JERRY_PARSER 1
#endif /* !defined (JERRY_PARSER) */

/**
 * Enable/Disable JerryScript byte code dump functions during parsing.
 * To dump the JerryScript byte code the engine must be initialized with opcodes
 * display flag. This option does not influence RegExp byte code dumps.
 *
 * Allowed values:
 *  0: Disable all bytecode dump functions.
 *  1: Enable bytecode dump functions.
 *
 * Default value: 0
 */
#ifndef JERRY_PARSER_DUMP_BYTE_CODE
#define JERRY_PARSER_DUMP_BYTE_CODE 0
#endif /* defined (JERRY_PARSER_DUMP_BYTE_CODE) */

/**
 * Enable/Disable ECMA property hashmap.
 *
 * Allowed values:
 *  0: Disable property hashmap.
 *  1: Enable property hashmap.
 *
 * Default value: 1
 */
#ifndef JERRY_PROPERTY_HASHMAP
#define JERRY_PROPERTY_HASHMAP 0
#endif /* !defined (JERRY_PROPERTY_HASHMAP) */

/**
 * Enable/Disable byte code dump functions for RegExp objects.
 * To dump the RegExp byte code the engine must be initialized with
 * regexp opcodes display flag. This option does not influence the
 * JerryScript byte code dumps.
 *
 * Allowed values:
 *  0: Disable all bytecode dump functions.
 *  1: Enable bytecode dump functions.
 *
 * Default value: 0
 */
#ifndef JERRY_REGEXP_DUMP_BYTE_CODE
#define JERRY_REGEXP_DUMP_BYTE_CODE 0
#endif /* !defined (JERRY_REGEXP_DUMP_BYTE_CODE) */

/**
 * Enable/Disable the snapshot execution functions.
 *
 * Allowed values:
 *  0: Disable snapshot execution.
 *  1: Enable snapshot execution.
 *
 * Default value: 0
 */
#ifndef JERRY_SNAPSHOT_EXEC
#define JERRY_SNAPSHOT_EXEC 0
#endif /* !defined (JERRY_SNAPSHOT_EXEC) */

/**
 * Enable/Disable the snapshot save functions.
 *
 * Allowed values:
 *  0: Disable snapshot save functions.
 *  1: Enable snapshot save functions.
 */
#ifndef JERRY_SNAPSHOT_SAVE
#define JERRY_SNAPSHOT_SAVE 0
#endif /* !defined (JERRY_SNAPSHOT_SAVE) */

/**
 * Enable/Disable usage of system allocator.
 *
 * Allowed values:
 *  0: Disable usage of system allocator.
 *  1: Enable usage of system allocator.
 *
 * Default value: 0
 */
#ifndef JERRY_SYSTEM_ALLOCATOR
#define JERRY_SYSTEM_ALLOCATOR 0
#endif /* !defined (JERRY_SYSTEM_ALLOCATOR) */

/**
 * Enables/disables the unicode case conversion in the engine.
 * By default Unicode case conversion is enabled.
 */
#ifndef JERRY_UNICODE_CASE_CONVERSION
#define JERRY_UNICODE_CASE_CONVERSION 0
#endif /* !defined (JERRY_UNICODE_CASE_CONVERSION) */

/**
 * Enable/Disable the vm execution stop callback function.
 *
 * Allowed values:
 *  0: Disable vm exec stop callback support.
 *  1: Enable vm exec stop callback support.
 */
#ifndef JERRY_VM_HALT
#define JERRY_VM_HALT 0
#endif /* !defined (JERRY_VM_HALT) */

/**
 * Enable/Disable the vm throw callback function.
 *
 * Allowed values:
 *  0: Disable vm throw callback support.
 *  1: Enable vm throw callback support.
 */
#ifndef JERRY_VM_THROW
#define JERRY_VM_THROW 0
#endif /* !defined (JERRY_VM_THROW) */

/**
 * Advanced section configurations.
 */

/**
 * Allow configuring attributes on a few constant data inside the engine.
 *
 * One of the main usages:
 * Normally compilers store const(ant)s in ROM. Thus saving RAM.
 * But if your compiler does not support it then the directive below can force it.
 *
 * For the moment it is mainly meant for the following targets:
 *      - ESP8266
 *
 * Example configuration for moving (some) constants into a given section:
 *  # define JERRY_ATTR_CONST_DATA __attribute__((section(".rodata.const")))
 */
#ifndef JERRY_ATTR_CONST_DATA
#define JERRY_ATTR_CONST_DATA
#endif /* !defined (JERRY_ATTR_CONST_DATA) */

/**
 * The JERRY_ATTR_GLOBAL_HEAP allows adding extra attributes for the Jerry global heap.
 *
 * Example on how to move the global heap into its own section:
 *   #define JERRY_ATTR_GLOBAL_HEAP __attribute__((section(".text.globalheap")))
 */
#ifndef JERRY_ATTR_GLOBAL_HEAP
#define JERRY_ATTR_GLOBAL_HEAP
#endif /* !defined (JERRY_ATTR_GLOBAL_HEAP) */

/**
 * Source name related types into a single guard
 */
#if JERRY_ERROR_MESSAGES
#define JERRY_SOURCE_NAME 1
#else /* !(JERRY_ERROR_MESSAGES) */
#define JERRY_SOURCE_NAME 0
#endif /* JERRY_ERROR_MESSAGES */

#endif /* !JERRYSCRIPT_CONFIG_H */
