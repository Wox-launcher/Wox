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

#include "ecma-module.h"

#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-lex-env.h"
#include "ecma-objects.h"

#include "jcontext.h"
#include "lit-char-helpers.h"
#include "vm.h"

#if JERRY_MODULE_SYSTEM

/**
 * Type of the result returned by ecma_module_resolve_export.
 */
typedef enum
{
  ECMA_MODULE_RESOLVE_NOT_FOUND, /**< reference not found */
  ECMA_MODULE_RESOLVE_CIRCULAR, /**< only circular references are found */
  ECMA_MODULE_RESOLVE_ERROR, /**< module in error state is encountered */
  ECMA_MODULE_RESOLVE_AMBIGUOUS, /**< reference is ambiguous */
  ECMA_MODULE_RESOLVE_FOUND, /**< reference found */
} ecma_module_resolve_result_type_t;

/**
 *  A record that stores the result of ecma_module_resolve_export.
 */
typedef struct
{
  ecma_module_resolve_result_type_t result_type; /**< result type */
  ecma_value_t result; /**< result value */
} ecma_module_resolve_result_t;

/**
 * This flag is set in the result if the value is a namespace object.
 */
#define ECMA_MODULE_NAMESPACE_RESULT_FLAG 0x2

/**
 * Initialize context variables for the root module.
 *
 * @return new module
 */
ecma_module_t *
ecma_module_create (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (JERRY_CONTEXT (module_current_p) == NULL);

  ecma_object_t *obj_p = ecma_create_object (NULL, sizeof (ecma_module_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;
  ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_MODULE;
  ext_object_p->u.cls.u1.module_state = JERRY_MODULE_STATE_UNLINKED;
  ext_object_p->u.cls.u2.module_flags = 0;

  ecma_module_t *module_p = (ecma_module_t *) obj_p;

  module_p->scope_p = NULL;
  module_p->namespace_object_p = NULL;
  module_p->imports_p = NULL;
  module_p->local_exports_p = NULL;
  module_p->indirect_exports_p = NULL;
  module_p->star_exports_p = NULL;
  module_p->u.compiled_code_p = NULL;

  return module_p;
} /* ecma_module_create */

/**
 * Cleanup context variables for the root module.
 */
void
ecma_module_cleanup_context (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_deref_object ((ecma_object_t *) JERRY_CONTEXT (module_current_p));
#ifndef JERRY_NDEBUG
  JERRY_CONTEXT (module_current_p) = NULL;
#endif /* JERRY_NDEBUG */
} /* ecma_module_cleanup_context */

/**
 * Sets module state to error.
 */
static void
ecma_module_set_error_state (ecma_module_t *module_p) /**< module */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  module_p->header.u.cls.u1.module_state = JERRY_MODULE_STATE_ERROR;

  if (JERRY_CONTEXT (module_state_changed_callback_p) != NULL && !jcontext_has_pending_abort ())
  {
    jerry_value_t exception = jcontext_take_exception ();

    JERRY_CONTEXT (module_state_changed_callback_p)
    (JERRY_MODULE_STATE_ERROR,
     ecma_make_object_value (&module_p->header.object),
     exception,
     JERRY_CONTEXT (module_state_changed_callback_user_p));
    jcontext_raise_exception (exception);
  }
} /* ecma_module_set_error_state */

/**
 * Gets the internal module pointer of a module
 *
 * @return module pointer
 */
static inline ecma_module_t *
ecma_module_get_from_object (ecma_value_t module_val) /**< module */
{
  JERRY_ASSERT (ecma_is_value_object (module_val));

  ecma_object_t *object_p = ecma_get_object_from_value (module_val);

  JERRY_ASSERT (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_MODULE));

  return (ecma_module_t *) object_p;
} /* ecma_module_get_from_object */

/**
 * Cleans up a list of module names.
 */
void
ecma_module_release_module_names (ecma_module_names_t *module_name_p) /**< first module name */
{
  while (module_name_p != NULL)
  {
    ecma_module_names_t *next_p = module_name_p->next_p;

    ecma_deref_ecma_string (module_name_p->imex_name_p);
    ecma_deref_ecma_string (module_name_p->local_name_p);
    jmem_heap_free_block (module_name_p, sizeof (ecma_module_names_t));

    module_name_p = next_p;
  }
} /* ecma_module_release_module_names */

/**
 * Cleans up a list of module nodes.
 */
static void
ecma_module_release_module_nodes (ecma_module_node_t *module_node_p, /**< first module node */
                                  bool is_import) /**< free path variable */
{
  while (module_node_p != NULL)
  {
    ecma_module_node_t *next_p = module_node_p->next_p;

    ecma_module_release_module_names (module_node_p->module_names_p);

    if (is_import && ecma_is_value_string (module_node_p->u.path_or_module))
    {
      ecma_deref_ecma_string (ecma_get_string_from_value (module_node_p->u.path_or_module));
    }

    jmem_heap_free_block (module_node_p, sizeof (ecma_module_node_t));
    module_node_p = next_p;
  }
} /* ecma_module_release_module_nodes */

/**
 *  Creates a new resolve set item from a {module, export_name} pair.
 *
 *  @return new resolve set item
 */
static ecma_module_resolve_set_t *
ecma_module_resolve_set_create (ecma_module_t *const module_p, /**< module */
                                ecma_string_t *const export_name_p) /**< export name */
{
  ecma_module_resolve_set_t *new_p;
  new_p = (ecma_module_resolve_set_t *) jmem_heap_alloc_block (sizeof (ecma_module_resolve_set_t));

  new_p->next_p = NULL;
  new_p->module_p = module_p;
  new_p->name_p = export_name_p;

  return new_p;
} /* ecma_module_resolve_set_create */

/**
 *  Appends a {module, export_name} record into a resolve set.
 *
 *  @return true - if the record is appended successfully
 *          false - otherwise
 */
