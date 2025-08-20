#ifndef MONGORY_TABLE_H
#define MONGORY_TABLE_H

/**
 * @file table.h
 * @brief Defines the mongory_table structure (a hash table) and its
 * associated operations.
 *
 * mongory_table implements a hash table mapping string keys to mongory_value
 * pointers. It uses a mongory_memory_pool for its allocations and handles
 * operations like get, set, delete, and iteration over key-value pairs.
 * The table automatically resizes (rehashes) when its load factor is exceeded.
 */

#include "mongory-core/foundations/array.h" // Used internally by the table
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include <stdbool.h>
#include <stddef.h> // For size_t

// Forward declaration of the mongory_table structure.
struct mongory_table;
/**
 * @brief Alias for `struct mongory_table`.
 */
typedef struct mongory_table mongory_table;

/**
 * @brief Callback function type for iterating over key-value pairs in a table.
 * @param key The current key (null-terminated string).
 * @param value A pointer to the current mongory_value.
 * @param acc An accumulator or context pointer passed through the iteration.
 * @return bool Return true to continue iteration, false to stop.
 */
typedef bool (*mongory_table_each_pair_callback_func)(char *key, mongory_value *value, void *acc);

/**
 * @brief Function pointer type for retrieving a value by its key.
 * @param self A pointer to the mongory_table instance.
 * @param key The null-terminated string key.
 * @return mongory_value* A pointer to the mongory_value associated with the
 * key, or NULL if the key is not found.
 */
typedef mongory_value *(*mongory_table_get_func)(mongory_table *self, char *key);

/**
 * @brief Function pointer type for setting (adding or updating) a key-value
 * pair.
 * @param self A pointer to the mongory_table instance.
 * @param key The null-terminated string key. The table will make its own copy
 * of this key.
 * @param value A pointer to the mongory_value to associate with the key.
 * @return bool True if the operation was successful, false otherwise (e.g.,
 * memory allocation failure).
 */
typedef bool (*mongory_table_set_func)(mongory_table *self, char *key, mongory_value *value);

/**
 * @brief Function pointer type for iterating over all key-value pairs in the
 * table.
 * @param self A pointer to the mongory_table instance.
 * @param acc An accumulator or context pointer to be passed to the callback.
 * @param callback The function to be executed for each key-value pair.
 * @return bool True if the iteration completed fully, false if it was stopped
 * by a callback.
 */
typedef bool (*mongory_table_each_func)(mongory_table *self, void *acc, mongory_table_each_pair_callback_func callback);

/**
 * @brief Function pointer type for deleting a key-value pair from the table.
 * @param self A pointer to the mongory_table instance.
 * @param key The null-terminated string key of the pair to delete.
 * @return bool True if the key was found and deleted, false otherwise.
 */
typedef bool (*mongory_table_del_func)(mongory_table *self, char *key);

/**
 * @brief Creates a new mongory_table instance.
 *
 * Initializes the hash table with a default capacity and associates it with the
 * provided memory pool for all internal allocations.
 *
 * @param pool A pointer to the mongory_memory_pool to be used for allocations.
 * @return mongory_table* A pointer to the newly created mongory_table, or NULL
 * if creation fails (e.g., memory allocation failure).
 */
mongory_table *mongory_table_new(mongory_memory_pool *pool);

/**
 * @brief Creates a new mongory_table instance with a nested table.
 *
 * This function is used to create a new mongory_table instance with a nested
 * table. The nested table is created with the given memory pool and the given
 * key-value pairs. The nested table is returned as a pointer to the
 * mongory_table structure.
 *
 * @param pool A pointer to the mongory_memory_pool to be used for allocations.
 * @param argc The number of key-value pairs to be set.
 * @param ... The key-value pairs to be set.
 * @return mongory_table* A pointer to the newly created mongory_table, or NULL
 * if creation fails (e.g., memory allocation failure).
 */
mongory_table *mongory_table_nested_wrap(mongory_memory_pool *pool, int argc, ...);
#define MG_TABLE_WRAP(pool, n, ...) mongory_value_wrap_t(pool, mongory_table_nested_wrap(pool, n, __VA_ARGS__))

/**
 * @struct mongory_table
 * @brief Represents a hash table mapping string keys to mongory_value pointers.
 *
 * Provides function pointers for common table operations, similar to an
 * object-oriented interface. The `count` field indicates the number of
 * key-value pairs currently in the table and should be treated as read-only by
 * users of the table.
 */
struct mongory_table {
  mongory_memory_pool *pool;    /**< The memory pool used for allocations. */
  size_t count;                 /**< Read-only. The current number of key-value pairs in the
                                   table. */
  mongory_table_get_func get;   /**< Function to get a value by key. */
  mongory_table_each_func each; /**< Function to iterate over key-value pairs. */
  mongory_table_set_func set;   /**< Function to set a key-value pair. */
  mongory_table_del_func del;   /**< Function to delete a key-value pair. */
};

mongory_table *mongory_table_merge(mongory_table *table, mongory_table *other);

#endif /* MONGORY_TABLE_H */
