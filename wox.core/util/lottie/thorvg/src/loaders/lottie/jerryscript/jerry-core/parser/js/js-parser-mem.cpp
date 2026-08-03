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

#include "js-parser-internal.h"

#if JERRY_PARSER

/** \addtogroup mem Memory allocation
 * @{
 *
 * \addtogroup mem_parser Parser memory manager
 * @{
 */

/**********************************************************************/
/* Memory allocation                                                  */
/**********************************************************************/

/**
 * Allocate memory.
 *
 * @return allocated memory.
 */
void *
parser_malloc (parser_context_t *context_p, /**< context */
               size_t size) /**< size of the memory block */
{
  void *result;

  JERRY_ASSERT (size > 0);
  result = jmem_heap_alloc_block_null_on_error (size);

  if (result == NULL)
  {
    parser_raise_error (context_p, PARSER_ERR_OUT_OF_MEMORY);
  }
  return result;
} /* parser_malloc */

/**
 * Free memory allocated by parser_malloc.
 */
void 
parser_free (void *ptr, /**< pointer to free */
             size_t size) /**< size of the memory block */
{
  jmem_heap_free_block (ptr, size);
} /* parser_free */

/**
 * Allocate local memory for short term use.
 *
 * @return allocated memory.
 */
void *
parser_malloc_local (parser_context_t *context_p, /**< context */
                     size_t size) /**< size of the memory */
{
  void *result;

  JERRY_ASSERT (size > 0);
  result = jmem_heap_alloc_block (size);
  if (result == 0)
  {
    parser_raise_error (context_p, PARSER_ERR_OUT_OF_MEMORY);
  }
  return result;
} /* parser_malloc_local */

/**
 * Free memory allocated by parser_malloc_local.
 */
void
parser_free_local (void *ptr, /**< pointer to free */
                   size_t size) /**< size of the memory */
{
  jmem_heap_free_block (ptr, size);
} /* parser_free_local */

/**
 * Free the dynamically allocated buffer stored in the context
 */
void 
parser_free_allocated_buffer (parser_context_t *context_p) /**< context */
{
  if (context_p->u.allocated_buffer_p != NULL)
  {
    parser_free_local (context_p->u.allocated_buffer_p, context_p->allocated_buffer_size);
    context_p->u.allocated_buffer_p = NULL;
  }
} /* parser_free_allocated_buffer */

/**********************************************************************/
/* Parser data management functions                                   */
/**********************************************************************/

/**
 * Initialize parse data.
 */
static void
parser_data_init (parser_mem_data_t *data_p, /**< memory manager */
                  uint32_t page_size) /**< size of each page */
{
  data_p->first_p = NULL;
  data_p->last_p = NULL;
  data_p->last_position = page_size;
} /* parser_data_init */

/**
 * Free parse data.
 */
static void
parser_data_free (parser_mem_data_t *data_p, /**< memory manager */
                  uint32_t page_size) /**< size of each page */
{
  parser_mem_page_t *page_p = data_p->first_p;

  while (page_p != NULL)
  {
    parser_mem_page_t *next_p = page_p->next_p;

    parser_free (page_p, page_size);
    page_p = next_p;
  }
} /* parser_data_free */

/**********************************************************************/
/* Parser byte stream management functions                            */
/**********************************************************************/

/**
 * Initialize byte stream.
 */
void
parser_cbc_stream_init (parser_mem_data_t *data_p) /**< memory manager */
{
  parser_data_init (data_p, PARSER_CBC_STREAM_PAGE_SIZE);
} /* parser_cbc_stream_init */

/**
 * Free byte stream.
 */
void
parser_cbc_stream_free (parser_mem_data_t *data_p) /**< memory manager */
{
  parser_data_free (data_p, sizeof (parser_mem_page_t *) + PARSER_CBC_STREAM_PAGE_SIZE);
} /* parser_cbc_stream_free */

/**
 * Appends a byte at the end of the byte stream.
 */
void
parser_cbc_stream_alloc_page (parser_context_t *context_p, /**< context */
                              parser_mem_data_t *data_p) /**< memory manager */
{
  size_t size = sizeof (parser_mem_page_t *) + PARSER_CBC_STREAM_PAGE_SIZE;
  parser_mem_page_t *page_p = (parser_mem_page_t *) parser_malloc (context_p, size);

  page_p->next_p = NULL;
  data_p->last_position = 0;

  if (data_p->last_p != NULL)
  {
    data_p->last_p->next_p = page_p;
  }
  else
  {
    data_p->first_p = page_p;
  }
  data_p->last_p = page_p;
} /* parser_cbc_stream_alloc_page */

