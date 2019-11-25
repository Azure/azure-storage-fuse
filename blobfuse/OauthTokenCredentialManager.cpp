//
// Created by amanda on 11/19/19.
//
#include <time.h>
#include "OauthTokenCredentialManager.h"
#include <http_base.h>
#include <json.hpp>
#include <iomanip>
#include "OAuthToken.h"

using nlohmann::json;

/// <summary>
/// OauthTokenCredentialManager Constructor for MSI
/// Creates the MSI Request URI and requests an OAuth token and expiry time for that token
/// </summary>
OauthTokenCredentialManager::OauthTokenCredentialManager(
    std::string client_id_p,
    std::string object_id_p,
    std::string resource_id_p)
{
    // Create the URI token request
    uri_token_request = constants::msi_request_uri;
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

    // TODO: set constant for concurrency and figure out a better number for concurrency
    httpClient = std::make_shared<CurlEasyClient>(20);
    request_handle = httpClient->get_handle();

    request_handle->set_url(uri_token_request);
    request_handle->add_header(constants::header_metadata, "true");
    request_handle->set_method(http_base::http_method::get);

    printf("Token request URL: %s\n", uri_token_request.c_str());
    printf("https_proxy: %s, http_proxy: %s\n", std::getenv("https_proxy"), std::getenv("http_proxy"));

    // Create the OAuth token
    current_oauth_token = refresh_token();
    if(current_oauth_token.empty())
    {
        valid_authentication = false;
        syslog(LOG_ERR, "Unable to retrieve Oauth Token with given MSI credentials//todo better message");
        printf("Unable to retrieve Oauth Token with given MSI credentials. Authentication failed//todo better message");
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
std::string OauthTokenCredentialManager::refresh_token()
{
    // This solution to lock the refresh method
    if(token_mutex.try_lock())
    {
        storage_iostream ios = storage_iostream::create_storage_stream();
        request_handle->set_output_stream(ios.ostream());

        //TODO: decide retry interval, also make constant
        std::chrono::seconds retry_interval(5);
        std::string parsed_token;
        time_t parsed_expiry_time;
        request_handle->submit([&parsed_token, &parsed_expiry_time, &ios](http_base::http_code http_code_result, storage_istream resultStream, CURLcode curl_code )
        {
            if(curl_code != CURLE_OK || unsuccessful(http_code_result))
            {
                syslog(LOG_ERR, "Unable to retrieve Oauth Token //todo better message");
                printf("curlcode: %d\n", curl_code);
                printf("httpcode: %d\n", http_code_result);
            }
            else
            {
                std::string json_request_result(std::istreambuf_iterator<char> (ios.istream()),
                                                std::istreambuf_iterator<char>());
                printf("raw json: %s\n", json_request_result.c_str());

                json j;
                j = json::parse(json_request_result);
                OAuthToken t = j.get<OAuthToken>();

                //std::cout << "Expires on: " << std::chrono::system_clock::to_time_t(t.expires_on) << std::endl;
                std::cout << "Expires on: " << std::put_time(std::localtime(&t.expires_on), "%c %Z") << std::endl;
                time_t current = std::time(nullptr);
                std::cout << "Current time: " << std::put_time(std::localtime(&current), "%c %Z") << std::endl;
                printf("token: %s\n", t.access_token.c_str());


                parsed_token = t.access_token;
                parsed_expiry_time = t.expires_on;
            }
        }, retry_interval);

        current_oauth_token = parsed_token;
        expiry_time = parsed_expiry_time;
        valid_authentication = true;
        token_mutex.unlock();
    }
    else
    {
        if(is_token_expired()) {
            token_mutex.lock();
            token_mutex.unlock();
        }
    }
    return current_oauth_token;
}

/// <summary>
/// Returns current oauth_token
/// </summary>
std::string OauthTokenCredentialManager::get_token()
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
    return current_time + (60 * 5) <= expiry_time;
}