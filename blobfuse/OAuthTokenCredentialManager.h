//
// Created by amanda on 11/19/19.
//

#include <boost/thread/shared_mutex.hpp>
#include "http/libcurl_http_client.h"
#include "OAuthToken.h"

#ifndef OAUTH_TOKEN_CREDENTIAL_MANAGER_H
#define OAUTH_TOKEN_CREDENTIAL_MANAGER_H

using namespace microsoft_azure::storage;

class OAuthTokenCredentialManager {
public:
    /// <summary>
    /// OauthTokenCredentialManager Constructor
    /// </summary>
    OAuthTokenCredentialManager(std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> refreshCallback);
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
    OAuthToken current_oauth_token;
    bool valid_authentication;
    boost::shared_mutex token_mutex;
    std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> refreshTokenCallback;
};

// This is meant to be the singleton instance of OAuthTokenManager, and should not be instantiated more than once.
static std::shared_ptr<OAuthTokenCredentialManager> TokenManagerSingleton;

std::shared_ptr<OAuthTokenCredentialManager> GetTokenManagerInstance(std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)>);

// maybe TODO: SetUpSPNCallback, SetUpDeviceOAuthCallback.

/// <summary>
/// This is an empty callback, for when you don't particularly care about initializing the singleton OAuthTokenManager instance.
/// </summary>
static std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> EmptyCallback = nullptr;

/// <summary>
/// Sets up the callback for MSI authentication
/// </summary>
std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpMSICallback(
        std::string client_id_p = "",
        std::string object_id_p = "",
        std::string resource_id_p = "");

#endif