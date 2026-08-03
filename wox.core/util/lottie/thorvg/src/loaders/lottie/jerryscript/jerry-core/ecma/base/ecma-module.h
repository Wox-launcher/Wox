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

#ifndef ECMA_MODULE_H
#define ECMA_MODULE_H

#include "ecma-globals.h"

#include "common.h"

#if JERRY_MODULE_SYSTEM

#define ECMA_MODULE_MAX_PATH 255u

/**
 * Module status flags.
 */
typedef enum
{
  ECMA_MODULE_IS_NATIVE = (1 << 0), /**< native module */
  ECMA_MODULE_HAS_NAMESPACE = (1 << 1), /**< namespace object has been initialized */
} ecma_module_flags_t;

/**
 * Imported or exported names, such as "a as b"
 * Note: See https://www.ecma-international.org/ecma-262/6.0/#table-39
 *       and https://www.ecma-international.org/ecma-262/6.0/#table-41
 */
typedef struct ecma_module_names
{
  struct ecma_module_names *next_p; /**< next linked list node */
  ecma_string_t *imex_name_p; /**< Import/export name of the item */
  ecma_string_t *local_name_p; /**< Local name of the item */
} ecma_module_names_t;

/**
 * Module structure storing an instance of a module
 *
 * Note:
 *      The imports_p list follows the order of import-from/export-from statements in the source
 *      code of a module, even if a given module specifier is only used by export-from statements.
 */
typedef struct ecma_module
{
  /* Note: state is stored in header.u.class_prop.extra_info */
  ecma_extended_object_t header; /**< header part */
  /* TODO(dbatyai): These could be compressed pointers */
  ecma_object_t *scope_p; /**< lexical environment of the module */
  ecma_object_t *namespace_object_p; /**< namespace object of the module */
  struct ecma_module_node *imports_p; /**< import requests of the module */
  ecma_module_names_t *local_exports_p; /**< local exports of the module */
  struct ecma_module_node *indirect_exports_p; /**< indirect exports of the module */
  struct ecma_module_node *star_exports_p; /**< star exports of the module */

  /* Code used for evaluating a module */
  union
  {
    ecma_compiled_code_t *compiled_code_p; /**< compiled code for the module */
    jerry_native_module_evaluate_cb_t callback; /**< callback for evaluating native modules */
  } u;
} ecma_module_t;

/**
 * Module node to store imports / exports.
 *
 * Note:
 *      Only one module node is created for each module specifier: the names are
 *      concatenated if the same specifier is used multiple times in the source code.
 *      However, multiple nodes are created for modules with multiple alias
 *      (for example ./a.mjs and ././a.mjs can refer to the same module).
 */
typedef struct ecma_module_node
{
  struct ecma_module_node *next_p; /**< next linked list node */
  ecma_module_names_t *module_names_p; /**< names of the requested import/export node */

  union
  {
    ecma_value_t path_or_module; /**< imports: module specifier (if string) or module reference (if object) */
    ecma_value_t *module_object_p; /**< non-imports: reference to a path_or_module field in the imports */
  } u;
} ecma_module_node_t;

/**
 *  A list of module records that can be used to identify circular imports during resolution
 */
typedef struct ecma_module_resolve_set
{
  struct ecma_module_resolve_set *next_p; /**< next in linked list */
  ecma_module_t *module_p; /**< module */
  ecma_string_t *name_p; /**< identifier name */
} ecma_module_resolve_set_t;

/**
 * A list that is used like a stack to drive the resolution process, instead of recursion.
 */
typedef struct ecma_module_resolve_stack
{
  struct ecma_module_resolve_stack *next_p; /**< next in linked list */
  ecma_module_t *module_p; /**< module request */
  ecma_string_t *export_name_p; /**< export identifier name */
  bool resolving; /**< flag storing whether the current frame started resolving */
} ecma_module_resolve_stack_t;

ecma_value_t ecma_module_initialize (ecma_module_t *module_p);
ecma_module_t *ecma_module_get_resolved_module (ecma_value_t module_val);

ecma_value_t ecma_module_link (ecma_module_t *module_p, jerry_module_resolve_cb_t callback_p, void *user_p);
ecma_value_t ecma_module_evaluate (ecma_module_t *module_p);
ecma_value_t ecma_module_import (ecma_value_t specifier, ecma_value_t user_value);

ecma_module_t *ecma_module_create (void);
void ecma_module_cleanup_context (void);

void ecma_module_release_module_names (ecma_module_names_t *module_name_p);
void ecma_module_release_module (ecma_module_t *module_p);

#endif /* JERRY_MODULE_SYSTEM */

#endif /* !ECMA_MODULE_H */
