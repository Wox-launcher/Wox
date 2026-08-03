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

/* Description of built-in objects
   in format (ECMA_BUILTIN_ID_id, object_type, prototype_id, is_extensible, is_static, underscored_id) */

/* The Object.prototype object (15.2.4) */
BUILTIN (ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID__COUNT /* no prototype */,
         true,
         object_prototype)

/* The Object object (15.2.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_OBJECT,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 object)

#if JERRY_BUILTIN_ARRAY
/* The Array.prototype object (15.4.4) */
BUILTIN (ECMA_BUILTIN_ID_ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_ARRAY,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         array_prototype)

/* The Array object (15.4.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 array)
#endif /* JERRY_BUILTIN_ARRAY */

#if JERRY_BUILTIN_DATE
/* The Date.prototype object (20.3.4) */
BUILTIN (ECMA_BUILTIN_ID_DATE_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         date_prototype)
#endif /* JERRY_BUILTIN_DATE */

#if JERRY_BUILTIN_REGEXP
/* The RegExp.prototype object (21.2.5) */
BUILTIN (ECMA_BUILTIN_ID_REGEXP_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         regexp_prototype)
#endif /* JERRY_BUILTIN_REGEXP */

#if JERRY_BUILTIN_STRING
/* The String.prototype object (15.5.4) */
BUILTIN (ECMA_BUILTIN_ID_STRING_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_CLASS,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         string_prototype)

/* The String object (15.5.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_STRING,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 string)
#endif /* JERRY_BUILTIN_STRING */

#if JERRY_BUILTIN_BOOLEAN
/* The Boolean.prototype object (15.6.4) */
BUILTIN (ECMA_BUILTIN_ID_BOOLEAN_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_CLASS,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         boolean_prototype)

/* The Boolean object (15.6.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_BOOLEAN,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 boolean)
#endif /* JERRY_BUILTIN_BOOLEAN */

#if JERRY_BUILTIN_NUMBER
/* The Number.prototype object (15.7.4) */
BUILTIN (ECMA_BUILTIN_ID_NUMBER_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_CLASS,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         number_prototype)

/* The Number object (15.7.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_NUMBER,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 number)
#endif /* JERRY_BUILTIN_NUMBER */

/* The Function.prototype object (15.3.4) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
                 true,
                 function_prototype)

/* The Function object (15.3.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_FUNCTION,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 function)

#if JERRY_BUILTIN_MATH
/* The Math object (15.8) */
BUILTIN (ECMA_BUILTIN_ID_MATH, ECMA_OBJECT_TYPE_BUILT_IN_GENERAL, ECMA_BUILTIN_ID_OBJECT_PROTOTYPE, true, math)
#endif /* JERRY_BUILTIN_MATH */

#if JERRY_BUILTIN_REFLECT

/* The Reflect object (26.1) */
BUILTIN (ECMA_BUILTIN_ID_REFLECT, ECMA_OBJECT_TYPE_BUILT_IN_GENERAL, ECMA_BUILTIN_ID_OBJECT_PROTOTYPE, true, reflect)
#endif /* JERRY_BUILTIN_REFLECT */

#if JERRY_BUILTIN_ATOMICS
/* The Atomics object (24.4) */
BUILTIN (ECMA_BUILTIN_ID_ATOMICS, ECMA_OBJECT_TYPE_BUILT_IN_GENERAL, ECMA_BUILTIN_ID_OBJECT_PROTOTYPE, true, atomics)
#endif /* JERRY_BUILTIN_ATOMICS */

#if JERRY_BUILTIN_DATE
/* The Date object (15.9.3) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_DATE,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 date)
#endif /* JERRY_BUILTIN_DATE */

#if JERRY_BUILTIN_REGEXP
/* The RegExp object (15.10) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_REGEXP,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 regexp)
#endif /* JERRY_BUILTIN_REGEXP */

#define ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID ECMA_BUILTIN_ID_ERROR

/* The Error object (15.11.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 error)

/* The Error.prototype object (15.11.4) */
BUILTIN (ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         error_prototype)

#if JERRY_BUILTIN_ERRORS
/* The EvalError.prototype object (15.11.6.1) */
BUILTIN (ECMA_BUILTIN_ID_EVAL_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         true,
         eval_error_prototype)

