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

#ifndef ECMA_GLOBALS_H
#define ECMA_GLOBALS_H

#include "jerry-config.h"
#include "ecma-errors.h"
#include "jmem.h"
#include "jrt.h"
#include "lit-magic-strings.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmatypes ECMA types
 * @{
 *
 * \addtogroup compressedpointer Compressed pointer
 * @{
 */

/**
 * The NULL value for compressed pointers
 */
#define ECMA_NULL_POINTER JMEM_CP_NULL

#if defined(JMEM_CAN_STORE_POINTER_VALUE_DIRECTLY)

/**
 * JMEM_ALIGNMENT_LOG aligned pointers can be stored directly in ecma_value_t
 */
#define ECMA_VALUE_CAN_STORE_UINTPTR_VALUE_DIRECTLY

#endif /* JMEM_CAN_STORE_POINTER_VALUE_DIRECTLY */

/**
 * @}
 */

/**
 * JerryScript status flags.
 */
typedef enum
{
  ECMA_STATUS_API_ENABLED = (1u << 0), /**< api available */
  ECMA_STATUS_DIRECT_EVAL = (1u << 1), /**< eval is called directly */
#if JERRY_PROPERTY_HASHMAP
  ECMA_STATUS_HIGH_PRESSURE_GC = (1u << 2), /**< last gc was under high pressure */
#endif /* JERRY_PROPERTY_HASHMAP */
  ECMA_STATUS_EXCEPTION = (1u << 3), /**< last exception is a normal exception */
  ECMA_STATUS_ABORT = (1u << 4), /**< last exception is an abort */
  ECMA_STATUS_ERROR_UPDATE = (1u << 5), /**< the error_object_created_callback_p is called */
#if JERRY_VM_THROW
  ECMA_STATUS_ERROR_THROWN = (1u << 6), /**< the vm_throw_callback_p is called */
#endif /* JERRY_VM_THROW */
} ecma_status_flag_t;

/**
 * Type of ecma value
 */
typedef enum
{
  ECMA_TYPE_DIRECT = 0, /**< directly encoded value, a 28 bit signed integer or a simple value */
  ECMA_TYPE_STRING = 1, /**< pointer to description of a string */
  ECMA_TYPE_FLOAT = 2, /**< pointer to a 64 or 32 bit floating point number */
  ECMA_TYPE_OBJECT = 3, /**< pointer to description of an object */
  ECMA_TYPE_SYMBOL = 4, /**< pointer to description of a symbol */
  ECMA_TYPE_DIRECT_STRING = 5, /**< directly encoded string values */
  ECMA_TYPE_BIGINT = 6, /**< pointer to a bigint primitive */
  ECMA_TYPE_ERROR = 7, /**< pointer to description of an error reference (only supported by C API) */
  ECMA_TYPE_SNAPSHOT_OFFSET = ECMA_TYPE_ERROR, /**< offset to a snapshot number/string */
  ECMA_TYPE___MAX = ECMA_TYPE_ERROR /** highest value for ecma types */
} ecma_type_t;

/**
 * Option flags for parser_parse_script and internal flags for global_status_flags in parser context.
 */
typedef enum
{
  ECMA_PARSE_NO_OPTS = 0, /**< no options passed */
  ECMA_PARSE_STRICT_MODE = (1u << 0), /**< enable strict mode, must be same as PARSER_IS_STRICT */
  ECMA_PARSE_MODULE = (1u << 1), /**< module is parsed */
  ECMA_PARSE_EVAL = (1u << 2), /**< eval is called */
  ECMA_PARSE_DIRECT_EVAL = (1u << 3), /**< eval is called directly (ECMA-262 v5, 15.1.2.1.1) */
  ECMA_PARSE_CLASS_CONSTRUCTOR = (1u << 4), /**< a class constructor is being parsed */

  /* These five status flags must be in this order. The first four are also parser status flags.
   * See PARSER_SAVE_STATUS_FLAGS / PARSER_RESTORE_STATUS_FLAGS. */
  ECMA_PARSE_ALLOW_SUPER = (1u << 5), /**< allow super property access */
  ECMA_PARSE_ALLOW_SUPER_CALL = (1u << 6), /**< allow super constructor call */
  ECMA_PARSE_FUNCTION_IS_PARSING_ARGS = (1u << 7), /**< set when parsing function arguments */
  ECMA_PARSE_INSIDE_CLASS_FIELD = (1u << 8), /**< a class field is being parsed */
  ECMA_PARSE_ALLOW_NEW_TARGET = (1u << 9), /**< allow new.target access */
  ECMA_PARSE_FUNCTION_CONTEXT = (1u << 10), /**< function context is present (ECMA_PARSE_DIRECT_EVAL must be set) */

  ECMA_PARSE_HAS_SOURCE_VALUE = (1u << 11), /**< source_p points to a value list
                                             *   and the first value is the source code */
  ECMA_PARSE_HAS_ARGUMENT_LIST_VALUE = (1u << 12), /**< source_p points to a value list
                                                    *   and the second value is the argument list */
  ECMA_PARSE_GENERATOR_FUNCTION = (1u << 13), /**< generator function is parsed */
  ECMA_PARSE_ASYNC_FUNCTION = (1u << 14), /**< async function is parsed */

  /* These flags are internally used by the parser. */
  ECMA_PARSE_INTERNAL_FREE_SOURCE = (1u << 15), /**< free source_p data */
  ECMA_PARSE_INTERNAL_FREE_ARG_LIST = (1u << 16), /**< free arg_list_p data */
  ECMA_PARSE_INTERNAL_PRE_SCANNING = (1u << 17), /**< the parser is in pre-scanning mode */
#if JERRY_MODULE_SYSTEM
  ECMA_PARSE_INTERNAL_HAS_IMPORT_META = (1u << 18), /**< module has import.meta expression */
#endif /* JERRY_MODULE_SYSTEM */
#ifndef JERRY_NDEBUG
  /**
   * This flag represents an error in for in/of statements, which cannot be set
   * if the parsing is completed successfully.
   */
  ECMA_PARSE_INTERNAL_FOR_IN_OFF_CONTEXT_ERROR = (1u << 30),
#endif /* !JERRY_NDEBUG */
} ecma_parse_opts_t;

/**
 * Description of an ecma value
 *
 * Bit-field structure: type (3) | value (29)
 */
typedef uint32_t ecma_value_t;

/**
 * Type for directly encoded integer numbers in JerryScript.
 */
typedef int32_t ecma_integer_value_t;

/**
 * Mask for ecma types in ecma_value_t
 */
#define ECMA_VALUE_TYPE_MASK 0x7u

/**
 * Shift for value part in ecma_value_t
 */
#define ECMA_VALUE_SHIFT 3

/**
 * Mask for directly encoded values
 */
#define ECMA_DIRECT_TYPE_MASK ((1u << ECMA_VALUE_SHIFT) | ECMA_VALUE_TYPE_MASK)

/**
 * Ecma integer value type
 */
#define ECMA_DIRECT_TYPE_INTEGER_VALUE ((0u << ECMA_VALUE_SHIFT) | ECMA_TYPE_DIRECT)

/**
 * Ecma simple value type
 */
#define ECMA_DIRECT_TYPE_SIMPLE_VALUE ((1u << ECMA_VALUE_SHIFT) | ECMA_TYPE_DIRECT)

/**
 * Shift for directly encoded values in ecma_value_t
 */
#define ECMA_DIRECT_SHIFT 4

/**
 * ECMA make simple value
 */
#define ECMA_MAKE_VALUE(value) ((((ecma_value_t) (value)) << ECMA_DIRECT_SHIFT) | ECMA_DIRECT_TYPE_SIMPLE_VALUE)

/**
 * Simple ecma values
 */
enum
{
  /**
   * Empty value is implementation defined value, used for representing:
   *   - empty (uninitialized) values
   *   - immutable binding values
   *   - special register or stack values for vm
   */
  ECMA_VALUE_EMPTY = ECMA_MAKE_VALUE (0), /**< uninitialized value */
  ECMA_VALUE_ERROR = ECMA_MAKE_VALUE (1), /**< an error is currently thrown */
  ECMA_VALUE_FALSE = ECMA_MAKE_VALUE (2), /**< boolean false */
  ECMA_VALUE_TRUE = ECMA_MAKE_VALUE (3), /**< boolean true */
  ECMA_VALUE_UNDEFINED = ECMA_MAKE_VALUE (4), /**< undefined value */
  ECMA_VALUE_NULL = ECMA_MAKE_VALUE (5), /**< null value */
  ECMA_VALUE_UNINITIALIZED = ECMA_MAKE_VALUE (6), /**< a special value for uninitialized let/const declarations */
  ECMA_VALUE_NOT_FOUND = ECMA_MAKE_VALUE (7), /**< a special value returned by
                                               *   ecma_op_object_find */
  /* Values for controlling the VM */
  ECMA_VALUE_ARRAY_HOLE = ECMA_MAKE_VALUE (8), /**< array hole, used for
                                                *   initialization of an array literal */
  ECMA_VALUE_REGISTER_REF = ECMA_MAKE_VALUE (9), /**< register reference,
                                                  *   a special "base" value for vm */
  ECMA_VALUE_RELEASE_LEX_ENV = ECMA_MAKE_VALUE (10), /**< if this error remains on the stack when an exception occurs
                                                         the top lexical environment of the VM frame should be popped */
  ECMA_VALUE_SPREAD_ELEMENT = ECMA_MAKE_VALUE (11), /**< a special value for spread elements in array initialization
                                                     *   or function call argument list */
  /* Other values */
  ECMA_VALUE_ARGUMENT_NO_TRACK = ECMA_MAKE_VALUE (12), /**< represents not-tracked arguments formal parameter */
  ECMA_VALUE_SYNC_ITERATOR = ECMA_MAKE_VALUE (13), /**< option for ecma_op_get_iterator: sync iterator is requested */
  ECMA_VALUE_ASYNC_ITERATOR = ECMA_MAKE_VALUE (14), /**< option for ecma_op_get_iterator: async iterator is requested */
#if JERRY_BUILTIN_GLOBAL_THIS
  ECMA_VALUE_GLOBAL_THIS = ECMA_MAKE_VALUE (15), /**< globalThis built-in */
#endif /* JERRY_BUILTIN_GLOBAL_THIS */
};

/**
 * Maximum integer number for an ecma value
 */
#define ECMA_INTEGER_NUMBER_MAX 0x7fffff
/**
 * Maximum integer number for an ecma value (shifted left with ECMA_DIRECT_SHIFT)
 */
#define ECMA_INTEGER_NUMBER_MAX_SHIFTED 0x7fffff0

/**
 * Minimum integer number for an ecma value
 */
#define ECMA_INTEGER_NUMBER_MIN -0x7fffff
/**
 * Minimum integer number for an ecma value (shifted left with ECMA_DIRECT_SHIFT)
 */
#define ECMA_INTEGER_NUMBER_MIN_SHIFTED -0x7fffff0

#if ECMA_DIRECT_SHIFT != 4
#error "Please update ECMA_INTEGER_NUMBER_MIN/MAX_SHIFTED according to the new value of ECMA_DIRECT_SHIFT."
#endif /* ECMA_DIRECT_SHIFT != 4 */

/**
 * Checks whether the integer number is in the integer number range.
 */
#define ECMA_IS_INTEGER_NUMBER(num) (ECMA_INTEGER_NUMBER_MIN <= (num) && (num) <= ECMA_INTEGER_NUMBER_MAX)

/**
 * Maximum integer number, which if squared, still fits in ecma_integer_value_t
 */
#define ECMA_INTEGER_MULTIPLY_MAX 0xb50

/**
 * Checks whether the error flag is set.
 */
#define ECMA_IS_VALUE_ERROR(value) (JERRY_UNLIKELY ((value) == ECMA_VALUE_ERROR))

/**
 * Callback which tells whether the ECMAScript execution should be stopped.
 */
typedef ecma_value_t (*ecma_vm_exec_stop_callback_t) (void *user_p);

/**
 * Forward definition of jerry_call_info_t.
 */
struct jerry_call_info_t;

/**
 * Type of an external function handler.
 */
typedef ecma_value_t (*ecma_native_handler_t) (const struct jerry_call_info_t *call_info_p,
                                               const ecma_value_t args_p[],
                                               const uint32_t args_count);

/**
 * Representation of native pointer data.
 */
typedef struct
{
  void *native_p; /**< points to the data of the object */
  jerry_object_native_info_t *native_info_p; /**< native info */
} ecma_native_pointer_t;

/**
 * Representation of native pointer data chain.
 */
typedef struct ecma_native_pointer_chain_t
{
  ecma_native_pointer_t data; /**< pointer data */
  struct ecma_native_pointer_chain_t *next_p; /**< next in the list */
} ecma_native_pointer_chain_t;

/**
 * Representation for class constructor environment record.
 */
typedef struct
{
  ecma_value_t this_binding; /**< this binding */
  ecma_value_t function_object; /**< function object */
} ecma_environment_record_t;

/**
 * Property list:
 *   The property list of an object is a chain list of various items.
 *   The type of each item is stored in the first byte of the item.
 *
 *   The most common item is the property pair, which contains two
 *   ecmascript properties. It is also important, that after the
 *   first property pair, only property pair items are allowed.
 *
 *   Example for other items is property name hash map, or array of items.
 */

/**
 * Property name listing options.
 */
typedef enum
{
  ECMA_LIST_NO_OPTS = (0), /**< no options are provided */
  ECMA_LIST_ARRAY_INDICES = (1 << 0), /**< exclude properties with names
                                       *   that are not indices */
  ECMA_LIST_ENUMERABLE = (1 << 1), /**< exclude non-enumerable properties */
  ECMA_LIST_PROTOTYPE = (1 << 2), /**< list properties from prototype chain */
  ECMA_LIST_SYMBOLS = (1 << 3), /**< list symbol properties */
  ECMA_LIST_SYMBOLS_ONLY = (1 << 4), /**< list symbol properties only */
  ECMA_LIST_CONVERT_FAST_ARRAYS = (1 << 5), /**< after listing the properties convert
                                             *   the fast access mode array back to normal array */
} ecma_list_properties_options_t;

/**
 * Enumerable property name listing options.
 */
typedef enum
{
  ECMA_ENUMERABLE_PROPERTY_KEYS, /**< List only property names */
  ECMA_ENUMERABLE_PROPERTY_VALUES, /**< List only property values */
  ECMA_ENUMERABLE_PROPERTY_ENTRIES, /**< List both property names and values */

  ECMA_ENUMERABLE_PROPERTY__COUNT /**< Number of enumerable property listing types */
} ecma_enumerable_property_names_options_t;

/**
 * List enumerable properties and include the prototype chain.
 */
#define ECMA_LIST_ENUMERABLE_PROTOTYPE (ECMA_LIST_ENUMERABLE | ECMA_LIST_PROTOTYPE)

/**
 * Property flag list. Several flags are alias
 */
typedef enum
{
  ECMA_PROPERTY_FLAG_CONFIGURABLE = (1u << 0), /**< property is configurable */
  ECMA_PROPERTY_FLAG_ENUMERABLE = (1u << 1), /**< property is enumerable */
  ECMA_PROPERTY_FLAG_WRITABLE = (1u << 2), /**< property is writable */
  ECMA_PROPERTY_FLAG_SINGLE_EXTERNAL = (1u << 2), /**< only one external pointer is assigned to this object */
  ECMA_PROPERTY_FLAG_DELETED = (1u << 3), /**< property is deleted */
  ECMA_PROPERTY_FLAG_BUILT_IN = (1u << 3), /**< property is defined by the ECMAScript standard */
  ECMA_FAST_ARRAY_FLAG = (1u << 3), /**< array is fast array */
  ECMA_PROPERTY_FLAG_LCACHED = (1u << 4), /**< property is lcached */
  ECMA_ARRAY_TEMPLATE_LITERAL = (1u << 4), /**< array is a template literal constructed by the parser */
  ECMA_PROPERTY_FLAG_DATA = (1u << 5), /**< property contains data */
  /* The last two bits contains an ECMA_DIRECT_STRING value. */
} ecma_property_flags_t;

/**
 * Property flags configurable, enumerable, writable.
 */
#define ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE \
  (ECMA_PROPERTY_FLAG_CONFIGURABLE | ECMA_PROPERTY_FLAG_ENUMERABLE | ECMA_PROPERTY_FLAG_WRITABLE)

/**
 * Property flags configurable, enumerable.
 */
#define ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE (ECMA_PROPERTY_FLAG_CONFIGURABLE | ECMA_PROPERTY_FLAG_ENUMERABLE)

/**
 * Property flags configurable, enumerable.
 */
#define ECMA_PROPERTY_CONFIGURABLE_WRITABLE (ECMA_PROPERTY_FLAG_CONFIGURABLE | ECMA_PROPERTY_FLAG_WRITABLE)

/**
 * Property flag built-in.
 */
#define ECMA_PROPERTY_BUILT_IN_FIXED (ECMA_PROPERTY_FLAG_BUILT_IN)

/**
 * Property flags enumerable, writable.
 */
#define ECMA_PROPERTY_ENUMERABLE_WRITABLE (ECMA_PROPERTY_FLAG_ENUMERABLE | ECMA_PROPERTY_FLAG_WRITABLE)

/**
 * Property flags built-in, configurable.
 */
#define ECMA_PROPERTY_BUILT_IN_CONFIGURABLE (ECMA_PROPERTY_FLAG_BUILT_IN | ECMA_PROPERTY_FLAG_CONFIGURABLE)

/**
 * Property flags built-in, configurable, enumerable.
 */
#define ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_ENUMERABLE \
  (ECMA_PROPERTY_FLAG_BUILT_IN | ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE)

/**
 * Property flags built-in, configurable, writable.
 */
#define ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_WRITABLE (ECMA_PROPERTY_FLAG_BUILT_IN | ECMA_PROPERTY_CONFIGURABLE_WRITABLE)

/**
 * Property flags built-in, writable.
 */
#define ECMA_PROPERTY_BUILT_IN_WRITABLE (ECMA_PROPERTY_FLAG_BUILT_IN | ECMA_PROPERTY_FLAG_WRITABLE)

/**
 * Property flags built-in, configurable, enumerable, writable.
 */
#define ECMA_PROPERTY_BUILT_IN_CONFIGURABLE_ENUMERABLE_WRITABLE \
  (ECMA_PROPERTY_FLAG_BUILT_IN | ECMA_PROPERTY_CONFIGURABLE_ENUMERABLE_WRITABLE)

/**
 * No attributes can be changed for this property.
 */
#define ECMA_PROPERTY_FIXED 0

/**
 * Default flag of length property.
 */
#define ECMA_PROPERTY_FLAG_DEFAULT_LENGTH ECMA_PROPERTY_FLAG_CONFIGURABLE

/**
 * Shift for property name part.
 */
#define ECMA_PROPERTY_NAME_TYPE_SHIFT 6

/**
 * Type of hash-map property.
 */
#define ECMA_PROPERTY_TYPE_HASHMAP (ECMA_DIRECT_STRING_SPECIAL << ECMA_PROPERTY_NAME_TYPE_SHIFT)

/**
 * Type of deleted property.
 */
#define ECMA_PROPERTY_TYPE_DELETED \
  (ECMA_PROPERTY_FLAG_DELETED | (ECMA_DIRECT_STRING_SPECIAL << ECMA_PROPERTY_NAME_TYPE_SHIFT))

/**
 * Type of property not found.
 */
#define ECMA_PROPERTY_TYPE_NOT_FOUND ECMA_PROPERTY_TYPE_HASHMAP

/**
 * Type of property not found and no more searching in the proto chain.
 */
#define ECMA_PROPERTY_TYPE_NOT_FOUND_AND_STOP ECMA_PROPERTY_TYPE_DELETED

/**
 * Type of property not found and an exception is thrown.
 */
#define ECMA_PROPERTY_TYPE_NOT_FOUND_AND_THROW \
  (ECMA_PROPERTY_FLAG_LCACHED | (ECMA_DIRECT_STRING_SPECIAL << ECMA_PROPERTY_NAME_TYPE_SHIFT))

/**
 * Checks whether a property is not found.
 */
#define ECMA_PROPERTY_IS_FOUND(property)                                                                    \
  (((property) & (ECMA_PROPERTY_FLAG_DATA | (ECMA_DIRECT_STRING_SPECIAL << ECMA_PROPERTY_NAME_TYPE_SHIFT))) \
   != (ECMA_DIRECT_STRING_SPECIAL << ECMA_PROPERTY_NAME_TYPE_SHIFT))

/**
 * Abstract property representation.
 *
 * A property is a type_and_flags byte and an ecma_value_t value pair.
 * This pair is represented by a single pointer in JerryScript. Although
 * a packed struct would only consume sizeof(ecma_value_t)+1 memory
 * bytes, accessing such structure is inefficient from the CPU viewpoint
 * because the value is not naturally aligned. To improve performance,
 * two type bytes and values are packed together. The memory layout is
 * the following:
 *
 *  [type 1, type 2, unused byte 1, unused byte 2][value 1][value 2]
 *
 * The unused two bytes are used to store a compressed pointer for the
 * next property pair.
 *
 * The advantage of this layout is that the value reference can be computed
 * from the property address. However, property pointers cannot be compressed
 * anymore.
 */
typedef uint8_t ecma_property_t; /**< ecma_property_types_t (3 bit) and ecma_property_flags_t */

/**
 * Number of items in a property pair.
 */
#define ECMA_PROPERTY_PAIR_ITEM_COUNT 2

/**
 * Property header for all items in a property list.
 */
typedef struct
{
  ecma_property_t types[ECMA_PROPERTY_PAIR_ITEM_COUNT]; /**< two property type slot. The first represent
                                                         *   the type of this property (e.g. property pair) */
  jmem_cpointer_t next_property_cp; /**< next cpointer */
} ecma_property_header_t;

/**
 * Pair of pointers - to property's getter and setter
 */
typedef struct
{
  jmem_cpointer_t getter_cp; /**< compressed pointer to getter object */
  jmem_cpointer_t setter_cp; /**< compressed pointer to setter object */
} ecma_getter_setter_pointers_t;

/**
 * Property data.
 */
typedef union
{
  ecma_value_t value; /**< value of a property */
  ecma_getter_setter_pointers_t getter_setter_pair; /**< getter setter pair */
} ecma_property_value_t;

/**
 * Property pair.
 */
typedef struct
{
  ecma_property_header_t header; /**< header of the property */
  ecma_property_value_t values[ECMA_PROPERTY_PAIR_ITEM_COUNT]; /**< property value slots */
  jmem_cpointer_t names_cp[ECMA_PROPERTY_PAIR_ITEM_COUNT]; /**< property name slots */
} ecma_property_pair_t;

/**
 * Get property name type.
 */
#define ECMA_PROPERTY_GET_NAME_TYPE(property) ((property) >> ECMA_PROPERTY_NAME_TYPE_SHIFT)

/**
 * Returns true if the property pointer is a property pair.
 */
#define ECMA_PROPERTY_IS_PROPERTY_PAIR(property_header_p) ((property_header_p)->types[0] != ECMA_PROPERTY_TYPE_HASHMAP)

/**
 * Property value of all internal properties
 */
#define ECMA_PROPERTY_INTERNAL (ECMA_PROPERTY_FLAG_DATA | (ECMA_DIRECT_STRING_SPECIAL << ECMA_PROPERTY_NAME_TYPE_SHIFT))

/**
 * Checks whether a property is internal property
 */
#define ECMA_PROPERTY_IS_INTERNAL(property) ((property) >= ECMA_PROPERTY_INTERNAL)

/**
 * Checks whether a property is raw data or accessor property
 */
#define ECMA_PROPERTY_IS_RAW(property) ((property) < (ECMA_DIRECT_STRING_SPECIAL << ECMA_PROPERTY_NAME_TYPE_SHIFT))

/**
 * Checks whether a property is raw data property (should only be used in assertions)
 */
#define ECMA_PROPERTY_IS_RAW_DATA(property) \
  (((property) &ECMA_PROPERTY_FLAG_DATA) && (property) < ECMA_PROPERTY_INTERNAL)

/**
 * Create internal property.
 */
#define ECMA_CREATE_INTERNAL_PROPERTY(object_p, name_p, property_p, property_value_p)              \
  do                                                                                               \
  {                                                                                                \
    (property_value_p) = ecma_create_named_data_property ((object_p), (name_p), 0, &(property_p)); \
    JERRY_ASSERT (*(property_p) == ECMA_PROPERTY_INTERNAL);                                        \
  } while (0)

/**
 * Property type of all virtual properties
 */
#define ECMA_PROPERTY_VIRTUAL ECMA_PROPERTY_INTERNAL

/**
 * Checks whether a property is virtual property
 */
#define ECMA_PROPERTY_IS_VIRTUAL(property) ECMA_PROPERTY_IS_INTERNAL (property)

/**
 * Returns true if the property is named property.
 */
#define ECMA_PROPERTY_IS_NAMED_PROPERTY(property) \
  ((property) < ECMA_PROPERTY_TYPE_HASHMAP || (property) >= ECMA_PROPERTY_INTERNAL)

/**
 * Add the offset part to a property for computing its property data pointer.
 */
#define ECMA_PROPERTY_VALUE_ADD_OFFSET(property_p) \
  ((uintptr_t) ((((uint8_t *) (property_p)) + (sizeof (ecma_property_value_t) * 2 - 1))))

/**
 * Align the property for computing its property data pointer.
 */
#define ECMA_PROPERTY_VALUE_DATA_PTR(property_p) \
  (ECMA_PROPERTY_VALUE_ADD_OFFSET (property_p) & ~(sizeof (ecma_property_value_t) - 1))

/**
 * Compute the property data pointer of a property.
 * The property must be part of a property pair.
 */
#define ECMA_PROPERTY_VALUE_PTR(property_p) ((ecma_property_value_t *) ECMA_PROPERTY_VALUE_DATA_PTR (property_p))

/**
 * Property reference. It contains the value pointer
 * for real, and the value itself for virtual properties.
 */
typedef union
{
  ecma_property_value_t *value_p; /**< property value pointer for real properties */
  ecma_value_t virtual_value; /**< property value for virtual properties */
} ecma_property_ref_t;

/**
 * Extended property reference, which also contains the
 * property descriptor pointer for real properties.
 */
typedef struct
{
  ecma_property_ref_t property_ref; /**< property reference */
  ecma_property_t *property_p; /**< property descriptor pointer for real properties */
} ecma_extended_property_ref_t;

/**
 * Option flags for ecma_op_object_get_property.
 */
typedef enum
{
  ECMA_PROPERTY_GET_NO_OPTIONS = 0, /**< no option flags for ecma_op_object_get_property */
  ECMA_PROPERTY_GET_VALUE = 1u << 0, /**< fill virtual_value field for virtual properties */
  ECMA_PROPERTY_GET_EXT_REFERENCE = 1u << 1, /**< get extended reference to the property */
} ecma_property_get_option_bits_t;

/**
 * Internal object types.
 */
typedef enum
{
  ECMA_OBJECT_TYPE_GENERAL = 0, /**< all objects that are not belongs to the sub-types below */
  ECMA_OBJECT_TYPE_BUILT_IN_GENERAL = 1, /**< built-in general object */
  ECMA_OBJECT_TYPE_CLASS = 2, /**< Objects with class property */
  ECMA_OBJECT_TYPE_BUILT_IN_CLASS = 3, /**< built-in object with class property */
  ECMA_OBJECT_TYPE_ARRAY = 4, /**< Array object (15.4) */
  ECMA_OBJECT_TYPE_BUILT_IN_ARRAY = 5, /**< Built-in array object */
  ECMA_OBJECT_TYPE_PROXY = 6, /**< Proxy object ECMAScript v6 26.2 */
  /* Note: these 4 types must be in this order. See IsCallable operation.  */
  ECMA_OBJECT_TYPE_FUNCTION = 7, /**< Function objects (15.3), created through 13.2 routine */
  ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION = 8, /**< Native built-in function object */
  ECMA_OBJECT_TYPE_BOUND_FUNCTION = 9, /**< Function objects (15.3), created through 15.3.4.5 routine */
  ECMA_OBJECT_TYPE_CONSTRUCTOR_FUNCTION = 10, /**< implicit class constructor function */
  ECMA_OBJECT_TYPE_NATIVE_FUNCTION = 11, /**< Native function object */

  ECMA_OBJECT_TYPE__MAX /**< maximum value */
} ecma_object_type_t;

/**
 * Base object types without built-in flag.
 *
 * Note:
 *     only these types can be checked with ecma_get_object_base_type.
 */
typedef enum
{
  ECMA_OBJECT_BASE_TYPE_GENERAL = ECMA_OBJECT_TYPE_GENERAL, /**< generic objects */
  ECMA_OBJECT_BASE_TYPE_CLASS = ECMA_OBJECT_TYPE_CLASS, /**< Objects with class property */
  ECMA_OBJECT_BASE_TYPE_ARRAY = ECMA_OBJECT_TYPE_ARRAY, /**< Array object (15.4) */
} ecma_object_base_type_t;

/**
 * Types of objects with class property.
 *
 * Note:
 *     when this type is changed, both ecma_class_object_magic_string_id
 *     and jerry_class_object_type must be updated as well
 */
typedef enum
{
  /* These objects require custom property resolving. */
  ECMA_OBJECT_CLASS_STRING, /**< String Object (ECMAScript v5.1, 4.3.18) */
  ECMA_OBJECT_CLASS_ARGUMENTS, /**< Arguments object (10.6) */
#if JERRY_BUILTIN_TYPEDARRAY
  ECMA_OBJECT_CLASS_TYPEDARRAY, /**< TypedArray which does NOT need extra space to store length and offset */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_MODULE_SYSTEM
  ECMA_OBJECT_CLASS_MODULE_NAMESPACE, /**< Module Namespace (ECMAScript v11, 9.4.6) */
#endif /* JERRY_MODULE_SYSTEM */

  /* These objects are marked by Garbage Collector. */
  ECMA_OBJECT_CLASS_GENERATOR, /**< Generator object (ECMAScript v6, 25.2) */
  ECMA_OBJECT_CLASS_ASYNC_GENERATOR, /**< Async generator object (ECMAScript v11, 25.3) */
  ECMA_OBJECT_CLASS_ARRAY_ITERATOR, /**< Array iterator object (ECMAScript v6, 22.1.5.1) */
  ECMA_OBJECT_CLASS_SET_ITERATOR, /**< Set iterator object (ECMAScript v6, 23.2.5.1) */
  ECMA_OBJECT_CLASS_MAP_ITERATOR, /**< Map iterator object (ECMAScript v6, 23.1.5.1) */
#if JERRY_BUILTIN_REGEXP
  ECMA_OBJECT_CLASS_REGEXP_STRING_ITERATOR, /** RegExp string iterator object (ECMAScript v11, 21.2.7) */
#endif /* JERRY_BUILTIN_REGEXP */
#if JERRY_MODULE_SYSTEM
  ECMA_OBJECT_CLASS_MODULE, /**< Module (ECMAScript v6, 15.2) */
#endif /* JERRY_MODULE_SYSTEM */
  ECMA_OBJECT_CLASS_PROMISE, /**< Promise (ECMAScript v6, 25.4) */
  ECMA_OBJECT_CLASS_PROMISE_CAPABILITY, /**< Promise capability (ECMAScript v6, 25.4.1.1) */
  ECMA_OBJECT_CLASS_ASYNC_FROM_SYNC_ITERATOR, /**< AsyncFromSyncIterator (ECMAScript v11, 25.1.4) */
#if JERRY_BUILTIN_DATAVIEW
  ECMA_OBJECT_CLASS_DATAVIEW, /**< DataView (ECMAScript v6, 24.2) */
#endif /* JERRY_BUILTIN_DATAVIEW */
#if JERRY_BUILTIN_CONTAINER
  ECMA_OBJECT_CLASS_CONTAINER, /**< Container (Map, WeakMap, Set, WeakSet) */
#endif /* JERRY_BUILTIN_CONTAINER */

  /* Normal objects. */
  ECMA_OBJECT_CLASS_BOOLEAN, /**< Boolean Object (ECMAScript v5.1, 4.3.15) */
  ECMA_OBJECT_CLASS_NUMBER, /**< Number Object (ECMAScript v5.1, 4.3.21) */
  ECMA_OBJECT_CLASS_ERROR, /**< Error Object (ECMAScript v5.1, 15.11) */
  ECMA_OBJECT_CLASS_INTERNAL_OBJECT, /**< object for internal properties */
#if JERRY_PARSER
  ECMA_OBJECT_CLASS_SCRIPT, /**< Compiled ECMAScript byte code */
#endif /* JERRY_PARSER */
#if JERRY_BUILTIN_DATE
  ECMA_OBJECT_CLASS_DATE, /**< Date Object (ECMAScript v5.1, 15.9) */
#endif /* JERRY_BUILTIN_DATE */
#if JERRY_BUILTIN_REGEXP
  ECMA_OBJECT_CLASS_REGEXP, /**< RegExp Object (ECMAScript v5.1, 15.10) */
#endif /* JERRY_BUILTIN_REGEXP */
  ECMA_OBJECT_CLASS_SYMBOL, /**< Symbol object (ECMAScript v6, 4.3.27) */
  ECMA_OBJECT_CLASS_STRING_ITERATOR, /**< String iterator object (ECMAScript v6, 22.1.5.1) */
#if JERRY_BUILTIN_TYPEDARRAY
  ECMA_OBJECT_CLASS_ARRAY_BUFFER, /**< Array Buffer (ECMAScript v6, 24.1) */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
#if JERRY_BUILTIN_SHAREDARRAYBUFFER
  ECMA_OBJECT_CLASS_SHARED_ARRAY_BUFFER, /**< Shared Array Buffer (ECMAScript v11 24.2) */
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */
#if JERRY_BUILTIN_BIGINT
  ECMA_OBJECT_CLASS_BIGINT, /**< Bigint (ECMAScript v11, 4.3.27) */
#endif /* JERRY_BUILTIN_BIGINT */
#if JERRY_BUILTIN_WEAKREF
  ECMA_OBJECT_CLASS_WEAKREF, /**< WeakRef (Not standardized yet) */
#endif /* JERRY_BUILTIN_WEAKREF */

  ECMA_OBJECT_CLASS__MAX /**< maximum value */
} ecma_object_class_type_t;

/**
 * Types of lexical environments.
 */
typedef enum
{
  /* Types between 0 - 12 are ecma_object_type_t. */
  ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE = 13, /**< declarative lexical environment */
  ECMA_LEXICAL_ENVIRONMENT_CLASS = 14, /**< lexical environment with class */
  ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND = 15, /**< object-bound lexical environment */

  ECMA_LEXICAL_ENVIRONMENT_TYPE_START = ECMA_LEXICAL_ENVIRONMENT_DECLARATIVE, /**< first lexical
                                                                               *   environment type */
  ECMA_LEXICAL_ENVIRONMENT_TYPE__MAX = ECMA_LEXICAL_ENVIRONMENT_THIS_OBJECT_BOUND /**< maximum value */
} ecma_lexical_environment_type_t;

/**
 * Types of array iterators.
 */
typedef enum
{
  ECMA_ITERATOR_KEYS, /**< keys iterator */
  ECMA_ITERATOR_VALUES, /**< values iterator */
  ECMA_ITERATOR_ENTRIES, /**< entries iterator */
  ECMA_ITERATOR__COUNT, /**< number of iterator kinds */
} ecma_iterator_kind_t;

/**
 * Offset for JERRY_CONTEXT (status_flags) top 8 bits.
 */
#define ECMA_LOCAL_PARSE_OPTS_OFFSET ((sizeof (uint32_t) - sizeof (uint8_t)) * JERRY_BITSINBYTE)

/**
 * Set JERRY_CONTEXT (status_flags) top 8 bits to the specified 'opts'.
 */
#define ECMA_SET_LOCAL_PARSE_OPTS(opts)                                                                          \
  do                                                                                                             \
  {                                                                                                              \
    JERRY_CONTEXT (status_flags) |= ((uint32_t) opts << ECMA_LOCAL_PARSE_OPTS_OFFSET) | ECMA_STATUS_DIRECT_EVAL; \
  } while (0)

/**
 * Get JERRY_CONTEXT (status_flags) top 8 bits.
 */
#define ECMA_GET_LOCAL_PARSE_OPTS() \
  (JERRY_CONTEXT (status_flags) >> (ECMA_LOCAL_PARSE_OPTS_OFFSET - JERRY_LOG2 (ECMA_PARSE_ALLOW_SUPER)))

/**
 * Clear JERRY_CONTEXT (status_flags) top 8 bits.
 */
#define ECMA_CLEAR_LOCAL_PARSE_OPTS()                                          \
  do                                                                           \
  {                                                                            \
    JERRY_CONTEXT (status_flags) &= ((1 << ECMA_LOCAL_PARSE_OPTS_OFFSET) - 1); \
  } while (0)

/**
 * Ecma object type mask for getting the object type.
 */
#define ECMA_OBJECT_TYPE_MASK 0x00fu

/**
 * Extensible object.
 */
#define ECMA_OBJECT_FLAG_EXTENSIBLE 0x10

/**
 * Declarative lexical environments created for non-closure code blocks
 */
#define ECMA_OBJECT_FLAG_BLOCK ECMA_OBJECT_FLAG_EXTENSIBLE

/**
 * Lexical environments with class has extra data
 */
#define ECMA_OBJECT_FLAG_LEXICAL_ENV_HAS_DATA ECMA_OBJECT_FLAG_EXTENSIBLE

/**
 * Bitshift index for an ecma-object reference count field
 */
#define ECMA_OBJECT_REF_SHIFT 5

/**
 * Value for increasing or decreasing the object reference counter.
 */
#define ECMA_OBJECT_REF_ONE (1u << ECMA_OBJECT_REF_SHIFT)

/**
 * Type of the descriptor field of an object
 */
typedef uint16_t ecma_object_descriptor_t;

/**
 * Bitmask for an ecma-object reference count field
 */
#define ECMA_OBJECT_REF_MASK ((ecma_object_descriptor_t) (~0u << ECMA_OBJECT_REF_SHIFT))

/**
 * Represents non-visited white object
 */
#define ECMA_OBJECT_NON_VISITED ECMA_OBJECT_REF_MASK

/**
 * Maximum value of the object reference counter (1022 / 67108862).
 */
#define ECMA_OBJECT_MAX_REF (ECMA_OBJECT_NON_VISITED - ECMA_OBJECT_REF_ONE)

/**
 * Description of ECMA-object or lexical environment
 * (depending on is_lexical_environment).
 */
typedef struct
{
  /** type : 4 bit : ecma_object_type_t or ecma_lexical_environment_type_t
                     depending on ECMA_OBJECT_FLAG_BUILT_IN_OR_LEXICAL_ENV
      flags : 2 bit : ECMA_OBJECT_FLAG_BUILT_IN_OR_LEXICAL_ENV,
                      ECMA_OBJECT_FLAG_EXTENSIBLE or ECMA_OBJECT_FLAG_BLOCK
      refs : 10 / 26 bit (max 1022 / 67108862) */
  ecma_object_descriptor_t type_flags_refs;

  /** next in the object chain maintained by the garbage collector */
  jmem_cpointer_t gc_next_cp;

  /** compressed pointer to property list or bound object */
  union
  {
    jmem_cpointer_t property_list_cp; /**< compressed pointer to object's
                                       *   or declarative lexical environments's property list */
    jmem_cpointer_t bound_object_cp; /**< compressed pointer to lexical environments's the bound object */
    jmem_cpointer_t home_object_cp; /**< compressed pointer to lexical environments's the home object */
  } u1;

  /** object prototype or outer reference */
  union
  {
    jmem_cpointer_t prototype_cp; /**< compressed pointer to the object's prototype  */
    jmem_cpointer_t outer_reference_cp; /**< compressed pointer to the lexical environments's outer reference  */
  } u2;
} ecma_object_t;

/**
 * Description of built-in properties of an object.
 */
typedef struct
{
  uint8_t id; /**< built-in id */
  uint8_t routine_id; /**< routine id for built-in functions */
  /** built-in specific field */
  union
  {
    uint8_t length_and_bitset_size; /**< length and bit set size for generic built-ins */
    uint8_t routine_index; /**< property descriptor index for built-in routines */
  } u;
  /** extra built-in info */
  union
  {
    uint8_t instantiated_bitset[1]; /**< instantiated property bit set for generic built-ins */
    uint8_t routine_flags; /**< flags for built-in routines */
  } u2;

#if JERRY_BUILTIN_REALMS
  ecma_value_t realm_value; /**< realm value */
#else /* !JERRY_BUILTIN_REALMS */
  uint32_t continue_instantiated_bitset[1]; /**< bit set for instantiated properties */
#endif /* JERRY_BUILTIN_REALMS */
} ecma_built_in_props_t;

/**
 * Type of a built-in function handler.
 */
typedef ecma_value_t (*ecma_builtin_handler_t) (ecma_object_t *function_obj_p,
                                                const ecma_value_t args_p[],
                                                const uint32_t args_count);

#if JERRY_BUILTIN_REALMS

/**
 * Number of bits available in the instantiated bitset without allocation
 */
#define ECMA_BUILTIN_INSTANTIATED_BITSET_MIN_SIZE (8)

#else /* !JERRY_BUILTIN_REALMS */

/**
 * Number of bits available in the instantiated bitset without allocation
 */
#define ECMA_BUILTIN_INSTANTIATED_BITSET_MIN_SIZE (8 + 32)

#endif /* JERRY_BUILTIN_REALMS */

/**
 * Builtin routine function object status flags
 */
typedef enum
{
  ECMA_BUILTIN_ROUTINE_NO_OPTS = 0, /**< No options are provided */
  ECMA_BUILTIN_ROUTINE_LENGTH_INITIALIZED = (1u << 0), /**< 'length' property has been initialized */
  ECMA_BUILTIN_ROUTINE_NAME_INITIALIZED = (1u << 1), /**< 'name' property has been initialized */
  ECMA_BUILTIN_ROUTINE_GETTER = (1u << 2), /**< this routine is getter */
  ECMA_BUILTIN_ROUTINE_SETTER = (1u << 3), /**< this routine is setter */
} ecma_builtin_routine_flags_t;

/**
 * Start position of bit set size in length_and_bitset_size field.
 */
#define ECMA_BUILT_IN_BITSET_SHIFT 5

/**
 * Description of extended ECMA-object.
 *
 * The extended object is an object with extra fields.
 */
typedef struct
{
  ecma_object_t object; /**< object header */

  /**
   * Description of extra fields. These extra fields depend on the object type.
   */
  union
  {
    ecma_built_in_props_t built_in; /**< built-in object part */

    /**
     * Description of objects with class.
     *
     * Note:
     *     class is a reserved word in c++, so cls is used instead
     */
    struct
    {
      uint8_t type; /**< class type of the object */
      /**
       * Description of 8 bit extra fields. These extra fields depend on the type.
       */
      union
      {
        uint8_t arguments_flags; /**< arguments object flags */
        uint8_t error_type; /**< jerry_error_t type of native error objects */
#if JERRY_BUILTIN_DATE
        uint8_t date_flags; /**< flags for date objects */
#endif /* JERRY_BUILTIN_DATE */
#if JERRY_MODULE_SYSTEM
        uint8_t module_state; /**< Module state */
#endif /* JERRY_MODULE_SYSTEM */
        uint8_t iterator_kind; /**< type of iterator */
        uint8_t regexp_string_iterator_flags; /**< flags for RegExp string iterator */
        uint8_t promise_flags; /**< Promise object flags */
#if JERRY_BUILTIN_CONTAINER
        uint8_t container_flags; /**< container object flags */
#endif /* JERRY_BUILTIN_CONTAINER */
#if JERRY_BUILTIN_TYPEDARRAY
        uint8_t array_buffer_flags; /**< ArrayBuffer flags */
        uint8_t typedarray_type; /**< type of typed array */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
      } u1;
      /**
       * Description of 16 bit extra fields. These extra fields depend on the type.
       */
      union
      {
        uint16_t formal_params_number; /**< for arguments: formal parameters number */
#if JERRY_MODULE_SYSTEM
        uint16_t module_flags; /**< Module flags */
#endif /* JERRY_MODULE_SYSTEM */
        uint16_t iterator_index; /**< for %Iterator%: [[%Iterator%NextIndex]] property */
        uint16_t executable_obj_flags; /**< executable object flags */
#if JERRY_BUILTIN_CONTAINER
        uint16_t container_id; /**< magic string id of a container */
#endif /* JERRY_BUILTIN_CONTAINER */
#if JERRY_BUILTIN_TYPEDARRAY
        uint16_t typedarray_flags; /**< typed array object flags */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
      } u2;
      /**
       * Description of 32 bit / value. These extra fields depend on the type.
       */
      union
      {
        ecma_value_t value; /**< value of the object (e.g. boolean, number, string, etc.) */
        ecma_value_t target; /**< [[ProxyTarget]] or [[WeakRefTarget]] internal property */
#if JERRY_BUILTIN_TYPEDARRAY
        ecma_value_t arraybuffer; /**< for typedarray: ArrayBuffer reference */
#endif /* JERRY_BUILTIN_TYPEDARRAY */
        ecma_value_t head; /**< points to the async generator task queue head item */
        ecma_value_t iterated_value; /**< for %Iterator%: [[IteratedObject]] property */
        ecma_value_t promise; /**< PromiseCapability[[Promise]] internal slot */
        ecma_value_t sync_iterator; /**< IteratorRecord [[Iterator]] internal slot for AsyncFromSyncIterator */
        ecma_value_t spread_value; /**< for spread object: spread element */
        int32_t tza; /**< TimeZone adjustment for date objects */
        uint32_t length; /**< length related property (e.g. length of ArrayBuffer) */
        uint32_t arguments_number; /**< for arguments: arguments number */
#if JERRY_MODULE_SYSTEM
        uint32_t dfs_ancestor_index; /**< module dfs ancestor index (ES2020 15.2.1.16) */
#endif /* JERRY_MODULE_SYSTEM */
      } u3;
    } cls;

    /**
     * Description of function objects.
     */
    struct
    {
      jmem_cpointer_tag_t scope_cp; /**< function scope */
      ecma_value_t bytecode_cp; /**< function byte code */
    } function;

    /**
     * Description of array objects.
     */
    struct
    {
      uint32_t length; /**< length property value */
      uint32_t length_prop_and_hole_count; /**< length property attributes and number of array holes in
                                            *   a fast access mode array multiplied ECMA_FAST_ACCESS_HOLE_ONE */
    } array;

    /**
     * Description of bound function object.
     */
    struct
    {
      jmem_cpointer_tag_t target_function; /**< target function */
      ecma_value_t args_len_or_this; /**< length of arguments or this value */
    } bound_function;

    /**
     * Description of implicit class constructor function.
     */
    struct
    {
      ecma_value_t script_value; /**< script value */
      uint8_t flags; /**< constructor flags */
    } constructor_function;
  } u;
} ecma_extended_object_t;

/**
 * Description of built-in extended ECMA-object.
 */
typedef struct
{
  ecma_extended_object_t extended_object; /**< extended object part */
  ecma_built_in_props_t built_in; /**< built-in object part */
} ecma_extended_built_in_object_t;

/**
 * Type of lexical environment with class
 */
typedef enum
{
  ECMA_LEX_ENV_CLASS_TYPE_MODULE, /**< module object reference */
  ECMA_LEX_ENV_CLASS_TYPE_CLASS_ENV, /**< class constructor object reference */
} ecma_lexical_environment_class_type_t;

/**
 * Description of lexical environment with class
 */
typedef struct
{
  ecma_object_t lexical_env; /**< lexical environment header */
  uint32_t type; /**< element of ecma_lexical_environment_class_type_t */
  ecma_object_t *object_p; /**< object reference */
} ecma_lexical_environment_class_t;

/**
 * Check whether the given lexical class environment is a module
 */
#define ECMA_LEX_ENV_CLASS_IS_MODULE(lex_env_p)                           \
  (((lex_env_p)->type_flags_refs & ECMA_OBJECT_FLAG_LEXICAL_ENV_HAS_DATA) \
   && ((ecma_lexical_environment_class_t *) (lex_env_p))->type == ECMA_LEX_ENV_CLASS_TYPE_MODULE)

/**
 * Description of native functions
 */
typedef struct
{
  ecma_extended_object_t extended_object; /**< extended object part */
#if JERRY_BUILTIN_REALMS
  ecma_value_t realm_value; /**< realm value */
#endif /* JERRY_BUILTIN_REALMS */
  ecma_native_handler_t native_handler_cb; /**< external function */
} ecma_native_function_t;

/**
 * Alignment for the fast access mode array length.
 * The real length is aligned up for allocating the underlying buffer.
 */
#define ECMA_FAST_ARRAY_ALIGNMENT (8)

/**
 * Align the length of the fast mode array to get the allocated size of the underlying buffer
 */
#define ECMA_FAST_ARRAY_ALIGN_LENGTH(length) \
  (uint32_t) ((((length)) + ECMA_FAST_ARRAY_ALIGNMENT - 1) / ECMA_FAST_ARRAY_ALIGNMENT * ECMA_FAST_ARRAY_ALIGNMENT)

/**
 * Compiled byte code data.
 */
typedef struct
{
  uint16_t size; /**< real size >> JMEM_ALIGNMENT_LOG */
  uint16_t refs; /**< reference counter for the byte code */
  uint16_t status_flags; /**< various status flags:
                          *   CBC_IS_FUNCTION check tells whether the byte code
                          *   is function or regular expression.
                          *   If function, the other flags must be CBC_CODE_FLAGS...
                          *   If regexp, the other flags must be RE_FLAG... */
} ecma_compiled_code_t;

/**
 * Description of bound function objects.
 */
typedef struct
{
  ecma_extended_object_t header; /**< extended object header */
  ecma_value_t target_length; /**< length of target function */
} ecma_bound_function_t;

#if JERRY_SNAPSHOT_EXEC

/**
 * Description of static function objects.
 */
typedef struct
{
  ecma_extended_object_t header; /**< header part */
  const ecma_compiled_code_t *bytecode_p; /**< real byte code pointer */
} ecma_static_function_t;

#endif /* JERRY_SNAPSHOT_EXEC */

/**
 * Description of arrow function objects.
 */
typedef struct
{
  ecma_extended_object_t header; /**< extended object header */
  ecma_value_t this_binding; /**< value of 'this' binding */
  ecma_value_t new_target; /**< value of new.target */
} ecma_arrow_function_t;

#if JERRY_SNAPSHOT_EXEC

/**
 * Description of static arrow function objects.
 */
typedef struct
{
  ecma_arrow_function_t header;
  const ecma_compiled_code_t *bytecode_p;
} ecma_static_arrow_function_t;

#endif /* JERRY_SNAPSHOT_EXEC */

#if JERRY_BUILTIN_CONTAINER
/**
 * Flags for container objects
 */
typedef enum
{
  ECMA_CONTAINER_FLAGS_EMPTY = (0), /** empty flags */
  ECMA_CONTAINER_FLAGS_WEAK = (1 << 0) /** container object is weak */
} ecma_container_flags_t;

/**
 * Description of map collection.
 */
typedef struct
{
  ecma_value_t key; /**< key value */
  ecma_value_t value; /**< value of the key */
} ecma_container_pair_t;

/**
 * Size of a single element (in ecma_value_t unit).
 */
#define ECMA_CONTAINER_VALUE_SIZE 1

/**
 * Size of a key - value pair (in ecma_value_t unit).
 */
#define ECMA_CONTAINER_PAIR_SIZE 2

/**
 * Size of the internal buffer.
 */
#define ECMA_CONTAINER_GET_SIZE(container_p) (container_p->buffer_p[0])

/**
 * Remove the size field of the internal buffer.
 */
#define ECMA_CONTAINER_SET_SIZE(container_p, size) (container_p->buffer_p[0] = (ecma_value_t) (size))

/**
 * Number of entries of the internal buffer.
 */
#define ECMA_CONTAINER_ENTRY_COUNT(collection_p) (collection_p->item_count - 1)

/**
 * Pointer to the first entry of the internal buffer.
 */
#define ECMA_CONTAINER_START(collection_p) (collection_p->buffer_p + 1)

#endif /* JERRY_BUILTIN_CONTAINER */

/**
 * Description of ECMA property descriptor
 *
 * See also: ECMA-262 v5, 8.10.
 *
 * Note:
 *      If a component of descriptor is undefined then corresponding
 *      field should contain it's default value.
 */
typedef struct
{
  uint16_t flags; /**< any combination of jerry_property_descriptor_flags_t bits */
  ecma_value_t value; /**< [[Value]] */
  ecma_object_t *get_p; /**< [[Get]] */
  ecma_object_t *set_p; /**< [[Set]] */
} ecma_property_descriptor_t;

/**
 * Bitfield which represents a namedata property options in an ecma_property_descriptor_t
 * Attributes:
 *  - is_get_defined, is_set_defined : false
 *  - is_configurable, is_writable, is_enumerable : undefined (false)
 *  - is_throw : undefined (false)
 *  - is_value_defined : true
 *  - is_configurable_defined, is_writable_defined, is_enumerable_defined : true
 */
#define ECMA_NAME_DATA_PROPERTY_DESCRIPTOR_BITS                                                                    \
  ((uint16_t) (JERRY_PROP_IS_VALUE_DEFINED | JERRY_PROP_IS_CONFIGURABLE_DEFINED | JERRY_PROP_IS_ENUMERABLE_DEFINED \
               | JERRY_PROP_IS_WRITABLE_DEFINED))

/**
 * Bitmask to get a the physical property flags from an ecma_property_descriptor
 */
#define ECMA_PROPERTY_FLAGS_MASK \
  ((uint16_t) (JERRY_PROP_IS_CONFIGURABLE | JERRY_PROP_IS_ENUMERABLE | JERRY_PROP_IS_WRITABLE))

/**
 * Description of an ecma-number
 */
typedef float ecma_number_t;

/**
 * Convert double to an ecma-number.
 */
#define DOUBLE_TO_ECMA_NUMBER_T(value) ((ecma_number_t) (value))

/**
 * Value '0' of ecma_number_t
 */
#define ECMA_NUMBER_ZERO ((ecma_number_t) 0.0f)

/**
 * Value '1' of ecma_number_t
 */
#define ECMA_NUMBER_ONE ((ecma_number_t) 1.0f)

/**
 * Value '2' of ecma_number_t
 */
#define ECMA_NUMBER_TWO ((ecma_number_t) 2.0f)

/**
 * Value '0.5' of ecma_number_t
 */
#define ECMA_NUMBER_HALF ((ecma_number_t) 0.5f)

/**
 * Value '-1' of ecma_number_t
 */
#define ECMA_NUMBER_MINUS_ONE ((ecma_number_t) -1.0f)

/**
 * Maximum number of characters in string representation of ecma-number
 */
#define ECMA_MAX_CHARS_IN_STRINGIFIED_NUMBER 64

/**
 * Maximum number of characters in string representation of ecma-uint32
 */
#define ECMA_MAX_CHARS_IN_STRINGIFIED_UINT32 10

/**
 * String is not a valid array index.
 */
#define ECMA_STRING_NOT_ARRAY_INDEX UINT32_MAX

/**
 * Ecma-collection: a growable list of ecma-values.
 */
typedef struct
{
  uint32_t item_count; /**< number of items in the collection */
  uint32_t capacity; /**< number of items can be stored in the underlying buffer */
  ecma_value_t *buffer_p; /**< underlying data buffer */
} ecma_collection_t;

/**
 * Initial capacity of an ecma-collection
 */
#define ECMA_COLLECTION_INITIAL_CAPACITY 4

/**
 * Ecma-collection grow factor when the collection underlying buffer need to be reallocated
 */
#define ECMA_COLLECTION_GROW_FACTOR (ECMA_COLLECTION_INITIAL_CAPACITY * 2)

/**
 * Compute the total allocated size of the collection based on it's capacity
 */
#define ECMA_COLLECTION_ALLOCATED_SIZE(capacity) (uint32_t) (capacity * sizeof (ecma_value_t))

/**
 * Initial allocated size of an ecma-collection
 */
#define ECMA_COLLECTION_INITIAL_SIZE ECMA_COLLECTION_ALLOCATED_SIZE (ECMA_COLLECTION_INITIAL_CAPACITY)

/**
 * Size shift of a compact collection
 */
#define ECMA_COMPACT_COLLECTION_SIZE_SHIFT 3

/**
 * Get the size of the compact collection
 */
#define ECMA_COMPACT_COLLECTION_GET_SIZE(compact_collection_p) \
  ((compact_collection_p)[0] >> ECMA_COMPACT_COLLECTION_SIZE_SHIFT)

/**
 * Direct string types (2 bit).
 */
typedef enum
{
  ECMA_DIRECT_STRING_PTR = 0, /**< string is a string pointer, only used by property names */
  ECMA_DIRECT_STRING_MAGIC = 1, /**< string is a magic string */
  ECMA_DIRECT_STRING_UINT = 2, /**< string is an unsigned int */
  ECMA_DIRECT_STRING_SPECIAL = 3, /**< string is special */
} ecma_direct_string_type_t;

/**
 * Maximum value of the immediate part of a direct magic string.
 * Must be compatible with the immediate property name.
 */
#define ECMA_DIRECT_STRING_MAX_IMM 0x0000ffff

/**
 * Shift for direct string value part in ecma_value_t.
 */
#define ECMA_DIRECT_STRING_SHIFT (ECMA_VALUE_SHIFT + 2)

/**
 * Full mask for direct strings.
 */
#define ECMA_DIRECT_STRING_MASK ((uintptr_t) (ECMA_DIRECT_TYPE_MASK | (0x3u << ECMA_VALUE_SHIFT)))

/**
 * Create an ecma direct string.
 */
#define ECMA_CREATE_DIRECT_STRING(type, value) \
  ((uintptr_t) (ECMA_TYPE_DIRECT_STRING | ((type) << ECMA_VALUE_SHIFT) | (value) << ECMA_DIRECT_STRING_SHIFT))

/**
 * Create an ecma direct string from the given number.
 *
 * Note: the given number must be less or equal than ECMA_DIRECT_STRING_MAX_IMM
 */
#define ECMA_CREATE_DIRECT_UINT32_STRING(uint32_number) \
  ((ecma_string_t *) ECMA_CREATE_DIRECT_STRING (ECMA_DIRECT_STRING_UINT, (uintptr_t) uint32_number))

/**
 * Checks whether the string is direct.
 */
#define ECMA_IS_DIRECT_STRING(string_p) ((((uintptr_t) (string_p)) & 0x1) != 0)

/**
 * Checks whether the string is direct.
 */
#define ECMA_IS_DIRECT_STRING_WITH_TYPE(string_p, type) \
  ((((uintptr_t) (string_p)) & ECMA_DIRECT_STRING_MASK) == ECMA_CREATE_DIRECT_STRING (type, 0))

/**
 * Returns the type of a direct string.
 */
#define ECMA_GET_DIRECT_STRING_TYPE(string_p) ((((uintptr_t) (string_p)) >> ECMA_VALUE_SHIFT) & 0x3)

/**
 * Shift applied to type conversions.
 */
#define ECMA_STRING_TYPE_CONVERSION_SHIFT (ECMA_PROPERTY_NAME_TYPE_SHIFT - ECMA_VALUE_SHIFT)

/**
 * Converts direct string type to property name type.
 */
#define ECMA_DIRECT_STRING_TYPE_TO_PROP_NAME_TYPE(string_p) \
  ((((uintptr_t) (string_p)) & (0x3 << ECMA_VALUE_SHIFT)) << ECMA_STRING_TYPE_CONVERSION_SHIFT)

/**
 * Returns the value of a direct string.
 */
#define ECMA_GET_DIRECT_STRING_VALUE(string_p) (((uintptr_t) (string_p)) >> ECMA_DIRECT_STRING_SHIFT)

/**
 * Maximum number of bytes that a long-utf8-string is able to store
 */
#define ECMA_STRING_SIZE_LIMIT UINT32_MAX

typedef enum
{
  ECMA_STRING_CONTAINER_HEAP_UTF8_STRING, /**< actual data is on the heap as an utf-8 (cesu8) string
                                           *   maximum size is 2^16. */
  ECMA_STRING_CONTAINER_LONG_OR_EXTERNAL_STRING, /**< the string is a long string or provided externally
                                                  *   and only its attributes are stored. */
  ECMA_STRING_CONTAINER_UINT32_IN_DESC, /**< string representation of an uint32 number */
  ECMA_STRING_CONTAINER_HEAP_ASCII_STRING, /**< actual data is on the heap as an ASCII string
                                            *   maximum size is 2^16. */
  ECMA_STRING_CONTAINER_MAGIC_STRING_EX, /**< the ecma-string is equal to one of external magic strings */
  ECMA_STRING_CONTAINER_SYMBOL, /**< the ecma-string is a symbol */

  ECMA_STRING_CONTAINER__MAX = ECMA_STRING_CONTAINER_SYMBOL /**< maximum value */
} ecma_string_container_t;

/**
 * Mask for getting the container of a string.
 */
#define ECMA_STRING_CONTAINER_MASK 0x7u

/**
 * Value for increasing or decreasing the reference counter.
 */
#define ECMA_STRING_REF_ONE (1u << 4)

/**
 * Maximum value of the reference counter (4294967280).
 */
#define ECMA_STRING_MAX_REF (0xFFFFFFF0)

/**
 * Flag that identifies that the string is static which means it is stored in JERRY_CONTEXT (string_list_cp)
 */
#define ECMA_STATIC_STRING_FLAG (1 << 3)

/**
 * Set an ecma-string as static string
 */
#define ECMA_SET_STRING_AS_STATIC(string_p) (string_p)->refs_and_container |= ECMA_STATIC_STRING_FLAG

/**
 * Checks whether the ecma-string is static string
 */
#define ECMA_STRING_IS_STATIC(string_p) ((string_p)->refs_and_container & ECMA_STATIC_STRING_FLAG)

/**
 * Returns with the container type of a string.
 */
#define ECMA_STRING_GET_CONTAINER(string_desc_p) \
  ((ecma_string_container_t) ((string_desc_p)->refs_and_container & ECMA_STRING_CONTAINER_MASK))

/**
 * Checks whether the reference counter is 1 of a string.
 */
#define ECMA_STRING_IS_REF_EQUALS_TO_ONE(string_desc_p) (((string_desc_p)->refs_and_container >> 4) == 1)

/**
 * Checks whether the reference counter is 1 of an extended primitive.
 */
#define ECMA_EXTENDED_PRIMITIVE_IS_REF_EQUALS_TO_ONE(extended_primitive_p) \
  (((extended_primitive_p)->refs_and_type >> 3) == 1)

/**
 * ECMA string-value descriptor
 */
typedef struct
{
  /** Reference counter for the string */
  uint32_t refs_and_container;

  /**
   * Actual data or identifier of it's place in container (depending on 'container' field)
   */
  union
  {
    lit_string_hash_t hash; /**< hash of the ASCII/UTF8 string */
    uint32_t magic_string_ex_id; /**< identifier of an external magic string (lit_magic_string_ex_id_t) */
    uint32_t uint32_number; /**< uint32-represented number placed locally in the descriptor */
  } u;
} ecma_string_t;

/**
 * ECMA UTF8 string-value descriptor
 */
typedef struct
{
  ecma_string_t header; /**< string header */
  uint16_t size; /**< size of this utf-8 string in bytes */
  uint16_t length; /**< length of this utf-8 string in characters */
} ecma_short_string_t;

/**
 * Long or external CESU8 string-value descriptor
 */
typedef struct
{
  ecma_string_t header; /**< string header */
  const lit_utf8_byte_t *string_p; /**< string data */
  lit_utf8_size_t size; /**< size of this external string in bytes */
  lit_utf8_size_t length; /**< length of this external string in characters */
} ecma_long_string_t;

/**
 * External UTF8 string-value descriptor
 */
typedef struct
{
  ecma_long_string_t header; /**< long string header */
  void *user_p; /**< user pointer passed to the callback when the string is freed */
} ecma_external_string_t;

/**
 * Header size of an ecma ASCII string
 */
#define ECMA_ASCII_STRING_HEADER_SIZE ((lit_utf8_size_t) (sizeof (ecma_string_t) + sizeof (uint8_t)))

/**
 * Get the size of an ecma ASCII string
 */
#define ECMA_ASCII_STRING_GET_SIZE(string_p) \
  ((lit_utf8_size_t) * ((lit_utf8_byte_t *) (string_p) + sizeof (ecma_string_t)) + 1)

/**
 * Set the size of an ecma ASCII string
 */
#define ECMA_ASCII_STRING_SET_SIZE(string_p, size) \
  (*((lit_utf8_byte_t *) (string_p) + sizeof (ecma_string_t)) = (uint8_t) ((size) -1))

/**
 * Get the start position of the string buffer of an ecma ASCII string
 */
#define ECMA_ASCII_STRING_GET_BUFFER(string_p) ((lit_utf8_byte_t *) (string_p) + ECMA_ASCII_STRING_HEADER_SIZE)

/**
 * Get the start position of the string buffer of an ecma UTF8 string
 */
#define ECMA_SHORT_STRING_GET_BUFFER(string_p) ((lit_utf8_byte_t *) (string_p) + sizeof (ecma_short_string_t))

/**
 * Get the start position of the string buffer of an ecma long CESU8 string
 */
#define ECMA_LONG_STRING_BUFFER_START(string_p) ((lit_utf8_byte_t *) (string_p) + sizeof (ecma_long_string_t))

/**
 * ECMA extended string-value descriptor
 */
typedef struct
{
  ecma_string_t header; /**< string header */

  union
  {
    ecma_value_t symbol_descriptor; /**< symbol descriptor string-value */
    ecma_value_t value; /**< original key value corresponds to the map key string */
  } u;
} ecma_extended_string_t;

/**
 * Required number of ecma values in a compact collection to represent PrivateElement
 */
#define ECMA_PRIVATE_ELEMENT_LIST_SIZE 3

/**
 * String builder header
 */
typedef struct
{
  lit_utf8_size_t current_size; /**< size of the data in the buffer */
} ecma_stringbuilder_header_t;

/**
 * Get pointer to the beginning of the stored string in the string builder
 */
#define ECMA_STRINGBUILDER_STRING_PTR(header_p) \
  ((lit_utf8_byte_t *) (((lit_utf8_byte_t *) header_p) + ECMA_ASCII_STRING_HEADER_SIZE))

/**
 * Get the size of the stored string in the string builder
 */
#define ECMA_STRINGBUILDER_STRING_SIZE(header_p) \
  ((lit_utf8_size_t) (header_p->current_size - ECMA_ASCII_STRING_HEADER_SIZE))

/**
 * String builder handle
 */
typedef struct
{
  ecma_stringbuilder_header_t *header_p; /**< pointer to header */
} ecma_stringbuilder_t;

#ifndef JERRY_BUILTIN_BIGINT
/**
 * BigInt type.
 */
#define ECMA_EXTENDED_PRIMITIVE_BIGINT 0
#endif /* !defined (JERRY_BUILTIN_BIGINT) */

/**
 * Flags for exception values.
 */
typedef enum
{
  ECMA_ERROR_API_FLAG_NONE = 0,
  ECMA_ERROR_API_FLAG_ABORT = (1u << 0), /**< abort flag */
  ECMA_ERROR_API_FLAG_THROW_CAPTURED = (1u << 1), /**< throw captured flag */
} ecma_error_api_flags_t;

/**
 * Representation of a thrown value on API level.
 */
typedef struct
{
  uint32_t refs_and_type; /**< reference counter and type */
  union
  {
    ecma_value_t value; /**< referenced value */
    uint32_t bigint_sign_and_size; /**< BigInt properties */
  } u;
} ecma_extended_primitive_t;

/**
 * Value for increasing or decreasing the reference counter.
 */
#define ECMA_EXTENDED_PRIMITIVE_REF_ONE (1u << 3)

/**
 * Maximum value of the reference counter.
 */
#define ECMA_EXTENDED_PRIMITIVE_MAX_REF (UINT32_MAX - (ECMA_EXTENDED_PRIMITIVE_REF_ONE - 1))

#if JERRY_PROPERTY_HASHMAP

/**
 * The lowest state of the ecma_prop_hashmap_alloc_state counter.
 * If ecma_prop_hashmap_alloc_state other than this value, it is
 * disabled.
 */
#define ECMA_PROP_HASHMAP_ALLOC_ON 0

/**
 * The highest state of the ecma_prop_hashmap_alloc_state counter.
 */
#define ECMA_PROP_HASHMAP_ALLOC_MAX 4

#endif /* JERRY_PROPERTY_HASHMAP */

/**
 * Number of values in a literal storage item
 */
#define ECMA_LIT_STORAGE_VALUE_COUNT 3

/**
 * Literal storage item
 */
typedef struct
{
  jmem_cpointer_t next_cp; /**< cpointer ot next item */
  jmem_cpointer_t values[ECMA_LIT_STORAGE_VALUE_COUNT]; /**< list of values */
} ecma_lit_storage_item_t;

#if JERRY_LCACHE
/**
 * Container of an LCache entry identifier
 */
typedef uint32_t ecma_lcache_hash_entry_id_t;

/**
 * Entry of LCache hash table
 */
typedef struct
{
  /** Pointer to a property of the object */
  ecma_property_t *prop_p;

  /** Entry identifier in LCache */
  ecma_lcache_hash_entry_id_t id;
} ecma_lcache_hash_entry_t;

/**
 * Number of rows in LCache's hash table
 */
#define ECMA_LCACHE_HASH_ROWS_COUNT 128

/**
 * Number of entries in a row of LCache's hash table
 */
#define ECMA_LCACHE_HASH_ROW_LENGTH 2

#endif /* JERRY_LCACHE */

#if JERRY_BUILTIN_TYPEDARRAY

/**
 * Function callback descriptor of a %TypedArray% object getter
 */
typedef ecma_value_t (*ecma_typedarray_getter_fn_t) (lit_utf8_byte_t *src);

/**
 * Function callback descriptor of a %TypedArray% object setter
 */
typedef ecma_value_t (*ecma_typedarray_setter_fn_t) (lit_utf8_byte_t *src, ecma_value_t value);

/**
 * Builtin id for the different types of TypedArray's
 */
typedef enum
{
  ECMA_INT8_ARRAY, /**< Int8Array */
  ECMA_UINT8_ARRAY, /**< Uint8Array */
  ECMA_UINT8_CLAMPED_ARRAY, /**< Uint8ClampedArray */
  ECMA_INT16_ARRAY, /**< Int16Array */
  ECMA_UINT16_ARRAY, /**< Uint16Array */
  ECMA_INT32_ARRAY, /**< Int32Array */
  ECMA_UINT32_ARRAY, /**< Uint32Array */
  ECMA_FLOAT32_ARRAY, /**< Float32Array */
  ECMA_FLOAT64_ARRAY, /**< Float64Array */
  /* ECMA_TYPEDARRAY_IS_BIGINT_TYPE macro should be updated when new types are added */
  ECMA_BIGINT64_ARRAY, /**< BigInt64Array */
  ECMA_BIGUINT64_ARRAY, /**< BigUInt64Array */
} ecma_typedarray_type_t;

/**
 * TypedArray flags.
 */
typedef enum
{
  ECMA_TYPEDARRAY_IS_EXTENDED = (1u << 0), /* an ecma_extended_typedarray_object_t is allocated for the TypedArray */
} ecma_typedarray_flag_t;

/**
 * Array buffer flags.
 */
typedef enum
{
  ECMA_ARRAYBUFFER_HAS_POINTER = (1u << 0), /* ArrayBuffer has a buffer pointer. */
  ECMA_ARRAYBUFFER_ALLOCATED = (1u << 1), /* ArrayBuffer memory is allocated */
  ECMA_ARRAYBUFFER_DETACHED = (1u << 2), /* ArrayBuffer has been detached */
} ecma_arraybuffer_flag_t;

/**
 * Structure for array buffers with a backing store pointer.
 */
typedef struct
{
  ecma_extended_object_t extended_object; /**< extended object part */
  void *buffer_p; /**< pointer to the backing store of the array buffer object */
  void *arraybuffer_user_p; /**< user pointer passed to the free callback */
} ecma_arraybuffer_pointer_t;

/**
 * Some internal properties of TypedArray object.
 * It is only used when the offset is not 0, and
 * the array-length is not buffer-length / element_size.
 */
typedef struct
{
  ecma_extended_object_t extended_object; /**< extended object part */
  uint32_t byte_offset; /**< the byteoffset of the above arraybuffer */
  uint32_t array_length; /**< the array length */
} ecma_extended_typedarray_object_t;

/**
 * General structure for query %TypedArray% object's properties.
 **/
typedef struct
{
  ecma_object_t *array_buffer_p; /**< pointer to the typedArray's [[ViewedArrayBuffer]] internal slot */
  ecma_typedarray_type_t id; /**< [[TypedArrayName]] internal slot */
  uint32_t length; /**< [[ByteLength]] internal slot */
  uint32_t offset; /**< [[ByteOffset]] internal slot. */
  uint8_t shift; /**< the element size shift in the typedArray */
  uint8_t element_size; /**< element size based on [[TypedArrayName]] in Table 49 */
} ecma_typedarray_info_t;

#if JERRY_BUILTIN_BIGINT
/**
 * Checks whether a given typedarray is BigInt type or not.
 **/
#define ECMA_TYPEDARRAY_IS_BIGINT_TYPE(id) ((id) >= ECMA_BIGINT64_ARRAY)

#endif /* JERRY_BUILTIN_BIGINT */
#endif /* JERRY_BUILTIN_TYPEDARRAY */

/**
 * Executable (e.g. generator, async) object flags.
 */
typedef enum
{
  ECMA_EXECUTABLE_OBJECT_COMPLETED = (1u << 0), /**< executable object is completed and cannot be resumed */
  ECMA_EXECUTABLE_OBJECT_RUNNING = (1u << 1), /**< executable object is currently running */
  /* Generator specific flags. */
  ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD = (1u << 2), /**< the executable object performs
                                                         *   an await or a yield* operation */
  ECMA_ASYNC_GENERATOR_CALLED = (1u << 3), /**< the async generator was executed before */
  /* This must be the last generator specific flag. */
  ECMA_AWAIT_STATE_SHIFT = 4, /**< shift for await states */
} ecma_executable_object_flags_t;

/**
 * Async function states after an await is completed.
 */
typedef enum
{
  ECMA_AWAIT_YIELD_NEXT, /**< wait for an iterator result object */
  ECMA_AWAIT_YIELD_NEXT_RETURN, /**< wait for an iterator result object after a return operation */
  ECMA_AWAIT_YIELD_RETURN, /**< wait for the argument passed to return operation */
  ECMA_AWAIT_YIELD_NEXT_VALUE, /**< wait for the value property of an iterator result object */
  ECMA_AWAIT_YIELD_OPERATION, /**< wait for the generator operation (next/throw/return) */
  ECMA_AWAIT_YIELD_CLOSE, /**< wait for the result of iterator close operation */
  /* After adding new ECMA_AWAIT_YIELD items, the ECMA_AWAIT_YIELD_END should be updated. */
  ECMA_AWAIT_FOR_CLOSE, /**< wait for a close iterator result object of for-await-of statement */
  ECMA_AWAIT_FOR_NEXT, /**< wait for an iterator result object of for-await-of statement */
} ecma_await_states_t;

/**
 * Checks whether the executable object is waiting for resuming.
 */
#define ECMA_EXECUTABLE_OBJECT_IS_SUSPENDED(executable_object_p)          \
  (!((executable_object_p)->extended_object.u.cls.u2.executable_obj_flags \
     & (ECMA_EXECUTABLE_OBJECT_COMPLETED | ECMA_EXECUTABLE_OBJECT_RUNNING)))

/**
 * Last item of yield* related await states.
 */
#define ECMA_AWAIT_YIELD_END ECMA_AWAIT_YIELD_CLOSE

/**
 * Helper macro for ECMA_EXECUTABLE_OBJECT_RESUME_EXEC.
 */
#define ECMA_EXECUTABLE_OBJECT_RESUME_EXEC_MASK ((uint16_t) ~ECMA_EXECUTABLE_OBJECT_DO_AWAIT_OR_YIELD)

/**
 * Resume execution of the byte code.
 */
#define ECMA_EXECUTABLE_OBJECT_RESUME_EXEC(executable_object_p) \
  ((executable_object_p)->extended_object.u.cls.u2.executable_obj_flags &= ECMA_EXECUTABLE_OBJECT_RESUME_EXEC_MASK)

/**
 * Enqueued task of an AsyncGenerator.
 *
 * An execution of a task has three steps:
 *  1) Perform a next/throw/return operation
 *  2) Resume the execution of the AsyncGenerator
 *  3) Fulfill or reject a promise if the AsyncGenerator yielded a value
 *     (these Promises are created by the AsyncGenerator itself)
 */
typedef struct
{
  ecma_value_t next; /**< points to the next task which will be performed after this task is completed */
  ecma_value_t promise; /**< promise which will be fulfilled or rejected after this task is completed */
  ecma_value_t operation_value; /**< value argument of the operation */
  uint8_t operation_type; /**< type of operation (see ecma_async_generator_operation_type_t) */
} ecma_async_generator_task_t;

/**
 * Definition of PromiseCapability Records
 */
typedef struct
{
  ecma_extended_object_t header; /**< object header, and [[Promise]] internal slot */
  ecma_value_t resolve; /**< [[Resolve]] internal slot */
  ecma_value_t reject; /**< [[Reject]] internal slot */
} ecma_promise_capability_t;

/**
 * Definition of GetCapabilitiesExecutor Functions
 */
typedef struct
{
  ecma_extended_object_t header; /**< object header */
  ecma_value_t capability; /**< [[Capability]] internal slot */
} ecma_promise_capability_executor_t;

/**
 * Definition of Promise.all Resolve Element Functions
 */
typedef struct
{
  ecma_extended_object_t header; /**< object header */
  ecma_value_t remaining_elements; /**< [[Remaining elements]] internal slot */
  ecma_value_t capability; /**< [[Capabilities]] internal slot */
  ecma_value_t values; /**< [[Values]] or [[Errors]] internal slot */
  uint32_t index; /**< [[Index]] and [[AlreadyCalled]] internal slot
                   *   0 - if the element has been resolved
                   *   real index + 1 in the [[Values]] list - otherwise */
} ecma_promise_all_executor_t;

/**
 * Promise prototype methods helper.
 */
typedef enum
{
  ECMA_PROMISE_ALL_RESOLVE, /**< promise.all resolve */
  ECMA_PROMISE_ALLSETTLED_RESOLVE, /**< promise.allSettled resolve */
  ECMA_PROMISE_ALLSETTLED_REJECT, /**< promise.allSettled reject */
  ECMA_PROMISE_ANY_REJECT, /**< promise.any reject */
} ecma_promise_executor_type_t;

#if JERRY_BUILTIN_DATAVIEW
/**
 * Description of DataView objects.
 */
typedef struct
{
  ecma_extended_object_t header; /**< header part */
  ecma_object_t *buffer_p; /**< [[ViewedArrayBuffer]] internal slot */
  uint32_t byte_offset; /**< [[ByteOffset]] internal slot */
} ecma_dataview_object_t;
#endif /* JERRY_BUILTIN_DATAVIEW */

typedef enum
{
  ECMA_SYMBOL_FLAG_NONE = 0, /**< no options */
  ECMA_SYMBOL_FLAG_GLOBAL = (1 << 0), /**< symbol is a well known symbol, See also: 6.1.5.1 */
  ECMA_SYMBOL_FLAG_PRIVATE_KEY = (1 << 1), /**< symbol is a private field */
  ECMA_SYMBOL_FLAG_PRIVATE_INSTANCE_METHOD = (1 << 2), /**< symbol is a private method or accessor */
} ecma_symbol_flags_t;

/**
 * Bitshift index for the symbol hash property
 */
#define ECMA_SYMBOL_FLAGS_SHIFT 3

/**
 * Bitmask for symbol hash flags
 */
#define ECMA_SYMBOL_FLAGS_MASK ((1 << ECMA_SYMBOL_FLAGS_SHIFT) - 1)

#if (JERRY_STACK_LIMIT != 0)
/**
 * Check the current stack usage. If the limit is reached a RangeError is raised.
 * The macro argument specifies the return value which is usually ECMA_VALUE_ERROR or NULL.
 */
#define ECMA_CHECK_STACK_USAGE_RETURN(RETURN_VALUE)               \
  do                                                              \
  {                                                               \
    if (ecma_get_current_stack_usage () > CONFIG_MEM_STACK_LIMIT) \
    {                                                             \
      ecma_raise_maximum_callstack_error ();                      \
      return RETURN_VALUE;                                        \
    }                                                             \
  } while (0)

/**
 * Specialized version of ECMA_CHECK_STACK_USAGE_RETURN which returns ECMA_VALUE_ERROR.
 * This version should be used in most cases.
 */
#define ECMA_CHECK_STACK_USAGE() ECMA_CHECK_STACK_USAGE_RETURN (ECMA_VALUE_ERROR)
#else /* JERRY_STACK_LIMIT == 0) */
/**
 * If the stack limit is unlimited, this check is an empty macro.
 */
#define ECMA_CHECK_STACK_USAGE_RETURN(RETURN_VALUE)

/**
 * If the stack limit is unlimited, this check is an empty macro.
 */
#define ECMA_CHECK_STACK_USAGE()
#endif /* (JERRY_STACK_LIMIT != 0) */

/**
 * Invalid object pointer which represents abrupt completion
 */
#define ECMA_OBJECT_POINTER_ERROR ((ecma_object_t *) 0x01)

/**
 * Invalid property pointer which represents abrupt completion
 */
#define ECMA_PROPERTY_POINTER_ERROR ((ecma_property_t *) 0x01)

#if JERRY_BUILTIN_PROXY

/**
 * Proxy object flags.
 */
typedef enum
{
  ECMA_PROXY_SKIP_RESULT_VALIDATION = (1u << 0), /**< skip result validation for [[GetPrototypeOf]],
                                                  *   [[SetPrototypeOf]], [[IsExtensible]],
                                                  *   [[PreventExtensions]], [[GetOwnProperty]],
                                                  *   [[DefineOwnProperty]], [[HasProperty]], [[Get]],
                                                  *   [[Set]], [[Delete]] and [[OwnPropertyKeys]] */
  ECMA_PROXY_IS_CALLABLE = (1u << 1), /**< proxy is callable */
  ECMA_PROXY_IS_CONSTRUCTABLE = (1u << 2), /**< proxy is constructable */
} ecma_proxy_flag_types_t;

/**
 * Description of Proxy objects.
 *
 * A Proxy object's property list is used to store extra information:
 *  * The "header.u2.prototype_cp" 1st tag bit stores the IsCallable information.
 *  * The "header.u2.prototype_cp" 2nd tag bit stores the IsConstructor information.
 */
typedef struct
{
  ecma_object_t header; /**< header part */
  ecma_value_t target; /**< [[ProxyTarget]] internal slot */
  ecma_value_t handler; /**< [[ProxyHandler]] internal slot */
} ecma_proxy_object_t;

/**
 * Description of Proxy objects.
 */
typedef struct
{
  ecma_extended_object_t header; /**< header part */
  ecma_value_t proxy; /**< [[RevocableProxy]] internal slot */
} ecma_revocable_proxy_object_t;
#endif /* JERRY_BUILTIN_PROXY */

/**
 * Type to represent the maximum property index
 *
 * For ES6+ the maximum valid property index is 2**53 - 1
 */
typedef uint64_t ecma_length_t;

#if JERRY_BUILTIN_BIGINT

/**
 * BigUInt data is a sequence of uint32_t numbers.
 */
typedef uint32_t ecma_bigint_digit_t;

/**
 * Special BigInt value representing zero.
 */
#define ECMA_BIGINT_ZERO ((ecma_value_t) ECMA_TYPE_BIGINT)

/**
 * Special BigInt value representing zero when the result is pointer.
 */
#define ECMA_BIGINT_POINTER_TO_ZERO ((ecma_extended_primitive_t *) 0x1)

/**
 * Return the size of a BigInt value in ecma_bigint_data_t units.
 */
#define ECMA_BIGINT_GET_SIZE(value_p) \
  ((value_p)->u.bigint_sign_and_size & ~(uint32_t) (sizeof (ecma_bigint_digit_t) - 1))

/**
 * Size of memory needs to be allocated for the digits of a BigInt.
 * The value is rounded up for two digits.
 */
#define ECMA_BIGINT_GET_BYTE_SIZE(size) \
  (size_t) (((size) + sizeof (ecma_bigint_digit_t)) & ~(2 * sizeof (ecma_bigint_digit_t) - 1))

#endif /* JERRY_BUILTIN_BIGINT */

/**
 * Struct for counting the different types properties in objects
 */
typedef struct
{
  uint32_t array_index_named_props; /**< number of array index named properties */
  uint32_t string_named_props; /**< number of string named properties */
  uint32_t symbol_named_props; /**< number of symbol named properties */
} ecma_property_counter_t;

/**
 * Arguments object related status flags
 */
typedef enum
{
  ECMA_ARGUMENTS_OBJECT_NO_FLAGS = 0, /* unmapped arguments object */
  ECMA_ARGUMENTS_OBJECT_MAPPED = (1 << 0), /* mapped arguments object */
  ECMA_ARGUMENTS_OBJECT_STATIC_BYTECODE = (1 << 1), /* static mapped arguments object */
  ECMA_ARGUMENTS_OBJECT_CALLEE_INITIALIZED = (1 << 2), /* 'callee' property has been lazy initialized */
  ECMA_ARGUMENTS_OBJECT_LENGTH_INITIALIZED = (1 << 3), /* 'length' property has been lazy initialized */
  ECMA_ARGUMENTS_OBJECT_ITERATOR_INITIALIZED = (1 << 4), /* 'Symbol.iterator' property has been lazy initialized */
} ecma_arguments_object_flags_t;

/**
 * Definition of unmapped arguments object
 */
typedef struct
{
  ecma_extended_object_t header; /**< object header */
  ecma_value_t callee; /**< 'callee' property */
} ecma_unmapped_arguments_t;

/**
 * Definition of mapped arguments object
 */
typedef struct
{
  ecma_unmapped_arguments_t unmapped; /**< unmapped arguments object header */
  ecma_value_t lex_env; /**< environment reference */
  union
  {
    ecma_value_t byte_code; /**< callee's compiled code */
#if JERRY_SNAPSHOT_EXEC
    ecma_compiled_code_t *byte_code_p; /**< real byte code pointer */
#endif /* JERRY_SNAPSHOT_EXEC */
  } u;
} ecma_mapped_arguments_t;

/**
 * Date object descriptor flags
 */
typedef enum
{
  ECMA_DATE_TZA_NONE = 0, /**< no time-zone adjustment is set */
  ECMA_DATE_TZA_SET = (1 << 0), /**< time-zone adjustment is set */
} ecma_date_object_flags_t;

/**
 * Definition of date object
 */
typedef struct
{
  ecma_extended_object_t header; /**< object header */
  ecma_number_t date_value; /**< [[DateValue]] internal property */
} ecma_date_object_t;

/**
 * Implicit class constructor flags
 */
typedef enum
{
  ECMA_CONSTRUCTOR_FUNCTION_HAS_HERITAGE = (1 << 0), /**< heritage object is present */
} ecma_constructor_function_flags_t;

/**
 * Description of AsyncFromSyncIterator objects.
 */
typedef struct
{
  ecma_extended_object_t header; /**< header part */
  ecma_value_t sync_next_method; /**< IteratorRecord [[NextMethod]] internal slot */
} ecma_async_from_sync_iterator_object_t;

/**
 * Private method kind
 */
typedef enum
{
  ECMA_PRIVATE_FIELD = 0, /**< private field */
  ECMA_PRIVATE_METHOD, /**< private method */
  ECMA_PRIVATE_GETTER, /**< private setter */
  ECMA_PRIVATE_SETTER, /**< private getter */
} ecma_private_property_kind_t;

/**
 * Private static property flag
 */
#define ECMA_PRIVATE_PROPERTY_STATIC_FLAG (1 << 2)

/*
 * Get private property kind from a descriptor
 */
#define ECMA_PRIVATE_PROPERTY_KIND(prop) ((ecma_private_property_kind_t) ((int) (prop) & (ECMA_PRIVATE_PROPERTY_STATIC_FLAG - 1)))

/**
 * @}
 * @}
 */

#endif /* !ECMA_GLOBALS_H */
