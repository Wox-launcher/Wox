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

#include "js-parser-tagged-template-literal.h"

#include "ecma-array-object.h"
#include "ecma-builtin-helpers.h"
#include "ecma-gc.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"

#include "js-lexer.h"

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_tagged_template_literal Tagged template literal
 * @{
 */

/**
 * Append the cooked and raw string to the corresponding array
 */
void
parser_tagged_template_literal_append_strings (parser_context_t *context_p, /**< parser context */
                                               ecma_object_t *template_obj_p, /**< template object */
                                               ecma_object_t *raw_strings_p, /**< raw strings object */
                                               uint32_t prop_idx) /**< property index to set the values */
{
  lexer_lit_location_t *lit_loc_p = &context_p->token.lit_location;

  if (lit_loc_p->length == 0 && !(lit_loc_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE))
  {
    ecma_builtin_helper_def_prop_by_index (template_obj_p,
                                           prop_idx,
                                           ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY),
                                           ECMA_PROPERTY_FLAG_ENUMERABLE);

    ecma_builtin_helper_def_prop_by_index (raw_strings_p,
                                           prop_idx,
                                           ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY),
                                           ECMA_PROPERTY_FLAG_ENUMERABLE);
    return;
  }

  uint8_t local_byte_array[LEXER_MAX_LITERAL_LOCAL_BUFFER_SIZE];
  const uint8_t *source_p =
    lexer_convert_literal_to_chars (context_p, &context_p->token.lit_location, local_byte_array, LEXER_STRING_NO_OPTS);

  ecma_string_t *raw_str_p;
  ecma_string_t *cooked_str_p =
    ((lit_loc_p->status_flags & LEXER_FLAG_ASCII) ? ecma_new_ecma_string_from_ascii (source_p, lit_loc_p->length)
                                                  : ecma_new_ecma_string_from_utf8 (source_p, lit_loc_p->length));

  parser_free_allocated_buffer (context_p);

  if (lit_loc_p->status_flags & LEXER_LIT_LOCATION_HAS_ESCAPE)
  {
    context_p->source_p = context_p->token.lit_location.char_p - 1;
    lexer_parse_string (context_p, LEXER_STRING_RAW);
    source_p =
      lexer_convert_literal_to_chars (context_p, &context_p->token.lit_location, local_byte_array, LEXER_STRING_RAW);

    raw_str_p =
      ((lit_loc_p->status_flags & LEXER_FLAG_ASCII) ? ecma_new_ecma_string_from_ascii (source_p, lit_loc_p->length)
                                                    : ecma_new_ecma_string_from_utf8 (source_p, lit_loc_p->length));

    parser_free_allocated_buffer (context_p);
  }
  else
  {
    ecma_ref_ecma_string (cooked_str_p);
    raw_str_p = cooked_str_p;
  }

  ecma_builtin_helper_def_prop_by_index (template_obj_p,
                                         prop_idx,
                                         ecma_make_string_value (cooked_str_p),
                                         ECMA_PROPERTY_FLAG_ENUMERABLE);

  ecma_builtin_helper_def_prop_by_index (raw_strings_p,
                                         prop_idx,
                                         ecma_make_string_value (raw_str_p),
                                         ECMA_PROPERTY_FLAG_ENUMERABLE);

  ecma_deref_ecma_string (cooked_str_p);
  ecma_deref_ecma_string (raw_str_p);
} /* parser_tagged_template_literal_append_strings */

/**
 * Create new tagged template literal object
 *
 * @return pointer to the allocated object
 */
ecma_object_t *
parser_new_tagged_template_literal (ecma_object_t **raw_strings_p) /**< [out] raw strings object */
{
  ecma_object_t *template_obj_p = ecma_op_new_array_object (0);
  *raw_strings_p = ecma_op_new_array_object (0);

  ecma_extended_object_t *template_ext_obj_p = (ecma_extended_object_t *) template_obj_p;
  ecma_extended_object_t *raw_ext_obj_p = (ecma_extended_object_t *) *raw_strings_p;

  const uint8_t flags = ECMA_PROPERTY_VIRTUAL | ECMA_PROPERTY_FLAG_WRITABLE | ECMA_FAST_ARRAY_FLAG;
  JERRY_ASSERT (template_ext_obj_p->u.array.length_prop_and_hole_count == flags);
  JERRY_ASSERT (raw_ext_obj_p->u.array.length_prop_and_hole_count == flags);

  template_ext_obj_p->u.array.length_prop_and_hole_count = flags | ECMA_ARRAY_TEMPLATE_LITERAL;
  raw_ext_obj_p->u.array.length_prop_and_hole_count = flags | ECMA_ARRAY_TEMPLATE_LITERAL;

  ecma_builtin_helper_def_prop (template_obj_p,
                                ecma_get_magic_string (LIT_MAGIC_STRING_RAW),
                                ecma_make_object_value (*raw_strings_p),
                                ECMA_PROPERTY_FIXED);

  return template_obj_p;
} /* parser_new_tagged_template_literal */

/**
 * Set integrity level of the given template array object to "frozen"
 */
static void
parser_tagged_template_literal_freeze_array (ecma_object_t *obj_p /**< template object */)
{
  JERRY_ASSERT (ecma_get_object_type (obj_p) == ECMA_OBJECT_TYPE_ARRAY);
  ecma_op_ordinary_object_prevent_extensions (obj_p);
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) obj_p;
  ext_obj_p->u.array.length_prop_and_hole_count &= (uint32_t) ~ECMA_PROPERTY_FLAG_WRITABLE;
} /* parser_tagged_template_literal_freeze_array */

/**
 * Finalize the tagged template object
 */
void
parser_tagged_template_literal_finalize (ecma_object_t *template_obj_p, /**< template object */
                                         ecma_object_t *raw_strings_p) /**< raw strings object */
{
  parser_tagged_template_literal_freeze_array (template_obj_p);
  parser_tagged_template_literal_freeze_array (raw_strings_p);
} /* parser_tagged_template_literal_finalize */

/**
 * @}
 * @}
 * @}
 */
