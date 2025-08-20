#ifndef MONGORY_MEMORY_POOL
#define MONGORY_MEMORY_POOL

/**
 * @file memory_pool.h
 * @brief Defines the mongory_memory_pool structure and its associated
 * operations.
 *
 * A memory pool is used for efficient memory management within the Mongory
 * library. It allows for allocating blocks of memory that can be freed
 * all at once when the pool itself is destroyed. This can reduce the overhead
 * of individual allocations and deallocations and help prevent memory leaks.
 */

#include "mongory-core/foundations/error.h"
#include <stdbool.h>
#include <stdlib.h> // For size_t

// Forward declaration of the memory pool structure.
typedef struct mongory_memory_pool mongory_memory_pool;

#define MG_ALLOC(p, n) (p->alloc(p, n))
#define MG_ALLOC_PTR(p, t) ((t*)MG_ALLOC(p, sizeof(t)))
#define MG_ALLOC_OBJ(p, t) ((t)MG_ALLOC(p, sizeof(t)))
#define MG_ALLOC_ARY(p, t, n) ((t*)MG_ALLOC(p, sizeof(t) * (n)))
/**
 * @struct mongory_memory_pool
 * @brief Represents a memory pool for managing memory allocations.
 *
 * The pool uses a context (`ctx`) to store its internal state, which typically
 * includes a list of allocated memory chunks.
 * It provides function pointers for allocation, tracing (optional), and freeing
 * the entire pool. An error field can store information about the last error
 * encountered during pool operations.
 */
struct mongory_memory_pool {
  /**
   * @brief Allocates a block of memory from the pool.
   * @param ctx A pointer to the pool's internal context.
   * @param size The number of bytes to allocate.
   * @return void* A pointer to the allocated memory block, or NULL on failure.
   * Memory allocated this way is typically aligned.
   */
  void *(*alloc)(mongory_memory_pool *pool, size_t size);

  /**
   * @brief Traces an externally allocated memory block, associating it with the
   * pool.
   *
   * This is useful if memory is allocated outside the pool's `alloc` function
   * (e.g., by an external library) but its lifecycle should be tied to the
   * pool. When the pool is freed, traced memory blocks might also be freed
   * depending on the pool's implementation.
   *
   * @param ctx A pointer to the pool's internal context.
   * @param ptr A pointer to the memory block to trace.
   * @param size The size of the memory block.
   */
  void (*trace)(mongory_memory_pool *pool, void *ptr, size_t size);

  /**
   * @brief Resets the memory pool to its initial state.
   * @param ctx A pointer to the pool's internal context.
   */
  void (*reset)(mongory_memory_pool *pool);

  /**
   * @brief Frees the entire memory pool, including all memory blocks allocated
   * from it and any traced memory blocks (depending on implementation).
   * @param self A pointer to the mongory_memory_pool instance to be freed.
   */
  void (*free)(mongory_memory_pool *self);

  void *ctx;            /**< Pointer to the internal context/state of the memory
                           pool. This is managed by the pool implementation. */
  mongory_error *error; /**< Pointer to a mongory_error structure. If an
                           operation fails (e.g., allocation), this may be set
                           to describe the error. The memory for this error
                           struct itself is typically allocated from the pool or
                           is a static error object. */
};

/**
 * @brief Creates a new mongory_memory_pool instance.
 *
 * Initializes the pool structure and its internal context, preparing it for
 * allocations.
 *
 * @return mongory_memory_pool* A pointer to the newly created memory pool, or
 * NULL if creation fails (e.g., initial memory allocation for the pool's
 * context failed).
 */
mongory_memory_pool *mongory_memory_pool_new();

#endif /* MONGORY_MEMORY_POOL */
