//
// Created by adreed on 11/21/19.
//

#include "OAuthToken.h"
#include <iomanip>

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
    if (j.contains("access_token"))
    {
        j.at("access_token").get_to(t.access_token);
    }

    if (j.contains("refresh_token"))
    {
        j.at("refresh_token").get_to(t.refresh_token);
    }

    if (j.contains("expires_in"))
    {
        std::string expires_in;
        j.at("expires_in").get_to(expires_in);
        t.expires_in = std::stoi(expires_in);
    }

    if (j.contains("expires_on"))
    {
        std::string expires_on;
        j.at("expires_on").get_to(expires_on);
        t.expires_on = std::stoi(expires_on);
    }

    if (j.contains("not_before"))
    {
        std::string not_before;
        not_before = j.at("not_before");
        t.not_before = std::stoi(not_before);
    }

    if (j.contains("resource"))
    {
        j.at("resource").get_to(t.resource);
    }

    if (j.contains("token_type"))
    {
        j.at("token_type").get_to(t.token_type);
    }
}