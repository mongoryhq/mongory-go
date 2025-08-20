/**
 * @file compare_matcher.c
 * @brief Implements comparison matchers like `$eq`, `$gt`, `$lt`, etc.
 *
 * This file follows a factory pattern. A generic private constructor,
 * `mongory_matcher_compare_new`, is used to create a base matcher.
 * Each specific comparison operator (e.g., `$eq`, `$gt`) has its own public
 * constructor (e.g., `mongory_matcher_equal_new`) that provides a specialized
 * matching function to the generic constructor.
 */
#include "compare_matcher.h"
#include "base_matcher.h" // For mongory_matcher_base_new
#include <mongory-core.h> // For mongory_value, mongory_matcher types
#include "../foundations/utils.h"

/**
 * @brief Generic constructor for comparison matchers.
 *
 * Initializes a base matcher and sets its `match` function and
 * `original_match` context field to the provided `match_func`.
 *
 * @param pool The memory pool for allocation.
 * @param condition The `mongory_value` to be stored as the comparison target.
 * @param match_func The specific comparison logic function (e.g., for equality,
 * greater than).
 * @return mongory_matcher* A pointer to the newly created comparison matcher,
 * or NULL on failure.
 */
static inline mongory_matcher *mongory_matcher_compare_new(mongory_memory_pool *pool, mongory_value *condition,
                                                           mongory_matcher_match_func match_func, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_base_new(pool, condition, extern_ctx);
  if (matcher == NULL) {
    return NULL; // Base matcher allocation failed.
  }
  matcher->match = match_func;
  matcher->original_match = match_func; // Store original match function
  return matcher;
}

// ============================================================================
// Static Match Functions
//
// These functions contain the actual logic for each comparison operator.
// They all follow the `mongory_matcher_match_func` signature and are passed
// to the generic constructor.
// ============================================================================

/**
 * @brief Match function for equality ($eq).
 *
 * Compares the input `value` with the matcher's `condition` using the
 * polymorphic `comp` function of the value.
 *
 * @param matcher The equality matcher instance.
 * @param value The value to check.
 * @return True if `value` is equal to `matcher->condition`; false otherwise or
 * if the types are not comparable.
 */
static inline bool mongory_matcher_equal_match(mongory_matcher *matcher, mongory_value *value) {
  if (!value || !value->comp || !matcher->condition)
    return false; // Invalid inputs
  int result = value->comp(value, matcher->condition);
  if (result == mongory_value_compare_fail) {
    return false; // Types are not comparable or other comparison error.
  }
  return result == 0; // 0 indicates equality.
}

mongory_matcher *mongory_matcher_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_compare_new(pool, condition, mongory_matcher_equal_match, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->name = mongory_string_cpy(pool, "Eq");
  matcher->priority = 1.0;
  return matcher;
}

/**
 * @brief Match function for inequality ($ne).
 *
 * This is the logical inverse of the `equal_match` function.
 * If the types are not comparable (comparison fails), it is considered "not
 * equal", so this function returns true in that case.
 *
 * @param matcher The inequality matcher instance.
 * @param value The value to check.
 * @return True if `value` is not equal to `matcher->condition` or if they
 * are not comparable.
 */
static inline bool mongory_matcher_not_equal_match(mongory_matcher *matcher, mongory_value *value) {
  if (!value || !value->comp || !matcher->condition)
    return true; // Invalid inputs, treat as "not equal"
  int result = value->comp(value, matcher->condition);
  if (result == mongory_value_compare_fail) {
    return true; // Incomparable types are considered "not equal".
  }
  return result != 0; // Non-zero indicates inequality.
}

mongory_matcher *mongory_matcher_not_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_compare_new(pool, condition, mongory_matcher_not_equal_match, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->name = mongory_string_cpy(pool, "Ne");
  matcher->priority = 1.0;
  return matcher;
}

/**
 * @brief Match function for "greater than" ($gt).
 * @param matcher The $gt matcher instance.
 * @param value The value to check.
 * @return True if `value` is greater than `matcher->condition`, false
 * otherwise or on comparison failure.
 */
static inline bool mongory_matcher_greater_than_match(mongory_matcher *matcher, mongory_value *value) {
  if (!value || !value->comp || !matcher->condition)
    return false;
  int result = value->comp(value, matcher->condition);
  if (result == mongory_value_compare_fail) {
    return false;
  }
  return result == 1; // 1 indicates value > condition.
}

mongory_matcher *mongory_matcher_greater_than_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_compare_new(pool, condition, mongory_matcher_greater_than_match, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->name = mongory_string_cpy(pool, "Gt");
  matcher->priority = 2.0;
  return matcher;
}

/**
 * @brief Match function for "less than" ($lt).
 * @param matcher The $lt matcher instance.
 * @param value The value to check.
 * @return True if `value` is less than `matcher->condition`, false otherwise or
 * on comparison failure.
 */
static inline bool mongory_matcher_less_than_match(mongory_matcher *matcher, mongory_value *value) {
  if (!value || !value->comp || !matcher->condition)
    return false;
  int result = value->comp(value, matcher->condition);
  if (result == mongory_value_compare_fail) {
    return false;
  }
  return result == -1; // -1 indicates value < condition.
}

mongory_matcher *mongory_matcher_less_than_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_compare_new(pool, condition, mongory_matcher_less_than_match, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->name = mongory_string_cpy(pool, "Lt");
  matcher->priority = 2.0;
  return matcher;
}

/**
 * @brief Match function for "greater than or equal" ($gte).
 * @param matcher The $gte matcher instance.
 * @param value The value to check.
 * @return True if `value` is >= `matcher->condition`, false otherwise or on
 * comparison failure.
 */
static inline bool mongory_matcher_greater_than_or_equal_match(mongory_matcher *matcher, mongory_value *value) {
  if (!value || !value->comp || !matcher->condition)
    return false;
  int result = value->comp(value, matcher->condition);
  if (result == mongory_value_compare_fail) {
    return false;
  }
  return result >= 0; // 0 or 1 indicates value >= condition.
}

mongory_matcher *mongory_matcher_greater_than_or_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_compare_new(pool, condition, mongory_matcher_greater_than_or_equal_match, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->name = mongory_string_cpy(pool, "Gte");
  matcher->priority = 2.0;
  return matcher;
}

/**
 * @brief Match function for "less than or equal" ($lte).
 * @param matcher The $lte matcher instance.
 * @param value The value to check.
 * @return True if `value` is <= `matcher->condition`, false otherwise or on
 * comparison failure.
 */
static inline bool mongory_matcher_less_than_or_equal_match(mongory_matcher *matcher, mongory_value *value) {
  if (!value || !value->comp || !matcher->condition)
    return false;
  int result = value->comp(value, matcher->condition);
  if (result == mongory_value_compare_fail) {
    return false;
  }
  return result <= 0; // 0 or -1 indicates value <= condition.
}

mongory_matcher *mongory_matcher_less_than_or_equal_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  mongory_matcher *matcher = mongory_matcher_compare_new(pool, condition, mongory_matcher_less_than_or_equal_match, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->name = mongory_string_cpy(pool, "Lte");
  matcher->priority = 2.0;
  return matcher;
}
