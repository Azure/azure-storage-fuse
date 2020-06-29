#include "test_base.h"

// tell Catch to provide a main()
#define CATCH_CONFIG_MAIN
#include "catch2/catch.hpp"

#include <unordered_map>
#include <sstream>
#include <iostream>
#include <vector>
#include <random>

namespace as_test {
    static thread_local std::mt19937_64 random_generator(std::random_device{}());

    char get_random_char()
    {
        const char charset[] = "0123456789abcdefghijklmnopqrstuvwxyz";
        std::uniform_int_distribution<size_t> distribution(0, sizeof(charset) - 2);
        return charset[distribution(random_generator)];
    }

    std::string get_random_string(size_t size) {
        std::string str(size, 0);
        std::generate(str.begin(), str.end(), get_random_char);
        return str;
    }

    std::istringstream get_istringstream_with_random_buffer(size_t size) {
        std::istringstream ss;
        ss.str(get_random_string(size));
        return ss;
    }

    char* get_random_buffer(size_t size) {
        char* buffer = new char[size];
        std::generate(buffer, buffer + size, get_random_char);
        return buffer;
    }

    std::string to_base64(const char* base, size_t length)
    {
        static const char* base64_enctbl = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
        std::string result;
        for (int offset = 0; length - offset >= 3; offset += 3)
        {
            const char* ptr = base + offset;
            unsigned char idx0 = ptr[0] >> 2;
            unsigned char idx1 = ((ptr[0] & 0x3) << 4) | ptr[1] >> 4;
            unsigned char idx2 = ((ptr[1] & 0xF) << 2) | ptr[2] >> 6;
            unsigned char idx3 = ptr[2] & 0x3F;
            result.push_back(base64_enctbl[idx0]);
            result.push_back(base64_enctbl[idx1]);
            result.push_back(base64_enctbl[idx2]);
            result.push_back(base64_enctbl[idx3]);
        }
        switch (length % 3)
        {
        case 1:
        {

            const char* ptr = base + length - 1;
            unsigned char idx0 = ptr[0] >> 2;
            unsigned char idx1 = ((ptr[0] & 0x3) << 4);
            result.push_back(base64_enctbl[idx0]);
            result.push_back(base64_enctbl[idx1]);
            result.push_back('=');
            result.push_back('=');
            break;
        }
        case 2:
        {

            const char* ptr = base + length - 2;
            unsigned char idx0 = ptr[0] >> 2;
            unsigned char idx1 = ((ptr[0] & 0x3) << 4) | ptr[1] >> 4;
            unsigned char idx2 = ((ptr[1] & 0xF) << 2);
            result.push_back(base64_enctbl[idx0]);
            result.push_back(base64_enctbl[idx1]);
            result.push_back(base64_enctbl[idx2]);
            result.push_back('=');
            break;
        }
        }
        return result;
    }

    azure::storage_lite::blob_client& base::test_blob_client(int size) {
        static std::unordered_map<int, std::shared_ptr<azure::storage_lite::blob_client>> bcs;
        if (bcs[size] == NULL)
        {
            bcs[size] = std::make_shared<azure::storage_lite::blob_client>(azure::storage_lite::blob_client(init_account(standard_storage_connection_string()), size));
        }
        return *bcs[size];
    }

    const std::shared_ptr<azure::storage_lite::storage_account> base::init_account(const std::string& connection_string) {
        auto settings = parse_string_into_settings(connection_string);
        auto credential = std::make_shared<azure::storage_lite::shared_key_credential>(azure::storage_lite::shared_key_credential(settings["AccountName"], settings["AccountKey"]));
        bool use_https = true;
        if (settings["DefaultEndpointsProtocol"] == "http")
        {
            use_https = false;
        }
        std::string blob_endpoint;
        if (!settings["BlobEndpoint"].empty())
        {
            blob_endpoint = settings["BlobEndpoint"];
        }

        return std::make_shared<azure::storage_lite::storage_account>(azure::storage_lite::storage_account(settings["AccountName"], credential, use_https, blob_endpoint));
    }

    std::map<std::string, std::string> base::parse_string_into_settings(const std::string& connection_string)
    {
        std::map<std::string, std::string> settings;
        std::vector<std::string> splitted_string;

        // Split the connection string by ';'
        {
            std::istringstream iss(connection_string);
            std::string s;
            while (getline(iss, s, ';')) {
                splitted_string.push_back(s);
            }
        }

        for (auto iter = splitted_string.cbegin(); iter != splitted_string.cend(); ++iter)
        {
            if (!iter->empty())
            {
                auto equals = iter->find('=');

                std::string key = iter->substr(0, equals);
                if (!key.empty())
                {
                    std::string value;
                    if (equals != std::string::npos)
                    {
                        value = iter->substr(equals + 1);
                    }

                    settings.insert(std::make_pair(std::move(key), std::move(value)));
                }
                else
                {
                    throw std::logic_error("The format of connection string cannot be recognized.");
                }
            }
        }

        return settings;
    }
}
