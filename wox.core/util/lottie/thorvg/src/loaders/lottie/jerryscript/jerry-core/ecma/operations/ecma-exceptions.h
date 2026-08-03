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

#ifndef ECMA_EXCEPTIONS_H
#define ECMA_EXCEPTIONS_H

#include "ecma-globals.h"

#include "jrt.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup exceptions Exceptions
 * @{
 */

jerry_error_t ecma_get_error_type (ecma_object_t *error_object_p);
ecma_object_t *ecma_new_standard_error (jerry_error_t error_type, ecma_string_t *message_string_p);
#if JERRY_ERROR_MESSAGES
ecma_value_t ecma_raise_standard_error_with_format (jerry_error_t error_type, const char *msg_p, ...);
#endif /* JERRY_ERROR_MESSAGES */
ecma_value_t ecma_raise_standard_error (jerry_error_t error_type, ecma_error_msg_t msg);
ecma_value_t ecma_raise_common_error (ecma_error_msg_t msg);
ecma_value_t ecma_raise_range_error (ecma_error_msg_t msg);
ecma_value_t ecma_raise_reference_error (ecma_error_msg_t msg);
ecma_value_t ecma_raise_syntax_error (ecma_error_msg_t msg);
ecma_value_t ecma_raise_type_error (ecma_error_msg_t msg);
ecma_value_t ecma_raise_uri_error (ecma_error_msg_t msg);
#if (JERRY_STACK_LIMIT != 0)
ecma_value_t ecma_raise_maximum_callstack_error (void);
#endif /* (JERRY_STACK_LIMIT != 0) */
ecma_value_t ecma_new_aggregate_error (ecma_value_t error_list_val, ecma_value_t message_val);
ecma_value_t ecma_raise_aggregate_error (ecma_value_t error_list_val, ecma_value_t message_val);

/**
 * @}
 * @}
 */

#endif /* !ECMA_EXCEPTIONS_H */