static bool
ecma_module_resolve_set_append (ecma_module_resolve_set_t *set_p, /**< resolve set */
                                ecma_module_t *const module_p, /**< module */
                                ecma_string_t *const export_name_p) /**< export name */
{
  JERRY_ASSERT (set_p != NULL);
  ecma_module_resolve_set_t *current_p = set_p;

  while (true)
  {
    if (current_p->module_p == module_p && ecma_compare_ecma_strings (current_p->name_p, export_name_p))
    {
      return false;
    }

    ecma_module_resolve_set_t *next_p = current_p->next_p;

    if (next_p == NULL)
    {
      current_p->next_p = ecma_module_resolve_set_create (module_p, export_name_p);
      return true;
    }

    current_p = next_p;
  }
} /* ecma_module_resolve_set_append */

/**
 * Cleans up contents of a resolve set.
 */
static void
ecma_module_resolve_set_cleanup (ecma_module_resolve_set_t *set_p) /**< resolve set */
{
  while (set_p != NULL)
  {
    ecma_module_resolve_set_t *next_p = set_p->next_p;
    jmem_heap_free_block (set_p, sizeof (ecma_module_resolve_set_t));
    set_p = next_p;
  }
} /* ecma_module_resolve_set_cleanup */

/**
 * Throws the appropriate error based on the resolve result
 *
 * @return error value
 */
static ecma_value_t
ecma_module_resolve_throw (ecma_module_resolve_result_t *resolve_result_p, /**< resolve result */
                           ecma_string_t *name_p) /**< referenced value */
{
#if JERRY_ERROR_MESSAGES
  ecma_value_t name_val = ecma_make_string_value (name_p);
  const char *msg_p;

  switch (resolve_result_p->result_type)
  {
    case ECMA_MODULE_RESOLVE_CIRCULAR:
    {
      msg_p = "Detected cycle while resolving name '%' (module)";
      break;
    }
    case ECMA_MODULE_RESOLVE_AMBIGUOUS:
    {
      msg_p = "Name '%' is ambiguous (module)";
      break;
    }
    default:
    {
      JERRY_ASSERT (resolve_result_p->result_type == ECMA_MODULE_RESOLVE_NOT_FOUND
                    || resolve_result_p->result_type == ECMA_MODULE_RESOLVE_ERROR);

      msg_p = "Name '%' is not found (module)";
      break;
    }
  }

  return ecma_raise_standard_error_with_format (JERRY_ERROR_SYNTAX, msg_p, name_val);
#else /* JERRY_ERROR_MESSAGES */
  JERRY_UNUSED (resolve_result_p);
  JERRY_UNUSED (name_p);

  return ecma_raise_syntax_error (ECMA_ERR_EMPTY);
#endif /* !JERRY_ERROR_MESSAGES */
} /* ecma_module_resolve_throw */

/**
 * Updates the resolve record with the passed type/value pair
 *
 * @return true - if the record is updated successfully
 *         false - otherwise
 */
static bool
ecma_module_resolve_update (ecma_module_resolve_result_t *resolve_result_p, /**< [in,out] resolve result */
                            ecma_value_t result) /**< result value */
{
  JERRY_ASSERT (resolve_result_p->result_type != ECMA_MODULE_RESOLVE_AMBIGUOUS
                && resolve_result_p->result_type != ECMA_MODULE_RESOLVE_ERROR);

  if (resolve_result_p->result_type == ECMA_MODULE_RESOLVE_NOT_FOUND
      || resolve_result_p->result_type == ECMA_MODULE_RESOLVE_CIRCULAR)
  {
    resolve_result_p->result_type = ECMA_MODULE_RESOLVE_FOUND;
    resolve_result_p->result = result;
    return true;
  }

  JERRY_ASSERT (resolve_result_p->result_type == ECMA_MODULE_RESOLVE_FOUND);

  if (resolve_result_p->result == result)
  {
    return true;
  }

  resolve_result_p->result_type = ECMA_MODULE_RESOLVE_AMBIGUOUS;
  return false;
} /* ecma_module_resolve_update */

/**
 * Finds the reference in the imported bindings.
 *
 * Note:
 *     This function is needed because the namespace object is created before the imports are connected
 *
 * @return true - if the record is updated successfully
 *         false - otherwise
 */
static bool
ecma_module_resolve_import (ecma_module_resolve_result_t *resolve_result_p, /**< [in,out] resolve result */
                            ecma_module_resolve_set_t *resolve_set_p, /**< resolve set */
                            ecma_module_t *module_p, /**< base module */
                            ecma_string_t *local_name_p) /**< local name */
{
  ecma_module_node_t *import_node_p = module_p->imports_p;

  while (true)
  {
    JERRY_ASSERT (import_node_p != NULL);

    for (ecma_module_names_t *import_names_p = import_node_p->module_names_p; import_names_p != NULL;
         import_names_p = import_names_p->next_p)
    {
      if (ecma_compare_ecma_strings (local_name_p, import_names_p->local_name_p))
      {
        ecma_module_t *imported_module_p = ecma_module_get_from_object (import_node_p->u.path_or_module);

        if (ecma_compare_ecma_string_to_magic_id (import_names_p->imex_name_p, LIT_MAGIC_STRING_ASTERISK_CHAR))
        {
          /* Namespace import. */
          ecma_value_t ns = ecma_make_object_value (imported_module_p->namespace_object_p);

          JERRY_ASSERT (ns & ECMA_MODULE_NAMESPACE_RESULT_FLAG);

          return ecma_module_resolve_update (resolve_result_p, ns);
        }

        if (!ecma_module_resolve_set_append (resolve_set_p, imported_module_p, import_names_p->imex_name_p)
            && resolve_result_p->result_type == ECMA_MODULE_RESOLVE_NOT_FOUND)
        {
          resolve_result_p->result_type = ECMA_MODULE_RESOLVE_CIRCULAR;
        }

        return true;
      }
    }

    import_node_p = import_node_p->next_p;
  }
} /* ecma_module_resolve_import */

