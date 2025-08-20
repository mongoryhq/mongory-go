#ifndef MONGORY_MATCHER_EXPLAINABLE_H
#define MONGORY_MATCHER_EXPLAINABLE_H

#include "mongory-core/matchers/matcher.h"
#include "matcher_traversable.h"
#include <mongory-core.h>


/**
 * @brief Explains a matcher.
 * @param matcher The matcher to explain.
 * @param ctx The context to use for the explanation.
 */

bool mongory_matcher_base_explain(struct mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);

bool mongory_matcher_composite_explain(struct mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);

bool mongory_matcher_field_explain(struct mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);

bool mongory_matcher_literal_explain(struct mongory_matcher *matcher, mongory_matcher_traverse_context *ctx);

#endif /* MONGORY_MATCHER_EXPLAINABLE_H */
