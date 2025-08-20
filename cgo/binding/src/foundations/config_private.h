#ifndef MONGORY_FOUNDATIONS_CONFIG_PRIVATE_H
#define MONGORY_FOUNDATIONS_CONFIG_PRIVATE_H
#include "../matchers/base_matcher.h"
#include "mongory-core/foundations/config.h"
#include "mongory-core/foundations/table.h"
#include "mongory-core/matchers/matcher.h"

typedef struct mongory_regex_adapter {
  mongory_regex_func match_func;
  mongory_regex_stringify_func stringify_func;
} mongory_regex_adapter;

typedef struct mongory_value_converter {
  mongory_deep_convert_func deep_convert;
  mongory_shallow_convert_func shallow_convert;
  mongory_recover_func recover;
} mongory_value_converter;

/**
 * @brief Function pointer type for matching a value against an external matcher.
 *
 * This function is called when a value needs to be matched against an external
 * matcher.
 *
 * @param external_matcher The external reference to the matcher.
 * @param value The value to match against.
 * @return bool True if the value matches the matcher, false otherwise.
 */
typedef struct mongory_matcher_custom_adapter {
  bool (*match)(void *external_matcher, mongory_value *value); // Match a value against an external matcher.
  mongory_matcher_custom_context *(*build)(char *key, mongory_value *condition, void *extern_ctx); // Build an external matcher reference.
  bool (*lookup)(char *key); // Lookup a matcher reference by key.
} mongory_matcher_custom_adapter;

extern mongory_memory_pool *mongory_internal_pool;
extern mongory_regex_adapter mongory_internal_regex_adapter;
extern mongory_table *mongory_matcher_mapping;
extern mongory_value_converter mongory_internal_value_converter;
extern mongory_matcher_custom_adapter mongory_custom_matcher_adapter;
extern bool mongory_matcher_trace_result_colorful;

typedef mongory_matcher *(*mongory_matcher_build_func)(mongory_memory_pool *pool,
                                                       mongory_value *condition,
                                                       void *extern_ctx); // build function
void mongory_matcher_register(char *name, mongory_matcher_build_func build_func);
mongory_matcher_build_func mongory_matcher_build_func_get(char *name);

#endif
