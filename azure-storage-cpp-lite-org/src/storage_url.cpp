#include <ctime>

#include "storage_url.h"

#include "constants.h"

#ifdef _WIN32
#define WIN32_LEAN_AND_MEAN
#include <Windows.h>
#else
#include <uuid/uuid.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <fcntl.h>
#include <unistd.h>
#endif
#include <cctype>
#include <vector>
#include <algorithm>

namespace microsoft_azure 
{
    namespace storage 
    {
        bool is_alnum(char ch)        
        {
            return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9');
        }

        bool is_unreserved(char ch)
        {
            return is_alnum(ch) || ch == '-' || ch == '.' || ch == '_' || ch == '~';
        }
        bool is_sub_delim(char ch)
        {
            switch(ch)
            {
            case '!':
            case '$':
            case '&':
            case '\'':
            case '(':
            case ')':
            case '*':
            case '+':
            case ',':
            case ';':
            case '=':
                return true;
            default:
                return false;
            }
        }

        bool is_path_character(char ch)
        {
            return is_unreserved(ch) || is_sub_delim(ch) || ch == '%' || ch == '/' || ch == ':' || ch == '@';
        }
        
        bool is_query_character(char ch)
        {
            return is_path_character(ch) || ch == '?';
        }

        static const char* unreserved = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~";
        static const char* subdelimiters = "!$&'()*+,;=";
        static const char* encoded_chars[] = {
        "%00", "%01", "%02", "%03", "%04", "%05", "%06", "%07", "%08", "%09", "%0A", "%0B", "%0C", "%0D", "%0E", "%0F",
        "%10", "%11", "%12", "%13", "%14", "%15", "%16", "%17", "%18", "%19", "%1A", "%1B", "%1C", "%1D", "%1E", "%1F",
        "%20", "%21", "%22", "%23", "%24", "%25", "%26", "%27", "%28", "%29", "%2A", "%2B", "%2C", "%2D", "%2E", "%2F",
        "%30", "%31", "%32", "%33", "%34", "%35", "%36", "%37", "%38", "%39", "%3A", "%3B", "%3C", "%3D", "%3E", "%3F",
        "%40", "%41", "%42", "%43", "%44", "%45", "%46", "%47", "%48", "%49", "%4A", "%4B", "%4C", "%4D", "%4E", "%4F",
        "%50", "%51", "%52", "%53", "%54", "%55", "%56", "%57", "%58", "%59", "%5A", "%5B", "%5C", "%5D", "%5E", "%5F",
        "%60", "%61", "%62", "%63", "%64", "%65", "%66", "%67", "%68", "%69", "%6A", "%6B", "%6C", "%6D", "%6E", "%6F",
        "%70", "%71", "%72", "%73", "%74", "%75", "%76", "%77", "%78", "%79", "%7A", "%7B", "%7C", "%7D", "%7E", "%7F",
        "%80", "%81", "%82", "%83", "%84", "%85", "%86", "%87", "%88", "%89", "%8A", "%8B", "%8C", "%8D", "%8E", "%8F",
        "%90", "%91", "%92", "%93", "%94", "%95", "%96", "%97", "%98", "%99", "%9A", "%9B", "%9C", "%9D", "%9E", "%9F",
        "%A0", "%A1", "%A2", "%A3", "%A4", "%A5", "%A6", "%A7", "%A8", "%A9", "%AA", "%AB", "%AC", "%AD", "%AE", "%AF",
        "%B0", "%B1", "%B2", "%B3", "%B4", "%B5", "%B6", "%B7", "%B8", "%B9", "%BA", "%BB", "%BC", "%BD", "%BE", "%BF",
        "%C0", "%C1", "%C2", "%C3", "%C4", "%C5", "%C6", "%C7", "%C8", "%C9", "%CA", "%CB", "%CC", "%CD", "%CE", "%CF",
        "%D0", "%D1", "%D2", "%D3", "%D4", "%D5", "%D6", "%D7", "%D8", "%D9", "%DA", "%DB", "%DC", "%DD", "%DE", "%DF",
        "%E0", "%E1", "%E2", "%E3", "%E4", "%E5", "%E6", "%E7", "%E8", "%E9", "%EA", "%EB", "%EC", "%ED", "%EE", "%EF",
        "%F0", "%F1", "%F2", "%F3", "%F4", "%F5", "%F6", "%F7", "%F8", "%F9", "%FA", "%FB", "%FC", "%FD", "%FE", "%FF"
        };

        std::string encode_url_path(const std::string& path)
        {
            static const std::vector<uint8_t> is_path_char = []()
            {
                std::vector<uint8_t> ret(256, 0);
                for (char c : std::string(unreserved) + std::string(subdelimiters) + "%!@")
                {
                    ret[c] = 1;
                }
                // Parameter path is already joint with '/'.
                ret['/'] = 1;
                return ret;
            }();

            std::string result;
            for (char c : path)
            {
                if (is_path_char[c])
                {
                    result += c;
                }
                else
                {
                    result += encoded_chars[static_cast<unsigned char>(c)];
                }
            }

            return result;
        }

        std::string encode_url_query(const std::string& query)
        {
            static const std::vector<uint8_t> is_query_char = []()
            {
                std::vector<uint8_t> ret(256, 0);
                for (char c : std::string(unreserved) + std::string(subdelimiters) + "%!@/?")
                {
                    ret[c] = 1;
                }
                // Literal + needs to be encoded
                ret['+'] = 0;
                // Surprisingly, '=' also needs to be encoded because Azure Storage server side is so strict.
                ret['='] = 0;
                return ret;
            }();

            std::string result;
            for (char c : query)
            {
                if (is_query_char[c])
                {
                    result += c;
                }
                else
                {
                    result += encoded_chars[static_cast<unsigned char>(c)];
                }
            }

            return result;
        }

        std::string storage_url::to_string() const
        {
            std::string url(m_domain);
            url.append(encode_url_path(m_path));

            bool first_query = true;
            for (const auto &q : m_query) 
            {
                if (first_query) 
                {
                    url.append("?");
                    first_query = false;
                }
                else 
                {
                    url.append("&");
                }
                for (const auto &value : q.second) 
                {
                    url.append(encode_url_query(q.first)).append("=").append(encode_url_query(value));
                }
            }
            return url;
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
    }
}
