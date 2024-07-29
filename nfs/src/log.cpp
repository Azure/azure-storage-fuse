#include "log.h"

bool enable_debug_logs = false;

void init_log()
{
    /*
     * TODO: Initialize the logger to set the logfile, log format and anything
     *       else.
     */

    /*
     * Log info and above by default.
     * Later when we parse cmdline options, if -d or "-o debug" option
     * is passed we set the log level to debug.
     */
    spdlog::set_level(spdlog::level::info);

    /*
     * Add thread id in the log pattern, helps to debug when multiple
     * processes are accessing the mounted filesystem.
     */
    spdlog::set_pattern("[%t]%+");

    AZLogDebug("Logger initialized");
}
