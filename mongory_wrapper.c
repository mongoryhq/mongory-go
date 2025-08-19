#include "mongory_wrapper.h"

bool go_mongory_table_set(mongory_table* t, char* key, mongory_value* v) {
	return t->set(t, key, v);
}

mongory_value* go_mongory_table_get(mongory_table* t, char* key) {
	return t->get(t, key);
}

bool go_mongory_table_delete(mongory_table* t, char* key) {
	return t->del(t, key);
}

void go_mongory_memory_pool_free(mongory_memory_pool* pool) {
	pool->free(pool);
}


