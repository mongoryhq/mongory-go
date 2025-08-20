#ifndef MONGORY_FOUNDATIONS_CONFIG_H
#define MONGORY_FOUNDATIONS_CONFIG_H

/**
 * @file config.h
 * @brief Defines global configuration functions and types for the Mongory
 * library.
 *
 * This includes initialization and cleanup routines, as well as functions
 * for setting up custom behaviors like regex matching and value conversion.
 * It also provides a string copying utility that uses the library's memory
 * pool.
 */

#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include <stdbool.h>

/**
 * @brief Function pointer type for custom regex matching.
 *
 * This function is called when a regex match is required.
 *
 * @param pool The memory pool to use for any allocations during the match.
 * @param pattern The regex pattern (as a mongory_value, typically a string or
 * regex type).
 * @param value The value to match against (as a mongory_value, typically a
 * string).
 * @return bool True if the value matches the pattern, false otherwise.
 */
typedef bool (*mongory_regex_func)(mongory_memory_pool *pool, mongory_value *pattern, mongory_value *value);

/**
 * @brief Function pointer type for stringifying a regex pattern.
 *
 * This function is called when a regex pattern needs to be stringified.
 *
 * @param pool The memory pool to use for any allocations during the
 * stringification.
 * @param pattern The regex pattern (as a mongory_value, typically a string or
 * regex type).
 * @return char* A stringified representation of the regex pattern, or NULL on failure.
 */
typedef char* (*mongory_regex_stringify_func)(mongory_memory_pool *pool, mongory_value *pattern);

/**
 * @brief Function pointer type for deep conversion of an external value to a
 * mongory_value.
 *
 * "Deep" implies that if the external value is a complex type (e.g., a struct
 * representing a document or array), its contents should also be converted.
 *
 * @param pool The memory pool to use for allocating the new mongory_value and
 * its data.
 * @param value A pointer to the external value to convert.
 * @return mongory_value* A new mongory_value representing the converted data,
 * or NULL on failure.
 */
typedef mongory_value *(*mongory_deep_convert_func)(mongory_memory_pool *pool, void *value);

/**
 * @brief Function pointer type for shallow conversion of an external value to a
 * mongory_value.
 *
 * "Shallow" implies that if the external value is complex, only a wrapper or
 * pointer might be stored in the mongory_value, without deeply converting its
 * contents.
 *
 * @param pool The memory pool to use for allocating the new mongory_value.
 * @param value A pointer to the external value to convert.
 * @return mongory_value* A new mongory_value, or NULL on failure.
 */
typedef mongory_value *(*mongory_shallow_convert_func)(mongory_memory_pool *pool, void *value);

/**
 * @brief Function pointer type for recovering an external value from a
 * mongory_value.
 *
 * This is the reverse of a conversion, intended to get back the original
 * external data representation if it was stored or wrapped.
 *
 * @param pool The memory pool (can be used if recovery involves allocation,
 * though often it might just retrieve a stored pointer).
 * @param value The mongory_value from which to recover the external data.
 * @return void* A pointer to the recovered external value, or NULL if not
 * possible.
 */
typedef void *(*mongory_recover_func)(mongory_memory_pool *pool, mongory_value *value);

/**
 * @brief Initializes the Mongory library.
 *
 * Sets up internal structures, including the global memory pool,
 * regex adapter, matcher mapping, and value converter. This function
 * should be called before any other Mongory library functions are used.
 * It also registers the default set of matchers (e.g., $in, $eq).
 */
void mongory_init();

/**
 * @brief Cleans up resources used by the Mongory library.
 *
 * Frees the internal memory pool and resets global pointers. This should
 * be called when the library is no longer needed to prevent memory leaks.
 */
void mongory_cleanup();

/**
 * @brief Sets the custom regex matching function.
 *
 * If not called, a default function that always returns false is used.
 *
 * @param func The custom regex function to use.
 */
void mongory_regex_func_set(mongory_regex_func func);

/**
 * @brief Sets the custom regex stringify function.
 *
 * If not called, a default function that always returns NULL is used.
 *
 * @param func The custom regex stringify function to use.
 */
void mongory_regex_stringify_func_set(mongory_regex_stringify_func func);

/**
 * @brief Sets the custom deep value conversion function.
 * @param deep_convert The function to use for deep conversions.
 */
void mongory_value_converter_deep_convert_set(mongory_deep_convert_func deep_convert);

/**
 * @brief Sets the custom shallow value conversion function.
 * @param shallow_convert The function to use for shallow conversions.
 */
void mongory_value_converter_shallow_convert_set(mongory_shallow_convert_func shallow_convert);

/**
 * @brief Sets the custom value recovery function.
 * @param recover The function to use for recovering external values.
 */
void mongory_value_converter_recover_set(mongory_recover_func recover);

/**
 * @brief Function pointer type for matching a value against an external matcher.
 * @param external_matcher The external reference to the matcher.
 * @param value The value to match against.
 * @return bool True if the value matches the matcher, false otherwise.
 */
typedef struct mongory_matcher_custom_context {
  char *name; // Name of the custom matcher.
  void *external_matcher; // External reference to the custom matcher.
} mongory_matcher_custom_context;

void mongory_custom_matcher_match_func_set(bool (*match)(void *external_matcher, mongory_value *value));
void mongory_custom_matcher_build_func_set(mongory_matcher_custom_context *(*build)(char *key, mongory_value *condition, void *extern_ctx));
void mongory_custom_matcher_lookup_func_set(bool (*lookup)(char *key));

void mongory_matcher_trace_result_colorful_set(bool colorful);

#endif /* MONGORY_FOUNDATIONS_CONFIG_H */
