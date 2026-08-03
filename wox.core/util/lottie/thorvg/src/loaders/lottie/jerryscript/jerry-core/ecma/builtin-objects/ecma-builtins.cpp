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

#include "ecma-builtins.h"

#include "ecma-alloc.h"
#include "ecma-exceptions.h"
#include "ecma-function-object.h"
#include "ecma-gc.h"
#include "ecma-globals.h"
#include "ecma-helpers-number.h"
#include "ecma-helpers.h"
#include "ecma-objects.h"

#include "jcontext.h"
#include "jrt-bit-fields.h"

#define ECMA_BUILTINS_INTERNAL
#include "ecma-builtins-internal.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmabuiltins
 * @{
 */

JERRY_STATIC_ASSERT ((((int)ECMA_BUILTIN_ID_GLOBAL) == ((int)ECMA_BUILTIN_OBJECTS_COUNT)),
                     ecma_builtin_id_global_must_be_the_last_builtin_id);

/**
 * Checks whether the built-in is an ecma_extended_built_in_object_t
 */
#define ECMA_BUILTIN_IS_EXTENDED_BUILT_IN(object_type) \
  ((object_type) == ECMA_OBJECT_TYPE_BUILT_IN_CLASS || (object_type) == ECMA_OBJECT_TYPE_BUILT_IN_ARRAY)

/**
 * Helper definition for ecma_builtin_property_list_references.
 */
typedef const ecma_builtin_property_descriptor_t *ecma_builtin_property_list_reference_t;

/**
 * Definition of built-in dispatch routine function pointer.
 */
typedef ecma_value_t (*ecma_builtin_dispatch_routine_t) (uint8_t builtin_routine_id,
                                                         ecma_value_t this_arg,
                                                         const ecma_value_t arguments_list[],
                                                         uint32_t arguments_number);
/**
 * Definition of built-in dispatch call function pointer.
 */
typedef ecma_value_t (*ecma_builtin_dispatch_call_t) (const ecma_value_t arguments_list[], uint32_t arguments_number);
/**
 * Definition of a builtin descriptor which contains the builtin object's:
 * - prototype objects's id (13-bits)
 * - type (3-bits)
 *
 * Layout:
 *
 * |----------------------|---------------|
 *     prototype_id(12)      obj_type(4)
 */
typedef uint16_t ecma_builtin_descriptor_t;

/**
 * Bitshift index for get the prototype object's id from a builtin descriptor
 */
#define ECMA_BUILTIN_PROTOTYPE_ID_SHIFT 4

/**
 * Bitmask for get the object's type from a builtin descriptor
 */
#define ECMA_BUILTIN_OBJECT_TYPE_MASK ((1 << ECMA_BUILTIN_PROTOTYPE_ID_SHIFT) - 1)

/**
 * Create a builtin descriptor value
 */
#define ECMA_MAKE_BUILTIN_DESCRIPTOR(type, proto_id) (((proto_id) << ECMA_BUILTIN_PROTOTYPE_ID_SHIFT) | (type))

/**
 * List of the built-in descriptors.
 */
static const ecma_builtin_descriptor_t ecma_builtin_descriptors[] = {
/** @cond doxygen_suppress */
#define BUILTIN(a, b, c, d, e)
#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ECMA_MAKE_BUILTIN_DESCRIPTOR (object_type, object_prototype_builtin_id),
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
#define BUILTIN_ROUTINE(a, b, c, d, e)
#define BUILTIN(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ECMA_MAKE_BUILTIN_DESCRIPTOR (object_type, object_prototype_builtin_id),
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
  /** @endcond */
};

#ifndef JERRY_NDEBUG
/** @cond doxygen_suppress */
enum
{
  ECMA_BUILTIN_EXTENSIBLE_CHECK =
#define BUILTIN(a, b, c, d, e)
#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  (is_extensible != 0 || builtin_id == ECMA_BUILTIN_ID_TYPE_ERROR_THROWER) &&
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
#define BUILTIN_ROUTINE(a, b, c, d, e)
#define BUILTIN(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  (is_extensible != 0 || builtin_id == ECMA_BUILTIN_ID_TYPE_ERROR_THROWER) &&
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
    true
};
/** @endcond */

/**
 * All the builtin object must be extensible except the ThrowTypeError object.
 */
JERRY_STATIC_ASSERT (ECMA_BUILTIN_EXTENSIBLE_CHECK == true,
                     ecma_builtin_must_be_extensible_except_the_builtin_throw_type_error_object);
#endif /* !JERRY_NDEBUG */

/**
 * List of the built-in routines.
 */
static const ecma_builtin_dispatch_routine_t ecma_builtin_routines[] = {
/** @cond doxygen_suppress */
#define BUILTIN(a, b, c, d, e)
#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ecma_builtin_##lowercase_name##_dispatch_routine,
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
#define BUILTIN_ROUTINE(a, b, c, d, e)
#define BUILTIN(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ecma_builtin_##lowercase_name##_dispatch_routine,
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
  /** @endcond */
};

/**
 * List of the built-in call functions.
 */
static const ecma_builtin_dispatch_call_t ecma_builtin_call_functions[] = {
/** @cond doxygen_suppress */
#define BUILTIN(a, b, c, d, e)
#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ecma_builtin_##lowercase_name##_dispatch_call,
#include "ecma-builtins.inc.h"
#undef BUILTIN_ROUTINE
#undef BUILTIN
  /** @endcond */
};

/**
 * List of the built-in construct functions.
 */
static const ecma_builtin_dispatch_call_t ecma_builtin_construct_functions[] = {
/** @cond doxygen_suppress */
#define BUILTIN(a, b, c, d, e)
#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ecma_builtin_##lowercase_name##_dispatch_construct,
#include "ecma-builtins.inc.h"
#undef BUILTIN_ROUTINE
#undef BUILTIN
  /** @endcond */
};

/**
 * Property descriptor lists for all built-ins.
 */
static const ecma_builtin_property_list_reference_t ecma_builtin_property_list_references[] = {
/** @cond doxygen_suppress */
#define BUILTIN(a, b, c, d, e)
#define BUILTIN_ROUTINE(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ecma_builtin_##lowercase_name##_property_descriptor_list,
#include "ecma-builtins.inc.h"
#undef BUILTIN
#undef BUILTIN_ROUTINE
#define BUILTIN_ROUTINE(a, b, c, d, e)
#define BUILTIN(builtin_id, object_type, object_prototype_builtin_id, is_extensible, lowercase_name) \
  ecma_builtin_##lowercase_name##_property_descriptor_list,
#include "ecma-builtins.inc.h"
#undef BUILTIN_ROUTINE
#undef BUILTIN
  /** @endcond */
};

/**
 * Get the number of properties of a built-in object.
 *
 * @return the number of properties
 */
static size_t
ecma_builtin_get_property_count (ecma_builtin_id_t builtin_id) /**< built-in ID */
{
  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_ID__COUNT);
  const ecma_builtin_property_descriptor_t *property_list_p = ecma_builtin_property_list_references[builtin_id];

  const ecma_builtin_property_descriptor_t *curr_property_p = property_list_p;

  while (curr_property_p->magic_string_id != LIT_MAGIC_STRING__COUNT)
  {
    curr_property_p++;
  }

  return (size_t) (curr_property_p - property_list_p);
} /* ecma_builtin_get_property_count */

/**
 * Check if passed object is a global built-in.
 *
 * @return true  - if the object is a global built-in
 *         false - otherwise
 */