/**********************************************************************/
/* Parser list management functions                                   */
/**********************************************************************/

/**
 * Initialize parser list.
 */
void
parser_list_init (parser_list_t *list_p, /**< parser list */
                  uint32_t item_size, /**< size for each page */
                  uint32_t item_count) /**< number of items on each page */
{
  /* Align to pointer size. */
  item_size = (uint32_t) (((item_size) + sizeof (void *) - 1) & ~(sizeof (void *) - 1));
  parser_data_init (&list_p->data, item_size * item_count);
  list_p->page_size = item_size * item_count;
  list_p->item_size = item_size;
  list_p->item_count = item_count;
} /* parser_list_init */

/**
 * Free parser list.
 */
void
parser_list_free (parser_list_t *list_p) /**< parser list */
{
  parser_data_free (&list_p->data, (uint32_t) (sizeof (parser_mem_page_t *) + list_p->page_size));
} /* parser_list_free */

/**
 * Reset parser list.
 */
void
parser_list_reset (parser_list_t *list_p) /**< parser list */
{
  parser_data_init (&list_p->data, list_p->page_size);
} /* parser_list_reset */

/**
 * Allocate space for the next item.
 *
 * @return pointer to the appended item.
 */
void *
parser_list_append (parser_context_t *context_p, /**< context */
                    parser_list_t *list_p) /**< parser list */
{
  parser_mem_page_t *page_p = list_p->data.last_p;
  void *result;

  if (list_p->data.last_position + list_p->item_size > list_p->page_size)
  {
    size_t size = sizeof (parser_mem_page_t *) + list_p->page_size;

    page_p = (parser_mem_page_t *) parser_malloc (context_p, size);

    page_p->next_p = NULL;
    list_p->data.last_position = 0;

    if (list_p->data.last_p != NULL)
    {
      list_p->data.last_p->next_p = page_p;
    }
    else
    {
      list_p->data.first_p = page_p;
    }
    list_p->data.last_p = page_p;
  }

  result = page_p->bytes + list_p->data.last_position;
  list_p->data.last_position += list_p->item_size;
  return result;
} /* parser_list_append */

/**
 * Return the nth item of the list.
 *
 * @return pointer to the item.
 */
void *
parser_list_get (parser_list_t *list_p, /**< parser list */
                 size_t index) /**< item index */
{
  size_t item_count = list_p->item_count;
  parser_mem_page_t *page_p = list_p->data.first_p;

  while (index >= item_count)
  {
    JERRY_ASSERT (page_p != NULL);
    page_p = page_p->next_p;
    index -= item_count;
  }

  JERRY_ASSERT (page_p != NULL);
  JERRY_ASSERT (page_p != list_p->data.last_p || (index * list_p->item_size < list_p->data.last_position));
  return page_p->bytes + (index * list_p->item_size);
} /* parser_list_get */

/**
 * Initialize a parser list iterator.
 */
void
parser_list_iterator_init (parser_list_t *list_p, /**< parser list */
                           parser_list_iterator_t *iterator_p) /**< iterator */
{
  iterator_p->list_p = list_p;
  iterator_p->current_p = list_p->data.first_p;
  iterator_p->current_position = 0;
} /* parser_list_iterator_init */

/**
 * Next iterator step.
 *
 * @return the address of the current item, or NULL at the end.
 */
void *
parser_list_iterator_next (parser_list_iterator_t *iterator_p) /**< iterator */
{
  void *result;

  if (iterator_p->current_p == NULL)
  {
    return NULL;
  }

  result = iterator_p->current_p->bytes + iterator_p->current_position;
  iterator_p->current_position += iterator_p->list_p->item_size;

  if (iterator_p->current_p->next_p == NULL)
  {
    if (iterator_p->current_position >= iterator_p->list_p->data.last_position)
    {
      iterator_p->current_p = NULL;
      iterator_p->current_position = 0;
    }
  }
  else if (iterator_p->current_position >= iterator_p->list_p->page_size)
  {
    iterator_p->current_p = iterator_p->current_p->next_p;
    iterator_p->current_position = 0;
  }
  return result;
} /* parser_list_iterator_next */

/**********************************************************************/
/* Parser stack management functions                                  */
/**********************************************************************/

/* Stack is a reversed storage. */

