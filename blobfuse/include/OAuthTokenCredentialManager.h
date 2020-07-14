//
// Created by amanda on 11/19/19.
//

#include <boost/thread/shared_mutex.hpp>
#include "http/libcurl_http_client.h"
#include "OAuthToken.h"

#ifndef OAUTH_TOKEN_CREDENTIAL_MANAGER_H
#define OAUTH_TOKEN_CREDENTIAL_MANAGER_H

// Enable this to start a thread to refresh the token instread of 
// service thread checking the timer for each request and going for refresh
#define TOKEN_REFRESH_THREAD

// This shall be disabled as this expires token faster
/*
#ifdef TOKEN_REFRESH_THREAD
#define TEST_TOKEN_THR
#endif
*/


using namespace azure::storage_lite;

class OAuthTokenCredentialManager {
public:
    /// <summary>
    /// OauthTokenCredentialManager Constructor
    /// </summary>
    OAuthTokenCredentialManager(std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> refreshCallback);

    void StartTokenMonitor();
#ifdef TOKEN_REFRESH_THREAD
    void TokenMonitor();
#endif
    
    /// <summary>
    /// Check for valid authentication which is set by the constructor, and refresh functions.
    /// </summary>
    bool is_valid_connection();
    /// <summary>
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

/// <summary>
/// GetTokenManagerInstance handles a singleton instance of the OAuthTokenManager.
/// If it does not exist, it creates it using the supplied default callback.
/// If no callback is supplied and the token manager doesn't exist, this function will throw.
/// No callback is necessary to get the current instance.
/// </summary>
std::shared_ptr<OAuthTokenCredentialManager> GetTokenManagerInstance(std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)>);

// maybe TODO: SetUpSPNCallback, SetUpDeviceOAuthCallback.

/// <summary>
/// This is an empty callback, for when you don't particularly care about initializing the singleton OAuthTokenManager instance.
/// </summary>
static std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> EmptyCallback = nullptr;

/// <summary>
/// SetUpMSICallback sets up a refresh callback for MSI auth. This should be used to create a OAuthTokenManager instance.
/// </summary>
std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpMSICallback(
        std::string client_id_p = "",
        std::string object_id_p = "",
        std::string resource_id_p = "",
        std::string msi_endpoint_p = "",
        std::string msi_secret_p = "");

/// <summary>
/// SetUpSPNCallback sets up a refresh callback for service principal auth. This should be used to create a OAuthTokenManager instance.
/// </summary>
std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpSPNCallback(
        std::string tenant_id_p = "",
        std::string client_id_p = "",
        std::string client_secret_p = "",
        std::string aad_endpoint_p = "");
// BIG CONCERN: Taking in credentials via a plaintext file is a no-no security wise. For now, they'll only be taken in via the environment variable.


 /// <summary>
/// Checks if passed in token is expired. 
/// </summary>
bool is_token_expired_forcurrentutc(OAuthToken &token);

/// <summary>
/// get current time in utc
/// <summary>
time_t get_current_time_in_utc();

std::string GetTokenCallback();

#endif