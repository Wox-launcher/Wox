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

#include "ecma-alloc.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-exceptions.h"
#include "ecma-gc.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-regexp-object.h"

#include "jcontext.h"

#if JERRY_BUILTIN_REGEXP

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-regexp.inc.h"
#define BUILTIN_UNDERSCORED_ID  regexp
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup regexp ECMA RegExp object built-in
 * @{
 */

static ecma_value_t
ecma_builtin_regexp_dispatch_helper (const ecma_value_t *arguments_list_p, /**< arguments list */
                                     uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_value_t pattern_value = ECMA_VALUE_UNDEFINED;
  ecma_value_t flags_value = ECMA_VALUE_UNDEFINED;
  bool create_regexp_from_bc = false;
  bool free_arguments = false;
  ecma_object_t *new_target_p = JERRY_CONTEXT (current_new_target_p);

  if (arguments_list_len > 0)
  {
    /* pattern string or RegExp object */
    pattern_value = arguments_list_p[0];

    if (arguments_list_len > 1)
    {
      flags_value = arguments_list_p[1];
    }
  }

  ecma_value_t regexp_value = ecma_op_is_regexp (pattern_value);

  if (ECMA_IS_VALUE_ERROR (regexp_value))
  {
    return regexp_value;
  }

  bool pattern_is_regexp = regexp_value == ECMA_VALUE_TRUE;
  re_compiled_code_t *bc_p = NULL;

  if (new_target_p == NULL)
  {
    new_target_p = ecma_builtin_get (ECMA_BUILTIN_ID_REGEXP);

    if (pattern_is_regexp && ecma_is_value_undefined (flags_value))
    {
      ecma_object_t *pattern_obj_p = ecma_get_object_from_value (pattern_value);

      ecma_value_t pattern_constructor = ecma_op_object_get_by_magic_id (pattern_obj_p, LIT_MAGIC_STRING_CONSTRUCTOR);

      if (ECMA_IS_VALUE_ERROR (pattern_constructor))
      {
        return pattern_constructor;
      }

      bool is_same = ecma_op_same_value (ecma_make_object_value (new_target_p), pattern_constructor);
      ecma_free_value (pattern_constructor);

      if (is_same)
      {
        return ecma_copy_value (pattern_value);
      }
    }
  }

  if (ecma_object_is_regexp_object (pattern_value))
  {
    ecma_extended_object_t *pattern_obj_p = (ecma_extended_object_t *) ecma_get_object_from_value (pattern_value);
    bc_p = ECMA_GET_INTERNAL_VALUE_POINTER (re_compiled_code_t, pattern_obj_p->u.cls.u3.value);

    create_regexp_from_bc = ecma_is_value_undefined (flags_value);

    if (!create_regexp_from_bc)
    {
      pattern_value = bc_p->source;
    }
  }
  else if (pattern_is_regexp)
  {
    ecma_object_t *pattern_obj_p = ecma_get_object_from_value (pattern_value);

    pattern_value = ecma_op_object_get_by_magic_id (pattern_obj_p, LIT_MAGIC_STRING_SOURCE);

    if (ECMA_IS_VALUE_ERROR (pattern_value))
    {
      return pattern_value;
    }

    if (ecma_is_value_undefined (flags_value))
    {
      flags_value = ecma_op_object_get_by_magic_id (pattern_obj_p, LIT_MAGIC_STRING_FLAGS);

      if (ECMA_IS_VALUE_ERROR (flags_value))
      {
        ecma_free_value (pattern_value);
        return flags_value;
      }
    }
    else
    {
      flags_value = ecma_copy_value (flags_value);
    }

    free_arguments = true;
  }

  ecma_value_t ret_value = ECMA_VALUE_ERROR;
  ecma_object_t *new_target_obj_p = ecma_op_regexp_alloc (new_target_p);

  if (JERRY_LIKELY (new_target_obj_p != NULL))
  {
    if (create_regexp_from_bc)
    {
      ret_value = ecma_op_create_regexp_from_bytecode (new_target_obj_p, bc_p);
      JERRY_ASSERT (!ECMA_IS_VALUE_ERROR (ret_value));
    }
    else
    {
      ret_value = ecma_op_create_regexp_from_pattern (new_target_obj_p, pattern_value, flags_value);

      if (ECMA_IS_VALUE_ERROR (ret_value))
      {
        ecma_deref_object (new_target_obj_p);
      }
    }
  }

  if (free_arguments)
  {
    ecma_free_value (pattern_value);
    ecma_free_value (flags_value);
  }

  return ret_value;
} /* ecma_builtin_regexp_dispatch_helper */

/**
 * Handle calling [[Call]] of built-in RegExp object
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_regexp_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                   uint32_t arguments_list_len) /**< number of arguments */
{
  return ecma_builtin_regexp_dispatch_helper (arguments_list_p, arguments_list_len);
} /* ecma_builtin_regexp_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in RegExp object
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_regexp_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                        uint32_t arguments_list_len) /**< number of arguments */
{
  return ecma_builtin_regexp_dispatch_helper (arguments_list_p, arguments_list_len);
} /* ecma_builtin_regexp_dispatch_construct */

/**
 * 21.2.4.2 get RegExp [ @@species ] accessor
 *
 * @return ecma_value
 *         returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_builtin_regexp_species_get (ecma_value_t this_value) /**< This Value */
{
  return ecma_copy_value (this_value);
} /* ecma_builtin_regexp_species_get */

/**
 * @}
 * @}
 * @}
 */

#endif /* JERRY_BUILTIN_REGEXP */
