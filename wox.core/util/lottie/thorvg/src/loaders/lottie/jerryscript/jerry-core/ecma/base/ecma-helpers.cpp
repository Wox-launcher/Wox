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

#include "ecma-helpers.h"

#include "ecma-alloc.h"
#include "ecma-array-object.h"
#include "ecma-builtins.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-lcache.h"
#include "ecma-property-hashmap.h"

#include "byte-code.h"
#include "jcontext.h"
#include "jrt-bit-fields.h"
#include "re-compiler.h"


/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmahelpers Helpers for operations with ECMA data types
 * @{
 */

JERRY_STATIC_ASSERT (((int)ECMA_OBJECT_TYPE_MASK >= ((int)ECMA_OBJECT_TYPE__MAX) - 1),
                     ecma_object_types_must_be_lower_than_the_container_mask);

JERRY_STATIC_ASSERT (((int)ECMA_OBJECT_TYPE_MASK >= (int)ECMA_LEXICAL_ENVIRONMENT_TYPE__MAX),
                     ecma_lexical_environment_types_must_be_lower_than_the_container_mask);

JERRY_STATIC_ASSERT (((int)ECMA_OBJECT_FLAG_EXTENSIBLE == ((int)ECMA_OBJECT_TYPE_MASK) + 1),
                     ecma_extensible_flag_must_follow_the_object_type);

JERRY_STATIC_ASSERT ((ECMA_OBJECT_REF_ONE == (ECMA_OBJECT_FLAG_EXTENSIBLE << 1)),
                     ecma_object_ref_one_must_follow_the_extensible_flag);

JERRY_STATIC_ASSERT (((ECMA_OBJECT_MAX_REF + ECMA_OBJECT_REF_ONE) == ECMA_OBJECT_REF_MASK),
                     ecma_object_max_ref_does_not_fill_the_remaining_bits);

JERRY_STATIC_ASSERT (((ECMA_OBJECT_REF_MASK & (ECMA_OBJECT_TYPE_MASK | ECMA_OBJECT_FLAG_EXTENSIBLE)) == 0),
                     ecma_object_ref_mask_overlaps_with_object_type_or_extensible);

JERRY_STATIC_ASSERT ((ECMA_PROPERTY_FLAGS_MASK == ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE),
                     ecma_property_flags_mask_must_use_the_configurable_enumerable_writable_flags);

/* These checks are needed by ecma_get_object_base_type. */
JERRY_STATIC_ASSERT (((int) ECMA_OBJECT_TYPE_BUILT_IN_GENERAL == ((int) ECMA_OBJECT_TYPE_GENERAL | 0x1))
                       && (((int) ECMA_OBJECT_TYPE_GENERAL & 0x1) == 0),
                     ecma_object_type_built_in_general_has_unexpected_value);
JERRY_STATIC_ASSERT (((int) ECMA_OBJECT_TYPE_BUILT_IN_CLASS == ((int) ECMA_OBJECT_TYPE_CLASS | 0x1))
                       && (((int) ECMA_OBJECT_TYPE_CLASS & 0x1) == 0),
                     ecma_object_type_built_in_class_has_unexpected_value);
JERRY_STATIC_ASSERT (((int) ECMA_OBJECT_TYPE_BUILT_IN_ARRAY == ((int) ECMA_OBJECT_TYPE_ARRAY | 0x1))
                       && (((int) ECMA_OBJECT_TYPE_ARRAY & 0x1) == 0),
                     ecma_object_type_built_in_array_has_unexpected_value);

/**
 * Create an object with specified prototype object
 * (or NULL prototype if there is not prototype for the object)
 * and value of 'Extensible' attribute.
 *
 * Reference counter's value will be set to one.
 *
 * @return pointer to the object's descriptor
 */
ecma_object_t *
ecma_create_object (ecma_object_t *prototype_object_p, /**< pointer to prototype of the object (or NULL) */
                    size_t ext_object_size, /**< size of extended objects */
                    ecma_object_type_t type) /**< object type */
{
  ecma_object_t *new_object_p;

  if (ext_object_size > 0)
  {
    new_object_p = (ecma_object_t *) ecma_alloc_extended_object (ext_object_size);
  }
  else
  {
    new_object_p = ecma_alloc_object ();
  }

  new_object_p->type_flags_refs = (ecma_object_descriptor_t) (type | ECMA_OBJECT_FLAG_EXTENSIBLE);

  ecma_init_gc_info (new_object_p);

  new_object_p->u1.property_list_cp = JMEM_CP_NULL;

  ECMA_SET_POINTER (new_object_p->u2.prototype_cp, prototype_object_p);

  return new_object_p;
} /* ecma_create_object */

/**
 * Create a declarative lexical environment with specified outer lexical environment
 * (or NULL if the environment is not nested).
 *
 * See also: ECMA-262 v5, 10.2.1.1
 *
 * Reference counter's value will be set to one.
 *
 * @return pointer to the descriptor of lexical environment
 */
ecma_object_t *
ecma_create_decl_lex_env (ecma_object_t *outer_lexical_environment_p) /**< outer lexical environment */
{
  ecma_object_t *new_lexical_environment_p = ecma_alloc_object ();

  new_lexical_environment_p->type_flags_refs = ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE;

  ecma_init_gc_info (new_lexical_environment_p);

  new_lexical_environment_p->u1.property_list_cp = JMEM_CP_NULL;

  ECMA_SET_POINTER (new_lexical_environment_p->u2.outer_reference_cp, outer_lexical_environment_p);

  return new_lexical_environment_p;
} /* ecma_create_decl_lex_env */

/**
 * Create a object lexical environment with specified outer lexical environment
 * (or NULL if the environment is not nested), and binding object.
 *
 * See also: ECMA-262 v5, 10.2.1.2
 *
 * Reference counter's value will be set to one.
 *
 * @return pointer to the descriptor of lexical environment
 */
ecma_object_t *
ecma_create_object_lex_env (ecma_object_t *outer_lexical_environment_p, /**< outer lexical environment */
                            ecma_object_t *binding_obj_p) /**< binding object */
{
  JERRY_ASSERT (binding_obj_p != NULL && !ecma_is_lexical_environment (binding_obj_p));

  ecma_object_t *new_lexical_environment_p = ecma_alloc_object ();

  new_lexical_environment_p->type_flags_refs = ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND;

  ecma_init_gc_info (new_lexical_environment_p);

  ECMA_SET_NON_NULL_POINTER (new_lexical_environment_p->u1.bound_object_cp, binding_obj_p);

  ECMA_SET_POINTER (new_lexical_environment_p->u2.outer_reference_cp, outer_lexical_environment_p);

  return new_lexical_environment_p;
} /* ecma_create_object_lex_env */

/**
 * Create a lexical environment with a specified size.
 *
 * @return pointer to the descriptor of the lexical environment
 */
ecma_object_t *
ecma_create_lex_env_class (ecma_object_t *outer_lexical_environment_p, /**< outer lexical environment */
                           size_t lexical_env_size) /**< size of the lexical environment */
{
  ecma_object_t *new_lexical_environment_p;

  ecma_object_descriptor_t type_flags_refs = ECMA_LEXICAL_ENVIRONMENT_CLASS;

  if (lexical_env_size > 0)
  {
    new_lexical_environment_p = (ecma_object_t *) ecma_alloc_extended_object (lexical_env_size);
    type_flags_refs |= ECMA_OBJECT_FLAG_LEXICAL_ENV_HAS_DATA;
  }
  else
  {
    new_lexical_environment_p = ecma_alloc_object ();
  }

  new_lexical_environment_p->type_flags_refs = type_flags_refs;

  ecma_init_gc_info (new_lexical_environment_p);

  new_lexical_environment_p->u1.property_list_cp = JMEM_CP_NULL;

  ECMA_SET_POINTER (new_lexical_environment_p->u2.outer_reference_cp, outer_lexical_environment_p);

  return new_lexical_environment_p;
} /* ecma_create_lex_env_class */

/**
 * Check if the object is lexical environment.
 *
 * @return true  - if object is a lexical environment
 *         false - otherwise
 */
bool JERRY_ATTR_PURE
ecma_is_lexical_environment (const ecma_object_t *object_p) /**< object or lexical environment */
{
  JERRY_ASSERT (object_p != NULL);

  return (object_p->type_flags_refs & ECMA_OBJECT_TYPE_MASK) >= ECMA_LEXICAL_ENVIRONMENT_TYPE_START;
} /* ecma_is_lexical_environment */

/**
 * Set value of [[Extensible]] object's internal property.
 */
void
ecma_op_ordinary_object_set_extensible (ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p != NULL);
  JERRY_ASSERT (!ecma_is_lexical_environment (object_p));

  object_p->type_flags_refs |= ECMA_OBJECT_FLAG_EXTENSIBLE;
} /* ecma_op_ordinary_object_set_extensible */

