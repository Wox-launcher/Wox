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

#ifndef LIT_GLOBALS_H
#define LIT_GLOBALS_H

#include "jrt.h"

/**
 * ECMAScript standard defines terms "code unit" and "character" as 16-bit unsigned value
 * used to represent 16-bit unit of text, this is the same as code unit in UTF-16 (See ECMA-262 5.1 Chapter 6).
 *
 * The term "code point" or "Unicode character" is used to refer a single Unicode scalar value (may be longer
 * than 16 bits: 0x0 - 0x10FFFFF). One code point could be represented with one ore two 16-bit code units.
 *
 * According to the standard all strings and source text are assumed to be a sequence of code units.
 * Length of a string equals to number of code units in the string, which is not the same as number of Unicode
 * characters in a string.
 *
 * Internally JerryScript engine uses UTF-8 representation of strings to reduce memory overhead. Unicode character
 * occupies from one to four bytes in UTF-8 representation.
 *
 * Unicode scalar value   | Bytes in UTF-8             | Bytes in UTF-16
 *                        | (internal representation)  |
 * ----------------------------------------------------------------------
 *  0x0     - 0x7F        |  1 byte                    |  2 bytes
 *  0x80    - 0x7FF       |  2 bytes                   |  2 bytes
 *  0x800   - 0xFFFF      |  3 bytes                   |  2 bytes
 *  0x10000 - 0x10FFFF    |  4 bytes                   |  4 bytes
 *
 * Scalar values from 0xD800 to 0xDFFF are permanently reserved by Unicode standard to encode high and low
 * surrogates in UTF-16 (Code points 0x10000 - 0x10FFFF are encoded via pair of surrogates in UTF-16).
 * Despite that the official Unicode standard says that no UTF forms can encode these code points, we allow
 * them to be encoded inside strings. The reason for that is compatibility with ECMA standard.
 *
 * For example, assume a string which consists one Unicode character: 0x1D700 (Mathematical Italic Small Epsilon).
 * It has the following representation in UTF-16: 0xD835 0xDF00.
 *
 * ECMA standard allows extracting a substring from this string:
 * > var str = String.fromCharCode (0xD835, 0xDF00); // Create a string containing one character: 0x1D700
 * > str.length; // 2
 * > var str1 = str.substring (0, 1);
 * > str1.length; // 1
 * > str1.charCodeAt (0); // 55349 (this equals to 0xD835)
 *
 * Internally original string would be represented in UTF-8 as the following byte sequence: 0xF0 0x9D 0x9C 0x80.
 * After substring extraction high surrogate 0xD835 should be encoded via UTF-8: 0xED 0xA0 0xB5.
 *
 * Pair of low and high surrogates encoded separately should never occur in internal string representation,
 * it should be encoded as any code point and occupy 4 bytes. So, when constructing a string from two surrogates,
 * it should be processed gracefully;
 * > var str1 = String.fromCharCode (0xD835); // 0xED 0xA0 0xB5 - internal representation
 * > var str2 = String.fromCharCode (0xDF00); // 0xED 0xBC 0x80 - internal representation
 * > var str = str1 + str2; // 0xF0 0x9D 0x9C 0x80 - internal representation,
 *                          // !!! not 0xED 0xA0 0xB5 0xED 0xBC 0x80
 */

/**
 * Description of an ecma-character, which represents 16-bit code unit,
 * which is equal to UTF-16 character (see Chapter 6 from ECMA-262 5.1)
 */
typedef uint16_t ecma_char_t;

/**
 * Max bytes needed to represent a code unit (utf-16 char) via utf-8 encoding
 */
#define LIT_UTF8_MAX_BYTES_IN_CODE_UNIT (3)

/**
 * Max bytes needed to represent a code point (Unicode character) via utf-8 encoding
 */
#define LIT_UTF8_MAX_BYTES_IN_CODE_POINT (4)

/**
 * Max bytes needed to represent a code unit (utf-16 char) via cesu-8 encoding
 */
#define LIT_CESU8_MAX_BYTES_IN_CODE_UNIT (3)

/**
 * Max bytes needed to represent a code point (Unicode character) via cesu-8 encoding
 */
#define LIT_CESU8_MAX_BYTES_IN_CODE_POINT (6)

/**
 * A byte of utf-8 string
 */
typedef uint8_t lit_utf8_byte_t;

/**
 * Size of a utf-8 string in bytes
 */
typedef uint32_t lit_utf8_size_t;

/**
 * Size of a magic string in bytes
 */
typedef uint8_t lit_magic_size_t;

/**
 * Unicode code point
 */
typedef uint32_t lit_code_point_t;

/**
 * ECMA string hash
 */
typedef uint32_t lit_string_hash_t;

#endif /* !LIT_GLOBALS_H */
