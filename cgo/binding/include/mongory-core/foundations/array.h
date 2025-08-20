#ifndef MONGORY_ARRAY
#define MONGORY_ARRAY

/**
 * @file array.h
 * @brief Defines the mongory_array structure and its associated operations.
 *
 * mongory_array is a dynamic array implementation that stores mongory_value
 * pointers. It handles its own memory management for the array storage using a
 * provided mongory_memory_pool. The array can grow automatically as new
 * elements are added.
 */

#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include <stdbool.h>

/**
 * @brief Forward declaration for the mongory_array structure.
 */
typedef struct mongory_array mongory_array;

/**
 * @brief Callback function type used by mongory_array_each_func.
 *
 * This function is called for each item in the array during an iteration.
 *
 * @param item A pointer to the current mongory_value item in the array.
 * @param acc An accumulator or context pointer passed through the iteration.
 * @return bool Return true to continue iteration, false to stop.
 */
typedef bool (*mongory_array_callback_func)(mongory_value *item, void *acc);

/**
 * @brief Function pointer type for iterating over the array.
 * @param self A pointer to the mongory_array instance.
 * @param acc An accumulator or context pointer to be passed to the callback.
 * @param func The callback function to be executed for each item.
 * @return bool Returns true if the iteration completed fully, false if it was
 * stopped by a callback.
 */
typedef bool (*mongory_array_each_func)(mongory_array *self, void *acc, mongory_array_callback_func func);

/**
 * @brief Function pointer type for adding an element to the end of the array.
 * @param self A pointer to the mongory_array instance.
 * @param value A pointer to the mongory_value to add.
 * @return bool Returns true if the value was successfully added, false
 * otherwise (e.g., memory allocation failure).
 */
typedef bool (*mongory_array_push_func)(mongory_array *self, mongory_value *value);

/**
 * @brief Function pointer type for retrieving an element at a specific index.
 * @param self A pointer to the mongory_array instance.
 * @param index The index of the element to retrieve.
 * @return mongory_value* A pointer to the mongory_value at the given index, or
 * NULL if the index is out of bounds.
 */
typedef mongory_value *(*mongory_array_get_func)(mongory_array *self, size_t index);

/**
 * @brief Function pointer type for setting or replacing an element at a
 * specific index.
 *
 * If the index is beyond the current count, the array will be grown, and
 * intermediate elements will be initialized to NULL.
 *
 * @param self A pointer to the mongory_array instance.
 * @param index The index at which to set the value.
 * @param value A pointer to the mongory_value to set.
 * @return bool Returns true if the value was successfully set, false otherwise
 * (e.g., memory allocation failure).
 */
typedef bool (*mongory_array_set_func)(mongory_array *self, size_t index, mongory_value *value);

/**
 * @brief Creates a new mongory_array instance.
 *
 * The array is initialized with a default capacity and will use the provided
 * memory pool for all its internal allocations related to storing elements.
 *
 * @param pool A pointer to the mongory_memory_pool to be used for allocations.
 * @return mongory_array* A pointer to the newly created mongory_array, or NULL
 * if creation fails (e.g., memory allocation failure).
 */
mongory_array *mongory_array_new(mongory_memory_pool *pool);

/**
 * @brief Creates a new mongory_array instance with a nested array.
 *
 * This function is used to create a new mongory_array instance with a nested
 * array. The nested array is created with the given memory pool and the given
 * values. The nested array is returned as a pointer to the mongory_array
 * structure.
 *
 * @param pool A pointer to the mongory_memory_pool to be used for allocations.
 * @param argc The number of values to be set.
 * @param ... The values to be set.
 * @return mongory_array* A pointer to the newly created mongory_array, or NULL
 * if creation fails (e.g., memory allocation failure).
 */
mongory_array *mongory_array_nested_wrap(mongory_memory_pool *pool, int argc, ...);
#define MG_ARRAY_WRAP(pool, n, ...) mongory_value_wrap_a(pool, mongory_array_nested_wrap(pool, n, __VA_ARGS__))

/**
 * @struct mongory_array
 * @brief Represents a dynamic array of mongory_value pointers.
 *
 * This structure provides function pointers for common array operations,
 * allowing for a somewhat object-oriented interface in C.
 */
struct mongory_array {
  size_t count;                 /**< The current number of elements in the array. */
  mongory_memory_pool *pool;    /**< The memory pool used for allocations. */
  mongory_array_each_func each; /**< Function to iterate over elements in the array. */
  mongory_array_push_func push; /**< Function to add an element to the end of the array. */
  mongory_array_get_func get;   /**< Function to retrieve an element by index. */
  mongory_array_set_func set;   /**< Function to set or replace an element at an index. */
};

#endif /* MONGORY_ARRAY */
