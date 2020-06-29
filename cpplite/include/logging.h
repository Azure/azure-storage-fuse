#pragma once

#include <string>
#include <functional>
#include <memory>

#ifdef _WIN32
#include <Rpc.h>
#pragma warning(disable : 4996)
#define _SNPRINTF _snprintf
#else
#include <unistd.h>
#define _SNPRINTF snprintf
#endif

#include "storage_EXPORTS.h"

constexpr size_t MAX_LOG_LENGTH = 8192u;

namespace azure { namespace storage_lite {

    enum log_level {
        trace = 0x0,
        debug,
        info,
        warn,
        error,
        critical,
        none
    };

    class logger
    {
    public:
        static void log(log_level level, const std::string& msg)
        {
            s_logger(level, msg);
        }

        template<typename... Args>
        static void log(log_level level, const std::string& format, Args... args)
        {
            if (level > log_level::critical)
            {
                return; // does not support higher level logging.
            }
            size_t size = _SNPRINTF(nullptr, 0, format.data(), args...) + 1;
            // limit the maximum size of this log string to 8kb, as the buffer needs
            // to be allocated to a continuous memory and is likely to fail
            // when the size is relatively big.
            size = std::min(size, MAX_LOG_LENGTH);

            std::string msg;
            msg.resize(size);
            _SNPRINTF(&msg[0], size, format.data(), args...);
            log(level, msg);
        }

        template<typename... Args>
        static void debug(const std::string& msg, Args... args)
        {
            log(log_level::debug, msg, args...);
        }

        template<typename... Args>
        static void info(const std::string& msg, Args... args)
        {
            log(log_level::info, msg, args...);
        }

        template<typename... Args>
        static void warn(const std::string& msg, Args... args)
        {
            log(log_level::warn, msg, args...);
        }

        template<typename... Args>
        static void error(const std::string& msg, Args... args)
        {
            log(log_level::error, msg, args...);
        }

        template<typename... Args>
        static void critical(const std::string& msg, Args... args)
        {
            log(log_level::critical, msg, args...);
        }

        template<typename... Args>
        static void trace(const std::string& msg, Args... args)
        {
            log(log_level::trace, msg, args...);
        }

        static void set_logger(const std::function<void(log_level, const std::string&)>& new_logger)
        {
            s_logger = new_logger;
        }

    protected:
        AZURE_STORAGE_API static std::function<void(log_level, const std::string&)> s_logger;

        static void simple_logger(log_level level, const std::string& msg);
    };
}}
