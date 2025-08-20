/**
 * @file array_record_matcher.c
 * @brief Implements a versatile matcher for arrays, handling various condition
 * types. This is an internal implementation file for the matcher module.
 *
 * This matcher can interpret conditions as:
 * - A table: Parsed for operators like $elemMatch or implicit field conditions
 *   on array elements.
 * - A regex: Creates an $elemMatch to match array elements against the regex.
 * - A literal: Creates an $elemMatch to check for equality with elements.
 * - Another array: Checks for whole-array equality.
 * It often constructs a composite OR matcher to combine these possibilities.
 */
#include "array_record_matcher.h"
#include "../foundations/config_private.h"
#include "../foundations/utils.h"          // For mongory_try_parse_int
#include "base_matcher.h"                  // For mongory_matcher_base_new
#include "compare_matcher.h"               // For mongory_matcher_equal_new
#include "composite_matcher.h"             // For mongory_matcher_composite_new, $elemMatch, table_cond_new, or_match
#include "literal_matcher.h"               // Potentially used by table_cond_new
#include "mongory-core/foundations/table.h"
#include "mongory-core/foundations/value.h"
#include <mongory-core.h> // General include
#include <stdio.h>        // For NULL
#include <string.h>       // For strcmp

/**
 * @struct mongory_matcher_array_record_parse_table_context
 * @brief Context used when parsing a table condition for array record matching.
 *
 * Helps separate parts of the condition table:
 * - `parsed_table`: For explicit operators (e.g., $size, $all if implemented)
 *   or indexed conditions.
 * - `elem_match_table`: For conditions that should apply to individual elements
 *   via an implicit or explicit $elemMatch.
 */
typedef struct mongory_matcher_array_record_parse_table_context {
  mongory_table *parsed_table;     /**< Stores operator or indexed conditions. */
  mongory_table *elem_match_table; /**< Stores conditions for element matching. */
} mongory_matcher_array_record_parse_table_context;

/**
 * @brief Callback to parse a condition table for array matching.
 *
 * Iterates through the main condition table:
 * - If `key` is "$elemMatch" and `value` is a table, its contents are added to
 *   `context->elem_match_table`.
 * - If `key` starts with '$' (another operator) or is numeric (array index),
 *   it's added to `context->parsed_table`.
 * - Otherwise (plain field name), it's considered part of an implicit element
 *   match and added to `context->elem_match_table`.
 *
 * @param key Current key in the condition table.
 * @param value Current value for the key.
 * @param acc Pointer to `mongory_matcher_array_record_parse_table_context`.
 * @return Always true to continue.
 */
static inline bool mongory_matcher_array_record_parse_table_foreach(char *key, mongory_value *value, void *acc) {
  mongory_matcher_array_record_parse_table_context *context = (mongory_matcher_array_record_parse_table_context *)acc;
  mongory_table *parsed_table_for_operators = context->parsed_table;
  mongory_table *table_for_elem_match_conditions = context->elem_match_table;

  if (strcmp(key, "$elemMatch") == 0 && value->type == MONGORY_TYPE_TABLE && value->data.t != NULL) {
    // Explicit $elemMatch: iterate its sub-table and add to elem_match_table
    mongory_table_merge(table_for_elem_match_conditions, value->data.t);
  } else if (*key == '$' || mongory_try_parse_int(key, NULL)) {
    // Operator (like $size) or numeric index: goes to parsed_table
    parsed_table_for_operators->set(parsed_table_for_operators, key, value);
  } else {
    // Regular field name: implies a condition on elements, goes to
    // elem_match_table
    table_for_elem_match_conditions->set(table_for_elem_match_conditions, key, value);
  }
  return true;
}

/**
 * @brief Parses a table `condition` intended for array matching.
 *
 * Separates the condition into parts for direct array operations (like $size)
 * and parts for matching individual elements (implicit or explicit $elemMatch).
 * If any element-matching conditions are found, they are grouped under an
 * "$elemMatch" key in the returned `parsed_table`.
 *
 * @param condition The `mongory_value` (must be a table) to parse.
 * @return A new `mongory_value` (table) containing the parsed and restructured
 * condition. Returns NULL if input is not a table or on allocation failure.
 */
static inline mongory_value *mongory_matcher_array_record_parse_table(mongory_value *condition) {
  if (!condition->data.t || !condition->pool) {
    mongory_error *error = MG_ALLOC_PTR(condition->pool, mongory_error);
    if (!error) {
      condition->pool->error = &MONGORY_ALLOC_ERROR;
      return NULL;
    }
    error->type = MONGORY_ERROR_INVALID_TYPE;
    error->message = "Expected condition to be a table, got a non-table value";
    condition->pool->error = error; // TODO: This is a hack, we should use a better error handling mechanism
    return NULL; // Invalid input
  }
  mongory_memory_pool *pool = condition->pool;
  mongory_table *parsed_table = mongory_table_new(pool);
  mongory_table *elem_match_sub_table = mongory_table_new(pool);

  if (!parsed_table || !elem_match_sub_table) {
    // Cleanup if one allocation succeeded but other failed? Pool should handle.
    return NULL;
  }

  mongory_matcher_array_record_parse_table_context parse_ctx = {parsed_table, elem_match_sub_table};
  mongory_table *original_condition_table = condition->data.t;

  original_condition_table->each(original_condition_table, &parse_ctx,
                                 mongory_matcher_array_record_parse_table_foreach);

  if (elem_match_sub_table->count > 0) {
    // If there were conditions for element matching, add them as an $elemMatch
    // clause to the main parsed_table.
    parsed_table->set(parsed_table, "$elemMatch", mongory_value_wrap_t(pool, elem_match_sub_table));
  }
  // If elem_match_sub_table is empty, it's not added. The original parsed_table
  // (which might contain $size etc.) is returned.
  return mongory_value_wrap_t(pool, parsed_table);
}

/**
 * @brief Main constructor for `mongory_matcher_array_record_new`.
 *
 * Constructs a two-part matcher (often combined with OR):
 * - `left`: Handles element-wise conditions (like $elemMatch from various
 *   condition types).
 * - `right`: Handles whole-array equality if the original `condition` was an
 *   array.
 *
 * If `right` is NULL (i.e., original `condition` was not an array), only the
 * `left` matcher is returned. Otherwise, a composite OR matcher is created
 * with `left` and `right` as children.
 *
 * @param pool Memory pool.
 * @param condition The condition to apply to arrays.
 * @return The constructed array_record_matcher, or NULL on failure.
 */
mongory_matcher *mongory_matcher_array_record_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  if (!condition)
    return NULL;
  switch (condition->type) {
  case MONGORY_TYPE_TABLE:
    return mongory_matcher_table_cond_new(pool,
      mongory_matcher_array_record_parse_table(condition),
      extern_ctx
    );
  case MONGORY_TYPE_ARRAY:
    return mongory_matcher_or_new(pool, MG_ARRAY_WRAP(pool, 2,
      MG_TABLE_WRAP(pool, 1, "$eq", condition),
      MG_TABLE_WRAP(pool, 1, "$elemMatch",
        MG_TABLE_WRAP(pool, 1, "$eq", condition)
      )
    ), extern_ctx);
  case MONGORY_TYPE_REGEX:
    return mongory_matcher_elem_match_new(pool, MG_TABLE_WRAP(pool, 1, "$regex", condition), extern_ctx);
  default: // Literals (string, int, bool, etc.)
    return mongory_matcher_elem_match_new(pool, MG_TABLE_WRAP(pool, 1, "$eq", condition), extern_ctx);
  }
}
