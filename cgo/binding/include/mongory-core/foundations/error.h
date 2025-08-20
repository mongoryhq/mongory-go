#ifndef MONGORY_ERROR
#define MONGORY_ERROR

/**
 * @file error.h
 * @brief Defines error types and structures for the Mongory library.
 *
 * This file provides an enumeration of possible error codes and a structure
 * for representing error information, typically consisting of an error type
 * and an associated message.
 */

#include <stdbool.h>

/**
 * @def MONGORY_ERROR_TYPE_MAGIC
 * @brief A magic number used in generating enum values for error types.
 * Helps in creating somewhat unique integer values for the enums.
 */
#define MONGORY_ERROR_TYPE_MAGIC 107

/**
 * @def MONGORY_ERROR_TYPE_MACRO
 * @brief X-Macro for defining error types.
 * This macro allows defining the error enum, string conversion, etc.,
 * from a single list, promoting consistency.
 * Each entry takes the enum name, a numeric suffix, and a string description.
 */
#define MONGORY_ERROR_TYPE_MACRO(_)                                                                                    \
  _(MONGORY_ERROR_NONE, 10, "No Error")                                                                                \
  _(MONGORY_ERROR_MEMORY, 11, "Memory Allocation Error")                                                               \
  _(MONGORY_ERROR_INVALID_TYPE, 12, "Invalid Type Error")                                                              \
  _(MONGORY_ERROR_OUT_OF_BOUNDS, 13, "Out of Bounds Error")                                                            \
  _(MONGORY_ERROR_UNSUPPORTED_OPERATION, 14, "Unsupported Operation Error")                                            \
  _(MONGORY_ERROR_INVALID_ARGUMENT, 15, "Invalid Argument Error")                                                      \
  _(MONGORY_ERROR_IO, 16, "I/O Error")                                                                                 \
  _(MONGORY_ERROR_PARSE, 17, "Parse Error")                                                                            \
  _(MONGORY_ERROR_UNKNOWN, 99, "Unknown Error")

/**
 * @enum mongory_error_type
 * @brief Enumerates the types of errors that can occur within the Mongory
 * library. Values are generated using MONGORY_ERROR_TYPE_MACRO and
 * MONGORY_ERROR_TYPE_MAGIC.
 */
typedef enum mongory_error_type {
#define DEFINE_ERROR_ENUM(name, num, str) name = num * MONGORY_ERROR_TYPE_MAGIC,
  MONGORY_ERROR_TYPE_MACRO(DEFINE_ERROR_ENUM)
#undef DEFINE_ERROR_ENUM
} mongory_error_type;

/**
 * @brief Converts a mongory_error_type enum to its string representation.
 *
 * @param type The error type enum value.
 * @return const char* A string describing the error type. Returns "Unknown
 * Error Type" if the type is not recognized.
 */
const char *mongory_error_type_to_string(enum mongory_error_type type);

/**
 * @struct mongory_error
 * @brief Structure to hold error information.
 *
 * Contains the type of error and an optional descriptive message.
 * The message string is expected to be a literal or have a lifetime
 * managed elsewhere (e.g., allocated from a memory pool).
 */
typedef struct mongory_error {
  mongory_error_type type; /**< The type of the error. */
  const char *message;     /**< A descriptive message for the error. This message
                                might be a string literal or allocated from a
                                memory pool. Its lifetime should be considered
                                carefully. */
} mongory_error;

extern mongory_error MONGORY_ALLOC_ERROR;

#endif /* MONGORY_ERROR */
