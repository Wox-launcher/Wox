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

#include "jerry-config.h"
#include "lit-magic-strings.h"

#include "jcontext.h"
#include "lit-strings.h"

/**
 * Maximum number of external magic strings that can be registered.
 */
#define LIT_EXTERNAL_MAGIC_STRING_LIMIT (UINT32_MAX / 2)

/**
 * Get number of external magic strings
 *
 * @return number of the strings, if there were registered,
 *         zero - otherwise.
 */
uint32_t
lit_get_magic_string_ex_count (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  return JERRY_CONTEXT (lit_magic_string_ex_count);
} /* lit_get_magic_string_ex_count */

/**
 * Get specified magic string as zero-terminated string
 *
 * @return pointer to zero-terminated magic string
 */
const lit_utf8_byte_t *
lit_get_magic_string_utf8 (uint32_t id) /**< magic string id */
{
  static const lit_utf8_byte_t *const lit_magic_strings[] JERRY_ATTR_CONST_DATA = {
/** @cond doxygen_suppress */
#define LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE(size, id)
#define LIT_MAGIC_STRING_DEF(id, utf8_string) (const lit_utf8_byte_t *) utf8_string,
#include "lit-magic-strings.inc.h"
#undef LIT_MAGIC_STRING_DEF
#undef LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE
    /** @endcond */
  };

  JERRY_ASSERT (id < LIT_NON_INTERNAL_MAGIC_STRING__COUNT);

  return lit_magic_strings[id];
} /* lit_get_magic_string_utf8 */

/**
 * Get size of specified magic string
 *
 * @return size in bytes
 */
lit_utf8_size_t
lit_get_magic_string_size (uint32_t id) /**< magic string id */
{
  static const lit_magic_size_t lit_magic_string_sizes[] JERRY_ATTR_CONST_DATA = {
/** @cond doxygen_suppress */
#define LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE(size, id)
#define LIT_MAGIC_STRING_DEF(id, utf8_string) sizeof (utf8_string) - 1,
#include "lit-magic-strings.inc.h"
#undef LIT_MAGIC_STRING_DEF
#undef LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE
    /** @endcond */
  };

  JERRY_ASSERT (id < LIT_NON_INTERNAL_MAGIC_STRING__COUNT);

  return lit_magic_string_sizes[id];
} /* lit_get_magic_string_size */

/**
 * Get the block start element with the given size from
 * the list of ECMA and implementation-defined magic string constants
 *
 * @return magic string id
 */
static lit_magic_string_id_t
lit_get_magic_string_size_block_start (lit_utf8_size_t size) /**< magic string size */
{
  static const lit_magic_string_id_t lit_magic_string_size_block_starts[] JERRY_ATTR_CONST_DATA = {
/** @cond doxygen_suppress */
#define LIT_MAGIC_STRING_DEF(id, utf8_string)
#define LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE(size, id) id,
#include "lit-magic-strings.inc.h"
    LIT_NON_INTERNAL_MAGIC_STRING__COUNT
#undef LIT_MAGIC_STRING_DEF
#undef LIT_MAGIC_STRING_FIRST_STRING_WITH_SIZE
    /** @endcond */
  };

  JERRY_ASSERT (size <= (sizeof (lit_magic_string_size_block_starts) / sizeof (lit_magic_string_id_t)));

  return lit_magic_string_size_block_starts[size];
} /* lit_get_magic_string_size_block_start */

/**
 * Get specified magic string as zero-terminated string from external table
 *
 * @return pointer to zero-terminated magic string
 */
const lit_utf8_byte_t *
lit_get_magic_string_ex_utf8 (uint32_t id) /**< extern magic string id */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (JERRY_CONTEXT (lit_magic_string_ex_array) && id < JERRY_CONTEXT (lit_magic_string_ex_count));

  return JERRY_CONTEXT (lit_magic_string_ex_array)[id];
} /* lit_get_magic_string_ex_utf8 */

/**
 * Get size of specified external magic string
 *
 * @return size in bytes
 */
lit_utf8_size_t
lit_get_magic_string_ex_size (uint32_t id) /**< external magic string id */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  return JERRY_CONTEXT (lit_magic_string_ex_sizes)[id];
} /* lit_get_magic_string_ex_size */

/**
 * Register external magic strings
 */
void
lit_magic_strings_ex_set (const lit_utf8_byte_t *const *ex_str_items, /**< character arrays, representing
                                                                       *   external magic strings' contents */
                          uint32_t count, /**< number of the strings */
                          const lit_utf8_size_t *ex_str_sizes) /**< sizes of the strings */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (ex_str_items != NULL);
  JERRY_ASSERT (count > 0);
  JERRY_ASSERT (ex_str_sizes != NULL);

  JERRY_ASSERT (JERRY_CONTEXT (lit_magic_string_ex_array) == NULL);
  JERRY_ASSERT (JERRY_CONTEXT (lit_magic_string_ex_count) == 0);
  JERRY_ASSERT (JERRY_CONTEXT (lit_magic_string_ex_sizes) == NULL);

  /* Limit the number of external magic strings */
  if (count > LIT_EXTERNAL_MAGIC_STRING_LIMIT)
  {
    count = LIT_EXTERNAL_MAGIC_STRING_LIMIT;
  }

  /* Set external magic strings information */
  JERRY_CONTEXT (lit_magic_string_ex_array) = ex_str_items;
  JERRY_CONTEXT (lit_magic_string_ex_count) = count;
  JERRY_CONTEXT (lit_magic_string_ex_sizes) = ex_str_sizes;

