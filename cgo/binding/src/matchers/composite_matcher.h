#ifndef MONGORY_MATCHER_COMPOSITE_H
#define MONGORY_MATCHER_COMPOSITE_H

/**
 * @file composite_matcher.h
 * @brief Defines structures and constructors for composite matchers.
 * This is an internal header for the matcher module.
 *
 * Composite matchers combine other matchers, such as logical AND/OR,
 * or apply a matcher to elements of an array or fields of a table.
 */

#include "base_matcher.h"
#include "mongory-core/foundations/array.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure

/**
 * @struct mongory_composite_matcher
 * @brief Represents a matcher composed of other matchers.
 *
 * Typically has a `left` and/or `right` child matcher. The interpretation
 * of these children depends on the specific composite matcher type (e.g., for
 * AND/OR, both are used; for $elemMatch, `left` might be the sub-matcher for
 * elements).
 */
typedef struct mongory_composite_matcher {
  mongory_matcher base;   /**< Base matcher structure. */
  mongory_array *children; /**< Children matchers. */
} mongory_composite_matcher;

/** @name Composite Matcher Constructors
 *  Functions to create instances of various composite matchers.
 * @{
 */

/**
 * @brief Creates an "AND" ($and) matcher.
 * Matches if all conditions specified in an array of condition tables are met.
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_ARRAY`, where each
 * element is a `MONGORY_TYPE_TABLE` representing a sub-condition.
 * @return A new `$and` matcher, or NULL on failure or if condition is invalid.
 */
mongory_matcher *mongory_matcher_and_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates an "OR" ($or) matcher.
 * Matches if any condition specified in an array of condition tables is met.
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_ARRAY`, where each
 * element is a `MONGORY_TYPE_TABLE` representing a sub-condition.
 * @return A new `$or` matcher, or NULL on failure or if condition is invalid.
 */
mongory_matcher *mongory_matcher_or_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates an "element match" ($elemMatch) matcher.
 * Matches an array field if at least one element in the array matches the given
 * condition table.
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_TABLE` representing
 * the condition to apply to array elements.
 * @return A new `$elemMatch` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_elem_match_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates an "every element matches" ($every) matcher.
 * Matches an array field if ALL elements in the array match the given condition
 * table. (Note: This is a common pattern, though MongoDB's $all has more
 * complex behavior. This $every is simpler.)
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_TABLE` representing
 * the condition to apply to array elements.
 * @return A new `$every` matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_every_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a "table condition" matcher.
 * This is a core matcher that parses a condition table (similar to a MongoDB
 * query document) and builds a tree of sub-matchers based on its keys and
 * values. Field names imply field matchers, and `$`-prefixed keys imply
 * operator matchers. These are implicitly ANDed together.
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` of type `MONGORY_TYPE_TABLE`.
 * @return A new table condition matcher, or NULL on failure/invalid condition.
 */
mongory_matcher *mongory_matcher_table_cond_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);
/** @} */

/**
 * @brief Allocates and initializes a base `mongory_composite_matcher`
 * structure.
 *
 * Sets up the base `mongory_matcher` fields within the composite structure.
 * Child matchers (`left`, `right`) and the specific `match` function must be
 * set by the caller.
 *
 * @param pool Memory pool for allocation.
 * @param condition The condition associated with this composite matcher (can be
 * NULL if the condition is implicitly defined by children).
 * @return mongory_composite_matcher* Pointer to the new composite matcher
 * structure, or NULL on failure.
 */
mongory_composite_matcher *mongory_matcher_composite_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief The actual match logic for an OR operation on a composite matcher.
 *
 * This function is typically assigned to the `match` field of an OR composite
 * matcher. It checks if either the `left` or `right` child matcher matches the
 * given value.
 *
 * @param matcher A pointer to the `mongory_matcher` (which is expected to be a
 * `mongory_composite_matcher` for OR).
 * @param value The `mongory_value` to evaluate.
 * @return bool True if `left->match()` or `right->match()` returns true, false
 * otherwise. Returns false if children are NULL or if their match calls fail.
 */
bool mongory_matcher_or_match(mongory_matcher *matcher, mongory_value *value);

#endif /* MONGORY_MATCHER_COMPOSITE_H */
