#ifndef MONGORY_MATCHER_EXISTANCE_H
#define MONGORY_MATCHER_EXISTANCE_H

/**
 * @file existance_matcher.h
 * @brief Defines constructors for existence and presence matchers ($exists,
 * $present). This is an internal header for the matcher module.
 */

#include "base_matcher.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure

/**
 * @brief Creates an "exists" ($exists) matcher.
 *
 * This matcher checks for the existence of a field (i.e., if a value is
 * non-NULL when retrieved from a table or array). The `condition` for this
 * matcher must be a boolean `mongory_value`.
 * - If `condition` is true, it matches if the field exists (value is not NULL).
 * - If `condition` is false, it matches if the field does not exist (value is
 * NULL).
 *
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_BOOL`.
 * @return A new `$exists` matcher, or NULL on failure or invalid condition.
 */
mongory_matcher *mongory_matcher_exists_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "present" ($present) matcher.
 *
 * This matcher provides a more nuanced check than `$exists`. It considers a
 * value "present" based on its type and content (e.g., non-empty array/table,
 * non-empty string). The `condition` for this matcher must be a boolean
 * `mongory_value`.
 * - If `condition` is true, it matches if the value is considered "present".
 * - If `condition` is false, it matches if the value is not "present" (e.g.,
 * NULL, empty array/table/string).
 *
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_BOOL`.
 * @return A new `$present` matcher, or NULL on failure or invalid condition.
 */
mongory_matcher *mongory_matcher_present_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

#endif /* MONGORY_MATCHER_EXISTANCE_H */