#ifndef JERRY_NDEBUG
  for (lit_magic_string_ex_id_t id = (lit_magic_string_ex_id_t) 0; id < JERRY_CONTEXT (lit_magic_string_ex_count);
       id = (lit_magic_string_ex_id_t) (id + 1))
  {
    lit_utf8_size_t string_size = JERRY_CONTEXT (lit_magic_string_ex_sizes)[id];

    /**
     * Check whether the strings are sorted by size and lexicographically,
     * e.g., "Bb" < "aa" < "aaa" < "xyz0".
     */
    if (id > 0)
    {
      const lit_magic_string_ex_id_t prev_id = id - 1;
      const lit_utf8_size_t prev_string_size = lit_get_magic_string_ex_size (prev_id);
      JERRY_ASSERT (lit_is_valid_cesu8_string (lit_get_magic_string_ex_utf8 (id), string_size));
      JERRY_ASSERT (prev_string_size <= string_size);

      if (prev_string_size == string_size)
      {
        const lit_utf8_byte_t *prev_ex_string_p = lit_get_magic_string_ex_utf8 (prev_id);
        const lit_utf8_byte_t *curr_ex_string_p = lit_get_magic_string_ex_utf8 (id);
        JERRY_ASSERT (memcmp (prev_ex_string_p, curr_ex_string_p, string_size) < 0);
      }
    }
  }
#endif /* !JERRY_NDEBUG */
} /* lit_magic_strings_ex_set */

/**
 * Returns the magic string id of the argument string if it is available.
 *
 * @return id - if magic string id is found,
 *         LIT_MAGIC_STRING__COUNT - otherwise.
 */
lit_magic_string_id_t
lit_is_utf8_string_magic (const lit_utf8_byte_t *string_p, /**< utf-8 string */
                          lit_utf8_size_t string_size) /**< string size in bytes */
{
  if (string_size > lit_get_magic_string_size (LIT_NON_INTERNAL_MAGIC_STRING__COUNT - 1))
  {
    return LIT_MAGIC_STRING__COUNT;
  }

  /**< The string must be in this id range. */
  lit_utf8_size_t first = lit_get_magic_string_size_block_start (string_size);
  lit_utf8_size_t last = lit_get_magic_string_size_block_start (string_size + 1);

  while (first < last)
  {
    lit_utf8_size_t middle = ((first + last) / 2); /**< mid point of search */
    int compare = memcmp (lit_get_magic_string_utf8 ((lit_magic_string_id_t) middle), string_p, string_size);

    if (compare == 0)
    {
      return (lit_magic_string_id_t) middle;
    }
    else if (compare > 0)
    {
      last = middle;
    }
    else
    {
      first = middle + 1;
    }
  }

  return LIT_MAGIC_STRING__COUNT;
} /* lit_is_utf8_string_magic */

/**
 * Returns the magic string id of the argument string pair if it is available.
 *
 * @return id - if magic string id is found,
 *         LIT_MAGIC_STRING__COUNT - otherwise.
 */
lit_magic_string_id_t
lit_is_utf8_string_pair_magic (const lit_utf8_byte_t *string1_p, /**< first utf-8 string */
                               lit_utf8_size_t string1_size, /**< first string size in bytes */
                               const lit_utf8_byte_t *string2_p, /**< second utf-8 string */
                               lit_utf8_size_t string2_size) /**< second string size in bytes */
{
  lit_utf8_size_t total_string_size = string1_size + string2_size;

  if (total_string_size > lit_get_magic_string_size (LIT_NON_INTERNAL_MAGIC_STRING__COUNT - 1))
  {
    return LIT_MAGIC_STRING__COUNT;
  }

  /**< The string must be in this id range. */
  lit_utf8_size_t first = lit_get_magic_string_size_block_start (total_string_size);
  lit_utf8_size_t last = lit_get_magic_string_size_block_start (total_string_size + 1);

  while (first < last)
  {
    lit_utf8_size_t middle = ((first + last) / 2); /**< mid point of search */
    const lit_utf8_byte_t *middle_string_p = lit_get_magic_string_utf8 ((lit_magic_string_id_t) middle);

    int compare = memcmp (middle_string_p, string1_p, string1_size);

    if (compare == 0)
    {
      compare = memcmp (middle_string_p + string1_size, string2_p, string2_size);
    }

    if (compare == 0)
    {
      return (lit_magic_string_id_t) middle;
    }
    else if (compare > 0)
    {
      last = middle;
    }
    else
    {
      first = middle + 1;
    }
  }

  return LIT_MAGIC_STRING__COUNT;
} /* lit_is_utf8_string_pair_magic */