bool
ecma_builtin_is_global (ecma_object_t *object_p) /**< pointer to an object */
{
  return (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_BUILT_IN_GENERAL
          && ((ecma_extended_object_t *) object_p)->u.built_in.id == ECMA_BUILTIN_ID_GLOBAL);
} /* ecma_builtin_is_global */

/**
 * Get reference to the global object
 *
 * Note:
 *   Does not increase the reference counter.
 *
 * @return pointer to the global object
 */
ecma_object_t *
ecma_builtin_get_global (void)
{
  JERRY_DEFINE_CURRENT_CONTEXT ();
  JERRY_ASSERT (JERRY_CONTEXT (global_object_p) != NULL);

  return (ecma_object_t *) JERRY_CONTEXT (global_object_p);
} /* ecma_builtin_get_global */

/**
 * Checks whether the given function is a built-in routine
 *
 * @return true - if the function object is a built-in routine
 *         false - otherwise
 */
bool
ecma_builtin_function_is_routine (ecma_object_t *func_obj_p) /**< function object */
{
  JERRY_ASSERT (ecma_get_object_type (func_obj_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);

  ecma_extended_object_t *ext_func_obj_p = (ecma_extended_object_t *) func_obj_p;
  return (ext_func_obj_p->u.built_in.routine_id != 0);
} /* ecma_builtin_function_is_routine */

#if JERRY_BUILTIN_REALMS

/**
 * Get reference to the realm provided by another built-in object
 *
 * Note:
 *   Does not increase the reference counter.
 *
 * @return pointer to the global object
 */
static ecma_global_object_t *
ecma_builtin_get_realm (ecma_object_t *builtin_object_p) /**< built-in object */
{
  ecma_object_type_t object_type = ecma_get_object_type (builtin_object_p);
  ecma_value_t realm_value;

  JERRY_ASSERT (object_type == ECMA_OBJECT_TYPE_BUILT_IN_GENERAL || object_type == ECMA_OBJECT_TYPE_BUILT_IN_CLASS
                || object_type == ECMA_OBJECT_TYPE_BUILT_IN_ARRAY || object_type == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);

  if (ECMA_BUILTIN_IS_EXTENDED_BUILT_IN (object_type))
  {
    realm_value = ((ecma_extended_built_in_object_t *) builtin_object_p)->built_in.realm_value;
  }
  else
  {
    realm_value = ((ecma_extended_object_t *) builtin_object_p)->u.built_in.realm_value;
  }

  return ECMA_GET_INTERNAL_VALUE_POINTER (ecma_global_object_t, realm_value);
} /* ecma_builtin_get_realm */

#endif /* JERRY_BUILTIN_REALMS */

/**
 * Instantiate specified ECMA built-in object
 *
 * @return the newly instantiated built-in
 */
static ecma_object_t *
ecma_instantiate_builtin (ecma_global_object_t *global_object_p, /**< global object */
                          ecma_builtin_id_t obj_builtin_id) /**< built-in id */
{
  jmem_cpointer_t *builtin_objects = global_object_p->builtin_objects;

  JERRY_ASSERT (obj_builtin_id < ECMA_BUILTIN_OBJECTS_COUNT);
  JERRY_ASSERT (builtin_objects[obj_builtin_id] == JMEM_CP_NULL);

  ecma_builtin_descriptor_t builtin_desc = ecma_builtin_descriptors[obj_builtin_id];
  ecma_builtin_id_t object_prototype_builtin_id = (ecma_builtin_id_t) (builtin_desc >> ECMA_BUILTIN_PROTOTYPE_ID_SHIFT);

  ecma_object_t *prototype_obj_p;

  if (JERRY_UNLIKELY (object_prototype_builtin_id == ECMA_BUILTIN_ID__COUNT))
  {
    prototype_obj_p = NULL;
  }
  else
  {
    if (builtin_objects[object_prototype_builtin_id] == JMEM_CP_NULL)
    {
      ecma_instantiate_builtin (global_object_p, object_prototype_builtin_id);
    }
    prototype_obj_p = ECMA_GET_NON_NULL_POINTER (ecma_object_t, builtin_objects[object_prototype_builtin_id]);
    JERRY_ASSERT (prototype_obj_p != NULL);
  }

  ecma_object_type_t obj_type = (ecma_object_type_t) (builtin_desc & ECMA_BUILTIN_OBJECT_TYPE_MASK);

  JERRY_ASSERT (obj_type == ECMA_OBJECT_TYPE_BUILT_IN_GENERAL || obj_type == ECMA_OBJECT_TYPE_BUILT_IN_CLASS
                || obj_type == ECMA_OBJECT_TYPE_BUILT_IN_ARRAY || obj_type == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);

  bool is_extended_built_in = ECMA_BUILTIN_IS_EXTENDED_BUILT_IN (obj_type);

  size_t ext_object_size =
    (is_extended_built_in ? sizeof (ecma_extended_built_in_object_t) : sizeof (ecma_extended_object_t));

  size_t property_count = ecma_builtin_get_property_count (obj_builtin_id);

  if (property_count > ECMA_BUILTIN_INSTANTIATED_BITSET_MIN_SIZE)
  {
    /* Only 64 extra properties supported at the moment.
     * This can be extended to 256 later. */
    JERRY_ASSERT (property_count <= (ECMA_BUILTIN_INSTANTIATED_BITSET_MIN_SIZE + 64));

    ext_object_size += sizeof (uint64_t);
  }

  ecma_object_t *obj_p = ecma_create_object (prototype_obj_p, ext_object_size, obj_type);

  if (JERRY_UNLIKELY (obj_builtin_id == ECMA_BUILTIN_ID_TYPE_ERROR_THROWER))
  {
    ecma_op_ordinary_object_prevent_extensions (obj_p);
  }
  else
  {
    ecma_op_ordinary_object_set_extensible (obj_p);
  }

  /*
   * [[Class]] property of built-in object is not stored explicitly.
   *
   * See also: ecma_object_get_class_name
   */

  ecma_built_in_props_t *built_in_props_p;

  if (is_extended_built_in)
  {
    built_in_props_p = &((ecma_extended_built_in_object_t *) obj_p)->built_in;
  }
  else
  {
    built_in_props_p = &((ecma_extended_object_t *) obj_p)->u.built_in;
  }

  built_in_props_p->id = (uint8_t) obj_builtin_id;
  built_in_props_p->routine_id = 0;
  built_in_props_p->u.length_and_bitset_size = 0;
  built_in_props_p->u2.instantiated_bitset[0] = 0;
#if JERRY_BUILTIN_REALMS
  ECMA_SET_INTERNAL_VALUE_POINTER (built_in_props_p->realm_value, global_object_p);
#else /* !JERRY_BUILTIN_REALMS */
  built_in_props_p->continue_instantiated_bitset[0] = 0;
#endif /* JERRY_BUILTIN_REALMS */

  if (property_count > ECMA_BUILTIN_INSTANTIATED_BITSET_MIN_SIZE)
  {
    built_in_props_p->u.length_and_bitset_size = 1 << ECMA_BUILT_IN_BITSET_SHIFT;

    uint32_t *instantiated_bitset_p = (uint32_t *) (built_in_props_p + 1);
    instantiated_bitset_p[0] = 0;
    instantiated_bitset_p[1] = 0;
  }

  /** Initializing [[PrimitiveValue]] properties of built-in prototype objects */
  switch (obj_builtin_id)
  {
#if JERRY_BUILTIN_ARRAY
    case ECMA_BUILTIN_ID_ARRAY_PROTOTYPE:
    {
      JERRY_ASSERT (obj_type == ECMA_OBJECT_TYPE_BUILT_IN_ARRAY);
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

      ext_object_p->u.array.length = 0;
      ext_object_p->u.array.length_prop_and_hole_count = ECMA_PROPERTY_FLAG_WRITABLE | ECMA_PROPERTY_VIRTUAL;
      break;
    }
#endif /* JERRY_BUILTIN_ARRAY */

#if JERRY_BUILTIN_STRING
    case ECMA_BUILTIN_ID_STRING_PROTOTYPE:
    {
      JERRY_ASSERT (obj_type == ECMA_OBJECT_TYPE_BUILT_IN_CLASS);
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

      ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_STRING;
      ext_object_p->u.cls.u3.value = ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);
      break;
    }
#endif /* JERRY_BUILTIN_STRING */

#if JERRY_BUILTIN_NUMBER
    case ECMA_BUILTIN_ID_NUMBER_PROTOTYPE:
    {
      JERRY_ASSERT (obj_type == ECMA_OBJECT_TYPE_BUILT_IN_CLASS);
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

      ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_NUMBER;
      ext_object_p->u.cls.u3.value = ecma_make_integer_value (0);
      break;
    }
#endif /* JERRY_BUILTIN_NUMBER */

#if JERRY_BUILTIN_BOOLEAN
    case ECMA_BUILTIN_ID_BOOLEAN_PROTOTYPE:
    {
      JERRY_ASSERT (obj_type == ECMA_OBJECT_TYPE_BUILT_IN_CLASS);
      ecma_extended_object_t *ext_object_p = (ecma_extended_object_t *) obj_p;

      ext_object_p->u.cls.type = ECMA_OBJECT_CLASS_BOOLEAN;
      ext_object_p->u.cls.u3.value = ECMA_VALUE_FALSE;
      break;
    }
#endif /* JERRY_BUILTIN_BOOLEAN */

    default:
    {
      JERRY_ASSERT (obj_type != ECMA_OBJECT_TYPE_BUILT_IN_CLASS);
      break;
    }
  }

  ECMA_SET_NON_NULL_POINTER (builtin_objects[obj_builtin_id], obj_p);
  ecma_deref_object (obj_p);
  return obj_p;
} /* ecma_instantiate_builtin */