/**
 * Resolves which module satisfies an export based from a specific module in the import tree.
 *
 * Note: See ES11 15.2.1.17.3
 */
static void
ecma_module_resolve_export (ecma_module_t *const module_p, /**< base module */
                            ecma_string_t *const export_name_p, /**< export name */
                            ecma_module_resolve_result_t *resolve_result_p) /**< [out] resolve result */
{
  ecma_module_resolve_set_t *resolve_set_p = ecma_module_resolve_set_create (module_p, export_name_p);
  ecma_module_resolve_set_t *current_set_p = resolve_set_p;
  ecma_module_node_t *star_export_p;

  resolve_result_p->result_type = ECMA_MODULE_RESOLVE_NOT_FOUND;
  resolve_result_p->result = ECMA_VALUE_UNDEFINED;

  do
  {
    ecma_module_t *current_module_p = current_set_p->module_p;
    ecma_string_t *current_export_name_p = current_set_p->name_p;

    if (current_module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_ERROR)
    {
      resolve_result_p->result_type = ECMA_MODULE_RESOLVE_ERROR;
      goto exit;
    }

    if (current_module_p->header.u.cls.u2.module_flags & ECMA_MODULE_HAS_NAMESPACE)
    {
      ecma_property_t *property_p =
        ecma_find_named_property (current_module_p->namespace_object_p, current_export_name_p);

      if (property_p != NULL)
      {
        ecma_property_value_t *property_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

        JERRY_ASSERT (
          (!(*property_p & ECMA_PROPERTY_FLAG_DATA) && !(property_value_p->value & ECMA_MODULE_NAMESPACE_RESULT_FLAG))
          || ((*property_p & ECMA_PROPERTY_FLAG_DATA) && ecma_is_value_object (property_value_p->value)
              && ecma_object_class_is (ecma_get_object_from_value (property_value_p->value),
                                       ECMA_OBJECT_CLASS_MODULE_NAMESPACE)));

        if (!ecma_module_resolve_update (resolve_result_p, property_value_p->value))
        {
          goto exit;
        }

        goto next_iteration;
      }
    }
    else
    {
      /* 6. */
      ecma_module_names_t *export_names_p = current_module_p->local_exports_p;

      while (export_names_p != NULL)
      {
        if (ecma_compare_ecma_strings (current_export_name_p, export_names_p->imex_name_p))
        {
          ecma_property_t *property_p =
            ecma_find_named_property (current_module_p->scope_p, export_names_p->local_name_p);

          if (property_p != NULL)
          {
            ecma_value_t reference = ecma_property_to_reference (property_p);

            JERRY_ASSERT (!(reference & ECMA_MODULE_NAMESPACE_RESULT_FLAG));

            if (!ecma_module_resolve_update (resolve_result_p, reference))
            {
              goto exit;
            }
          }
          else if (!ecma_module_resolve_import (resolve_result_p,
                                                resolve_set_p,
                                                current_module_p,
                                                export_names_p->local_name_p))
          {
            goto exit;
          }

          goto next_iteration;
        }

        export_names_p = export_names_p->next_p;
      }

      /* 7. */
      ecma_module_node_t *indirect_export_p = current_module_p->indirect_exports_p;

      while (indirect_export_p != NULL)
      {
        export_names_p = indirect_export_p->module_names_p;

        while (export_names_p != NULL)
        {
          if (ecma_compare_ecma_strings (current_export_name_p, export_names_p->imex_name_p))
          {
            ecma_module_t *target_module_p = ecma_module_get_from_object (*indirect_export_p->u.module_object_p);

            if (ecma_compare_ecma_string_to_magic_id (export_names_p->local_name_p, LIT_MAGIC_STRING_ASTERISK_CHAR))
            {
              /* Namespace export. */
              ecma_value_t ns = ecma_make_object_value (target_module_p->namespace_object_p);

              JERRY_ASSERT (ns & ECMA_MODULE_NAMESPACE_RESULT_FLAG);

              if (!ecma_module_resolve_update (resolve_result_p, ns))
              {
                goto exit;
              }
            }
            else if (!ecma_module_resolve_set_append (resolve_set_p, target_module_p, export_names_p->local_name_p)
                     && resolve_result_p->result_type == ECMA_MODULE_RESOLVE_NOT_FOUND)
            {
              resolve_result_p->result_type = ECMA_MODULE_RESOLVE_CIRCULAR;
            }

            goto next_iteration;
          }

          export_names_p = export_names_p->next_p;
        }

        indirect_export_p = indirect_export_p->next_p;
      }
    }

    /* 8. */
    if (ecma_compare_ecma_string_to_magic_id (current_export_name_p, LIT_MAGIC_STRING_DEFAULT))
    {
      goto exit;
    }

    /* 10. */
    star_export_p = current_module_p->star_exports_p;
    while (star_export_p != NULL)
    {
      JERRY_ASSERT (star_export_p->module_names_p == NULL);

      ecma_module_t *target_module_p = ecma_module_get_from_object (*star_export_p->u.module_object_p);

      if (!ecma_module_resolve_set_append (resolve_set_p, target_module_p, current_export_name_p)
          && resolve_result_p->result_type == ECMA_MODULE_RESOLVE_NOT_FOUND)
      {
        resolve_result_p->result_type = ECMA_MODULE_RESOLVE_CIRCULAR;
      }

      star_export_p = star_export_p->next_p;
    }

next_iteration:
    current_set_p = current_set_p->next_p;
  } while (current_set_p != NULL);

exit:
  ecma_module_resolve_set_cleanup (resolve_set_p);
} /* ecma_module_resolve_export */

