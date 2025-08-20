#ifndef MONGORY_MATCHER_H
#define MONGORY_MATCHER_H

/**
 * @file matcher.h
 * @brief Defines the core mongory_matcher structure and related types.
 *
 * This file provides the basic structure for all matchers in the Mongory
 * library. A matcher is responsible for determining if a given `mongory_value`
 * meets certain criteria defined by a `condition`.
 */

#include "mongory-core/foundations/array.h" // For mongory_array (used in context)
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"

// Forward declaration for the main matcher structure.
typedef struct mongory_matcher mongory_matcher;
/**
 * @brief Creates a new generic matcher instance.
 *
 * This function typically serves as a high-level entry point for creating
 * matchers. In the current implementation, it delegates to
 * `mongory_matcher_table_cond_new`, implying that conditions are often
 * table-based (similar to query documents).
 *
 * @param pool The memory pool to use for the matcher's allocations.
 * @param condition A `mongory_value` representing the condition for this
 * matcher.
 * @return mongory_matcher* A pointer to the newly created matcher, or NULL on
 * failure.
 */
mongory_matcher *mongory_matcher_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Matches a value against a matcher.
 * @param matcher The matcher to match against.
 * @param value The value to match.
 * @return True if the value matches the matcher, false otherwise.
 */
bool mongory_matcher_match(mongory_matcher *matcher, mongory_value *value);

/**
 * @brief Explains a matcher.
 * @param matcher The matcher to explain.
 * @param temp_pool The temporary pool to use for the explanation.
 */
void mongory_matcher_explain(mongory_matcher *matcher, mongory_memory_pool *temp_pool);

/**
 * @brief Traces a matcher.
 * @param matcher The matcher to trace.
 * @param value The value to trace.
 */
bool mongory_matcher_trace(mongory_matcher *matcher, mongory_value *value);

/**
 * @brief Prints the trace of a matcher.
 * @param matcher The matcher to print the trace of.
 */
void mongory_matcher_print_trace(mongory_matcher *matcher);

/**
 * @brief Enables the trace of a matcher.
 * @param matcher The matcher to enable the trace of.
 * @param temp_pool The temporary pool to use for the trace.
 */
void mongory_matcher_enable_trace(mongory_matcher *matcher, mongory_memory_pool *temp_pool);

/**
 * @brief Disables the trace of a matcher.
 * @param matcher The matcher to disable the trace of.
 */
void mongory_matcher_disable_trace(mongory_matcher *matcher);

#endif /* MONGORY_MATCHER_H */
