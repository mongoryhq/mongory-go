#ifndef MONGORY_VALUE
#define MONGORY_VALUE

/**
 * @file value.h
 * @brief Defines the mongory_value structure, a generic value type for the
 * Mongory library.
 *
 * `mongory_value` is a tagged union that can represent various data types such
 * as null, boolean, integer, double, string, array, table, regex, pointers,
 * and an unsupported type. It includes functions for wrapping C types into
 * `mongory_value` objects, comparing values, and converting them to strings.
 */

#include "mongory-core/foundations/memory_pool.h"
#include <stdbool.h>
#include <stdint.h> // For int64_t

// Forward declarations for complex types that can be stored in mongory_value.
struct mongory_array;
struct mongory_table;

// Forward declaration of mongory_value itself and its type enum.
struct mongory_value;
/**
 * @brief Alias for `struct mongory_value`.
 */
typedef struct mongory_value mongory_value;

enum mongory_type;
/**
 * @brief Alias for `enum mongory_type`.
 */
typedef enum mongory_type mongory_type;

/**
 * @brief Function pointer type for comparing two mongory_value instances.
 *
 * @param a The first mongory_value.
 * @param b The second mongory_value.
 * @return int Returns:
 *         - 0 if a is equal to b.
 *         - A negative value if a is less than b.
 *         - A positive value if a is greater than b.
 *         - `mongory_value_compare_fail` (a specific constant) if the types
 *           are incompatible for comparison or an error occurs.
 */
typedef int (*mongory_value_compare_func)(mongory_value *a, mongory_value *b);

/**
 * @brief Function pointer type for converting a mongory_value to a string representation.
 * @param value The mongory_value to convert.
 * @param pool The memory pool to allocate from.
 * @return char* A string literal representing the value, or NULL if allocation
 * fails. The string is a literal and should not be freed.
 */
typedef char *(*mongory_value_to_str_func)(mongory_value *value, mongory_memory_pool *pool);

/**
 * @brief Converts the type of a mongory_value to its string representation.
 * @param value A pointer to the mongory_value.
 * @return char* A string literal representing the type (e.g., "Int", "String").
 * Returns "UnknownType" if the type is not recognized. This string is a
 * literal and should not be freed.
 */
char *mongory_type_to_string(mongory_value *value);

/**
 * @brief Extracts a pointer to the raw data stored within a mongory_value.
 * The type of the returned pointer depends on the `mongory_value`'s type.
 * For example, for an MONGORY_TYPE_INT, it returns `int64_t*`.
 * @param value A pointer to the mongory_value.
 * @return void* A pointer to the internal data, or NULL if the type is unknown
 * or has no direct data pointer (e.g. MONGORY_TYPE_NULL).
 */
void *mongory_value_extract(mongory_value *value);

/** @name Mongory Value Wrapper Functions
 *  Functions to create mongory_value instances from basic C types.
 *  These functions allocate a new mongory_value from the provided pool.
 *  @{
 */
mongory_value *mongory_value_wrap_n(mongory_memory_pool *pool, void *n);
mongory_value *mongory_value_wrap_b(mongory_memory_pool *pool, bool b);
mongory_value *mongory_value_wrap_i(mongory_memory_pool *pool, int64_t i);
mongory_value *mongory_value_wrap_d(mongory_memory_pool *pool, double d);
mongory_value *mongory_value_wrap_s(mongory_memory_pool *pool, char *s);
mongory_value *mongory_value_wrap_a(mongory_memory_pool *pool, struct mongory_array *a);
mongory_value *mongory_value_wrap_t(mongory_memory_pool *pool, struct mongory_table *t);
mongory_value *mongory_value_wrap_regex(mongory_memory_pool *pool, void *regex);
mongory_value *mongory_value_wrap_ptr(mongory_memory_pool *pool, void *ptr);
mongory_value *mongory_value_wrap_u(mongory_memory_pool *pool, void *u);
/** @} */