/**
 * Get the internal type of an object.
 *
 * @return type of the object (ecma_object_type_t)
 */
ecma_object_type_t JERRY_ATTR_PURE
ecma_get_object_type (const ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p != NULL);
  JERRY_ASSERT (!ecma_is_lexical_environment (object_p));

  return (ecma_object_type_t) (object_p->type_flags_refs & ECMA_OBJECT_TYPE_MASK);
} /* ecma_get_object_type */

/**
 * Get the internal base type of an object.
 *
 * @return base type of the object (ecma_object_base_type_t)
 */
ecma_object_base_type_t JERRY_ATTR_PURE
ecma_get_object_base_type (const ecma_object_t *object_p) /**< object */
{
  JERRY_ASSERT (object_p != NULL);
  JERRY_ASSERT (!ecma_is_lexical_environment (object_p));

  return (ecma_object_base_type_t) (object_p->type_flags_refs & (ECMA_OBJECT_TYPE_MASK - 0x1));
} /* ecma_get_object_base_type */

/**
 * Get value of an object if the class matches
 *
 * @return value of the object if the class matches
 *         ECMA_VALUE_NOT_FOUND otherwise
 */
bool
ecma_object_class_is (ecma_object_t *object_p, /**< object */
                      ecma_object_class_type_t class_id) /**< class id */
{
  if (ecma_get_object_base_type (object_p) == ECMA_OBJECT_BASE_TYPE_CLASS)
  {
    ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

    if (ext_object_p->u.cls.type == (uint8_t) class_id)
    {
      return true;
    }
  }

  return false;
} /* ecma_object_class_is */

/**
 * Get type of lexical environment.
 *
 * @return type of the lexical environment (ecma_lexical_environment_type_t)
 */
ecma_lexical_environment_type_t JERRY_ATTR_PURE
ecma_get_lex_env_type (const ecma_object_t *object_p) /**< lexical environment */
{
  JERRY_ASSERT (object_p != NULL);
  JERRY_ASSERT (ecma_is_lexical_environment (object_p));

  return (ecma_lexical_environment_type_t) (object_p->type_flags_refs & ECMA_OBJECT_TYPE_MASK);
} /* ecma_get_lex_env_type */

/**
 * Get lexical environment's bound object.
 *
 * @return pointer to ecma object
 */
ecma_object_t *JERRY_ATTR_PURE
ecma_get_lex_env_binding_object (const ecma_object_t *object_p) /**< object-bound lexical environment */
{
  JERRY_ASSERT (object_p != NULL);
  JERRY_ASSERT (ecma_is_lexical_environment (object_p));
  JERRY_ASSERT (ecma_get_lex_env_type (object_p) == ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND
                || (ecma_get_lex_env_type (object_p) == ECMA_LEXICAL_ENVIRONMENT_CLASS
                    && !ECMA_LEX_ENV_CLASS_IS_MODULE (object_p)));

  return ECMA_GET_NON_NULL_POINTER (ecma_object_t, object_p->u1.bound_object_cp);
} /* ecma_get_lex_env_binding_object */

/**
 * Create a new lexical environment with the same property list as the passed lexical environment
 *
 * @return pointer to the newly created lexical environment
 */
ecma_object_t *
ecma_clone_decl_lexical_environment (ecma_object_t *lex_env_p, /**< declarative lexical environment */
                                     bool copy_values) /**< copy property values as well */
{
  JERRY_ASSERT (ecma_get_lex_env_type (lex_env_p) == ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE);
  JERRY_ASSERT (lex_env_p->u2.outer_reference_cp != JMEM_CP_NULL);

  ecma_object_t *outer_lex_env_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, lex_env_p->u2.outer_reference_cp);
  ecma_object_t *new_lex_env_p = ecma_create_decl_lex_env (outer_lex_env_p);

  jmem_cpointer_t prop_iter_cp = lex_env_p->u1.property_list_cp;
  ecma_property_header_t *prop_iter_p;

  JERRY_ASSERT (prop_iter_cp != JMEM_CP_NULL);

#if JERRY_PROPERTY_HASHMAP
  prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

  if (prop_iter_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
  {
    prop_iter_cp = prop_iter_p->next_property_cp;
  }

  JERRY_ASSERT (prop_iter_cp != JMEM_CP_NULL);
#endif /* JERRY_PROPERTY_HASHMAP */

  do
  {
    prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

    ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

    for (int i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      if (prop_iter_p->types[i] != ECMA_PROPERTY_TYPE_DELETED)
      {
        JERRY_ASSERT (ECMA_PROPERTY_IS_RAW_DATA (prop_iter_p->types[i]));

        uint8_t prop_attributes = (uint8_t) (prop_iter_p->types[i] & ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE);
        ecma_string_t *name_p = ecma_string_from_property_name (prop_iter_p->types[i], prop_pair_p->names_cp[i]);

        ecma_property_value_t *property_value_p;
        property_value_p = ecma_create_named_data_property (new_lex_env_p, name_p, prop_attributes, NULL);

        ecma_deref_ecma_string (name_p);

        JERRY_ASSERT (property_value_p->value == ECMA_VALUE_UNDEFINED);

        if (copy_values)
        {
          property_value_p->value = ecma_copy_value_if_not_object (prop_pair_p->values[i].value);
        }
        else
        {
          property_value_p->value = ECMA_VALUE_UNINITIALIZED;
        }
      }
    }

    prop_iter_cp = prop_iter_p->next_property_cp;
  } while (prop_iter_cp != JMEM_CP_NULL);

  ecma_deref_object (lex_env_p);
  return new_lex_env_p;
} /* ecma_clone_decl_lexical_environment */

/**
 * Create a property in an object and link it into
 * the object's properties' linked-list (at start of the list).
 *
 * @return pointer to the newly created property value
 */
