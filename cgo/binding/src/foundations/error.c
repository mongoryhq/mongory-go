#ifndef MONGORY_ERROR_C
#define MONGORY_ERROR_C
/**
 * @file error.c
 * @brief Implements error type to string conversion for the Mongory library.
 */
#include <mongory-core/foundations/error.h>

/**
 * @brief Converts a mongory_error_type enum to its human-readable string
 * representation.
 *
 * This function uses the MONGORY_ERROR_TYPE_MACRO to generate a switch
 * statement that maps each error enum to its corresponding string.
 *
 * @param type The mongory_error_type enum value to convert.
 * @return const char* A string literal representing the error type. If the
 * type is not recognized, it returns "Unknown Error Type".
 */
const char *mongory_error_type_to_string(enum mongory_error_type type) {
  switch (type) {
// Use the X-Macro to generate case statements for each error type.
#define DEFINE_ERROR_STRING(name, num, str)                                                                            \
  case name:                                                                                                           \
    return str;
    MONGORY_ERROR_TYPE_MACRO(DEFINE_ERROR_STRING)
#undef DEFINE_ERROR_STRING // Undefine the macro after use.
  default:
    // Fallback for any undefined or unknown error types.
    return "Unknown Error Type";
  }
}

mongory_error MONGORY_ALLOC_ERROR = {
  .type = MONGORY_ERROR_MEMORY,
  .message = "Memory Allocation Failed",
};
#endif