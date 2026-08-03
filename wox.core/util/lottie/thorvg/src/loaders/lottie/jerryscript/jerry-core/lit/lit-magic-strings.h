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

#ifndef LIT_MAGIC_STRINGS_H
#define LIT_MAGIC_STRINGS_H

#include "lit-globals.h"

/**
 * Identifiers of ECMA and implementation-defined magic string constants
 */
typedef enum
{
/** @cond doxygen_suppress */
#define LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE(size, id)
#define LIT_MAGIC_STRING_DEF(id, ascii_zt_string) id,
#include "lit-magic-strings.inc.h"
#undef LIT_MAGIC_STRING_DEF
#undef LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE
  /** @endcond */
  LIT_NON_INTERNAL_MAGIC_STRING__COUNT, /**< number of non-internal magic strings */
  LIT_INTERNAL_MAGIC_API_INTERNAL = LIT_NON_INTERNAL_MAGIC_STRING__COUNT, /**< Used to add non-visible JS properties
                                                                           *   from the public API */
  LIT_INTERNAL_MAGIC_STRING_ITERATOR_NEXT_INDEX, /**< [[%Iterator%NextIndex]] property */
  LIT_INTERNAL_MAGIC_STRING_MAP_KEY, /**< Property key used when an object is a key in a map object */
  LIT_INTERNAL_MAGIC_STRING_ARRAY_PROTOTYPE_VALUES, /**< %ArrayProto_values% intrinsic routine */
  LIT_INTERNAL_MAGIC_STRING_TYPEDARRAY_PROTOTYPE_VALUES, /**< %TypedArray%.prototype values and [@@iterator] routine */
  LIT_INTERNAL_MAGIC_STRING_SET_PROTOTYPE_VALUES, /**< Set.prototype values, keys and [@@iterator] routines */
  LIT_INTERNAL_MAGIC_STRING_MAP_PROTOTYPE_ENTRIES, /**< Map.prototype entries and [@@iterator] routines */
  /* List of well known symbols */
  LIT_GLOBAL_SYMBOL_ASYNC_ITERATOR, /**< @@asyncIterator well known symbol */
  LIT_GLOBAL_SYMBOL__FIRST = LIT_GLOBAL_SYMBOL_ASYNC_ITERATOR, /**< first global symbol */
  LIT_GLOBAL_SYMBOL_HAS_INSTANCE, /**< @@hasInstance well known symbol */
  LIT_GLOBAL_SYMBOL_IS_CONCAT_SPREADABLE, /**< @@isConcatSpreadable well known symbol */
  LIT_GLOBAL_SYMBOL_ITERATOR, /**< @@iterator well known symbol */
  LIT_GLOBAL_SYMBOL_MATCH, /**< @@match well known symbol */
  LIT_GLOBAL_SYMBOL_REPLACE, /**< @@replace well known symbol */
  LIT_GLOBAL_SYMBOL_SEARCH, /**< @@search well known symbol */
  LIT_GLOBAL_SYMBOL_SPECIES, /**< @@species well known symbol */
  LIT_GLOBAL_SYMBOL_SPLIT, /**< @@split well known symbol */
  LIT_GLOBAL_SYMBOL_TO_PRIMITIVE, /**< @@toPrimitive well known symbol */
  LIT_GLOBAL_SYMBOL_TO_STRING_TAG, /**< @@toStringTag well known symbol */
  LIT_GLOBAL_SYMBOL_UNSCOPABLES, /**< @@unscopables well known symbol */
  LIT_GLOBAL_SYMBOL_MATCH_ALL, /**< @@matchAll well known symbol */
  LIT_GLOBAL_SYMBOL__LAST = LIT_GLOBAL_SYMBOL_MATCH_ALL, /**< last global symbol */

  LIT_INTERNAL_MAGIC_STRING_DELETED, /**< special value for deleted properties */
  LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_INIT, /**< function which initializes properties */

  LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER, /**< native pointer info associated with an object */
  LIT_INTERNAL_MAGIC_STRING_FIRST_DATA = LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER, /**< first index of special
                                                                                    *   data properties */
  LIT_INTERNAL_MAGIC_STRING_NATIVE_POINTER_WITH_REFERENCES, /**< native pointer info associated with an object
                                                             *   which contains references to other values */
  LIT_INTERNAL_MAGIC_STRING_ENVIRONMENT_RECORD, /**< dynamic environment record needed by class constructors */
  LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_COMPUTED, /**< computed class field name list */
  LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS, /**< internal collection for storing private elements */
  LIT_INTERNAL_MAGIC_STRING_CONTAINER_WEAK_REFS, /**< Weak references to the current container object */
  LIT_INTERNAL_MAGIC_STRING_WEAK_REFS, /**< Weak references to the current object */
  LIT_MAGIC_STRING__COUNT /**< number of magic strings */
} lit_magic_string_id_t;

/**
 * Properties that are need to be ignored for opfunc_set_class_attributes
 */
#define LIT_INTERNAL_MAGIC_STRING_IGNORED(id) \
  ((id) >= LIT_INTERNAL_MAGIC_STRING_CLASS_FIELD_COMPUTED && (id) <= LIT_INTERNAL_MAGIC_STRING_CLASS_PRIVATE_ELEMENTS)

/**
 * Checks whether the given id corresponds to a global symbol
 */
#define LIT_IS_GLOBAL_SYMBOL(id) ((id) >= LIT_GLOBAL_SYMBOL__FIRST && (id) <= LIT_GLOBAL_SYMBOL__LAST)

/**
 * Identifiers of implementation-defined external magic string constants
 */
typedef uint32_t lit_magic_string_ex_id_t;

uint32_t lit_get_magic_string_ex_count (void);

const lit_utf8_byte_t *lit_get_magic_string_utf8 (uint32_t id);
lit_utf8_size_t lit_get_magic_string_size (uint32_t id);

const lit_utf8_byte_t *lit_get_magic_string_ex_utf8 (uint32_t id);
lit_utf8_size_t lit_get_magic_string_ex_size (uint32_t id);

void lit_magic_strings_ex_set (const lit_utf8_byte_t *const *ex_str_items,
                               uint32_t count,
                               const lit_utf8_size_t *ex_str_sizes);

lit_magic_string_id_t lit_is_utf8_string_magic (const lit_utf8_byte_t *string_p, lit_utf8_size_t string_size);
lit_magic_string_id_t lit_is_utf8_string_pair_magic (const lit_utf8_byte_t *string1_p,
                                                     lit_utf8_size_t string1_size,
                                                     const lit_utf8_byte_t *string2_p,
                                                     lit_utf8_size_t string2_size);

lit_magic_string_ex_id_t lit_is_ex_utf8_string_magic (const lit_utf8_byte_t *string_p, lit_utf8_size_t string_size);
lit_magic_string_ex_id_t lit_is_ex_utf8_string_pair_magic (const lit_utf8_byte_t *string1_p,
                                                           lit_utf8_size_t string1_size,
                                                           const lit_utf8_byte_t *string2_p,
                                                           lit_utf8_size_t string2_size);

lit_utf8_byte_t *
lit_copy_magic_string_to_buffer (lit_magic_string_id_t id, lit_utf8_byte_t *buffer_p, lit_utf8_size_t buffer_size);

#endif /* !LIT_MAGIC_STRINGS_H */
