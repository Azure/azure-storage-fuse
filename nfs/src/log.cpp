#include "log.h"

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

    AZLogDebug("Logger initialized");
}
