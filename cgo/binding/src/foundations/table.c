/**
 * @file table.c
 * @brief Implements the mongory_table hash table.
 *
 * This file contains the internal logic for a hash table that maps string keys
 * to mongory_value pointers. It uses separate chaining for collision resolution,
 * where each bucket in the hash table is a linked list of nodes. The underlying
 * storage for buckets is a mongory_array. The table automatically rehashes
 * when the load factor exceeds a threshold.
 */
#include "array_private.h" // For mongory_array_private details if needed
#include <mongory-core/foundations/array.h>
#include <mongory-core/foundations/config.h> // For mongory_string_cpy
#include <mongory-core/foundations/table.h>
#include <mongory-core/foundations/value.h>
#include "utils.h"
#include <stdarg.h>
#include <string.h> // For strcmp, strlen

/**
 * @def MONGORY_TABLE_INIT_SIZE
 * @brief Initial capacity (number of buckets) for a new hash table.
 * Should ideally be a prime number.
 */
#define MONGORY_TABLE_INIT_SIZE 17

/**
 * @def MONGORY_TABLE_LOAD_FACTOR
 * @brief The maximum load factor before the table is rehashed.
 * Load factor = count / capacity.
 */
#define MONGORY_TABLE_LOAD_FACTOR 0.75

/**
 * @struct mongory_table_node
 * @brief Represents a node in a hash table bucket's linked list.
 * Stores a key-value pair and a pointer to the next node in the chain.
 */
typedef struct mongory_table_node {
  char *key;                       /**< The string key for this entry. */
  mongory_value *value;            /**< The mongory_value associated with the key. */
  struct mongory_table_node *next; /**< Pointer to the next node in the collision chain. */
} mongory_table_node;

/**
 * @struct mongory_table_internal
 * @brief Internal representation of the hash table.
 * Extends the public mongory_table structure with capacity information
 * and the underlying array used for buckets.
 */
typedef struct mongory_table_internal {
  mongory_table base;   /**< Public part of the table structure. */
  size_t capacity;      /**< Current number of buckets in the table. */
  mongory_array *array; /**< Array of mongory_table_node pointers (the buckets). */
} mongory_table_internal;

// ============================================================================
// Static Helper Functions
//
// The following functions are static and provide the core logic for the hash
// table operations. They are not part of the public API.
// ============================================================================

/**
 * @brief Allocates a new mongory_table_node from the table's memory pool.
 * @param self Pointer to the mongory_table (used to access its memory pool).
 * @return mongory_table_node* Pointer to the new node, or NULL on failure.
 */
static inline mongory_table_node *mongory_table_node_new(mongory_table *self) {
  return MG_ALLOC_PTR(self->pool, mongory_table_node);
}

/**
 * @brief Finds the next prime number greater than or equal to n.
 *
 * Using a prime number for the capacity of a hash table helps to distribute
 * keys more uniformly, reducing collisions.
 *
 * @param n The number to start searching from.
 * @return The next prime number.
 */
static inline size_t next_prime(size_t n) {
  if (n <= 2) {
    return 2;
  }
  if (n % 2 == 0) // Ensure n is odd to start.
    n++;

  while (1) {
    bool is_prime = true;
    // Check divisibility up to sqrt(n).
    for (size_t i = 3; i * i <= n; i += 2) {
      if (n % i == 0) {
        is_prime = false;
        break;
      }
    }
    if (is_prime) {
      return n;
    }
    n += 2; // Check next odd number.
  }
}

/**
 * @brief Computes a hash value for a null-terminated string (djb2 algorithm).
 * @param str The string to hash.
 * @return The hash value.
 */
static inline size_t hash_string(const char *str) {
  size_t hash = 5381; // Initial hash value.
  int c;
  while ((c = *str++)) {             // Iterate through characters of the string.
    hash = ((hash << 5) + hash) + c; /* hash * 33 + c */
  }
  return hash;
}

/**
 * @brief Walks a linked list of mongory_table_node, applying a callback to each.
 * @param head The head of the linked list.
 * @param acc Accumulator/context for the callback.
 * @param callback Function to call for each node. Stops if callback returns
 * false.
 * @return true if iteration completed, false if callback stopped it.
 */
static inline bool mongory_table_node_walk(mongory_table_node *head, void *acc,
                                           bool (*callback)(mongory_table_node *node, void *acc)) {
  for (mongory_table_node *node = head; node;) {
    mongory_table_node *next = node->next; // Save next before callback modifies node
    if (!callback(node, acc)) {
      return false; // Callback requested stop.
    }
    node = next;
  }
  return true;
}

