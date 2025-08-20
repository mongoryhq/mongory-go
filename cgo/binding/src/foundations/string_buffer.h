#include <mongory-core.h>

#ifndef MONGORY_STRING_BUFFER_H
#define MONGORY_STRING_BUFFER_H

typedef struct mongory_string_buffer {
  mongory_memory_pool *pool;
  char *buffer;
  size_t size;
  size_t capacity;
} mongory_string_buffer;

mongory_string_buffer *mongory_string_buffer_new(mongory_memory_pool *pool);

/**
 * @brief Appends a string to the buffer.
 * @param buffer The buffer to append to.
 * @param str The string to append.
 */
void mongory_string_buffer_append(mongory_string_buffer *buffer, const char *str);

/**
 * @brief Appends a formatted string to the buffer.
 * @param buffer The buffer to append to.
 * @param format The format string.
 * @param ... The arguments to the format string.
 */
void mongory_string_buffer_appendf(mongory_string_buffer *buffer, const char *format, ...);

/**
 * @brief Returns a C string representation of the buffer.
 * @param buffer The buffer to get the C string from.
 * @return A C string representation of the buffer.
 */
char *mongory_string_buffer_cstr(mongory_string_buffer *buffer);

/**
 * @brief Clears the buffer.
 * @param buffer The buffer to clear.
 */
void mongory_string_buffer_clear(mongory_string_buffer *buffer);

/**
 * @brief Frees the buffer.
 * @param buffer The buffer to free.
 */
void mongory_string_buffer_free(mongory_string_buffer *buffer);

#endif
