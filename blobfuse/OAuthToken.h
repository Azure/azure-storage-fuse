//
// Created by adreed on 11/21/19.
//
#pragma once
#include <cstdint>
#include <string>
#include <json.hpp>

using json = nlohmann::json;

/*
{
  "access_token": "eyJ0eXAi...",
  "refresh_token": "",
  "expires_in": "3599",
  "expires_on": "1506484173",
  "not_before": "1506480273",
  "resource": "https://management.azure.com/",
  "token_type": "Bearer"
}
*/

class OAuthToken {
public:
    bool empty();

    std::string access_token; // The access token to be used.
    std::string refresh_token; // The refresh token to be used.
    uint32_t expires_in; // Defines the number of seconds until the token's intended expiry.
    time_t expires_on; // Defines the UTC-based UNIX time representation in which the token will expire
    time_t not_before; // Defines the UTC-based UNIX time representation when this token CANNOT be used before
    std::string resource; // Defines the azure resource this targets
    std::string token_type; // Defines the type of token, typically Bearer.
};

void from_json(const json &j, OAuthToken &t);
void to_json(json &j, const OAuthToken &t);
