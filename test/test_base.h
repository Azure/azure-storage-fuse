#pragma once

#include "../include/blob/blob_client.h"

#include <string>

namespace as_test {

    std::string get_random_string(size_t size);
    std::istringstream get_istringstream_with_random_buffer(size_t size);
    char* get_random_buffer(size_t size);
    std::string to_base64(const char* base, size_t length);

    class base {
    public:
        static azure::storage_lite::blob_client& test_blob_client(int size = 1);

        static const std::string& standard_storage_connection_string() {
            static std::string sscs = "DefaultEndpointsProtocol=https;";
            return sscs;
        }

    protected:
        static const std::shared_ptr<azure::storage_lite::storage_account> init_account(const std::string& connection_string);
        static std::map<std::string, std::string> parse_string_into_settings(const std::string& connection_string);
    };
}