/**
 * Create a global object
 *
 * @return a new global object
 */
ecma_global_object_t *
ecma_builtin_create_global_object (void)
{
  ecma_builtin_descriptor_t builtin_desc = ecma_builtin_descriptors[ECMA_BUILTIN_ID_GLOBAL];
  ecma_builtin_id_t prototype_builtin_id = (ecma_builtin_id_t) (builtin_desc >> ECMA_BUILTIN_PROTOTYPE_ID_SHIFT);
  ecma_object_type_t obj_type = (ecma_object_type_t) (builtin_desc & ECMA_BUILTIN_OBJECT_TYPE_MASK);
  ecma_builtin_get_property_count (ECMA_BUILTIN_ID_GLOBAL);
  ecma_object_t *object_p = ecma_create_object (NULL, sizeof (ecma_global_object_t), obj_type);

  ecma_op_ordinary_object_set_extensible (object_p);

  ecma_global_object_t *global_object_p = (ecma_global_object_t *) object_p;

  global_object_p->extended_object.u.built_in.id = (uint8_t) ECMA_BUILTIN_ID_GLOBAL;
  global_object_p->extended_object.u.built_in.routine_id = 0;
  /* Bitset size is ignored by the gc. */
  global_object_p->extended_object.u.built_in.u.length_and_bitset_size = 0;
  global_object_p->extended_object.u.built_in.u2.instantiated_bitset[0] = 0;
  global_object_p->extra_instantiated_bitset[0] = 0;
#if JERRY_BUILTIN_REALMS
  ECMA_SET_INTERNAL_VALUE_POINTER (global_object_p->extended_object.u.built_in.realm_value, global_object_p);
  global_object_p->extra_realms_bitset = 0;
  global_object_p->this_binding = ecma_make_object_value (object_p);
#else /* !JERRY_BUILTIN_REALMS */
  global_object_p->extended_object.u.built_in.continue_instantiated_bitset[0] = 0;
#endif /* JERRY_BUILTIN_REALMS */

  memset (global_object_p->builtin_objects, 0, (sizeof (jmem_cpointer_t) * ECMA_BUILTIN_OBJECTS_COUNT));

  /* Temporary self reference for GC mark. */
  ECMA_SET_NON_NULL_POINTER (global_object_p->global_env_cp, object_p);
  global_object_p->global_scope_cp = global_object_p->global_env_cp;

  ecma_object_t *global_lex_env_p = ecma_create_object_lex_env (NULL, object_p);

  ECMA_SET_NON_NULL_POINTER (global_object_p->global_env_cp, global_lex_env_p);
  global_object_p->global_scope_cp = global_object_p->global_env_cp;

  ecma_deref_object (global_lex_env_p);

  ecma_object_t *prototype_object_p;
  prototype_object_p = ecma_instantiate_builtin (global_object_p, prototype_builtin_id);
  JERRY_ASSERT (prototype_object_p != NULL);

  ECMA_SET_NON_NULL_POINTER (object_p->u2.prototype_cp, prototype_object_p);

  return global_object_p;
} /* ecma_builtin_create_global_object */

/**
 * Get reference to specified built-in object
 *
 * Note:
 *   Does not increase the reference counter.
 *
 * @return pointer to the object's instance
 */
ecma_object_t *
ecma_builtin_get (ecma_builtin_id_t builtin_id) /**< id of built-in to check on */
{
  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_OBJECTS_COUNT);

  ecma_global_object_t *global_object_p = (ecma_global_object_t *) ecma_builtin_get_global ();
  jmem_cpointer_t *builtin_p = global_object_p->builtin_objects + builtin_id;

  if (JERRY_UNLIKELY (*builtin_p == JMEM_CP_NULL))
  {
    return ecma_instantiate_builtin (global_object_p, builtin_id);
  }

  return ECMA_GET_NON_NULL_POINTER (ecma_object_t, *builtin_p);
} /* ecma_builtin_get */

#if JERRY_BUILTIN_REALMS

/**
 * Get reference to specified built-in object using the realm provided by another built-in object
 *
 * Note:
 *   Does not increase the reference counter.
 *
 * @return pointer to the object's instance
 */
ecma_object_t *
ecma_builtin_get_from_realm (ecma_global_object_t *global_object_p, /**< global object */
                             ecma_builtin_id_t builtin_id) /**< id of built-in to check on */
{
  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_OBJECTS_COUNT);

  jmem_cpointer_t *builtin_p = global_object_p->builtin_objects + builtin_id;

  if (JERRY_UNLIKELY (*builtin_p == JMEM_CP_NULL))
  {
    return ecma_instantiate_builtin (global_object_p, builtin_id);
  }

  return ECMA_GET_NON_NULL_POINTER (ecma_object_t, *builtin_p);
} /* ecma_builtin_get_from_realm */

#endif /* JERRY_BUILTIN_REALMS */

/**
 * Get reference to specified built-in object using the realm provided by another built-in object
 *
 * Note:
 *   Does not increase the reference counter.
 *
 * @return pointer to the object's instance
 */
static inline ecma_object_t *
ecma_builtin_get_from_builtin (ecma_object_t *builtin_object_p, /**< built-in object */
                               ecma_builtin_id_t builtin_id) /**< id of built-in to check on */
{
  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_OBJECTS_COUNT);

#if JERRY_BUILTIN_REALMS
  return ecma_builtin_get_from_realm (ecma_builtin_get_realm (builtin_object_p), builtin_id);
