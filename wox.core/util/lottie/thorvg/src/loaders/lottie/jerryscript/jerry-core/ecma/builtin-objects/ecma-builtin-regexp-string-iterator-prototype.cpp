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

#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-conversion.h"
#include "ecma-gc.h"
#include "ecma-iterator-object.h"
#include "ecma-objects.h"

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

#define BUILTIN_INC_HEADER_NAME "ecma-builtin-regexp-string-iterator-prototype.inc.h"
#define BUILTIN_UNDERSCORED_ID  regexp_string_iterator_prototype
#include "ecma-builtin-internal-routines-template.inc.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 *
 * \addtogroup %regexpstringiteratorprototype% ECMA %ArrayIteratorPrototype% object built-in
 * @{
 */

/**
 * The %RegExpStringIteratorPrototype% object's 'next' routine
 *
 * See also:
 *          ECMA-262 v11, 21.2.7.1.1
 *
 * Note:
 *     Returned value must be freed with ecma_free_value.
 *
 * @return iterator result object, if success
 *         error - otherwise
 */
#ifdef JERRY_BUILTIN_REGEXP
static ecma_value_t
ecma_builtin_regexp_string_iterator_prototype_object_next (ecma_value_t this_val) /**< this argument */
{
  /* 2. */
  if (!ecma_is_value_object (this_val))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_OBJECT);
  }

  ecma_object_t *obj_p = ecma_get_object_from_value (this_val);

  /* 3. */
  if (!ecma_object_class_is (obj_p, ECMA_OBJECT_CLASS_REGEXP_STRING_ITERATOR))
  {
    return ecma_raise_type_error (ECMA_ERR_ARGUMENT_THIS_NOT_ITERATOR);
  }

  ecma_regexp_string_iterator_t *regexp_string_iterator_obj = (ecma_regexp_string_iterator_t *) obj_p;

  /* 4. */
  if (ecma_is_value_empty (regexp_string_iterator_obj->iterated_string))
  {
    return ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
  }

  /* 5. */
  ecma_value_t regexp = regexp_string_iterator_obj->iterating_regexp;

  /* 6. */
  ecma_value_t matcher_str_value = regexp_string_iterator_obj->iterated_string;
  ecma_string_t *matcher_str_p = ecma_get_string_from_value (matcher_str_value);

  /* 9. */
  ecma_value_t match = ecma_op_regexp_exec (regexp, matcher_str_p);

  if (ECMA_IS_VALUE_ERROR (match))
  {
    return match;
  }

  /* 10. */
  if (ecma_is_value_null (match))
  {
    ecma_free_value (regexp_string_iterator_obj->iterated_string);
    regexp_string_iterator_obj->iterated_string = ECMA_VALUE_EMPTY;
    return ecma_create_iter_result_object (ECMA_VALUE_UNDEFINED, ECMA_VALUE_TRUE);
  }

  ecma_object_t *match_result_array_p = ecma_get_object_from_value (match);

  ecma_value_t result = ECMA_VALUE_ERROR;

  /* 11. */
  if (regexp_string_iterator_obj->header.u.cls.u1.regexp_string_iterator_flags & RE_FLAG_GLOBAL)
  {
    ecma_value_t matched_str_value = ecma_op_object_get_by_index (match_result_array_p, 0);

    if (ECMA_IS_VALUE_ERROR (matched_str_value))
    {
      goto free_variables;
    }

    ecma_string_t *matched_str_p = ecma_op_to_string (matched_str_value);

    ecma_free_value (matched_str_value);

    if (JERRY_UNLIKELY (matched_str_p == NULL))
    {
      goto free_variables;
    }

    if (ecma_string_is_empty (matched_str_p))
    {
      ecma_object_t *regexp_obj_p = ecma_get_object_from_value (regexp);

      ecma_value_t get_last_index = ecma_op_object_get_by_magic_id (regexp_obj_p, LIT_MAGIC_STRING_LASTINDEX_UL);

      if (ECMA_IS_VALUE_ERROR (get_last_index))
      {
        goto free_variables;
      }

      ecma_length_t this_index;
      ecma_value_t to_len = ecma_op_to_length (get_last_index, &this_index);

      ecma_free_value (get_last_index);

      if (ECMA_IS_VALUE_ERROR (to_len))
      {
        goto free_variables;
      }

      uint8_t flags = regexp_string_iterator_obj->header.u.cls.u1.regexp_string_iterator_flags;
      ecma_length_t next_index =
        ecma_op_advance_string_index (matcher_str_p, this_index, (flags & RE_FLAG_UNICODE) != 0);

      ecma_value_t next_index_value = ecma_make_length_value (next_index);
      ecma_value_t set = ecma_op_object_put (regexp_obj_p,
                                             ecma_get_magic_string (LIT_MAGIC_STRING_LASTINDEX_UL),
                                             next_index_value,
                                             true);

      ecma_free_value (next_index_value);

      if (ECMA_IS_VALUE_ERROR (set))
      {
        goto free_variables;
      }
    }
    else
    {
      ecma_deref_ecma_string (matched_str_p);
    }
  }
  else
  {
    ecma_free_value (regexp_string_iterator_obj->iterated_string);
    regexp_string_iterator_obj->iterated_string = ECMA_VALUE_EMPTY;
  }

  result = ecma_create_iter_result_object (match, ECMA_VALUE_FALSE);

free_variables:
  ecma_deref_object (match_result_array_p);

  return result;
} /* ecma_builtin_regexp_string_iterator_prototype_object_next */
#endif

/**
 * @}
 * @}
 * @}
 */
