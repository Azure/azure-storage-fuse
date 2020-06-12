#ifndef __BLOBFUSE_CONSTANTS_H__
#define __BLOBFUSE_CONSTANTS_H__

#include <string>
#include <map>
#include <syslog.h>


#define AZS_DEBUGLOGV(fmt,...) do {syslog(LOG_DEBUG,"Function %s, in file %s, line %d: " fmt, __func__, __FILE__, __LINE__, __VA_ARGS__); } while(0)
#define AZS_DEBUGLOG(fmt) do {syslog(LOG_DEBUG,"Function %s, in file %s, line %d: " fmt, __func__, __FILE__, __LINE__); } while(0)

/* Define errors and return codes */
enum D_RETURN_CODE
{
    D_NOTEXIST = -1,
    D_EMPTY = 0,
    D_NOTEMPTY = 1,
    D_FAILED = 2
};

enum AUTH_TYPE {
    MSI_AUTH,
    SAS_AUTH,
    KEY_AUTH,
    SPN_AUTH,
    INVALID_AUTH
};

/* Define high and low gc_cache threshold values*/
/* These threshold values were not calculated and are just an approximation of when we should be clearing the cache */
#define HIGH_THRESHOLD_VALUE 90
#define LOW_THRESHOLD_VALUE 80

namespace blobfuse_constants {
    extern const int max_concurrency_oauth;
    extern const int max_retry_oauth;
    extern const int max_concurrency_blob_wrapper;

    extern const char* oauth_request_uri;
    extern const char* spn_request_path;
    extern const char* msi_request_uri;
    extern const char* param_oauth_resource_data;

    extern const char* param_client_id;
    extern const char* param_object_id;
    extern const char* param_mi_res_id;
    extern const char* param_mi_api_version;
    extern const char* param_mi_api_version_data;
    extern const char* param_oauth_resource;
    extern const char* header_metadata;
    extern const char* header_msi_secret;
    extern const char* header_user_agent;
    extern const char* header_value_user_agent;
    extern const char * header_ms_date;
    extern const char* header_ms_version;
    extern const char* header_value_storage_version;
    extern const char* header_ms_properties;
    extern const char* header_ms_resource_type;

    extern const int unknown_error;

    extern const unsigned int acl_size;
    extern const int http_request_conflict;

    // Needed for compatibility with pre-GA blobfuse:
    // String that signifies that this blob represents a directory.
    // This string should be appended to the name of the directory.  The resultant string should be the name of a zero-length blob; this represents the directory on the service.
    extern const std::string former_directory_signifier;

    // Currently, the cpp lite lib puts the HTTP status code in errno.
    // This mapping tries to convert the HTTP status code to a standard Linux errno.
    // TODO: Ensure that we map any potential HTTP status codes we might receive.
    // Used to map HTTP errors (ex. 404) to Linux errno (ex ENOENT)
    extern const std::map<int, int> error_mapping;

    extern const int HTTP_REQUEST_CONFLICT;
}

#endif