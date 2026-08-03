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

#ifndef ECMA_EXTENDED_INFO_H
#define ECMA_EXTENDED_INFO_H

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaextendedinfo Extended info
 * @{
 */

#include "ecma-globals.h"

/**
 * Vlq encoding: flag which is set for all bytes except the last one.
 */
#define ECMA_EXTENDED_INFO_VLQ_CONTINUE 0x80

/**
 * Vlq encoding: mask to decode the number fragment.
 */
#define ECMA_EXTENDED_INFO_VLQ_MASK 0x7f

/**
 * Vlq encoding: number of bits stored in a byte.
 */
#define ECMA_EXTENDED_INFO_VLQ_SHIFT 7

uint32_t ecma_extended_info_decode_vlq (uint8_t **buffer_p);
void ecma_extended_info_encode_vlq (uint8_t **buffer_p, uint32_t value);
uint32_t ecma_extended_info_get_encoded_length (uint32_t value);

uint8_t *ecma_compiled_code_resolve_extended_info (const ecma_compiled_code_t *bytecode_header_p);

/**
 * @}
 * @}
 */

#endif /* !ECMA_EXTENDED_INFO_H */
