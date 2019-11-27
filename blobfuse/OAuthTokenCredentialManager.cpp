//
// Created by amanda on 11/19/19.
//
#include "OAuthTokenCredentialManager.h"
#include <http_base.h>
#include <json.hpp>
#include <iomanip>
#include "OAuthToken.h"
#include "constants.h"
#include "utility.h"
#include <syslog.h>

using nlohmann::json;

/// <summary>
/// GetTokenManagerInstance handles a singleton instance of the OAuthTokenManager.
/// If it does not exist,
/// </summary>
std::shared_ptr<OAuthTokenCredentialManager> GetTokenManagerInstance(std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> defaultCallback) {
    if(TokenManagerSingleton == nullptr) {
        if (defaultCallback == nullptr) {
            throw std::runtime_error("Tried to initialize the OAuthTokenCredentialManager, but failed because the default callback was empty!");
        }

        TokenManagerSingleton = std::make_shared<OAuthTokenCredentialManager>(defaultCallback);
    }

    return TokenManagerSingleton;
}

/// <summary>
/// OauthTokenCredentialManager Constructor for MSI
/// Creates the MSI Request URI and requests an OAuth token and expiry time for that token
/// </summary>
OAuthTokenCredentialManager::OAuthTokenCredentialManager(
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

    try {
        current_oauth_token = refresh_token();
    } catch(std::exception& ex) {
        syslog(LOG_ERR, "Unable to retrieve OAuth token: %s", ex.what());
        printf("Unable to retrieve OAuth token: %s\n", ex.what());
        valid_authentication = false;
        return;
    }

    if(current_oauth_token.empty()) {
        valid_authentication = false;
        syslog(LOG_ERR, "Unable to retrieve OAuth Token with given credentials.");
        printf("Unable to retrieve OAuth Token with given credentials.\n");
    }
}
/// <summary>
/// Check for valid authentication which is set by the constructor, and refresh functions.
/// </summary>
bool OAuthTokenCredentialManager::is_valid_connection()
{
    return valid_authentication;
}

/// <summary>
/// Refreshes the token. Note this can throw an error, so be prepared to catch.
/// Unless you absolutely _need_ to force a refresh, just call get_token instead.
/// <param>
/// </summary>
OAuthToken OAuthTokenCredentialManager::refresh_token()
{
    try {
        return refreshTokenCallback(httpClient);
    } catch(std::exception& ex) {
        valid_authentication = false;
        throw ex;
    }
}

/// <summary>
/// Returns current oauth_token, implicitly refreshing if the current token is invalid.
/// Note that this can throw an error if the refresh fails, so be prepared to catch.
/// </summary>
OAuthToken OAuthTokenCredentialManager::get_token()
{
    if (is_token_expired()) {
        try {
            current_oauth_token = refresh_token();
        } catch(std::exception& ex) {
            syslog(LOG_ERR, "Unable to retrieve OAuth token: %s", ex.what());
            valid_authentication = false;
            throw std::runtime_error(std::string("Failed to refresh OAuth token: ") + std::string(ex.what()));
        }
    }

    return current_oauth_token;
}

/// <summary>
/// TODO: check the expiry time against the current utc time
/// <summary>
bool OAuthTokenCredentialManager::is_token_expired()
{
    time_t current_time;

    time ( &current_time );

    // check if about to expire via the buffered expiry time
    return current_time + (60 * 5) >= current_oauth_token.expires_on;
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

        // Set up the output stream for the request
        storage_iostream ios = storage_iostream::create_storage_stream();
        request_handle->set_output_stream(ios.ostream());

        // TODO: decide retry interval, also make constant
        std::chrono::seconds retry_interval(5);
        OAuthToken parsed_token;
        request_handle->submit([&parsed_token, &ios](http_base::http_code http_code_result, const storage_istream&, CURLcode curl_code)
        {
            if (curl_code != CURLE_OK || unsuccessful(http_code_result)) {
             std::string req_result = "";

             try { // to avoid crashing to any potential errors while reading the stream, we back it with a try catch statement.
                 std::string json_request_result(std::istreambuf_iterator<char>(ios.istream()),
                                                 std::istreambuf_iterator<char>());

                 req_result = json_request_result;
             } catch(std::exception&){}

             std::ostringstream errStream;
             errStream << "Failed to retrieve OAuth Token from IMDS endpoint (CURLCode: " << curl_code << ", HTTP code: " << http_code_result << "): " << req_result;
             throw std::runtime_error(errStream.str());
            }
            else
            {
             std::string json_request_result(std::istreambuf_iterator<char>(ios.istream()),
                                             std::istreambuf_iterator<char>());
             printf("raw json: %s\n", json_request_result.c_str());

             try {
                 json j;
                 j = json::parse(json_request_result);
                 parsed_token = j.get<OAuthToken>();
             } catch(std::exception& ex) {
                 throw std::runtime_error(std::string("Failed to parse OAuth token: ") + std::string(ex.what()));
             }
            }
        }, retry_interval);

        // request_handle destructs because it's no longer referenced
        return parsed_token;
    };
}