static ecma_property_value_t *
ecma_create_property (ecma_object_t *object_p, /**< the object */
                      ecma_string_t *name_p, /**< property name */
                      uint8_t type_and_flags, /**< type and flags, see ecma_property_info_t */
                      ecma_property_value_t value, /**< property value */
                      ecma_property_t **out_prop_p) /**< [out] the property is also returned
                                                     *         if this field is non-NULL */
{
  JERRY_ASSERT (ECMA_PROPERTY_PAIR_ITEM_COUNT == 2);
  JERRY_ASSERT (name_p != NULL);
  JERRY_ASSERT (object_p != NULL);

  jmem_cpointer_t *property_list_head_p = &object_p->u1.property_list_cp;

  if (*property_list_head_p != ECMA_NULL_POINTER)
  {
    /* If the first entry is free (deleted), it is reused. */
    ecma_property_header_t *first_property_p =
      ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, *property_list_head_p);

#if JERRY_PROPERTY_HASHMAP
    bool has_hashmap = false;

    if (first_property_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
    {
      property_list_head_p = &first_property_p->next_property_cp;
      first_property_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, *property_list_head_p);
      has_hashmap = true;
    }
#endif /* JERRY_PROPERTY_HASHMAP */

    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (first_property_p));

    if (first_property_p->types[0] == ECMA_PROPERTY_TYPE_DELETED)
    {
      ecma_property_pair_t *first_property_pair_p = (ecma_property_pair_t *) first_property_p;

      ecma_property_t name_type;
      first_property_pair_p->names_cp[0] = ecma_string_to_property_name (name_p, &name_type);
      first_property_p->types[0] = (ecma_property_t) (type_and_flags | name_type);

      ecma_property_t *property_p = first_property_p->types + 0;

      JERRY_ASSERT (ECMA_PROPERTY_VALUE_PTR (property_p) == first_property_pair_p->values + 0);

      if (out_prop_p != NULL)
      {
        *out_prop_p = property_p;
      }

      first_property_pair_p->values[0] = value;

#if JERRY_PROPERTY_HASHMAP
      /* The property must be fully initialized before ecma_property_hashmap_insert
       * is called, because the insert operation may reallocate the hashmap, and
       * that triggers garbage collection which scans all properties of all objects.
       * A not fully initialized but queued property may cause a crash. */

      if (has_hashmap)
      {
        ecma_property_hashmap_insert (object_p, name_p, first_property_pair_p, 0);
      }
#endif /* JERRY_PROPERTY_HASHMAP */

      return first_property_pair_p->values + 0;
    }
  }

  /* Otherwise we create a new property pair and use its second value. */
  ecma_property_pair_t *first_property_pair_p = ecma_alloc_property_pair ();

  /* Need to query property_list_head_p again and recheck the existence
   * of property hashmap, because ecma_alloc_property_pair may delete them. */
  property_list_head_p = &object_p->u1.property_list_cp;
#if JERRY_PROPERTY_HASHMAP
  bool has_hashmap = false;

  if (*property_list_head_p != ECMA_NULL_POINTER)
  {
    ecma_property_header_t *first_property_p =
      ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, *property_list_head_p);

    if (first_property_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
    {
      property_list_head_p = &first_property_p->next_property_cp;
      has_hashmap = true;
    }
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  /* Just copy the previous value (no need to decompress, compress). */
  first_property_pair_p->header.next_property_cp = *property_list_head_p;
  first_property_pair_p->header.types[0] = ECMA_PROPERTY_TYPE_DELETED;
  first_property_pair_p->names_cp[0] = LIT_INTERNAL_MAGIC_STRING_DELETED;

  ecma_property_t name_type;
  first_property_pair_p->names_cp[1] = ecma_string_to_property_name (name_p, &name_type);

  first_property_pair_p->header.types[1] = (ecma_property_t) (type_and_flags | name_type);

  ECMA_SET_NON_NULL_POINTER (*property_list_head_p, &first_property_pair_p->header);

  ecma_property_t *property_p = first_property_pair_p->header.types + 1;

  JERRY_ASSERT (ECMA_PROPERTY_VALUE_PTR (property_p) == first_property_pair_p->values + 1);

  if (out_prop_p != NULL)
  {
    *out_prop_p = property_p;
  }

  first_property_pair_p->values[1] = value;

#if JERRY_PROPERTY_HASHMAP
  /* See the comment before the other ecma_property_hashmap_insert above. */

  if (has_hashmap)
  {
    ecma_property_hashmap_insert (object_p, name_p, first_property_pair_p, 1);
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  return first_property_pair_p->values + 1;
} /* ecma_create_property */

/**
 * Create named data property with given name, attributes and undefined value
 * in the specified object.
 *
 * @return pointer to the newly created property value
 */
ecma_property_value_t *
ecma_create_named_data_property (ecma_object_t *object_p, /**< object */
                                 ecma_string_t *name_p, /**< property name */
                                 uint8_t prop_attributes, /**< property attributes (See: ecma_property_flags_t) */
                                 ecma_property_t **out_prop_p) /**< [out] the property is also returned
                                                                *         if this field is non-NULL */
{
  JERRY_ASSERT (object_p != NULL && name_p != NULL);
  JERRY_ASSERT (ecma_is_lexical_environment (object_p) || !ecma_op_object_is_fast_array (object_p));
  JERRY_ASSERT (ecma_find_named_property (object_p, name_p) == NULL);
  JERRY_ASSERT ((prop_attributes & ~ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_ENUMERABLE_WRITABLE) == 0);

  uint8_t type_and_flags = ECMA_PROPERTY_FLAG_DATA | prop_attributes;

  ecma_property_value_t value;
  value.value = ECMA_VALUE_UNDEFINED;

  return ecma_create_property (object_p, name_p, type_and_flags, value, out_prop_p);
} /* ecma_create_named_data_property */

/**
 * Create named accessor property with given name, attributes, getter and setter.
 *
 * @return pointer to the newly created property value
 */
ecma_property_value_t *
ecma_create_named_accessor_property (ecma_object_t *object_p, /**< object */
                                     ecma_string_t *name_p, /**< property name */
                                     ecma_object_t *get_p, /**< getter */
                                     ecma_object_t *set_p, /**< setter */
                                     uint8_t prop_attributes, /**< property attributes */
                                     ecma_property_t **out_prop_p) /**< [out] the property is also returned
                                                                    *         if this field is non-NULL */
{
  JERRY_ASSERT (object_p != NULL && name_p != NULL);
  JERRY_ASSERT (ecma_is_lexical_environment (object_p) || !ecma_op_object_is_fast_array (object_p));
  JERRY_ASSERT (ecma_find_named_property (object_p, name_p) == NULL);
  JERRY_ASSERT ((prop_attributes & ~ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_ENUMERABLE) == 0);

  uint8_t type_and_flags = prop_attributes;

  ecma_property_value_t value;
  ECMA_SET_POINTER (value.getter_setter_pair.getter_cp, get_p);
  ECMA_SET_POINTER (value.getter_setter_pair.setter_cp, set_p);

  return ecma_create_property (object_p, name_p, type_and_flags, value, out_prop_p);
} /* ecma_create_named_accessor_property */

#if JERRY_MODULE_SYSTEM
/**
 * Create property reference
 */
void
ecma_create_named_reference_property (ecma_object_t *object_p, /**< object */
                                      ecma_string_t *name_p, /**< property name */
                                      ecma_value_t reference) /**< property reference */
{
  JERRY_ASSERT (object_p != NULL && name_p != NULL);
  JERRY_ASSERT (ecma_find_named_property (object_p, name_p) == NULL);
  JERRY_ASSERT ((ecma_is_lexical_environment (object_p)
                 && ecma_get_lex_env_type (object_p) == ECMA_LEXICAL_ENVIRONMENT_CLASS
                 && ECMA_LEX_ENV_CLASS_IS_MODULE (object_p))
                || ecma_object_class_is (object_p, ECMA_OBJECT_CLASS_MODULE_NAMESPACE));

  uint8_t type_and_flags = ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE;
  ecma_property_value_t value;

  value.value = reference;

  ecma_create_property (object_p, name_p, type_and_flags, value, NULL);
} /* ecma_create_named_reference_property */

#endif /* JERRY_MODULE_SYSTEM */

/**
 * Find named data property or named accessor property in a specified object.
 *
 * @return pointer to the property, if it is found,
 *         NULL - otherwise.
 */
ecma_property_t *
ecma_find_named_property (ecma_object_t *obj_p, /**< object to find property in */
                          ecma_string_t *name_p) /**< property's name */
{
  JERRY_ASSERT (obj_p != NULL);
  JERRY_ASSERT (name_p != NULL);
  JERRY_ASSERT (ecma_is_lexical_environment (obj_p) || !ecma_op_object_is_fast_array (obj_p));

#if JERRY_LCACHE
  ecma_property_t *property_p = ecma_lcache_lookup (obj_p, name_p);
  if (property_p != NULL)
  {
    return property_p;
  }
#else /* !JERRY_LCACHE */
  ecma_property_t *property_p = NULL;
#endif /* JERRY_LCACHE */

  jmem_cpointer_t prop_iter_cp = obj_p->u1.property_list_cp;

#if JERRY_PROPERTY_HASHMAP
  if (prop_iter_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);
    if (prop_iter_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
    {
      jmem_cpointer_t property_real_name_cp;
      property_p = ecma_property_hashmap_find ((ecma_property_hashmap_t *) prop_iter_p, name_p, &property_real_name_cp);
#if JERRY_LCACHE
      if (property_p != NULL && !ecma_is_property_lcached (property_p))
      {
        ecma_lcache_insert (obj_p, property_real_name_cp, property_p);
      }
#endif /* JERRY_LCACHE */
      return property_p;
    }
  }
#endif /* JERRY_PROPERTY_HASHMAP */

#if JERRY_PROPERTY_HASHMAP
  uint32_t steps = 0;
#endif /* JERRY_PROPERTY_HASHMAP */
  jmem_cpointer_t property_name_cp = ECMA_NULL_POINTER;

  if (ECMA_IS_DIRECT_STRING (name_p))
  {
    ecma_property_t prop_name_type = (ecma_property_t) ECMA_GET_DIRECT_STRING_TYPE (name_p);
    property_name_cp = (jmem_cpointer_t) ECMA_GET_DIRECT_STRING_VALUE (name_p);

    JERRY_ASSERT (prop_name_type > 0);

    while (prop_iter_cp != JMEM_CP_NULL)
    {
      ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

      JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

      ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

      if (prop_pair_p->names_cp[0] == property_name_cp
          && ECMA_PROPERTY_GET_NAME_TYPE (prop_iter_p->types[0]) == prop_name_type)
      {
        JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (prop_iter_p->types[0]));

        property_p = prop_iter_p->types + 0;
        break;
      }

      if (prop_pair_p->names_cp[1] == property_name_cp
          && ECMA_PROPERTY_GET_NAME_TYPE (prop_iter_p->types[1]) == prop_name_type)
      {
        JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (prop_iter_p->types[1]));

        property_p = prop_iter_p->types + 1;
        break;
      }

#if JERRY_PROPERTY_HASHMAP
      steps++;
#endif /* JERRY_PROPERTY_HASHMAP */
      prop_iter_cp = prop_iter_p->next_property_cp;
    }
  }
  else
  {
    while (prop_iter_cp != JMEM_CP_NULL)
    {
      ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

      JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

      ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

      if (ECMA_PROPERTY_GET_NAME_TYPE (prop_iter_p->types[0]) == ECMA_DIRECT_STRING_PTR)
      {
        property_name_cp = prop_pair_p->names_cp[0];
        ecma_string_t *prop_name_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, property_name_cp);

        if (ecma_compare_ecma_non_direct_strings (name_p, prop_name_p))
        {
          property_p = prop_iter_p->types + 0;
          break;
        }
      }

      if (ECMA_PROPERTY_GET_NAME_TYPE (prop_iter_p->types[1]) == ECMA_DIRECT_STRING_PTR)
      {
        property_name_cp = prop_pair_p->names_cp[1];
        ecma_string_t *prop_name_p = ECMA_GET_NON_NULL_POINTER (ecma_string_t, property_name_cp);

        if (ecma_compare_ecma_non_direct_strings (name_p, prop_name_p))
        {
          property_p = prop_iter_p->types + 1;
          break;
        }
      }

#if JERRY_PROPERTY_HASHMAP
      steps++;
#endif /* JERRY_PROPERTY_HASHMAP */
      prop_iter_cp = prop_iter_p->next_property_cp;
    }
  }

