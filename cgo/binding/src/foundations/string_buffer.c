#include "string_buffer.h"
#include <mongory-core.h>
#include <stdarg.h>
#include <stdio.h>
#include <string.h>

#define MONGORY_STRING_BUFFER_INITIAL_CAPACITY 16

mongory_string_buffer *mongory_string_buffer_new(mongory_memory_pool *pool) {
  mongory_string_buffer *buffer = MG_ALLOC_PTR(pool, mongory_string_buffer);
  buffer->pool = pool;
  buffer->buffer = (char *)MG_ALLOC(pool, MONGORY_STRING_BUFFER_INITIAL_CAPACITY);
  buffer->size = 0;
  buffer->capacity = MONGORY_STRING_BUFFER_INITIAL_CAPACITY;
  buffer->buffer[0] = '\0';
  return buffer;
}

void mongory_string_buffer_free(mongory_string_buffer *buffer) { buffer->pool->free(buffer->pool->ctx); }

static inline void mongory_string_buffer_grow(mongory_string_buffer *buffer, size_t additional_size) {
  bool trigger_grow = false;
  while (buffer->size + additional_size + 1 > buffer->capacity) {
    buffer->capacity *= 2;
    trigger_grow = true;
  }
  if (!trigger_grow) {
    return;
  }
  char *previous_buffer = buffer->buffer;
  buffer->buffer = (char *)MG_ALLOC(buffer->pool, buffer->capacity);
  if (previous_buffer) {
    strcpy(buffer->buffer, previous_buffer);
  }
}

void mongory_string_buffer_append(mongory_string_buffer *buffer, const char *str) {
  size_t len = strlen(str);
  mongory_string_buffer_grow(buffer, len);
  strcpy(buffer->buffer + buffer->size, str);
  buffer->size += len;
  buffer->buffer[buffer->size] = '\0';
}

void mongory_string_buffer_appendf(mongory_string_buffer *buffer, const char *format, ...) {
  va_list args;
  va_start(args, format);
  int len = vsnprintf(NULL, 0, format, args);
  va_end(args);
  mongory_string_buffer_grow(buffer, len);
  va_start(args, format);
  vsnprintf(buffer->buffer + buffer->size, buffer->capacity - buffer->size, format, args);
  va_end(args);
  buffer->size += len;
  buffer->buffer[buffer->size] = '\0';
}

char *mongory_string_buffer_cstr(mongory_string_buffer *buffer) { return buffer->buffer; }

void mongory_string_buffer_clear(mongory_string_buffer *buffer) {
  buffer->buffer = (char *)MG_ALLOC(buffer->pool, MONGORY_STRING_BUFFER_INITIAL_CAPACITY);
  buffer->capacity = MONGORY_STRING_BUFFER_INITIAL_CAPACITY;
  buffer->size = 0;
  buffer->buffer[0] = '\0';
}