/**
 * @def MONGORY_TYPE_MACRO
 * @brief X-Macro for defining mongory_type enum members and associated data.
 * Simplifies the definition of types, their string names, and corresponding
 * union fields.
 * _(ENUM_NAME, UNIQUE_NUM_SUFFIX, "STRING_NAME", UNION_FIELD_NAME)
 * Note: For MONGORY_TYPE_NULL, the 'i' field (int64_t) is arbitrarily used in
 * the macro as the union needs a field, but it's not meaningful for null.
 */
#define MONGORY_TYPE_MACRO(_)                                                                                          \
  _(MONGORY_TYPE_NULL, 0, "Null", i) /* Field 'i' is arbitrary for NULL */                                             \
  _(MONGORY_TYPE_BOOL, 10, "Bool", b)                                                                                  \
  _(MONGORY_TYPE_INT, 11, "Int", i)                                                                                    \
  _(MONGORY_TYPE_DOUBLE, 12, "Double", d)                                                                              \
  _(MONGORY_TYPE_STRING, 13, "String", s)                                                                              \
  _(MONGORY_TYPE_ARRAY, 14, "Array", a)                                                                                \
  _(MONGORY_TYPE_TABLE, 15, "Table", t)                                                                                \
  _(MONGORY_TYPE_REGEX, 16, "Regex", regex)          /* Custom regex object pointer */                                 \
  _(MONGORY_TYPE_POINTER, 17, "Pointer", ptr)        /* Generic void pointer */                                        \
  _(MONGORY_TYPE_UNSUPPORTED, 999, "Unsupported", u) /* External/unknown type pointer */

/**
 * @def MONGORY_ENUM_MAGIC
 * @brief A magic number used in generating enum values for mongory_type.
 * Helps in creating somewhat unique integer values for the enums.
 */
#define MONGORY_ENUM_MAGIC 103

/**
 * @enum mongory_type
 * @brief Enumerates the possible data types a mongory_value can hold.
 * Values are generated using MONGORY_TYPE_MACRO and MONGORY_ENUM_MAGIC.
 */
enum mongory_type {
#define DEFINE_ENUM(name, num, str, field) name = num * MONGORY_ENUM_MAGIC,
  MONGORY_TYPE_MACRO(DEFINE_ENUM)
#undef DEFINE_ENUM
};

/**
 * @var mongory_value_compare_fail
 * @brief Special return value from compare functions indicating comparison
 * failure (e.g. incompatible types).
 */
static const int mongory_value_compare_fail = 97;

/**
 * @struct mongory_value
 * @brief Represents a generic value in the Mongory system.
 *
 * It's a tagged union, where `type` indicates which field in the `data` union
 * is active. Each value also carries a pointer to the `mongory_memory_pool`
 * it was allocated from (or is associated with) and a `comp` function for
 * comparisons. The `origin` field can be used to store a pointer to an
 * original external data structure if the `mongory_value` is a bridge or
 * wrapper.
 */
struct mongory_value {
  mongory_memory_pool *pool;        /**< Memory pool associated with this value. */
  mongory_type type;                /**< The type of data stored in the union. */
  mongory_value_compare_func comp;  /**< Function to compare this value with
                                       another. */
  mongory_value_to_str_func to_str; /**< Function to convert this value to a
                                      string representation. */
  union {
    bool b;                  /**< Boolean data. */
    int64_t i;               /**< Integer data (64-bit). */
    double d;                /**< Double-precision floating-point data. */
    char *s;                 /**< String data (null-terminated). String memory
                                is typically managed by the pool. */
    struct mongory_array *a; /**< Pointer to a mongory_array. */
    struct mongory_table *t; /**< Pointer to a mongory_table. */
    void *regex;             /**< Pointer to a custom regex object/structure. */
    void *ptr;               /**< Generic void pointer for other data. */
    void *u;                 /**< Pointer for unsupported/external types. */
  } data;                    /**< Union holding the actual data based on type. */
  void *origin;              /**< Optional pointer to an original external value, useful for
                                bridging with other data systems. */
};

#endif /* MONGORY_VALUE */
