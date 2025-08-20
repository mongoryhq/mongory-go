/**
 * @file value.c
 * @brief Implements the mongory_value generic value type and its operations.
 *
 * This file provides functions for creating (`wrap`), comparing, and
 * converting `mongory_value` instances to strings. Each data type supported by
 * `mongory_value` has a corresponding comparison function and wrapping
 * function.
 */
#include "string_buffer.h"
#include <mongory-core/foundations/array.h>
#include <mongory-core/foundations/config.h> 
#include "utils.h" // For mongory_string_cpy
#include <mongory-core/foundations/memory_pool.h>
#include <mongory-core/foundations/table.h>
#include <mongory-core/foundations/value.h>
#include "config_private.h"
#include <stddef.h> // For NULL
#include <stdio.h>  // For snprintf
#include <stdlib.h> // For general utilities (not directly used here but common)
#include <string.h> // For strcmp

/**
 * @brief Returns a string literal representing the type of the mongory_value.
 * @param value The mongory_value to inspect.
 * @return A string like "Int", "String", "Array", etc., or "UnknownType".
 */
char *mongory_type_to_string(mongory_value *value) {
  if (!value)
    return "NullValuePtr"; // Handle null pointer to value itself
  switch (value->type) {
// Use X-Macro to generate cases for each defined type.
#define CASE_GEN(name, num, str, field)                                                                                \
  case name:                                                                                                           \
    return str;
    MONGORY_TYPE_MACRO(CASE_GEN)
#undef CASE_GEN
  default:
    return "UnknownType"; // Fallback for unrecognized types.
  }
}

/**
 * @brief Extracts a void pointer to the raw data within the mongory_value.
 * The caller must know the actual type to cast this pointer correctly.
 * @param value The mongory_value from which to extract data.
 * @return A void pointer to the data, or NULL for unknown types or MONGORY_TYPE_NULL.
 */
void *mongory_value_extract(mongory_value *value) {
  if (!value)
    return NULL;
  switch (value->type) {
// Use X-Macro to generate cases. For MONGORY_TYPE_NULL, &value->data.i is
// returned but is meaningless as data for NULL.
#define EXTRACT_CASE(name, num, str, field)                                                                            \
  case name:                                                                                                           \
    return (void *)&value->data.field;
    MONGORY_TYPE_MACRO(EXTRACT_CASE)
#undef EXTRACT_CASE
  default:
    return NULL; // Unknown type.
  }
}

static char *mongory_value_null_to_str(mongory_value *value, mongory_memory_pool *pool);
static char *mongory_value_bool_to_str(mongory_value *value, mongory_memory_pool *pool);
static char *mongory_value_int_to_str(mongory_value *value, mongory_memory_pool *pool);
static char *mongory_value_double_to_str(mongory_value *value, mongory_memory_pool *pool);
static char *mongory_value_string_to_str(mongory_value *value, mongory_memory_pool *pool);
static char *mongory_value_array_to_str(mongory_value *value, mongory_memory_pool *pool);
static char *mongory_value_table_to_str(mongory_value *value, mongory_memory_pool *pool);
static char *mongory_value_generic_ptr_to_str(mongory_value *value, mongory_memory_pool *pool);

/**
 * @brief Internal helper to allocate a new mongory_value structure from a pool.
 * @param pool The memory pool to allocate from.
 * @return A pointer to the new mongory_value, or NULL on allocation failure.
 */
static inline mongory_value *mongory_value_new(mongory_memory_pool *pool) {
  if (!pool || !pool->alloc)
    return NULL; // Invalid pool.
  mongory_value *value = MG_ALLOC_PTR(pool, mongory_value);
  if (!value) {
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }
  value->pool = pool;
  value->origin = NULL; // Default origin to NULL.
  return value;
}

// ============================================================================
// Comparison Functions
//
// Each data type has a corresponding comparison function. These functions
// are assigned to the `comp` function pointer in the `mongory_value` struct.
// They typically handle comparisons with other compatible types (e.g., Int
// can be compared with Double).
// ============================================================================

/** Compares two MONGORY_TYPE_NULL values. Always equal. */
static inline int mongory_value_null_compare(mongory_value *a, mongory_value *b) {
  (void)a; // 'a' is implicitly MONGORY_TYPE_NULL by context of call.
  if (b->type != MONGORY_TYPE_NULL) {
    return mongory_value_compare_fail; // Cannot compare NULL with non-NULL.
  }
  return 0; // NULL is equal to NULL.
}

