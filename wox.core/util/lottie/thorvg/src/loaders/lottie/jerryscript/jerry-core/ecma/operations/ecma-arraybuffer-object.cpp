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

#include "ecma-arraybuffer-object.h"

#include "ecma-builtin-helpers.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"
#include "ecma-shared-arraybuffer-object.h"
#include "ecma-typedarray-object.h"

#include "jcontext.h"

#if JERRY_BUILTIN_TYPEDARRAY

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmaarraybufferobject ECMA ArrayBuffer object related routines
 * @{
 */

/**
 * Creating ArrayBuffer objects with a buffer after the arraybuffer header
 *
 * @return new ArrayBuffer object
 */
ecma_object_t *
ecma_arraybuffer_create_object (uint8_t type, /**< type of the arraybuffer */
                                uint32_t length) /**< length of the arraybuffer */
{
  ecma_builtin_id_t prototype_id;

#if JERRY_BUILTIN_SHAREDARRAYBUFFER
  JERRY_ASSERT (type == ECMA_OBJECT_CLASS_ARRAY_BUFFER || type == ECMA_OBJECT_CLASS_SHARED_ARRAY_BUFFER);

  prototype_id = (type == ECMA_OBJECT_CLASS_ARRAY_BUFFER ? ECMA_BUILTIN_ID_ARRAYBUFFER_PROTOTYPE
                                                         : ECMA_BUILTIN_ID_SHARED_ARRAYBUFFER_PROTOTYPE);
#else /* !JERRY_BUILTIN_SHAREDARRAYBUFFER */
  JERRY_ASSERT (type == ECMA_OBJECT_CLASS_ARRAY_BUFFER);

  prototype_id = ECMA_BUILTIN_ID_ARRAYBUFFER_PROTOTYPE;
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */

  ecma_object_t *object_p = ecma_create_object (ecma_builtin_get (prototype_id),
                                                sizeof (ecma_extended_object_t) + length,
                                                ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  ext_object_p->u.cls.type = type;
  ext_object_p->u.cls.u1.array_buffer_flags = ECMA_ARRAYBUFFER_ALLOCATED;
  ext_object_p->u.cls.u3.length = length;

  memset ((uint8_t *) (ext_object_p + 1), 0, length);
  return object_p;
} /* ecma_arraybuffer_create_object */

/**
 * Creating ArrayBuffer objects with a pointer to its buffer
 *
 * @return new ArrayBuffer object
 */
ecma_object_t *
ecma_arraybuffer_create_object_with_buffer (uint8_t type, /**< type of the arraybuffer */
                                            uint32_t length)
{
  ecma_builtin_id_t prototype_id;

#if JERRY_BUILTIN_SHAREDARRAYBUFFER
  JERRY_ASSERT (type == ECMA_OBJECT_CLASS_ARRAY_BUFFER || type == ECMA_OBJECT_CLASS_SHARED_ARRAY_BUFFER);

  prototype_id = (type == ECMA_OBJECT_CLASS_ARRAY_BUFFER ? ECMA_BUILTIN_ID_ARRAYBUFFER_PROTOTYPE
                                                         : ECMA_BUILTIN_ID_SHARED_ARRAYBUFFER_PROTOTYPE);
#else /* !JERRY_BUILTIN_SHAREDARRAYBUFFER */
  JERRY_ASSERT (type == ECMA_OBJECT_CLASS_ARRAY_BUFFER);

  prototype_id = ECMA_BUILTIN_ID_ARRAYBUFFER_PROTOTYPE;
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */

  ecma_object_t *object_p =
    ecma_create_object (ecma_builtin_get (prototype_id), sizeof (ecma_arraybuffer_pointer_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_arraybuffer_pointer_t *arraybuffer_pointer_p = (ecma_arraybuffer_pointer_t *) object_p;
  arraybuffer_pointer_p->extended_object.u.cls.type = type;
  arraybuffer_pointer_p->extended_object.u.cls.u1.array_buffer_flags = ECMA_ARRAYBUFFER_HAS_POINTER;
  arraybuffer_pointer_p->extended_object.u.cls.u3.length = length;

  arraybuffer_pointer_p->buffer_p = NULL;
  arraybuffer_pointer_p->arraybuffer_user_p = NULL;

  return object_p;
} /* ecma_arraybuffer_create_object_with_buffer */

/**
 * Creating ArrayBuffer objects based on the array length
 *
 * @return new ArrayBuffer object
 */
ecma_object_t *
ecma_arraybuffer_new_object (uint32_t length) /**< length of the arraybuffer */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  if (length > JERRY_CONTEXT (arraybuffer_compact_allocation_limit))
  {
    return ecma_arraybuffer_create_object_with_buffer (ECMA_OBJECT_CLASS_ARRAY_BUFFER, length);
  }

  return ecma_arraybuffer_create_object (ECMA_OBJECT_CLASS_ARRAY_BUFFER, length);
} /* ecma_arraybuffer_new_object */

/**
 * Allocate a backing store for an array buffer.
 *
 * @return buffer pointer on success,
 *         NULL otherwise
 */
ecma_value_t
ecma_arraybuffer_allocate_buffer (ecma_object_t *arraybuffer_p) /**< ArrayBuffer object */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (!(ECMA_ARRAYBUFFER_GET_FLAGS (arraybuffer_p) & ECMA_ARRAYBUFFER_ALLOCATED));

  ecma_extended_object_t *extended_object_p = (ecma_extended_object_t *) arraybuffer_p;

  if (ECMA_ARRAYBUFFER_GET_FLAGS (arraybuffer_p) & ECMA_ARRAYBUFFER_DETACHED)
  {
    extended_object_p->u.cls.u1.array_buffer_flags |= ECMA_ARRAYBUFFER_ALLOCATED;
    return ECMA_VALUE_UNDEFINED;
  }

  uint32_t arraybuffer_length = extended_object_p->u.cls.u3.length;
  ecma_arraybuffer_pointer_t *arraybuffer_pointer_p = (ecma_arraybuffer_pointer_t *) arraybuffer_p;
  jerry_arraybuffer_allocate_cb_t arraybuffer_allocate_callback = JERRY_CONTEXT (arraybuffer_allocate_callback);
  uint8_t *buffer_p;

  if (arraybuffer_allocate_callback != NULL)
  {
    jerry_arraybuffer_type_t type = JERRY_ARRAYBUFFER_TYPE_ARRAYBUFFER;

#if JERRY_BUILTIN_SHAREDARRAYBUFFER
    if (extended_object_p->u.cls.type == ECMA_OBJECT_CLASS_SHARED_ARRAY_BUFFER)
    {
      type = JERRY_ARRAYBUFFER_TYPE_SHARED_ARRAYBUFFER;
    }
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */

    buffer_p = arraybuffer_allocate_callback (type,
                                              arraybuffer_length,
                                              &arraybuffer_pointer_p->arraybuffer_user_p,
                                              JERRY_CONTEXT (arraybuffer_allocate_callback_user_p));
  }
  else
  {
    buffer_p = (uint8_t *) jmem_heap_alloc_block_null_on_error (arraybuffer_length);
  }

  if (buffer_p == NULL)
  {
    return ecma_raise_range_error (ECMA_ERR_ALLOCATE_ARRAY_BUFFER);
  }

  arraybuffer_pointer_p->buffer_p = buffer_p;
  extended_object_p->u.cls.u1.array_buffer_flags |= ECMA_ARRAYBUFFER_ALLOCATED;

  memset (buffer_p, 0, arraybuffer_length);
  return ECMA_VALUE_UNDEFINED;
} /* ecma_arraybuffer_allocate_buffer */

/**
 * Allocate a backing store for an array buffer, throws an error if the allocation fails.
 *
 * @return ECMA_VALUE_UNDEFINED on success,
 *         ECMA_VALUE_ERROR otherwise
 */
ecma_value_t
ecma_arraybuffer_allocate_buffer_throw (ecma_object_t *arraybuffer_p)
{
  JERRY_ASSERT (!(ECMA_ARRAYBUFFER_GET_FLAGS (arraybuffer_p) & ECMA_ARRAYBUFFER_ALLOCATED));

  return ecma_arraybuffer_allocate_buffer (arraybuffer_p);
} /* ecma_arraybuffer_allocate_buffer_throw */

/**
 * Release the backing store allocated by an array buffer.
 */
void
ecma_arraybuffer_release_buffer (ecma_object_t *arraybuffer_p) /**< ArrayBuffer object */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (ecma_object_class_is (arraybuffer_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER)
                || ecma_object_is_shared_arraybuffer (arraybuffer_p));

  jerry_arraybuffer_free_cb_t free_callback = JERRY_CONTEXT (arraybuffer_free_callback);
  ecma_arraybuffer_pointer_t *arraybuffer_pointer_p = (ecma_arraybuffer_pointer_t *) arraybuffer_p;

  if (arraybuffer_pointer_p->buffer_p == NULL)
  {
    return;
  }

  uint32_t arraybuffer_length = arraybuffer_pointer_p->extended_object.u.cls.u3.length;

  if (free_callback == NULL)
  {
    jmem_heap_free_block (arraybuffer_pointer_p->buffer_p, arraybuffer_length);
    return;
  }

  jerry_arraybuffer_type_t type = JERRY_ARRAYBUFFER_TYPE_ARRAYBUFFER;

#if JERRY_BUILTIN_SHAREDARRAYBUFFER
  if (arraybuffer_pointer_p->extended_object.u.cls.type == ECMA_OBJECT_CLASS_SHARED_ARRAY_BUFFER)
  {
    type = JERRY_ARRAYBUFFER_TYPE_SHARED_ARRAYBUFFER;
  }
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */

  free_callback (type,
                 (uint8_t*) arraybuffer_pointer_p->buffer_p,
                 arraybuffer_length,
                 arraybuffer_pointer_p->arraybuffer_user_p,
                 JERRY_CONTEXT (arraybuffer_allocate_callback_user_p));
} /* ecma_arraybuffer_release_buffer */

/**
 * ArrayBuffer object creation operation.
 *
 * See also: ES2015 24.1.1.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_create_arraybuffer_object (const ecma_value_t *arguments_list_p, /**< list of arguments that
                                                                          *   are passed to String constructor */
                                   uint32_t arguments_list_len) /**< length of the arguments' list */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  ecma_object_t *proto_p = ecma_op_get_prototype_from_constructor (JERRY_CONTEXT (current_new_target_p),
                                                                   ECMA_BUILTIN_ID_ARRAYBUFFER_PROTOTYPE);

  if (proto_p == NULL)
  {
    return ECMA_VALUE_ERROR;
  }

  ecma_number_t length_num = 0;

  if (arguments_list_len > 0)
  {
    if (ecma_is_value_number (arguments_list_p[0]))
    {
      length_num = ecma_get_number_from_value (arguments_list_p[0]);
    }
    else
    {
      ecma_value_t to_number_value = ecma_op_to_number (arguments_list_p[0], &length_num);

      if (ECMA_IS_VALUE_ERROR (to_number_value))
      {
        ecma_deref_object (proto_p);
        return to_number_value;
      }
    }

    if (ecma_number_is_nan (length_num))
    {
      length_num = 0;
    }

    const uint32_t maximum_size_in_byte = UINT32_MAX - sizeof (ecma_extended_object_t) - JMEM_ALIGNMENT + 1;

    if (length_num <= (ecma_number_t) -1.0f || length_num > (ecma_number_t) maximum_size_in_byte + (ecma_number_t) 0.5f)
    {
      ecma_deref_object (proto_p);
      return ecma_raise_range_error (ECMA_ERR_INVALID_ARRAYBUFFER_LENGTH);
    }
  }

  uint32_t length_uint32 = ecma_number_to_uint32 (length_num);

  ecma_object_t *array_buffer = ecma_arraybuffer_new_object (length_uint32);
  ECMA_SET_NON_NULL_POINTER (array_buffer->u2.prototype_cp, proto_p);
  ecma_deref_object (proto_p);

  return ecma_make_object_value (array_buffer);
} /* ecma_op_create_arraybuffer_object */

