#ifndef MONGORY_MATCHER_INCLUSION_H
#define MONGORY_MATCHER_INCLUSION_H

/**
 * @file inclusion_matcher.h
 * @brief Defines constructors for inclusion matchers ($in, $nin).
 * This is an internal header for the matcher module.
 *
 * These matchers check if a value is present or not present in a given array
 * of values.
 */

#include "base_matcher.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure

/**
 * @brief Creates an "in" ($in) matcher.
 *
 * Matches if the input value is equal to any of the values in the `condition`
 * array. If the input value itself is an array, it matches if any element of
 * the input array is found in the `condition` array (set intersection).
 *
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_ARRAY` containing
 * the set of values to check against.
 * @return A new `$in` matcher, or NULL on failure or invalid condition.
 */
mongory_matcher *mongory_matcher_in_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "not in" ($nin) matcher.
 *
 * Matches if the input value is not equal to any of the values in the
 * `condition` array. If the input value itself is an array, it matches if no
 * element of the input array is found in the `condition` array.
 *
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_ARRAY` containing
 * the set of values to check against.
 * @return A new `$nin` matcher, or NULL on failure or invalid condition.
 */
mongory_matcher *mongory_matcher_not_in_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

#endif /* MONGORY_MATCHER_INCLUSION_H */