#else /* !JERRY_BUILTIN_REALMS */
  JERRY_UNUSED (builtin_object_p);
  return ecma_builtin_get (builtin_id);
#endif /* JERRY_BUILTIN_REALMS */
} /* ecma_builtin_get_from_builtin */

/**
 * Construct a Function object for specified built-in routine
 *
 * See also: ECMA-262 v5, 15
 *
 * @return pointer to constructed Function object
 */
static ecma_object_t *
ecma_builtin_make_function_object_for_routine (ecma_object_t *builtin_object_p, /**< builtin object */
                                               uint8_t routine_id, /**< builtin-wide identifier of the built-in
                                                                    *   object's routine property */
                                               uint32_t routine_index, /**< property descriptor index of routine */
                                               uint8_t flags) /**< see also: ecma_builtin_routine_flags */
{
  ecma_object_t *prototype_obj_p = ecma_builtin_get_from_builtin (builtin_object_p, ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE);

  size_t ext_object_size = sizeof (ecma_extended_object_t);

  ecma_object_t *func_obj_p = ecma_create_object (prototype_obj_p, ext_object_size, ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);

  JERRY_ASSERT (routine_id > 0);
  JERRY_ASSERT (routine_index <= UINT8_MAX);

  ecma_built_in_props_t *built_in_props_p;

  if (ECMA_BUILTIN_IS_EXTENDED_BUILT_IN (ecma_get_object_type (builtin_object_p)))
  {
    built_in_props_p = &((ecma_extended_built_in_object_t *) builtin_object_p)->built_in;
  }
  else
  {
    built_in_props_p = &((ecma_extended_object_t *) builtin_object_p)->u.built_in;
  }

  ecma_extended_object_t *ext_func_obj_p = (ecma_extended_object_t *) func_obj_p;
  ext_func_obj_p->u.built_in.id = built_in_props_p->id;
  ext_func_obj_p->u.built_in.routine_id = routine_id;
  ext_func_obj_p->u.built_in.u.routine_index = (uint8_t) routine_index;
  ext_func_obj_p->u.built_in.u2.routine_flags = flags;

#if JERRY_BUILTIN_REALMS
  ext_func_obj_p->u.built_in.realm_value = built_in_props_p->realm_value;
#endif /* JERRY_BUILTIN_REALMS */

  return func_obj_p;
} /* ecma_builtin_make_function_object_for_routine */

/**
 * Construct a Function object for specified built-in accessor getter
 *
 * @return pointer to constructed accessor getter Function object
 */
static ecma_object_t *
ecma_builtin_make_function_object_for_getter_accessor (ecma_object_t *builtin_object_p, /**< builtin object */
                                                       uint8_t routine_id, /**< builtin-wide id of the built-in
                                                                            *   object's routine property */
                                                       uint32_t routine_index) /**< property descriptor index
                                                                                *   of routine */
{
  return ecma_builtin_make_function_object_for_routine (builtin_object_p,
                                                        routine_id,
                                                        routine_index,
                                                        ECMA_BUILTIN_ROUTINE_GETTER);
} /* ecma_builtin_make_function_object_for_getter_accessor */

/**
 * Construct a Function object for specified built-in accessor setter
 *
 * @return pointer to constructed accessor getter Function object
 */
static ecma_object_t *
ecma_builtin_make_function_object_for_setter_accessor (ecma_object_t *builtin_object_p, /**< builtin object */
                                                       uint8_t routine_id, /**< builtin-wide id of the built-in
                                                                            *   object's routine property */
                                                       uint32_t routine_index) /**< property descriptor index
                                                                                *   of routine */
{
  return ecma_builtin_make_function_object_for_routine (builtin_object_p,
                                                        routine_id,
                                                        routine_index,
                                                        ECMA_BUILTIN_ROUTINE_SETTER);
} /* ecma_builtin_make_function_object_for_setter_accessor */

/**
 * Create specification defined properties for built-in native handlers.
 *
 * @return pointer property, if one was instantiated,
 *         NULL - otherwise.
 */
static ecma_property_t *
ecma_builtin_native_handler_try_to_instantiate_property (ecma_object_t *object_p, /**< object */
                                                         ecma_string_t *property_name_p) /**< property's name */
{
  JERRY_ASSERT (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;
  ecma_property_t *prop_p = NULL;

  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_NAME))
  {
    if ((ext_obj_p->u.built_in.u2.routine_flags & ECMA_NATIVE_HANDLER_FLAGS_NAME_INITIALIZED) == 0)
    {
      ecma_property_value_t *value_p =
        ecma_create_named_data_property (object_p, property_name_p, ECMA_PROPERTY_BUILT_IN_CONFIGURABLE, &prop_p);

      value_p->value = ecma_make_magic_string_value (LIT_MAGIC_STRING__EMPTY);
    }
  }
  else if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_LENGTH))
  {
    if ((ext_obj_p->u.built_in.u2.routine_flags & ECMA_NATIVE_HANDLER_FLAGS_LENGTH_INITIALIZED) == 0)
    {
      ecma_property_value_t *value_p =
        ecma_create_named_data_property (object_p, property_name_p, ECMA_PROPERTY_BUILT_IN_CONFIGURABLE, &prop_p);

      const uint8_t length = ecma_builtin_handler_get_length ((ecma_native_handler_id_t) ext_obj_p->u.built_in.routine_id);
      value_p->value = ecma_make_integer_value (length);
    }
  }

  return prop_p;
} /* ecma_builtin_native_handler_try_to_instantiate_property */

/**
 * Lazy instantiation of builtin routine property of builtin object
 *
 * If the property is not instantiated yet, instantiate the property and
 * return pointer to the instantiated property.
 *
 * @return pointer property, if one was instantiated,
 *         NULL - otherwise.
 */