/**
 * Helper function: check if the target is ArrayBuffer
 *
 *
 * See also: ES2015 24.1.1.4
 *
 * @return true - if value is an ArrayBuffer object
 *         false - otherwise
 */
bool
ecma_is_arraybuffer (ecma_value_t target) /**< the target value */
{
  return (ecma_is_value_object (target)
          && ecma_object_class_is (ecma_get_object_from_value (target), ECMA_OBJECT_CLASS_ARRAY_BUFFER));
} /* ecma_is_arraybuffer */

/**
 * Helper function: return the length of the buffer inside the arraybuffer object
 *
 * @return uint32_t, the length of the arraybuffer
 */
uint32_t JERRY_ATTR_PURE
ecma_arraybuffer_get_length (ecma_object_t *object_p) /**< pointer to the ArrayBuffer object */
{
  JERRY_ASSERT (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER)
                || ecma_object_is_shared_arraybuffer (object_p));

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  return ecma_arraybuffer_is_detached (object_p) ? 0 : ext_object_p->u.cls.u3.length;
} /* ecma_arraybuffer_get_length */

/**
 * Helper function: return the pointer to the data buffer inside the arraybuffer object
 *
 * @return pointer to the data buffer
 */
uint8_t *JERRY_ATTR_PURE
ecma_arraybuffer_get_buffer (ecma_object_t *object_p) /**< pointer to the ArrayBuffer object */
{
  JERRY_ASSERT (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER)
                || ecma_object_is_shared_arraybuffer (object_p));

  if (!(ECMA_ARRAYBUFFER_GET_FLAGS (object_p) & ECMA_ARRAYBUFFER_HAS_POINTER))
  {
    return (uint8_t *) object_p + sizeof (ecma_extended_object_t);
  }

  ecma_arraybuffer_pointer_t *arraybuffer_pointer_p = (ecma_arraybuffer_pointer_t *) object_p;
  return (uint8_t *) arraybuffer_pointer_p->buffer_p;
} /* ecma_arraybuffer_get_buffer */

