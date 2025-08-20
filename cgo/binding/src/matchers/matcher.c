/**
 * @file matcher.c
 * @brief Implements the generic mongory_matcher constructor.
 *
 * This file provides the implementation for the top-level matcher creation
 * function.
 */
#include <stdio.h>
#include <string.h>
#include "mongory-core/matchers/matcher.h" // Public API
#include "../foundations/utils.h"           // For mongory_string_cpyf
// Required internal headers for delegation
#include "../foundations/config_private.h" // Potentially for global settings
#include "base_matcher.h"                  // For mongory_matcher_base_new if used directly
#include "composite_matcher.h"             // For mongory_matcher_table_cond_new
#include "literal_matcher.h"               // Potentially for other default constructions
#include "mongory-core/foundations/array.h"
#include "../foundations/string_buffer.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include <mongory-core.h> // General include, might not be strictly necessary here


/**
 * @brief Creates a new matcher based on the provided condition.
 *
 * This is the primary public entry point for creating a matcher. The library
 * uses a factory pattern where this function determines the appropriate
 * specific matcher to create based on the structure of the `condition` value.
 *
 * Currently, it always delegates to `mongory_matcher_table_cond_new`,
 * which handles query documents (tables). This is the most common use case,
 * where the condition is a table like `{ "field": { "$op": "value" } }`.
 *
 * @param pool The memory pool to be used for allocating the matcher.
 * @param condition A `mongory_value` defining the matching criteria. This is
 *                  typically a `mongory_table`.
 * @return mongory_matcher* A pointer to the newly constructed matcher, or NULL
 * if allocation fails or the condition is invalid.
 */
mongory_matcher *mongory_matcher_new(mongory_memory_pool *pool, mongory_value *condition, void *extern_ctx) {
  // The core logic is delegated to a more specific constructor.
  // This design allows for easy extension; for example, a different constructor
  // could be chosen here based on the `condition->type`.
  mongory_matcher *matcher = mongory_matcher_table_cond_new(pool, condition, extern_ctx);
  if (matcher == NULL) {
    return NULL;
  }

  return matcher;
}

/**
 * @brief Executes the matching logic for the given matcher.
 *
 * This function is a polymorphic wrapper. It invokes the `match` function
 * pointer on the specific `mongory_matcher` instance, which will be one of
 * the internal matching functions (e.g., from a compare_matcher or
 * composite_matcher).
 *
 * @param matcher The matcher to use.
 * @param value The value to check against the matcher's condition.
 * @return True if the value satisfies the matcher's condition, false otherwise.
 */
bool mongory_matcher_match(mongory_matcher *matcher, mongory_value *value) { return matcher->match(matcher, value); }

static bool mongory_matcher_explain_cb(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  matcher->explain(matcher, ctx);
  return true;
}

/**
 * @brief Generates a human-readable explanation of the matcher's criteria.
 *
 * This function is a polymorphic wrapper around the `explain` function pointer,
 * allowing different matcher types to provide their own specific explanations.
 *
 * @param matcher The matcher to explain.
 * @param temp_pool A temporary memory pool for allocating the explanation string(s).
 */
void mongory_matcher_explain(mongory_matcher *matcher, mongory_memory_pool *temp_pool) {
  mongory_matcher_traverse_context ctx = {
      .pool = temp_pool,
      .count = 0,
      .total = 0,
      .acc = "",
      .callback = mongory_matcher_explain_cb,
  };
  matcher->traverse(matcher, &ctx);
}

typedef struct mongory_matcher_traced_match_context {
  char *message;
  int level;
} mongory_matcher_traced_match_context;