ecma_property_t *
ecma_builtin_routine_try_to_instantiate_property (ecma_object_t *object_p, /**< object */
                                                  ecma_string_t *property_name_p) /**< property name */
{
  JERRY_ASSERT (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION
                && ecma_builtin_function_is_routine (object_p));

  ecma_extended_object_t *ext_func_p = (ecma_extended_object_t *) object_p;

  if (JERRY_UNLIKELY (ext_func_p->u.built_in.id == ECMA_BUILTIN_ID_HANDLER))
  {
    return ecma_builtin_native_handler_try_to_instantiate_property (object_p, property_name_p);
  }

  if (ecma_string_is_length (property_name_p))
  {
    /*
     * Lazy instantiation of 'length' property
     */
    ecma_property_t *len_prop_p;

    uint8_t *bitset_p = &ext_func_p->u.built_in.u2.routine_flags;

    if (*bitset_p & ECMA_BUILTIN_ROUTINE_LENGTH_INITIALIZED)
    {
      /* length property was already instantiated */
      return NULL;
    }

    /* We mark that the property was lazily instantiated,
     * as it is configurable and so can be deleted (ECMA-262 v6, 19.2.4.1) */
    ecma_property_value_t *len_prop_value_p =
      ecma_create_named_data_property (object_p, property_name_p, ECMA_PROPERTY_BUILT_IN_CONFIGURABLE, &len_prop_p);

    uint8_t length = 0;

    if (ext_func_p->u.built_in.u2.routine_flags & ECMA_BUILTIN_ROUTINE_SETTER)
    {
      length = 1;
    }
    else if (!(ext_func_p->u.built_in.u2.routine_flags & ECMA_BUILTIN_ROUTINE_GETTER))
    {
      uint8_t routine_index = ext_func_p->u.built_in.u.routine_index;
      const ecma_builtin_property_descriptor_t *property_list_p;

      property_list_p = ecma_builtin_property_list_references[ext_func_p->u.built_in.id];

      JERRY_ASSERT (property_list_p[routine_index].type == ECMA_BUILTIN_PROPERTY_ROUTINE);

      length = ECMA_GET_ROUTINE_LENGTH (property_list_p[routine_index].value);
    }

    len_prop_value_p->value = ecma_make_integer_value (length);
    return len_prop_p;
  }

  /*
   * Lazy instantiation of 'name' property
   */
  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_NAME))
  {
    uint8_t *bitset_p = &ext_func_p->u.built_in.u2.routine_flags;

    if (*bitset_p & ECMA_BUILTIN_ROUTINE_NAME_INITIALIZED)
    {
      /* name property was already instantiated */
      return NULL;
    }

    /* We mark that the property was lazily instantiated */
    ecma_property_t *name_prop_p;
    ecma_property_value_t *name_prop_value_p =
      ecma_create_named_data_property (object_p, property_name_p, ECMA_PROPERTY_BUILT_IN_CONFIGURABLE, &name_prop_p);

    uint8_t routine_index = ext_func_p->u.built_in.u.routine_index;
    const ecma_builtin_property_descriptor_t *property_list_p;

    property_list_p = ecma_builtin_property_list_references[ext_func_p->u.built_in.id];

    JERRY_ASSERT (property_list_p[routine_index].type == ECMA_BUILTIN_PROPERTY_ROUTINE
                  || property_list_p[routine_index].type == ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_WRITE
                  || property_list_p[routine_index].type == ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_ONLY);

    lit_magic_string_id_t name_id = (lit_magic_string_id_t) property_list_p[routine_index].magic_string_id;
    ecma_string_t *name_p;

    if (JERRY_UNLIKELY (name_id > LIT_NON_INTERNAL_MAGIC_STRING__COUNT))
    {
      /* Note: Whenever new intrinsic routine is being added this mapping should be updated as well! */
      if (JERRY_UNLIKELY (name_id == LIT_INTERNAL_MAGIC_STRING_ARRAY_PROTOTYPE_VALUES)
          || JERRY_UNLIKELY (name_id == LIT_INTERNAL_MAGIC_STRING_TYPEDARRAY_PROTOTYPE_VALUES)
          || JERRY_UNLIKELY (name_id == LIT_INTERNAL_MAGIC_STRING_SET_PROTOTYPE_VALUES))
      {
        name_p = ecma_get_magic_string (LIT_MAGIC_STRING_VALUES);
      }
      else if (JERRY_UNLIKELY (name_id == LIT_INTERNAL_MAGIC_STRING_MAP_PROTOTYPE_ENTRIES))
      {
        name_p = ecma_get_magic_string (LIT_MAGIC_STRING_ENTRIES);
      }
      else
      {
        JERRY_ASSERT (LIT_IS_GLOBAL_SYMBOL (name_id));
        name_p = ecma_op_get_global_symbol (name_id);
      }
    }
    else
    {
      name_p = ecma_get_magic_string (name_id);
    }

    const char *prefix_p = NULL;
    lit_utf8_size_t prefix_size = 0;

    if (*bitset_p & (ECMA_BUILTIN_ROUTINE_GETTER | ECMA_BUILTIN_ROUTINE_SETTER))
    {
      prefix_size = 4;
      prefix_p = (*bitset_p & ECMA_BUILTIN_ROUTINE_GETTER) ? "get " : "set ";
    }

    name_prop_value_p->value = ecma_op_function_form_name (name_p, (char*)prefix_p, prefix_size);

    if (JERRY_UNLIKELY (name_id > LIT_NON_INTERNAL_MAGIC_STRING__COUNT))
    {
      ecma_deref_ecma_string (name_p);
    }

    return name_prop_p;
  }

  return NULL;
} /* ecma_builtin_routine_try_to_instantiate_property */

/**
 * If the property's name is one of built-in properties of the object
 * that is not instantiated yet, instantiate the property and
 * return pointer to the instantiated property.
 *
 * @return pointer property, if one was instantiated,
 *         NULL - otherwise.
 */
ecma_property_t *
ecma_builtin_try_to_instantiate_property (ecma_object_t *object_p, /**< object */
                                          ecma_string_t *property_name_p) /**< property's name */
{
  lit_magic_string_id_t magic_string_id = ecma_get_string_magic (property_name_p);

  if (JERRY_UNLIKELY (ecma_prop_name_is_symbol (property_name_p)) && property_name_p->u.hash & ECMA_SYMBOL_FLAG_GLOBAL)
  {
    magic_string_id = (lit_magic_string_id_t) ((int) property_name_p->u.hash >> (int) ECMA_SYMBOL_FLAGS_SHIFT);
  }

  if (magic_string_id == LIT_MAGIC_STRING__COUNT)
  {
    return NULL;
  }

  ecma_built_in_props_t *built_in_props_p;
  ecma_object_type_t object_type = ecma_get_object_type (object_p);

  JERRY_ASSERT (object_type == ECMA_OBJECT_TYPE_BUILT_IN_GENERAL || object_type == ECMA_OBJECT_TYPE_BUILT_IN_CLASS
                || object_type == ECMA_OBJECT_TYPE_BUILT_IN_ARRAY
                || (object_type == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION && !ecma_builtin_function_is_routine (object_p)));

  if (ECMA_BUILTIN_IS_EXTENDED_BUILT_IN (object_type))
  {
    built_in_props_p = &((ecma_extended_built_in_object_t *) object_p)->built_in;
  }
  else
  {
    built_in_props_p = &((ecma_extended_object_t *) object_p)->u.built_in;
  }

  ecma_builtin_id_t builtin_id = (ecma_builtin_id_t) built_in_props_p->id;

  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_ID__COUNT);

  const ecma_builtin_property_descriptor_t *property_list_p = ecma_builtin_property_list_references[builtin_id];
  const ecma_builtin_property_descriptor_t *curr_property_p = property_list_p;

  while (curr_property_p->magic_string_id != magic_string_id)
  {
    if (curr_property_p->magic_string_id == LIT_MAGIC_STRING__COUNT)
    {
      return NULL;
    }
    curr_property_p++;
  }

  uint32_t index = (uint32_t) (curr_property_p - property_list_p);
  uint8_t *bitset_p = built_in_props_p->u2.instantiated_bitset + (index >> 3);

#if JERRY_BUILTIN_REALMS
  if (index >= 8 * sizeof (uint8_t))
  {
    bitset_p += sizeof (ecma_value_t);
  }
