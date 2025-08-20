#ifndef MONGORY_MATCHER_COMPARE_H
#define MONGORY_MATCHER_COMPARE_H

/**
 * @file compare_matcher.h
 * @brief Defines constructors for comparison matchers (e.g., equality, greater
 * than). This is an internal header for the matcher module.
 *
 * These matchers compare an input `mongory_value` against a `condition` value
 * stored within the matcher, using the `comp` function of the input value.
 */

#include "base_matcher.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure
#include <stdbool.h>

/** @name Comparison Matcher Constructors
 *  Functions to create instances of various comparison matchers.
 *  Each takes a memory pool and a condition value.
 * @{
 */

/**
 * @brief Creates an "equal" ($eq) matcher.
 * Matches if the input value is equal to the matcher's condition value.
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` to compare against.
 * @return A new `$eq` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "not equal" ($ne) matcher.
 * Matches if the input value is not equal to the matcher's condition value.
 * Also matches if the types are incompatible for comparison.
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` to compare against.
 * @return A new `$ne` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_not_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "greater than" ($gt) matcher.
 * Matches if the input value is greater than the matcher's condition value.
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` to compare against.
 * @return A new `$gt` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_greater_than_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "less than" ($lt) matcher.
 * Matches if the input value is less than the matcher's condition value.
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` to compare against.
 * @return A new `$lt` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_less_than_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "greater than or equal" ($gte) matcher.
 * Matches if the input value is greater than or equal to the matcher's
 * condition value.
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` to compare against.
 * @return A new `$gte` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_greater_than_or_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "less than or equal" ($lte) matcher.
 * Matches if the input value is less than or equal to the matcher's condition
 * value.
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` to compare against.
 * @return A new `$lte` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_less_than_or_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);
/** @} */

#endif /* MONGORY_MATCHER_COMPARE_H */
