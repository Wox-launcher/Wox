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

#ifndef JRT_BIT_FIELDS_H
#define JRT_BIT_FIELDS_H

/**
 * Extract a bit-field.
 *
 * @param type type of container
 * @param container container to extract bit-field from
 * @param lsb least significant bit of the value to be extracted
 * @param width width of the bit-field to be extracted
 * @return bit-field's value
 */
#define JRT_EXTRACT_BIT_FIELD(type, container, lsb, width) (((container) >> lsb) & ((((type) 1) << (width)) - 1))

/**
 * Set a bit-field.
 *
 * @param type type of container
 * @param container container to insert bit-field to
 * @param new_bit_field_value value of bit-field to insert
 * @param lsb least significant bit of the value to be inserted
 * @param width width of the bit-field to be inserted
 * @return bit-field's value
 */
#define JRT_SET_BIT_FIELD_VALUE(type, container, new_bit_field_value, lsb, width) \
  (((container) & ~(((((type) 1) << (width)) - 1) << (lsb))) | (((type) new_bit_field_value) << (lsb)))

#endif /* !JRT_BIT_FIELDS_H */