#endif /* JERRY_BUILTIN_REALMS */

  uint8_t bit_for_index = (uint8_t) (1u << (index & 0x7));

  if (*bitset_p & bit_for_index)
  {
    /* This property was instantiated before. */
    return NULL;
  }

  ecma_value_t value = ECMA_VALUE_EMPTY;
  bool is_accessor = false;
  ecma_object_t *getter_p = NULL;
  ecma_object_t *setter_p = NULL;

  switch (curr_property_p->type)
  {
    case ECMA_BUILTIN_PROPERTY_SIMPLE:
    {
      value = curr_property_p->value;

#if JERRY_BUILTIN_GLOBAL_THIS
      if (value == ECMA_VALUE_GLOBAL_THIS)
      {
        /* Only the global object has globalThis property. */
        JERRY_ASSERT (ecma_builtin_is_global (object_p));
        ecma_ref_object (object_p);
        value = ecma_make_object_value (object_p);
      }
#endif /* JERRY_BUILTIN_GLOBAL_THIS */
      break;
    }
    case ECMA_BUILTIN_PROPERTY_NUMBER:
    {
      ecma_number_t num = 0.0;

      if (curr_property_p->value < ECMA_BUILTIN_NUMBER_MAX)
      {
        num = curr_property_p->value;
      }
      else if (curr_property_p->value < ECMA_BUILTIN_NUMBER_NAN)
      {
        static const ecma_number_t builtin_number_list[] = {
          ECMA_NUMBER_MAX_VALUE,
          ECMA_NUMBER_MIN_VALUE,
          ECMA_NUMBER_EPSILON,
          ECMA_NUMBER_MAX_SAFE_INTEGER,
          ECMA_NUMBER_MIN_SAFE_INTEGER,
          ECMA_NUMBER_E,
          ECMA_NUMBER_PI,
          ECMA_NUMBER_LN10,
          ECMA_NUMBER_LN2,
          ECMA_NUMBER_LOG2E,
          ECMA_NUMBER_LOG10E,
          ECMA_NUMBER_SQRT2,
          ECMA_NUMBER_SQRT_1_2,
        };

        num = builtin_number_list[curr_property_p->value - ECMA_BUILTIN_NUMBER_MAX];
      }
      else
      {
        switch (curr_property_p->value)
        {
          case ECMA_BUILTIN_NUMBER_POSITIVE_INFINITY:
          {
            num = ecma_number_make_infinity (false);
            break;
          }
          case ECMA_BUILTIN_NUMBER_NEGATIVE_INFINITY:
          {
            num = ecma_number_make_infinity (true);
            break;
          }
          default:
          {
            JERRY_ASSERT (curr_property_p->value == ECMA_BUILTIN_NUMBER_NAN);

            num = ecma_number_make_nan ();
            break;
          }
        }
      }

      value = ecma_make_number_value (num);
      break;
    }
    case ECMA_BUILTIN_PROPERTY_STRING:
    {
      value = ecma_make_magic_string_value ((lit_magic_string_id_t) curr_property_p->value);
      break;
    }
    case ECMA_BUILTIN_PROPERTY_SYMBOL:
    {
      lit_magic_string_id_t symbol_id = (lit_magic_string_id_t) curr_property_p->value;

      value = ecma_make_symbol_value (ecma_op_get_global_symbol (symbol_id));
      break;
    }
    case ECMA_BUILTIN_PROPERTY_INTRINSIC_PROPERTY:
    {
      ecma_object_t *intrinsic_object_p = ecma_builtin_get_from_builtin (object_p, ECMA_BUILTIN_ID_INTRINSIC_OBJECT);
      value = ecma_op_object_get_by_magic_id (intrinsic_object_p, (lit_magic_string_id_t) curr_property_p->value);
      break;
    }
    case ECMA_BUILTIN_PROPERTY_ACCESSOR_BUILTIN_FUNCTION:
    {
      is_accessor = true;
      uint16_t getter_id = ECMA_ACCESSOR_READ_WRITE_GET_GETTER_ID (curr_property_p->value);
      uint16_t setter_id = ECMA_ACCESSOR_READ_WRITE_GET_SETTER_ID (curr_property_p->value);
      getter_p = ecma_builtin_get_from_builtin (object_p, (ecma_builtin_id_t) getter_id);
      setter_p = ecma_builtin_get_from_builtin (object_p, (ecma_builtin_id_t) setter_id);
      ecma_ref_object (getter_p);
      ecma_ref_object (setter_p);
      break;
    }
    case ECMA_BUILTIN_PROPERTY_OBJECT:
    {
      ecma_object_t *builtin_object_p;
      builtin_object_p = ecma_builtin_get_from_builtin (object_p, (ecma_builtin_id_t) curr_property_p->value);
      ecma_ref_object (builtin_object_p);
      value = ecma_make_object_value (builtin_object_p);
      break;
    }
    case ECMA_BUILTIN_PROPERTY_ROUTINE:
    {
      ecma_object_t *func_obj_p;
      func_obj_p = ecma_builtin_make_function_object_for_routine (object_p,
                                                                  ECMA_GET_ROUTINE_ID (curr_property_p->value),
                                                                  index,
                                                                  ECMA_BUILTIN_ROUTINE_NO_OPTS);
      value = ecma_make_object_value (func_obj_p);
      break;
    }
    case ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_WRITE:
    {
      is_accessor = true;
      uint8_t getter_id = ECMA_ACCESSOR_READ_WRITE_GET_GETTER_ID (curr_property_p->value);
      uint8_t setter_id = ECMA_ACCESSOR_READ_WRITE_GET_SETTER_ID (curr_property_p->value);
      getter_p = ecma_builtin_make_function_object_for_getter_accessor (object_p, getter_id, index);
      setter_p = ecma_builtin_make_function_object_for_setter_accessor (object_p, setter_id, index);
      break;
    }
    default:
    {
      JERRY_ASSERT (curr_property_p->type == ECMA_BUILTIN_PROPERTY_ACCESSOR_READ_ONLY);

      is_accessor = true;
      uint8_t getter_id = (uint8_t) curr_property_p->value;
      getter_p = ecma_builtin_make_function_object_for_getter_accessor (object_p, getter_id, index);
      break;
    }
  }

  ecma_property_t *prop_p;

  JERRY_ASSERT (curr_property_p->attributes & ECMA_PROPERTY_FLAG_BUILT_IN);

  if (is_accessor)
  {
    ecma_create_named_accessor_property (object_p,
                                         property_name_p,
                                         getter_p,
                                         setter_p,
                                         curr_property_p->attributes,
                                         &prop_p);

    if (setter_p)
    {
      ecma_deref_object (setter_p);
    }
    if (getter_p)
    {
      ecma_deref_object (getter_p);
    }
  }
  else
  {
    ecma_property_value_t *prop_value_p =
      ecma_create_named_data_property (object_p, property_name_p, curr_property_p->attributes, &prop_p);
    prop_value_p->value = value;

    /* Reference count of objects must be decreased. */
    ecma_deref_if_object (value);
  }

  return prop_p;
} /* ecma_builtin_try_to_instantiate_property */

/**
 * Delete configurable properties of native handlers.
 */
static void
ecma_builtin_native_handler_delete_built_in_property (ecma_object_t *object_p, /**< object */
                                                      ecma_string_t *property_name_p) /**< property name */
{
  ecma_extended_object_t *extended_obj_p = (ecma_extended_object_t *) object_p;

  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_LENGTH))
  {
    JERRY_ASSERT (!(extended_obj_p->u.built_in.u2.routine_flags & ECMA_NATIVE_HANDLER_FLAGS_LENGTH_INITIALIZED));

    extended_obj_p->u.built_in.u2.routine_flags |= ECMA_NATIVE_HANDLER_FLAGS_LENGTH_INITIALIZED;
    return;
  }

  JERRY_ASSERT (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_NAME));
  JERRY_ASSERT (!(extended_obj_p->u.built_in.u2.routine_flags & ECMA_NATIVE_HANDLER_FLAGS_NAME_INITIALIZED));

  extended_obj_p->u.built_in.u2.routine_flags |= ECMA_NATIVE_HANDLER_FLAGS_NAME_INITIALIZED;
} /* ecma_builtin_native_handler_delete_built_in_property */

/**
 * Delete configurable properties of built-in routines.
 */
