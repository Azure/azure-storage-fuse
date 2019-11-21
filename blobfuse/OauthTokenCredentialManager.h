//
// Created by amanda on 11/19/19.
//

#include "blob/blob_client.h"

#ifndef BLOBFUSE_OAUTHTOKENCREDENTIALMANAGER_H
#define BLOBFUSE_OAUTHTOKENCREDENTIALMANAGER_H

using namespace microsoft_azure::storage;

class OauthTokenCredentialManager {
public:
    /// <summary>
    /// OauthTokenCredentialManager Constructor
    ///
    /// </summary>
    OauthTokenCredentialManager(
        std::string client_id_p = "",
        std::string object_id_p = "",
        std::string resource_id_p = "");
    /// <summary>
    /// Check for valid authentication which is set by the constructor
    /// </summary>
    bool is_valid_connection();
    /// <summary>
    /// TODO: call the service to refresh the oauth token
    /// TODO: set the oauth_token with the new token and set the expirey_time
    /// </summary>
    const std::string refresh_token();
    /// <summary>
    /// Returns current oauth_token
    /// </summary>
    const std::string get_token();
    /// <summary>
    /// TODO: check the expirey time against the current utc time
    /// <summary>
    bool is_token_expired();

private:
    std::shared_ptr<CurlEasyClient> httpClient;
    std::shared_ptr<CurlEasyRequest> request_handle;
    std::string uri_token_request;
    std::string current_oauth_token;
    double expiry_time;
    bool valid_authentication;
    boost::shared_mutex token_mutex;
};


#endif //BLOBFUSE_OAUTHTOKENCREDENTIALMANAGER_H
