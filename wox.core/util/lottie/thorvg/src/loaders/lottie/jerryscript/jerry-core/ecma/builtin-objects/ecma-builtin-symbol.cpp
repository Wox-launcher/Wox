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
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-literal-storage.h"
#include "ecma-objects.h"
#include "ecma-symbol-object.h"

#include "jcontext.h"
#include "jrt.h"

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-symbol.inc.h"
#define BUILTIN_UNDERSCORED_ID  symbol
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup symbol ECMA Symbol object built-in
 * @{
 */

/**
 * Handle calling [[Call]] of built-in Symbol object.
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_symbol_dispatch_call (const ecma_value_t *arguments_list_p, /**< arguments list */
                                   uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_op_create_symbol (arguments_list_p, arguments_list_len);
} /* ecma_builtin_symbol_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in Symbol object.
 *
 * Symbol constructor is not intended to be used
 * with the new operator or to be subclassed.
 *
 * See also:
 *          ECMA-262 v6, 19.4.1
 * @return ecma value
 */
ecma_value_t
ecma_builtin_symbol_dispatch_construct (const ecma_value_t *arguments_list_p, /**< arguments list */
                                        uint32_t arguments_list_len) /**< number of arguments */
{
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  return ecma_raise_type_error (ECMA_ERR_SYMBOL_IS_NOT_A_CONSTRUCTOR);
} /* ecma_builtin_symbol_dispatch_construct */

/**
 * Helper function for Symbol object's 'for' and `keyFor`
 * routines common parts
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_symbol_for_helper (ecma_value_t value_to_find) /**< symbol or ecma-string */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  ecma_string_t *string_p;

  bool is_for = ecma_is_value_string (value_to_find);

  if (is_for)
  {
    string_p = ecma_get_string_from_value (value_to_find);
  }
  else
  {
    string_p = ecma_get_symbol_from_value (value_to_find);
  }

  jmem_cpointer_t symbol_list_cp = JERRY_CONTEXT (symbol_list_first_cp);
  jmem_cpointer_t *empty_cpointer_p = NULL;

  while (symbol_list_cp != JMEM_CP_NULL)
  {
    ecma_lit_storage_item_t *symbol_list_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_lit_storage_item_t, symbol_list_cp);

    for (int i = 0; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
    {
      if (symbol_list_p->values[i] != JMEM_CP_NULL)
      {
        ecma_string_t *value_p = JMEM_CP_GET_NON_NULL_POINTER (ecma_string_t, symbol_list_p->values[i]);

        if (is_for)
        {
          ecma_value_t symbol_desc = ecma_get_symbol_description (value_p);

          if (ecma_is_value_undefined (symbol_desc))
          {
            ecma_ref_ecma_string (value_p);
            return ecma_make_symbol_value (value_p);
          }

          ecma_string_t *symbol_desc_p = ecma_get_string_from_value (symbol_desc);

          if (ecma_compare_ecma_strings (symbol_desc_p, string_p))
          {
            /* The current symbol's descriptor matches with the value_to_find,
               so the value is no longer needed. */
            ecma_deref_ecma_string (string_p);
            return ecma_copy_value (ecma_make_symbol_value (value_p));
          }
        }
        else
        {
          if (string_p == value_p)
          {
            ecma_value_t symbol_desc = ecma_get_symbol_description (string_p);

            if (ecma_is_value_undefined (symbol_desc))
            {
              return symbol_desc;
            }

            ecma_string_t *symbol_desc_p = ecma_get_string_from_value (symbol_desc);
            ecma_ref_ecma_string (symbol_desc_p);
            return symbol_desc;
          }
        }
      }
      else
      {
        if (empty_cpointer_p == NULL)
        {
          empty_cpointer_p = symbol_list_p->values + i;
        }
      }
    }

    symbol_list_cp = symbol_list_p->next_cp;
  }

  if (!is_for)
  {
    return ECMA_VALUE_UNDEFINED;
  }

  /* There was no matching, sp a new symbol should be added the global symbol list. The symbol creation requires
     an extra reference to the descriptor string, but this reference has already been added. */
  ecma_string_t *new_symbol_p = ecma_new_symbol_from_descriptor_string (value_to_find);

  jmem_cpointer_t result;
  JMEM_CP_SET_NON_NULL_POINTER (result, new_symbol_p);

  if (empty_cpointer_p != NULL)
  {
    *empty_cpointer_p = result;
    return ecma_copy_value (ecma_make_symbol_value (new_symbol_p));
  }

  ecma_lit_storage_item_t *new_item_p;
  new_item_p = (ecma_lit_storage_item_t *) jmem_pools_alloc (sizeof (ecma_lit_storage_item_t));

  new_item_p->values[0] = result;
  for (int i = 1; i < ECMA_LIT_STORAGE_VALUE_COUNT; i++)
  {
    new_item_p->values[i] = JMEM_CP_NULL;
  }

  new_item_p->next_cp = JERRY_CONTEXT (symbol_list_first_cp);
  JMEM_CP_SET_NON_NULL_POINTER (JERRY_CONTEXT (symbol_list_first_cp), new_item_p);

  return ecma_copy_value (ecma_make_symbol_value (new_symbol_p));
} /* ecma_builtin_symbol_for_helper */

/**
 * The Symbol object's 'for' routine
 *
 * See also:
 *          ECMA-262 v6, 19.4.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_symbol_for (ecma_value_t this_arg, /**< this argument */
                         ecma_value_t key) /**< key string */
{
  JERRY_UNUSED (this_arg);
  ecma_string_t *string_desc_p = ecma_op_to_string (key);

  /* 1. */
  if (JERRY_UNLIKELY (string_desc_p == NULL))
  {
    /* 2. */
    return ECMA_VALUE_ERROR;
  }

  return ecma_builtin_symbol_for_helper (ecma_make_string_value (string_desc_p));
} /* ecma_builtin_symbol_for */

/**
 * The Symbol object's 'keyFor' routine
 *
 * See also:
 *          ECMA-262 v6, 19.4.2.
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_symbol_key_for (ecma_value_t this_arg, /**< this argument */
                             ecma_value_t symbol) /**< symbol */
{
  JERRY_UNUSED (this_arg);

  /* 1. */
  if (!ecma_is_value_symbol (symbol))
  {
    return ecma_raise_type_error (ECMA_ERR_THE_GIVEN_ARGUMENT_IS_NOT_A_SYMBOL);
  }

  /* 2-4. */
  return ecma_builtin_symbol_for_helper (symbol);
} /* ecma_builtin_symbol_key_for */

/**
 * @}
 * @}
 * @}
 */
