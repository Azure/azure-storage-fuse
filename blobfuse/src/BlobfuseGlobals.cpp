#include "BlobfuseGlobals.h"
#include <ctype.h>
#include <sys/utsname.h>

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
            if(!config_options.accountKey.empty()) // An account name is already expected to be specified.
                return KEY_AUTH;
            else
                return INVALID_AUTH;
        } else if (lcAuthType == "sas") {
            if (!config_options.sasToken.empty()) // An account name is already expected to be specified.
                return SAS_AUTH;
            else
                return INVALID_AUTH;
        } else if (lcAuthType == "spn") {
            return SPN_AUTH;
        }
    } else {
        if (!config_options.objectId.empty() ||
            !config_options.identityClientId.empty() ||
            !config_options.resourceId.empty() ||
            !config_options.msiSecret.empty() ||
            !config_options.msiEndpoint.empty()) {
            return MSI_AUTH;
        } else if (!config_options.accountKey.empty()) {
            return KEY_AUTH;
        } else if (!config_options.sasToken.empty()) {
            return SAS_AUTH;
        } else if (!config_options.spnClientSecret.empty() && !config_options.spnClientId.empty() && !config_options.spnTenantId.empty()){
            return SPN_AUTH;
        }
    }
    return INVALID_AUTH;
}

float kernel_version = 0.0;
void populate_kernel_version()
{
    struct utsname buffer;
	if (uname (&buffer) == 0) {
		char *p = buffer.release;
		int i = 0;
		float ver[5];

		while (*p) {
			if (isdigit(*p)) {
				ver[i] = strtof(p, &p);
				i++;
			} else {
				p++;
			}
			if (i >= 5) break;
		}
		if (i > 2)
            kernel_version = ver[0];
	}
}