#if JERRY_PROPERTY_HASHMAP
  if (steps >= (ECMA_PROPERTY_HASHMAP_MINIMUM_SIZE / 2))
  {
    ecma_property_hashmap_create (obj_p);
  }
#endif /* JERRY_PROPERTY_HASHMAP */

#if JERRY_LCACHE
  if (property_p != NULL && !ecma_is_property_lcached (property_p))
  {
    ecma_lcache_insert (obj_p, property_name_cp, property_p);
  }
#endif /* JERRY_LCACHE */

  return property_p;
} /* ecma_find_named_property */

/**
 * Get named data property or named access property in specified object.
 *
 * Warning:
 *         the property must exist
 *
 * @return pointer to the property, if it is found,
 *         NULL - otherwise.
 */
ecma_property_value_t *
ecma_get_named_data_property (ecma_object_t *obj_p, /**< object to find property in */
                              ecma_string_t *name_p) /**< property's name */
{
  JERRY_ASSERT (obj_p != NULL);
  JERRY_ASSERT (name_p != NULL);
  JERRY_ASSERT (ecma_is_lexical_environment (obj_p) || !ecma_op_object_is_fast_array (obj_p));

  ecma_property_t *property_p = ecma_find_named_property (obj_p, name_p);

  JERRY_ASSERT (property_p != NULL && ECMA_PROPERTY_IS_RAW_DATA (*property_p));

  return ECMA_PROPERTY_VALUE_PTR (property_p);
} /* ecma_get_named_data_property */

/**
 * Delete the object's property referenced by its value pointer.
 *
 * Note: specified property must be owned by specified object.
 */
void
ecma_delete_property (ecma_object_t *object_p, /**< object */
                      ecma_property_value_t *prop_value_p) /**< property value reference */
{
  jmem_cpointer_t cur_prop_cp = object_p->u1.property_list_cp;

  ecma_property_header_t *prev_prop_p = NULL;

#if JERRY_PROPERTY_HASHMAP
  ecma_property_hashmap_delete_status hashmap_status = ECMA_PROPERTY_HASHMAP_DELETE_NO_HASHMAP;

  if (cur_prop_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *cur_prop_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, cur_prop_cp);

    if (cur_prop_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
    {
      prev_prop_p = cur_prop_p;
      cur_prop_cp = cur_prop_p->next_property_cp;
      hashmap_status = ECMA_PROPERTY_HASHMAP_DELETE_HAS_HASHMAP;
    }
  }
#endif /* JERRY_PROPERTY_HASHMAP */

  while (cur_prop_cp != JMEM_CP_NULL)
  {
    ecma_property_header_t *cur_prop_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, cur_prop_cp);

    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (cur_prop_p));

    ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) cur_prop_p;

    for (uint32_t i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      if ((prop_pair_p->values + i) == prop_value_p)
      {
        JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (cur_prop_p->types[i]));

#if JERRY_PROPERTY_HASHMAP
        if (hashmap_status == ECMA_PROPERTY_HASHMAP_DELETE_HAS_HASHMAP)
        {
          hashmap_status = ecma_property_hashmap_delete (object_p, prop_pair_p->names_cp[i], cur_prop_p->types + i);
        }
#endif /* JERRY_PROPERTY_HASHMAP */

        ecma_gc_free_property (object_p, prop_pair_p, i);
        cur_prop_p->types[i] = ECMA_PROPERTY_TYPE_DELETED;
        prop_pair_p->names_cp[i] = LIT_INTERNAL_MAGIC_STRING_DELETED;

        JERRY_ASSERT (ECMA_PROPERTY_PAIR_ITEM_COUNT == 2);

        if (cur_prop_p->types[1 - i] != ECMA_PROPERTY_TYPE_DELETED)
        {
#if JERRY_PROPERTY_HASHMAP
          /* The other property is still valid. */
          if (hashmap_status == ECMA_PROPERTY_HASHMAP_DELETE_RECREATE_HASHMAP)
          {
            ecma_property_hashmap_free (object_p);
            ecma_property_hashmap_create (object_p);
          }
#endif /* JERRY_PROPERTY_HASHMAP */
          return;
        }

        JERRY_ASSERT (cur_prop_p->types[i] == ECMA_PROPERTY_TYPE_DELETED);

        if (prev_prop_p == NULL)
        {
          object_p->u1.property_list_cp = cur_prop_p->next_property_cp;
        }
        else
        {
          prev_prop_p->next_property_cp = cur_prop_p->next_property_cp;
        }

        ecma_dealloc_property_pair ((ecma_property_pair_t *) cur_prop_p);

#if JERRY_PROPERTY_HASHMAP
        if (hashmap_status == ECMA_PROPERTY_HASHMAP_DELETE_RECREATE_HASHMAP)
        {
          ecma_property_hashmap_free (object_p);
          ecma_property_hashmap_create (object_p);
        }
#endif /* JERRY_PROPERTY_HASHMAP */
        return;
      }
    }

    prev_prop_p = cur_prop_p;
    cur_prop_cp = cur_prop_p->next_property_cp;
  }
} /* ecma_delete_property */