static bool mongory_matcher_traced_match(mongory_matcher *matcher, mongory_value *value) {
  bool matched = matcher->original_match(matcher, value);
  mongory_memory_pool *pool = matcher->trace_stack->pool;
  char *res = NULL;
  if (mongory_matcher_trace_result_colorful) {
    res = matched ? "\e[30;42mMatched\e[0m" : "\e[30;41mDismatch\e[0m"; // Green for matched, red for mismatch.
  } else {
    res = matched ? "Matched" : "Dismatch";
  }
  char *cdtn = matcher->condition->to_str(matcher->condition, pool);
  char *rcd = value == NULL ? "Nothing" : value->to_str(value, pool);
  char *name = matcher->name;
  char *message;

  if (strcmp(name, "Field") == 0) {
    mongory_field_matcher *field_matcher = (mongory_field_matcher *)matcher;
    char *fd = field_matcher->field;
    message = mongory_string_cpyf(pool, "%s: %s, field: \"%s\", condition: %s, record: %s\n", name, res, fd, cdtn, rcd);
  } else {
    message = mongory_string_cpyf(pool, "%s: %s, condition: %s, record: %s\n", name, res, cdtn, rcd);
  }

  mongory_matcher_traced_match_context *trace_result = MG_ALLOC_PTR(pool, mongory_matcher_traced_match_context);
  trace_result->message = message;
  trace_result->level = matcher->trace_level;
  matcher->trace_stack->push(matcher->trace_stack, mongory_value_wrap_ptr(pool, (void *)trace_result));

  return matched;
}

static bool mongory_matcher_enable_trace_cb(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  matcher->trace_stack = (mongory_array *)ctx->acc;
  matcher->trace_level = ctx->level;
  matcher->match = mongory_matcher_traced_match;
  return true;
}

static bool mongory_matcher_disable_trace_cb(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  (void)ctx;
  matcher->match = matcher->original_match;
  matcher->trace_stack = NULL;
  return true;
}

static mongory_array *mongory_matcher_traces_sort(mongory_array *self, int level) {
  mongory_array *sorted_array = mongory_array_new(self->pool);
  mongory_array *group = mongory_array_new(self->pool);
  int total = (int)self->count;
  for (int i = 0; i < total; i++) {
    mongory_value *item = self->get(self, i);
    mongory_matcher_traced_match_context *trace = (mongory_matcher_traced_match_context *)item->data.ptr;
    if (trace->level == level) {
      sorted_array->push(sorted_array, item);
      mongory_array *sorted_group = mongory_matcher_traces_sort(group, level + 1);
      int sorted_group_total = (int)sorted_group->count;
      for (int j = 0; j < sorted_group_total; j++) {
        sorted_array->push(sorted_array, sorted_group->get(sorted_group, j));
      }
      group = mongory_array_new(self->pool);
    } else {
      group->push(group, item);
    }
  }
  return sorted_array;
}

void mongory_matcher_enable_trace(mongory_matcher *matcher, mongory_memory_pool *temp_pool) {
  mongory_array *trace_stack = mongory_array_new(temp_pool);
  mongory_matcher_traverse_context ctx = {
      .pool = temp_pool,
      .level = 0,
      .count = 0,
      .total = 0,
      .acc = (void *)trace_stack,
      .callback = mongory_matcher_enable_trace_cb,
  };
  matcher->traverse(matcher, &ctx);
}

void mongory_matcher_disable_trace(mongory_matcher *matcher) {
  mongory_matcher_traverse_context ctx = {
      .level = 0,
      .count = 0,
      .total = 0,
      .callback = mongory_matcher_disable_trace_cb,
  };
  matcher->traverse(matcher, &ctx);
}

void mongory_matcher_print_trace(mongory_matcher *matcher) {
  if (matcher->trace_stack == NULL)
    return;
  mongory_array *sorted_trace_stack = mongory_matcher_traces_sort(matcher->trace_stack, 0);
  int total = (int)sorted_trace_stack->count;
  for (int i = 0; i < total; i++) {
    mongory_value *item = sorted_trace_stack->get(sorted_trace_stack, i);
    mongory_matcher_traced_match_context *trace = (mongory_matcher_traced_match_context *)item->data.ptr;
    int indent_size = trace->level * 2;
    char *indent = MG_ALLOC(sorted_trace_stack->pool, indent_size + 1);
    memset(indent, ' ', indent_size);
    indent[indent_size] = '\0';
    printf("%s%s", indent, trace->message);
  }
}

bool mongory_matcher_trace(mongory_matcher *matcher, mongory_value *value) {
  mongory_matcher_enable_trace(matcher, value->pool);
  bool matched = matcher->match(matcher, value);
  mongory_matcher_print_trace(matcher);
  mongory_matcher_disable_trace(matcher);
  return matched;
}
