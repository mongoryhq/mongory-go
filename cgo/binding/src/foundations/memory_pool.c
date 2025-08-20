/**
 * @file memory_pool.c
 * @brief Implements a memory pool for the Mongory library.
 *
 * This implementation uses a linked list of memory chunks. Allocations are
 * made from the current chunk. If the current chunk is full, a new, larger
 * chunk is allocated and added to the list. Freeing the pool deallocates all
 * chunks. It also supports tracing externally allocated memory.
 */
#include <mongory-core/foundations/memory_pool.h>
#include <stdio.h>  // For NULL, though stdlib.h or stddef.h is more common
#include <stdlib.h> // For calloc, free, etc.
#include <string.h> // For memset

/**
 * @def MONGORY_INITIAL_CHUNK_SIZE
 * @brief The initial size in bytes for the first memory chunk allocated by the
 * pool.
 */
#define MONGORY_INITIAL_CHUNK_SIZE 2048

/**
 * @def MONGORY_ALIGN8(size)
 * @brief Macro to align a given size to an 8-byte boundary.
 * This is useful for ensuring that memory allocations are suitably aligned
 * for various data types.
 * @param size The original size.
 * @return The size aligned up to the nearest multiple of 8.
 */
#define MONGORY_ALIGN8(size) (((size) + 7) & ~((size_t)7))

/**
 * @struct mongory_memory_node
 * @brief Represents a node in the linked list of memory chunks within the pool.
 *
 * Each node holds a pointer to a block of memory (`ptr`), its total `size`,
 * how much of it is `used`, and a pointer to the `next` node in the list.
 * This structure is also used by the tracing mechanism to track external
 * allocations.
 */
typedef struct mongory_memory_node {
  void *ptr;                        /**< Pointer to the allocated memory block. */
  size_t size;                      /**< Total size of the memory block. */
  size_t used;                      /**< Bytes used in this memory block. */
  struct mongory_memory_node *next; /**< Pointer to the next memory node. */
} mongory_memory_node;

/**
 * @struct mongory_memory_pool_ctx
 * @brief Internal context for a mongory_memory_pool.
 *
 * Stores the current `chunk_size` (which doubles on growth), pointers to
 * the `head` and `current` memory nodes for allocations, and a list (`extra`)
 * for traced external allocations.
 */
typedef struct mongory_memory_pool_ctx {
  size_t chunk_size;            /**< Current preferred size for new chunks. */
  mongory_memory_node *head;    /**< Head of the list of memory chunks. */
  mongory_memory_node *current; /**< Current chunk to allocate from. */
  mongory_memory_node *extra;   /**< Head of list for externally traced memory. */
} mongory_memory_pool_ctx;

/**
 * @brief Allocates a new memory chunk (node and its associated memory block).
 *
 * Uses `calloc` to ensure memory is zero-initialized.
 *
 * @param chunk_size The size of the memory block to allocate for this chunk.
 * @return mongory_memory_node* Pointer to the new memory node, or NULL on
 * failure.
 */
static inline mongory_memory_node *mongory_memory_chunk_new(size_t chunk_size) {
  mongory_memory_node *node = calloc(1, sizeof(mongory_memory_node));
  if (!node) {
    return NULL; // Failed to allocate node structure.
  }

  void *mem = calloc(1, chunk_size);
  if (!mem) {
    free(node);  // Clean up allocated node structure.
    return NULL; // Failed to allocate memory block for the chunk.
  }

  node->ptr = mem;
  node->size = chunk_size;
  node->used = 0;
  node->next = NULL;

  return node;
}

/**
 * @brief Grows the memory pool by adding a new, larger chunk.
 *
 * The new chunk size is at least double the previous chunk size, and large
 * enough to satisfy `request_size`. The new chunk becomes the `current` chunk.
 *
 * @param ctx Pointer to the memory pool's context.
 * @param request_size The minimum size required from the new chunk for an
 * upcoming allocation.
 * @return true if growth was successful, false otherwise.
 */
static inline bool mongory_memory_pool_grow(mongory_memory_pool_ctx *ctx, size_t request_size) {
  if (ctx->current->next) {
    ctx->current = ctx->current->next;
    return true; // Already have a next chunk.
  }
  // Double the chunk size, ensuring it's at least as large as request_size.
  ctx->chunk_size *= 2;
  while (request_size > ctx->chunk_size) {
    ctx->chunk_size *= 2;
  }

  mongory_memory_node *new_chunk = mongory_memory_chunk_new(ctx->chunk_size);
  if (!new_chunk) {
    return false; // Failed to create a new chunk.
  }

  // Link the new chunk and update the current pointer.
  ctx->current->next = new_chunk;
  ctx->current = new_chunk;

  return true;
}

/**
 * @brief Allocates memory from the pool. Implements `pool->alloc`.
 *
 * Allocates `size` bytes (aligned to 8 bytes) from the `current` chunk.
 * If the current chunk doesn't have enough space, `mongory_memory_pool_grow`
 * is called to add a new chunk.
 *
 * @param pool Pointer to the `mongory_memory_pool`.
 * @param size The number of bytes to allocate.
 * @return void* Pointer to the allocated memory, or NULL on failure.
 */
static inline void *mongory_memory_pool_alloc(mongory_memory_pool *pool, size_t size) {
  mongory_memory_pool_ctx *pool_ctx = (mongory_memory_pool_ctx *)pool->ctx;
  size = MONGORY_ALIGN8(size); // Ensure 8-byte alignment.

  size_t balance = pool_ctx->current->size - pool_ctx->current->used;
  if (size > balance) {
    // Not enough space in current chunk, try to grow.
    if (!mongory_memory_pool_grow(pool_ctx, size)) {
      return NULL; // Growth failed.
    }
    // After successful growth, pool_ctx->current points to the new chunk.
  }

  // Allocate from the current chunk.
  void *ptr = (char *)pool_ctx->current->ptr + pool_ctx->current->used;
  pool_ctx->current->used += size;

  return ptr;
}

