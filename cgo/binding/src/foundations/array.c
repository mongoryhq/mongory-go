/**
 * @file array.c
 * @brief Implements the mongory_array dynamic array.
 *
 * This file contains the internal implementation details for the mongory_array,
 * including memory management, resizing, and the core array operations.
 * It uses a private structure `mongory_array_private` which extends
 * `mongory_array` with capacity information and the actual item storage.
 */

#include <string.h>
#include "array_private.h"
#include <mongory-core/foundations/array.h>
#include <mongory-core/foundations/memory_pool.h>
#include <stdarg.h>
#include <mongory-core/foundations/value.h>

/**
 * @def MONGORY_ARRAY_INIT_SIZE
 * @brief Initial capacity of a newly created mongory_array.
 */
#define MONGORY_ARRAY_INIT_SIZE 4

/**
 * @brief Resizes the internal storage of the array.
 *
 * Allocates a new block of memory for items and copies existing items to it.
 * The old item storage is not explicitly freed here as it's managed by the
 * memory pool; this function assumes the new allocation replaces the old one
 * in terms of where `internal->items` points.
 *
 * @param self Pointer to the mongory_array instance.
 * @param size The new capacity (number of elements) for the array.
 * @return true if resizing was successful, false otherwise (e.g., memory
 * allocation failure).
 */
bool mongory_array_resize(mongory_array *self, size_t size) {
  mongory_array_private *internal = (mongory_array_private *)self;

  // Allocate a new, larger block of memory for the items.
  mongory_value **new_items = MG_ALLOC_ARY(self->pool, mongory_value*, size);
  if (!new_items) {
    self->pool->error = &MONGORY_ALLOC_ERROR;
    return false;
  }

  // Copy existing item pointers to the new memory block.
  memcpy(new_items, internal->items, sizeof(mongory_value *) * self->count);

  internal->items = new_items;
  internal->capacity = size;
  return true;
}

/**
 * @brief Ensures the array has enough capacity for a given size, growing it if
 * necessary.
 *
 * If the required size is smaller than the current capacity, this function does
 * nothing. Otherwise, it calculates a new capacity (typically double the
 * current, or more if needed to fit `size`) and calls mongory_array_resize.
 *
 * @param self Pointer to the mongory_array instance.
 * @param size The minimum number of elements the array needs to accommodate.
 * @return true if the array has sufficient capacity (or was successfully
 * grown), false otherwise.
 */
// ============================================================================
// Static Helper Functions
// ============================================================================
static inline bool mongory_array_grow_if_needed(mongory_array *self, size_t size) {
  mongory_array_private *internal = (mongory_array_private *)self;
  if (size < internal->capacity) {
    return true;
  }

  // Calculate new capacity, typically doubling, until it's sufficient.
  size_t new_capacity = internal->capacity * 2;
  while (new_capacity <= size) {
    new_capacity *= 2;
  }

  return mongory_array_resize(self, new_capacity);
}

/**
 * @brief Internal implementation for iterating over array elements.
 * @param self Pointer to the mongory_array instance.
 * @param acc Accumulator/context passed to the callback.
 * @param func Callback function executed for each item.
 * @return true if iteration completed, false if callback stopped it.
 */
static inline bool mongory_array_each(mongory_array *self, void *acc, mongory_array_callback_func func) {
  mongory_array_private *internal = (mongory_array_private *)self;
  for (size_t i = 0; i < self->count; i++) {
    mongory_value *item = internal->items[i];
    if (!func(item, acc)) {
      return false; // Callback requested to stop iteration.
    }
  }
  return true;
}

/**
 * @brief Internal implementation for adding an element to the end of the array.
 * Grows the array if necessary.
 * @param self Pointer to the mongory_array instance.
 * @param value The mongory_value to add.
 * @return true if successful, false on failure (e.g., memory allocation).
 */
static inline bool mongory_array_push(mongory_array *self, mongory_value *value) {
  mongory_array_private *internal = (mongory_array_private *)self;
  // Ensure there's space for one more element (current count).
  if (!mongory_array_grow_if_needed(self, self->count)) {
    return false;
  }

  internal->items[self->count++] = value;
  return true;
}

/**
 * @brief Internal implementation for retrieving an element by index.
 * @param self Pointer to the mongory_array instance.
 * @param index The index of the element.
 * @return Pointer to the mongory_value, or NULL if index is out of bounds.
 */
static inline mongory_value *mongory_array_get(mongory_array *self, size_t index) {
  mongory_array_private *internal = (mongory_array_private *)self;
  if (index >= self->count) {
    return NULL; // Index out of bounds.
  }
  return internal->items[index];
}

/**
 * @brief Internal implementation for setting an element at a specific index.
 *
 * Grows the array if necessary. If the index is beyond the current count,
 * intermediate elements are set to NULL, and the array's count is updated.
 *
 * @param self Pointer to the mongory_array instance.
 * @param index The index at which to set the value.
 * @param value The mongory_value to set.
 * @return true if successful, false on failure (e.g., memory allocation).
 */
