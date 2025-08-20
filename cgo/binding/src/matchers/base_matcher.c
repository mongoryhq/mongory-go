/**
 * @file base_matcher.c
 * @brief Implements the base matcher constructor and related utility functions.
 * This is an internal implementation file for the matcher module.
 */
#include "base_matcher.h"
#include "../foundations/string_buffer.h"
#include <errno.h>        // For errno, ERANGE
#include <limits.h>       // For INT_MIN, INT_MAX
#include <mongory-core.h> // General include, for mongory_matcher types
#include <stdbool.h>
#include <stdio.h>  // For printf
#include <stdlib.h> // For strtol
#include "matcher_explainable.h"
#include "matcher_traversable.h"
#include "../foundations/utils.h"

/**
 * @brief Allocates and initializes common fields of a `mongory_matcher`.
 *
 * This function serves as a common initializer for all specific matcher types.
 * It allocates memory for the `mongory_matcher` structure itself from the
 * provided `pool`, sets the `pool` and `condition` members.
 * The `matcher->match` function pointer specific to the matcher type must be
 * set by the caller. `original_match` and `trace` in the context are set to
 * NULL.
 *
 * @param pool The memory pool to use for allocation. Must be non-NULL and
 * valid.
 * @param condition The condition value for this matcher.
 * @return mongory_matcher* Pointer to the newly allocated and partially
 * initialized matcher, or NULL if allocation fails.
 */
mongory_matcher *mongory_matcher_base_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  if (!pool || !pool->alloc) {
    return NULL; // Invalid memory pool.
  }
  mongory_matcher *matcher = MG_ALLOC_PTR(pool, mongory_matcher);
  if (matcher == NULL) {
    // Allocation failed, pool->alloc might set pool->error.
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }

  // Initialize common fields
  matcher->original_match = NULL;
  matcher->sub_count = 0;
  matcher->pool = pool;
  matcher->condition = condition;
  matcher->name = NULL;                            // Name is not set by base_new.
  matcher->match = NULL;                           // Specific match function must be set by derived type.
  matcher->explain = mongory_matcher_base_explain; // Specific explain function must be set by derived type.
  matcher->traverse = mongory_matcher_leaf_traverse;
  matcher->extern_ctx = extern_ctx;                // Set the external context.
  matcher->priority = 1.0;                         // Set the priority to 1.0.
  return matcher;
}

/**
 * @brief The match function for a matcher that always returns true.
 * @param matcher Unused.
 * @param value Unused.
 * @return Always true.
 */
static inline bool mongory_matcher_always_true_match(mongory_matcher *matcher, mongory_value *value) {
  (void)matcher; // Mark as unused to prevent compiler warnings.
  (void)value;   // Mark as unused.
  return true;   // This matcher always indicates a match.
}

/**
 * @brief Creates a matcher instance that will always evaluate to true.
 * Useful as a placeholder or for default cases.
 * @param pool Memory pool for allocation.
 * @param condition Condition (typically ignored by this matcher).
 * @return A new `mongory_matcher` or NULL on failure.
 */
mongory_matcher *mongory_matcher_always_true_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_base_new(pool, condition, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->match = mongory_matcher_always_true_match;
  matcher->original_match = mongory_matcher_always_true_match;
  matcher->name = mongory_string_cpy(pool, "Always True");
  matcher->explain = mongory_matcher_base_explain;
  // Optionally set original_match as well if it's a strict policy
  // matcher->context.original_match = mongory_matcher_always_true_match;
  return matcher;
}

/**
 * @brief The match function for a matcher that always returns false.
 * @param matcher Unused.
 * @param value Unused.
 * @return Always false.
 */
static inline bool mongory_matcher_always_false_match(mongory_matcher *matcher, mongory_value *value) {
  (void)matcher; // Mark as unused.
  (void)value;   // Mark as unused.
  return false;  // This matcher never indicates a match.
}

/**
 * @brief Creates a matcher instance that will always evaluate to false.
 * Useful for conditions that should never match.
 * @param pool Memory pool for allocation.
 * @param condition Condition (typically ignored by this matcher).
 * @return A new `mongory_matcher` or NULL on failure.
 */
mongory_matcher *mongory_matcher_always_false_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_base_new(pool, condition, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->match = mongory_matcher_always_false_match;
  matcher->original_match = mongory_matcher_always_false_match;
  matcher->name = mongory_string_cpy(pool, "Always False");
  matcher->explain = mongory_matcher_base_explain;
  // matcher->context.original_match = mongory_matcher_always_false_match;
  return matcher;
}
