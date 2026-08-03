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

#ifndef JERRYSCRIPT_COMPILER_H
#define JERRYSCRIPT_COMPILER_H

#ifdef __cplusplus
#define JERRY_C_API_BEGIN extern "C" {
#define JERRY_C_API_END   }
#else /* !__cplusplus */
#define JERRY_C_API_BEGIN
#define JERRY_C_API_END
#endif /* __cplusplus */

JERRY_C_API_BEGIN

/** \addtogroup jerry-compiler Jerry compiler compatibility components
 * @{
 */

#ifdef __GNUC__

/*
 * Compiler-specific macros relevant for GCC.
 */
#define JERRY_ATTR_ALIGNED(ALIGNMENT) __attribute__ ((aligned (ALIGNMENT)))
#define JERRY_ATTR_CONST              __attribute__ ((const))
#define JERRY_ATTR_DEPRECATED         __attribute__ ((deprecated))
#define JERRY_ATTR_FORMAT(...)        __attribute__ ((format (__VA_ARGS__)))
#define JERRY_ATTR_HOT                __attribute__ ((hot))
#define JERRY_ATTR_NOINLINE           __attribute__ ((noinline))
#define JERRY_ATTR_NORETURN           __attribute__ ((noreturn))
#define JERRY_ATTR_PURE               __attribute__ ((pure))
#define JERRY_ATTR_WARN_UNUSED_RESULT __attribute__ ((warn_unused_result))
#define JERRY_ATTR_WEAK               __attribute__ ((weak))

#define JERRY_WEAK_SYMBOL_SUPPORT

#ifndef JERRY_LIKELY
#define JERRY_LIKELY(x) __builtin_expect (!!(x), 1)
#endif /* !JERRY_LIKELY */

#ifndef JERRY_UNLIKELY
#define JERRY_UNLIKELY(x) __builtin_expect (!!(x), 0)
#endif /* !JERRY_UNLIKELY */

#endif /* __GNUC__ */

#ifdef _MSC_VER

/*
 * Compiler-specific macros relevant for Microsoft Visual C/C++ Compiler.
 */
#define JERRY_ATTR_DEPRECATED __declspec (deprecated)
#define JERRY_ATTR_NOINLINE   __declspec (noinline)
#define JERRY_ATTR_NORETURN   __declspec (noreturn)

/*
 * Microsoft Visual C/C++ Compiler doesn't support for VLA, using _alloca
 * instead.
 */
void *__cdecl _alloca (size_t _Size);
#define JERRY_VLA(type, name, size) type *name = (type *) (_alloca (sizeof (type) * (size)))

#endif /* _MSC_VER */

/*
 * Default empty definitions for all compiler-specific macros. Define any of
 * these in a guarded block above (e.g., as for GCC) to fine tune compilation
 * for your own compiler. */

/**
 * Function attribute to align function to given number of bytes.
 */
#ifndef JERRY_ATTR_ALIGNED
#define JERRY_ATTR_ALIGNED(ALIGNMENT)
#endif /* !JERRY_ATTR_ALIGNED */


/**
 * Function attribute to declare that function has no effect except the return
 * value and it only depends on parameters.
 */
#ifndef JERRY_ATTR_CONST
#define JERRY_ATTR_CONST
#endif /* !JERRY_ATTR_CONST */

/**
 * Function attribute to trigger warning if deprecated function is called.
 */
#ifndef JERRY_ATTR_DEPRECATED
#define JERRY_ATTR_DEPRECATED
#endif /* !JERRY_ATTR_DEPRECATED */

/**
 * Function attribute to declare that function is variadic and takes a format
 * string and some arguments as parameters.
 */
#ifndef JERRY_ATTR_FORMAT
#define JERRY_ATTR_FORMAT(...)
#endif /* !JERRY_ATTR_FORMAT */

/**
 * Function attribute to predict that function is a hot spot, and therefore
 * should be optimized aggressively.
 */
#ifndef JERRY_ATTR_HOT
#define JERRY_ATTR_HOT
#endif /* !JERRY_ATTR_HOT */

/**
 * Function attribute not to inline function ever.
 */
#ifndef JERRY_ATTR_NOINLINE
#define JERRY_ATTR_NOINLINE
#endif /* !JERRY_ATTR_NOINLINE */

/**
 * Function attribute to declare that function never returns.
 */
#ifndef JERRY_ATTR_NORETURN
#define JERRY_ATTR_NORETURN
#endif /* !JERRY_ATTR_NORETURN */

/**
 * Function attribute to declare that function has no effect except the return
 * value and it only depends on parameters and global variables.
 */
#ifndef JERRY_ATTR_PURE
#define JERRY_ATTR_PURE
#endif /* !JERRY_ATTR_PURE */

/**
 * Function attribute to trigger warning if function's caller doesn't use (e.g.,
 * check) the return value.
 */
#ifndef JERRY_ATTR_WARN_UNUSED_RESULT
#define JERRY_ATTR_WARN_UNUSED_RESULT
#endif /* !JERRY_ATTR_WARN_UNUSED_RESULT */

/**
 * Function attribute to declare a function a weak symbol
 */
#ifndef JERRY_ATTR_WEAK
#define JERRY_ATTR_WEAK
#endif /* !JERRY_ATTR_WEAK */

/**
 * Helper to predict that a condition is likely.
 */
#ifndef JERRY_LIKELY
#define JERRY_LIKELY(x) (x)
#endif /* !JERRY_LIKELY */

/**
 * Helper to predict that a condition is unlikely.
 */
#ifndef JERRY_UNLIKELY
#define JERRY_UNLIKELY(x) (x)
#endif /* !JERRY_UNLIKELY */

/**
 * Helper to declare (or mimic) a C99 variable-length array.
 */
#ifndef JERRY_VLA
#define JERRY_VLA(type, name, size) type* name = (type*)alloca((size) * sizeof(type))
#endif /* !JERRY_VLA */

/**
 * @}
 */

JERRY_C_API_END

#endif /* !JERRYSCRIPT_COMPILER_H */