/**
 * @brief Callback used during rehashing to re-insert a node into the new table
 * structure.
 * Calculates the new bucket index for the node and prepends it to that
 * bucket's list.
 * @param node The node to rehash.
 * @param acc Pointer to the mongory_table_internal structure (cast from void*).
 * @return Always true to continue walking the old bucket list.
 */
static inline bool mongory_table_rehash_on_node(mongory_table_node *node, void *acc) {
  mongory_table_internal *internal = (mongory_table_internal *)acc;
  mongory_array *new_bucket_array = internal->array;
  // Calculate index in the new array based on new capacity.
  size_t new_index = hash_string(node->key) % internal->capacity;

  // Get current head of the new bucket's list.
  mongory_table_node *current_bucket_head = (mongory_table_node *)new_bucket_array->get(new_bucket_array, new_index);
  // Prepend the node to this list.
  node->next = current_bucket_head;
  new_bucket_array->set(new_bucket_array, new_index, (mongory_value *)node);
  return true;
}

/**
 * @brief Rehashes the table when the load factor is exceeded.
 * Creates a new underlying array with a larger, prime capacity, and
 * re-inserts all existing nodes into this new array.
 * @param self Pointer to the mongory_table.
 * @return true if rehashing was successful, false otherwise.
 */
static inline bool mongory_table_rehash(mongory_table *self) {
  mongory_table_internal *internal = (mongory_table_internal *)self;
  mongory_array_private *old_array_private_view = (mongory_array_private *)internal->array;

  mongory_value **original_items_ptr = old_array_private_view->items;
  size_t original_capacity = internal->capacity;
  size_t new_capacity = next_prime(original_capacity * 2);

  // Reset the count of the existing array before resizing, as resize
  // might copy elements if count is not zero. We are managing elements manually.
  internal->array->count = 0;

  if (!mongory_array_resize(internal->array, new_capacity)) {
    // Error: Rehashing failed because the underlying array could not be resized.
    // The table remains functional but may have a suboptimal load factor.
    self->pool->error = &MONGORY_ALLOC_ERROR;
    internal->array->count = self->count; // Try to restore roughly
    return false;
  }
  // After resize, internal->array (and old_array_private_view) points to the
  // new items array. The old items array (original_items_ptr) is now detached
  // but its contents (the nodes) are still valid.

  internal->capacity = new_capacity; // Update table's capacity.

  // Iterate through all buckets of the old array structure.
  for (size_t i = 0; i < original_capacity; i++) {
    // Walk the linked list in each old bucket and rehash each node.
    mongory_table_node_walk((mongory_table_node *)original_items_ptr[i], self, mongory_table_rehash_on_node);
  }
  // The memory for original_items_ptr itself (the array of pointers) is now
  // stale / managed by the memory pool if mongory_array_resize reallocated.
  // The mongory_table_nodes it pointed to have been relinked into the new array.
  return true;
}

/**
 * @struct mongory_table_kv_context
 * @brief Context structure used for get and set operations within a bucket
 * list.
 */
typedef struct mongory_table_kv_context {
  char *key;            /**< The key being searched for or set. */
  mongory_value *value; /**< Stores the found value (for get) or the value to set. */
} mongory_table_kv_context;

/**
 * @brief Callback for mongory_table_get to find a key in a bucket's list.
 * If the node's key matches ctx->key, sets ctx->value and returns false to stop.
 * @param node Current node in the list.
 * @param acc Pointer to mongory_table_kv_context.
 * @return true to continue search, false if key found.
 */
static inline bool mongory_table_get_on_node(mongory_table_node *node, void *acc) {
  mongory_table_kv_context *ctx = (mongory_table_kv_context *)acc;
  if (strcmp(node->key, ctx->key) == 0) {
    ctx->value = node->value; // Key found, store value.
    return false;             // Stop search.
  }
  return true; // Continue search.
}

/**
 * @brief Retrieves a value associated with a key. Implements `table->get`.
 * @param self Pointer to the mongory_table.
 * @param key The key to search for.
 * @return Pointer to the mongory_value, or NULL if not found.
 */
mongory_value *mongory_table_get(mongory_table *self, char *key) {
  mongory_table_internal *internal = (mongory_table_internal *)self;
  mongory_array *bucket_array = internal->array;
  size_t index = hash_string(key) % internal->capacity;

  mongory_table_node *bucket_head = (mongory_table_node *)bucket_array->get(bucket_array, index);

  mongory_table_kv_context ctx = {key, NULL};
  mongory_table_node_walk(bucket_head, &ctx, mongory_table_get_on_node);
  return ctx.value; // NULL if not found, otherwise the value.
}