/**
 * Initialize parser stack.
 */
void
parser_stack_init (parser_context_t *context_p) /**< context */
{
  parser_data_init (&context_p->stack, PARSER_STACK_PAGE_SIZE);
  context_p->free_page_p = NULL;
} /* parser_stack_init */

/**
 * Free parser stack.
 */
void
parser_stack_free (parser_context_t *context_p) /**< context */
{
  parser_data_free (&context_p->stack, sizeof (parser_mem_page_t *) + PARSER_STACK_PAGE_SIZE);

  if (context_p->free_page_p != NULL)
  {
    parser_free (context_p->free_page_p, sizeof (parser_mem_page_t *) + PARSER_STACK_PAGE_SIZE);
  }
} /* parser_stack_free */

/**
 * Pushes an uint8_t value onto the stack.
 */
void
parser_stack_push_uint8 (parser_context_t *context_p, /**< context */
                         uint8_t uint8_value) /**< value pushed onto the stack */
{
  parser_mem_page_t *page_p = context_p->stack.first_p;

  /* This assert might trigger false positive valgrind errors, when
   * parser_stack_push() pushes not fully initialized structures.
   * More precisely when the last byte of the structure is uninitialized. */
  JERRY_ASSERT (page_p == NULL || context_p->stack_top_uint8 == page_p->bytes[context_p->stack.last_position - 1]);

  if (context_p->stack.last_position >= PARSER_STACK_PAGE_SIZE)
  {
    if (context_p->free_page_p != NULL)
    {
      page_p = context_p->free_page_p;
      context_p->free_page_p = NULL;
    }
    else
    {
      size_t size = sizeof (parser_mem_page_t *) + PARSER_STACK_PAGE_SIZE;
      page_p = (parser_mem_page_t *) parser_malloc (context_p, size);
    }

    page_p->next_p = context_p->stack.first_p;
    context_p->stack.last_position = 0;
    context_p->stack.first_p = page_p;
  }

  page_p->bytes[context_p->stack.last_position++] = uint8_value;
  context_p->stack_top_uint8 = uint8_value;
} /* parser_stack_push_uint8 */

/**
 * Pops the last uint8_t value from the stack.
 */
void
parser_stack_pop_uint8 (parser_context_t *context_p) /**< context */
{
  parser_mem_page_t *page_p = context_p->stack.first_p;

  JERRY_ASSERT (page_p != NULL && context_p->stack_top_uint8 == page_p->bytes[context_p->stack.last_position - 1]);

  context_p->stack.last_position--;

  if (context_p->stack.last_position == 0)
  {
    context_p->stack.first_p = page_p->next_p;
    context_p->stack.last_position = PARSER_STACK_PAGE_SIZE;

    if (context_p->free_page_p == NULL)
    {
      context_p->free_page_p = page_p;
    }
    else
    {
      parser_free (page_p, sizeof (parser_mem_page_t *) + PARSER_STACK_PAGE_SIZE);
    }

    page_p = context_p->stack.first_p;

    JERRY_ASSERT (page_p != NULL);
  }

  context_p->stack_top_uint8 = page_p->bytes[context_p->stack.last_position - 1];
} /* parser_stack_pop_uint8 */

/**
 * Change last byte of the stack.
 */
void
parser_stack_change_last_uint8 (parser_context_t *context_p, /**< context */
                                uint8_t new_value) /**< new value */
{
  parser_mem_page_t *page_p = context_p->stack.first_p;

  JERRY_ASSERT (page_p != NULL && context_p->stack_top_uint8 == page_p->bytes[context_p->stack.last_position - 1]);

  page_p->bytes[context_p->stack.last_position - 1] = new_value;
  context_p->stack_top_uint8 = new_value;
} /* parser_stack_change_last_uint8 */

/**
 * Get the uint8 value before the top of the stack.
 *
 * Pointer to the uint8 value
 */
uint8_t *
parser_stack_get_prev_uint8 (parser_context_t *context_p) /**< context */
{
  parser_mem_page_t *page_p = context_p->stack.first_p;

  JERRY_ASSERT (page_p != NULL && (context_p->stack.last_position >= 2 || page_p->next_p != NULL));

  if (context_p->stack.last_position >= 2)
  {
    return page_p->bytes + (context_p->stack.last_position - 2);
  }

  return page_p->next_p->bytes + (PARSER_STACK_PAGE_SIZE - 1);
} /* parser_stack_get_prev_uint8 */

