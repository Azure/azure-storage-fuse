#include <ctime>

#include "utility.h"

#include "constants.h"

#ifdef _WIN32
#include <windows.h>
#include <filesystem>
#else
#include <uuid/uuid.h>
#include <sys/stat.h>
#include <sys/types.h>
#include <fcntl.h>
#include <unistd.h>
#endif
namespace microsoft_azure {
    namespace storage {
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
        try
        {
            std::experimental::filesystem::resize_file(path, static_cast<uintmax_t>(length));
        }
        catch (...)
        {
            return false;
        }
        return true;
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
        std::string get_ms_date(date_format format) {
            char buf[30];
            std::time_t t = std::time(nullptr);
            std::tm *pm;
#ifdef WIN32
            std::tm m;
            pm = &m;
            gmtime_s(pm, &t);
#else
            pm = std::gmtime(&t);
#endif
            size_t s = std::strftime(buf, 30, (format == date_format::iso_8601 ? constants::date_format_iso_8601 : constants::date_format_rfc_1123), pm);
            return std::string(buf, s);
        }

        std::string get_ms_range(unsigned long long start_byte, unsigned long long end_byte) {
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

        std::string get_http_verb(http_base::http_method method) {
            switch (method) {
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
            }
            return std::string();
        }

        void add_access_condition_headers(http_base &h, storage_headers &headers, const blob_request_base &r) {
            if (!r.if_modified_since().empty()) {
                h.add_header(constants::header_if_modified_since, r.if_modified_since());
                headers.if_modified_since = r.if_modified_since();
            }
            if (!r.if_match().empty()) {
                h.add_header(constants::header_if_match, r.if_match());
                headers.if_match = r.if_match();
            }
            if (!r.if_none_match().empty()) {
                h.add_header(constants::header_if_none_match, r.if_none_match());
                headers.if_none_match = r.if_none_match();
            }
            if (!r.if_unmodified_since().empty()) {
                h.add_header(constants::header_if_unmodified_since, r.if_unmodified_since());
                headers.if_unmodified_since = r.if_unmodified_since();
            }
        }

        bool retryable(http_base::http_code status_code) {
            if (status_code == 408 /*Request Timeout*/) {
                return true;
            }
            if (status_code >= 300 && status_code < 500) {
                return false;
            }
            if (status_code == 501 /*Not Implemented*/ || status_code == 505 /*HTTP Version Not Supported*/) {
                return false;
            }
            return true;
        }

    }
}