/**
 * @brief Callback for mongory_table_set to update a value if key exists in a
 * bucket's list.
 * If node's key matches ctx->key, updates node's value and returns false to
 * stop.
 * @param node Current node in the list.
 * @param acc Pointer to mongory_table_kv_context.
 * @return true to continue search (if key not matched), false if key updated.
 */
static inline bool mongory_table_set_on_node(mongory_table_node *node, void *acc) {
  mongory_table_kv_context *ctx = (mongory_table_kv_context *)acc;
  if (strcmp(node->key, ctx->key) == 0) {
    node->value = ctx->value; // Key found, update value.
    return false;             // Stop search, indicates update happened.
  }
  return true; // Continue search.
}

/**
 * @brief Sets (adds or updates) a key-value pair in the table. Implements
 * `table->set`.
 * If key exists, its value is updated. Otherwise, a new node is created and
 * added. Triggers rehashing if load factor is exceeded after adding.
 * @param self Pointer to the mongory_table.
 * @param key The key to set. A copy of this key will be made.
 * @param value The value to associate with the key.
 * @return true if successful, false on failure (e.g., memory allocation).
 */
bool mongory_table_set(mongory_table *self, char *key, mongory_value *value) {
  mongory_table_internal *internal = (mongory_table_internal *)self;
  mongory_array *bucket_array = internal->array;
  size_t index = hash_string(key) % internal->capacity;

  mongory_table_node *bucket_head = (mongory_table_node *)bucket_array->get(bucket_array, index);

  mongory_table_kv_context ctx = {key, value};
  // Try to find and update existing key. If mongory_table_node_walk returns
  // false, it means set_on_node found the key and updated it.
  if (!mongory_table_node_walk(bucket_head, &ctx, mongory_table_set_on_node)) {
    return true; // Update successful.
  }

  // Key not found, create a new node and prepend it to the bucket list.
  mongory_table_node *new_node = mongory_table_node_new(self);
  if (!new_node) {
    self->pool->error = &MONGORY_ALLOC_ERROR;
    return false; // Node allocation failed.
  }

  char *key_copy = mongory_string_cpy(self->pool, key);
  if (!key_copy) {
    self->pool->error = &MONGORY_ALLOC_ERROR;
    return false; // Key copy failed.
  }

  new_node->key = key_copy;
  new_node->value = value;
  new_node->next = bucket_head; // Prepend to list.
  bucket_array->set(bucket_array, index, (mongory_value *)new_node);

  self->count++;
  // Check load factor and rehash if necessary.
  if (self->count > internal->capacity * MONGORY_TABLE_LOAD_FACTOR) {
    if (!mongory_table_rehash(self)) {
      // Rehashing failed. The table will still work, but its performance
      // may be degraded due to a higher-than-optimal load factor.
      self->pool->error = &MONGORY_ALLOC_ERROR;
    }
  }
  return true;
}

/**
 * @brief Deletes a key-value pair from the table. Implements `table->del`.
 * @param self Pointer to the mongory_table.
 * @param key The key to delete.
 * @return true if key was found and deleted, false otherwise.
 */
bool mongory_table_del(mongory_table *self, char *key) {
  mongory_table_internal *internal = (mongory_table_internal *)self;
  mongory_array *bucket_array = internal->array;
  size_t index = hash_string(key) % internal->capacity;

  mongory_table_node *node = (mongory_table_node *)bucket_array->get(bucket_array, index);
  mongory_table_node *prev = NULL;

  while (node) {
    if (strcmp(node->key, key) == 0) {
      if (prev) {
        prev->next = node->next; // Unlink from middle/end of list.
      } else {
        // Node is the head of the list for this bucket.
        bucket_array->set(bucket_array, index, (mongory_value *)node->next);
      }
      // The memory for the node and its key is not freed here.
      // It was allocated from the memory pool and will be reclaimed all at
      // once when the pool is destroyed. This is a core design principle
      // of the library's memory management.
      self->count--;
      return true; // Deletion successful.
    }
    prev = node;
    node = node->next;
  }
  return false; // Key not found.
}

/**
 * @struct mongory_table_each_pair_context
 * @brief Context for iterating over table key-value pairs.
 */
typedef struct mongory_table_each_pair_context {
  void *acc;                                      /**< User-provided accumulator. */
  mongory_table_each_pair_callback_func callback; /**< User callback function. */
} mongory_table_each_pair_context;

/**
 * @brief Callback for mongory_table_each_pair, applied to each node in a
 * bucket list. Invokes the user's callback.
 * @param node Current table node.
 * @param acc Pointer to mongory_table_each_pair_context.
 * @return Result of the user's callback.
 */
