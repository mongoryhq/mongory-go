/**
 * @file config.c
 * @brief Implements global configuration and initialization for the Mongory
 * library.
 *
 * This file manages the lifecycle of global resources such as the internal
 * memory pool, regex adapter, matcher registration table, and value converters.
 * It provides the `mongory_init` and `mongory_cleanup` functions, as well as
 * setters for customizable components and a string copy utility.
 */
#include "mongory-core/foundations/config.h"
#include "../matchers/base_matcher.h"      // For mongory_matcher_build_func
#include "../matchers/compare_matcher.h"   // For specific matcher constructors
#include "../matchers/composite_matcher.h" // For specific matcher constructors
#include "../matchers/existance_matcher.h" // For specific matcher constructors
#include "../matchers/inclusion_matcher.h" // For specific matcher constructors
#include "../matchers/literal_matcher.h"   // For specific matcher constructors
#include "../matchers/external_matcher.h"     // For specific matcher constructors
#include "config_private.h"                // For mongory_regex_adapter, mongory_value_converter, etc.
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/table.h"
#include "mongory-core/foundations/value.h"

static bool mongory_regex_default_func(mongory_memory_pool *pool, mongory_value *pattern, mongory_value *value);
static char *mongory_regex_default_stringify_func(mongory_memory_pool *pool, mongory_value *pattern);

// Global internal memory pool for the library.
mongory_memory_pool *mongory_internal_pool = NULL;
// Global adapter for regex operations.
mongory_regex_adapter mongory_internal_regex_adapter = {
  .match_func = mongory_regex_default_func,
  .stringify_func = mongory_regex_default_stringify_func,
};
// Global table mapping matcher names (e.g., "$eq") to their build functions.
mongory_table *mongory_matcher_mapping = NULL;
// Global converter for handling external data types.
mongory_value_converter mongory_internal_value_converter = {
  .deep_convert = NULL,
  .shallow_convert = NULL,
  .recover = NULL,
};
// Global adapter for custom matchers.
mongory_matcher_custom_adapter mongory_custom_matcher_adapter = {
  .build = NULL,
  .lookup = NULL,
  .match = NULL,
};

bool mongory_matcher_trace_result_colorful = true;

/**
 * @brief Initializes the internal memory pool if it hasn't been already.
 * This pool is used for allocations by various library components.
 */
static inline void mongory_internal_pool_init() {
  if (mongory_internal_pool != NULL) {
    return; // Already initialized.
  }
  mongory_internal_pool = mongory_memory_pool_new();
  // TODO: Add error handling if mongory_memory_pool_new returns NULL.
}

/**
 * @brief Default regex function that always returns false.
 * Used if no custom regex function is set via mongory_regex_func_set.
 * @param pool Unused.
 * @param pattern Unused.
 * @param value Unused.
 * @return Always false.
 */
static bool mongory_regex_default_func(mongory_memory_pool *pool, mongory_value *pattern, mongory_value *value) {
  (void)pool;    // Mark as unused to prevent compiler warnings.
  (void)pattern; // Mark as unused.
  (void)value;   // Mark as unused.
  return false;  // Default behavior is no match.
}

static char *mongory_regex_default_stringify_func(mongory_memory_pool *pool, mongory_value *pattern) {
  (void)pool;    // Mark as unused to prevent compiler warnings.
  (void)pattern; // Mark as unused.
  return "//";   // Default behavior is no stringification.
}

/**
 * @brief Sets the global regex matching function.
 * Initializes the regex adapter if it's not already.
 * @param func The custom regex function to use.
 */
void mongory_regex_func_set(mongory_regex_func func) {
  mongory_internal_regex_adapter.match_func = func;
}

/**
 * @brief Sets the global regex stringify function.
 * Initializes the regex adapter if it's not already.
 * @param func The custom regex stringify function to use.
 */
void mongory_regex_stringify_func_set(mongory_regex_stringify_func func) {
  mongory_internal_regex_adapter.stringify_func = func;
}
/**
 * @brief Initializes the matcher mapping table if it hasn't been already.
 * This table stores registrations of matcher names to their constructor
 * functions.
 */
static inline void mongory_matcher_mapping_init() {
  if (mongory_matcher_mapping != NULL) {
    return; // Already initialized.
  }

  // Ensure the internal pool is initialized first.
  mongory_internal_pool_init();
  if (mongory_internal_pool == NULL) {
    return; // Cannot proceed if pool initialization failed.
  }

  mongory_matcher_mapping = mongory_table_new(mongory_internal_pool);
  if (mongory_matcher_mapping == NULL) {
    // TODO: Set error state.
    return;
  }
}

/**
 * @brief Registers a matcher build function with a given name.
 * The build function is stored in the global matcher mapping table.
 * @param name The name of the matcher (e.g., "$eq", "$in").
 * @param build_func A function pointer to the matcher's constructor.
 */
void mongory_matcher_register(char *name, mongory_matcher_build_func build_func) {
  // Ensure mapping is initialized.
  if (mongory_matcher_mapping == NULL) {
    mongory_matcher_mapping_init();
    if (mongory_matcher_mapping == NULL) {
      // TODO: Set error: Cannot register if mapping init failed.
      return;
    }
  }

  // Wrap the function pointer in a mongory_value to store in the table.
  mongory_value *value = mongory_value_wrap_ptr(mongory_internal_pool, build_func);
  if (value == NULL) {
    // TODO: Set error: Failed to wrap pointer.
    return;
  }

  mongory_matcher_mapping->set(mongory_matcher_mapping, name, value);
  // TODO: Check return value of set?
}

