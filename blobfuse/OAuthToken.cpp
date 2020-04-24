//
// OAuthToken.cpp . This class is used to validate a token passed in and extract all the meaningful fields from it. 
// It is also used to convert a string to a token class. It follows the OAuth2 guidelines for token expiration 
//

#include "OAuthToken.h"
#include <iomanip>
#include <time.h>
#include <syslog.h>

bool OAuthToken::empty() 
{
    // We consider a totally unusable oauth token as empty because it doesn't make sense to treat it as a usable one.
    return access_token.empty() && refresh_token.empty();
}

void to_json(json &j, const OAuthToken &t) 
{
    j = json{
            {"access_token",  t.access_token},
            {"refresh_token", t.refresh_token},
            {"expires_in",    std::to_string(t.expires_in)},
            {"expires_on",    std::to_string(t.expires_on)},
            {"not_before",    std::to_string(t.not_before)},
            {"resource",      t.resource},
            {"token_type",    t.token_type}
    };
}

void from_json(const json &j, OAuthToken &t) 
{
    // Just print out the string, leave it as WARNING not INFO because users usually don't turn on INFO logging, this will be useful for debugging OAuthtoken mismatches/varieties
    std::string s = j.dump();   
    syslog(LOG_WARNING, "INFO not WARNING: Printing Json Token as string for visibility%s\n", s.c_str());
    
    t.access_token = j.value("access_token","");
    t.refresh_token = j.value("refresh_token", "");

    if (t.access_token.empty()) 
    {
        throw std::runtime_error("OAuth token is unusable: Oauth token did not return with an access token.");
    }

    t.resource = j.value("resource", "");
    t.token_type = j.value("token_type", "");

    // If there is expires_in honor only that, otherwise look for expires_on and parse that   
    bool expin_failed = false;
    try 
    {
        if (j.contains("expires_in"))
        {
            auto val = j.at("expires_in");
            if(val.is_number())
            {
                val.get_to(t.expires_in);
            }
            else 
            {
                std::string expires_in = val.get<std::string>();
                
                if (is_dt_number(expires_in)) // check with the custom method as the above does not catch everything
                {
                    t.expires_in = std::stoi(expires_in);
                }    
                else
                {
                    syslog(LOG_WARNING, "Token does not have expires_in date");
                    expin_failed = true;
                }
            }
        }
        else
        {
            syslog(LOG_WARNING, "Token does not have expires_in date");
            expin_failed = true;
        }
    } 
    catch(std::exception&)
    {
        syslog(LOG_WARNING, "Token does not have expires_in date");
        expin_failed = true;
    }
    
    // if there is an expires_in just use it and dont worry parsing expires_on field
    if (!expin_failed) 
    {
        time_t current_time;
            
        struct tm *temptm;
            
        time(&current_time);
            
        temptm = gmtime( &current_time );

        current_time = mktime(temptm);

        t.expires_on = current_time + t.expires_in;    
        
        syslog(LOG_WARNING, "After adding specified expires_in token expiry time in utc %s\n", ctime(&t.expires_on));
    }
    else if (j.contains("expires_on"))
    {
        try 
        {
            auto val = j.at("expires_on");
            if(val.is_number())
            {
                // if the expires_on it is localtime. Do not convert it.
                syslog(LOG_INFO, "expires_on date is a number so using val_get_to to convert it");
                val.get_to(t.expires_on);
            }
            else 
            {
                std::string expires_on = val.get<std::string>();
                
                if (is_dt_number(expires_on)) // check with the custom method as the above does not catch everything
                {
                    t.expires_on = std::stoi(expires_on);
                    syslog(LOG_WARNING, "After conerting the UTC integer token expiry time in utc %s\n", ctime(&t.expires_on));
                }    
                else 
                { // now the date is a string in either the UTC or localtime format so just parse it
                    fprintf(stdout, "expires_on date is string");
                    
                    
                    // try UTC first then use else to implement in local time in AM or PM
                    int utcIndex = expires_on.find("+0000");
                    if ( utcIndex > 19)
                    { 
                       // remove timezone
                        expires_on = expires_on.substr(0, utcIndex);
                        // remove the trailing space if any
                        expires_on.erase(std::find_if(expires_on.rbegin(), expires_on.rend(), std::bind1st(std::not_equal_to<char>(), ' ')).base(), expires_on.end());
                        // Now remove the milliseconds because strptime is going crazy with those milliseconds anyway we dont use them so.
                        int millisecondsIndex  =  expires_on.find(".");
                        if (millisecondsIndex > 18)
                        {
                            expires_on = expires_on.substr(0, millisecondsIndex);
                        }
                        struct tm timeStruct;
                        // Ref: for formats: https://www.tutorialcup.com/cplusplus/date-time.htm
                        if ((strptime(expires_on.c_str(), "%Y-%m-%d %H:%M:%S", &timeStruct) != NULL)
                            ||
                            (strptime(expires_on.c_str(), "%Y-%b-%d %H:%M:%S", &timeStruct) != NULL))                  
                            {                                   
                                t.expires_on = mktime(&timeStruct);
                                if (t.expires_on == -1)
                                {
                                    syslog(LOG_ERR, "Incorrect UTC date format %s", val.get<std::string>().c_str());
                                }
                            }
                    }
                    else
                    {
                        syslog(LOG_ERR, "parsing expires_on failed. Blobfuse cannot auth: OAuth token is unusable: OAuth token has an expires_on date in a non-UTC format, only UTC is supported for string format dates.\n Examples of correct UTC format dates are \"2020-04-14 16:49:11.72 +0000 UTC\" and \"2020-Apr-14 16:49:11.72 +0000 UTC\" \n Cannot use expires_in as it is missing too.\n");
                        throw std::runtime_error("Blobfuse cannot auth: OAuth token is unusable: OAuth token has an expires_on date in a non-UTC format, only UTC is supported for string format dates.\n Examples of correct UTC format dates are \"2020-04-14 16:49:11.72 +0000 UTC\" and \"2020-Apr-14 16:49:11.72 +0000 UTC\" \n Cannot use expires_in as it is missing too.\n");
                    }
                }
            }
        
        } 
        catch(std::exception&)
        {
            fprintf(stdout, "Blobfuse cannot auth: parsing expires_on in the OAuthToken failed. It is not an integer or string\n");
            throw std::runtime_error("Blobfuse cannot auth: OAuth token is unusable: OAuth token did not return with an expiry time of any form.");
        }
    }
    else
    {
        throw std::runtime_error("OAuth token is unusable: OAuth token did not return with an expiry time of any form.Both expires_in and expires_on are missing in the OAuth token.");
    }

    if (j.contains("not_before"))
    {
        std::string not_before;
        try 
        {
            not_before = j.at("not_before");
            t.not_before = std::stoi(not_before);
        } 
        catch(std::exception&)
        {
            syslog(LOG_INFO, "Incorrect Not before date format %s", not_before.c_str());
        } // We don't particularly care about the not_before field in blobfuse so only send an info.
    }
}

/// Helper method for parsing a string char by char and evaluating if it has any non-digits
/// If the string is empty or has non-digits a false is returned otherwise a true value is returned.
/// This is used in this class for calculating time from dates that come in as strings but are essentially integers that say the minutes elapsed from 1970.
bool is_dt_number(const std::string &s)
{
    return !s.empty() && std::find_if(s.begin(), s.end(), [](unsigned char c) { return !std::isdigit(c); }) == s.end();
}