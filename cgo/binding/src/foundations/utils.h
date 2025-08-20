#ifndef MONGORY_UTILS_H
#define MONGORY_UTILS_H

#include "mongory-core/foundations/memory_pool.h"
#include <stdbool.h>

/**
 * @brief Attempts to parse an integer from a string.
 *
 * This utility function is useful for matchers that might operate on array
 * indices or numeric string fields. It checks for valid integer formats and
 * range (`INT_MIN`, `INT_MAX`).
 *
 * @param key The null-terminated string to parse. Must not be NULL or empty.
 * @param out A pointer to an integer where the parsed value will be stored if
 * successful. Must not be NULL.
 * @return bool True if the string was successfully parsed as an integer and is
 * within `int` range, false otherwise. `errno` might be set by `strtol` on
 * failure (e.g. `ERANGE`).
 */
bool mongory_try_parse_int(const char *key, int *out);

/**
 * @brief Copies a string using the Mongory library's memory pool.
 *
 * Allocates memory from the given pool and copies the source string into it.
 * The returned string is null-terminated.
 *
 * @param pool The memory pool to use for allocating the new string.
 * @param str The source string to copy. If NULL, returns NULL.
 * @return char* A pointer to the newly allocated and copied string, or NULL
 * if allocation fails or str is NULL. The caller does not own this memory
 * directly; it will be freed when the pool is freed.
 */
char *mongory_string_cpy(mongory_memory_pool *pool, char *str);

/**
 * @brief Copies a formatted string using the Mongory library's memory pool.
 *
 * Allocates memory from the given pool and copies the formatted string into it.
 * The returned string is null-terminated.
 *
 * @param pool The memory pool to use for allocating the new string.
 * @param format The format string to copy.
 * @param ... The arguments to be formatted into the string.
 * @return char* A pointer to the newly allocated and copied string, or NULL
 * if allocation fails or format is NULL. The caller does not own this memory
 * directly; it will be freed when the pool is freed.
 */
char *mongory_string_cpyf(mongory_memory_pool *pool, char *format, ...);

double mongory_log(double x, double base);
#endif // MONGORY_UTILS_H