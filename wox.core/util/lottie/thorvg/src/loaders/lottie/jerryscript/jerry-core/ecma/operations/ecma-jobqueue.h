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

#ifndef ECMA_JOB_QUEUE_H
#define ECMA_JOB_QUEUE_H

#include "ecma-globals.h"

/** \addtogroup ecma ECMA
 * @{
 *
 * \addtogroup ecmajobqueue ECMA Job Queue related routines
 * @{
 */

/**
 * Job queue item types.
 */
typedef enum
{
  ECMA_JOB_PROMISE_REACTION, /**< promise reaction job */
  ECMA_JOB_PROMISE_ASYNC_REACTION_FULFILLED, /**< fulfilled promise async reaction job */
  ECMA_JOB_PROMISE_ASYNC_REACTION_REJECTED, /**< rejected promise async reaction job */
  ECMA_JOB_PROMISE_ASYNC_GENERATOR, /**< continue async generator */
  ECMA_JOB_PROMISE_THENABLE, /**< promise thenable job */
} ecma_job_queue_item_type_t;

/**
 * Description of the job queue item.
 */
typedef struct
{
  uintptr_t next_and_type; /**< next and type members of a queue item */
} ecma_job_queue_item_t;

void ecma_job_queue_init (void);

void ecma_enqueue_promise_reaction_job (ecma_value_t capability, ecma_value_t handler, ecma_value_t argument);
void ecma_enqueue_promise_async_reaction_job (ecma_value_t executable_object, ecma_value_t argument, bool is_rejected);
void ecma_enqueue_promise_async_generator_job (ecma_value_t executable_object);
void ecma_enqueue_promise_resolve_thenable_job (ecma_value_t promise, ecma_value_t thenable, ecma_value_t then);
void ecma_free_all_enqueued_jobs (void);

ecma_value_t ecma_process_all_enqueued_jobs (void);

/**
 * @}
 * @}
 */

#endif /* !ECMA_JOB_QUEUE_H */
