#ifndef MONGORY_ARRAY_PRIVATE_H
#define MONGORY_ARRAY_PRIVATE_H
#include "mongory-core/foundations/array.h"
#include "mongory-core/foundations/memory_pool.h"
#include "mongory-core/foundations/value.h"
#include <stdbool.h>

bool mongory_array_resize(mongory_array *self, size_t size); // resize array

typedef struct mongory_array_private {
  mongory_array base;    // public array
  mongory_value **items; // items pointer
  size_t capacity;       // capacity
} mongory_array_private;

mongory_array *mongory_array_sort_by(mongory_array *self, mongory_memory_pool *temp_pool, void *ctx, size_t(*callback)(mongory_value *value, void *ctx));
bool mongory_array_includes(mongory_array *self, mongory_value *value);

#endif // MONGORY_ARRAY_PRIVATE_H