static inline void mongory_memory_pool_reset(mongory_memory_pool *pool) {
  mongory_memory_pool_ctx *pool_ctx = (mongory_memory_pool_ctx *)pool->ctx;
  pool_ctx->current = pool_ctx->head;
  mongory_memory_node *node = pool_ctx->head;
  while (node) {
    node->used = 0;
    node = node->next;
  }
}

/**
 * @brief Frees a linked list of memory nodes.
 *
 * Iterates through the list, freeing the memory block (`node->ptr`) and then
 * the node structure itself for each node. Memory blocks are zeroed out before
 * freeing for security/safety.
 *
 * @param head Pointer to the head of the memory node list to free.
 */
static inline void mongory_memory_pool_node_list_free(mongory_memory_node *head) {
  mongory_memory_node *node = head;
  while (node) {
    mongory_memory_node *next = node->next;
    if (node->ptr) {
      memset(node->ptr, 0, node->size); // Clear memory for safety.
      free(node->ptr);                  // Free the actual memory block.
    }
    memset(node, 0,
           sizeof(mongory_memory_node)); // Clear the node structure itself.
    free(node);                          // Free the node structure.
    node = next;
  }
}

/**
 * @brief Destroys the memory pool and frees all associated memory.
 * Implements `pool->free`.
 *
 * Frees all memory chunks allocated by the pool (`ctx->head` list) and all
 * externally traced memory blocks (`ctx->extra` list). Then frees the pool
 * context and the pool structure itself.
 *
 * @param pool Pointer to the `mongory_memory_pool` to destroy.
 */
static inline void mongory_memory_pool_destroy(mongory_memory_pool *pool) {
  if (!pool)
    return;
  mongory_memory_pool_ctx *ctx = (mongory_memory_pool_ctx *)pool->ctx;

  if (ctx) {
    // Free the main list of memory chunks.
    mongory_memory_pool_node_list_free(ctx->head);
    // Free the list of externally traced memory chunks.
    mongory_memory_pool_node_list_free(ctx->extra);

    memset(ctx, 0,
           sizeof(mongory_memory_pool_ctx)); // Clear the context structure.
    free(ctx);                               // Free the context structure.
  }

  memset(pool, 0, sizeof(mongory_memory_pool)); // Clear the pool structure.
  free(pool);                                   // Free the pool structure.
}

/**
 * @brief Traces an externally allocated piece of memory. Implements
 * `pool->trace`.
 *
 * Creates a new `mongory_memory_node` to track the external memory block and
 * adds it to the `extra` list in the pool's context. This memory will be
 * freed when `mongory_memory_pool_destroy` is called if this trace function
 * is used.
 *
 * @param pool Pointer to the `mongory_memory_pool`.
 * @param ptr Pointer to the externally allocated memory.
 * @param size Size of the externally allocated memory.
 */
static inline void mongory_memory_pool_trace(mongory_memory_pool *pool, void *ptr, size_t size) {
  mongory_memory_pool_ctx *pool_ctx = (mongory_memory_pool_ctx *)pool->ctx;

  // Create a new node to trace this external allocation.
  mongory_memory_node *extra_alloc_tracer = calloc(1, sizeof(mongory_memory_node));
  if (!extra_alloc_tracer) {
    // Allocation of tracer node failed; cannot trace.
    // This might lead to a leak of 'ptr' if the caller expects the pool to
    // manage it. Consider error reporting.
    return;
  }
  extra_alloc_tracer->ptr = ptr;              // This is the externally allocated memory.
  extra_alloc_tracer->size = size;            // Its size.
  extra_alloc_tracer->used = size;            // Mark as fully "used" in context of tracing.
  extra_alloc_tracer->next = pool_ctx->extra; // Prepend to extra list.
  pool_ctx->extra = extra_alloc_tracer;
}

/**
 * @brief Creates and initializes a new memory pool.
 *
 * Allocates the `mongory_memory_pool` structure, its internal
 * `mongory_memory_pool_ctx`, and the first memory chunk. Sets up the function
 * pointers for `alloc`, `free`, and `trace`.
 *
 * @return mongory_memory_pool* Pointer to the new pool, or NULL on failure.
 */
mongory_memory_pool *mongory_memory_pool_new() {
  // Allocate the main pool structure.
  mongory_memory_pool *pool = calloc(1, sizeof(mongory_memory_pool));
  if (!pool) {
    return NULL;
  }

  // Allocate the internal context for the pool.
  mongory_memory_pool_ctx *ctx = calloc(1, sizeof(mongory_memory_pool_ctx));
  if (!ctx) {
    free(pool); // Clean up partially allocated pool.
    return NULL;
  }

  // Allocate the first memory chunk.
  mongory_memory_node *first_chunk = mongory_memory_chunk_new(MONGORY_INITIAL_CHUNK_SIZE);
  if (!first_chunk) {
    free(ctx);  // Clean up context.
    free(pool); // Clean up pool structure.
    return NULL;
  }

  // Initialize context fields.
  ctx->chunk_size = MONGORY_INITIAL_CHUNK_SIZE;
  ctx->head = first_chunk;
  ctx->current = first_chunk;
  ctx->extra = NULL; // No extra traced allocations initially.

  // Initialize pool fields.
  pool->ctx = ctx;
  pool->alloc = mongory_memory_pool_alloc;
  pool->reset = mongory_memory_pool_reset;
  pool->free = mongory_memory_pool_destroy;
  pool->trace = mongory_memory_pool_trace;
  pool->error = NULL; // No error initially.

  return pool;
}
