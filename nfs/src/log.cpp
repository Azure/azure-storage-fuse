#include "log.h"

void init_log()
{
    /*
     * TODO: Initialize the logger to set the logfile, log format and anything
     *       else.
     */

    /*
     * Log debug and above.
     *
     * TODO: Set it based on build type.
     */
    spdlog::set_level(spdlog::level::debug);

    AZLogDebug("Logger initialized");
}
