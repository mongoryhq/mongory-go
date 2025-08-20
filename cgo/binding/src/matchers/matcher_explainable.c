#ifndef MONGORY_MATCHER_EXPLAINABLE_C
#define MONGORY_MATCHER_EXPLAINABLE_C

#include "matcher_explainable.h"
#include "../foundations/string_buffer.h"
#include "base_matcher.h"
#include "composite_matcher.h"
#include "literal_matcher.h"
#include "../foundations/utils.h"
#include <stdio.h>

char *mongory_matcher_title(mongory_matcher *matcher, mongory_memory_pool *pool) {
  mongory_value *condition = matcher->condition;
  return mongory_string_cpyf(pool, "%s: %s",
    matcher->name,
    condition->to_str(condition, pool)
  );
}

char *mongory_matcher_title_with_field(mongory_matcher *matcher, mongory_memory_pool *pool) {
  mongory_field_matcher *field_matcher = (mongory_field_matcher *)matcher;
  mongory_value *condition = matcher->condition;
  return mongory_string_cpyf(pool, "Field: \"%s\", to match: %s",
    field_matcher->field,
    condition->to_str(condition, pool)
  );
}

static inline char *mongory_matcher_tail_connection(int count, int total) {
  if (total == 0) {
    return "";
  }
  if (total - count == 1) {
    return "└─ ";
  }
  return "├─ ";
}

static inline char *mongory_matcher_indent_connection(int count, int total) {
  if (total == 0) {
    return "";
  }
  if (total - count == 1) {
    return "   ";
  }
  return "│  ";
}

bool mongory_matcher_base_explain(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  char *connection = mongory_matcher_tail_connection(ctx->count, ctx->total);
  char *title = mongory_matcher_title(matcher, ctx->pool);
  char *prefix = (char *)ctx->acc;
  printf("%s%s%s\n", prefix, connection, title);
  return true;
}

bool mongory_matcher_composite_explain(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  mongory_matcher_base_explain(matcher, ctx);
  char *prefix = (char *)ctx->acc;
  char *indent = mongory_matcher_indent_connection(ctx->count, ctx->total);
  ctx->acc = mongory_string_cpyf(ctx->pool, "%s%s", prefix, indent);
  return true;
}

bool mongory_matcher_literal_explain(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  return mongory_matcher_composite_explain(matcher, ctx);
}

bool mongory_matcher_field_explain(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  char *tail_connection = mongory_matcher_tail_connection(ctx->count, ctx->total);
  char *title = mongory_matcher_title_with_field(matcher, ctx->pool);
  char *prefix = (char *)ctx->acc;
  printf("%s%s%s\n", prefix, tail_connection, title);
  char *indent = mongory_matcher_indent_connection(ctx->count, ctx->total);
  ctx->acc = mongory_string_cpyf(ctx->pool, "%s%s", prefix, indent);
  return true;
}

#endif /* MONGORY_MATCHER_EXPLAINABLE_C */
