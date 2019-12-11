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
/// If it does not exist, it creates it using the supplied default callback.
/// If no callback is supplied and the token manager doesn't exist, this function will throw.
/// No callback is necessary to get the current instance.
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
/// OauthTokenCredentialManager Constructor
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

    httpClient = std::make_shared<CurlEasyClient>(constants::max_concurrency_oauth);
    refreshTokenCallback = refreshCallback;

    try {
        refresh_token();
    } catch(std::runtime_error& ex) {
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
/// Furthermore, note that refresh_token does not manage the token lock itself-- Please lock the token if you plan to explicitly refresh.
/// <param>
/// </summary>
OAuthToken OAuthTokenCredentialManager::refresh_token()
{
    try {
        current_oauth_token = refreshTokenCallback(httpClient);
        valid_authentication = true;
        return current_oauth_token;
    } catch(std::runtime_error& ex) {
        valid_authentication = false;
        throw ex;
    }
}

/// <summary>
/// Returns current oauth_token, implicitly refreshing if the current token is invalid.
/// This will _not_ throw. It will just syslog if a token refresh failed.
/// </summary>
OAuthToken OAuthTokenCredentialManager::get_token()
{
    if (is_token_expired()) {
        // Lock the mutex.
        if (token_mutex.try_lock()) {
            try {
                // Attempt to refresh.
                refresh_token();
            } catch (std::runtime_error &ex) {
                // If we fail, explain ourselves and unlock.
                syslog(LOG_ERR, "Unable to retrieve OAuth token: %s", ex.what());
                valid_authentication = false;
                token_mutex.unlock();
            }
        } else {
            time_t current_time;
            time(&current_time);

            // There's a five minute segment where the token hasn't actually expired yet, but we're already trying to refresh it.
            // We can just use the currently active token instead, rather than waiting for the refresh.
            // This'll save some downtime on systems under constant use.
            if (current_time < current_oauth_token.expires_on) {
                return current_oauth_token;
            } else {
                // If it's not still live, let's just wait for the refresh to finish.
                // This is a sub-optimal method to wait for this event as it can end up blocking other routines after the token has finished refreshing.
                // If we were working in Go, I (Adele) would suggest using a sync.WaitGroup for this functionality.
                token_mutex.lock();
            }
        }
        // Note that we don't always lock in this function and sometimes return early.
        // Be sure to ensure you're safely manipulating the lock when modifying this function.
        token_mutex.unlock();
    }

    return current_oauth_token;
}

/// <summary>
/// TODO: check the expiry time against the current utc time
/// <summary>
bool OAuthTokenCredentialManager::is_token_expired()
{
    if(!valid_authentication)
        return true;

    time_t current_time;

    time ( &current_time );

    // check if about to expire via the buffered expiry time
    return current_time + (60 * 5) >= current_oauth_token.expires_on;
}

// ===== CALLBACK SETUP ZONE =====

/// <summary>
/// SetUpMSICallback sets up a refresh callback for MSI auth. This should be used to create a OAuthTokenManager instance.
/// </summary>
std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpMSICallback(std::string client_id_p, std::string object_id_p, std::string resource_id_p, std::string msi_endpoint_p)
{
    // Create the URI token request
    std::shared_ptr<microsoft_azure::storage::storage_url> uri_token_request_url;
    bool custom_endpoint = !msi_endpoint_p.empty();
    if (!custom_endpoint) {
        uri_token_request_url = parse_url(constants::msi_request_uri);

        if(!client_id_p.empty())
        {
            uri_token_request_url->add_query(constants::param_client_id, client_id_p);
        }
        if(!object_id_p.empty())
        {
            uri_token_request_url->add_query(constants::param_object_id, object_id_p);
        }
        if(!resource_id_p.empty())
        {
            uri_token_request_url->add_query(constants::param_mi_res_id, resource_id_p);
        }
    }
    else
    {
        uri_token_request_url = parse_url(msi_endpoint_p);
    }

    uri_token_request_url->add_query(constants::param_mi_api_version, constants::param_mi_api_version_data);
    uri_token_request_url->add_query(constants::param_oauth_resource, constants::param_oauth_resource_data);

    return [uri_token_request_url, client_id_p, custom_endpoint](std::shared_ptr<CurlEasyClient> httpClient) {
        // prepare the CURL handle
        std::shared_ptr<CurlEasyRequest> request_handle = httpClient->get_handle();

        request_handle->set_url(uri_token_request_url->to_string());
        request_handle->add_header(constants::header_metadata, "true");
        if(custom_endpoint)
            request_handle->add_header(constants::header_msi_secret, client_id_p);

        request_handle->set_method(http_base::http_method::get);

        // Set up the output stream for the request
        storage_iostream ios = storage_iostream::create_storage_stream();
        request_handle->set_output_stream(ios.ostream());

        // TODO: decide retry interval, also make constant
        std::chrono::seconds retry_interval(constants::max_retry_oauth);
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