static inline bool mongory_table_each_pair_on_node(mongory_table_node *node, void *acc) {
  mongory_table_each_pair_context *ctx = (mongory_table_each_pair_context *)acc;
  return ctx->callback(node->key, node->value, ctx->acc);
}

/**
 * @brief Callback for iterating through the table's underlying array of buckets.
 * For each non-empty bucket (which is a mongory_table_node* cast to
 * mongory_value*), it walks that bucket's linked list.
 * @param value The head of a bucket's linked list (cast to mongory_value*).
 * @param acc Pointer to mongory_table_each_pair_context.
 * @return Result of mongory_table_node_walk on the bucket list.
 */
static inline bool mongory_table_each_pair_on_root(mongory_value *value, void *acc) {
  mongory_table_node *node_head = (mongory_table_node *)value;
  // If bucket is empty (value is NULL from array->get), this will do nothing.
  return mongory_table_node_walk(node_head, acc, mongory_table_each_pair_on_node);
}

/**
 * @brief Iterates over all key-value pairs in the table. Implements
 * `table->each`.
 * @param self Pointer to the mongory_table.
 * @param acc Accumulator/context for the user callback.
 * @param callback User function to call for each pair.
 * @return true if iteration completed, false if callback stopped it.
 */
bool mongory_table_each_pair(mongory_table *self, void *acc, mongory_table_each_pair_callback_func callback) {
  mongory_table_internal *internal = (mongory_table_internal *)self;
  mongory_table_each_pair_context each_ctx = {acc, callback};
  // Iterate over the array of buckets. Each element in this array is the head
  // of a linked list of table nodes (or NULL if bucket is empty).
  return internal->array->each(internal->array, &each_ctx, mongory_table_each_pair_on_root);
}

/**
 * @brief Creates and initializes a new mongory_table.
 * Allocates the table structure, its internal array for buckets, and sets up
 * function pointers.
 * @param pool The memory pool to use for all allocations.
 * @return Pointer to the new mongory_table, or NULL on failure.
 */
mongory_table *mongory_table_new(mongory_memory_pool *pool) {
  if (!pool)
    return NULL; // Must have a valid pool.

  size_t init_capacity = MONGORY_TABLE_INIT_SIZE;
  mongory_array *bucket_array = mongory_array_new(pool);

  // Initialize the bucket array. We resize it to the initial capacity and
  // then set the last element to NULL. The `mongory_array_set` function
  // will automatically fill all intermediate slots with NULL if the index is
  // out of bounds, which perfectly initializes our bucket array.
  bool array_init_success = bucket_array && mongory_array_resize(bucket_array, init_capacity) &&
                            bucket_array->set(bucket_array, init_capacity - 1, NULL);

  if (!array_init_success) {
    // If array initialization fails, we cannot proceed.
    // The memory for `bucket_array` itself (if allocated) will be handled
    // by the memory pool when it's eventually freed.
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }
  // After initialization, bucket_array->count will equal init_capacity.
  // This is correct for the array's state, but the table's logical count is 0.
  // We leave array->count as is, since the table's `each` function iterates
  // over the array's full capacity.
  bucket_array->count = init_capacity;

  mongory_table_internal *internal = MG_ALLOC_PTR(pool, mongory_table_internal);
  if (!internal) {
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }

  // Initialize public part (base)
  internal->base.pool = pool;
  internal->base.count = 0; // Table is initially empty.
  internal->base.each = mongory_table_each_pair;
  internal->base.get = mongory_table_get;
  internal->base.set = mongory_table_set;
  internal->base.del = mongory_table_del;

  // Initialize private part
  internal->array = bucket_array;
  internal->capacity = init_capacity;

  return &internal->base; // Return pointer to the public structure.
}

mongory_table *mongory_table_nested_wrap(mongory_memory_pool *pool, int argc, ...) {
  mongory_table *table = mongory_table_new(pool);
  if (!table) {
    return NULL;
  }
  va_list args;
  va_start(args, argc);
  for (int i = 0; i < argc; i++) {
    char *key = va_arg(args, char *);
    mongory_value *value = va_arg(args, mongory_value *);
    mongory_table_set(table, key, value);
  }
  va_end(args);
  return table;
}

static inline bool mongory_table_merge_cb(char *key, mongory_value *value, void *acc) {
  mongory_table *table = (mongory_table *)acc;
  mongory_table_set(table, key, value);
  return true;
}

mongory_table *mongory_table_merge(mongory_table *table, mongory_table *other) {
  mongory_table_each_pair(other, table, mongory_table_merge_cb);
  return table;
}