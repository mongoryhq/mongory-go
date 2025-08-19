#pragma once
#include <stdbool.h>
#include "mongory-core/include/mongory-core.h"

#ifdef __cplusplus
extern "C" {
#endif

bool go_mongory_table_set(mongory_table* t, char* key, mongory_value* v);
mongory_value* go_mongory_table_get(mongory_table* t, char* key);
bool go_mongory_table_delete(mongory_table* t, char* key);
void go_mongory_memory_pool_free(mongory_memory_pool* pool);

#ifdef __cplusplus
}
#endif