void
ecma_builtin_routine_delete_built_in_property (ecma_object_t *object_p, /**< object */
                                               ecma_string_t *property_name_p) /**< property name */
{
  JERRY_ASSERT (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION
                && ecma_builtin_function_is_routine (object_p));

  ecma_extended_object_t *extended_obj_p = (ecma_extended_object_t *) object_p;

  if (JERRY_UNLIKELY (extended_obj_p->u.built_in.id == ECMA_BUILTIN_ID_HANDLER))
  {
    ecma_builtin_native_handler_delete_built_in_property (object_p, property_name_p);
    return;
  }

  uint8_t *bitset_p = &extended_obj_p->u.built_in.u2.routine_flags;

  if (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_LENGTH))
  {
    JERRY_ASSERT (!(*bitset_p & ECMA_BUILTIN_ROUTINE_LENGTH_INITIALIZED));

    *bitset_p |= ECMA_BUILTIN_ROUTINE_LENGTH_INITIALIZED;
    return;
  }

  JERRY_ASSERT (ecma_compare_ecma_string_to_magic_id (property_name_p, LIT_MAGIC_STRING_NAME));
  JERRY_ASSERT (!(*bitset_p & ECMA_BUILTIN_ROUTINE_NAME_INITIALIZED));

  *bitset_p |= ECMA_BUILTIN_ROUTINE_NAME_INITIALIZED;
} /* ecma_builtin_routine_delete_built_in_property */

/**
 * Delete configurable properties of built-ins.
 */
void
ecma_builtin_delete_built_in_property (ecma_object_t *object_p, /**< object */
                                       ecma_string_t *property_name_p) /**< property name */
{
  lit_magic_string_id_t magic_string_id = ecma_get_string_magic (property_name_p);

  if (JERRY_UNLIKELY (ecma_prop_name_is_symbol (property_name_p)))
  {
    if (property_name_p->u.hash & ECMA_SYMBOL_FLAG_GLOBAL)
    {
      magic_string_id = (lit_magic_string_id_t ) ((int) property_name_p->u.hash >> ECMA_SYMBOL_FLAGS_SHIFT);
    }
  }

  ecma_built_in_props_t *built_in_props_p;
  ecma_object_type_t object_type = ecma_get_object_type (object_p);

  JERRY_ASSERT (object_type == ECMA_OBJECT_TYPE_BUILT_IN_GENERAL || object_type == ECMA_OBJECT_TYPE_BUILT_IN_CLASS
                || object_type == ECMA_OBJECT_TYPE_BUILT_IN_ARRAY
                || (object_type == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION && !ecma_builtin_function_is_routine (object_p)));

  if (ECMA_BUILTIN_IS_EXTENDED_BUILT_IN (object_type))
  {
    built_in_props_p = &((ecma_extended_built_in_object_t *) object_p)->built_in;
  }
  else
  {
    built_in_props_p = &((ecma_extended_object_t *) object_p)->u.built_in;
  }

  ecma_builtin_id_t builtin_id = (ecma_builtin_id_t) built_in_props_p->id;

  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_ID__COUNT);

  const ecma_builtin_property_descriptor_t *property_list_p = ecma_builtin_property_list_references[builtin_id];
  const ecma_builtin_property_descriptor_t *curr_property_p = property_list_p;

  while (curr_property_p->magic_string_id != magic_string_id)
  {
    JERRY_ASSERT (curr_property_p->magic_string_id != LIT_MAGIC_STRING__COUNT);
    curr_property_p++;
  }

  uint32_t index = (uint32_t) (curr_property_p - property_list_p);
  uint8_t *bitset_p = built_in_props_p->u2.instantiated_bitset + (index >> 3);

#if JERRY_BUILTIN_REALMS
  if (index >= 8 * sizeof (uint8_t))
  {
    bitset_p += sizeof (ecma_value_t);
  }
#endif /* JERRY_BUILTIN_REALMS */

  uint8_t bit_for_index = (uint8_t) (1u << (index & 0x7));
  JERRY_ASSERT (!(*bitset_p & bit_for_index));

  *bitset_p |= bit_for_index;
} /* ecma_builtin_delete_built_in_property */

/**
 * List names of an Built-in native handler object's lazy instantiated properties,
 * adding them to corresponding string collections
 */
static void
ecma_builtin_native_handler_list_lazy_property_names (ecma_object_t *object_p, /**< function object */
                                                      ecma_collection_t *prop_names_p, /**< prop name collection */
                                                      ecma_property_counter_t *prop_counter_p) /**< prop counter */
{
  JERRY_ASSERT (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);
  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) object_p;

  if ((ext_obj_p->u.built_in.u2.routine_flags & ECMA_NATIVE_HANDLER_FLAGS_NAME_INITIALIZED) == 0)
  {
    ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_NAME));
    prop_counter_p->string_named_props++;
  }

  if ((ext_obj_p->u.built_in.u2.routine_flags & ECMA_NATIVE_HANDLER_FLAGS_LENGTH_INITIALIZED) == 0)
  {
    ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_LENGTH));
    prop_counter_p->string_named_props++;
  }
} /* ecma_builtin_native_handler_list_lazy_property_names */

/**
 * List names of a built-in function's lazy instantiated properties
 *
 * See also:
 *          ecma_builtin_routine_try_to_instantiate_property
 */
void
ecma_builtin_routine_list_lazy_property_names (ecma_object_t *object_p, /**< a built-in object */
                                               ecma_collection_t *prop_names_p, /**< prop name collection */
                                               ecma_property_counter_t *prop_counter_p, /**< property counters */
                                               jerry_property_filter_t filter) /**< name filters */
{
  JERRY_ASSERT (ecma_get_object_type (object_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);
  JERRY_ASSERT (ecma_builtin_function_is_routine (object_p));

  if (filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS)
  {
    return;
  }

  ecma_extended_object_t *ext_func_p = (ecma_extended_object_t *) object_p;

  if (JERRY_UNLIKELY (ext_func_p->u.built_in.id == ECMA_BUILTIN_ID_HANDLER))
  {
    ecma_builtin_native_handler_list_lazy_property_names (object_p, prop_names_p, prop_counter_p);
    return;
  }

  if (!(ext_func_p->u.built_in.u2.routine_flags & ECMA_BUILTIN_ROUTINE_LENGTH_INITIALIZED))
  {
    /* Uninitialized 'length' property is non-enumerable (ECMA-262 v6, 19.2.4.1) */
    ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_LENGTH));
    prop_counter_p->string_named_props++;
  }
  if (!(ext_func_p->u.built_in.u2.routine_flags & ECMA_BUILTIN_ROUTINE_NAME_INITIALIZED))
  {
    /* Uninitialized 'name' property is non-enumerable (ECMA-262 v6, 19.2.4.2) */
    ecma_collection_push_back (prop_names_p, ecma_make_magic_string_value (LIT_MAGIC_STRING_NAME));
    prop_counter_p->string_named_props++;
  }
} /* ecma_builtin_routine_list_lazy_property_names */

/**
 * List names of a built-in object's lazy instantiated properties
 *
 * See also:
 *          ecma_builtin_try_to_instantiate_property
 */