/** Wraps a NULL value. The `n` parameter is ignored. */
mongory_value *mongory_value_wrap_n(mongory_memory_pool *pool, void *n) {
  (void)n; // Parameter 'n' is unused for wrapping NULL.
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_NULL;
  value->comp = mongory_value_null_compare;
  value->to_str = mongory_value_null_to_str;
  // data union is not explicitly set for NULL.
  return value;
}

// ============================================================================
// Wrapper Functions
//
// These functions create a `mongory_value` and wrap a native C type
// (like bool, int, char*). They allocate the value from the memory pool and
// set its type, data, and function pointers (`comp`, `to_str`).
// ============================================================================

/** Compares two MONGORY_TYPE_BOOL values. */
static inline int mongory_value_bool_compare(mongory_value *a, mongory_value *b) {
  if (b->type != MONGORY_TYPE_BOOL)
    return mongory_value_compare_fail;
  // Standard comparison: (true > false)
  return (a->data.b > b->data.b) - (a->data.b < b->data.b);
}

/** Wraps a boolean value. */
mongory_value *mongory_value_wrap_b(mongory_memory_pool *pool, bool b_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_BOOL;
  value->data.b = b_val;
  value->comp = mongory_value_bool_compare;
  value->to_str = mongory_value_bool_to_str;
  return value;
}

/** Compares MONGORY_TYPE_INT with INT or DOUBLE. */
static inline int mongory_value_int_compare(mongory_value *a, mongory_value *b) {
  if (b->type == MONGORY_TYPE_DOUBLE) {
    // Compare int (a) as double with double (b).
    double a_as_double = (double)a->data.i;
    double b_value = b->data.d;
    return (a_as_double > b_value) - (a_as_double < b_value);
  }
  if (b->type == MONGORY_TYPE_INT) {
    // Compare int (a) with int (b).
    int64_t a_value = a->data.i;
    int64_t b_value = b->data.i;
    return (a_value > b_value) - (a_value < b_value);
  }
  return mongory_value_compare_fail; // Incompatible type for comparison.
}

/** Wraps an integer (promoted to int64_t). */
mongory_value *mongory_value_wrap_i(mongory_memory_pool *pool, int64_t i_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_INT;
  value->data.i = i_val;
  value->comp = mongory_value_int_compare;
  value->to_str = mongory_value_int_to_str;
  return value;
}

/** Compares MONGORY_TYPE_DOUBLE with DOUBLE or INT. */
static inline int mongory_value_double_compare(mongory_value *a, mongory_value *b) {
  if (b->type == MONGORY_TYPE_DOUBLE) {
    double a_value = a->data.d;
    double b_value = b->data.d;
    return (a_value > b_value) - (a_value < b_value);
  }
  if (b->type == MONGORY_TYPE_INT) {
    // Compare double (a) with int (b) as double.
    double a_value = a->data.d;
    double b_as_double = (double)b->data.i;
    return (a_value > b_as_double) - (a_value < b_as_double);
  }
  return mongory_value_compare_fail; // Incompatible type.
}

/** Wraps a double value. */
mongory_value *mongory_value_wrap_d(mongory_memory_pool *pool, double d_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_DOUBLE;
  value->data.d = d_val;
  value->comp = mongory_value_double_compare;
  value->to_str = mongory_value_double_to_str;
  return value;
}

/** Compares two MONGORY_TYPE_STRING values lexicographically. */
static inline int mongory_value_string_compare(mongory_value *a, mongory_value *b) {
  // Ensure both are strings and not NULL pointers.
  if (b->type != MONGORY_TYPE_STRING || a->data.s == NULL || b->data.s == NULL) {
    // If one is NULL string and other is not, how to compare?
    // For now, strict: both must be valid strings.
    // Or, define NULL string < non-NULL string.
    // Current: fail if not string or if actual char* is NULL.
    return mongory_value_compare_fail;
  }
  int cmp_result = strcmp(a->data.s, b->data.s);
  return (cmp_result > 0) - (cmp_result < 0); // Normalize to -1, 0, 1
}

/** Wraps a string. Makes a copy of the string using the pool. */
mongory_value *mongory_value_wrap_s(mongory_memory_pool *pool, char *s_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_STRING;
  // `mongory_string_cpy` handles the allocation and copying of the string.
  // If `s_val` is NULL, `value->data.s` will be NULL.
  // If allocation fails, `value->data.s` will also be NULL.
  // TODO: This function should ideally return NULL if the string copy fails
  // when `s_val` was not NULL, but this requires a more complex error handling
  // strategy with the memory pool.
  value->data.s = mongory_string_cpy(pool, s_val);
  value->comp = mongory_value_string_compare;
  value->to_str = mongory_value_string_to_str;
  return value;
}