static inline bool mongory_array_set(mongory_array *self, size_t index, mongory_value *value) {
  mongory_array_private *internal = (mongory_array_private *)self;
  // Ensure capacity for the given index (index + 1 elements).
  if (!mongory_array_grow_if_needed(self, index + 1)) {
    return false;
  }

  // If setting beyond the current count, fill intermediate spots with NULL.
  if (index >= self->count) {
    for (size_t i = self->count; i < index; i++) {
      internal->items[i] = NULL;
    }
    self->count = index + 1; // Update count to include the new element.
  }

  internal->items[index] = value;
  return true;
}

/**
 * @brief Creates and initializes a new mongory_array.
 *
 * Allocates memory for the array structure itself and its initial item storage
 * using the provided memory pool. Assigns function pointers for array
 * operations.
 *
 * @param pool The memory pool to use for allocations.
 * @return Pointer to the new mongory_array, or NULL on allocation failure.
 */
mongory_array *mongory_array_new(mongory_memory_pool *pool) {
  // First, allocate the private structure that holds all array metadata.
  mongory_array_private *internal = MG_ALLOC_PTR(pool, mongory_array_private);
  if (!internal) {
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }

  // Then, allocate the initial block of memory for the array items.
  mongory_value **items = MG_ALLOC_ARY(pool, mongory_value*, MONGORY_ARRAY_INIT_SIZE);
  if (!items) {
    // If this fails, the 'internal' struct allocated above will be cleaned up
    // by the pool, assuming it's a tracing pool.
    pool->error = &MONGORY_ALLOC_ERROR;
    return NULL;
  }

  // Initialize the public part of the array structure.
  internal->base.pool = pool;
  internal->base.count = 0;
  internal->base.each = mongory_array_each;
  internal->base.get = mongory_array_get;
  internal->base.push = mongory_array_push;
  internal->base.set = mongory_array_set;

  // Initialize the private part of the array structure.
  internal->items = items;
  internal->capacity = MONGORY_ARRAY_INIT_SIZE;

  return &internal->base; // Return pointer to the public structure.
}

/**
 * @brief Creates a new mongory_array instance with a nested array.
 *
 * This function is used to create a new mongory_array instance with a nested
 * array. The nested array is created with the given memory pool and the given
 * values. The nested array is returned as a pointer to the mongory_array structure.
 *
 * @param pool A pointer to the mongory_memory_pool to be used for allocations.
 * @param argc The number of values to be set.
 * @param ... The values to be set.
 * @return mongory_array* A pointer to the newly created mongory_array, or NULL
 * if creation fails (e.g., memory allocation failure).
 */
mongory_array *mongory_array_nested_wrap(mongory_memory_pool *pool, int argc, ...) {
  mongory_array *array = mongory_array_new(pool);
  if (!array) {
    return NULL;
  }
  va_list args;
  va_start(args, argc);
  for (int i = 0; i < argc; i++) {
    mongory_value *value = va_arg(args, mongory_value *);
    array->push(array, value);
  }
  va_end(args);
  return array;
}

typedef size_t(*mongory_array_sort_cb)(mongory_value *value, void *ctx);

static mongory_value** mongory_array_merge_sort_finalize(mongory_memory_pool *pool, mongory_value **left, mongory_value **right, size_t count, void *ctx, mongory_array_sort_cb callback) {
  mongory_value **result = MG_ALLOC_ARY(pool, mongory_value*, count);
  size_t i = 0, j = 0, k = 0;
  while (i < count / 2 && j < count - count / 2) {
    if (callback(left[i], ctx) < callback(right[j], ctx)) {
      result[k++] = left[i++];
    } else {
      result[k++] = right[j++];
    }
  }
  while (i < count / 2) {
    result[k++] = left[i++];
  }
  while (j < count - count / 2) {
    result[k++] = right[j++];
  }
  return result;
}

static mongory_value** mongory_array_merge_sort(mongory_memory_pool *pool, mongory_value **origin_items, size_t count, void *ctx, mongory_array_sort_cb callback) {
  if (count <= 1) {
    return origin_items;
  }
  size_t mid = count / 2;
  mongory_value **left = mongory_array_merge_sort(pool, origin_items, mid, ctx, callback);
  mongory_value **right = mongory_array_merge_sort(pool, origin_items + mid, count - mid, ctx, callback);
  return mongory_array_merge_sort_finalize(pool, left, right, count, ctx, callback);
}

mongory_array *mongory_array_sort_by(mongory_array *self, mongory_memory_pool *temp_pool, void *ctx, mongory_array_sort_cb callback) {
  mongory_array_private *internal = (mongory_array_private *)self;

  mongory_value **new_items = mongory_array_merge_sort(temp_pool, internal->items, self->count, ctx, callback);
  mongory_array_private *new_array = (mongory_array_private *)mongory_array_new(self->pool);
  new_array->capacity = internal->capacity;
  new_array->base.count = self->count;
  new_array->items = MG_ALLOC_ARY(self->pool, mongory_value*, new_array->capacity);
  memcpy(new_array->items, new_items, sizeof(mongory_value *) * self->count);
  return &new_array->base;
}

bool mongory_array_includes(mongory_array *self, mongory_value *value) {
  for (size_t i = 0; i < self->count; i++) {
    mongory_value *item = self->get(self, i);
    if (item->comp(item, value) == 0) {
      return true;
    }
  }
  return false;
}