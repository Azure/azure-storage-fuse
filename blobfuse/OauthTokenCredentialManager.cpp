//
// Created by amanda on 11/19/19.
//
#include <time.h>
#include "OauthTokenCredentialManager.h"
#include <http_base.h>

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
    request_handle->add_header(constants::header_metadata, constants::header_value_metadata);

    printf("%s", uri_token_request.c_str());

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
/// <param>
/// </summary>
const std::string OauthTokenCredentialManager::refresh_token()
{
    // This solution to lock the refresh method
    if(token_mutex.try_lock())
    {
        //TODO: decide retry interval, also make constant
        std::chrono::seconds retry_interval(5);
        std::string parsed_token;
        request_handle->submit([parsed_token](http_base::http_code http_code_result, storage_istream resultStream, CURLcode curl_code )
        {
            if(curl_code != CURLE_OK || unsuccessful(http_code_result))
            {
                syslog(LOG_ERR, "Unable to retrieve Oauth Token //todo better message");
                printf("curlcode: %d\n", curl_code);
                printf("httpcode: %d\n", http_code_result);
            }
            else
            {
                //TODO: parsing json request for oauth token and expiry time
                // set parsed_token here
                std::string json_request_result(std::istreambuf_iterator<char> (resultStream.istream()),
                                                std::istreambuf_iterator<char>());
                syslog(LOG_DEBUG, "Retrieved json Oauth Token: %s", json_request_result.c_str());
                printf("%s", json_request_result.c_str());
                //parsed_token = json_request_result;
            }
        }, retry_interval);
    }
    else
    {
        //TODO: check if the expirey token is still good and just return the current token to allow other threads
        // to keep chugging along while we try to refresh the token.
        token_mutex.lock();
    }
    token_mutex.unlock();
    return current_oauth_token;
}

/// <summary>
/// Returns current oauth_token
/// </summary>
const std::string OauthTokenCredentialManager::get_token()
{
    return current_oauth_token;
}

/// <summary>
/// TODO: check the expiry time against the current utc time
/// <summary>
bool OauthTokenCredentialManager::is_token_expired()
{
    time_t current_time;

    time ( &current_time );

    // check if expired with buffer
    if( (current_time + 5) > expiry_time)
    {
        return false;
    }
    return true;
}