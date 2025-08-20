#ifndef MONGORY_MATCHER_LITERAL_H
#define MONGORY_MATCHER_LITERAL_H

/**
 * @file literal_matcher.h
 * @brief Defines constructors for literal-based matchers, field matchers,
 * $not, and $size. This is an internal header for the matcher module.
 *
 * "Literal" here often refers to matching a field's value against a specific
 * condition, which might itself be a simple value or a more complex condition
 * table.
 */

#include "base_matcher.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure
#include "matcher_explainable.h"
#include "composite_matcher.h"

typedef struct mongory_literal_matcher {
  mongory_matcher base;
  mongory_matcher *delegate_matcher;
  mongory_matcher *array_record_matcher;
} mongory_literal_matcher;

/**
 * @struct mongory_field_matcher
 * @brief Specialized composite matcher for matching a specific field.
 * Stores the field name/index.
 */
typedef struct mongory_field_matcher {
  mongory_literal_matcher literal; /**< Base composite matcher structure. */
  char *field;                         /**< Name/index of the field to match. Copied string. */
} mongory_field_matcher;
/**
 * @brief Creates a "field" matcher.
 *
 * This matcher extracts a value from a specified `field` (or array index) of
 * an input `mongory_value` (which is expected to be a table or array). It then
 * applies a sub-matcher (derived from `condition`) to this extracted field
 * value.
 *
 * @param pool Memory pool for allocation.
 * @param field The name of the field (if input is a table) or string
 * representation of an index (if input is an array). A copy of this string is
 * made.
 * @param condition The `mongory_value` condition to apply to the field's value.
 * This condition is processed by `mongory_matcher_literal_delegate` to determine
 * the actual sub-matcher (e.g., equality, regex, nested table condition).
 * @return A new field matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_field_new(mongory_memory_pool *pool, char *field, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "NOT" ($not) matcher.
 *
 * This matcher negates the result of a sub-matcher derived from `condition`.
 * The sub-matcher is typically determined by
 * `mongory_matcher_literal_delegate`.
 *
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` condition whose result will be negated.
 * @return A new $not matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_not_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "size" ($size) matcher.
 *
 * This matcher checks if the size (count of elements) of an input array
 * matches the given `condition`. The `condition` itself is processed by
 * `mongory_matcher_literal_delegate` (e.g., it could be a number for exact
 * size, or a condition table like {$gt: 5}).
 *
 * @param pool Memory pool for allocation.
 * @param condition The `mongory_value` condition to apply to the array's size.
 * @return A new $size matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_size_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "literal" matcher (deprecated or internal use).
 *
 * This function seems to be a more generic entry point that delegates to
 * `mongory_matcher_literal_delegate` and potentially `array_record_matcher`.
 * Its direct public use might be limited compared to `field_new` or specific
 * operator matchers. It appears to be part of the internal logic for how field
 * values are matched.
 *
 * @param pool Memory pool for allocation.
 * @param condition The literal `mongory_value` or condition table.
 * @return A new literal matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_literal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

#endif /* MONGORY_MATCHER_LITERAL_H */
