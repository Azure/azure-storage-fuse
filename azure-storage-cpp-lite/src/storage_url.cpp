#include "storage_url.h"

namespace microsoft_azure {
    namespace storage {
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

        std::string encode_url_path(const std::string& path)
        {
            const char* const hex = "0123456789ABCDEF";
            std::string encoded;
            for(size_t index = 0; index < path.size(); ++index)
            {
                char ch = path[index];
                if(!is_path_character(ch)
                    || ch == '%'
                    || ch == '+'
                    || ch == '&')
                {
                    encoded.push_back('%');
                    encoded.push_back(hex[ (ch >> 4) & 0xF]);
                    encoded.push_back(hex[ ch & 0xF ]);
                }
                else
                {
                    encoded.push_back(ch);
                }
            }
            return encoded;
        }

        std::string encode_url_query(const std::string& path)
        {
            const char* const hex = "0123456789ABCDEF";
            std::string encoded;
            for(size_t index = 0; index < path.size(); ++index)
            {
                char ch = path[index];
                if(!is_query_character(ch)
                    || ch == '%'
                    || ch == '+'
                    || ch == '&')
                {
                    encoded.push_back('%');
                    encoded.push_back(hex[ (ch >> 4) & 0xF]);
                    encoded.push_back(hex[ ch & 0xF ]);
                }
                else
                {
                    encoded.push_back(ch);
                }
            }
            return encoded;
        }

        std::string storage_url::to_string() const {
            std::string url(m_domain);
            url.append(encode_url_path(m_path));

            bool first_query = true;
            for (const auto &q : m_query) {
                if (first_query) {
                    url.append("?");
                    first_query = false;
                }
                else {
                    url.append("&");
                }
                for (const auto &value : q.second) {
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
