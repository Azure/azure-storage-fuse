
#include "blobfuse_constants.h"

namespace blobfuse_constants {
    const int max_concurrency_oauth = 2;
    const int max_retry_oauth = 5;
    const int max_concurrency_blob_wrapper = 20;

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

    const int unknown_error = 1600;
}