/**
 * Evaluates an EcmaScript module.
 *
 * @return ECMA_VALUE_ERROR - if an error occurred
 *         ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
ecma_module_evaluate (ecma_module_t *module_p) /**< module */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_ERROR)
  {
    return ecma_raise_range_error (ECMA_ERR_MODULE_IS_IN_ERROR_STATE);
  }

  if (module_p->header.u.cls.u1.module_state >= JERRY_MODULE_STATE_EVALUATING)
  {
    return ECMA_VALUE_EMPTY;
  }

  JERRY_ASSERT (module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_LINKED);
  JERRY_ASSERT (module_p->scope_p != NULL);

  module_p->header.u.cls.u1.module_state = JERRY_MODULE_STATE_EVALUATING;

  ecma_value_t ret_value;

  if (module_p->header.u.cls.u2.module_flags & ECMA_MODULE_IS_NATIVE)
  {
    ret_value = ECMA_VALUE_UNDEFINED;

    if (module_p->u.callback)
    {
      ret_value = module_p->u.callback (ecma_make_object_value (&module_p->header.object));

      if (JERRY_UNLIKELY (ecma_is_value_exception (ret_value)))
      {
        ecma_throw_exception (ret_value);
        ret_value = ECMA_VALUE_ERROR;
      }
    }
  }
  else
  {
    ret_value = vm_run_module (module_p);
  }

  if (JERRY_LIKELY (!ECMA_IS_VALUE_ERROR (ret_value)))
  {
    module_p->header.u.cls.u1.module_state = JERRY_MODULE_STATE_EVALUATED;

    if (JERRY_CONTEXT (module_state_changed_callback_p) != NULL)
    {
      JERRY_CONTEXT (module_state_changed_callback_p)
      (JERRY_MODULE_STATE_EVALUATED,
       ecma_make_object_value (&module_p->header.object),
       ret_value,
       JERRY_CONTEXT (module_state_changed_callback_user_p));
    }
  }
  else
  {
    ecma_module_set_error_state (module_p);
  }

  if (!(module_p->header.u.cls.u2.module_flags & ECMA_MODULE_IS_NATIVE))
  {
    ecma_bytecode_deref (module_p->u.compiled_code_p);
  }

  module_p->u.compiled_code_p = NULL;
  return ret_value;
} /* ecma_module_evaluate */

/**
 * Resolves an export and adds it to the modules namespace object, if the export name is not yet handled.
 * Note: See 15.2.1.16.2 and 15.2.1.18
 *
 * @return ECMA_VALUE_ERROR - if an error occurred
 *         ECMA_VALUE_EMPTY - otherwise
 */
static ecma_value_t
ecma_module_namespace_object_add_export_if_needed (ecma_collection_t *properties_p, /**< collection of properties */
                                                   ecma_module_t *module_p, /**< module */
                                                   ecma_string_t *export_name_p, /**< export name */
                                                   bool allow_default) /**< allow default export */
{
  if (!allow_default)
  {
    if (ecma_compare_ecma_string_to_magic_id (export_name_p, LIT_MAGIC_STRING_DEFAULT))
    {
      return ECMA_VALUE_EMPTY;
    }

    /* No need to check duplications before star exports are processed. */
    ecma_value_t *buffer_p = properties_p->buffer_p;
    ecma_value_t *buffer_end_p = properties_p->buffer_p + properties_p->item_count;

    while (buffer_p < buffer_end_p)
    {
      if (ecma_compare_ecma_strings (ecma_get_string_from_value (*buffer_p), export_name_p))
      {
        return ECMA_VALUE_EMPTY;
      }

      buffer_p += 2;
    }
  }

  ecma_module_resolve_result_t resolve_result;
  ecma_module_resolve_export (module_p, export_name_p, &resolve_result);

  if (resolve_result.result_type == ECMA_MODULE_RESOLVE_AMBIGUOUS)
  {
    return ECMA_VALUE_EMPTY;
  }

  if (resolve_result.result_type != ECMA_MODULE_RESOLVE_FOUND)
  {
    return ecma_module_resolve_throw (&resolve_result, export_name_p);
  }

  ecma_collection_push_back (properties_p, ecma_make_string_value (export_name_p));
  ecma_collection_push_back (properties_p, resolve_result.result);
  return ECMA_VALUE_EMPTY;
} /* ecma_module_namespace_object_add_export_if_needed */

/**
 * Helper routine for heapsort algorithm.
 */
static void
ecma_module_heap_sort_shift_down (ecma_value_t *buffer_p, /**< array of items */
                                  uint32_t item_count, /**< number of items */
                                  uint32_t item_index) /**< index of updated item */
{
  while (true)
  {
    uint32_t highest_index = item_index;
    uint32_t current_index = (item_index << 1) + 2;

    if (current_index >= item_count)
    {
      return;
    }

    if (ecma_compare_ecma_strings_relational (ecma_get_string_from_value (buffer_p[highest_index]),
                                              ecma_get_string_from_value (buffer_p[current_index])))
    {
      highest_index = current_index;
    }

    current_index += 2;

    if (current_index < item_count
        && ecma_compare_ecma_strings_relational (ecma_get_string_from_value (buffer_p[highest_index]),
                                                 ecma_get_string_from_value (buffer_p[current_index])))
    {
      highest_index = current_index;
    }

    if (highest_index == item_index)
    {
      return;
    }

    ecma_value_t tmp = buffer_p[highest_index];
    buffer_p[highest_index] = buffer_p[item_index];
    buffer_p[item_index] = tmp;

    tmp = buffer_p[highest_index + 1];
    buffer_p[highest_index + 1] = buffer_p[item_index + 1];
    buffer_p[item_index + 1] = tmp;

    item_index = highest_index;
  }
} /* ecma_module_heap_sort_shift_down */

/**
 * Creates a namespace object for a module.
 * Note: See 15.2.1.18
 *
 * @return ECMA_VALUE_ERROR - if an error occurred
 *         ECMA_VALUE_EMPTY - otherwise
 */