/**
 * Helper function: check if the target ArrayBuffer is detached
 *
 * @return true - if value is an detached ArrayBuffer object
 *         false - otherwise
 */
bool JERRY_ATTR_PURE
ecma_arraybuffer_is_detached (ecma_object_t *object_p) /**< pointer to the ArrayBuffer object */
{
  JERRY_ASSERT (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER)
                || ecma_object_is_shared_arraybuffer (object_p));

  return (ECMA_ARRAYBUFFER_GET_FLAGS (object_p) & ECMA_ARRAYBUFFER_DETACHED) != 0;
} /* ecma_arraybuffer_is_detached */

/**
 * ArrayBuffer object detaching operation
 *
 * See also: ES2015 24.1.1.3
 *
 * @return true - if detach operation is succeeded
 *         false - otherwise
 */
bool
ecma_arraybuffer_detach (ecma_object_t *object_p) /**< pointer to the ArrayBuffer object */
{
  JERRY_ASSERT (ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER));

  if (ECMA_ARRAYBUFFER_GET_FLAGS (object_p) & ECMA_ARRAYBUFFER_DETACHED)
  {
    return false;
  }

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  ext_object_p->u.cls.u1.array_buffer_flags |= ECMA_ARRAYBUFFER_DETACHED;

  if (!(ECMA_ARRAYBUFFER_GET_FLAGS (object_p) & ECMA_ARRAYBUFFER_ALLOCATED))
  {
    return true;
  }

  ext_object_p->u.cls.u1.array_buffer_flags &= (uint8_t) ~ECMA_ARRAYBUFFER_ALLOCATED;

  if (!(ECMA_ARRAYBUFFER_GET_FLAGS (object_p) & ECMA_ARRAYBUFFER_HAS_POINTER))
  {
    return true;
  }

  ecma_arraybuffer_release_buffer (object_p);
  return true;
} /* ecma_arraybuffer_detach */