/** Compares two MONGORY_TYPE_ARRAY values element by element. */
static inline int mongory_value_array_compare(mongory_value *a, mongory_value *b) {
  if (b->type != MONGORY_TYPE_ARRAY || a->data.a == NULL || b->data.a == NULL) {
    return mongory_value_compare_fail; // Must be two valid arrays.
  }
  struct mongory_array *array_a = a->data.a;
  struct mongory_array *array_b = b->data.a;

  // First, compare by array size. An array with fewer elements is "less than"
  // an array with more elements.
  if (array_a->count != array_b->count) {
    return (array_a->count > array_b->count) - (array_a->count < array_b->count);
  }

  // Same count, compare element by element.
  for (size_t i = 0; i < array_a->count; i++) {
    mongory_value *item_a = array_a->get(array_a, i);
    mongory_value *item_b = array_b->get(array_b, i);

    // Handle NULL elements within arrays carefully.
    bool a_item_is_null = (item_a == NULL || item_a->type == MONGORY_TYPE_NULL);
    bool b_item_is_null = (item_b == NULL || item_b->type == MONGORY_TYPE_NULL);

    if (a_item_is_null && b_item_is_null)
      continue; // Both null, considered equal here.
    if (a_item_is_null)
      return -1; // Null is less than non-null.
    if (b_item_is_null)
      return 1; // Non-null is greater than null.

    // Both items are non-null, compare them using their own comp functions.
    if (!item_a->comp || !item_b->comp)
      return mongory_value_compare_fail; // Should not happen for valid values

    int cmp_result = item_a->comp(item_a, item_b);
    if (cmp_result == mongory_value_compare_fail)
      return mongory_value_compare_fail; // Incomparable elements
    if (cmp_result != 0) {
      return cmp_result; // First differing element determines array order.
    }
  }
  return 0; // All elements are equal.
}

/** Wraps a mongory_array. */
mongory_value *mongory_value_wrap_a(mongory_memory_pool *pool, struct mongory_array *a_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_ARRAY;
  value->data.a = a_val;
  value->comp = mongory_value_array_compare;
  value->to_str = mongory_value_array_to_str;
  return value;
}

/** Compares two MONGORY_TYPE_TABLE values. Currently unsupported (always fails). */
static inline int mongory_value_table_compare(mongory_value *a, mongory_value *b) {
  (void)a; // Unused.
  (void)b; // Unused.
  // True table comparison is complex because the order of keys is not
  // significant. It would require iterating over keys of one table and checking
  // for their presence and value equality in the other. This is not
  // implemented. Therefore, tables are considered incomparable.
  return mongory_value_compare_fail;
}

/** Wraps a mongory_table. */
mongory_value *mongory_value_wrap_t(mongory_memory_pool *pool, struct mongory_table *t_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_TABLE;
  value->data.t = t_val;
  value->comp = mongory_value_table_compare;
  value->to_str = mongory_value_table_to_str;
  return value;
}

/** Generic comparison for types not otherwise handled (e.g. POINTER, REGEX, UNSUPPORTED). Always fails. */
static inline int mongory_value_generic_ptr_compare(mongory_value *a, mongory_value *b) {
  (void)a; // Unused.
  (void)b; // Unused.
  // Generic pointers are generally not comparable in a meaningful way beyond identity.
  return mongory_value_compare_fail;
}

/** Wraps an unsupported/unknown type pointer. */
mongory_value *mongory_value_wrap_u(mongory_memory_pool *pool, void *u_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_UNSUPPORTED;
  value->data.u = u_val;
  value->comp = mongory_value_generic_ptr_compare; // Unsupported types are not comparable.
  value->to_str = mongory_value_generic_ptr_to_str;
  return value;
}

static char *mongory_value_regex_to_str(mongory_value *value, mongory_memory_pool *pool) {
  char *str = mongory_internal_regex_adapter.stringify_func(pool, value);
  if (!str)
    return NULL;
  return str;
}

/** Wraps a regex type pointer. */
mongory_value *mongory_value_wrap_regex(mongory_memory_pool *pool, void *regex_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_REGEX;
  value->data.regex = regex_val;
  value->comp = mongory_value_generic_ptr_compare; // Regex values are not directly comparable.
  value->to_str = mongory_value_regex_to_str;
  return value;
}

