#ifndef __AZNFSC_LOG_H__
#define __AZNFSC_LOG_H__

#include "spdlog/spdlog.h"
#include "spdlog/cfg/env.h"   // support for loading levels from the environment variable
#include "spdlog/fmt/ostr.h"  // support for user defined types

#ifdef ENABLE_DEBUG
/*
 * __FILE__ like macro but returns the short filename, which is more usable
 * in logs.
 * The LOC* macros are used to add caller's file:line information to a function.
 * This can aid debugging in some cases. Use with caution, when really required.
 */
#define __FILENAME__ ({const char *p = ::strrchr(__FILE__, '/'); p ? p + 1 : __FILE__;})

#define LOC_PARAMS  const char *__srcfile, int __srcline,
#define LOC_FMT     "[{}:{}] "
#define LOC_ARGS    __srcfile, __srcline,
#define LOC_VAL     __FILENAME__, __LINE__,
#else /* !ENABLE_DEBUG */
#define LOC_PARAMS  /* nothing */
#define LOC_FMT     ""
#define LOC_ARGS    /* nothing */
#define LOC_VAL     /* nothing */
#endif /* ENABLE_DEBUG */

/*
 * Despite their claims, spdlog because of its typeless logging is seen to
 * consume lot of cpu. We can quickly verify that by uncommenting this.
 * If seen to be a real problem we will have to move to simpler logging
 * lib.
 */
//#define DISABLE_NON_CRIT_LOGGING

#ifndef ENABLE_DEBUG
#define AZLogCrit(fmt, ...) \
    spdlog::critical(fmt, ##__VA_ARGS__)
#define AZLogError(fmt, ...) \
    spdlog::error(fmt, ##__VA_ARGS__)
#define AZLogWarn(fmt, ...) \
    spdlog::warn(fmt, ##__VA_ARGS__)
#else /* !ENABLE_DEBUG */
#define AZLogCrit(fmt, ...) \
    spdlog::critical(LOC_FMT fmt, __FILENAME__, __LINE__, ##__VA_ARGS__)
#define AZLogError(fmt, ...) \
    spdlog::error(LOC_FMT fmt, __FILENAME__, __LINE__, ##__VA_ARGS__)
#define AZLogWarn(fmt, ...) \
    spdlog::warn(LOC_FMT fmt, __FILENAME__, __LINE__, ##__VA_ARGS__)
#endif /* ENABLE_DEBUG */

#ifdef DISABLE_NON_CRIT_LOGGING
#define AZLogInfo(...)     /* nothing */
#define AZLogDebug(...)    /* nothing */
#else /* !DISABLE_NON_CRIT_LOGGING */
#ifndef ENABLE_DEBUG
#define AZLogInfo(fmt, ...) \
    spdlog::info(fmt, ##__VA_ARGS__)
#define AZLogDebug(fmt, ...) \
    if (enable_debug_logs) spdlog::debug(fmt, ##__VA_ARGS__)
#else /* !ENABLE_DEBUG */
#define AZLogInfo(fmt, ...) \
    spdlog::info(LOC_FMT fmt, __FILENAME__, __LINE__, ##__VA_ARGS__)
#define AZLogDebug(fmt, ...) \
    if (enable_debug_logs) spdlog::debug(LOC_FMT fmt, __FILENAME__, __LINE__, ##__VA_ARGS__)
#endif /* ENABLE_DEBUG */
#endif /* DISABLE_NON_CRIT_LOGGING */

/*
 * For some special debugging needs we may want very chatty logs,
 * which for normal debugging causes too much distraction.
 */
#ifdef ENABLE_CHATTY
#ifndef ENABLE_DEBUG
#define AZLogVerbose(...) \
    spdlog::debug(__VA_ARGS__)
#else /* !ENABLE_DEBUG */
#define AZLogVerbose(fmt, ...) \
    spdlog::debug(LOC_FMT fmt, __FILENAME__, __LINE__, ##__VA_ARGS__)
#endif /* ENABLE_DEBUG */
#else /* !ENABLE_CHATTY */
#define AZLogVerbose(...)  /* nothing */
#endif

void init_log();
extern bool enable_debug_logs;

#endif /* __AZNFSC_LOG_H__ */
