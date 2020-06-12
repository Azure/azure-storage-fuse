
#include <BlobfuseConstants.h>

namespace blobfuse_constants {
    const int max_concurrency_oauth = 2;
    const int max_retry_oauth = 5;
    const int max_concurrency_blob_wrapper = 40;

    const char* oauth_request_uri = "https://login.microsoftonline.com";
    const char* spn_request_path = "oauth2/v2.0/token";
    const char* msi_request_uri = "http://169.254.169.254/metadata/identity/oauth2/token";
    const char* param_oauth_resource_data = "https://storage.azure.com/";

    const char* param_client_id = "client_id";
    const char* param_object_id = "object_id";
    const char* param_mi_res_id = "mi_res_id";
    const char* param_mi_api_version = "api-version";
    const char* param_mi_api_version_data = "2018-02-01";
    const char* param_oauth_resource = "resource";
    const char* header_metadata = "Metadata";
    const char* header_msi_secret = "Secret";

    const char* header_user_agent = "User-Agent";
    const char* header_value_user_agent = "Azure-Storage-Fuse/1.2.4-TEST";
    const char* header_ms_date = "x-ms-date";
    const char* header_ms_version = "x-ms-version";
    const char* header_value_storage_version = "2018-11-09";
    const char* header_ms_properties = "x-ms-properties";
    const char* header_ms_resource_type = "x-ms-resource-type";

    const int unknown_error = 1600;

    const unsigned int acl_size = 9;

    const int http_request_conflict = 409;

    const std::string former_directory_signifier = ".directory";

    const std::map<int, int> error_mapping = {{404, ENOENT}, {403, EACCES}, {1600, ENOENT}};

    const int HTTP_REQUEST_CONFLICT = 409;
}