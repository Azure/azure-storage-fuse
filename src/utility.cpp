#include <ctime>
#include <iomanip>
#include <sstream>

#include "utility.h"

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

namespace azure {  namespace storage_lite {

    std::string to_lowercase(std::string str)
    {
        std::transform(str.begin(), str.end(), str.begin(), [](char c) { return char(std::tolower(c)); });
        return str;
    }

    std::string get_uuid()
    {
        std::string res;
#ifdef _WIN32
        UUID uuid;
        UuidCreate(&uuid);
        char* uuid_cstr = nullptr;
        UuidToStringA(&uuid, reinterpret_cast<RPC_CSTR*>(&uuid_cstr));
        res = std::string(uuid_cstr);
        RpcStringFreeA(reinterpret_cast<RPC_CSTR*>(&uuid_cstr));
#else
        uuid_t uuid;
        char uuid_cstr[37]; // 36 byte uuid plus null.
        uuid_generate(uuid);
        uuid_unparse(uuid, uuid_cstr);
        res = std::string(uuid_cstr);
#endif

        return res;
    }

    bool create_or_resize_file(const std::string& path, unsigned long long length) noexcept
    {
#ifdef _WIN32
        HANDLE h_file = CreateFile(path.data(), GENERIC_READ | GENERIC_WRITE, 0, NULL, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);

        if (h_file == INVALID_HANDLE_VALUE)
        {
            return false;
        }

        LARGE_INTEGER distance;
        distance.QuadPart = length;
        bool ret = SetFilePointerEx(h_file, distance, nullptr, FILE_BEGIN) && SetEndOfFile(h_file);

        CloseHandle(h_file);
        return ret;
#else
        auto fd = open(path.c_str(), O_WRONLY, 0770);
        if (-1 == fd) {
            return false;
        }
        if (-1 == ftruncate(fd, length)) {
            close(fd);
            return false;
        }
        close(fd);
        return true;
#endif
    }

    std::string get_ms_date(date_format format)
    {
        std::time_t t = std::time(nullptr);
        std::tm *pm;
#ifdef _WIN32
        std::tm m;
        pm = &m;
        gmtime_s(pm, &t);
#else
        pm = std::gmtime(&t);
#endif
        if (format == date_format::iso_8601)
        {
            char buf[32];
            std::strftime(buf, sizeof(buf), constants::date_format_iso_8601, pm);
            return std::string(buf);
        }
        else if (format == date_format::rfc_1123)
        {
            std::stringstream ss;
            ss.imbue(std::locale("C"));
            ss << std::put_time(pm, constants::date_format_rfc_1123);
            return ss.str();
        }
        else
        {
            throw std::runtime_error("unknown datetime format");
        }
    }

    std::string get_ms_range(unsigned long long start_byte, unsigned long long end_byte)
    {
        std::string result;
        if (start_byte == 0 && end_byte == 0)
        {
            return result;
        }
        result.append("bytes=" + std::to_string(start_byte) + "-");
        if (end_byte != 0) {
            result.append(std::to_string(end_byte));
        }
        return result;
    }

    std::string get_http_verb(http_base::http_method method)
    {
        switch (method)
        {
        case http_base::http_method::del:
            return constants::http_delete;
        case http_base::http_method::get:
            return constants::http_get;
        case http_base::http_method::head:
            return constants::http_head;
        case http_base::http_method::post:
            return constants::http_post;
        case http_base::http_method::put:
            return constants::http_put;
        case http_base::http_method::patch:
            return constants::http_patch;
        }
        return std::string();
    }

    void add_access_condition_headers(http_base &h, storage_headers &headers, const blob_request_base &r)
    {
        if (!r.if_modified_since().empty())
        {
            h.add_header(constants::header_if_modified_since, r.if_modified_since());
            headers.if_modified_since = r.if_modified_since();
        }
        if (!r.if_match().empty())
        {
            h.add_header(constants::header_if_match, r.if_match());
            headers.if_match = r.if_match();
        }
        if (!r.if_none_match().empty())
        {
            h.add_header(constants::header_if_none_match, r.if_none_match());
            headers.if_none_match = r.if_none_match();
        }
        if (!r.if_unmodified_since().empty())
        {
            h.add_header(constants::header_if_unmodified_since, r.if_unmodified_since());
            headers.if_unmodified_since = r.if_unmodified_since();
        }
    }

    bool retryable(http_base::http_code status_code)
    {
        if (status_code == 408 /*Request Timeout*/)
        {
            return true;
        }
        if (status_code >= 300 && status_code < 500)
        {
            return false;
        }
        if (status_code == 501 /*Not Implemented*/ || status_code == 505 /*HTTP Version Not Supported*/)
        {
            return false;
        }
        return true;
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

}}   // azure::storage_lite
