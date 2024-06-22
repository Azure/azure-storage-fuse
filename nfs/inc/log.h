#ifndef __AZNFSC_LOG_H__
#define __AZNFSC_LOG_H__

#include "spdlog/spdlog.h"
#include "spdlog/cfg/env.h"   // support for loading levels from the environment variable
#include "spdlog/fmt/ostr.h"  // support for user defined types

#define AZLogCrit(...)     spdlog::critical(__VA_ARGS__)
#define AZLogError(...)    spdlog::error(__VA_ARGS__)
#define AZLogWarn(...)     spdlog::warn(__VA_ARGS__)
#define AZLogInfo(...)     spdlog::info(__VA_ARGS__)
#define AZLogDebug(...)    spdlog::debug(__VA_ARGS__)

/*
 * For some special debugging needs we may want very chatty logs,
 * which for normal debugging causes too much distraction.
 */
#ifdef ENABLE_CHATTY
#define AZLogVerbose(...)  spdlog::debug(__VA_ARGS__)
#else
#define AZLogVerbose(...)  /* nothing */
#endif

void init_log();

#endif /* __AZNFSC_LOG_H__ */
