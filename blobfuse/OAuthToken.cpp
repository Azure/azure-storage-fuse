//
// Created by adreed on 11/21/19.
//

#include "OAuthToken.h"
#include <iomanip>
#include <time.h>
#include <syslog.h>

bool OAuthToken::empty() {
    // We consider a totally unusable oauth token as empty because it doesn't make sense to treat it as a usable one.
    return access_token.empty() && refresh_token.empty();
}

void to_json(json &j, const OAuthToken &t) {
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

void from_json(const json &j, OAuthToken &t) {
	// JUST PRINT OUT THE STRIGN SO THAT WE KNOW THE CUSTOM ENDPOINT IS SENDING THE RIGHT JSON
	std::string s = j.dump();   
	syslog(LOG_WARNING, "Printing Json Token as string %s\n", s.c_str());
	fprintf(stdout, "printing json token as string %s\n", s.c_str());
	
    t.access_token = j.value("access_token","");
    t.refresh_token = j.value("refresh_token", "");

    if (t.access_token.empty()) {
        throw std::runtime_error("OAuth token is unusable: Oauth token did not return with an access token.");
    }


    t.resource = j.value("resource", "");
    t.token_type = j.value("token_type", "");

    // The below factors need numeric conversion, so we'll attempt that.
    // try/catch individually so that if something like expires_in fails over, we don't lose the more important detail, expires_on
    bool expin_failed = false;
    try {
        if (j.contains("expires_in"))
        {
            auto val = j.at("expires_in");
            if(val.is_number())
            {
                val.get_to(t.expires_in);
            }
            else {
                std::string expires_in = val.get<std::string>();
				
				//TODO: On 4/12/2020 by Nara. the below line will give garage because it is clearly not a number. Why do this?? 
                t.expires_in = std::stoi(expires_in);
            }
        }
        else
        {
            printf("no expin");
            expin_failed = true;
        }
    } catch(std::exception&){
        expin_failed = true;
    }

    bool expon_failed = false;
    try {
        if (j.contains("expires_on"))
        {
            auto val = j.at("expires_on");
            if(val.is_number())
            {
                val.get_to(t.expires_on);
            }
            else {
                std::string expires_on = val.get<std::string>();
				
				// try UTC first then use else to implement in local time in AM or PM
				int utcIndex = expires_on.find("+0000");
				if ( utcIndex > 19)
				{ 
			    // remove tiemzone
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
							t.expires_on = timegm(&timeStruct);
							if (t.expires_on == -1)
							{
								fprintf(stderr, "Incorrect UTC date format %s", val.get<std::string>().c_str());
								syslog(LOG_ERR, "Incorrect UTC date format %s", val.get<std::string>().c_str());
							}
						}
				}
            }
        }
        else
        {
            expon_failed = true;
        }
    } catch(std::exception&){
        expon_failed = true;
    }

	if (j.contains("not_before"))
    {
		
			std::string not_before;
		try {
			not_before = j.at("not_before");
			t.not_before = std::stoi(not_before);
		} 
		catch(std::exception&){
			fprintf(stdout, "Incorrect Not before date %s", not_before.c_str());
			syslog(LOG_WARNING, "Incorrect Not before date format %s", not_before.c_str());
		} // We don't particularly care about the not_before field in blobfuse so only throw a warning.
	}

    if(expon_failed) {
        // We can failover and set this manually via expires_in. If expires_in failed as well, there isn't much we can do.
        if(expin_failed) {
            throw std::runtime_error("OAuth token is unusable: OAuth token did not return with an expiry time of any form.");
        } else {
            time_t current_time;
            time(&current_time);

            t.expires_on = current_time + t.expires_in;
        }
    }

    // TODO: Infer expries_on if expires_in is not present, vice versa
    //  this will __PROBABLY__ never happen. But, just on the off-chance that it does, we should be able to cope.
}