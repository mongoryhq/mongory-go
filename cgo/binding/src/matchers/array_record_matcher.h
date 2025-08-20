#ifndef MONGORY_MATCHER_ARRAY_RECORD_H
#define MONGORY_MATCHER_ARRAY_RECORD_H

/**
 * @file array_record_matcher.h
 * @brief Defines the constructor for a matcher that handles complex conditions
 * against arrays. This is an internal header for the matcher module.
 *
 * This matcher is more specialized than simple `$elemMatch` or `$every`. It can
 * interpret a condition that might be a direct value to find, a regex to match
 * elements, or a table defining multiple criteria for array elements (similar
 * to an implicit `$elemMatch` with that table). It can also handle checking if
 * an input array is equal to a condition array.
 */

#include "base_matcher.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure

/**
 * @brief Creates an "array record" matcher.
 *
 * This matcher is designed to apply various types of conditions to an input
 * array or its elements. The behavior depends on the `condition`'s type:
 * - If `condition` is a table: It's parsed. Keys like `$elemMatch` are handled
 *   specifically. Other field-value pairs in the table are typically wrapped
 *   into an implicit `$elemMatch` condition that applies to elements of the
 *   target array.
 * - If `condition` is a regex: It effectively creates an `$elemMatch` where
 *   each element of the target array is matched against this regex.
 * - If `condition` is a simple literal (string, number, bool): It creates an
 *   `$elemMatch` where elements are checked for equality against this literal.
 * - If `condition` itself is an array: It creates a matcher that checks if the
 *   target array is equal to this condition array.
 *
 * The resulting matcher might be a composite of several internal matchers (e.g.,
 * an OR between element matching and whole-array equality).
 *
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` representing the condition to apply to an
 * array or its elements.
 * @return A new array record matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_array_record_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

#endif /* MONGORY_MATCHER_ARRAY_RECORD_H */
