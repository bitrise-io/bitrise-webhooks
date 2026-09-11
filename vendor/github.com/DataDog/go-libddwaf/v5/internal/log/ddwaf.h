// Unless explicitly stated otherwise all files in this repository are
// dual-licensed under the Apache-2.0 License or BSD-3-Clause License.
//
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2021 Datadog, Inc.

#ifndef DDWAF_H
#define DDWAF_H

#ifdef __cplusplus
#include <cstddef>

namespace ddwaf{
class waf;
class context;
class subcontext;
class waf_builder;
} // namespace ddwaf

using ddwaf_handle = ddwaf::waf *;
using ddwaf_context = ddwaf::context *;
using ddwaf_subcontext = ddwaf::subcontext *;
using ddwaf_builder = ddwaf::waf_builder *;
using ddwaf_allocator = void *;
using ddwaf_alloc_fn_type = void *(*)(void *, size_t, size_t);
using ddwaf_free_fn_type = void (*)(void *, void *, size_t, size_t);
using ddwaf_udata_free_fn_type = void (*)(void *);

extern "C"
{
#endif

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>

/**
 * @enum DDWAF_OBJ_TYPE
 *
 * Specifies the type of a ddwaf::object.
 **/
typedef enum
{
    /** Unkmown or uninitialised type **/
    DDWAF_OBJ_INVALID  = 0,
    /** Null type, only used for its semantical value **/
    DDWAF_OBJ_NULL     = 0x01,
    /** Boolean type **/
    DDWAF_OBJ_BOOL     = 0x02,
    /** 64-bit signed integer type **/
    DDWAF_OBJ_SIGNED   = 0x04,
    /** 64-bit unsigned integer type **/
    DDWAF_OBJ_UNSIGNED = 0x06,
    /** 64-bit float (or double) type **/
    DDWAF_OBJ_FLOAT    = 0x08,
    /** Dynamic UTF-8 string of up to max(uint32) length **/
    DDWAF_OBJ_STRING   = 0x10,
    /** Literal UTF-8 string of up to max(uint32) length, these are never freed **/
    DDWAF_OBJ_LITERAL_STRING   = 0x12,
    /** UTF-8 string of up to 14 bytes in length **/
    DDWAF_OBJ_SMALL_STRING   = 0x14,
    /** Array of ddwaf_object, up to max(uint16) capacity **/
    DDWAF_OBJ_ARRAY    = 0x20,
    /** Array of ddwaf_object_kv, up to max(uint16) capacity **/
    DDWAF_OBJ_MAP      = 0x40,
} DDWAF_OBJ_TYPE;

/**
 * @enum DDWAF_RET_CODE
 *
 * Codes returned by ddwaf_context_eval.
 **/
typedef enum
{
    /** Unknown error, typically due to an unexpected exception **/
    DDWAF_ERR_INTERNAL          = -3,
    /** The provided data object didn't match the expected schema **/
    DDWAF_ERR_INVALID_OBJECT    = -2,
    /** One or more of the provided arguments to a function is invalid **/
    DDWAF_ERR_INVALID_ARGUMENT  = -1,
    /** The data evaluation didn't yield any events, attributes, etc **/
    DDWAF_OK                    = 0,
    /** The data evaluation resulted in an event, attribute, etc **/
    DDWAF_MATCH                 = 1,
} DDWAF_RET_CODE;

/**
 * @enum DDWAF_LOG_LEVEL
 *
 * Internal WAF log levels, to be used when setting the minimum log level and cb.
 **/
typedef enum
{
    /** Finest-grained logging for detailed tracing */
    DDWAF_LOG_TRACE = 0,
    /** Debugging information for development */
    DDWAF_LOG_DEBUG = 1,
    /** General informational messages */
    DDWAF_LOG_INFO = 2,
    /** Warning messages for potential issues */
    DDWAF_LOG_WARN = 3,
    /** Error messages for failures */
    DDWAF_LOG_ERROR = 4,
    /** Disable all logging */
    DDWAF_LOG_OFF = 5,
} DDWAF_LOG_LEVEL;

#ifndef __cplusplus
typedef struct _ddwaf_handle* ddwaf_handle;
typedef struct _ddwaf_context* ddwaf_context;
typedef struct _ddwaf_subcontext* ddwaf_subcontext;
typedef struct _ddwaf_builder* ddwaf_builder;
typedef struct _ddwaf_allocator* ddwaf_allocator;

typedef void *(ddwaf_alloc_fn_type)(void *, size_t, size_t);
typedef void (ddwaf_free_fn_type)(void *, void *, size_t, size_t);
typedef void (ddwaf_udata_free_fn_type)(void *);
#endif

typedef union _ddwaf_object ddwaf_object;
typedef struct _ddwaf_object_kv ddwaf_object_kv;

struct _ddwaf_object_bool {
    uint8_t type;
    bool val;
};

struct _ddwaf_object_signed {
    uint8_t type;
    int64_t val;
};

struct _ddwaf_object_unsigned {
    uint8_t type;
    uint64_t val;
};

struct _ddwaf_object_float {
    uint8_t type;
    double val;
};

struct _ddwaf_object_string {
    uint8_t type;
    uint32_t size;
    char *ptr;
};

struct _ddwaf_object_small_string {
#define DDWAF_OBJ_SSTR_SIZE 14
    uint8_t type;
    uint8_t size;
    char data[DDWAF_OBJ_SSTR_SIZE];
};

struct _ddwaf_object_array {
    uint8_t type;
    uint16_t size;
    uint16_t capacity;
    union _ddwaf_object *ptr;
};

struct _ddwaf_object_map {
    uint8_t type;
    uint16_t size;
    uint16_t capacity;
    struct _ddwaf_object_kv *ptr;
};

/**
 * @struct ddwaf_object
 *
 * Generic object used to pass data and rules to the WAF.
 **/
#ifdef _MSC_VER
union _ddwaf_object {
#else
union __attribute__((may_alias)) _ddwaf_object {
#endif
    uint8_t type;
    union {
        struct _ddwaf_object_bool b8;
        struct _ddwaf_object_signed i64;
        struct _ddwaf_object_unsigned u64;
        struct _ddwaf_object_float f64;
        struct _ddwaf_object_string str;
        struct _ddwaf_object_small_string sstr;
        struct _ddwaf_object_array array;
        struct _ddwaf_object_map map;
    } via;
};

struct _ddwaf_object_kv {
    union _ddwaf_object key;
    union _ddwaf_object val;
};

#if defined(_Static_assert) || defined(static_assert)
#ifndef static_assert
#define static_assert _Static_assert
#endif

static_assert(sizeof(union _ddwaf_object) == 16);
static_assert(sizeof(struct _ddwaf_object_kv) == 32);
#endif

/**
 * @typedef ddwaf_log_cb
 *
 * Callback that libddwaf will call to relay messages to the binding.
 *
 * @param level The logging level.
 * @param function The native function that emitted the message. (nonnull)
 * @param file The file of the native function that emmitted the message. (nonnull)
 * @param line The line where the message was emmitted.
 * @param message The size of the logging message. NUL-terminated
 * @param message_len The length of the logging message (excluding NUL terminator).
 */
typedef void (*ddwaf_log_cb)(
    DDWAF_LOG_LEVEL level, const char* function, const char* file, unsigned line,
    const char* message, uint64_t message_len);

/**
 * Initialize a ddwaf instance
 *
 * @param ruleset ddwaf::object map containing rules, exclusions, rules_override and rules_data. (nonnull)
 * @param diagnostics Optional ruleset parsing diagnostics. (nullable)
 *
 * @return Handle to the WAF instance or NULL on error.
 *
 * @note If ruleset is NULL, the diagnostics object will not be initialised.
 *
 * @note The deallocation of the diagnostics must be made with default allocator.
 **/
ddwaf_handle ddwaf_init(const ddwaf_object *ruleset,  ddwaf_object *diagnostics);

/**
 * Destroy a WAF instance.
 *
 * @param handle Handle to the WAF instance.
 */
void ddwaf_destroy(ddwaf_handle handle);

/**
 * Get an array of known (root) addresses used by rules, exclusion filters and
 * processors. This array contains both required and optional addresses. A more
 * accurate distinction between required and optional addresses is provided
 * within the diagnostics.
 *
 * The memory is owned by the WAF and should not be freed.
 *
 * @param handle Handle to the WAF instance.
 * @param size Output parameter in which the size will be returned. The value of
 *             size will be 0 if the return value is NULL.
 * @return NULL if empty, otherwise a pointer to an array with size elements.
 *
 * @note This function is not thread-safe
 * @note The returned array should be considered invalid after calling ddwaf_destroy
 *       on the handle used to obtain it.
 **/
const char* const* ddwaf_known_addresses(const ddwaf_handle handle, uint32_t *size);
/**
 * Get an array of all the action types which could be triggered as a result of
 * the current set of rules and exclusion filters.
 *
 * The memory is owned by the WAF and should not be freed.
 *
 * @param handle Handle to the WAF instance.
 * @param size Output parameter in which the size will be returned. The value of
 *             size will be 0 if the return value is NULL.
 * @return NULL if empty, otherwise a pointer to an array with size elements.
 *
 * @note This function is not thread-safe
 * @note The returned array should be considered invalid after calling ddwaf_destroy
 *       on the handle used to obtain it.
 **/
const char *const *ddwaf_known_actions(const ddwaf_handle handle, uint32_t *size);
/**
 * Context object to perform matching using the provided WAF instance.
 *
 * @param handle Handle of the WAF instance containing the ruleset definition. (nonnull)
 * @param output_alloc Allocator used to serve output objects created during evaluation (nonnull)

 * @return Handle to the context instance.
 *
 * @note The WAF instance needs to be valid for the lifetime of the context.
 **/
ddwaf_context ddwaf_context_init(const ddwaf_handle handle, ddwaf_allocator output_alloc);

/**
 * Perform a matching operation on the provided data
 *
 * @param context WAF context to be used in this run, this will determine the
 *                ruleset which will be used and it will also ensure that
 *                parameters are taken into account across runs (nonnull)
 *
 * @param data (nonnull) Data on which to perform the pattern matching. This
 *    data will be stored by the context and used across multiple calls to this
 *    function or ddwaf_subcontext_eval. Once the context is destroyed, the user
 *    defined allocator will be used to free the data provided. Note that the
 *    data passed must be valid until the destruction of the context. The object
 *    must be a map of {string, <value>} in which each key represents the
 *    relevant address associated to the value, which can be of an arbitrary
 *    type.
 *
 * @param alloc (nullable) Allocator used to free the data provided. If NULL,
 *              the data will not be freed.
 *
 * @param result (nullable) Object map containing the following items:
 *               - events: an array of the generated events.
 *               - actions: a map of the generated actions in the format:
 *                          "{action type: { <parameter map> }, ...}"
 *               - duration: an unsigned specifying the total runtime of the
 *                           call in nanoseconds.
 *               - timeout: whether there has been a timeout during the call.
 *               - attributes: a map containing all derived objects in the
 *                             format: {tag, value}
 *               - keep: whether the data contained herein must override any
 *                       transport sampling through the relevant mechanism.
 *               - evaluated: an unsigned integer indicating the number of input
 *                            batches that were fully evaluated. For this single
 *                            evaluation it is 1 once the batch has been
 *                            evaluated, or 0 if a timeout prevented that. See
 *                            ddwaf_context_multieval for the multi-batch case.
 *               This structure must be freed by the caller using the output
 *               allocator provided through ddwaf_context_init. The object will
 *               contain all specified keys when the value returned by
 *               ddwaf_context_eval is either DDWAF_OK or DDWAF_MATCH and will
 *               be empty otherwise.
 *               IMPORTANT: This object is not allocated with the allocator
 *               passed in this call. It uses the allocator given to
 *               ddwaf_context_init instead.
 * @param timeout Maximum time budget in microseconds.
 *
 * @return Return code of the operation.
 * @retval DDWAF_ERR_INVALID_ARGUMENT The context is invalid, the data will not
 *                                   be freed.
 * @retval DDWAF_ERR_INVALID_OBJECT The data provided didn't match the desired
 *                                 structure or contained invalid objects, the
 *                                 data will be freed by this function.
 * @retval DDWAF_ERR_INTERNAL There was an unexpected error and the operation did
 *                           not succeed. The state of the WAF is undefined if
 *                           this error is produced and the ownership of the
 *                           data is unknown. The result structure will not be
 *                           filled if this error occurs.
 *
 * Notes on addresses:
 * - Within a single run, addresses provided should be unique.
 *   If duplicate addresses are provided:
 *   - Within the same batch, the latest one in the structure will be the one
 *     used for evaluation.
 *   - Within two different batches, the second batch will only use the new data.
 **/
DDWAF_RET_CODE ddwaf_context_eval(ddwaf_context context, ddwaf_object *data,
    ddwaf_allocator alloc, ddwaf_object *result,  uint64_t timeout);

/**
 * Perform multiple matching operations on the provided data, evaluating each
 * batch in sequence and returning a single combined result.
 *
 * This function operates identically to ddwaf_context_eval, with the
 * distinction that data must be an array of maps. Each element is treated as
 * a separate input batch and is evaluated in order. Addresses provided in
 * earlier batches persist in the store and remain available when evaluating
 * later batches, so a rule that requires multiple addresses can be satisfied
 * by data spread across different batches within a single call. The result
 * reflects all events, actions, and attributes accumulated across the entire
 * sequence of batches.
 *
 * @param context WAF context to be used in this run, this will determine the
 *                ruleset which will be used and it will also ensure that
 *                parameters are taken into account across runs. (nonnull)
 *
 * @param data (nonnull) Array of input batches to evaluate. Each element must
 *    be a map of {string, <value>} where each key represents the relevant
 *    address associated to the value, which can be of an arbitrary type. The
 *    batches are evaluated in order. If the same address appears in multiple
 *    batches, the later value replaces the earlier one. Passing a non-array
 *    object will return DDWAF_ERR_INVALID_OBJECT.
 *    Ownership and lifetime semantics are identical to ddwaf_context_eval:
 *    the data is stored by the context and the provided allocator is used to
 *    free it once the context is destroyed.
 *
 * @param alloc (nullable) Allocator used to free the data provided. If NULL,
 *              the data will not be freed.
 *
 * @param result (nullable) Object map containing the following items:
 *               - events: an array of all events generated across all batches.
 *               - actions: a map of all actions generated across all batches
 *                          in the format: "{action type: { <parameter map> }, ...}"
 *               - duration: an unsigned specifying the total runtime of the
 *                           call in nanoseconds.
 *               - timeout: whether there has been a timeout during the call.
 *               - attributes: a map containing all derived objects in the
 *                             format: {tag, value}
 *               - keep: whether the data contained herein must override any
 *                       transport sampling through the relevant mechanism.
 *               - evaluated: an unsigned integer indicating the number of input
 *                            batches that were fully evaluated. In the normal
 *                            case this equals the total number of batches
 *                            provided; it is lower when evaluation stops early,
 *                            e.g. on a timeout or once a batch produces a
 *                            blocking result. Empty batches are evaluated like
 *                            any other and count towards this value.
 *               This structure must be freed by the caller using the output
 *               allocator provided through ddwaf_context_init. The object will
 *               contain all specified keys when the value returned by
 *               ddwaf_context_multieval is either DDWAF_OK or DDWAF_MATCH and
 *               will be empty otherwise.
 *               IMPORTANT: This object is not allocated with the allocator
 *               passed in this call. It uses the allocator given to
 *               ddwaf_context_init instead.
 * @param timeout Maximum time budget in microseconds.
 *
 * @return Return code of the operation.
 * @retval DDWAF_ERR_INVALID_ARGUMENT The context is invalid, the data will not
 *                                   be freed.
 * @retval DDWAF_ERR_INVALID_OBJECT The data provided didn't match the desired
 *                                 structure or contained invalid objects, the
 *                                 data will be freed by this function.
 * @retval DDWAF_ERR_INTERNAL There was an unexpected error and the operation did
 *                           not succeed. The state of the WAF is undefined if
 *                           this error is produced and the ownership of the
 *                           data is unknown. The result structure will not be
 *                           filled if this error occurs.
 *
 * Notes on addresses:
 * - Within a single batch, addresses provided should be unique.
 *   If duplicate addresses are provided within the same batch, the latest one
 *   in the structure will be used for evaluation.
 * - Addresses from earlier batches persist in the store and are accessible
 *   during evaluation of subsequent batches within the same call. If the same
 *   address appears in a later batch, the later value replaces the earlier one.
 * - A rule that requires multiple addresses can be satisfied by data spread
 *   across different batches within a single call.
 **/
DDWAF_RET_CODE ddwaf_context_multieval(ddwaf_context context, ddwaf_object *data,
    ddwaf_allocator alloc, ddwaf_object *result, uint64_t timeout);

/**
 * Performs the destruction of the context, freeing the data passed to it through
 * ddwaf_context_eval using the provided allocator during evaluation.
 *
 * @param context Context to destroy. (nonnull)
 **/
void ddwaf_context_destroy(ddwaf_context context);

/**
 * Subcontext object to perform matching using the provided WAF instance.
 *
 * @param context Context from which to derive this subcontext. (nonnull)

 * @return Handle to the subcontext instance.
 **/
ddwaf_subcontext ddwaf_subcontext_init(ddwaf_context context);

/**
 * Perform a matching operation on the provided data
 *
 * @param subcontext WAF subcontext to be used in this run, this will determine
 * the ruleset which will be used and it will also ensure that parameters are
 * taken into account across runs (nonnull)
 *
 * @param data (nonnull) Data on which to perform the pattern matching. This
 *    data will be stored by the subcontext and used across multiple calls to this
 *    function. Once the subcontext is destroyed, the user defined allocator will
 *    be used to free the data provided. Note that the data passed must be valid
 *    until the destruction of the subcontext. The object must be a map of
 *    {string, <value>} in which each key represents the  relevant address
 *    associated to the value, which can be of an arbitrary type.
 *
 * @param alloc (nullable) Allocator used to free the data provided. If NULL,
 *              the data will not be freed.
 *
 * @param result (nullable) Object map containing the following items:
 *               - events: an array of the generated events.
 *               - actions: a map of the generated actions in the format:
 *                          "{action type: { <parameter map> }, ...}"
 *               - duration: an unsigned specifying the total runtime of the
 *                           call in nanoseconds.
 *               - timeout: whether there has been a timeout during the call.
 *               - attributes: a map containing all derived objects in the
 *                             format: {tag, value}
 *               - keep: whether the data contained herein must override any
 *                       transport sampling through the relevant mechanism.
 *               - evaluated: an unsigned integer indicating the number of input
 *                            batches that were fully evaluated. For this single
 *                            evaluation it is 1 once the batch has been
 *                            evaluated, or 0 if a timeout prevented that. See
 *                            ddwaf_subcontext_multieval for the multi-batch case.
 *               This structure must be freed by the caller and will contain all
 *               specified keys when the value returned by ddwaf_subcontext_eval
 *               is either DDWAF_OK or DDWAF_MATCH and will be empty otherwise.
 *               IMPORTANT: This object is not allocated with the allocator
 *               passed in this call. It uses the allocator given to
 *               ddwaf_context_init instead.
 * @param timeout Maximum time budget in microseconds.
 *
 * @return Return code of the operation.
 * @retval DDWAF_ERR_INVALID_ARGUMENT The subcontext is invalid, the data will not
 *                                   be freed.
 * @retval DDWAF_ERR_INVALID_OBJECT The data provided didn't match the desired
 *                                 structure or contained invalid objects, the
 *                                 data will be freed by this function.
 * @retval DDWAF_ERR_INTERNAL There was an unexpected error and the operation did
 *                           not succeed. The state of the WAF is undefined if
 *                           this error is produced and the ownership of the
 *                           data is unknown. The result structure will not be
 *                           filled if this error occurs.
 *
 * Notes on addresses:
 * - Within a single run, addresses provided should be unique.
 *   If duplicate addresses are provided:
 *   - Within the same batch, the latest one in the structure will be the one
 *     used for evaluation.
 *   - Within two different batches, the second batch will only use the new data.
 **/
DDWAF_RET_CODE ddwaf_subcontext_eval(ddwaf_subcontext subcontext, ddwaf_object *data,
    ddwaf_allocator alloc, ddwaf_object *result,  uint64_t timeout);

/**
 * Perform multiple matching operations on the provided data, evaluating each
 * batch in sequence and returning a single combined result.
 *
 * This function operates identically to ddwaf_subcontext_eval, with the
 * distinction that data must be an array of maps. Each element is treated as
 * a separate input batch and is evaluated in order. Addresses provided in
 * earlier batches persist in the store and remain available when evaluating
 * later batches, so a rule that requires multiple addresses can be satisfied
 * by data spread across different batches within a single call. The result
 * reflects all events, actions, and attributes accumulated across the entire
 * sequence of batches.
 *
 * @param subcontext WAF subcontext to be used in this run, this will determine
 *                   the ruleset which will be used and it will also ensure that
 *                   parameters are taken into account across runs. (nonnull)
 *
 * @param data (nonnull) Array of input batches to evaluate. Each element must
 *    be a map of {string, <value>} where each key represents the relevant
 *    address associated to the value, which can be of an arbitrary type. The
 *    batches are evaluated in order. If the same address appears in multiple
 *    batches, the later value replaces the earlier one. Passing a non-array
 *    object will return DDWAF_ERR_INVALID_OBJECT.
 *    Ownership and lifetime semantics are identical to ddwaf_subcontext_eval:
 *    the data is stored by the subcontext and the provided allocator is used
 *    to free it once the subcontext is destroyed.
 *
 * @param alloc (nullable) Allocator used to free the data provided. If NULL,
 *              the data will not be freed.
 *
 * @param result (nullable) Object map containing the following items:
 *               - events: an array of all events generated across all batches.
 *               - actions: a map of all actions generated across all batches
 *                          in the format: "{action type: { <parameter map> }, ...}"
 *               - duration: an unsigned specifying the total runtime of the
 *                           call in nanoseconds.
 *               - timeout: whether there has been a timeout during the call.
 *               - attributes: a map containing all derived objects in the
 *                             format: {tag, value}
 *               - keep: whether the data contained herein must override any
 *                       transport sampling through the relevant mechanism.
 *               - evaluated: an unsigned integer indicating the number of input
 *                            batches that were fully evaluated. In the normal
 *                            case this equals the total number of batches
 *                            provided; it is lower when evaluation stops early,
 *                            e.g. on a timeout or once a batch produces a
 *                            blocking result. Empty batches are evaluated like
 *                            any other and count towards this value.
 *               This structure must be freed by the caller and will contain all
 *               specified keys when the value returned by
 *               ddwaf_subcontext_multieval is either DDWAF_OK or DDWAF_MATCH
 *               and will be empty otherwise.
 *               IMPORTANT: This object is not allocated with the allocator
 *               passed in this call. It uses the allocator given to
 *               ddwaf_context_init instead.
 * @param timeout Maximum time budget in microseconds.
 *
 * @return Return code of the operation.
 * @retval DDWAF_ERR_INVALID_ARGUMENT The subcontext is invalid, the data will
 *                                   not be freed.
 * @retval DDWAF_ERR_INVALID_OBJECT The data provided didn't match the desired
 *                                 structure or contained invalid objects, the
 *                                 data will be freed by this function.
 * @retval DDWAF_ERR_INTERNAL There was an unexpected error and the operation did
 *                           not succeed. The state of the WAF is undefined if
 *                           this error is produced and the ownership of the
 *                           data is unknown. The result structure will not be
 *                           filled if this error occurs.
 *
 * Notes on addresses:
 * - Within a single batch, addresses provided should be unique.
 *   If duplicate addresses are provided within the same batch, the latest one
 *   in the structure will be used for evaluation.
 * - Addresses from earlier batches persist in the store and are accessible
 *   during evaluation of subsequent batches within the same call. If the same
 *   address appears in a later batch, the later value replaces the earlier one.
 * - A rule that requires multiple addresses can be satisfied by data spread
 *   across different batches within a single call.
 **/
DDWAF_RET_CODE ddwaf_subcontext_multieval(ddwaf_subcontext subcontext, ddwaf_object *data,
    ddwaf_allocator alloc, ddwaf_object *result, uint64_t timeout);

/**
 * Performs the destruction of the subcontext, freeing the data passed to it through
 * ddwaf_subcontext_eval using the used-defined allocator.
 *
 * @param subcontext subcontext to destroy. (nonnull)
 **/
void ddwaf_subcontext_destroy(ddwaf_subcontext subcontext);


/**
 * Initialize an instace of the waf builder.
 *
 * @return Handle to the builer instance or NULL on error.
 *
 * @note If config is NULL, default values will be used
 **/
ddwaf_builder ddwaf_builder_init();

/**
 * Adds or updates a configuration based on the given path, which must be a unique
 * identifier for the provided configuration.
 *
 * @param builder Builder to perform the operation on. (nonnull)
 * @param path A string containing the path of the configuration, this must uniquely identify the configuration. (nonnull)
 * @param path_len The length of the string contained within path.
 * @param config ddwaf::object map containing rules, exclusions, rules_override and rules_data. (nonnull)
 * @param diagnostics Optional ruleset parsing diagnostics. (nullable)
 *
 * @return Whether the operation succeeded (true) or failed (false).
 *
 * @note if any of the arguments are NULL, the diagnostics object will not be initialised.
 * @note The memory associated with the path, config and diagnostics must be freed by the caller.
 * @note The deallocation of the diagnostics must be made with default allocator.
 * @note This function is not thread-safe.
 **/
bool ddwaf_builder_add_or_update_config(ddwaf_builder builder, const char *path, uint32_t path_len, const ddwaf_object *config, ddwaf_object *diagnostics);

/**
 * Removes a configuration based on the provided path.
 *
 * @param builder Builder to perform the operation on. (nonnull)
 * @param path A string containing the path of the configuration to be removed. (nonnull)
 * @param path_len The length of the string contained within path.
 *
 * @return Whether the operation succeeded (true) or failed (false).
 *
 * @note The memory associated with the path must be freed by the caller.
 * @note This function is not thread-safe.
 **/
bool ddwaf_builder_remove_config(ddwaf_builder builder, const char *path, uint32_t path_len);

/**
 * Builds a ddwaf instance based on the current set of configurations.
 *
 * @param builder Builder to perform the operation on. (nonnull)
 *
 * @return Handle to the new WAF instance or NULL if there was an error.
 *
 * @note This function is not thread-safe.
 **/
ddwaf_handle ddwaf_builder_build_instance(ddwaf_builder builder);

/**
 * Provides an array of the currently loaded paths, optionally matching the
 * regex provided in filter. In addition, the count is provided as the return
 * value, allowing paths to be nullptr.
 *
 * @param builder Builder to perform the operation on. (nonnull)
 * @param paths The object in which paths will be returned, as an array of
 *        strings. If NULL, only the count is provided. (nullable)
 * @param filter An optional string regex to filter the provided paths. The
 *        provided regular expression is used unanchored so matches can be found
 *        at any point within the path, any necessary anchors must be explicitly
 *        added to the regex. (nullable).
 * @param filter_len The length of the filter string (or 0 otherwise).
 *
 * @return The total number of configurations loaded or, if provided, the number
 *         of those matching the filter.
 *
 * @note This function is not thread-safe and the memory of the paths object must
 *       be freed by the caller using the default allocator.
 **/
uint32_t ddwaf_builder_get_config_paths(ddwaf_builder builder, ddwaf_object *paths, const char *filter, uint32_t filter_len);

/**
 * Destroy an instance of the builder.
 *
 * @param builder Builder to perform the operation on. (nonnull)
 */
void ddwaf_builder_destroy(ddwaf_builder builder);

/**
 * Returns the default allocator used by the library.
 *
 * @return Allocator handle.
 **/
ddwaf_allocator ddwaf_get_default_allocator();

/**
 * Creates a thread-safe pool allocator. Allocations are served from internal
 * pools sized by block class to reduce fragmentation and allocator overhead;
 * memory freed via the corresponding ddwaf APIs is returned to the pools for
 * reuse. This allocator can be shared across threads safely.
 *
 * Lifetime and safety:
 * - The allocator must not be destroyed while any memory obtained from it is
 *   still in use; doing so will invalidate outstanding pointers.
 *
 * @return Allocator handle.
 **/
ddwaf_allocator ddwaf_synchronized_pool_allocator_init();

/**
 * Creates a pool allocator without internal synchronization. It provides the
 * same pooling characteristics as the synchronized variant but with lower
 * overhead. This allocator must not be used concurrently from multiple threads
 * unless externally synchronized.
 *
 * Lifetime and safety:
 * - The allocator must not be destroyed while any memory obtained from it is
 *   still in use; doing so will invalidate outstanding pointers.
 *
 * @return Allocator handle.
 **/
ddwaf_allocator ddwaf_unsynchronized_pool_allocator_init();

/**
 * Creates a monotonic (growing) allocator. Allocations are fast and never freed
 * individually; all memory is reclaimed only when the allocator is destroyed.
 * This allocator must not be used concurrently from multiple threads unless
 * externally synchronized.
 *
 * Lifetime and safety:
 * - Objects allocated from this allocator remain valid until the allocator is
 *   destroyed; individual frees have no effect.
 * - The allocator must not be destroyed while any memory obtained from it is
 *   still in use; doing so will invalidate outstanding pointers.
 *
 * @return Allocator handle.
 **/
ddwaf_allocator ddwaf_monotonic_allocator_init();

/**
 * Creates an allocator that forwards allocation and deallocation to user
 * provided callbacks.
 *
 * @param alloc_fn Allocation callback. It receives the opaque `udata`, the
 *        requested `size` and `alignment`, and must return a pointer meeting the
 *        alignment requirements or NULL on failure. (nonnull)
 * @param free_fn Deallocation callback. It receives the opaque `udata`, the
 *        pointer to free, and the original `size` and `alignment`. It must be
 *        able to free any pointer previously returned by `alloc_fn`. (nonnull)
 * @param udata Opaque user pointer forwarded to both callbacks; can be used to
 *        carry custom state. (nullable)
 * @param udata_free_fn User data destruction callback, used to perform any
 *        relevant destruction and reclamation operations on the provided user
 *        data.
 *
 * @return Allocator handle.
 **/
ddwaf_allocator ddwaf_user_allocator_init(ddwaf_alloc_fn_type alloc_fn, ddwaf_free_fn_type free_fn, void *udata, ddwaf_udata_free_fn_type udata_free_fn);

/**
 * Allocates a block of memory from the given allocator with the requested
 * alignment.
 *
 * Usage and guarantees:
 * - The returned pointer is aligned to `alignment` bytes.
 * - Returns NULL on allocation failure.
 * - Memory obtained with this function must be released with
 *   ddwaf_allocator_free using the same allocator and the same `bytes` and
 *   `alignment` values.
 * - Thread-safety depends on the allocator type; see the corresponding
 *   allocator init function for details.
 *
 * @param alloc Allocator to use for the allocation. (nonnull)
 * @param bytes Number of bytes to allocate.
 * @param alignment Required alignment in bytes; must be a power of two.
 *
 * @return Pointer to the allocated memory or NULL on failure.
 **/
void *ddwaf_allocator_alloc(ddwaf_allocator alloc, size_t bytes, size_t alignment);

/**
 * Releases a block of memory previously obtained via ddwaf_allocator_alloc
 * from the same allocator.
 *
 * Requirements and safety:
 * - `p` must point to memory returned by ddwaf_allocator_alloc using `alloc`.
 * - `bytes` and `alignment` must match the values used for the allocation.
 * - After this call, the memory referenced by `p` must no longer be accessed.
 * - Do not mix allocators; freeing with a different allocator is undefined.
 * - Thread-safety depends on the allocator type; see the corresponding
 *   allocator init function for details.
 *
 * @param alloc Allocator used for the original allocation. (nonnull)
 * @param p Pointer to the memory to free. (nonnull)
 * @param bytes Size in bytes of the original allocation.
 * @param alignment Alignment in bytes of the original allocation.
 **/
void ddwaf_allocator_free(ddwaf_allocator alloc, void *p, size_t bytes, size_t alignment);


/**
 * Destroys an allocator created by one of the ddwaf_*_allocator_init functions
 * and releases any internal resources it holds.
 *
 * Safety and lifetime:
 * - It is the caller's responsibility to ensure no outstanding memory from the
 *   allocator is still in use at the time of destruction.
 * - Must not called concurrently with other operations using the same allocator.
 * - Attempting to destroy the default allocator is a no-op and has no ill-effects.
 *
 * @param alloc Allocator to destroy. (nonnull)
 **/
void ddwaf_allocator_destroy(ddwaf_allocator alloc);

/**
 * Creates an invalid object.
 *
 * @param object Object to perform the operation on. (nonnull)
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_invalid(ddwaf_object *object);

/**
 * Creates an null object. Provides a different semantical value to invalid as
 * it can be used to signify that a value is null rather than of an unknown type.
 *
 * @param object Object to perform the operation on. (nonnull)
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_null(ddwaf_object *object);

/**
 * Creates an object from a string.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param string String to initialise the object with, this string will be copied. (nonnull)
 * @param length Length of the string.
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_string(ddwaf_object *object, const char *string, uint32_t length, ddwaf_allocator alloc);

/**
 * Creates an object from a literal string and its length.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param string Literal string to initialise the object with, this string will not be copied
 *               and must remain valid for the lifetime of the object. (nonnull)
 * @param length Length of the string.
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_string_literal(ddwaf_object *object, const char *string, uint32_t length);

/**
 * Creates an object with the string pointer and length provided, without copying the string.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param string String pointer to initialise the object with, this string will not be copied
 *               and must remain valid for the lifetime of the object. (nonnull)
 * @param length Length of the string.
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 *
 * @note The provided string must have been allocated with the same allocator used
 * with ddwaf_object_destroy.
 **/
ddwaf_object* ddwaf_object_set_string_nocopy(ddwaf_object *object, const char *string, uint32_t length);
/**
 * Creates an object using an unsigned integer (64-bit). The resulting object
 * will contain an unsigned integer as opposed to a string.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param value Integer to initialise the object with.
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_unsigned(ddwaf_object *object, uint64_t value);

/**
 * Creates an object using a signed integer (64-bit). The resulting object
 * will contain a signed integer as opposed to a string.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param value Integer to initialise the object with.
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_signed(ddwaf_object *object, int64_t value);

/**
 * Creates an object using a boolean, the resulting object will contain a
 * boolean as opposed to a string.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param value Boolean to initialise the object with.
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_bool(ddwaf_object *object, bool value);

/**
 * Creates an object using a double, the resulting object will contain a
 * double as opposed to a string.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param value Double to initialise the object with.
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_float(ddwaf_object *object, double value);

/**
 * Creates an array object, for sequential storage.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param capacity Initial capacity of the array.
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_array(ddwaf_object *object, uint16_t capacity, ddwaf_allocator alloc);

/**
 * Creates a map object, for key-value storage.
 *
 * @param object Object to perform the operation on. (nonnull)
 * @param capacity Initial capacity of the map.
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the passed object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_set_map(ddwaf_object *object, uint16_t capacity, ddwaf_allocator alloc);

/**
 * Inserts a new object into an array object.
 *
 * @param array Array in which to insert the object. (nonnull)
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the newly inserted object or NULL if the operation failed.
 **/

ddwaf_object *ddwaf_object_insert(ddwaf_object *array, ddwaf_allocator alloc);

/**
 * Inserts a new object into a map object, using a key.
 *
 * @param map Map in which to insert the object. (nonnull)
 * @param key The key for indexing purposes, this string will be copied. (nonnull)
 * @param length Length of the key.
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the newly inserted object or NULL if the operation failed.
 **/
ddwaf_object *ddwaf_object_insert_key(ddwaf_object *map, const char *key, uint32_t length, ddwaf_allocator alloc);

/**
 * Inserts a new object into a map object, using a literal key.
 *
 * @param map Map in which to insert the object. (nonnull)
 * @param key The key for indexing purposes, this string will not be copied. (nonnull)
 * @param length Length of the key.
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the newly inserted object or NULL if the operation failed.
 **/
ddwaf_object *ddwaf_object_insert_literal_key(ddwaf_object *map, const char *key, uint32_t length, ddwaf_allocator alloc);


/**
 * Inserts a new object into a map object, using a key and its length, but without
 * creating a copy of the key.
 *
 * @param map Map in which to insert the object. (nonnull)
 * @param key The key for indexing purposes, this string will not be copied. (nonnull)
 * @param length Length of the key.
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the newly inserted object or NULL if the operation failed.
 *
 * @note The provided string must have been allocated with the same allocator used
 * with ddwaf_object_destroy.
 **/
ddwaf_object *ddwaf_object_insert_key_nocopy(ddwaf_object *map, const char *key, uint32_t length, ddwaf_allocator alloc);

/**
 * Creates a ddwaf_object from a JSON string. The JSON will be parsed and converted
 * into the appropriate ddwaf_object structure, supporting all JSON types including
 * objects, arrays, strings, numbers, booleans, and null values.
 *
 * @param output Object to populate with the parsed JSON data. (nonnull)
 * @param json_str The JSON string to parse. (nonnull)
 * @param length Length of the JSON string.
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return The success or failure of the operation.
 *
 * @note The output object must be freed by the caller using ddwaf_object_free.
 * @note If parsing fails, the output object will be left in an undefined state.
 * @note The provided JSON string is owned by the caller.
 **/
bool ddwaf_object_from_json(ddwaf_object *output, const char *json_str, uint32_t length, ddwaf_allocator alloc);

/**
 * Returns the type of the object.
 *
 * @param object The object from which to get the type.
 *
 * @return The object type of DDWAF_OBJ_INVALID if NULL.
 **/
DDWAF_OBJ_TYPE ddwaf_object_get_type(const ddwaf_object *object);

/**
 * Returns the size of the container object.
 *
 * @param object The object from which to get the size.
 *
 * @return The object size or 0 if the object is not a container (array, map).
 **/
size_t ddwaf_object_get_size(const ddwaf_object *object);

/**
 * Returns the length of the string object.
 *
 * @param object The object from which to get the length.
 *
 * @return The string length or 0 if the object is not a string.
 **/
size_t ddwaf_object_get_length(const ddwaf_object *object);

/**
 * Returns the string contained within the object.
 *
 * @param object The object from which to get the string.
 * @param length Output parameter on which to return the length of the string,
 *               this parameter is optional / nullable.
 *
 * @return The string of the object or NULL if the object is not a string.
 **/
const char* ddwaf_object_get_string(const ddwaf_object *object, size_t *length);

/**
 * Returns the uint64 contained within the object.
 *
 * @param object The object from which to get the integer.
 *
 * @return The integer or 0 if the object is not an unsigned.
 **/
uint64_t ddwaf_object_get_unsigned(const ddwaf_object *object);

/**
 * Returns the int64 contained within the object.
 *
 * @param object The object from which to get the integer.
 *
 * @return The integer or 0 if the object is not a signed.
 **/
int64_t ddwaf_object_get_signed(const ddwaf_object *object);

/**
 * Returns the float64 (double) contained within the object.
 *
 * @param object The object from which to get the float.
 *
 * @return The float or 0.0 if the object is not a float.
 **/
double ddwaf_object_get_float(const ddwaf_object *object);

/**
 * Returns the boolean contained within the object.
 *
 * @param object The object from which to get the boolean.
 *
 * @return The boolean or false if the object is not a boolean.
 **/
bool ddwaf_object_get_bool(const ddwaf_object *object);

/**
 * Returns the key contained in the container at the given index.
 *
 * @param object The container from which to extract the object.
 * @param index The position of the required object within the container.
 *
 * @return The requested object or NULL if the index is out of bounds or the
 *         object is not a container.
 **/
const ddwaf_object* ddwaf_object_at_key(const ddwaf_object *object, size_t index);


/**
 * Returns the object contained in the container at the given index.
 *
 * @param object The container from which to extract the object.
 * @param index The position of the required object within the container.
 *
 * @return The requested object or NULL if the index is out of bounds or the
 *         object is not a container.
 **/
const ddwaf_object* ddwaf_object_at_value(const ddwaf_object *object, size_t index);

/**
 * Returns the object within the given map with a key matching the provided one.
 *
 * @param object The container from which to extract the object.
 * @param key A string representing the key to find.
 * @param length Length of the key.
 *
 * @return The requested object or NULL if the key was not found or the
 *         object is not a container.
 **/
const ddwaf_object* ddwaf_object_find(const ddwaf_object *object, const char *key, size_t length);

/**
 * Creates a deep copy of the source object into the destination object.
 *
 * @param source The source object to clone from. (nonnull)
 * @param destination The destination object to clone into. (nonnull)
 * @param alloc Allocator to use for memory allocation. (nonnull)
 *
 * @return A pointer to the destination object or NULL if the operation failed.
 **/
ddwaf_object* ddwaf_object_clone(const ddwaf_object *source, ddwaf_object *destination, ddwaf_allocator alloc);

/**
 * Returns true if the object is invalid.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is invalid, false otherwise.
 **/
bool ddwaf_object_is_invalid(const ddwaf_object *object);

/**
 * Returns true if the object is null.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is null, false otherwise.
 **/
bool ddwaf_object_is_null(const ddwaf_object *object);

/**
 * Returns true if the object is a boolean.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is a boolean, false otherwise.
 **/
bool ddwaf_object_is_bool(const ddwaf_object *object);

/**
 * Returns true if the object is a signed integer.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is a signed integer, false otherwise.
 **/
bool ddwaf_object_is_signed(const ddwaf_object *object);

/**
 * Returns true if the object is an unsigned integer.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is an unsigned integer, false otherwise.
 **/
bool ddwaf_object_is_unsigned(const ddwaf_object *object);

/**
 * Returns true if the object is a float.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is a float, false otherwise.
 **/
bool ddwaf_object_is_float(const ddwaf_object *object);

/**
 * Returns true if the object is a string.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is a string, false otherwise.
 **/
bool ddwaf_object_is_string(const ddwaf_object *object);

/**
 * Returns true if the object is an array.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is an array, false otherwise.
 **/
bool ddwaf_object_is_array(const ddwaf_object *object);

/**
 * Returns true if the object is a map.
 *
 * @param object The object from which to get the type.
 *
 * @return True if the object is a map, false otherwise.
 **/
bool ddwaf_object_is_map(const ddwaf_object *object);

/**
 * Frees the memory contained within the object.
 *
 * @param object Object to destroy. (nonnull)
 * @param alloc Allocator to use for memory reclamation. (nonnull)
 **/
void ddwaf_object_destroy(ddwaf_object *object, ddwaf_allocator alloc);

/**
 * Return the version of the library
 *
 * @return version Version string, note that this should not be freed
 **/
const char *ddwaf_get_version();

/**
 * Sets the callback to relay logging messages to the binding
 *
 * @param cb The callback to call, or NULL to stop relaying messages
 * @param min_level The minimum logging level for which to relay messages
 *
 * @return whether the operation succeeded or not
 *
 * @note This function is not thread-safe
 **/
bool ddwaf_set_log_cb(ddwaf_log_cb cb, DDWAF_LOG_LEVEL min_level);

#ifdef __cplusplus
}
#endif /* __cplusplus */

#endif /*DDWAF_H */
