/**
 * @file existance_matcher.c
 * @brief Implements $exists and $present matchers.
 * This is an internal implementation file for the matcher module.
 */
#include "existance_matcher.h"
#include "base_matcher.h"                   // For mongory_matcher_base_new
#include "mongory-core/foundations/array.h" // For value->data.a->count
#include "mongory-core/foundations/error.h" // For mongory_error types
#include "mongory-core/foundations/table.h" // For value->data.t->count
#include "mongory-core/foundations/value.h" // For mongory_value types
#include <mongory-core.h>                   // General include
#include "../foundations/utils.h"

/**
 * @brief Validates that the condition for an existence/presence matcher is a
 * boolean.
 * @param condition The `mongory_value` condition to validate.
 * @return True if `condition` is not NULL and is of type `MONGORY_TYPE_BOOL`,
 * false otherwise.
 */
static bool mongory_matcher_validate_bool_condition(mongory_value *condition) {
  if (condition == NULL) {
    return false; // Condition itself must exist.
  }
  if (condition->type != MONGORY_TYPE_BOOL) {
    return false; // Condition must be a boolean value.
  }
  return true;
}

/**
 * @brief Match function for the $exists matcher.
 *
 * Compares the existence of the input `value` (i.e., `value != NULL`)
 * with the boolean `matcher->condition->data.b`.
 * - If `condition` is true, matches if `value` is not NULL.
 * - If `condition` is false, matches if `value` is NULL.
 *
 * @param matcher The $exists matcher instance. Its condition must be boolean.
 * @param value The value whose existence is being checked. This is typically
 *              the result of a field lookup.
 * @return True if the existence matches the condition, false otherwise.
 */
static inline bool mongory_matcher_exists_match(mongory_matcher *matcher, mongory_value *value) {
  // The condition for $exists is a boolean (e.g., field: {$exists: true})
  bool condition_expects_existence = matcher->condition->data.b;
  bool actual_value_exists = (value != NULL);

  return condition_expects_existence == actual_value_exists;
}

mongory_matcher *mongory_matcher_exists_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  if (!mongory_matcher_validate_bool_condition(condition)) {
    pool->error = MG_ALLOC_PTR(pool, mongory_error);
    if (pool->error) {
      pool->error->type = MONGORY_ERROR_INVALID_ARGUMENT;
      pool->error->message = "$exists condition must be a boolean value.";
    }
    return NULL;
  }
  mongory_matcher *matcher = mongory_matcher_base_new(pool, condition, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->match = mongory_matcher_exists_match;
  matcher->original_match = mongory_matcher_exists_match;
  matcher->name = mongory_string_cpy(pool, "Exists");
  matcher->priority = 2.0;
  return matcher;
}

/**
 * @brief Match function for the $present matcher.
 *
 * Determines if a `value` is "present" based on its type and content,
 * and compares this with the boolean `matcher->condition->data.b`.
 * - NULL value: Present if condition is false.
 * - Array: Present if non-empty and condition is true.
 * - Table: Present if non-empty and condition is true.
 * - String: Present if non-NULL, non-empty, and condition is true.
 * - MONGORY_TYPE_NULL: Not present (matches if condition is false).
 * - Boolean: Present if its value matches the condition.
 * - Other non-NULL types: Considered present if condition is true.
 *
 * @param matcher The $present matcher. Its condition must be boolean.
 * @param value The value to check for presence.
 * @return True if the presence status matches the condition, false otherwise.
 */
static inline bool mongory_matcher_present_match(mongory_matcher *matcher, mongory_value *value) {
  bool condition_expects_presence = matcher->condition->data.b;

  if (value == NULL) {
    // A NULL value (field does not exist) is "present" if condition_expects_presence is false.
    return !condition_expects_presence;
  }

  bool actual_value_is_present;
  switch (value->type) {
  case MONGORY_TYPE_ARRAY:
    actual_value_is_present = (value->data.a != NULL && value->data.a->count > 0);
    break;
  case MONGORY_TYPE_TABLE:
    actual_value_is_present = (value->data.t != NULL && value->data.t->count > 0);
    break;
  case MONGORY_TYPE_STRING:
    actual_value_is_present = (value->data.s != NULL && *(value->data.s) != '\0');
    break;
  case MONGORY_TYPE_NULL:
    actual_value_is_present = false; // An explicit BSON-style Null is not "present".
    break;
  case MONGORY_TYPE_BOOL:
    // For a boolean value, "present" means its own value matches the condition.
    // e.g. {$present: true} on a true boolean is true.
    //      {$present: false} on a true boolean is false.
    //      {$present: true} on a false boolean is false.
    //      {$present: false} on a false boolean is true.
    return value->data.b == condition_expects_presence;
  default:
    // For other types (int, double, pointer, regex, unsupported), if they are
    // not NULL (checked above), they are considered "present".
    actual_value_is_present = true;
    break;
  }

  return condition_expects_presence == actual_value_is_present;
}

mongory_matcher *mongory_matcher_present_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  if (!mongory_matcher_validate_bool_condition(condition)) {
    pool->error = MG_ALLOC_PTR(pool, mongory_error);
    if (pool->error) {
      pool->error->type = MONGORY_ERROR_INVALID_ARGUMENT;
      pool->error->message = "$present condition must be a boolean value.";
    }
    return NULL;
  }
  mongory_matcher *matcher = mongory_matcher_base_new(pool, condition, extern_ctx);
  if (!matcher) {
    return NULL;
  }
  matcher->match = mongory_matcher_present_match;
  matcher->original_match = mongory_matcher_present_match;
  matcher->name = mongory_string_cpy(pool, "Present");
  matcher->priority = 2.0;
  return matcher;
}
