// This file aggregates all mongory-core C sources (synced into cgo/binding)
// into a single compilation unit so that cgo can compile automatically.

#include "binding/src/foundations/array.c"
#include "binding/src/foundations/config.c"
#include "binding/src/foundations/error.c"
#include "binding/src/foundations/memory_pool.c"
#include "binding/src/foundations/string_buffer.c"
#include "binding/src/foundations/table.c"
#include "binding/src/foundations/utils.c"
#include "binding/src/foundations/value.c"

#include "binding/src/matchers/array_record_matcher.c"
#include "binding/src/matchers/base_matcher.c"
#include "binding/src/matchers/compare_matcher.c"
#include "binding/src/matchers/composite_matcher.c"
#include "binding/src/matchers/existance_matcher.c"
#include "binding/src/matchers/external_matcher.c"
#include "binding/src/matchers/inclusion_matcher.c"
#include "binding/src/matchers/literal_matcher.c"
#include "binding/src/matchers/matcher_explainable.c"
#include "binding/src/matchers/matcher_traversable.c"
#include "binding/src/matchers/matcher.c"