/* The EvalError object (15.11.6.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_EVAL_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID,
                 true,
                 eval_error)

/* The RangeError.prototype object (15.11.6.2) */
BUILTIN (ECMA_BUILTIN_ID_RANGE_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         true,
         range_error_prototype)

/* The RangeError object (15.11.6.2) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_RANGE_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID,
                 true,
                 range_error)

/* The ReferenceError.prototype object (15.11.6.3) */
BUILTIN (ECMA_BUILTIN_ID_REFERENCE_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         true,
         reference_error_prototype)

/* The ReferenceError object (15.11.6.3) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_REFERENCE_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID,
                 true,
                 reference_error)

/* The SyntaxError.prototype object (15.11.6.4) */
BUILTIN (ECMA_BUILTIN_ID_SYNTAX_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         true,
         syntax_error_prototype)

/* The SyntaxError object (15.11.6.4) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_SYNTAX_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID,
                 true,
                 syntax_error)

/* The TypeError.prototype object (15.11.6.5) */
BUILTIN (ECMA_BUILTIN_ID_TYPE_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         true,
         type_error_prototype)

/* The TypeError object (15.11.6.5) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_TYPE_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID,
                 true,
                 type_error)

/* The AggregateError.prototype object (15.11.6.5) */
BUILTIN (ECMA_BUILTIN_ID_AGGREGATE_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         true,
         aggregate_error_prototype)

/* The AggregateError object (15.11.6.5) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_AGGREGATE_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID,
                 true,
                 aggregate_error)

/* The URIError.prototype object (15.11.6.6) */
BUILTIN (ECMA_BUILTIN_ID_URI_ERROR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ERROR_PROTOTYPE,
         true,
         uri_error_prototype)

/* The URIError object (15.11.6.6) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_URI_ERROR,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_NATIVE_ERROR_PROTOTYPE_ID,
                 true,
                 uri_error)
#endif /* JERRY_BUILTIN_ERRORS */

/**< The [[ThrowTypeError]] object (13.2.3) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_TYPE_ERROR_THROWER,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 false,
                 type_error_thrower)

#if JERRY_BUILTIN_TYPEDARRAY

/* The ArrayBuffer.prototype object (ES2015 24.1.4) */
BUILTIN (ECMA_BUILTIN_ID_ARRAYBUFFER_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         arraybuffer_prototype)

/* The ArrayBuffer object (ES2015 24.1.2) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_ARRAYBUFFER,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 arraybuffer)

#if JERRY_BUILTIN_SHAREDARRAYBUFFER

/* The SharedArrayBuffer.prototype object (ES2015 24.2.4) */
BUILTIN (ECMA_BUILTIN_ID_SHARED_ARRAYBUFFER_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         shared_arraybuffer_prototype)

/* The SharedArrayBuffer object (ES2015 24.2.2) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_SHARED_ARRAYBUFFER,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 shared_arraybuffer)
#endif /* JERRY_BUILTIN_SHAREDARRAYBUFFER */

/* The %TypedArrayPrototype% object (ES2015 24.2.3) */
BUILTIN (ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         typedarray_prototype)

/* The %TypedArray% intrinsic object (ES2015 22.2.1)
   Note: The routines must be in this order. */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_TYPEDARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 typedarray)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_INT8ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 int8array)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_UINT8ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 uint8array)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_UINT8CLAMPEDARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 uint8clampedarray)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_INT16ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 int16array)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_UINT16ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 uint16array)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_INT32ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 int32array)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_UINT32ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 uint32array)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_FLOAT32ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 float32array)


#if JERRY_BUILTIN_BIGINT
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_BIGINT64ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 bigint64array)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_BIGUINT64ARRAY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_TYPEDARRAY,
                 true,
                 biguint64array)
#endif /* JERRY_BUILTIN_BIGINT */

BUILTIN (ECMA_BUILTIN_ID_INT8ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         int8array_prototype)

BUILTIN (ECMA_BUILTIN_ID_UINT8ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         uint8array_prototype)

BUILTIN (ECMA_BUILTIN_ID_UINT8CLAMPEDARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         uint8clampedarray_prototype)

