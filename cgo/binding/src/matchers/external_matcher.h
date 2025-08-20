#ifndef MONGORY_MATCHER_EXTERNAL_H
#define MONGORY_MATCHER_EXTERNAL_H

/**
 * @file external_matcher.h
 * @brief Defines the constructor for matchers that are not built-in.
 * This is an internal header for the matcher module.
 */

#include "base_matcher.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include "mongory-core/matchers/matcher.h" // For mongory_matcher structure

/**
 * @brief Creates a regular expression ($regex) matcher.
 *
 * This matcher tests if an input string `mongory_value` matches a given regular
 * expression. The regular expression itself is provided in the `condition`
 * value, which can be either a `MONGORY_TYPE_STRING` (the pattern) or a
 * `MONGORY_TYPE_REGEX` (a pre-compiled regex object, if the underlying regex
 * engine supports it and it's wrapped). The actual regex matching logic is
 * delegated to a function provided via `mongory_regex_func_set` (see
 * `foundations/config.h`).
 *
 * @param pool Memory pool for allocation.
 * @param condition A `mongory_value` representing the regex pattern. This
 * should be of type `MONGORY_TYPE_STRING` or `MONGORY_TYPE_REGEX`.
 * @return A new $regex matcher, or NULL on failure or if the condition is
 * invalid.
 */
mongory_matcher *mongory_matcher_regex_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx);

/**
 * @brief Creates a new custom matcher instance.
 *
 * @param pool The memory pool to use for the matcher's allocations.
 * @param key The key for the custom matcher.
 * @param condition The `mongory_value` representing the condition for this
 * matcher.
 * @return mongory_matcher* A pointer to the newly created custom matcher, or
 * NULL on failure.
 */
mongory_matcher *mongory_matcher_custom_new(mongory_memory_pool *pool, char *key, mongory_value *condition, void *extern_ctx);

#endif /* MONGORY_MATCHER_EXTERNAL_H */