/**
 * Pushes an uint16_t value onto the stack.
 */
void
parser_stack_push_uint16 (parser_context_t *context_p, /**< context */
                          uint16_t uint16_value) /**< value pushed onto the stack */
{
  if (context_p->stack.last_position + 2 <= PARSER_STACK_PAGE_SIZE)
  {
    parser_mem_page_t *page_p = context_p->stack.first_p;

    JERRY_ASSERT (page_p != NULL && context_p->stack_top_uint8 == page_p->bytes[context_p->stack.last_position - 1]);

    page_p->bytes[context_p->stack.last_position++] = (uint8_t) (uint16_value >> 8);
    page_p->bytes[context_p->stack.last_position++] = (uint8_t) uint16_value;
    context_p->stack_top_uint8 = (uint8_t) uint16_value;
  }
  else
  {
    parser_stack_push_uint8 (context_p, (uint8_t) (uint16_value >> 8));
    parser_stack_push_uint8 (context_p, (uint8_t) uint16_value);
  }
} /* parser_stack_push_uint16 */

/**
 * Pops the last uint16_t value from the stack.
 *
 * @return the value popped from the stack.
 */
uint16_t
parser_stack_pop_uint16 (parser_context_t *context_p) /**< context */
{
  uint32_t value = context_p->stack_top_uint8;

  if (context_p->stack.last_position >= 3)
  {
    parser_mem_page_t *page_p = context_p->stack.first_p;

    JERRY_ASSERT (page_p != NULL && context_p->stack_top_uint8 == page_p->bytes[context_p->stack.last_position - 1]);

    value |= ((uint32_t) page_p->bytes[context_p->stack.last_position - 2]) << 8;
    context_p->stack_top_uint8 = page_p->bytes[context_p->stack.last_position - 3];
    context_p->stack.last_position -= 2;
  }
  else
  {
    parser_stack_pop_uint8 (context_p);
    value |= ((uint32_t) context_p->stack_top_uint8) << 8;
    parser_stack_pop_uint8 (context_p);
  }
  return (uint16_t) value;
} /* parser_stack_pop_uint16 */

/**
 * Pushes a data onto the stack.
 */
void
parser_stack_push (parser_context_t *context_p, /**< context */
                   const void *data_p, /**< data pushed onto the stack */
                   uint32_t length) /**< length of the data */
{
  uint32_t fragment_length = PARSER_STACK_PAGE_SIZE - context_p->stack.last_position;
  const uint8_t *bytes_p = (const uint8_t *) data_p;
  parser_mem_page_t *page_p;

  JERRY_ASSERT (length < PARSER_STACK_PAGE_SIZE && length > 0);

  context_p->stack_top_uint8 = bytes_p[length - 1];

  if (fragment_length > 0)
  {
    /* Fill the remaining bytes. */
    if (fragment_length > length)
    {
      fragment_length = length;
    }

    memcpy (context_p->stack.first_p->bytes + context_p->stack.last_position, bytes_p, fragment_length);

    if (fragment_length == length)
    {
      context_p->stack.last_position += length;
      return;
    }

    bytes_p += fragment_length;
    length -= fragment_length;
  }

  if (context_p->free_page_p != NULL)
  {
    page_p = context_p->free_page_p;
    context_p->free_page_p = NULL;
  }
  else
  {
    size_t size = sizeof (parser_mem_page_t *) + PARSER_STACK_PAGE_SIZE;

    page_p = (parser_mem_page_t *) parser_malloc (context_p, size);
  }

  page_p->next_p = context_p->stack.first_p;

  context_p->stack.first_p = page_p;

  memcpy (page_p->bytes, bytes_p, length);
  context_p->stack.last_position = length;
} /* parser_stack_push */

/**
 * Pop bytes from the top of the stack.
 */