BUILTIN (ECMA_BUILTIN_ID_INT16ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         int16array_prototype)

BUILTIN (ECMA_BUILTIN_ID_UINT16ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         uint16array_prototype)

BUILTIN (ECMA_BUILTIN_ID_INT32ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         int32array_prototype)

BUILTIN (ECMA_BUILTIN_ID_UINT32ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         uint32array_prototype)

BUILTIN (ECMA_BUILTIN_ID_FLOAT32ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         float32array_prototype)


#if JERRY_BUILTIN_BIGINT
BUILTIN (ECMA_BUILTIN_ID_BIGINT64ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         bigint64array_prototype)

BUILTIN (ECMA_BUILTIN_ID_BIGUINT64ARRAY_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_TYPEDARRAY_PROTOTYPE,
         true,
         biguint64array_prototype)
#endif /* JERRY_BUILTIN_BIGINT */
#endif /* JERRY_BUILTIN_TYPEDARRAY */

BUILTIN (ECMA_BUILTIN_ID_PROMISE_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         promise_prototype)

BUILTIN_ROUTINE (ECMA_BUILTIN_ID_PROMISE,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 promise)

#if JERRY_BUILTIN_CONTAINER

/* The Map prototype object (23.1.3) */
BUILTIN (ECMA_BUILTIN_ID_MAP_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         map_prototype)

/* The Map routine (ECMA-262 v6, 23.1.1.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_MAP, ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION, ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE, true, map)

/* The Set prototype object (23.1.3) */
BUILTIN (ECMA_BUILTIN_ID_SET_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         set_prototype)

/* The Set routine (ECMA-262 v6, 23.1.1.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_SET, ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION, ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE, true, set)

/* The WeakMap prototype object (23.1.3) */
BUILTIN (ECMA_BUILTIN_ID_WEAKMAP_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         weakmap_prototype)

/* The WeakMap routine (ECMA-262 v6, 23.1.1.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_WEAKMAP,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 weakmap)

/* The WeakSet prototype object (23.1.3) */
BUILTIN (ECMA_BUILTIN_ID_WEAKSET_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         weakset_prototype)

/* The WeakSet routine (ECMA-262 v6, 23.1.1.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_WEAKSET,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 weakset)

#endif /* JERRY_BUILTIN_CONTAINER */

#if JERRY_BUILTIN_WEAKREF

/* The WeakRef prototype object */
BUILTIN (ECMA_BUILTIN_ID_WEAKREF_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         weakref_prototype)

/* The WeakRef routine */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_WEAKREF,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 weakref)

#endif /* JERRY_BUILTIN_WEAKREF */

#if (JERRY_BUILTIN_PROXY)
/* The Proxy routine (ECMA-262 v6, 26.2.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_PROXY,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 proxy)
#endif /* JERRY_BUILTIN_PROXY */

/* Intrinsic hidden builtin object  */
BUILTIN (ECMA_BUILTIN_ID_INTRINSIC_OBJECT,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID__COUNT /* no prototype */,
         true,
         intrinsic)

/* The Array.prototype[@@unscopables] object */
BUILTIN (ECMA_BUILTIN_ID_ARRAY_PROTOTYPE_UNSCOPABLES,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID__COUNT /* no prototype */,
         true,
         array_prototype_unscopables)

/* The Symbol prototype object (ECMA-262 v6, 19.4.2.7) */
BUILTIN (ECMA_BUILTIN_ID_SYMBOL_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         symbol_prototype)

/* The Symbol routine (ECMA-262 v6, 19.4.2.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_SYMBOL,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 symbol)

/* The %AsyncFunction% object (ECMA-262 v11, 25.7.2) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_ASYNC_FUNCTION,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION,
                 true,
                 async_function)

/* The %AsyncFunctionPrototype% object (ECMA-262 v11, 25.7.3) */
BUILTIN (ECMA_BUILTIN_ID_ASYNC_FUNCTION_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
         true,
         async_function_prototype)

/* The %IteratorPrototype% object (ECMA-262 v6, 25.1.2) */
BUILTIN (ECMA_BUILTIN_ID_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         iterator_prototype)