static ecma_value_t
ecma_module_create_namespace_object (ecma_module_t *module_p) /**< module */
{
  JERRY_ASSERT (module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_LINKING);
  JERRY_ASSERT (module_p->namespace_object_p != NULL);
  JERRY_ASSERT (!(module_p->header.u.cls.u2.module_flags & ECMA_MODULE_HAS_NAMESPACE));

  ecma_module_resolve_set_t *resolve_set_p;
  resolve_set_p = ecma_module_resolve_set_create (module_p, ecma_get_magic_string (LIT_MAGIC_STRING_ASTERISK_CHAR));

  /* The properties collection stores name / result item pairs. Name is always
   * a string, and result can be a property reference or namespace object. */
  ecma_module_resolve_set_t *current_set_p = resolve_set_p;
  ecma_collection_t *properties_p = ecma_new_collection ();
  ecma_value_t result = ECMA_VALUE_EMPTY;
  bool allow_default = true;

  ecma_value_t *buffer_p;
  uint32_t item_count;
  ecma_value_t *buffer_end_p;

  do
  {
    ecma_module_t *current_module_p = current_set_p->module_p;

    if (current_module_p->header.u.cls.u2.module_flags & ECMA_MODULE_HAS_NAMESPACE)
    {
      JERRY_ASSERT (!allow_default);

      jmem_cpointer_t prop_iter_cp = current_module_p->namespace_object_p->u1.property_list_cp;

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

        ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

        for (int i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
        {
          if (!ECMA_PROPERTY_IS_RAW (prop_iter_p->types[i]))
          {
            continue;
          }

          ecma_string_t *name_p = ecma_string_from_property_name (prop_iter_p->types[i], prop_pair_p->names_cp[i]);
          result = ecma_module_namespace_object_add_export_if_needed (properties_p, module_p, name_p, false);
          ecma_deref_ecma_string (name_p);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto exit;
          }
        }

        prop_iter_cp = prop_iter_p->next_property_cp;
      }
    }
    else
    {
      ecma_module_names_t *export_names_p = current_module_p->local_exports_p;

      if (export_names_p != NULL)
      {
        do
        {
          result = ecma_module_namespace_object_add_export_if_needed (properties_p,
                                                                      module_p,
                                                                      export_names_p->imex_name_p,
                                                                      allow_default);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto exit;
          }

          export_names_p = export_names_p->next_p;
        } while (export_names_p != NULL);
      }

      ecma_module_node_t *indirect_export_p = current_module_p->indirect_exports_p;

      while (indirect_export_p != NULL)
      {
        export_names_p = indirect_export_p->module_names_p;

        while (export_names_p != NULL)
        {
          result = ecma_module_namespace_object_add_export_if_needed (properties_p,
                                                                      module_p,
                                                                      export_names_p->imex_name_p,
                                                                      allow_default);

          if (ECMA_IS_VALUE_ERROR (result))
          {
            goto exit;
          }

          export_names_p = export_names_p->next_p;
        }

        indirect_export_p = indirect_export_p->next_p;
      }
    }

    allow_default = false;

    ecma_module_node_t *star_export_p = current_module_p->star_exports_p;

    while (star_export_p != NULL)
    {
      JERRY_ASSERT (star_export_p->module_names_p == NULL);

      /* Circular imports are ignored */
      ecma_module_resolve_set_append (resolve_set_p,
                                      ecma_module_get_from_object (*star_export_p->u.module_object_p),
                                      ecma_get_magic_string (LIT_MAGIC_STRING_ASTERISK_CHAR));

      star_export_p = star_export_p->next_p;
    }

    current_set_p = current_set_p->next_p;
  } while (current_set_p != NULL);

  buffer_p = properties_p->buffer_p;
  item_count = properties_p->item_count;
  buffer_end_p = properties_p->buffer_p + item_count;

  if (item_count >= 4)
  {
    /* Sort items with heapsort if at least two items are stored in the buffer. */
    uint32_t end = (item_count >> 1) & ~(uint32_t) 0x1;

    do
    {
      end -= 2;
      ecma_module_heap_sort_shift_down (buffer_p, item_count, end);
    } while (end > 0);

    end = item_count - 2;

    do
    {
      ecma_value_t tmp = buffer_p[end];
      buffer_p[end] = buffer_p[0];
      buffer_p[0] = tmp;

      tmp = buffer_p[end + 1];
      buffer_p[end + 1] = buffer_p[1];
      buffer_p[1] = tmp;

      ecma_module_heap_sort_shift_down (buffer_p, end, 0);
      end -= 2;
    } while (end > 0);
  }

  buffer_end_p = properties_p->buffer_p + item_count;

  while (buffer_p < buffer_end_p)
  {
    if (buffer_p[1] & ECMA_MODULE_NAMESPACE_RESULT_FLAG)
    {
      ecma_property_value_t *property_value_p;
      property_value_p = ecma_create_named_data_property (module_p->namespace_object_p,
                                                          ecma_get_string_from_value (buffer_p[0]),
                                                          ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE,
                                                          NULL);
      property_value_p->value = buffer_p[1];
    }
    else
    {
      ecma_create_named_reference_property (module_p->namespace_object_p,
                                            ecma_get_string_from_value (buffer_p[0]),
                                            buffer_p[1]);
    }

    buffer_p += 2;
  }

  module_p->header.u.cls.u2.module_flags |= ECMA_MODULE_HAS_NAMESPACE;

  ecma_module_release_module_names (module_p->local_exports_p);
  module_p->local_exports_p = NULL;

  ecma_module_release_module_nodes (module_p->indirect_exports_p, false);
  module_p->indirect_exports_p = NULL;

exit:
  /* Clean up. */
  ecma_module_resolve_set_cleanup (resolve_set_p);
  ecma_collection_destroy (properties_p);
  return result;
} /* ecma_module_create_namespace_object */

