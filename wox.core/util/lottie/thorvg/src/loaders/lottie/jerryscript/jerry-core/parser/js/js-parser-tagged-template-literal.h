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

#ifndef ECMA_TAGGED_TEMPLATE_LITERAL_H
#define ECMA_TAGGED_TEMPLATE_LITERAL_H

/** \addtogroup parser Parser
 * @{
 *
 * \addtogroup jsparser JavaScript
 * @{
 *
 * \addtogroup jsparser_tagged_template_literal Tagged template literal
 * @{
 */

#include "ecma-globals.h"

#include "common.h"
#include "js-parser-internal.h"

ecma_object_t *parser_new_tagged_template_literal (ecma_object_t **raw_strings_p);

void parser_tagged_template_literal_append_strings (parser_context_t *context_p,
                                                    ecma_object_t *template_obj_p,
                                                    ecma_object_t *raw_strings_p,
                                                    uint32_t prop_index);

void parser_tagged_template_literal_finalize (ecma_object_t *template_obj_p, ecma_object_t *raw_strings_p);

/**
 * @}
 * @}
 * @}
 */

#endif /* ECMA_TAGGED_TEMPLATE_LITERAL_H */
