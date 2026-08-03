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

#include "ecma-string-object.h"

#include "ecma-alloc.h"
#include "ecma-builtins.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers.h"
#include "ecma-objects-general.h"
#include "ecma-objects.h"

#include "jcontext.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmastringobject ECMA String object related routines
 * @{
 */

/**
 * String object creation operation.
 *
 * See also: ECMA-262 v5, 15.5.2.1
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value
 */
ecma_value_t
ecma_op_create_string_object (const ecma_value_t *arguments_list_p, /**< list of arguments that
                                                                         are passed to String constructor */
                              uint32_t arguments_list_len) /**< length of the arguments' list */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (arguments_list_len == 0 || arguments_list_p != NULL);

  ecma_value_t prim_value = ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);

  if (arguments_list_len > 0)
  {
    ecma_string_t *str_p = ecma_op_to_string (arguments_list_p[0]);

    if (JERRY_UNLIKELY (str_p == NULL))
    {
      return ECMA_VALUE_ERROR;
    }

    prim_value = ecma_make_string_value (str_p);
  }

  ecma_builtin_id_t proto_id;
#if JERRY_BUILTIN_STRING
  proto_id = ECMA_BUILTIN_ID_STRING_PROTOTYPE;
#else /* !JERRY_BUILTIN_STRING */
  proto_id = ECMA_BUILTIN_ID_OBJECT_PROTOTYPE;
#endif /* JERRY_BUILTIN_STRING */
  ecma_object_t *prototype_obj_p = ecma_builtin_get (proto_id);

  ecma_object_t *new_target = JERRY_CONTEXT (current_new_target_p);
  if (new_target)
  {
    prototype_obj_p = ecma_op_get_prototype_from_constructor (new_target, proto_id);
    if (JERRY_UNLIKELY (prototype_obj_p == NULL))
    {
      return ECMA_VALUE_ERROR;
    }
  }

  ecma_object_t *object_p =
    ecma_create_object (prototype_obj_p, sizeof (ecma_extended_object_t), ECMA_OBJECT_TYPE_CLASS);

  ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;
  ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_STRING;
  ext_object_p->u.cls.u3.value = prim_value;

  if (new_target)
  {
    ecma_deref_object (prototype_obj_p);
  }

  return ecma_make_object_value (object_p);
} /* ecma_op_create_string_object */

/**
 * List names of a String object's lazy instantiated properties
 */
void
ecma_op_string_list_lazy_property_names (ecma_object_t *obj_p, /**< a String object */
                                         ecma_collection_t *prop_names_p, /**< prop name collection */
                                         ecma_property_counter_t *prop_counter_p, /**< property counters */
                                         jerry_property_filter_t filter) /**< property name filter options */
{
  JERRY_ASSERT (ecma_get_object_base_type (obj_p) == ECMA_OBJECT_BASE_TYPE_CLASS);

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_INTEGER_INDICES))
  {
    ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;
    JERRY_ASSERT (ext_object_p->u.cls.type == ECMA_OBJECT_CLASS_STRING);

    ecma_string_t *prim_value_str_p = ecma_get_string_from_value (ext_object_p->u.cls.u3.value);

    lit_utf8_size_t length = ecma_string_get_length (prim_value_str_p);

    for (lit_utf8_size_t i = 0; i < length; i++)
    {
      ecma_string_t *name_p = ecma_new_ecma_string_from_uint32 (i);

      /* the properties are enumerable (ECMA-262 v5, 15.5.5.2.9) */
      ecma_collection_push_back (prop_names_p, ecma_make_string_value (name_p));
    }

    prop_counter_p->array_index_named_props += length;
  }

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS))
  {
    ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_LENGTH));
    prop_counter_p->string_named_props++;
  }
} /* ecma_op_string_list_lazy_property_names */

/**
 * @}
 * @}
 */
