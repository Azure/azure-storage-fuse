#include "blob_integration_base.h"
#include "mstream.h"

#include "catch2/catch.hpp"

#include <iostream>
#include <random>
#include <chrono>
#include <limits>

TEST_CASE("SingleThreadPerformance", "[performance][!hide]")
{
    azure::storage_lite::blob_client client = as_test::base::test_blob_client();

    std::string container_name = "large-scale-test";
    std::string blob_name = as_test::get_random_string(10);

    auto ret = client.create_container(container_name).get();
    if (!ret.success() && ret.error().code != "409")
    {
        std::cout << "Failed to create container, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }

    std::size_t buffer_size = 1024 * 1024 * 1024;
    std::string buffer;
    buffer.resize(buffer_size);
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<uint64_t> distrib(0, std::numeric_limits<uint64_t>::max());
    for (std::size_t i = 0; i < buffer_size; i += sizeof(uint64_t))
        *(reinterpret_cast<uint64_t*>(&buffer[0] + i)) = distrib(gen);
    azure::storage_lite::imstream istream(buffer.data(), buffer.size());
    std::vector<std::pair<std::string, std::string>> metadata;
    auto timer_start = std::chrono::system_clock::now();
    ret = client.upload_block_blob_from_stream(container_name, blob_name, istream, metadata).get();
    auto timer_end = std::chrono::system_clock::now();

    if (!ret.success())
    {
        std::cout << "Failed to upload blob, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }
    else
    {
        double speed = static_cast<double>(buffer.length()) / 1024 / 1024
            / std::chrono::duration_cast<std::chrono::milliseconds>(timer_end - timer_start).count()
            * 1000;
        std::cout << "Upload speed: " << speed << "MiB/s" << std::endl;
    }

    azure::storage_lite::omstream ostream(&buffer[0], buffer.size());
    timer_start = std::chrono::system_clock::now();
    ret = client.download_blob_to_stream(container_name, blob_name, 0, 0, ostream).get();
    timer_end = std::chrono::system_clock::now();
    if (!ret.success())
    {
        std::cout << "Failed to download blob, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }
    else
    {
        double speed = static_cast<double>(buffer.length()) / 1024 / 1024
            / std::chrono::duration_cast<std::chrono::milliseconds>(timer_end - timer_start).count()
            * 1000;
        std::cout << "Download speed: " << speed << "MiB/s" << std::endl;
    }
}

TEST_CASE("MultiThreadPerformance", "[performance][!hide]")
{
    int concurrency = 24;

    azure::storage_lite::blob_client client = as_test::base::test_blob_client(concurrency);

    std::string container_name = "large-scale-test";
    std::string blob_name = as_test::get_random_string(10);

    auto ret = client.create_container(container_name).get();
    if (!ret.success() && ret.error().code != "409")
    {
        std::cout << "Failed to create container, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }

    std::size_t buffer_size = 8 * 1024 * 1024 * 1024ULL;
    std::string buffer;
    buffer.resize(buffer_size);
    std::random_device rd;
    std::mt19937 gen(rd());
    std::uniform_int_distribution<uint64_t> distrib(0, std::numeric_limits<uint64_t>::max());
    for (std::size_t i = 0; i < buffer_size; i += sizeof(uint64_t))
        *(reinterpret_cast<uint64_t*>(&buffer[0] + i)) = distrib(gen);
    std::vector<std::pair<std::string, std::string>> metadata;
    auto timer_start = std::chrono::system_clock::now();
    ret = client.upload_block_blob_from_buffer(container_name, blob_name, buffer.data(), metadata, buffer.length(), concurrency).get();
    auto timer_end = std::chrono::system_clock::now();

    if (!ret.success())
    {
        std::cout << "Failed to upload blob, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }
    else
    {
        double speed = static_cast<double>(buffer.length()) / 1024 / 1024
            / std::chrono::duration_cast<std::chrono::milliseconds>(timer_end - timer_start).count()
            * 1000;
        std::cout << concurrency << " thread upload speed: " << speed << "MiB/s" << std::endl;
    }

    timer_start = std::chrono::system_clock::now();
    ret = client.download_blob_to_buffer(container_name, blob_name, 0, buffer.size(), &buffer[0], concurrency).get();
    timer_end = std::chrono::system_clock::now();
    if (!ret.success())
    {
        std::cout << "Failed to download blob, Error: " << ret.error().code << ", " << ret.error().code_name << std::endl;
    }
    else
    {
        double speed = static_cast<double>(buffer.length()) / 1024 / 1024
            / std::chrono::duration_cast<std::chrono::milliseconds>(timer_end - timer_start).count()
            * 1000;
        std::cout << concurrency << " thread download speed: " << speed << "MiB/s" << std::endl;
    }
}