/**
 * Connects imported values to the current module scope.
 *
 * @return ECMA_VALUE_ERROR - if an error occurred
 *         ECMA_VALUE_EMPTY - otherwise
 */
static ecma_value_t
ecma_module_connect_imports (ecma_module_t *module_p)
{
  ecma_object_t *local_env_p = module_p->scope_p;
  JERRY_ASSERT (ecma_is_lexical_environment (local_env_p));

  ecma_module_node_t *import_node_p = module_p->imports_p;

  /* Check that the imported bindings don't exist yet. */
  while (import_node_p != NULL)
  {
    ecma_module_names_t *import_names_p = import_node_p->module_names_p;

    JERRY_ASSERT (local_env_p->type_flags_refs & ECMA_OBJECT_FLAG_BLOCK);

    while (import_names_p != NULL)
    {
      ecma_property_t *binding_p = ecma_find_named_property (local_env_p, import_names_p->local_name_p);

      if (binding_p != NULL)
      {
        return ecma_raise_syntax_error (ECMA_ERR_IMPORTED_BINDING_SHADOWS_LOCAL_VARIABLE);
      }

      import_names_p = import_names_p->next_p;
    }

    import_node_p = import_node_p->next_p;
  }

  import_node_p = module_p->imports_p;

  /* Resolve imports and create local bindings. */
  while (import_node_p != NULL)
  {
    ecma_module_names_t *import_names_p = import_node_p->module_names_p;
    ecma_module_t *imported_module_p = ecma_module_get_from_object (import_node_p->u.path_or_module);

    while (import_names_p != NULL)
    {
      if (ecma_compare_ecma_string_to_magic_id (import_names_p->imex_name_p, LIT_MAGIC_STRING_ASTERISK_CHAR))
      {
        /* Namespace import. */
        ecma_property_value_t *value_p;
        value_p =
          ecma_create_named_data_property (module_p->scope_p, import_names_p->local_name_p, ECMA_PROPERTY_FIXED, NULL);
        value_p->value = ecma_make_object_value (imported_module_p->namespace_object_p);
      }
      else
      {
        ecma_module_resolve_result_t resolve_result;
        ecma_module_resolve_export (imported_module_p, import_names_p->imex_name_p, &resolve_result);

        if (resolve_result.result_type != ECMA_MODULE_RESOLVE_FOUND)
        {
          return ecma_module_resolve_throw (&resolve_result, import_names_p->imex_name_p);
        }

        if (resolve_result.result & ECMA_MODULE_NAMESPACE_RESULT_FLAG)
        {
          ecma_property_value_t *property_value_p;
          property_value_p = ecma_create_named_data_property (module_p->scope_p,
                                                              import_names_p->local_name_p,
                                                              ECMA_PROPERTY_FIXED,
                                                              NULL);
          property_value_p->value = resolve_result.result;
        }
        else
        {
          ecma_create_named_reference_property (module_p->scope_p, import_names_p->local_name_p, resolve_result.result);
        }
      }

      import_names_p = import_names_p->next_p;
    }

    ecma_module_release_module_names (import_node_p->module_names_p);
    import_node_p->module_names_p = NULL;

    import_node_p = import_node_p->next_p;
  }

  return ECMA_VALUE_EMPTY;
} /* ecma_module_connect_imports */

/**
 * Initialize the current module by creating the local binding for the imported variables
 * and verifying indirect exports.
 *
 * @return ECMA_VALUE_ERROR - if an error occurred
 *         ECMA_VALUE_EMPTY - otherwise
 */
ecma_value_t
ecma_module_initialize (ecma_module_t *module_p) /**< module */
{
  ecma_module_node_t *import_node_p = module_p->imports_p;

  while (import_node_p != NULL)
  {
    /* Module is evaluated even if it is used only in export-from statements. */
    ecma_value_t result = ecma_module_evaluate (ecma_module_get_from_object (import_node_p->u.path_or_module));

    if (ECMA_IS_VALUE_ERROR (result))
    {
      return result;
    }

    ecma_free_value (result);

    import_node_p = import_node_p->next_p;
  }

  return ECMA_VALUE_EMPTY;
} /* ecma_module_initialize */

/**
 * Gets the internal module pointer of a module
 *
 * @return module pointer - if module_val is a valid module,
 *         NULL - otherwise
 */
ecma_module_t *
ecma_module_get_resolved_module (ecma_value_t module_val) /**< module */
{
  if (!ecma_is_value_object (module_val))
  {
    return NULL;
  }

  ecma_object_t *object_p = ecma_get_object_from_value (module_val);

  if (!ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_MODULE))
  {
    return NULL;
  }

  return (ecma_module_t *) object_p;
} /* ecma_module_get_resolved_module */

/**
 * A module stack for depth-first search
 */
typedef struct ecma_module_stack_item_t
{
  struct ecma_module_stack_item_t *prev_p; /**< prev in the stack */
  struct ecma_module_stack_item_t *parent_p; /**< parent item in the stack */
  ecma_module_t *module_p; /**< currently processed module */
  ecma_module_node_t *node_p; /**< currently processed node */
  uint32_t dfs_index; /**< dfs index (ES2020 15.2.1.16) */
} ecma_module_stack_item_t;

/**
 * Link module dependencies
 *
 * @return ECMA_VALUE_ERROR - if an error occurred
 *         ECMA_VALUE_UNDEFINED - otherwise
 */
