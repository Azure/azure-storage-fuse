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

    }
}