void
ecma_builtin_list_lazy_property_names (ecma_object_t *object_p, /**< a built-in object */
                                       ecma_collection_t *prop_names_p, /**< prop name collection */
                                       ecma_property_counter_t *prop_counter_p, /**< property counters */
                                       jerry_property_filter_t filter) /**< name filters */
{
  JERRY_ASSERT (ecma_get_object_type (object_p) != ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION
                || !ecma_builtin_function_is_routine (object_p));

  ecma_built_in_props_t *built_in_props_p;
  ecma_object_type_t object_type = ecma_get_object_type (object_p);

  if (ECMA_BUILTIN_IS_EXTENDED_BUILT_IN (object_type))
  {
    built_in_props_p = &((ecma_extended_built_in_object_t *) object_p)->built_in;
  }
  else
  {
    built_in_props_p = &((ecma_extended_object_t *) object_p)->u.built_in;
  }

  ecma_builtin_id_t builtin_id = (ecma_builtin_id_t) built_in_props_p->id;

  JERRY_ASSERT (builtin_id < ECMA_BUILTIN_ID__COUNT);

#if JERRY_BUILTIN_REALMS
  uint8_t *bitset_p = built_in_props_p->u2.instantiated_bitset + 1 + sizeof (ecma_value_t);
#else /* !JERRY_BUILTIN_REALMS */
  uint8_t *bitset_p = built_in_props_p->u2.instantiated_bitset + 1;
#endif /* JERRY_BUILTIN_REALMS */

  uint8_t *symbol_bitset_p = bitset_p;
  bool has_symbol = true;

  if (!(filter & JERRY_PROPERTY_FILTER_EXCLUDE_STRINGS))
  {
    const ecma_builtin_property_descriptor_t *curr_property_p = ecma_builtin_property_list_references[builtin_id];
    uint8_t bitset = built_in_props_p->u2.instantiated_bitset[0];
    uint32_t index = 0;

    has_symbol = false;

    while (curr_property_p->magic_string_id != LIT_MAGIC_STRING__COUNT)
    {
      if (index == 8)
      {
        bitset = *bitset_p++;
        index = 0;
      }

      uint32_t bit_for_index = (uint32_t) 1u << index;

      if (!(bitset & bit_for_index))
      {
        if (JERRY_LIKELY (curr_property_p->magic_string_id < LIT_NON_INTERNAL_MAGIC_STRING__COUNT))
        {
          ecma_value_t name = ecma_make_magic_string_value ((lit_magic_string_id_t) curr_property_p->magic_string_id);
          ecma_collection_push_back (prop_names_p, name);
          prop_counter_p->string_named_props++;
        }
        else
        {
          JERRY_ASSERT (LIT_IS_GLOBAL_SYMBOL (curr_property_p->magic_string_id));
          has_symbol = true;
        }
      }

      curr_property_p++;
      index++;
    }
  }

  if (has_symbol && !(filter & JERRY_PROPERTY_FILTER_EXCLUDE_SYMBOLS))
  {
    const ecma_builtin_property_descriptor_t *curr_property_p = ecma_builtin_property_list_references[builtin_id];
    uint8_t bitset = built_in_props_p->u2.instantiated_bitset[0];
    uint32_t index = 0;

    while (curr_property_p->magic_string_id != LIT_MAGIC_STRING__COUNT)
    {
      if (index == 8)
      {
        bitset = *symbol_bitset_p++;
        index = 0;
      }

      uint32_t bit_for_index = (uint32_t) 1u << index;

      if (curr_property_p->magic_string_id > LIT_NON_INTERNAL_MAGIC_STRING__COUNT && !(bitset & bit_for_index))
      {
        ecma_string_t *name_p = ecma_op_get_global_symbol ((lit_magic_string_id_t ) curr_property_p->magic_string_id);
        ecma_collection_push_back (prop_names_p, ecma_make_symbol_value (name_p));
        prop_counter_p->symbol_named_props++;
      }

      curr_property_p++;
      index++;
    }
  }
} /* ecma_builtin_list_lazy_property_names */

/**
 * Dispatcher of built-in routines
 *
 * @return ecma value
 *         Returned value must be freed with ecma_free_value.
 */
static ecma_value_t
ecma_builtin_dispatch_routine (ecma_extended_object_t *func_obj_p, /**< builtin object */
                               ecma_value_t this_arg_value, /**< 'this' argument value */
                               const ecma_value_t *arguments_list_p, /**< list of arguments passed to routine */
                               uint32_t arguments_list_len) /**< length of arguments' list */
{
  JERRY_ASSERT (ecma_builtin_function_is_routine ((ecma_object_t *) func_obj_p));

  ecma_value_t padded_arguments_list_p[3] = { ECMA_VALUE_UNDEFINED, ECMA_VALUE_UNDEFINED, ECMA_VALUE_UNDEFINED };

  if (arguments_list_len <= 2)
  {
    switch (arguments_list_len)
    {
      case 2:
      {
        padded_arguments_list_p[1] = arguments_list_p[1];
        /* FALLTHRU */
      }
      case 1:
      {
        padded_arguments_list_p[0] = arguments_list_p[0];
        break;
      }
      default:
      {
        JERRY_ASSERT (arguments_list_len == 0);
      }
    }

    arguments_list_p = padded_arguments_list_p;
  }

  return ecma_builtin_routines[func_obj_p->u.built_in.id](func_obj_p->u.built_in.routine_id,
                                                          this_arg_value,
                                                          arguments_list_p,
                                                          arguments_list_len);
} /* ecma_builtin_dispatch_routine */

/**
 * Handle calling [[Call]] of built-in object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_dispatch_call (ecma_object_t *obj_p, /**< built-in object */
                            ecma_value_t this_arg_value, /**< 'this' argument value */
                            const ecma_value_t *arguments_list_p, /**< arguments list */
                            uint32_t arguments_list_len) /**< arguments list length */
{
  JERRY_ASSERT (ecma_get_object_type (obj_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) obj_p;

  if (ecma_builtin_function_is_routine (obj_p))
  {
    if (JERRY_UNLIKELY (ext_obj_p->u.built_in.id == ECMA_BUILTIN_ID_HANDLER))
    {
      ecma_builtin_handler_t handler = ecma_builtin_handler_get ((ecma_native_handler_id_t ) ext_obj_p->u.built_in.routine_id);
      return handler (obj_p, arguments_list_p, arguments_list_len);
    }

    return ecma_builtin_dispatch_routine (ext_obj_p, this_arg_value, arguments_list_p, arguments_list_len);
  }

  ecma_builtin_id_t builtin_object_id = (ecma_builtin_id_t) ext_obj_p->u.built_in.id;
  JERRY_ASSERT (builtin_object_id < sizeof (ecma_builtin_call_functions) / sizeof (ecma_builtin_dispatch_call_t));
  return ecma_builtin_call_functions[builtin_object_id](arguments_list_p, arguments_list_len);
} /* ecma_builtin_dispatch_call */

/**
 * Handle calling [[Construct]] of built-in object
 *
 * @return ecma value
 */
ecma_value_t
ecma_builtin_dispatch_construct (ecma_object_t *obj_p, /**< built-in object */
                                 const ecma_value_t *arguments_list_p, /**< arguments list */
                                 uint32_t arguments_list_len) /**< arguments list length */
{
  JERRY_ASSERT (ecma_get_object_type (obj_p) == ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION);

  if (ecma_builtin_function_is_routine (obj_p))
  {
    return ecma_raise_type_error (ECMA_ERR_BUILTIN_ROUTINES_HAVE_NO_CONSTRUCTOR);
  }

  ecma_extended_object_t *ext_obj_p = (ecma_extended_object_t *) obj_p;
  ecma_builtin_id_t builtin_object_id = (ecma_builtin_id_t) ext_obj_p->u.built_in.id;
  JERRY_ASSERT (builtin_object_id < sizeof (ecma_builtin_construct_functions) / sizeof (ecma_builtin_dispatch_call_t));

  return ecma_builtin_construct_functions[builtin_object_id](arguments_list_p, arguments_list_len);
} /* ecma_builtin_dispatch_construct */

/**
 * @}
 * @}
 */