/**
 * Check whether the object contains a property
 */
static void
ecma_assert_object_contains_the_property (const ecma_object_t *object_p, /**< ecma-object */
                                          const ecma_property_value_t *prop_value_p, /**< property value */
                                          bool is_data) /**< property should be data property */
{
#ifndef JERRY_NDEBUG
  jmem_cpointer_t prop_iter_cp = object_p->u1.property_list_cp;
  JERRY_ASSERT (prop_iter_cp != JMEM_CP_NULL);

  ecma_property_header_t *prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

  if (prop_iter_p->types[0] == ECMA_PROPERTY_TYPE_HASHMAP)
  {
    prop_iter_cp = prop_iter_p->next_property_cp;
  }

  while (prop_iter_cp != JMEM_CP_NULL)
  {
    prop_iter_p = ECMA_GET_NON_NULL_POINTER (ecma_property_header_t, prop_iter_cp);

    JERRY_ASSERT (ECMA_PROPERTY_IS_PROPERTY_PAIR (prop_iter_p));

    ecma_property_pair_t *prop_pair_p = (ecma_property_pair_t *) prop_iter_p;

    for (int i = 0; i < ECMA_PROPERTY_PAIR_ITEM_COUNT; i++)
    {
      if ((prop_pair_p->values + i) == prop_value_p)
      {
        JERRY_ASSERT (is_data == ((prop_pair_p->header.types[i] & ECMA_PROPERTY_FLAG_DATA) != 0));
        return;
      }
    }

    prop_iter_cp = prop_iter_p->next_property_cp;
  }
#else /* JERRY_NDEBUG */
  JERRY_UNUSED (object_p);
  JERRY_UNUSED (prop_value_p);
  JERRY_UNUSED (is_data);
#endif /* !JERRY_NDEBUG */
} /* ecma_assert_object_contains_the_property */

/**
 * Assign value to named data property
 *
 * Note:
 *      value previously stored in the property is freed
 *
 * @return void
 */
void
ecma_named_data_property_assign_value (ecma_object_t *obj_p, /**< object */
                                       ecma_property_value_t *prop_value_p, /**< property value reference */
                                       ecma_value_t value) /**< value to assign */
{
  ecma_assert_object_contains_the_property (obj_p, prop_value_p, true);

  ecma_value_assign_value (&prop_value_p->value, value);
} /* ecma_named_data_property_assign_value */

/**
 * Get named accessor property getter-setter-pair
 *
 * @return pointer to object's getter-setter pair
 */
ecma_getter_setter_pointers_t *
ecma_get_named_accessor_property (const ecma_property_value_t *prop_value_p) /**< property value reference */
{
  return (ecma_getter_setter_pointers_t *) &prop_value_p->getter_setter_pair;
} /* ecma_get_named_accessor_property */

/**
 * Set getter of named accessor property
 */
void
ecma_set_named_accessor_property_getter (ecma_object_t *object_p, /**< the property's container */
                                         ecma_property_value_t *prop_value_p, /**< property value reference */
                                         ecma_object_t *getter_p) /**< getter object */
{
  ecma_assert_object_contains_the_property (object_p, prop_value_p, false);

  ECMA_SET_POINTER (prop_value_p->getter_setter_pair.getter_cp, getter_p);
} /* ecma_set_named_accessor_property_getter */

/**
 * Set setter of named accessor property
 */
void
ecma_set_named_accessor_property_setter (ecma_object_t *object_p, /**< the property's container */
                                         ecma_property_value_t *prop_value_p, /**< property value reference */
                                         ecma_object_t *setter_p) /**< setter object */
{
  ecma_assert_object_contains_the_property (object_p, prop_value_p, false);

  ECMA_SET_POINTER (prop_value_p->getter_setter_pair.setter_cp, setter_p);
} /* ecma_set_named_accessor_property_setter */

#if JERRY_MODULE_SYSTEM

/**
 * Construct a reference to a given property
 *
 * @return property reference
 */
ecma_value_t
ecma_property_to_reference (ecma_property_t *property_p) /**< data or reference property */
{
  ecma_property_value_t *referenced_value_p = ECMA_PROPERTY_VALUE_PTR (property_p);

  if (!(*property_p & ECMA_PROPERTY_FLAG_DATA))
  {
    return referenced_value_p->value;
  }

  jmem_cpointer_tag_t offset = (jmem_cpointer_tag_t) (((uintptr_t) property_p) & 0x1);

  if (offset == 0)
  {
    ++referenced_value_p;
  }

  JERRY_ASSERT ((((uintptr_t) referenced_value_p) & (((uintptr_t) 1 << JMEM_ALIGNMENT_LOG) - 1)) == 0);

  ecma_value_t result;
  ECMA_SET_NON_NULL_POINTER_TAG (result, referenced_value_p, offset);
  return result;
} /* ecma_property_to_reference */

/**
 * Gets the referenced property value
 *
 * @return pointer to the value
 */
ecma_property_value_t *
ecma_get_property_value_from_named_reference (ecma_property_value_t *reference_p) /**< data property reference */
{
  ecma_value_t value = reference_p->value;
  reference_p = ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_property_value_t, value);

  if (!ECMA_GET_FIRST_BIT_FROM_POINTER_TAG (value))
  {
    --reference_p;
  }

  return reference_p;
} /* ecma_get_property_value_from_named_reference */

#endif /* JERRY_MODULE_SYSTEM */

/**
 * Get property's 'Writable' attribute value
 *
 * @return true - property is writable,
 *         false - otherwise
 */
bool
ecma_is_property_writable (ecma_property_t property) /**< property */
{
  JERRY_ASSERT (property & ECMA_PROPERTY_FLAG_DATA);

  return (property & ECMA_PROPERTY_FLAG_WRITABLE) != 0;
} /* ecma_is_property_writable */

/**
 * Set property's 'Writable' attribute value
 */
