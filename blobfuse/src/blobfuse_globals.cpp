#include "blobfuse_globals.h"

std::string to_lower(std::string original) 
{
    std::string out;

    for (auto idx = original.begin(); idx < original.end(); idx++) {
        if(*idx >= 'A' && *idx <= 'Z') {
            out += char(*idx + 32); // This cast isn't required, but clang-tidy wants to complain without it.
        } else {
            out += *idx;
        }
    }

    return out;
}

inline bool is_lowercase_string(const std::string &s)
{
    return (s.size() == static_cast<size_t>(std::count_if(s.begin(), s.end(),[](unsigned char c)
    {
        return std::islower(c);
    })));
}

AUTH_TYPE get_auth_type(std::string authStr) 
{
    if(!authStr.empty()) {
        std::string lcAuthType = to_lower(authStr);
        if (lcAuthType == "msi") {
            // MSI does not require any parameters to work, asa a lone system assigned identity will work with no parameters.
            return MSI_AUTH;
        } else if (lcAuthType == "key") {
            if(!str_options.accountKey.empty()) // An account name is already expected to be specified.
                return KEY_AUTH;
            else
                return INVALID_AUTH;
        } else if (lcAuthType == "sas") {
            if (!str_options.sasToken.empty()) // An account name is already expected to be specified.
                return SAS_AUTH;
            else
                return INVALID_AUTH;
        } else if (lcAuthType == "spn") {
            return SPN_AUTH;
        }
    } else {
        if (!str_options.objectId.empty() ||
            !str_options.identityClientId.empty() ||
            !str_options.resourceId.empty() ||
            !str_options.msiSecret.empty() ||
            !str_options.msiEndpoint.empty()) {
            return MSI_AUTH;
        } else if (!str_options.accountKey.empty()) {
            return KEY_AUTH;
        } else if (!str_options.sasToken.empty()) {
            return SAS_AUTH;
        } else if (!str_options.spnClientSecret.empty() && !str_options.spnClientId.empty() && !str_options.spnTenantId.empty()){
            return SPN_AUTH;
        }
    }
    return INVALID_AUTH;
}