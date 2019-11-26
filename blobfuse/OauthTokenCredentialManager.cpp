//
// Created by amanda on 11/19/19.
//
#include <time.h>
#include "OauthTokenCredentialManager.h"
#include <http_base.h>
#include <json.hpp>
#include <iomanip>
#include <utility>
#include "OAuthToken.h"

using nlohmann::json;

/// <summary>
/// OauthTokenCredentialManager Constructor for MSI
/// Creates the MSI Request URI and requests an OAuth token and expiry time for that token
/// </summary>
OauthTokenCredentialManager::OauthTokenCredentialManager(
    std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> refreshCallback)
{
    if (refreshCallback == nullptr) {
        valid_authentication = false;
        syslog(LOG_ERR, "OAuthTokenManager was supplied an invalid refresh callback.");
        printf("OAuthTokenManager was supplied an invalid refresh callback.");
        return;
    }

    httpClient = std::make_shared<CurlEasyClient>(20);
    refreshTokenCallback = refreshCallback;

    current_oauth_token = refresh_token();
    if(current_oauth_token.empty()) {
        valid_authentication = false;
        syslog(LOG_ERR, "Unable to retrieve OAuth Token with given credentials."); // todo: better error message
        printf("Unable to retrieve OAuth Token with given credentials."); // todo: better error message
    }
}
/// <summary>
/// Check for valid authentication which is set by the constructor
/// </summary>
bool OauthTokenCredentialManager::is_valid_connection()
{
    return valid_authentication;
}
/// <summary>
/// TODO: call the service to refresh the oauth token
/// TODO: set the oauth_token with the new token and set the expiry_time
/// TODO: make this call a refresh callback instead of having refresh logic in here
/// <param>
/// </summary>
OAuthToken OauthTokenCredentialManager::refresh_token()
{
    printf("attempting refresh\n");
    return refreshTokenCallback(httpClient);
}

/// <summary>
/// Returns current oauth_token
/// </summary>
OAuthToken OauthTokenCredentialManager::get_token()
{
    if (is_token_expired()) {
        return refresh_token();
    }

    return current_oauth_token;
}

/// <summary>
/// TODO: check the expiry time against the current utc time
/// <summary>
bool OauthTokenCredentialManager::is_token_expired()
{
    time_t current_time;

    time ( &current_time );

    // check if about to expire via the buffered expiry time
    return current_time + (60 * 5) <= current_oauth_token.expires_on;
}

// ===== CALLBACK SETUP ZONE =====

std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpMSICallback(std::string client_id_p, std::string object_id_p, std::string resource_id_p)
{
    // Create the URI token request
    std::string uri_token_request = constants::msi_request_uri;
    if(!client_id_p.empty())
    {
        uri_token_request += constants::param_client_id + client_id_p;
    }
    if(!object_id_p.empty())
    {
        uri_token_request += constants::param_object_id + object_id_p;
    }
    if(!resource_id_p.empty())
    {
        uri_token_request += constants::param_mi_res_id + resource_id_p;
    }

    return [uri_token_request](std::shared_ptr<CurlEasyClient> httpClient) {
        // prepare the CURL handle
        std::shared_ptr<CurlEasyRequest> request_handle = httpClient->get_handle();

        request_handle->set_url(uri_token_request);
        request_handle->add_header(constants::header_metadata, "true");
        request_handle->set_method(http_base::http_method::get);

        printf("Token request URL: %s\n", uri_token_request.c_str());

        // Set up the output stream for the request
        storage_iostream ios = storage_iostream::create_storage_stream();
        request_handle->set_output_stream(ios.ostream());

        // TODO: decide retry interval, also make constant
        std::chrono::seconds retry_interval(5);
        OAuthToken parsed_token;
        request_handle->submit([&parsed_token, &ios](http_base::http_code http_code_result, const storage_istream&, CURLcode curl_code)
        {
            if (curl_code != CURLE_OK || unsuccessful(http_code_result)) {
             syslog(LOG_ERR, "Unable to retrieve OAuth Token"); // todo better message
             printf("curlcode: %d\n", curl_code);
             printf("httpcode: %d\n", http_code_result);
            }
            else
            {
             std::string json_request_result(std::istreambuf_iterator<char>(ios.istream()),
                                             std::istreambuf_iterator<char>());
             printf("raw json: %s\n", json_request_result.c_str());

             json j;
             j = json::parse(json_request_result);
             parsed_token = j.get<OAuthToken>();
            }
        }, retry_interval);

        // request_handle destructs because it's no longer referenced
        return parsed_token;
    };
}