void
ecma_set_property_writable_attr (ecma_property_t *property_p, /**< [in,out] property */
                                 bool is_writable) /**< new value for writable flag */
{
  JERRY_ASSERT (ECMA_PROPERTY_IS_RAW_DATA (*property_p));

  if (is_writable)
  {
    *property_p = (uint8_t) (*property_p | ECMA_PROPERTY_FLAG_WRITABLE);
  }
  else
  {
    *property_p = (uint8_t) (*property_p & ~ECMA_PROPERTY_FLAG_WRITABLE);
  }
} /* ecma_set_property_writable_attr */

/**
 * Get property's 'Enumerable' attribute value
 *
 * @return true - property is enumerable,
 *         false - otherwise
 */
bool
ecma_is_property_enumerable (ecma_property_t property) /**< property */
{
  JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (property));

  return (property & ECMA_PROPERTY_FLAG_ENUMERABLE) != 0;
} /* ecma_is_property_enumerable */

/**
 * Set property's 'Enumerable' attribute value
 */
void
ecma_set_property_enumerable_attr (ecma_property_t *property_p, /**< [in,out] property */
                                   bool is_enumerable) /**< new value for enumerable flag */
{
  JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

  if (is_enumerable)
  {
    *property_p = (uint8_t) (*property_p | ECMA_PROPERTY_FLAG_ENUMERABLE);
  }
  else
  {
    *property_p = (uint8_t) (*property_p & ~ECMA_PROPERTY_FLAG_ENUMERABLE);
  }
} /* ecma_set_property_enumerable_attr */

/**
 * Get property's 'Configurable' attribute value
 *
 * @return true - property is configurable,
 *         false - otherwise
 */
bool
ecma_is_property_configurable (ecma_property_t property) /**< property */
{
  JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (property));

  return (property & ECMA_PROPERTY_FLAG_CONFIGURABLE) != 0;
} /* ecma_is_property_configurable */

/**
 * Set property's 'Configurable' attribute value
 */
void
ecma_set_property_configurable_attr (ecma_property_t *property_p, /**< [in,out] property */
                                     bool is_configurable) /**< new value for configurable flag */
{
  JERRY_ASSERT (ECMA_PROPERTY_IS_RAW (*property_p));

  if (is_configurable)
  {
    *property_p = (uint8_t) (*property_p | ECMA_PROPERTY_FLAG_CONFIGURABLE);
  }
  else
  {
    *property_p = (uint8_t) (*property_p & ~ECMA_PROPERTY_FLAG_CONFIGURABLE);
  }
} /* ecma_set_property_configurable_attr */

#if JERRY_LCACHE

/**
 * Check whether the property is registered in LCache
 *
 * @return true / false
 */
bool
ecma_is_property_lcached (ecma_property_t *property_p) /**< property */
{
  JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (*property_p));

  return (*property_p & ECMA_PROPERTY_FLAG_LCACHED) != 0;
} /* ecma_is_property_lcached */

/**
 * Set value of flag indicating whether the property is registered in LCache
 */
void
ecma_set_property_lcached (ecma_property_t *property_p, /**< property */
                           bool is_lcached) /**< new value for lcached flag */
{
  JERRY_ASSERT (ECMA_PROPERTY_IS_NAMED_PROPERTY (*property_p));

  if (is_lcached)
  {
    *property_p = (uint8_t) (*property_p | ECMA_PROPERTY_FLAG_LCACHED);
  }
  else
  {
    *property_p = (uint8_t) (*property_p & ~ECMA_PROPERTY_FLAG_LCACHED);
  }
} /* ecma_set_property_lcached */

#endif /* JERRY_LCACHE */

/**
 * Construct empty property descriptor, i.e.:
 *  property descriptor with all is_defined flags set to false and the rest - to default value.
 *
 * @return empty property descriptor
 */
ecma_property_descriptor_t
ecma_make_empty_property_descriptor (void)
{
  ecma_property_descriptor_t prop_desc;

  prop_desc.flags = 0;
  prop_desc.value = ECMA_VALUE_UNDEFINED;
  prop_desc.get_p = NULL;
  prop_desc.set_p = NULL;

  return prop_desc;
} /* ecma_make_empty_property_descriptor */

/**
 * Free values contained in the property descriptor
 * and make it empty property descriptor
 */
void
ecma_free_property_descriptor (ecma_property_descriptor_t *prop_desc_p) /**< property descriptor */
{
  if (prop_desc_p->flags & JERRY_PROP_IS_VALUE_DEFINED)
  {
    ecma_free_value (prop_desc_p->value);
  }

  if ((prop_desc_p->flags & JERRY_PROP_IS_GET_DEFINED) && prop_desc_p->get_p != NULL)
  {
    ecma_deref_object (prop_desc_p->get_p);
  }

  if ((prop_desc_p->flags & JERRY_PROP_IS_SET_DEFINED) && prop_desc_p->set_p != NULL)
  {
    ecma_deref_object (prop_desc_p->set_p);
  }

  *prop_desc_p = ecma_make_empty_property_descriptor ();
} /* ecma_free_property_descriptor */

/**
 * Increase ref count of an extended primitive value.
 */
void
ecma_ref_extended_primitive (ecma_extended_primitive_t *primitive_p) /**< extended primitive value */
{
  if (JERRY_LIKELY (primitive_p->refs_and_type < ECMA_EXTENDED_PRIMITIVE_MAX_REF))
  {
    primitive_p->refs_and_type += ECMA_EXTENDED_PRIMITIVE_REF_ONE;
  }
  else
  {
    jerry_fatal (JERRY_FATAL_REF_COUNT_LIMIT);
  }
} /* ecma_ref_extended_primitive */

/**
 * Decrease ref count of an error reference.
 */
void
ecma_deref_exception (ecma_extended_primitive_t *error_ref_p) /**< error reference */
{
  JERRY_ASSERT (error_ref_p->refs_and_type >= ECMA_EXTENDED_PRIMITIVE_REF_ONE);

  error_ref_p->refs_and_type -= ECMA_EXTENDED_PRIMITIVE_REF_ONE;

  if (error_ref_p->refs_and_type < ECMA_EXTENDED_PRIMITIVE_REF_ONE)
  {
    ecma_free_value (error_ref_p->u.value);
    jmem_pools_free (error_ref_p, sizeof (ecma_extended_primitive_t));
  }
} /* ecma_deref_exception */

#if JERRY_BUILTIN_BIGINT

/**
 * Decrease ref count of a bigint value.
 */
void
ecma_deref_bigint (ecma_extended_primitive_t *bigint_p) /**< bigint value */
{
  JERRY_ASSERT (bigint_p->refs_and_type >= ECMA_EXTENDED_PRIMITIVE_REF_ONE);

  bigint_p->refs_and_type -= ECMA_EXTENDED_PRIMITIVE_REF_ONE;

  if (bigint_p->refs_and_type >= ECMA_EXTENDED_PRIMITIVE_REF_ONE)
  {
    return;
  }

  uint32_t size = ECMA_BIGINT_GET_SIZE (bigint_p);

  JERRY_ASSERT (size > 0);

  size_t mem_size = ECMA_BIGINT_GET_BYTE_SIZE (size) + sizeof (ecma_extended_primitive_t);
  jmem_heap_free_block (bigint_p, mem_size);
} /* ecma_deref_bigint */

#endif /* JERRY_BUILTIN_BIGINT */

/**
 * Create an error reference from a given value.
 *
 * Note:
 *   Reference of the value is taken.
 *
 * @return error reference value
 */
