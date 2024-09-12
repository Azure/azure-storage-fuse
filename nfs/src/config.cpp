#include "aznfsc.h"
#include "yaml-cpp/yaml.h"

using namespace std;

/*
 * Global aznfsc config instance holding all the aznfs client configuration.
 */
struct aznfsc_cfg aznfsc_cfg;

/*
 * This function parses the contents of the yaml config file denoted by path
 * config_file into the aznfsc_cfg structure.
 */
bool aznfsc_cfg::parse_config_yaml()
{

#define __CHECK_INT(var, min, max, zeroisvalid) \
do { \
    if (((var) == -1) && config[#var]) { \
        (var) = config[#var].as<int>(); \
        if ((var) < min || (var) > max) { \
            if ((zeroisvalid) && ((var) == 0)) { \
                break; \
            } \
            throw YAML::Exception( \
                    config[#var].Mark(), \
                    std::string("Invalid value for config "#var": ") + \
                    std::to_string(var) + \
                    std::string(" (valid range [") + \
                    std::to_string(min) + ", " + std::to_string(max) + "])"); \
        }\
    } \
} while (0);

/*
 * Macro to check validity of config var of integer type.
 */
#define _CHECK_INT(var, min, max) __CHECK_INT(var, min, max, false)

/*
 * Macro to check validity of config var of integer type with 0 being a
 * valid value.
 */
#define _CHECK_INTZ(var, min, max) __CHECK_INT(var, min, max, true)

/*
 * Macro to check validity of config var of boolean type.
 */
#define _CHECK_BOOL(var) \
do { \
    if (config[#var]) { \
        (var) = config[#var].as<bool>(); \
    } \
} while (0);


/*
 * Macro to check validity of config var of string type.
 */
#define __CHECK_STR(var, is_valid, ignore_empty) \
do { \
    if (((var) == nullptr) && config[#var]) { \
        /* Empty key is returned as "null" by the yaml parser */ \
        if ((ignore_empty) && config[#var].as<std::string>() == "null") { \
            break; \
        } \
        (var) = ::strdup(config[#var].as<std::string>().c_str()); \
        if (!is_valid(var)) { \
            throw YAML::Exception( \
                    config[#var].Mark(), \
                    std::string("Invalid value for config "#var": ") + \
                    std::string(var)); \
        } \
    } \
} while (0);

#define _CHECK_STR(var) __CHECK_STR(var, is_valid_##var, false)
#define _CHECK_STR2(var, is_valid) __CHECK_STR(var, is_valid, false)

    if (config_yaml == nullptr) {
        return true;
    }

    AZLogDebug("Parsing config yaml {}", config_yaml);

    /*
     * We parse the config yaml and set *only* those options which are not yet
     * set by cmdline. Thus cmdline options are given higher priority than the
     * corresponding option in the config yaml.
     */
    try {
        YAML::Node config = YAML::LoadFile(config_yaml);

        _CHECK_STR(account);
        _CHECK_STR(container);
        _CHECK_STR(cloud_suffix);

        if ((port == -1) && config["port"]) {
            port = config["port"].as<int>();
#ifndef ENABLE_NON_AZURE_NFS
            if (port != 2048 && port != 2047) {
                throw YAML::Exception(
                    config["port"].Mark(),
                    std::string("Invalid port number: ") +
                    std::to_string(port) +
                    std::string(" (can be 2048 or 2047)"));
            }
#endif
        }

        _CHECK_INT(nconnect, AZNFSCFG_NCONNECT_MIN, AZNFSCFG_NCONNECT_MAX);
        _CHECK_INT(timeo, AZNFSCFG_TIMEO_MIN, AZNFSCFG_TIMEO_MAX);
        _CHECK_INT(acregmin, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(acregmax, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(acdirmin, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(acdirmax, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_INT(actimeo, AZNFSCFG_ACTIMEO_MIN, AZNFSCFG_ACTIMEO_MAX);
        _CHECK_STR(lookupcache);
        _CHECK_INT(rsize, AZNFSCFG_RSIZE_MIN, AZNFSCFG_RSIZE_MAX);
        _CHECK_INT(wsize, AZNFSCFG_WSIZE_MIN, AZNFSCFG_WSIZE_MAX);
        _CHECK_INT(retrans, AZNFSCFG_RETRANS_MIN, AZNFSCFG_RETRANS_MAX);
        _CHECK_INT(readdir_maxcount, AZNFSCFG_READDIR_MIN, AZNFSCFG_READDIR_MAX);
        /*
         * Allow special value of 0 to disable readahead.
         * Mostly useful for testing.
         */
        _CHECK_INTZ(readahead_kb, AZNFSCFG_READAHEAD_KB_MIN, AZNFSCFG_READAHEAD_KB_MAX);
        _CHECK_INT(fuse_max_background, AZNFSCFG_FUSE_MAX_BG_MIN, AZNFSCFG_FUSE_MAX_BG_MAX);

        _CHECK_BOOL(cache.readdir.kernel.enable);
        _CHECK_BOOL(cache.readdir.user.enable);
        _CHECK_BOOL(cache.data.kernel.enable);

        _CHECK_BOOL(cache.data.user.enable);
        if (cache.data.user.enable) {
            _CHECK_INT(cache.data.user.max_size_mb,
                       AZNFSCFG_CACHE_MAX_MB_MIN, AZNFSCFG_CACHE_MAX_MB_MAX);
        }

        _CHECK_BOOL(filecache.enable);
        if (filecache.enable) {
            _CHECK_STR2(filecache.cachedir, is_valid_cachedir);
            _CHECK_INT(filecache.max_size_gb, AZNFSCFG_FILECACHE_MAX_GB_MIN, AZNFSCFG_FILECACHE_MAX_GB_MAX);
        }

    } catch (const YAML::BadFile& e) {
        AZLogError("Error loading config file {}: {}", config_yaml, e.what());
        return false;
    } catch (const YAML::Exception& e) {
        AZLogError("Error parsing config file {}: {}", config_yaml, e.what());
        return false;
    } catch (...) {
        AZLogError("Unknown error parsing config file {}", config_yaml);
        return false;
    }

    return true;
}

/**
 * Set default values for options not yet assigned.
 * This must be called after fuse_opt_parse() and parse_config_yaml()
 * assign config values from command line and the config yaml file.
 * Also sanitizes various values.
 */
void aznfsc_cfg::set_defaults_and_sanitize()
{
    if (port == -1)
        port = 2048;
    if (nconnect == -1)
        nconnect = 1;
    if (rsize == -1)
        rsize = 1048576;
    if (wsize == -1)
        wsize = 1048576;
    if (retrans == -1)
        retrans = 3;
    if (timeo == -1)
        timeo = 600;
    if (acregmin == -1)
        acregmin = 3;
    if (acregmax == -1)
        acregmax = 60;
    if (acdirmin == -1)
        acdirmin = 30;
    if (acdirmax == -1)
        acdirmax = 60;
    if (actimeo != -1) {
        acregmin = acregmax = acdirmin = acdirmax = actimeo;
    } else {
        /*
         * This is used only by nfs_client::reply_entry() for setting the
         * timeout of negative lookup result.
         * Rest everywhere we will use ac{reg|dir}{min|max}.
         */
        actimeo = AZNFSCFG_ACTIMEO_MIN;
    }
    if (acregmin > acregmax)
        acregmin = acregmax;
    if (acdirmin > acdirmax)
        acdirmin = acdirmax;

    if (std::string(lookupcache) == "all") {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_ALL;
    } else if (std::string(lookupcache) == "none") {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_NONE;
    } else if (std::string(lookupcache) == "pos" ||
            std::string(lookupcache) == "positive") {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_POS;
    } else {
        lookupcache_int = AZNFSCFG_LOOKUPCACHE_DEF;
    }

    if (readdir_maxcount == -1)
        readdir_maxcount = 1048576;
    if (readahead_kb == -1)
        readahead_kb = 16384;
    if (cache.data.user.enable) {
        if (cache.data.user.max_size_mb == -1)
            cache.data.user.max_size_mb = AZNFSCFG_CACHE_MAX_MB_DEF;
    }
    if (filecache.enable) {
        if (filecache.max_size_gb == -1)
            filecache.max_size_gb = AZNFSCFG_FILECACHE_MAX_GB_DEF;
    }
    if (cloud_suffix == nullptr)
        cloud_suffix = ::strdup("blob.core.windows.net");

    assert(account != nullptr);
    assert(container != nullptr);

    // Set aggregates.
    server = std::string(account) + "." + std::string(cloud_suffix);
    export_path = "/" + std::string(account) + "/" + std::string(container);

    // Dump the final config values for debugging.
    AZLogDebug("===== config start =====");
    AZLogDebug("port = {}", port);
    AZLogDebug("nconnect = {}", nconnect);
    AZLogDebug("rsize = {}", rsize);
    AZLogDebug("wsize = {}", wsize);
    AZLogDebug("retrans = {}", retrans);
    AZLogDebug("timeo = {}", timeo);
    AZLogDebug("acregmin = {}", acregmin);
    AZLogDebug("acregmax = {}", acregmax);
    AZLogDebug("acdirmin = {}", acdirmin);
    AZLogDebug("acdirmax = {}", acdirmax);
    AZLogDebug("actimeo = {}", actimeo);
    AZLogDebug("lookupcache = {}", lookupcache);
    AZLogDebug("readdir_maxcount = {}", readdir_maxcount);
    AZLogDebug("readahead_kb = {}", readahead_kb);
    AZLogDebug("fuse_max_background = {}", fuse_max_background);
    AZLogDebug("cache.readdir.kernel.enable = {}", cache.readdir.kernel.enable);
    AZLogDebug("cache.readdir.user.enable = {}", cache.readdir.user.enable);
    AZLogDebug("cache.data.kernel.enable = {}", cache.data.kernel.enable);
    AZLogDebug("cache.data.user.enable = {}", cache.data.user.enable);
    AZLogDebug("cache.data.user.max_size_mb = {}", cache.data.user.max_size_mb);
    AZLogDebug("filecache.enable = {}", filecache.enable);
    AZLogDebug("filecache.cachedir = {}", filecache.cachedir ? filecache.cachedir : "");
    AZLogDebug("filecache.max_size_gb = {}", filecache.max_size_gb);
    AZLogDebug("account = {}", account);
    AZLogDebug("container = {}", container);
    AZLogDebug("cloud_suffix = {}", cloud_suffix);
    AZLogDebug("mountpoint = {}", mountpoint);
    AZLogDebug("===== config end =====");
}
