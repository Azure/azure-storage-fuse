#ifndef __AZNFSC_LOG_H__
#define __AZNFSC_LOG_H__

#include "spdlog/spdlog.h"
#include "spdlog/cfg/env.h"   // support for loading levels from the environment variable
#include "spdlog/fmt/ostr.h"  // support for user defined types

/*
 * Despite their claims, spdlog because of its typeless logging is seen to
 * consume lot of cpu. We can quickly verify that by uncommenting this.
 * If seen to be a real problem we will have to move to simpler logging
 * lib.
 */
//#define DISABLE_NON_CRIT_LOGGING

#define AZLogCrit(...)     spdlog::critical(__VA_ARGS__)
#define AZLogError(...)    spdlog::error(__VA_ARGS__)
#define AZLogWarn(...)     spdlog::warn(__VA_ARGS__)

#ifdef DISABLE_NON_CRIT_LOGGING
#define AZLogInfo(...)     /* nothing */
#define AZLogDebug(...)    /* nothing */
#else
#define AZLogInfo(...)     spdlog::info(__VA_ARGS__)
#define AZLogDebug(...)    if (enable_debug_logs) spdlog::debug(__VA_ARGS__)
#endif

/*
 * For some special debugging needs we may want very chatty logs,
 * which for normal debugging causes too much distraction.
 */
#ifdef ENABLE_CHATTY
#define AZLogVerbose(...)  spdlog::debug(__VA_ARGS__)
#else
#define AZLogVerbose(...)  /* nothing */
#endif

#ifdef ENABLE_DEBUG
/*
 * __FILE__ like macro but returns the short filename, which is more usable
 * in logs.
 * The LOC* macros are used to add caller's file:line information to a function.
 * This can aid debugging in some cases. Use with caution, when really required.
 */
#define __FILENAME__ ({const char *p = ::strrchr(__FILE__, '/'); p ? p + 1 : __FILE__;})

#define LOC_PARAMS  const char *__srcfile, int __srcline,
#define LOC_FMT     "[{}:{}]"
#define LOC_ARGS    __srcfile, __srcline,
#define LOC_VAL     __FILENAME__, __LINE__,
#else /* ! ENABLE_DEBUG */
#define LOC_PARAMS  /* nothing */
#define LOC_FMT     ""
#define LOC_ARGS    /* nothing */
#define LOC_VAL     /* nothing */
#endif /* ENABLE_DEBUG */


void init_log();
extern bool enable_debug_logs;

#endif /* __AZNFSC_LOG_H__ */