/* The %ArrayIteratorPrototype% object (ECMA-262 v6, 22.1.5.2) */
BUILTIN (ECMA_BUILTIN_ID_ARRAY_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ITERATOR_PROTOTYPE,
         true,
         array_iterator_prototype)

/* The %StringIteratorPrototype% object (ECMA-262 v6, 22.1.5.2) */
BUILTIN (ECMA_BUILTIN_ID_STRING_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ITERATOR_PROTOTYPE,
         true,
         string_iterator_prototype)

/* The %RegExpStringIteratorPrototype% object (ECMA-262 v11, 21.2.7.1) */
BUILTIN (ECMA_BUILTIN_ID_REGEXP_STRING_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ITERATOR_PROTOTYPE,
         true,
         regexp_string_iterator_prototype)

/* The %AsyncIteratorPrototype% object (ECMA-262 v10, 25.1.3) */
BUILTIN (ECMA_BUILTIN_ID_ASYNC_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         async_iterator_prototype)

/* The %AsyncFromSyncIteratorPrototype% object (ECMA-262 v11, 25.1.4.2) */
BUILTIN (ECMA_BUILTIN_ID_ASYNC_FROM_SYNC_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ASYNC_ITERATOR_PROTOTYPE,
         true,
         async_from_sync_iterator_prototype)

/* The %(GeneratorFunction)% object */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_GENERATOR_FUNCTION,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION,
                 true,
                 generator_function)

/* The %(Generator)% object */
BUILTIN (ECMA_BUILTIN_ID_GENERATOR,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
         true,
         generator)

/* The %(Generator).prototype% object */
BUILTIN (ECMA_BUILTIN_ID_GENERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ITERATOR_PROTOTYPE,
         true,
         generator_prototype)

/* The %(AsyncGeneratorFunction)% object */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_ASYNC_GENERATOR_FUNCTION,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION,
                 true,
                 async_generator_function)

/* The %(AsyncGenerator)% object */
BUILTIN (ECMA_BUILTIN_ID_ASYNC_GENERATOR,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
         true,
         async_generator)

/* The %(AsyncGenerator).prototype% object */
BUILTIN (ECMA_BUILTIN_ID_ASYNC_GENERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ASYNC_ITERATOR_PROTOTYPE,
         true,
         async_generator_prototype)

#if JERRY_BUILTIN_CONTAINER
/* The %SetIteratorPrototype% object (ECMA-262 v6, 23.2.5.2) */
BUILTIN (ECMA_BUILTIN_ID_SET_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ITERATOR_PROTOTYPE,
         true,
         set_iterator_prototype)

/* The %MapIteratorPrototype% object (ECMA-262 v6, 23.1.5.2) */
BUILTIN (ECMA_BUILTIN_ID_MAP_ITERATOR_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_ITERATOR_PROTOTYPE,
         true,
         map_iterator_prototype)
#endif /* JERRY_BUILTIN_CONTAINER */

#if JERRY_BUILTIN_BIGINT
/* The %BigInt.prototype% object */
BUILTIN (ECMA_BUILTIN_ID_BIGINT_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         bigint_prototype)

/* The %BigInt% object */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_BIGINT,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 bigint)
#endif /* JERRY_BUILTIN_BIGINT */

#if JERRY_BUILTIN_DATAVIEW
/* The DataView prototype object (ECMA-262 v6, 24.2.3.1) */
BUILTIN (ECMA_BUILTIN_ID_DATAVIEW_PROTOTYPE,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE,
         true,
         dataview_prototype)

/* The DataView routine (ECMA-262 v6, 24.2.2.1) */
BUILTIN_ROUTINE (ECMA_BUILTIN_ID_DATAVIEW,
                 ECMA_OBJECT_TYPE_BUILT_IN_FUNCTION,
                 ECMA_BUILTIN_ID_FUNCTION_PROTOTYPE,
                 true,
                 dataview)
#endif /* JERRY_BUILTIN_DATAVIEW */

/* The Global object (15.1) */
BUILTIN (ECMA_BUILTIN_ID_GLOBAL,
         ECMA_OBJECT_TYPE_BUILT_IN_GENERAL,
         ECMA_BUILTIN_ID_OBJECT_PROTOTYPE, /* Implementation-dependent */
         true,
         global)

#undef BUILTIN