ecma_value_t
ecma_module_link (ecma_module_t *module_p, /**< root module */
                  jerry_module_resolve_cb_t callback, /**< resolve module callback */
                  void *user_p) /**< pointer passed to the resolve callback */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (module_p->header.u.cls.u1.module_state != JERRY_MODULE_STATE_UNLINKED)
  {
    return ecma_raise_type_error (ECMA_ERR_MODULE_MUST_BE_IN_UNLINKED_STATE);
  }

  module_p->header.u.cls.u1.module_state = JERRY_MODULE_STATE_LINKING;

  uint32_t dfs_index = 0;
  ecma_module_stack_item_t *last_p;
  ecma_module_node_t *node_p;

  last_p = (ecma_module_stack_item_t *) jmem_heap_alloc_block (sizeof (ecma_module_stack_item_t));
  last_p->prev_p = NULL;
  last_p->parent_p = NULL;
  last_p->module_p = module_p;
  last_p->node_p = module_p->imports_p;
  last_p->dfs_index = dfs_index;

  module_p->header.u.cls.u3.dfs_ancestor_index = dfs_index;

  ecma_value_t module_val = ecma_make_object_value (&module_p->header.object);
  ecma_module_stack_item_t *current_p = last_p;

restart:
  /* Entering into processing new node phase. Resolve dependencies first. */
  node_p = current_p->node_p;

  JERRY_ASSERT (ecma_module_get_from_object (module_val)->imports_p == node_p);

  while (node_p != NULL)
  {
    ecma_module_t *resolved_module_p;

    if (!ecma_is_value_object (node_p->u.path_or_module))
    {
      JERRY_ASSERT (ecma_is_value_string (node_p->u.path_or_module));

      ecma_value_t resolve_result = callback (node_p->u.path_or_module, module_val, user_p);

      if (JERRY_UNLIKELY (ecma_is_value_exception (resolve_result)))
      {
        ecma_throw_exception (resolve_result);
        goto error;
      }

      resolved_module_p = ecma_module_get_resolved_module (resolve_result);

      if (resolved_module_p == NULL)
      {
        ecma_free_value (resolve_result);
        ecma_raise_type_error (ECMA_ERR_CALLBACK_RESULT_NOT_MODULE);
        goto error;
      }

      ecma_deref_ecma_string (ecma_get_string_from_value (node_p->u.path_or_module));
      node_p->u.path_or_module = resolve_result;
      ecma_deref_object (ecma_get_object_from_value (resolve_result));
    }
    else
    {
      resolved_module_p = ecma_module_get_from_object (node_p->u.path_or_module);
    }

    if (resolved_module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_ERROR)
    {
      ecma_raise_type_error (ECMA_ERR_LINK_TO_MODULE_IN_ERROR_STATE);
      goto error;
    }

    node_p = node_p->next_p;
  }

  /* Find next unlinked node, or return to parent */
  while (true)
  {
    ecma_module_t *current_module_p = current_p->module_p;
    node_p = current_p->node_p;

    while (node_p != NULL)
    {
      module_p = ecma_module_get_from_object (node_p->u.path_or_module);

      if (module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_UNLINKED)
      {
        current_p->node_p = node_p->next_p;
        module_p->header.u.cls.u1.module_state = JERRY_MODULE_STATE_LINKING;

        ecma_module_stack_item_t *item_p;
        item_p = (ecma_module_stack_item_t *) jmem_heap_alloc_block (sizeof (ecma_module_stack_item_t));

        dfs_index++;

        item_p->prev_p = last_p;
        item_p->parent_p = current_p;
        item_p->module_p = module_p;
        item_p->node_p = module_p->imports_p;
        item_p->dfs_index = dfs_index;

        module_p->header.u.cls.u3.dfs_ancestor_index = dfs_index;

        last_p = item_p;
        current_p = item_p;
        module_val = node_p->u.path_or_module;
        goto restart;
      }

      if (module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_LINKING)
      {
        uint32_t dfs_ancestor_index = module_p->header.u.cls.u3.dfs_ancestor_index;

        if (dfs_ancestor_index < current_module_p->header.u.cls.u3.dfs_ancestor_index)
        {
          current_module_p->header.u.cls.u3.dfs_ancestor_index = dfs_ancestor_index;
        }
      }

      node_p = node_p->next_p;
    }

    if (current_module_p->scope_p == NULL)
    {
      JERRY_ASSERT (!(current_module_p->header.u.cls.u2.module_flags & ECMA_MODULE_IS_NATIVE));

      /* Initialize scope for handling circular references. */
      ecma_value_t result = vm_init_module_scope (current_module_p);

      if (ECMA_IS_VALUE_ERROR (result))
      {
        ecma_module_set_error_state (current_module_p);
        goto error;
      }

      JERRY_ASSERT (result == ECMA_VALUE_EMPTY);
    }

    if (current_module_p->namespace_object_p == NULL)
    {
      ecma_object_t *namespace_object_p =
        ecma_create_object (NULL, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

      namespace_object_p->type_flags_refs &= (ecma_object_descriptor_t) ~ECMA_OBJECT_FLAG_EXTENSIBLE;

      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) namespace_object_p;
      ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_MODULE_NAMESPACE;
      ECMA_SET_INTERNAL_VALUE_POINTER (ext_object_p->u.cls.u3.value, module_p);

      current_module_p->namespace_object_p = namespace_object_p;
      ecma_deref_object (namespace_object_p);
    }

    if (current_module_p->header.u.cls.u3.dfs_ancestor_index != current_p->dfs_index)
    {
      current_p = current_p->parent_p;
      JERRY_ASSERT (current_p != NULL);

      uint32_t dfs_ancestor_index = current_module_p->header.u.cls.u3.dfs_ancestor_index;

      if (dfs_ancestor_index < current_p->module_p->header.u.cls.u3.dfs_ancestor_index)
      {
        current_p->module_p->header.u.cls.u3.dfs_ancestor_index = dfs_ancestor_index;
      }
      continue;
    }

    ecma_module_stack_item_t *end_p = current_p->prev_p;
    current_p = current_p->parent_p;

    ecma_module_stack_item_t *iterator_p = last_p;

    do
    {
      JERRY_ASSERT (iterator_p->module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_LINKING);

      if (ECMA_IS_VALUE_ERROR (ecma_module_create_namespace_object (iterator_p->module_p)))
      {
        ecma_module_set_error_state (iterator_p->module_p);
        goto error;
      }

      iterator_p = iterator_p->prev_p;
    } while (iterator_p != end_p);

    iterator_p = last_p;

    do
    {
      JERRY_ASSERT (iterator_p->module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_LINKING);

      if (ECMA_IS_VALUE_ERROR (ecma_module_connect_imports (iterator_p->module_p)))
      {
        ecma_module_set_error_state (iterator_p->module_p);
        goto error;
      }

      iterator_p = iterator_p->prev_p;
    } while (iterator_p != end_p);

    do
    {
      ecma_module_stack_item_t *prev_p = last_p->prev_p;

      JERRY_ASSERT (last_p->module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_LINKING);
      last_p->module_p->header.u.cls.u1.module_state = JERRY_MODULE_STATE_LINKED;

      if (JERRY_CONTEXT (module_state_changed_callback_p) != NULL)
      {
        JERRY_CONTEXT (module_state_changed_callback_p)
        (JERRY_MODULE_STATE_LINKED,
         ecma_make_object_value (&last_p->module_p->header.object),
         ECMA_VALUE_UNDEFINED,
         JERRY_CONTEXT (module_state_changed_callback_user_p));
      }

      jmem_heap_free_block (last_p, sizeof (ecma_module_stack_item_t));
      last_p = prev_p;
    } while (last_p != end_p);

    if (current_p == NULL)
    {
      return ECMA_VALUE_TRUE;
    }
  }