ecma_value_t
ecma_create_exception (ecma_value_t value, /**< referenced value */
                       uint32_t options) /**< ECMA_ERROR_API_* options */
{
  ecma_extended_primitive_t *error_ref_p;
  error_ref_p = (ecma_extended_primitive_t *) jmem_pools_alloc (sizeof (ecma_extended_primitive_t));

  error_ref_p->refs_and_type = ECMA_EXTENDED_PRIMITIVE_REF_ONE | options;
  error_ref_p->u.value = value;
  return ecma_make_extended_primitive_value (error_ref_p, ECMA_TYPE_ERROR);
} /* ecma_create_exception */

/**
 * Create an error reference from the currently thrown error value.
 *
 * @return error reference value
 */
ecma_value_t
ecma_create_exception_from_context (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  uint32_t options = 0;
  uint32_t status_flags = JERRY_CONTEXT (status_flags);

  if (status_flags & ECMA_STATUS_ABORT)
  {
    options |= ECMA_ERROR_API_FLAG_ABORT;
  }

#if JERRY_VM_THROW
  if (status_flags & ECMA_STATUS_ERROR_THROWN)
  {
    options |= ECMA_ERROR_API_FLAG_THROW_CAPTURED;
  }
#endif /* JERRY_VM_THROW */

  return ecma_create_exception (jcontext_take_exception (), options);
} /* ecma_create_exception_from_context */

/**
 * Create an exception from a given object.
 *
 * Note:
 *   Reference of the object is taken.
 *
 * @return exception value
 */
ecma_value_t
ecma_create_exception_from_object (ecma_object_t *object_p) /**< referenced object */
{
  return ecma_create_exception (ecma_make_object_value (object_p), 0);
} /* ecma_create_exception_from_object */

/**
 * Raise a new exception from the argument exception value.
 *
 * Note: the argument exceptions reference count is decreased
 */
void
ecma_throw_exception (ecma_value_t value) /**< error reference */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (!jcontext_has_pending_exception () && !jcontext_has_pending_abort ());
  ecma_extended_primitive_t *error_ref_p = ecma_get_extended_primitive_from_value (value);

  JERRY_ASSERT (error_ref_p->refs_and_type >= ECMA_EXTENDED_PRIMITIVE_REF_ONE);

  ecma_value_t referenced_value = error_ref_p->u.value;
  uint32_t status_flags = JERRY_CONTEXT (status_flags);

  status_flags |= (ECMA_STATUS_EXCEPTION
#if JERRY_VM_THROW
                   | ECMA_STATUS_ERROR_THROWN
#endif /* JERRY_VM_THROW */
                   | ECMA_STATUS_ABORT);

  if (!(error_ref_p->refs_and_type & ECMA_ERROR_API_FLAG_ABORT))
  {
    status_flags &= ~(uint32_t) ECMA_STATUS_ABORT;
  }

#if JERRY_VM_THROW
  if (!(error_ref_p->refs_and_type & ECMA_ERROR_API_FLAG_THROW_CAPTURED))
  {
    status_flags &= ~(uint32_t) ECMA_STATUS_ERROR_THROWN;
  }
#endif /* JERRY_VM_THROW */

  JERRY_CONTEXT (status_flags) = status_flags;

  if (error_ref_p->refs_and_type >= 2 * ECMA_EXTENDED_PRIMITIVE_REF_ONE)
  {
    error_ref_p->refs_and_type -= ECMA_EXTENDED_PRIMITIVE_REF_ONE;
    referenced_value = ecma_copy_value (referenced_value);
  }
  else
  {
    jmem_pools_free (error_ref_p, sizeof (ecma_extended_primitive_t));
  }

  JERRY_CONTEXT (error_value) = referenced_value;
} /* ecma_throw_exception */

/**
 * Decrease the reference counter of a script value.
 */
void
ecma_script_deref (ecma_value_t script_value) /**< script value */
{
  cbc_script_t *script_p = ECMA_GET_INTERNAL_VALUE_POINTER (cbc_script_t, script_value);
  script_p->refs_and_type -= CBC_SCRIPT_REF_ONE;

  if (script_p->refs_and_type >= CBC_SCRIPT_REF_ONE)
  {
    return;
  }

  size_t script_size = sizeof (cbc_script_t);
  uint32_t type = script_p->refs_and_type;

  if (type & CBC_SCRIPT_HAS_USER_VALUE)
  {
    script_size += sizeof (ecma_value_t);

    if (!(type & CBC_SCRIPT_USER_VALUE_IS_OBJECT))
    {
      ecma_value_t user_value = CBC_SCRIPT_GET_USER_VALUE (script_p);

      JERRY_ASSERT (!ecma_is_value_object (user_value));
      ecma_free_value (user_value);
    }
  }

#if JERRY_SOURCE_NAME
  ecma_deref_ecma_string (ecma_get_string_from_value (script_p->source_name));
#endif /* JERRY_SOURCE_NAME */

#if JERRY_MODULE_SYSTEM
  if (type & CBC_SCRIPT_HAS_IMPORT_META)
  {
    JERRY_ASSERT (!(type & CBC_SCRIPT_HAS_FUNCTION_ARGUMENTS));
    JERRY_ASSERT (ecma_is_value_object (CBC_SCRIPT_GET_IMPORT_META (script_p, type)));

    script_size += sizeof (ecma_value_t);
  }
#endif /* JERRY_MODULE_SYSTEM */

  jmem_heap_free_block (script_p, script_size);
} /* ecma_script_deref */

/**
 * Increase reference counter of Compact
 * Byte Code or regexp byte code.
 */
void
ecma_bytecode_ref (ecma_compiled_code_t *bytecode_p) /**< byte code pointer */
{
  /* Abort program if maximum reference number is reached. */
  if (bytecode_p->refs >= UINT16_MAX)
  {
    jerry_fatal (JERRY_FATAL_REF_COUNT_LIMIT);
  }

  bytecode_p->refs++;
} /* ecma_bytecode_ref */

/**
 * Decrease reference counter of Compact
 * Byte Code or regexp byte code.
 */
void
ecma_bytecode_deref (ecma_compiled_code_t *bytecode_p) /**< byte code pointer */
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (bytecode_p->refs > 0);
  JERRY_ASSERT (!CBC_IS_FUNCTION (bytecode_p->status_flags)
                || !(bytecode_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION));

  bytecode_p->refs--;

  if (bytecode_p->refs > 0)
  {
    /* Non-zero reference counter. */
    return;
  }

  if (CBC_IS_FUNCTION (bytecode_p->status_flags))
  {
    ecma_value_t *literal_start_p = NULL;
    uint32_t literal_end;
    uint32_t const_literal_end;

    if (bytecode_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)
    {
      cbc_uint16_arguments_t *args_p = (cbc_uint16_arguments_t *) bytecode_p;
      literal_end = args_p->literal_end;
      const_literal_end = args_p->const_literal_end;

      literal_start_p = (ecma_value_t *) ((uint8_t *) bytecode_p + sizeof (cbc_uint16_arguments_t));
      literal_start_p -= args_p->register_end;
    }
    else
    {
      cbc_uint8_arguments_t *args_p = (cbc_uint8_arguments_t *) bytecode_p;
      literal_end = args_p->literal_end;
      const_literal_end = args_p->const_literal_end;

      literal_start_p = (ecma_value_t *) ((uint8_t *) bytecode_p + sizeof (cbc_uint8_arguments_t));
      literal_start_p -= args_p->register_end;
    }

    for (uint32_t i = const_literal_end; i < literal_end; i++)
    {
      ecma_compiled_code_t *bytecode_literal_p =
        ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, literal_start_p[i]);

      /* Self references are ignored. */
      if (bytecode_literal_p != bytecode_p)
      {
        ecma_bytecode_deref (bytecode_literal_p);
      }
    }

    ecma_script_deref (((cbc_uint8_arguments_t *) bytecode_p)->script_value);

    if (bytecode_p->status_flags & CBC_CODE_FLAGS_HAS_TAGGED_LITERALS)
    {
      ecma_collection_t *collection_p = ecma_compiled_code_get_tagged_template_collection (bytecode_p);

      /* Since the objects in the tagged template collection are not strong referenced anymore by the compiled code
         we can treat them as 'new' objects. */
      JERRY_CONTEXT (ecma_gc_new_objects) += collection_p->item_count * 2;
      ecma_collection_free_template_literal (collection_p);
    }

  }
  else
  {
#if JERRY_BUILTIN_REGEXP
    re_compiled_code_t *re_bytecode_p = (re_compiled_code_t *) bytecode_p;

    ecma_deref_ecma_string (ecma_get_string_from_value (re_bytecode_p->source));
#endif /* JERRY_BUILTIN_REGEXP */
  }

  jmem_heap_free_block (bytecode_p, ((size_t) bytecode_p->size) << JMEM_ALIGNMENT_LOG);
} /* ecma_bytecode_deref */

