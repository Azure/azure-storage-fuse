#ifndef __BLOBFUSE_CONSTANTS_H__
#define __BLOBFUSE_CONSTANTS_H__

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
    extern const int unknown_error;
}

#endif