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

ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_PROMISE_RESOLVE, ecma_promise_resolve_handler, 1)
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_PROMISE_REJECT, ecma_promise_reject_handler, 1)
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_PROMISE_THEN_FINALLY, ecma_promise_then_finally_cb, 1)
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_PROMISE_CATCH_FINALLY, ecma_promise_catch_finally_cb, 1)
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_PROMISE_ALL_HELPER, ecma_promise_all_or_all_settled_handler_cb, 1)
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_PROMISE_CAPABILITY_EXECUTOR, ecma_op_get_capabilities_executor_cb, 2)
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_ASYNC_FROM_SYNC_ITERATOR_UNWRAP, ecma_async_from_sync_iterator_unwrap_cb, 1)
#if JERRY_BUILTIN_PROXY
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_PROXY_REVOKE, ecma_proxy_revoke_cb, 0)
#endif /* JERRY_BUILTIN_PROXY */
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_VALUE_THUNK, ecma_value_thunk_helper_cb, 0)
ECMA_NATIVE_HANDLER (ECMA_NATIVE_HANDLER_VALUE_THROWER, ecma_value_thunk_thrower_cb, 0)
