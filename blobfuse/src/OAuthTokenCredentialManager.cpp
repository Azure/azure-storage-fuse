//
// OAuthTokenCredentialManager.cpp
// This class calls the Aunthentication provider and caches an OAuthToke.
// It refreshes the token 5 minutes before expiry
//
#include "OAuthTokenCredentialManager.h"
#include <http_base.h>
#include <json.hpp>
#include <iomanip>
#include "OAuthToken.h"
#include "constants.h"
#include "utility.h"
#include <syslog.h>
#include "BlobfuseConstants.h"

using nlohmann::json;

std::string GetTokenCallback()
{
    std::shared_ptr<OAuthTokenCredentialManager> tokenManager = GetTokenManagerInstance(EmptyCallback);
    OAuthToken temp_token = tokenManager->get_token();
    return temp_token.access_token;
}

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
        syslog(LOG_ERR, "OAuthTokenManager was supplied an invalid refresh callback.\n");
        printf("OAuthTokenManager was supplied an invalid refresh callback.\n");
        return;
    }
    else 
    {
        syslog(LOG_INFO, "refresh callback set");
    }

    httpClient = std::make_shared<CurlEasyClient>(blobfuse_constants::max_concurrency_oauth);
    refreshTokenCallback = refreshCallback;

    try 
    {
        syslog(LOG_INFO, "calling refresh token\n");
        refresh_token();
    } 
    catch(std::runtime_error& ex)
    {
        syslog(LOG_ERR, "Unable to retrieve OAuth token: %s\n", ex.what());
        fprintf(stderr, "Unable to retrieve OAuth token: %s\n", ex.what());
        valid_authentication = false;
        return;
    }

    if(current_oauth_token.empty()) 
    {
        valid_authentication = false;
        syslog(LOG_ERR, "Unable to retrieve OAuth Token with given credentials.");
    }
}


void OAuthTokenCredentialManager::StartTokenMonitor()
{
    #ifdef TOKEN_REFRESH_THREAD
    std::thread t1(std::bind(&OAuthTokenCredentialManager::TokenMonitor, this));
    t1.detach();
    syslog(LOG_WARNING,"OAUTH Token : Token expiry monitor started");
    #endif
}


#ifdef TOKEN_REFRESH_THREAD
#ifdef TEST_TOKEN_THR
unsigned int test_cnt = 0;
#endif
void OAuthTokenCredentialManager::TokenMonitor()
{
    int retry_count = 0;
    bool refreshed = false;

    while(true)
    {
        if (!is_token_expired())
        {
             // Anyway we try to refresh token 5 minutes before so it is ok to sleep for 60 seconds.
            sleep(60);
            continue;
        }

        retry_count = 0;
        refreshed = false;

        token_mutex.lock();
        while ((!refreshed) && (retry_count < 5)) {
            try {
                // Attempt to refresh.
                fprintf(stdout, "OAUTH Token : TokenMonitor : token expired so calling refresh_token() %d\n", retry_count);
                syslog(LOG_WARNING, "OAUTH Token : TokenMonitor : token expired so calling refresh_token() %d\n", retry_count);
            
                refresh_token();
                refreshed = true;
            } catch (std::runtime_error &ex) {
                // If we fail, explain ourselves and unlock.
                syslog(LOG_ERR, "OAUTH Token : TokenMonitor : Failed to refresh (%s)\n", ex.what());
                syslog(LOG_ERR, "OAUTH Token : TokenMonitor : Retry refreshing the token attempt %d", retry_count++);
                usleep(10 * 1000);
            }
        }
        token_mutex.unlock();

        if (retry_count >= 5) {
            syslog(LOG_ERR, "OAUTH Token : TokenMonitor : token Expired and refresh has failed\n");
        }
    }
}
#endif

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
    #ifdef TEST_TOKEN_THR
    if ((test_cnt > 2) && (test_cnt % 5 == 0)) {
        throw std::runtime_error("Failed");
    }
    #endif

    #ifdef TOKEN_REFRESH_THREAD
    valid_authentication = false;
    #endif

    try
    {
        current_oauth_token = refreshTokenCallback(httpClient);
        valid_authentication = true;
        syslog(LOG_ERR,"OAUTH Token : Refresh token succeeded expiry time is: %s", asctime(gmtime(&current_oauth_token.expires_on)));
        return current_oauth_token;
    } 
    catch(std::runtime_error& ex) 
    {
        valid_authentication = false;
        fprintf(stderr,"OAUTH Token : Refresh token failed %s", ex.what());
        syslog(LOG_ERR, "OAUTH Token : Refresh token failed %s", ex.what());
        throw ex;
    }
}