/**
 * Gets the script data assigned to a script / module / function
 *
 * @return script data - if available, JMEM_CP_NULL - otherwise
 */
ecma_value_t
ecma_script_get_from_value (ecma_value_t value) /**< compiled code */
{
  if (!ecma_is_value_object (value))
  {
    return JMEM_CP_NULL;
  }

  ecma_object_t *object_p = ecma_get_object_from_value (value);
  const ecma_compiled_code_t *bytecode_p = NULL;

  while (true)
  {
    switch (ecma_get_object_type (object_p))
    {
      case ECMA_OBJECT_TYPE_CLASS:
      {
        ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

        if (ext_object_p->u.cls.type == ECMA_OBJECT_CLASS_SCRIPT)
        {
          bytecode_p = ECMA_GET_INTERNAL_VALUE_POINTER (ecma_compiled_code_t, ext_object_p->u.cls.u3.value);
          break;
        }

#if JERRY_MODULE_SYSTEM
        if (ext_object_p->u.cls.type == ECMA_OBJECT_CLASS_MODULE)
        {
          ecma_module_t *module_p = (ecma_module_t *) object_p;

          if (!(module_p->header.u.cls.u2.module_flags & ECMA_MODULE_IS_NATIVE))
          {
            bytecode_p = module_p->u.compiled_code_p;
            break;
          }
        }
#endif /* JERRY_MODULE_SYSTEM */
        return JMEM_CP_NULL;
      }
      case ECMA_OBJECT_TYPE_FUNCTION:
      {
        bytecode_p = ecma_op_function_get_compiled_code ((ecma_extended_object_t *) object_p);
        break;
      }
      case ECMA_OBJECT_TYPE_BOUND_FUNCTION:
      {
        ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) object_p;

        object_p =
          ECMA_GET_NON_NULL_POINTER_FROM_POINTER_TAG (ecma_object_t, ext_object_p->u.bound_function.target_function);
        continue;
      }
      case ECMA_OBJECT_TYPE_CONSTRUCTOR_FUNCTION:
      {
        return ((ecma_extended_object_t *) object_p)->u.constructor_function.script_value;
      }
      default:
      {
        return JMEM_CP_NULL;
      }
    }

    JERRY_ASSERT (bytecode_p != NULL);
    return ((cbc_uint8_arguments_t *) bytecode_p)->script_value;
  }
} /* ecma_script_get_from_value */

/**
 * Resolve the position of the arguments list start of the compiled code
 *
 * @return start position of the arguments list start of the compiled code
 */
ecma_value_t *
ecma_compiled_code_resolve_arguments_start (const ecma_compiled_code_t *bytecode_header_p) /**< compiled code */
{
  JERRY_ASSERT (bytecode_header_p != NULL);

  uint8_t *byte_p = (uint8_t *) bytecode_header_p;
  byte_p += ((size_t) bytecode_header_p->size) << JMEM_ALIGNMENT_LOG;

  if (!(bytecode_header_p->status_flags & CBC_CODE_FLAGS_MAPPED_ARGUMENTS_NEEDED))
  {
    return ((ecma_value_t *) byte_p);
  }

  if (JERRY_LIKELY (!(bytecode_header_p->status_flags & CBC_CODE_FLAGS_UINT16_ARGUMENTS)))
  {
    return ((ecma_value_t *) byte_p) - ((cbc_uint8_arguments_t *) bytecode_header_p)->argument_end;
  }

  return ((ecma_value_t *) byte_p) - ((cbc_uint16_arguments_t *) bytecode_header_p)->argument_end;
} /* ecma_compiled_code_resolve_arguments_start */

/**
 * Resolve the position of the function name of the compiled code
 *
 * @return position of the function name of the compiled code
 */
ecma_value_t *
ecma_compiled_code_resolve_function_name (const ecma_compiled_code_t *bytecode_header_p) /**< compiled code */
{
  JERRY_ASSERT (bytecode_header_p != NULL);
  ecma_value_t *base_p = ecma_compiled_code_resolve_arguments_start (bytecode_header_p);

  if (CBC_FUNCTION_GET_TYPE (bytecode_header_p->status_flags) != CBC_FUNCTION_CONSTRUCTOR)
  {
    base_p--;
  }

  return base_p;
} /* ecma_compiled_code_resolve_function_name */

/**
 * Get the tagged template collection of the compiled code
 *
 * @return pointer to the tagged template collection
 */
ecma_collection_t *
ecma_compiled_code_get_tagged_template_collection (const ecma_compiled_code_t *bytecode_header_p) /**< compiled code */
{
  JERRY_ASSERT (bytecode_header_p != NULL);
  JERRY_ASSERT (bytecode_header_p->status_flags & CBC_CODE_FLAGS_HAS_TAGGED_LITERALS);

  ecma_value_t *base_p = ecma_compiled_code_resolve_function_name (bytecode_header_p);
  return ECMA_GET_INTERNAL_VALUE_POINTER (ecma_collection_t, base_p[-1]);
} /* ecma_compiled_code_get_tagged_template_collection */

/**
 * Get the source name of a compiled code.
 *
 * @return source name value
 */
ecma_value_t
ecma_get_source_name (const ecma_compiled_code_t *bytecode_p) /**< compiled code */
{
#if JERRY_SOURCE_NAME
#if JERRY_SNAPSHOT_EXEC
  if (JERRY_UNLIKELY (bytecode_p->status_flags & CBC_CODE_FLAGS_STATIC_FUNCTION))
  {
    return ecma_make_magic_string_value (LIT_MAGIC_STRING_SOURCE_NAME_ANON);
  }
#endif /* JERRY_SNAPSHOT_EXEC */

  ecma_value_t script_value = ((cbc_uint8_arguments_t *) bytecode_p)->script_value;
  return ECMA_GET_INTERNAL_VALUE_POINTER (cbc_script_t, script_value)->source_name;
#else /* !JERRY_SOURCE_NAME */
  JERRY_UNUSED (bytecode_p);
  return ecma_make_magic_string_value (LIT_MAGIC_STRING_SOURCE_NAME_ANON);
#endif /* !JERRY_SOURCE_NAME */
} /* ecma_get_source_name */

#if (JERRY_STACK_LIMIT != 0)
/**
 * Check the current stack usage by calculating the difference from the initial stack base.
 *
 * @return current stack usage in bytes
 */
uintptr_t JERRY_ATTR_NOINLINE
ecma_get_current_stack_usage (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  volatile int __sp;
  return (uintptr_t) (JERRY_CONTEXT (stack_base) - (uintptr_t) &__sp);
} /* ecma_get_current_stack_usage */

#endif /* (JERRY_STACK_LIMIT != 0) */

/**
 * @}
 * @}
 */
