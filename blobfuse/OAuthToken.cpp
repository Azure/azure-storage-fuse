//
// Created by adreed on 11/21/19.
//

#include "OAuthToken.h"
#include <iomanip>

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
                t.expires_on = std::stoi(expires_on);
            }
        }
        else
        {
            printf("no expon");
            expon_failed = true;
        }
    } catch(std::exception&){
        expon_failed = true;
    }

    try {
        std::string not_before;
        not_before = j.at("not_before");
        t.not_before = std::stoi(not_before);
    } catch(std::exception&){} // We don't particularly care about the not_before field in blobfuse.

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