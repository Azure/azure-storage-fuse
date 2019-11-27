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
    try {
        std::string expires_in;
        j.at("expires_in").get_to(expires_in);
        t.expires_in = std::stoi(expires_in);
    } catch(std::exception&){}

    try {
        std::string expires_on;
        j.at("expires_on").get_to(expires_on);
        t.expires_on = std::stoi(expires_on);
    } catch(std::exception&){}

    try {
        std::string not_before;
        not_before = j.at("not_before");
        t.not_before = std::stoi(not_before);
    } catch(std::exception&){}

    // TODO: Infer expries_on if expires_in is not present, vice versa
    //  this will __PROBABLY__ never happen. But, just on the off-chance that it does, we should be able to cope.
}