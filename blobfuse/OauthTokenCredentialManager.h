//
// Created by amanda on 11/19/19.
//

#include "blob/blob_client.h"
#include "OAuthToken.h"

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
        std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> refreshCallback);
    /// <summary>
    /// Check for valid authentication which is set by the constructor
    /// </summary>
    bool is_valid_connection();
    /// <summary>
    /// TODO: use a callback rather than a distinct function for refreshing
    /// Refreshes the currently existing OAuth token. get_token makes this call implicitly if the current token is expired, so don't worry about calling it yourself, except for init.
    /// </summary>
    OAuthToken refresh_token();
    /// <summary>
    /// Returns current oauth_token
    /// </summary>
    OAuthToken get_token();
    /// <summary>
    /// Checks if the currently active token is expired. get_token makes this call implicitly, so don't worry about calling it yourself.
    /// </summary>
    bool is_token_expired();

private:
    std::shared_ptr<CurlEasyClient> httpClient;
    std::shared_ptr<CurlEasyRequest> request_handle;
    std::string uri_token_request;
    OAuthToken current_oauth_token;
    bool valid_authentication;
    boost::shared_mutex token_mutex;
    std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> refreshTokenCallback;
};

// maybe TODO: SetUpSPNCallback, SetUpDeviceOAuthCallback.
/// <summary>
/// Sets up the callback for MSI authentication
/// </summary>
std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpMSICallback(
        std::string client_id_p = "",
        std::string object_id_p = "",
        std::string resource_id_p = "");

#endif //BLOBFUSE_OAUTHTOKENCREDENTIALMANAGER_H