/** Wraps a generic void pointer. */
mongory_value *mongory_value_wrap_ptr(mongory_memory_pool *pool, void *ptr_val) {
  mongory_value *value = mongory_value_new(pool);
  if (!value)
    return NULL;
  value->type = MONGORY_TYPE_POINTER;
  value->data.ptr = ptr_val;
  value->comp = mongory_value_generic_ptr_compare; // Generic pointers are not comparable.
  value->to_str = mongory_value_generic_ptr_to_str;
  return value;
}

// ============================================================================
// Stringify Functions
//
// Each data type has a corresponding `to_str` function that converts the
// value to a string representation (often JSON-like) and appends it to a
// `mongory_string_buffer`.
// ============================================================================

static char *mongory_value_null_to_str(mongory_value *value, mongory_memory_pool *pool) {
  (void)value;
  return mongory_string_cpy(pool, "null");
}

static char *mongory_value_bool_to_str(mongory_value *value, mongory_memory_pool *pool) {
  return mongory_string_cpy(pool, value->data.b ? "true" : "false");
}

static char *mongory_value_int_to_str(mongory_value *value, mongory_memory_pool *pool) {
  return mongory_string_cpyf(pool, "%lld", (long long)value->data.i);
}

static char *mongory_value_double_to_str(mongory_value *value, mongory_memory_pool *pool) {
  return mongory_string_cpyf(pool, "%f", value->data.d);
}

static char *mongory_value_string_to_str(mongory_value *value, mongory_memory_pool *pool) {
  return mongory_string_cpyf(pool, "\"%s\"", value->data.s);
}

typedef struct mongory_value_container_to_str_ctx {
  size_t count;
  size_t total;
  mongory_string_buffer *buffer; /**< The string buffer to append to. */
} mongory_value_container_to_str_ctx;

static bool mongory_value_array_to_str_each(mongory_value *value, void *ctx) {
  mongory_value_container_to_str_ctx *context = (mongory_value_container_to_str_ctx *)ctx;
  mongory_string_buffer *buffer = context->buffer;
  char *str = value->to_str(value, buffer->pool);
  if (!str)
    return false;
  mongory_string_buffer_append(buffer, str);
  context->count++;
  if (context->count < context->total) {
    mongory_string_buffer_append(buffer, ",");
  }
  return true;
}

static char *mongory_value_array_to_str(mongory_value *value, mongory_memory_pool *pool) {
  mongory_string_buffer *buffer = mongory_string_buffer_new(pool);
  if (!buffer)
    return NULL;
  mongory_string_buffer_append(buffer, "[");
  struct mongory_array *array = value->data.a;
  mongory_value_container_to_str_ctx ctx = {.count = 0, .total = array->count, .buffer = buffer};
  array->each(array, &ctx, mongory_value_array_to_str_each);
  mongory_string_buffer_append(buffer, "]");
  return mongory_string_buffer_cstr(buffer);
}

static bool mongory_value_table_to_str_each(char *key, mongory_value *value, void *ctx) {
  mongory_value_container_to_str_ctx *context = (mongory_value_container_to_str_ctx *)ctx;
  mongory_string_buffer *buffer = context->buffer;
  mongory_string_buffer_appendf(buffer, "\"%s\":", key);
  char *str = value->to_str(value, buffer->pool);
  if (!str)
    return false;
  mongory_string_buffer_append(buffer, str);
  context->count++;
  if (context->count < context->total) {
    mongory_string_buffer_append(buffer, ",");
  }
  return true;
}

static char *mongory_value_table_to_str(mongory_value *value, mongory_memory_pool *pool) {
  mongory_string_buffer *buffer = mongory_string_buffer_new(pool);
  if (!buffer)
    return NULL;
  mongory_string_buffer_append(buffer, "{");
  struct mongory_table *table = value->data.t;
  mongory_value_container_to_str_ctx ctx = {.count = 0, .total = table->count, .buffer = buffer};
  table->each(table, &ctx, mongory_value_table_to_str_each);
  mongory_string_buffer_append(buffer, "}");
  return mongory_string_buffer_cstr(buffer);
}

static char *mongory_value_generic_ptr_to_str(mongory_value *value, mongory_memory_pool *pool) {
  return mongory_string_cpyf(pool, "%p", value->data.ptr);
}
