#ifndef MONGORY_UTILS_C
#define MONGORY_UTILS_C

#include "utils.h"
#include <errno.h>
#include <limits.h>
#include <stdlib.h>
#include <stdbool.h>
#include <stdarg.h>
#include <string.h>
#include <stdio.h>
#include <math.h>

/**
 * @brief Attempts to parse a string `key` into an integer `out`.
 *
 * Uses `strtol` for conversion and checks for common parsing errors:
 * - Input string is NULL or empty.
 * - The string contains non-numeric characters after the number.
 * - The parsed number is out of the range of `int` (`INT_MIN`, `INT_MAX`).
 *
 * @param key The null-terminated string to parse.
 * @param out Pointer to an integer where the result is stored.
 * @return `true` if parsing is successful and the value fits in an `int`.
 *         `false` otherwise. `errno` may be set by `strtol`.
 */
bool mongory_try_parse_int(const char *key, int *out) {
  if (key == NULL || *key == '\0') {
    return false; // Invalid input string.
  }
  if (out == NULL) {
    return false; // Output pointer must be valid.
  }

  char *endptr = NULL;
  errno = 0;                           // Clear errno before calling strtol.
  long val = strtol(key, &endptr, 10); // Base 10 conversion.

  // Check for parsing errors reported by strtol.
  if (endptr == key) {
    return false; // No digits were found.
  }
  if (*endptr != '\0') {
    return false; // Additional characters after the number.
  }
  if (errno == ERANGE || val < INT_MIN || val > INT_MAX) {
    // Value out of range for int. ERANGE is set by strtol for overflow/underflow.
    return false;
  }

  *out = (int)val; // Successfully parsed and within int range.
  return true;
}

/**
 * @brief Creates a copy of a string using the specified memory pool.
 * @param pool The memory pool to use for allocation.
 * @param str The null-terminated string to copy.
 * @return A pointer to the newly allocated copy, or NULL if str is NULL or
 * allocation fails.
 */
char *mongory_string_cpy(mongory_memory_pool *pool, char *str) {
  if (str == NULL) {
    return NULL;
  }

  size_t len = strlen(str);
  char *new_str = (char *)MG_ALLOC(pool, len + 1); // +1 for null terminator.
  if (new_str == NULL) {
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }

  strcpy(new_str, str);
  return new_str;
}

char *mongory_string_cpyf(mongory_memory_pool *pool, char *format, ...) {
  va_list args;
  va_start(args, format);
  int len = vsnprintf(NULL, 0, format, args);
  va_end(args);
  char *new_str = (char *)MG_ALLOC(pool, len + 1);
  if (new_str == NULL) {
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }
  va_start(args, format);
  vsnprintf(new_str, len + 1, format, args);
  va_end(args);
  new_str[len] = '\0';
  return new_str;
}

double mongory_log(double x, double base) {
  return log(x) / log(base);
}

#endif // MONGORY_UTILS_C