/**
 * ArrayBuffer slice operation
 *
 * See also:
 *          ECMA-262 v11, 24.1.4.3
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
ecma_value_t
ecma_builtin_arraybuffer_slice (ecma_value_t this_arg, const ecma_value_t *argument_list_p, uint32_t arguments_number)
{
  ecma_object_t *object_p = ecma_get_object_from_value (this_arg);

  /* 3-4. */
  if (ECMA_ARRAYBUFFER_LAZY_ALLOC (object_p))
  {
    return ECMA_VALUE_ERROR;
  }

  if (ecma_arraybuffer_is_detached (object_p))
  {
    return ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
  }

  /* 5. */
  uint32_t len = ecma_arraybuffer_get_length (object_p);

  uint32_t start = 0;
  uint32_t end = len;

  if (arguments_number > 0)
  {
    /* 6-7. */
    if (ECMA_IS_VALUE_ERROR (ecma_builtin_helper_uint32_index_normalize (argument_list_p[0], len, &start)))
    {
      return ECMA_VALUE_ERROR;
    }

    if (arguments_number > 1 && !ecma_is_value_undefined (argument_list_p[1]))
    {
      /* 8-9. */
      if (ECMA_IS_VALUE_ERROR (ecma_builtin_helper_uint32_index_normalize (argument_list_p[1], len, &end)))
      {
        return ECMA_VALUE_ERROR;
      }
    }
  }

  /* 10. */
  uint32_t new_len = (end >= start) ? (end - start) : 0;

  /* 11. */
  ecma_builtin_id_t buffer_builtin_id = ECMA_BUILTIN_ID_ARRAYBUFFER;

  ecma_value_t ctor = ecma_op_species_constructor (object_p, buffer_builtin_id);

  if (ECMA_IS_VALUE_ERROR (ctor))
  {
    return ctor;
  }

  /* 12. */
  ecma_object_t *ctor_obj_p = ecma_get_object_from_value (ctor);
  ecma_value_t new_len_value = ecma_make_uint32_value (new_len);

  ecma_value_t new_arraybuffer = ecma_op_function_construct (ctor_obj_p, ctor_obj_p, &new_len_value, 1);

  ecma_deref_object (ctor_obj_p);
  ecma_free_value (new_len_value);

  if (ECMA_IS_VALUE_ERROR (new_arraybuffer))
  {
    return new_arraybuffer;
  }

  ecma_object_t *new_arraybuffer_p = ecma_get_object_from_value (new_arraybuffer);
  ecma_value_t ret_value = ECMA_VALUE_EMPTY;  
  lit_utf8_byte_t *old_buf;
  lit_utf8_byte_t *new_buf;

  /* 13. */
  if (!(ecma_object_class_is (new_arraybuffer_p, ECMA_OBJECT_CLASS_ARRAY_BUFFER)
        || ecma_object_is_shared_arraybuffer (new_arraybuffer_p)))
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_RETURN_VALUE_IS_NOT_AN_ARRAYBUFFER_OBJECT);
    goto free_new_arraybuffer;
  }

  /* 14-15. */
  if (ECMA_ARRAYBUFFER_LAZY_ALLOC (new_arraybuffer_p))
  {
    ret_value = ECMA_VALUE_ERROR;
    goto free_new_arraybuffer;
  }

  if (ecma_arraybuffer_is_detached (new_arraybuffer_p))
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_ARRAYBUFFER_IS_DETACHED);
    goto free_new_arraybuffer;
  }

  /* 16. */
  if (new_arraybuffer == this_arg)
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_ARRAY_BUFFER_RETURNED_THIS_FROM_CONSTRUCTOR);
    goto free_new_arraybuffer;
  }

  /* 17. */
  if (ecma_arraybuffer_get_length (new_arraybuffer_p) < new_len)
  {
    ret_value = ecma_raise_type_error (ECMA_ERR_DERIVED_ARRAY_BUFFER_CTOR_BUFFER_TOO_SMALL);
    goto free_new_arraybuffer;
  }

  /* 19. */
  if (ecma_arraybuffer_is_detached (object_p))
  {
    ret_value = ECMA_VALUE_ERROR;
    goto free_new_arraybuffer;
  }

  /* 20. */
  old_buf = ecma_arraybuffer_get_buffer (object_p);

  /* 21. */
  new_buf = ecma_arraybuffer_get_buffer (new_arraybuffer_p);

  /* 22. */
  memcpy (new_buf, old_buf + start, new_len);

free_new_arraybuffer:
  if (ret_value != ECMA_VALUE_EMPTY)
  {
    ecma_deref_object (new_arraybuffer_p);
  }
  else
  {
    /* 23. */
    ret_value = ecma_make_object_value (new_arraybuffer_p);
  }

  return ret_value;
} /* ecma_builtin_arraybuffer_slice */

/**
 * @}
 * @}
 */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