/**
 * Returns the ex magic string id of the argument string if it is available.
 *
 * @return id - if magic string id is found,
 *         lit_get_magic_string_ex_count () - otherwise.
 */
lit_magic_string_ex_id_t
lit_is_ex_utf8_string_magic (const lit_utf8_byte_t *string_p, /**< utf-8 string */
                             lit_utf8_size_t string_size) /**< string size in bytes */
{
  const uint32_t magic_string_ex_count = lit_get_magic_string_ex_count ();

  if (magic_string_ex_count == 0 || string_size > lit_get_magic_string_ex_size (magic_string_ex_count - 1))
  {
    return (lit_magic_string_ex_id_t) magic_string_ex_count;
  }

  lit_magic_string_ex_id_t first = 0;
  lit_magic_string_ex_id_t last = (lit_magic_string_ex_id_t) magic_string_ex_count;

  while (first < last)
  {
    const lit_magic_string_ex_id_t middle = (first + last) / 2;
    const lit_utf8_byte_t *ext_string_p = lit_get_magic_string_ex_utf8 (middle);
    const lit_utf8_size_t ext_string_size = lit_get_magic_string_ex_size (middle);

    if (string_size == ext_string_size)
    {
      const int string_compare = memcmp (ext_string_p, string_p, string_size);

      if (string_compare == 0)
      {
        return middle;
      }
      else if (string_compare < 0)
      {
        first = middle + 1;
      }
      else
      {
        last = middle;
      }
    }
    else if (string_size > ext_string_size)
    {
      first = middle + 1;
    }
    else
    {
      last = middle;
    }
  }

  return (lit_magic_string_ex_id_t) magic_string_ex_count;
} /* lit_is_ex_utf8_string_magic */

/**
 * Returns the ex magic string id of the argument string pair if it is available.
 *
 * @return id - if magic string id is found,
 *         lit_get_magic_string_ex_count () - otherwise.
 */
lit_magic_string_ex_id_t
lit_is_ex_utf8_string_pair_magic (const lit_utf8_byte_t *string1_p, /**< first utf-8 string */
                                  lit_utf8_size_t string1_size, /**< first string size in bytes */
                                  const lit_utf8_byte_t *string2_p, /**< second utf-8 string */
                                  lit_utf8_size_t string2_size) /**< second string size in bytes */
{
  const uint32_t magic_string_ex_count = lit_get_magic_string_ex_count ();
  const lit_utf8_size_t total_string_size = string1_size + string2_size;

  if (magic_string_ex_count == 0 || total_string_size > lit_get_magic_string_ex_size (magic_string_ex_count - 1))
  {
    return (lit_magic_string_ex_id_t) magic_string_ex_count;
  }

  lit_magic_string_ex_id_t first = 0;
  lit_magic_string_ex_id_t last = (lit_magic_string_ex_id_t) magic_string_ex_count;

  while (first < last)
  {
    const lit_magic_string_ex_id_t middle = (first + last) / 2;
    const lit_utf8_byte_t *ext_string_p = lit_get_magic_string_ex_utf8 (middle);
    const lit_utf8_size_t ext_string_size = lit_get_magic_string_ex_size (middle);

    if (total_string_size == ext_string_size)
    {
      int string_compare = memcmp (ext_string_p, string1_p, string1_size);

      if (string_compare == 0)
      {
        string_compare = memcmp (ext_string_p + string1_size, string2_p, string2_size);
      }

      if (string_compare == 0)
      {
        return middle;
      }
      else if (string_compare < 0)
      {
        first = middle + 1;
      }
      else
      {
        last = middle;
      }
    }
    else if (total_string_size > ext_string_size)
    {
      first = middle + 1;
    }
    else
    {
      last = middle;
    }
  }

  return (lit_magic_string_ex_id_t) magic_string_ex_count;
} /* lit_is_ex_utf8_string_pair_magic */

/**
 * Copy magic string to buffer
 *
 * Warning:
 *         the routine requires that buffer size is enough
 *
 * @return pointer to the byte next to the last copied in the buffer
 */
lit_utf8_byte_t *
lit_copy_magic_string_to_buffer (lit_magic_string_id_t id, /**< magic string id */
                                 lit_utf8_byte_t *buffer_p, /**< destination buffer */
                                 lit_utf8_size_t buffer_size) /**< size of buffer */
{
  const lit_utf8_byte_t *magic_string_bytes_p = lit_get_magic_string_utf8 (id);
  lit_utf8_size_t magic_string_bytes_count = lit_get_magic_string_size (id);

  const lit_utf8_byte_t *str_iter_p = magic_string_bytes_p;
  lit_utf8_byte_t *buf_iter_p = buffer_p;

  while (magic_string_bytes_count--)
  {
    *buf_iter_p++ = *str_iter_p++;
  }

  return buf_iter_p;
} /* lit_copy_magic_string_to_buffer */
