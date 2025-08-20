/**
 * @file inclusion_matcher.c
 * @brief Implements $in and $nin matchers for checking value inclusion in an
 * array. This is an internal implementation file for the matcher module.
 */
#include "inclusion_matcher.h"
#include "base_matcher.h"                   // For mongory_matcher_base_new
#include "../foundations/array_private.h"   // For mongory_array_includes
#include "mongory-core/foundations/array.h" // For mongory_array operations
#include "mongory-core/foundations/error.h" // For mongory_error
#include "mongory-core/foundations/value.h" // For mongory_value
#include <mongory-core.h>                   // General include
#include "../foundations/utils.h"            // For mongory_string_cpy

/**
 * @brief Validates that the condition for an inclusion matcher is a valid
 * array.
 * @param condition The `mongory_value` condition to validate.
 * @return True if `condition` is not NULL, is of type `MONGORY_TYPE_ARRAY`,
 * and its internal array data `condition->data.a` is not NULL. False otherwise.
 */
static inline bool mongory_matcher_validate_array_condition(mongory_value *condition) {
  if (condition == NULL) {
    return false; // Condition must exist.
  }
  if (condition->type != MONGORY_TYPE_ARRAY) {
    return false; // Condition must be an array.
  }
  if (condition->data.a == NULL) {
    return false; // The array data within the condition must be valid.
  }
  return true;
}

/**
 * @brief Match function for the $in matcher.
 *
 * If `value_to_check` is a scalar, it checks if `value_to_check` is present in
 * `matcher->condition` (which must be an array).
 * If `value_to_check` is an array, it checks if any element of `value_to_check`
 * is present in `matcher->condition` (array intersection).
 *
 * @param matcher The $in matcher instance.
 * @param value_to_check The value to check for inclusion.
 * @return True if `value_to_check` (or one of its elements) is found in the
 * condition array, false otherwise.
 */
static inline bool mongory_matcher_in_match(mongory_matcher *matcher, mongory_value *value_to_check) {
  if (!value_to_check || !matcher->condition || !matcher->condition->data.a) {
    // Invalid inputs or condition is not a proper array.
    return false;
  }

  mongory_array *condition_array = matcher->condition->data.a;

  if (value_to_check->type != MONGORY_TYPE_ARRAY) {
    return mongory_array_includes(condition_array, value_to_check);
  }

  mongory_array *input_array = value_to_check->data.a;
  if (!input_array) {
    return false;
  }

  for (size_t i = 0; i < input_array->count; i++) {
    mongory_value *input_item = input_array->get(input_array, i);
    if (mongory_array_includes(condition_array, input_item)) {
      return true;
    }
  }

  return false;
}

mongory_matcher *mongory_matcher_in_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  if (!mongory_matcher_validate_array_condition(condition)) {
    pool->error = MG_ALLOC_PTR(pool, mongory_error);
    if (pool->error) {
      pool->error->type = MONGORY_ERROR_INVALID_ARGUMENT;
      pool->error->message = "$in condition must be a valid array.";
    }
    return NULL;
  }
  mongory_matcher *matcher = mongory_matcher_base_new(pool, condition, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->match = mongory_matcher_in_match;
  matcher->original_match = mongory_matcher_in_match;
  matcher->name = mongory_string_cpy(pool, "In");
  matcher->priority = 1.0 + mongory_log((double)condition->data.a->count + 1.0, 1.5);
  return matcher;
}

/**
 * @brief Match function for the $nin (not in) matcher.
 * Simply negates the result of the $in logic.
 * @param matcher The $nin matcher instance.
 * @param value The value to check.
 * @return True if the value is NOT found according to $in logic, false
 * otherwise.
 */
static inline bool mongory_matcher_not_in_match(mongory_matcher *matcher, mongory_value *value) {
  // $nin is true if $in is false.
  return !mongory_matcher_in_match(matcher, value);
}

mongory_matcher *mongory_matcher_not_in_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  if (!mongory_matcher_validate_array_condition(condition)) {
    pool->error = MG_ALLOC_PTR(pool, mongory_error);
    if (pool->error) {
      pool->error->type = MONGORY_ERROR_INVALID_ARGUMENT;
      pool->error->message = "$nin condition must be a valid array.";
    }
    return NULL;
  }
  mongory_matcher *matcher = mongory_matcher_base_new(pool, condition, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->match = mongory_matcher_not_in_match;
  matcher->original_match = mongory_matcher_not_in_match;
  matcher->name = mongory_string_cpy(pool, "Nin");
  matcher->priority = 1.0 + mongory_log((double)condition->data.a->count + 1.0, 1.5);
  return matcher;
}
