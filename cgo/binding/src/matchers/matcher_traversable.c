#ifndef MONGORY_MATCHER_TRAVERSABLE_C
#define MONGORY_MATCHER_TRAVERSABLE_C

#include "matcher_traversable.h"
#include "base_matcher.h"
#include "composite_matcher.h"
#include "literal_matcher.h"

bool mongory_matcher_leaf_traverse(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  bool continue_traverse = ctx->callback(matcher, ctx);
  if (!continue_traverse)
    return false;
  ctx->count++;
  return true;
}

bool mongory_matcher_composite_traverse(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  void *prev_acc = ctx->acc;
  if (!mongory_matcher_leaf_traverse(matcher, ctx))
    return false;
  mongory_composite_matcher *composite = (mongory_composite_matcher *)matcher;
  mongory_array *children = composite->children;
  int total = (int)children->count;
  mongory_matcher_traverse_context child_ctx = {
      .pool = ctx->pool,
      .level = ctx->level + 1,
      .count = 0,
      .total = total,
      .acc = ctx->acc,
      .callback = ctx->callback,
  };

  for (int i = 0; i < total; i++) {
    mongory_matcher *child = (mongory_matcher *)children->get(children, i);
    if (!child->traverse(child, &child_ctx))
      return false;
  }
  ctx->acc = prev_acc;
  return true;
}

bool mongory_matcher_literal_traverse(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx) {
  void *prev_acc = ctx->acc;
  if (!mongory_matcher_leaf_traverse(matcher, ctx))
    return false;
  mongory_literal_matcher *literal = (mongory_literal_matcher *)matcher;
  mongory_matcher *next_matcher = literal->array_record_matcher ? literal->array_record_matcher : literal->delegate_matcher;
  mongory_matcher_traverse_context child_ctx = {
      .pool = ctx->pool,
      .level = ctx->level + 1,
      .count = 0,
      .total = 1,
      .acc = ctx->acc,
      .callback = ctx->callback,
  };
  bool result = next_matcher->traverse(next_matcher, &child_ctx);
  ctx->acc = prev_acc;
  return result;
}
#endif /* MONGORY_MATCHER_TRAVERSABLE_C */