/// <summary>
/// Returns current oauth_token, implicitly refreshing if the current token is invalid.
/// This will _not_ throw. It will just syslog if a token refresh failed.
/// </summary>
OAuthToken OAuthTokenCredentialManager::get_token()
{
    #ifdef TOKEN_REFRESH_THREAD
    // since the thread would have updated the expired token just check if valid_authentication
   // and return the current token, control will fall to the if is_token_expired() below if this directive is undefined.
    if (valid_authentication)
    {
        return current_oauth_token;
    }
    #endif
    syslog(LOG_DEBUG, "No thread to refresh token so checking if the token has expired ...\n");
    if (is_token_expired()) {
        // Lock the mutex.
        if (token_mutex.try_lock()) {
            try {
                // Attempt to refresh.
                fprintf(stdout, "oauth token has expired so calling refresh_token()\n");
                syslog(LOG_WARNING, "oauth token has expired so calling refresh_token()\n");
          
                refresh_token();
            } catch (std::runtime_error &ex) {
                // If we fail, explain ourselves and unlock.
                syslog(LOG_ERR, "Unable to retrieve OAuth token: %s\n", ex.what());
                valid_authentication = false;
                token_mutex.unlock();
            }
        } else {
            fprintf(stdout, "Locking mutex failed, so some token is being acquired., so just wait and get that\n");
            syslog(LOG_WARNING, "Locking mutex failed, so some token is being acquired., so just wait and get that\n");
            
            time_t current_time;

            current_time =  get_current_time_in_utc();

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
/// check the expiry time againsyt the current utc time
/// <summary>
bool OAuthTokenCredentialManager::is_token_expired()
{
    #ifdef TEST_TOKEN_THR
    test_cnt++;

    if (test_cnt % 4 == 0) {
        return true;
    }
    #endif

    if(!valid_authentication)
    {
        syslog(LOG_INFO, "At is_token_expired: valid_authentication is false so token expired");
    
        return true;
    }
    return is_token_expired_forcurrentutc(current_oauth_token);
    
}

/// <summary>
/// check the expiry time against the current utc time
/// <summary>
bool is_token_expired_forcurrentutc(OAuthToken &token)
{
    time_t current_time;

    current_time =  get_current_time_in_utc();
    
    // Even if 5 minutes are left to expire we want to request a new token, so make the current clock look 5 minutes ahead.

    time_t safety_current_time = current_time + (60 * 5); // time_t adds time seconds, we are adding 5 minutes here

    bool isExpired = safety_current_time >= token.expires_on;    
   
    // check if about to expire via the buffered expiry time
    return isExpired;
}

/// <summary>
/// get current time in utc
/// <summary>
time_t get_current_time_in_utc()
{
    time_t current_time;

    struct tm *temptm;
    
    // get the current time
        
    time(&current_time);
    //convert it to UTC
            
    temptm = gmtime( &current_time );

    current_time = mktime(temptm);
    
    return current_time;
}

// ===== CALLBACK SETUP ZONE =====

// Is this duplicating code present in cpplite? _yes_ but no.
// CPPlite's version of this treats the ENTIRE STRING as the full query... This means stuff like ?, =, and / didn't get encoded.
// This encodes the actual element, rather than the full query.
std::string encode_query_element(std::string input) {
    std::stringstream result;

    for (unsigned long i = 0; i < input.length(); i++) {
        // ASCII 65 to 90 (A-Z), ASCII 97-122 (a-z) are OK
        // ASCII 48 to 57 (0-9) are OK
        // *, -, ., _ are OK
        // + gets encoded
        // space becomes %20
        // ~ should __probably__ be encoded as well

        char x = input[i];

        // A-Z, a-z, 0-9
        if((x >= 65 && x <= 90) || (x >= 97 && x <= 122) || (x >= 48 && x <= 57))
        {
            result << x;
        }
        else if (x == '*' || x == '-' || x == '.' || x == '_')
        { // *, -, ., _
            result << x;
        }
        else
        {
            result << "%" << std::uppercase << std::hex << (int) x;
        }
    }

    return result.str();
}


// The URLs this is going to need to parse are fairly unadvanced.
// They'll all be similar to http://blah.com/path1/path2?query1=xxx&query2=xxx
// It's assumed they will be pre-encoded.
// This is _primarily_ to support the custom MSI endpoint scenario requested by AML.
std::shared_ptr<storage_url> parse_url(const std::string& url) {
    auto output = std::make_shared<storage_url>();

    std::string runningString;
    std::string qpname; // A secondary buffer for query parameter strings.
    // 0 = scheme, 1 = hostname, 2 = path, 3 = query
    // the scheme ends up attached to the hostname due to the way storage_urls work.
    int segment = 0;
    for (auto charptr = url.begin(); charptr < url.end(); charptr++) {
        switch (segment) {
            case 0:
                runningString += *charptr;

                // ends up something like "https://"
                if (*(charptr - 2) == ':' && *(charptr - 1) == '/' && *charptr == '/')
                {
                    // We've reached the end of the scheme.
                    segment++;
                }
                break;
            case 1:
                // Avoid adding the / between the path element and the domain, as storage_url does that for us.
                if(*charptr != '/')
                    runningString += *charptr;

                if (*charptr == '/' || charptr == url.end() - 1)
                {
                    // Only append the new char if it's the end of the string.
                    output->set_domain(std::string(runningString));
                    // empty the buffer, do not append the new char to the string because storage_url handles it for us, rather than checking itself
                    runningString.clear();
                    segment++;
                }
                break;
            case 2:
                // Avoid adding the ? to the path.
                if(*charptr != '?')
                    runningString += *charptr;

                if (*charptr == '?' || charptr == url.end() - 1)
                {
                    // We don't need to append by segment here, we can just append the entire thing.
                    output->append_path(std::string(runningString));
                    // Empty the buffer
                    runningString.clear();
                    segment++;
                }
                break;
            case 3:
                // Avoid adding any of the separators to the path.
                if (*charptr != '=' && *charptr != '&')
                    runningString += *charptr;

                if (*charptr == '=')
                {
                    qpname = std::string(runningString);
                    runningString.clear();
                }
                else if (*charptr == '&' || charptr == url.end() - 1)
                {
                    output->add_query(std::string(qpname), std::string(runningString));
                    qpname.clear();
                    runningString.clear();
                }
                break;
            default:
                throw std::runtime_error("Unexpected segment section");
        }
    }

    return output;
}


/// <summary>
/// SetUpMSICallback sets up a refresh callback for MSI auth. This should be used to create a OAuthTokenManager instance.
/// </summary>
std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpMSICallback(std::string client_id_p, std::string object_id_p, std::string resource_id_p, std::string msi_endpoint_p, std::string msi_secret_p)
{
    // Create the URI token request
    std::shared_ptr<azure::storage_lite::storage_url> uri_token_request_url;
    bool custom_endpoint = !msi_endpoint_p.empty();
    if (!custom_endpoint) {
        uri_token_request_url = parse_url(blobfuse_constants::msi_request_uri);

        if(!client_id_p.empty())
        {
            uri_token_request_url->add_query(blobfuse_constants::param_client_id, client_id_p);
        }
        if(!object_id_p.empty())
        {
            uri_token_request_url->add_query(blobfuse_constants::param_object_id, object_id_p);
        }
        if(!resource_id_p.empty())
        {
            uri_token_request_url->add_query(blobfuse_constants::param_mi_res_id, resource_id_p);
        }
    }
    else
    {
        uri_token_request_url = parse_url(msi_endpoint_p);

        if(!client_id_p.empty())
        { // The alternate endpoint in the doc uses clientid as its parameter name, not client_id.
            uri_token_request_url->add_query("clientid", client_id_p);
        }
         fprintf(stdout, "SetUP MSI callback with custom token issuing %s, Identity client %s\n", msi_endpoint_p.c_str(), client_id_p.c_str());
         syslog(LOG_INFO, "SetUP MSI callback with custom token issuing endpoint ");
    }

    uri_token_request_url->add_query(blobfuse_constants::param_mi_api_version, blobfuse_constants::param_mi_api_version_data);
    uri_token_request_url->add_query(blobfuse_constants::param_oauth_resource, blobfuse_constants::param_oauth_resource_data);
    
    fprintf(stdout, "URI token request URL printed out %s \n", uri_token_request_url->to_string().c_str());

    return [uri_token_request_url, msi_secret_p, custom_endpoint](std::shared_ptr<CurlEasyClient> httpClient) {
        // prepare the CURL handle
        std::shared_ptr<CurlEasyRequest> request_handle = httpClient->get_handle();

        request_handle->set_url(uri_token_request_url->to_string());
        request_handle->add_header(blobfuse_constants::header_metadata, "true");
        if(custom_endpoint)
            request_handle->add_header(blobfuse_constants::header_msi_secret, msi_secret_p);

        request_handle->set_method(http_base::http_method::get);

        // Set up the output stream for the request
        storage_iostream ios = storage_iostream::create_storage_stream();
        request_handle->set_output_stream(ios.ostream());

        // TODO: decide retry interval, also make constant
        std::chrono::seconds retry_interval(blobfuse_constants::max_retry_oauth);
        OAuthToken parsed_token;
        request_handle->submit([&parsed_token, &ios](http_base::http_code http_code_result, const storage_istream&, CURLcode curl_code)
        {
            if (curl_code != CURLE_OK || unsuccessful(http_code_result)) 
            {
                std::string req_result;

                try 
                { 
                // to avoid crashing to any potential errors while reading the stream, we back it with a try catch statement.
                    std::string json_request_result(std::istreambuf_iterator<char>(ios.istream()),
                                                 std::istreambuf_iterator<char>());

                    req_result = json_request_result;
                } 
                catch(std::exception& exp)
                {
                    syslog(LOG_WARNING, "Error while reading json token return stream %s\n" ,exp.what()); 
                }

                std::ostringstream errStream;
                errStream << "Failed to retrieve OAuth Token from IMDS endpoint (CURLCode: " << curl_code << ", HTTP code: " << http_code_result << "): " << req_result;
                throw std::runtime_error(errStream.str());
            }
            else
            {
                std::string json_request_result(std::istreambuf_iterator<char>(ios.istream()),
                                             std::istreambuf_iterator<char>());

                try 
                {
                    json j;
                    j = json::parse(json_request_result);
                    parsed_token = j.get<OAuthToken>();
                } 
                catch(std::exception& ex) 
                {
                    throw std::runtime_error(std::string("Failed to parse OAuth token: ") + std::string(ex.what()));
                }
            }
        }, retry_interval);

        // request_handle destructs because it's no longer referenced
        return parsed_token;
    };
}

std::function<OAuthToken(std::shared_ptr<CurlEasyClient>)> SetUpSPNCallback(std::string tenant_id_p, std::string client_id_p, std::string client_secret_p, std::string aad_endpoint_p)
{
    // Requesting a service principal token requires we folow the client creds flow.
    // https://docs.microsoft.com/en-us/azure/active-directory/develop/v2-oauth2-client-creds-grant-flow
    // This means that we need to put our query parameters in the body, which seems quite odd.
    // They _also_ need to be URL-encoded. Luckily, storage_url had an unexposed encode_query function.
    // azure::storage_lite::encode_url_query()

    // Step 1: Construct the URL
    std::shared_ptr<azure::storage_lite::storage_url> uri_token_request_url = std::make_shared<azure::storage_lite::storage_url>();

   if(aad_endpoint_p.empty())
    {
        uri_token_request_url->set_domain(blobfuse_constants::oauth_request_uri);
    }
    else
    {
        uri_token_request_url = parse_url(aad_endpoint_p);
    }

    uri_token_request_url->append_path((tenant_id_p.empty() ? "common" : tenant_id_p) + "/" + blobfuse_constants::spn_request_path); // /tenant/oauth2/v2.0/token

    // Step 2: Construct the body query
    std::string queryString("client_id=" + client_id_p); // client_id=...
    queryString.append("&scope=" + encode_query_element(std::string(blobfuse_constants::param_oauth_resource_data) + ".default")); // &scope=https://storage.azure.com/.default
    queryString.append("&client_secret=" + encode_query_element(client_secret_p)); // &client_secret=...
    queryString.append("&grant_type=client_credentials"); // &grant_type=client_credentials


    syslog(LOG_DEBUG, "SPN auth uri token request url = %s", uri_token_request_url->to_string().c_str());

    return [uri_token_request_url, queryString](std::shared_ptr<CurlEasyClient> http_client) {
        std::shared_ptr<CurlEasyRequest> request_handle = http_client->get_handle();

        // set up URI and headers
       request_handle->set_url(uri_token_request_url->to_string());
       request_handle->add_header("Content-Type", "application/x-www-form-urlencoded");

       // Set up body query, SPN expects it in the body
       auto body = std::make_shared<std::stringstream>(queryString);
       request_handle->set_input_stream(storage_istream(body));
       request_handle->set_input_content_length(queryString.length());
       request_handle->add_header("Content-Length", std::to_string(queryString.length()));

        // Set up output stream
        storage_iostream ios = storage_iostream::create_storage_stream();
        request_handle->set_output_stream(ios.ostream());

       // Set request method
        request_handle->set_method(http_base::http_method::post);

        std::chrono::seconds retry_interval(blobfuse_constants::max_retry_oauth);
        OAuthToken parsed_token;

        request_handle->submit([&parsed_token, &ios](http_base::http_code http_code_result, const storage_istream&, CURLcode curl_code){
            if (curl_code != CURLE_OK || unsuccessful(http_code_result)) {
                std::string req_result;

                try {
                    std::string json_request_result(std::istreambuf_iterator<char>(ios.istream()),
                                                    std::istreambuf_iterator<char>());
                    req_result = json_request_result;
                } 
                catch(std::exception &ex)
                {
                    syslog(LOG_WARNING, "Exception while extracting the SPN auth unsuccessful http_code %s", ex.what());
                }

                std::ostringstream errStream;
                errStream << "Failed to retrieve OAuth Token (CURLCode: " << curl_code << ", HTTP code: " << http_code_result << "): " << req_result;
                throw std::runtime_error(errStream.str());
            } else {
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
    
        return parsed_token;
    };
}
    