/**
 * @brief Retrieves a matcher build function by its name from the global
 * mapping.
 * @param name The name of the matcher to retrieve.
 * @return mongory_matcher_build_func A function pointer to the matcher's
 * constructor, or NULL if not found.
 */
mongory_matcher_build_func mongory_matcher_build_func_get(char *name) {
  if (mongory_matcher_mapping == NULL) {
    return NULL; // Mapping not initialized or init failed.
  }

  mongory_value *value = mongory_matcher_mapping->get(mongory_matcher_mapping, name);
  if (value == NULL || value->type != MONGORY_TYPE_POINTER) {
    return NULL; // Not found or not a pointer type as expected.
  }

  return (mongory_matcher_build_func)value->data.ptr;
}

/**
 * @brief Sets the function for deep conversion of external values.
 * Initializes the value converter if necessary.
 * @param deep_convert The deep conversion function.
 */
void mongory_value_converter_deep_convert_set(mongory_deep_convert_func deep_convert) {
  mongory_internal_value_converter.deep_convert = deep_convert;
}

/**
 * @brief Sets the function for shallow conversion of external values.
 * Initializes the value converter if necessary.
 * @param shallow_convert The shallow conversion function.
 */
void mongory_value_converter_shallow_convert_set(mongory_shallow_convert_func shallow_convert) {
  mongory_internal_value_converter.shallow_convert = shallow_convert;
}

/**
 * @brief Sets the function for recovering external values from mongory_value.
 * Initializes the value converter if necessary.
 * @param recover The recovery function.
 */
void mongory_value_converter_recover_set(mongory_recover_func recover) {
  mongory_internal_value_converter.recover = recover;
}


void mongory_custom_matcher_match_func_set(bool (*match)(void *external_matcher, mongory_value *value)) {
  mongory_custom_matcher_adapter.match = match;
}

void mongory_custom_matcher_build_func_set(mongory_matcher_custom_context *(*build)(char *key, mongory_value *condition, void *extern_ctx)) {
  mongory_custom_matcher_adapter.build = build;
}

void mongory_custom_matcher_lookup_func_set(bool (*lookup)(char *key)) {
  mongory_custom_matcher_adapter.lookup = lookup;
}

void mongory_matcher_trace_result_colorful_set(bool colorful) {
  mongory_matcher_trace_result_colorful = colorful;
}

/**
 * @brief Initializes all core Mongory library components.
 * This includes the internal memory pool, regex adapter, matcher mapping table,
 * and value converter. It then registers all standard matcher types.
 * This function MUST be called before using most other library features.
 */
void mongory_init() {
  // Initialize all global components. Order can be important if one init
  // depends on another (e.g., most depend on the pool).
  mongory_internal_pool_init();
  mongory_matcher_mapping_init();

  // Register all standard matchers.
  // Note: These registrations rely on mongory_internal_pool and
  // mongory_matcher_mapping being successfully initialized.
  // TODO: Add checks to ensure initializations were successful before
  // proceeding.
  mongory_matcher_register("$in", mongory_matcher_in_new);
  mongory_matcher_register("$nin", mongory_matcher_not_in_new);
  mongory_matcher_register("$eq", mongory_matcher_equal_new);
  mongory_matcher_register("$ne", mongory_matcher_not_equal_new);
  mongory_matcher_register("$gt", mongory_matcher_greater_than_new);
  mongory_matcher_register("$gte", mongory_matcher_greater_than_or_equal_new);
  mongory_matcher_register("$lt", mongory_matcher_less_than_new);
  mongory_matcher_register("$lte", mongory_matcher_less_than_or_equal_new);
  mongory_matcher_register("$exists", mongory_matcher_exists_new);
  mongory_matcher_register("$present", mongory_matcher_present_new);
  mongory_matcher_register("$regex", mongory_matcher_regex_new);
  mongory_matcher_register("$and", mongory_matcher_and_new);
  mongory_matcher_register("$or", mongory_matcher_or_new);
  mongory_matcher_register("$elemMatch", mongory_matcher_elem_match_new);
  mongory_matcher_register("$every", mongory_matcher_every_new);
  mongory_matcher_register("$not", mongory_matcher_not_new);
  mongory_matcher_register("$size", mongory_matcher_size_new);
}

/**
 * @brief Cleans up all resources allocated by the Mongory library.
 * This primarily involves freeing the internal memory pool, which should, in
 * turn, release all memory allocated from it. It also resets global pointers
 * to NULL. This should be called when the library is no longer needed to
 * prevent memory leaks.
 */
void mongory_cleanup() {
  if (mongory_internal_pool != NULL) {
    // Freeing the pool should handle all allocations made from it,
    // including the regex_adapter, matcher_mapping table (and its contents if
    // they were allocated from this pool), and value_converter.
    mongory_internal_pool->free(mongory_internal_pool);
    mongory_internal_pool = NULL;
  }

  // Set other global pointers to NULL to indicate they are no longer valid.
  // The memory they pointed to should have been managed by the internal pool.
  mongory_matcher_mapping = NULL; // The table itself and its nodes.
}
