#include "logging.h"
#include "common.h"

#ifndef WIN32
#include <vector>
#include <syslog.h>
#include <stdarg.h>
#endif

namespace azure {    namespace storage_lite {

    std::function<void(log_level, const std::string&)> logger::s_logger = logger::simple_logger;

#ifdef _WIN32
    void logger::simple_logger(log_level level, const std::string& msg)
    {
        unused(level, msg);
        //Do nothing for now.
        //TODO: integrate with Windows trace log.
    }
#else
    int get_syslog_priority(log_level level)
    {
        static std::vector<int> indexing = { LOG_DEBUG, LOG_DEBUG, LOG_INFO, LOG_WARNING, LOG_ERR, LOG_CRIT };

        return indexing[level];
    }

    void logger::simple_logger(log_level level, const std::string& msg)
    {
        syslog(get_syslog_priority(level), "%s", msg.c_str());
    }
#endif
}}