error:
  JERRY_ASSERT (last_p != NULL);

  do
  {
    ecma_module_stack_item_t *prev_p = last_p->prev_p;

    if (last_p->module_p->header.u.cls.u1.module_state != JERRY_MODULE_STATE_ERROR)
    {
      JERRY_ASSERT (last_p->module_p->header.u.cls.u1.module_state == JERRY_MODULE_STATE_LINKING);
      last_p->module_p->header.u.cls.u1.module_state = JERRY_MODULE_STATE_UNLINKED;
    }

    jmem_heap_free_block (last_p, sizeof (ecma_module_stack_item_t));
    last_p = prev_p;
  } while (last_p != NULL);

  return ECMA_VALUE_ERROR;
} /* ecma_module_link */

/**
 * Compute the result of 'import()' calls
 *
 * @return promise object representing the result of the operation
 */
ecma_value_t
ecma_module_import (ecma_value_t specifier, /**< module specifier */
                    ecma_value_t user_value) /**< user value assigned to the script */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_string_t *specifier_p = ecma_op_to_string (specifier);
  ecma_module_t *module_p;

  if (JERRY_UNLIKELY (specifier_p == NULL))
  {
    goto error;
  }

  if (JERRY_CONTEXT (module_import_callback_p) == NULL)
  {
    ecma_deref_ecma_string (specifier_p);
    goto error_module_instantiate;
  }

  jerry_value_t result;
  result = JERRY_CONTEXT (module_import_callback_p) (ecma_make_string_value (specifier_p),
                                                     user_value,
                                                     JERRY_CONTEXT (module_import_callback_user_p));
  ecma_deref_ecma_string (specifier_p);

  if (JERRY_UNLIKELY (ecma_is_value_exception (result)))
  {
    ecma_throw_exception (result);
    goto error;
  }

  if (ecma_is_value_object (result) && ecma_is_promise (ecma_get_object_from_value (result)))
  {
    return result;
  }

  module_p = ecma_module_get_resolved_module (result);

  if (module_p == NULL)
  {
    ecma_free_value (result);
    goto error_module_instantiate;
  }

  if (module_p->header.u.cls.u1.module_state != JERRY_MODULE_STATE_EVALUATED)
  {
    ecma_deref_object (&module_p->header.object);
    goto error_module_instantiate;
  }

  result = ecma_op_create_promise_object (ECMA_VALUE_EMPTY, ECMA_VALUE_UNDEFINED, NULL);
  ecma_fulfill_promise (result, ecma_make_object_value (module_p->namespace_object_p));
  ecma_deref_object (&module_p->header.object);
  return result;

error_module_instantiate:
  ecma_raise_range_error (ECMA_ERR_MODULE_CANNOT_BE_INSTANTIATED);

error:
  if (jcontext_has_pending_abort ())
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_value_t exception = jcontext_take_exception ();

  ecma_value_t promise = ecma_op_create_promise_object (ECMA_VALUE_EMPTY, ECMA_VALUE_UNDEFINED, NULL);
  ecma_reject_promise (promise, exception);
  ecma_free_value (exception);
  return promise;
} /* ecma_module_import */

/**
 * Cleans up and releases a module structure including all referenced modules.
 */
void
ecma_module_release_module (ecma_module_t *module_p) /**< module */
{
  jerry_module_state_t state = (jerry_module_state_t) module_p->header.u.cls.u1.module_state;

  JERRY_ASSERT (state != JERRY_MODULE_STATE_INVALID);

#ifndef JERRY_NDEBUG
  module_p->scope_p = NULL;
  module_p->namespace_object_p = NULL;
#endif /* JERRY_NDEBUG */

  ecma_module_release_module_names (module_p->local_exports_p);

  if (module_p->header.u.cls.u2.module_flags & ECMA_MODULE_IS_NATIVE)
  {
    return;
  }

  ecma_module_release_module_nodes (module_p->imports_p, true);
  ecma_module_release_module_nodes (module_p->indirect_exports_p, false);
  ecma_module_release_module_nodes (module_p->star_exports_p, false);

  if (module_p->u.compiled_code_p != NULL)
  {
    ecma_bytecode_deref (module_p->u.compiled_code_p);
#ifndef JERRY_NDEBUG
    module_p->u.compiled_code_p = NULL;
#endif /* JERRY_NDEBUG */
  }
} /* ecma_module_release_module */

#endif /* JERRY_MODULE_SYSTEM */