void
parser_stack_pop (parser_context_t *context_p, /**< context */
                  void *data_p, /**< destination buffer, can be NULL */
                  uint32_t length) /**< length of the data */
{
  uint8_t *bytes_p = (uint8_t *) data_p;
  parser_mem_page_t *page_p = context_p->stack.first_p;

  JERRY_ASSERT (length < PARSER_STACK_PAGE_SIZE && length > 0);

  if (context_p->stack.last_position > length)
  {
    context_p->stack.last_position -= length;
    context_p->stack_top_uint8 = page_p->bytes[context_p->stack.last_position - 1];

    if (bytes_p != NULL)
    {
      memcpy (bytes_p, context_p->stack.first_p->bytes + context_p->stack.last_position, length);
    }
    return;
  }

  JERRY_ASSERT (page_p->next_p != NULL);

  length -= context_p->stack.last_position;

  if (bytes_p != NULL)
  {
    memcpy (bytes_p + length, page_p->bytes, context_p->stack.last_position);
  }

  context_p->stack.first_p = page_p->next_p;
  context_p->stack.last_position = PARSER_STACK_PAGE_SIZE - length;
  context_p->stack_top_uint8 = page_p->next_p->bytes[context_p->stack.last_position - 1];

  if (bytes_p != NULL && length > 0)
  {
    memcpy (bytes_p, page_p->next_p->bytes + context_p->stack.last_position, length);
  }

  JERRY_ASSERT (context_p->stack.last_position > 0);

  if (context_p->free_page_p == NULL)
  {
    context_p->free_page_p = page_p;
  }
  else
  {
    parser_free (page_p, sizeof (parser_mem_page_t *) + PARSER_STACK_PAGE_SIZE);
  }
} /* parser_stack_pop */

/**
 * Initialize stack iterator.
 */
void
parser_stack_iterator_init (parser_context_t *context_p, /**< context */
                            parser_stack_iterator_t *iterator) /**< iterator */
{
  iterator->current_p = context_p->stack.first_p;
  iterator->current_position = context_p->stack.last_position;
} /* parser_stack_iterator_init */

/**
 * Read the next byte from the stack.
 *
 * @return byte
 */
uint8_t
parser_stack_iterator_read_uint8 (parser_stack_iterator_t *iterator) /**< iterator */
{
  JERRY_ASSERT (iterator->current_position > 0 && iterator->current_position <= PARSER_STACK_PAGE_SIZE);
  return iterator->current_p->bytes[iterator->current_position - 1];
} /* parser_stack_iterator_read_uint8 */

/**
 * Skip the next n bytes of the stack.
 */
void
parser_stack_iterator_skip (parser_stack_iterator_t *iterator, /**< iterator */
                            size_t length) /**< number of skipped bytes */
{
  JERRY_ASSERT (length < PARSER_STACK_PAGE_SIZE && length > 0);

  if (length < iterator->current_position)
  {
    iterator->current_position -= length;
  }
  else
  {
    iterator->current_position = PARSER_STACK_PAGE_SIZE - (length - iterator->current_position);
    iterator->current_p = iterator->current_p->next_p;
  }
} /* parser_stack_iterator_skip */

/**
 * Read bytes from the stack.
 */
void
parser_stack_iterator_read (parser_stack_iterator_t *iterator, /**< iterator */
                            void *data_p, /**< destination buffer */
                            size_t length) /**< length of the data */
{
  uint8_t *bytes_p = (uint8_t *) data_p;

  JERRY_ASSERT (length < PARSER_STACK_PAGE_SIZE && length > 0);

  if (length <= iterator->current_position)
  {
    memcpy (bytes_p, iterator->current_p->bytes + iterator->current_position - length, length);
  }
  else
  {
    JERRY_ASSERT (iterator->current_p->next_p != NULL);

    length -= iterator->current_position;
    memcpy (bytes_p + length, iterator->current_p->bytes, iterator->current_position);
    memcpy (bytes_p, iterator->current_p->next_p->bytes + PARSER_STACK_PAGE_SIZE - length, length);
  }
} /* parser_stack_iterator_read */

/**
 * Write bytes onto the stack.
 */
void
parser_stack_iterator_write (parser_stack_iterator_t *iterator, /**< iterator */
                             const void *data_p, /**< destination buffer */
                             size_t length) /**< length of the data */
{
  const uint8_t *bytes_p = (const uint8_t *) data_p;

  JERRY_ASSERT (length < PARSER_STACK_PAGE_SIZE && length > 0);

  if (length <= iterator->current_position)
  {
    memcpy (iterator->current_p->bytes + iterator->current_position - length, bytes_p, length);
  }
  else
  {
    JERRY_ASSERT (iterator->current_p->next_p != NULL);

    length -= iterator->current_position;
    memcpy (iterator->current_p->bytes, bytes_p + length, iterator->current_position);
    memcpy (iterator->current_p->next_p->bytes + PARSER_STACK_PAGE_SIZE - length, bytes_p, length);
  }
} /* parser_stack_iterator_write */

/**
 * @}
 * @}
 */

#endif /* JERRY_PARSER */
