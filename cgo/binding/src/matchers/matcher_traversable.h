#ifndef MONGORY_MATCHER_TRAVERSABLE_H
#define MONGORY_MATCHER_TRAVERSABLE_H

#include "mongory-core/matchers/matcher.h"

typedef struct mongory_matcher_traverse_context mongory_matcher_traverse_context;
typedef bool (*mongory_matcher_traverse_func)(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);
struct mongory_matcher_traverse_context {
  mongory_memory_pool *pool;
  int level;
  int count;
  int total;
  void *acc;
  mongory_matcher_traverse_func callback;
} ;

bool mongory_matcher_leaf_traverse(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);

bool mongory_matcher_composite_traverse(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);

bool mongory_matcher_literal_traverse(mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);

#endif /* MONGORY_MATCHER_TRAVERSABLE_H */