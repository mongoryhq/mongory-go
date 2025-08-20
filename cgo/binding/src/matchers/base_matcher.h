#ifndef MONGORY_MATCHER_BASE_H
#define MONGORY_MATCHER_BASE_H

/**
 * @file base_matcher.h
 * @brief Defines the base matcher constructor and utility functions for
 * matchers. This is an internal header for the matcher module.
 *
 * This includes a constructor for the fundamental `mongory_matcher` structure,
 * constructors for trivial matchers (always true/false), and a utility
 * for parsing integers from strings.
 */

#include "mongory-core/foundations/array.h" // For mongory_array (context trace)
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure
#include "matcher_explainable.h"
#include "matcher_traversable.h"
#include <stdbool.h>

/**
 * @brief Function pointer type for a matcher's core matching logic.
 *
 * @param matcher A pointer to the `mongory_matcher` instance itself.
 * @param value A pointer to the `mongory_value` to be evaluated against the
 * matcher's condition.
 * @return bool True if the `value` matches the condition, false otherwise.
 */
typedef bool (*mongory_matcher_match_func)(mongory_matcher *matcher, mongory_value *value);

/**
 * @struct mongory_matcher
 * @brief Represents a generic matcher in the Mongory system.
 *
 * Each matcher has a name (optional, for identification), a condition value
 * that defines its criteria, a function pointer to its matching logic,
 * a memory pool for its allocations, and a context.
 */
struct mongory_matcher {
  char *name;                           /**< Optional name for the matcher (e.g., "$eq").
                                           String is typically allocated from the pool. */
  mongory_value *condition;             /**< The condition (a `mongory_value`) that this
                                           matcher evaluates against. */
  mongory_matcher_match_func match;     /**< Function pointer to the specific matching
                                           logic for this matcher type. */
  mongory_memory_pool *pool;            /**< The memory pool used for allocations related
                                           to this matcher instance. */
  mongory_matcher_traverse_func explain; /**< Function pointer to the explanation
                                          logic for this matcher type. */
  mongory_matcher_match_func original_match; /**< Stores the original match function, potentially for
                                                restoration or delegation. */
  size_t sub_count;                          /**< The number of sub-matchers. */
  mongory_matcher_traverse_func traverse;    /**< Function pointer to the traversal logic for this matcher type. */
  mongory_array *trace_stack;                /**< The trace stack for this matcher. */
  int trace_level;                           /**< The trace level for this matcher. */
  double priority;                           /**< The priority for this matcher. */
  void *extern_ctx;                          /**< External context for the matcher. */
};

/**
 * @brief Creates a new base `mongory_matcher` instance and initializes its
 * common fields.
 *
 * This function allocates a `mongory_matcher` structure from the provided pool
 * and sets its `pool` and `condition` fields. The `match` function and other
 * specific fields must be set by the caller or derived matcher constructors.
 * The context's original_match and trace are initialized to NULL.
 *
 * @param pool The memory pool to use for allocating the matcher.
 * @param condition The `mongory_value` representing the condition for this
 * matcher.
 * @return mongory_matcher* A pointer to the newly allocated base matcher, or
 * NULL on allocation failure.
 */
mongory_matcher *mongory_matcher_base_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a new matcher that always evaluates to true.
 *
 * @param pool The memory pool for allocation.
 * @param condition The condition value (often unused by this matcher but stored
 * for consistency).
 * @return mongory_matcher* A pointer to the "always true" matcher, or NULL on
 * failure.
 */
mongory_matcher *mongory_matcher_always_true_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a new matcher that always evaluates to false.
 *
 * @param pool The memory pool for allocation.
 * @param condition The condition value (often unused by this matcher but stored
 * for consistency).
 * @return mongory_matcher* A pointer to the "always false" matcher, or NULL on
 * failure.
 */
mongory_matcher *mongory_matcher_always_false_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

#endif /* MONGORY_MATCHER_BASE_H */